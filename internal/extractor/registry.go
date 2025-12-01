package extractor

import "net/url"

// registry holds all registered extractors
var registry []Extractor

// Register adds an extractor to the registry
func Register(e Extractor) {
	registry = append(registry, e)
}

// Match finds the first extractor that can handle the URL
// It parses the URL once and passes the parsed URL to each extractor
func Match(rawURL string) Extractor {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	for _, e := range registry {
		if e.Match(u) {
			return e
		}
	}
	return nil
}

// List returns all registered extractors
func List() []Extractor {
	return registry
}

func init() {
	// Register extractors in order of priority
	Register(&TwitterExtractor{})
	// Register(&DirectExtractor{})  // TODO: add direct MP4 support
}
