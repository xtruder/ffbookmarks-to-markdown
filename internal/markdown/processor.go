package markdown

import (
	"fmt"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xtruder/ffbookmarks-to-markdown/internal/bookmarks"
	"github.com/xtruder/ffbookmarks-to-markdown/internal/web"
)

// ProcessorOptions contains configuration for markdown processing
type ProcessorOptions struct {
	OutputDir      string
	IgnoredFolders []string
}

type Frontmatter struct {
	CreatedAt   string   `yaml:"created_at"`
	Path        string   `yaml:"path"`
	URL         string   `yaml:"url"`
	ID          string   `yaml:"id"`
	Description string   `yaml:"description,omitempty"`
	Title       string   `yaml:"title"`
	Tags        []string `yaml:"tags,omitempty"`
}

// Update String method to handle tags
func (f Frontmatter) String() string {
	var sb strings.Builder

	writeKV := func(key string, value string) {
		if value != "" {
			sb.WriteString(fmt.Sprintf("%s: %s\n", key, value))
		}
	}

	writeList := func(key string, values []string) {
		if len(values) > 0 {
			sb.WriteString(fmt.Sprintf("%s: [\"%s\"]\n", key, strings.Join(values, ", ")))
		}
	}

	sb.WriteString("---\n")
	if strings.Contains(f.Title, "'") {
		writeKV("title", "\""+f.Title+"\"")
	} else {
		writeKV("title", "'"+f.Title+"'")
	}
	writeKV("url", f.URL)
	writeKV("path", f.Path)
	writeKV("description", f.Description)
	writeKV("created_at", f.CreatedAt)
	writeKV("id", f.ID)
	writeKV("cssclasses", "line3")
	writeList("tags", f.Tags)
	sb.WriteString("---")

	return sb.String()
}

// Processor handles markdown file generation
type Processor struct {
	outputDir         string
	ignoredFolders    []string
	contentService    *web.ContentService
	screenshotService *web.ScreenshotService
	cache             Cache
}

// NewProcessor creates a new markdown processor
func NewProcessor(opts ProcessorOptions, contentService *web.ContentService, screenshotService *web.ScreenshotService, cache Cache) *Processor {
	return &Processor{
		outputDir:         opts.OutputDir,
		ignoredFolders:    opts.IgnoredFolders,
		contentService:    contentService,
		screenshotService: screenshotService,
		cache:             cache,
	}
}

// ProcessBookmarks processes bookmarks recursively
func (p *Processor) ProcessBookmarks(folder bookmarks.Bookmark, currentPath string) error {
	// Create folder path for non-root folders
	if currentPath != "" {
		folderPath := filepath.Join(p.outputDir, currentPath)
		if err := os.MkdirAll(folderPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", folderPath, err)
		}
	}

	for _, bookmark := range folder.Children {
		if bookmark.Type == "bookmark" && !bookmark.Deleted {
			// Check if bookmark exists in cache
			if _, exists := p.cache[bookmark.ID]; !exists {
				if err := p.createBookmarkFile(bookmark, currentPath); err != nil {
					slog.Error("failed to create bookmark file",
						"title", bookmark.Title,
						"error", err)
					continue
				}
				p.cache[bookmark.ID] = bookmark
			}
		} else if bookmark.Type == "folder" {
			// Skip ignored folders
			if p.shouldIgnoreFolder(bookmark.Title) {
				slog.Info("skipping ignored folder", "folder", bookmark.Title)
				continue
			}

			// Process nested folders
			newPath := bookmark.Title
			if currentPath != "" {
				newPath = filepath.Join(currentPath, bookmark.Title)
			}
			if err := p.ProcessBookmarks(bookmark, newPath); err != nil {
				return fmt.Errorf("failed to process folder %s: %w", newPath, err)
			}
		}
	}

	return nil
}

// createBookmarkFile creates a markdown file for a bookmark
func (p *Processor) createBookmarkFile(bookmark bookmarks.Bookmark, currentPath string) error {
	slog.Info("creating markdown file",
		"title", bookmark.Title,
		"url", bookmark.URI,
		"path", currentPath)

	// Get content
	content, err := p.contentService.FetchContent(bookmark.URI)
	if err != nil {
		return fmt.Errorf("failed to fetch content: %w", err)
	}

	// Generate frontmatter
	frontmatter := Frontmatter{
		CreatedAt: time.Unix(bookmark.AddedUnix, 0).Format("2006-01-02"),
		Path:      currentPath,
		URL:       bookmark.URI,
		ID:        bookmark.ID,
		Title:     bookmark.Title,
		Tags:      []string{"bookmark"},
	}

	markdownContent := fmt.Sprintf("%s\n%s\n", frontmatter.String(), content)
	if p.screenshotService != nil {
		// Get screenshot URL
		screenshotURL := p.screenshotService.GetScreenshotURL(bookmark.URI)

		// Create markdown content
		markdownContent = fmt.Sprintf("%s\n![Screenshot](%s)\n%s\n",
			frontmatter.String(),
			screenshotURL,
			content)
	}

	// Write file
	filename := sanitizeFilename(bookmark.Title, bookmark.URI)
	filePath := filepath.Join(p.outputDir, currentPath, filename)
	if err := os.WriteFile(filePath, []byte(markdownContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// shouldIgnoreFolder checks if a folder should be ignored
func (p *Processor) shouldIgnoreFolder(name string) bool {
	for _, ignored := range p.ignoredFolders {
		if strings.TrimSpace(ignored) == name {
			return true
		}
	}
	return false
}

// sanitizeFilename creates a safe filename from bookmark title and URL
func sanitizeFilename(title string, url string) string {
	// Extract domain from URL
	domain := extractDomain(url)

	// Replace invalid characters
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalid {
		title = strings.ReplaceAll(title, char, " ")
	}

	// Clean up spaces
	title = strings.Join(strings.Fields(title), " ")

	// Add domain prefix if not already present
	if domain != "" && !strings.HasPrefix(strings.ToLower(title), strings.ToLower(domain)) {
		return fmt.Sprintf("%s - %s.md", domain, title)
	}
	return title + ".md"
}

// extractDomain extracts domain from URL
func extractDomain(url string) string {
	url = strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
	domain := strings.Split(url, "/")[0]
	domain = strings.TrimPrefix(domain, "www.")
	if colonIndex := strings.Index(domain, ":"); colonIndex != -1 {
		domain = domain[:colonIndex]
	}
	return domain
}

// CreateYearIndexes creates index files for each year
func (p *Processor) CreateYearIndexes(bookmarks iter.Seq[*bookmarks.Bookmark]) error {
	slog.Info("creating year indexes")

	// Collect years from bookmarks
	years := make(map[string]bool)
	for bookmark := range bookmarks {
		year := time.Unix(bookmark.AddedUnix, 0).Format("2006")
		years[year] = true
	}

	// Create index for each year
	for year := range years {
		mdStart := "```dataview"
		mdEnd := "```"
		content := fmt.Sprintf(`---
cssclasses: ["line3"]
---
%s
TABLE path, url, dateformat(created_at, "dd.MM") as "date"
FROM #bookmark
WHERE dateformat(created_at, "yyyy") = "%s"
SORT created_at DESC
%s
`, mdStart, year, mdEnd)

		indexPath := filepath.Join(p.outputDir, fmt.Sprintf("%s.md", year))
		if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write year index %s: %w", year, err)
		}
		slog.Debug("wrote year index", "year", year)
	}

	return nil
}
