// Web content fetching (HTML and GitHub)
// Contains: fetchGenericMarkdown, fetchGitHubReadme, getYouTubeEmbed

package web

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/xtruder/ffbookmarks-to-markdown/internal/x"
)

type ContentCleaner interface {
	CleanMarkdown(content string) (string, error)
}

// FetchOptions contains configuration for content fetching
type FetchOptions struct {
	BaseURL        string
	ScreenshotURL  string
	Cache          x.Cache
	ContentCleaner ContentCleaner
}

// ContentService handles web content fetching
type ContentService struct {
	youtube  ContentFetcher
	github   ContentFetcher
	markdown ContentFetcher
	cache    x.Cache
}

// NewContentService creates a new content fetching service
func NewContentService(client HTTPClient, opts FetchOptions) *ContentService {
	return &ContentService{
		youtube:  NewYouTubeFetcher(),
		github:   NewGitHubFetcher(client),
		markdown: NewMarkdownFetcher(client, opts.BaseURL, opts.ContentCleaner),
		cache:    opts.Cache,
	}
}

// FetchContent fetches content from a URL based on its type
func (s *ContentService) FetchContent(u string) (string, error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Try cache first
	if s.cache != nil {
		if content, ok := s.cache.Get(getURLKey(u)); ok {
			slog.Debug("using cached content", "url", u)
			return content, nil
		}
	}

	// Fetch content based on URL type
	var content string
	switch parsedURL.Host {
	case "youtube.com", "www.youtube.com", "youtu.be":
		slog.Info("generating YouTube embed", "url", u)
		content, err = s.youtube.Fetch(parsedURL)
	case "github.com", "www.github.com":
		slog.Info("fetching GitHub README", "url", u)
		content, err = s.github.Fetch(parsedURL)
	default:
		slog.Info("fetching generic markdown", "url", u)
		content, err = s.markdown.Fetch(parsedURL)
	}

	if err != nil {
		return "", err
	}

	// Cache the content
	if s.cache != nil {
		if err := s.cache.Set(getURLKey(u), content); err != nil {
			slog.Warn("failed to cache content", "error", err)
		}
	}

	return content, nil
}

func getURLKey(u string) string {
	hash := sha256.Sum256([]byte(u))
	return base64.URLEncoding.EncodeToString(hash[:])
}
