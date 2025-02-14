package markdown

import (
	"fmt"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
	"github.com/xtruder/ffbookmarks-to-markdown/internal/bookmarks"
)

// Cache maps bookmark IDs to bookmarks
type Cache map[string]bookmarks.Bookmark

// BuildCache builds the cache from markdown files in the output directory
func BuildCache(outputDir string) (Cache, error) {
	slog.Info("building markdown cache", "dir", outputDir)
	cache := make(Cache)

	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			slog.Warn("failed to access file", "path", path, "error", err)
			return nil
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			slog.Debug("processing cache file", "path", path)
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			var matter Frontmatter
			_, err = frontmatter.Parse(strings.NewReader(string(content)), &matter)
			if err != nil {
				slog.Warn("failed to parse frontmatter", "path", path, "error", err)
				return nil
			}

			if matter.ID != "" {
				cache[matter.ID] = bookmarks.Bookmark{
					ID:        matter.ID,
					Title:     matter.Title,
					URI:       matter.URL,
					AddedUnix: parseCreatedAt(matter.CreatedAt),
					Type:      "bookmark",
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error building cache: %w", err)
	}

	slog.Info("markdown cache built", "entries", len(cache))
	return cache, nil
}

// CollectNewURLs returns URLs that don't exist in the cache
func (c Cache) CollectNewURLs(bookmarks iter.Seq[*bookmarks.Bookmark]) []string {
	var urls []string
	for bookmark := range bookmarks {
		if _, exists := c[bookmark.ID]; !exists {
			urls = append(urls, bookmark.URI)
		}
	}
	return urls
}

// parseCreatedAt parses a date string into Unix timestamp
func parseCreatedAt(date string) int64 {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return 0
	}
	return t.Unix()
}
