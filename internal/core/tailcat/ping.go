package tailcat

import (
	"context"
	"errors"
	"time"
)

// PingResult describes one probe of a tailcat server.
type PingResult struct {
	RelayRTT time.Duration // registration round trip through the relay
	Latency  time.Duration // disco ping round trip
	Direct   bool          // the pong came back over a direct UDP path
	Endpoint string        // the direct ip:port, when Direct
	Relay    string        // the relay carrying traffic, when not Direct
}

// ErrNoDirectPath is returned by Ping when untilDirect was set and no direct
// path was found before the timeout; traffic then flows through the relay.
var ErrNoDirectPath = errors.New("no direct path to the server; traffic goes through the relay")

// Ping registers with the server behind addr and disco-pings it, which also
// nudges NAT traversal along. With untilDirect it keeps pinging once a second
// (bounded by timeout) until a pong arrives over a direct path, reporting each
// pong to report. It returns the last result.
func Ping(ctx context.Context, addr string, untilDirect bool, timeout time.Duration, report func(PingResult)) (PingResult, error) {
	var last PingResult
	t, err := tunnelFor(addr)
	if err != nil {
		return last, err
	}
	deadline := time.Now().Add(timeout)
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	relay := RelayOf(addr)
	first, err := t.c.Ping(ctx)
	if err != nil {
		return last, err
	}
	t.dialed.Store(true)
	for {
		t0 := time.Now()
		res, err := t.c.DiscoPing(ctx)
		if err != nil {
			if untilDirect && errors.Is(err, context.DeadlineExceeded) {
				return last, ErrNoDirectPath
			}
			return last, err
		}
		last = PingResult{
			RelayRTT: first.Latency,
			Latency:  time.Duration(res.LatencySeconds * float64(time.Second)).Round(10 * time.Microsecond),
			Direct:   res.Endpoint != "",
			Endpoint: res.Endpoint,
			Relay:    relay,
		}
		if !last.Direct && last.Relay == "" {
			last.Relay = res.DERPRegionCode
		}
		if report != nil {
			report(last)
		}
		if last.Direct || !untilDirect {
			return last, nil
		}
		if time.Until(deadline) < time.Second/2 {
			return last, ErrNoDirectPath
		}
		select {
		case <-ctx.Done():
			return last, ErrNoDirectPath
		case <-time.After(max(0, time.Second-time.Since(t0))):
		}
	}
}
