// Screenshot handling
// Contains: submitScreenshot, fetchExistingScreenshots, screenshot structs

package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// ScreenshotService handles website screenshots
type ScreenshotService struct {
	client  HTTPClient
	baseURL string
}

// NewScreenshotService creates a new screenshot service
func NewScreenshotService(client HTTPClient, baseURL string) *ScreenshotService {
	return &ScreenshotService{
		client:  client,
		baseURL: baseURL,
	}
}

// ScreenshotRequest represents a batch screenshot request
type ScreenshotRequest struct {
	URLs []string `json:"urls"`
}

// ScreenshotResult represents a single screenshot result
type ScreenshotResult struct {
	ID           int      `json:"id"`
	ProbedAt     string   `json:"probed_at"`
	URL          string   `json:"url"`
	ResponseCode int      `json:"response_code"`
	Title        string   `json:"title"`
	FileName     string   `json:"file_name"`
	Failed       bool     `json:"failed"`
	Technologies []string `json:"technologies"`
}

// ScreenshotGallery represents the gallery response
type ScreenshotGallery struct {
	Results []ScreenshotResult `json:"results"`
}

// GetExistingScreenshots fetches the list of existing screenshots
func (s *ScreenshotService) GetExistingScreenshots() (map[string]bool, error) {
	slog.Info("fetching existing screenshots")

	resp, err := s.client.Get(s.baseURL + "/api/results/gallery?limit=10000")
	if err != nil {
		return nil, fmt.Errorf("error fetching screenshot gallery: %w", err)
	}
	defer resp.Body.Close()

	var gallery ScreenshotGallery
	if err := json.NewDecoder(resp.Body).Decode(&gallery); err != nil {
		return nil, fmt.Errorf("error decoding gallery response: %w", err)
	}

	// Create map of successful screenshots
	screenshots := make(map[string]bool)
	for _, result := range gallery.Results {
		if !result.Failed {
			screenshots[result.URL] = true
		}
	}

	slog.Info("fetched existing screenshots", "count", len(screenshots))
	return screenshots, nil
}

// SubmitScreenshots submits URLs for screenshots
func (s *ScreenshotService) SubmitScreenshots(urls []string) error {
	slog.Info("submitting screenshot request", "count", len(urls))

	request := ScreenshotRequest{
		URLs: urls,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("error marshaling request: %w", err)
	}

	resp, err := http.Post(s.baseURL+"/api/submit", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error submitting screenshot request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("screenshot submission failed with status: %d", resp.StatusCode)
	}

	slog.Debug("screenshot request submitted successfully")
	return nil
}

// GetScreenshotURL returns the URL for a screenshot
func (s *ScreenshotService) GetScreenshotURL(url string) string {
	screenshotPath := strings.NewReplacer(
		"/", "-",
		":", "-",
		"?", "-",
		"=", "-",
		"&", "-",
		"_", "-",
		"#", "-",
	).Replace(url)
	return fmt.Sprintf("%s/screenshots/%s.jpeg", s.baseURL, screenshotPath)
}
