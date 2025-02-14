package web

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type GitHubFetcher struct {
	client HTTPClient
}

func NewGitHubFetcher(client HTTPClient) *GitHubFetcher {
	return &GitHubFetcher{client: client}
}

func (f *GitHubFetcher) Fetch(u *url.URL) (string, error) {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid GitHub URL format")
	}

	repo := fmt.Sprintf("%s/%s", parts[0], parts[1])
	baseURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/HEAD/", repo)

	readmeFiles := []string{
		"README.md",
		"README.MD",
		"README.org",
		"Readme.md",
		"readme.md",
	}

	var lastErr error
	for _, filename := range readmeFiles {
		rawURL := baseURL + filename
		resp, err := f.client.Get(rawURL)
		if err != nil {
			lastErr = fmt.Errorf("failed to fetch github readme: %w", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("failed to fetch github readme: %d", resp.StatusCode)
			continue
		}

		content, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read github readme: %w", err)
			continue
		}

		return string(content), nil
	}

	return "", fmt.Errorf("failed to fetch any readme file: %w", lastErr)
}
