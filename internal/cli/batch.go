package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/guiyumin/vget/internal/core/extractor"
)

// runBatch reads URLs from a file and downloads each one
func runBatch(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var urls []string
	var invalidURLs []string
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Normalize URL: add https:// if missing
		normalized, err := extractor.NormalizeURL(line)
		if err != nil {
			invalidURLs = append(invalidURLs, fmt.Sprintf("line %d: %s", lineNum, line))
			continue
		}
		urls = append(urls, normalized)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if len(urls) == 0 {
		if len(invalidURLs) > 0 {
			return fmt.Errorf("no valid URLs found in file (%d invalid)", len(invalidURLs))
		}
		return fmt.Errorf("no URLs found in file")
	}

	// Warn about invalid URLs
	if len(invalidURLs) > 0 {
		fmt.Printf("\033[33mWarning: %d invalid URL(s) skipped:\033[0m\n", len(invalidURLs))
		for _, u := range invalidURLs {
			fmt.Printf("  - %s\n", u)
		}
		fmt.Println()
	}

	fmt.Printf("Found %d URL(s) to download\n\n", len(urls))

	var succeeded, failed int
	var failedURLs []string

	for i, url := range urls {
		fmt.Printf("[%d/%d] %s\n", i+1, len(urls), truncateURL(url, 60))

		if err := runDownload(url); err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			failed++
			failedURLs = append(failedURLs, url)
		} else {
			succeeded++
		}
		fmt.Println()
	}

	// Print summary
	fmt.Println("----------------------------------------")
	fmt.Printf("Completed: %d/%d", succeeded, len(urls))
	if failed > 0 {
		fmt.Printf(", Failed: %d", failed)
	}
	fmt.Println()

	// List failed URLs if any
	if len(failedURLs) > 0 {
		fmt.Println("\nFailed URLs:")
		for _, url := range failedURLs {
			fmt.Printf("  - %s\n", url)
		}
	}

	return nil
}

// truncateURL shortens a URL for display
func truncateURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return url[:maxLen-3] + "..."
}
