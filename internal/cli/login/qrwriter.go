package login

import (
	"github.com/mattn/go-runewidth"
	termbox "github.com/nsf/termbox-go"
	"github.com/yeqown/go-qrcode/v2"
)

// compactQRWriter is a compact QR code writer using half-block characters
// for 2x vertical compression (▀▄█ ) and 1 char per block horizontally.
type compactQRWriter struct {
	matrix [][]bool
	width  int
	height int
}

func vGetCompactQRWriter() *compactQRWriter {
	return &compactQRWriter{}
}

func (w *compactQRWriter) Write(mat qrcode.Matrix) error {
	w.width = mat.Width()
	w.height = mat.Height()
	w.matrix = make([][]bool, w.height)
	for y := 0; y < w.height; y++ {
		w.matrix[y] = make([]bool, w.width)
	}

	mat.Iterate(qrcode.IterDirection_ROW, func(x int, y int, v qrcode.QRValue) {
		w.matrix[y][x] = v.IsSet()
	})

	return w.render()
}

func (w *compactQRWriter) Close() error {
	termbox.Close()
	return nil
}

func (w *compactQRWriter) getPixel(x, y int) bool {
	if x < 0 || x >= w.width || y < 0 || y >= w.height {
		return false // outside bounds = white (quiet zone)
	}
	return w.matrix[y][x]
}

func (w *compactQRWriter) render() error {
	if err := termbox.Init(); err != nil {
		return err
	}
	termbox.SetOutputMode(termbox.Output256)

	padding := 2 // quiet zone
	// Calculate display dimensions (half height due to half-blocks)
	displayHeight := (w.height+2*padding+1)/2 + padding
	displayWidth := w.width + 2*padding

	// Pre-fill with white background
	for y := 0; y < displayHeight+2; y++ {
		for x := 0; x < displayWidth; x++ {
			termbox.SetCell(x, y, ' ', termbox.ColorWhite, termbox.ColorWhite)
		}
	}

	// Draw QR code using half-block characters
	// Each terminal row represents 2 QR rows
	for ty := 0; ty < displayHeight; ty++ {
		for tx := 0; tx < displayWidth; tx++ {
			// Map terminal coords back to QR coords (accounting for padding)
			qrX := tx - padding
			qrY1 := ty*2 - padding*2     // top pixel
			qrY2 := ty*2 + 1 - padding*2 // bottom pixel

			top := w.getPixel(qrX, qrY1)
			bot := w.getPixel(qrX, qrY2)

			var ch rune
			var fg, bg termbox.Attribute

			// QR: IsSet()=true means black module
			if top && bot {
				// Both black: full block
				ch = '█'
				fg = termbox.ColorBlack
				bg = termbox.ColorWhite
			} else if !top && !bot {
				// Both white: space
				ch = ' '
				fg = termbox.ColorWhite
				bg = termbox.ColorWhite
			} else if top && !bot {
				// Top black, bottom white: upper half block
				ch = '▀'
				fg = termbox.ColorBlack
				bg = termbox.ColorWhite
			} else {
				// Top white, bottom black: lower half block
				ch = '▄'
				fg = termbox.ColorBlack
				bg = termbox.ColorWhite
			}

			termbox.SetCell(tx, ty, ch, fg, bg)
		}
	}

	// Print tip
	tip := "扫码后，手机点击确认，成功后按任意键继续..."
	tipY := displayHeight + 1
	x := 0
	for _, r := range tip {
		rw := runewidth.RuneWidth(r)
		if rw == 0 {
			rw = 1
		}
		termbox.SetCell(x, tipY, r, termbox.ColorDefault, termbox.ColorDefault)
		x += rw
	}

	if err := termbox.Flush(); err != nil {
		return err
	}

	// Wait for key press
	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			return nil
		}
	}
}
