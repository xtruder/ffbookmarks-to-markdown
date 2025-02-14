package web

import (
	"net/http"
	"net/url"
)

// ContentFetcher defines the interface for fetching content
type ContentFetcher interface {
	Fetch(url *url.URL) (string, error)
}

// HTTPClient defines the interface for making HTTP requests
type HTTPClient interface {
	Get(url string) (*http.Response, error)
}
