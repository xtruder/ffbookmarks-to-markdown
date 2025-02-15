// Firefox bookmark fetching and parsing
// Contains: getFirefoxBookmarks, findFolder, Bookmark struct

package firefox

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
)

// FirefoxFetcher handles fetching bookmarks from Firefox
type FirefoxFetcher struct {
	FFSyncCmd string
}

// NewFirefoxFetcher creates a new Firefox bookmarks fetcher
func NewFirefoxFetcher() *FirefoxFetcher {
	return &FirefoxFetcher{FFSyncCmd: "ffsclient"}
}

// GetBookmarks fetches all bookmarks from Firefox
func (f *FirefoxFetcher) GetBookmarks() (*BookmarksRoot, error) {
	cmd := exec.Command(f.FFSyncCmd, "bookmarks", "list", "--format=json")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			slog.Error("failed to execute ffsclient", "stderr", string(exitErr.Stderr))
			return nil, err
		}

		return nil, err
	}

	var root BookmarksRoot
	if err := json.Unmarshal(output, &root); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &root, nil
}
