package web

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type MarkdownFetcher struct {
	client  HTTPClient
	baseURL string
	cleaner ContentCleaner
}

func NewMarkdownFetcher(client HTTPClient, baseURL string, cleaner ContentCleaner) *MarkdownFetcher {
	return &MarkdownFetcher{
		client:  client,
		baseURL: baseURL,
		cleaner: cleaner,
	}
}

func (f *MarkdownFetcher) Fetch(u *url.URL) (string, error) {
	content, err := f.fetchRaw(u)
	if err != nil {
		return "", err
	}

	return f.clean(content, u)
}

// fetchRaw gets the raw content from the markdown service
func (f *MarkdownFetcher) fetchRaw(u *url.URL) (string, error) {
	// Fetch content
	encodedURL := fmt.Sprintf("%s/?url=%s&enableDetailedResponse=true",
		f.baseURL,
		url.QueryEscape(u.String()))

	resp, err := f.client.Get(encodedURL)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status: %d", resp.StatusCode)
	}

	return string(body), nil
}

// clean processes the markdown content
func (f *MarkdownFetcher) clean(content string, u *url.URL) (string, error) {
	// Fix relative links
	baseURL := fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
	content = fixMarkdownLinks(content, baseURL)

	if f.cleaner != nil {
		// Clean with LLM if available
		cleaned, err := f.cleaner.CleanMarkdown(content)
		if err != nil {
			slog.Warn("LLM cleaning failed, using original content", "error", err)
		} else {
			content = cleaned
		}
	}

	// Remove empty lines
	lines := strings.Split(content, "\n")
	var cleanLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n"), nil
}

// fixMarkdownLinks fixes relative links in markdown content
func fixMarkdownLinks(content string, baseURL string) string {
	// Match both markdown links and images, capturing the ! separately
	re := regexp.MustCompile(`(!)?\[(.*?)\]\((.*?)\)`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract parts
		parts := re.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}

		isImage := parts[1] == "!" // The ! prefix if present
		text := parts[2]           // Link text
		link := parts[3]           // The URL

		// Skip data URLs and absolute URLs
		if strings.HasPrefix(link, "data:") ||
			strings.HasPrefix(link, "http://") ||
			strings.HasPrefix(link, "https://") {
			return match
		}

		// Reconstruct the link with proper image prefix
		prefix := ""
		if isImage {
			prefix = "!"
		}

		baseURL = strings.TrimSuffix(baseURL, "/")

		if !strings.HasPrefix(link, "/") {
			link = "/" + link
		}

		return fmt.Sprintf("%s[%s](%s%s)", prefix, text, baseURL, link)
	})
}
