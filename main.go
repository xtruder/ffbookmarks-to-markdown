package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// Bookmark represents a Firefox bookmark
type Bookmark struct {
	Added     string     `json:"added"`
	AddedUnix int64      `json:"added_unix"`
	Deleted   bool       `json:"deleted"`
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Type      string     `json:"type"`
	URI       string     `json:"uri,omitempty"`
	Children  []Bookmark `json:"children,omitempty"`
}

// FrontMatter represents the YAML frontmatter in markdown files
type FrontMatter struct {
	CreatedAt   string `yaml:"created_at"`
	Path        string `yaml:"path"`
	URL         string `yaml:"url"`
	ID          string `yaml:"id"`
	Description string `yaml:"description,omitempty"`
	Title       string `yaml:"title"`
}

// Update String method to skip empty fields
func (f FrontMatter) String() string {
	var sb strings.Builder

	writeKV := func(key string, value string) {
		if value != "" {
			sb.WriteString(fmt.Sprintf("%s: %s\n", key, value))
		}
	}

	sb.WriteString("---\n")
	writeKV("title", f.Title)
	writeKV("url", f.URL)
	writeKV("path", f.Path)
	writeKV("description", f.Description)
	writeKV("created_at", f.CreatedAt)
	writeKV("id", f.ID)
	sb.WriteString("---")

	return sb.String()
}

// Add new struct for the root JSON structure
type BookmarksRoot struct {
	Bookmarks struct {
		Menu    Bookmark `json:"menu"`
		Mobile  Bookmark `json:"mobile"`
		Toolbar Bookmark `json:"toolbar"`
		Unfiled Bookmark `json:"unfiled"`
	} `json:"bookmarks"`
	Missing      []string `json:"missing"`
	Unreferenced []string `json:"unreferenced"`
}

// Add new struct for screenshot submission
type ScreenshotRequest struct {
	URLs []string `json:"urls"`
}

// Add structs for screenshot results
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

type ScreenshotGallery struct {
	Results []ScreenshotResult `json:"results"`
}

// Add OpenAI client configuration
type LLMConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

// Add retryable client configuration
var retryClient *retryablehttp.Client

type Logger struct{}

func (l *Logger) Printf(format string, v ...any) {
	slog.Info(fmt.Sprintf(format, v...))
}

// Add function to create retryable client
func newRetryableClient() *retryablehttp.Client {
	client := retryablehttp.NewClient()
	client.RetryMax = 3
	client.RetryWaitMin = 10 * time.Second
	client.RetryWaitMax = 60 * time.Second
	client.Logger = &Logger{}

	// Add custom retry policy for rate limits
	client.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		// Use default retry policy first
		retry, err := retryablehttp.DefaultRetryPolicy(ctx, resp, err)
		if retry || err != nil {
			return retry, err
		}

		// Also retry on 429 Too Many Requests
		if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
			return true, nil
		}

		return false, nil
	}

	return client
}

// Add function to create OpenAI client
func newLLMClient(config LLMConfig) *openai.Client {
	return openai.NewClient(
		option.WithAPIKey(config.APIKey),
		option.WithBaseURL(config.BaseURL),
		option.WithHTTPClient(retryClient.StandardClient()),
	)
}

// Add LLM cache functions
func getLLMCacheKey(prompt string, content string, model string) string {
	data := fmt.Sprintf("%s\n---\n%s\n---\n%s", model, prompt, content)
	hash := sha256.Sum256([]byte(data))
	return "llm_" + base64.URLEncoding.EncodeToString(hash[:])
}

// Update cleanMarkdownWithLLM to use cache
func cleanMarkdownWithLLM(client *openai.Client, content string, model string) (string, error) {
	prompt := `Clean and enhance this markdown content following these strict rules:

CONTENT RULES:
1. Keep only information directly related to the main topic
2. Remove any promotional, advertising, or unrelated content
3. Remove navigation elements, footers, and sidebars
4. Keep code blocks and technical content if relevant
5. Preserve important quotes and key points

FORMATTING RULES:
1. Use proper markdown heading hierarchy (h1 -> h2 -> h3)
2. Ensure consistent spacing between sections
3. Fix or remove malformed markdown syntax
4. Convert HTML to markdown where possible
5. Remove redundant line breaks and spaces

IMAGE AND LINK RULES:
1. Keep only the most relevant and informative images
2. Remove decorative or redundant images
3. Remove broken or relative links
4. Remove duplicate links pointing to the same content
5. Keep essential reference links

CLEANUP RULES:
1. Remove empty sections
2. Remove non-English content unless it's code
3. Fix list formatting and indentation
4. Remove HTML comments and metadata
5. Remove social media embeds unless they're the main content

Content to clean:
` + content

	// Check cache first
	cacheKey := getLLMCacheKey(prompt, content, model)
	if cached, ok := getCachedContent(cacheKey); ok {
		slog.Info("using cached LLM response")
		return cached, nil
	}

	slog.Info("cleaning markdown with LLM", "model", model)

	chatCompletion, err := client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a markdown content curator. Your task is to clean and restructure markdown content while preserving its essential information and improving its readability. Be thorough and strict in following the cleaning rules."),
			openai.UserMessage(prompt),
		}),
		Model:       openai.F(model),
		Temperature: openai.F(0.1), // Add low temperature for more consistent results
	})
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %v", err)
	}

	cleanContent := strings.TrimSpace(chatCompletion.Choices[0].Message.Content)
	cleanContent = strings.TrimPrefix(cleanContent, "```markdown\n")
	cleanContent = strings.TrimPrefix(cleanContent, "```\n")
	cleanContent = strings.TrimSuffix(cleanContent, "\n```")

	// Cache the cleaned content
	if err := setCachedContent(cacheKey, cleanContent); err != nil {
		slog.Warn("failed to cache LLM response", "error", err)
	}

	return cleanContent, nil
}

// Add this function to check if a folder should be ignored
func shouldIgnoreFolder(folderName string, ignoredFolders []string) bool {
	for _, ignored := range ignoredFolders {
		if strings.TrimSpace(ignored) == folderName {
			return true
		}
	}
	return false
}

// Add cache configuration
var (
	cacheDir string
)

// Add cache helper functions
func initCache() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	cacheDir = filepath.Join(homeDir, ".cache", "ffbookmarks-to-markdown")
	return os.MkdirAll(cacheDir, 0755)
}

func getCacheKey(url string) string {
	hash := sha256.Sum256([]byte(url))
	return base64.URLEncoding.EncodeToString(hash[:])
}

func getCachedContent(key string) (string, bool) {
	path := filepath.Join(cacheDir, key)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(content), true
}

func setCachedContent(key string, content string) error {
	path := filepath.Join(cacheDir, key)
	return os.WriteFile(path, []byte(content), 0644)
}

// Add function to clean existing link structure
func cleanLinkStructure(outputDir string) error {
	slog.Info("cleaning up existing symlink structure")
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("failed to read output directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), "_") {
			path := filepath.Join(outputDir, entry.Name())
			slog.Info("removing directory", "path", path)
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("failed to remove directory %s: %w", path, err)
			}
		}
	}
	return nil
}

// Add helper function to get markdown filename
func getMarkdownFilename(bookmark Bookmark, outputDir string) string {
	// Create base filename
	filename := sanitizeFilename(bookmark.Title, bookmark.URI)
	datePrefix := time.Unix(bookmark.AddedUnix, 0).Format("06-01-02")
	year := time.Unix(bookmark.AddedUnix, 0).Format("2006")
	datedFilename := fmt.Sprintf("%s %s", datePrefix, filename)
	return filepath.Join(outputDir, "_years", year, datedFilename)
}

// Update createMarkdownFile to use helper
func createMarkdownFile(bookmark Bookmark, outputDir string, currentPath string, screenshotAPI string, llmClient *openai.Client, model string) {
	slog.Info("creating markdown file",
		"title", bookmark.Title,
		"url", bookmark.URI,
		"path", currentPath)

	// Get file path and ensure directory exists
	filePath := getMarkdownFilename(bookmark, outputDir)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		slog.Error("failed to create directory", "error", err)
		return
	}

	// Process title
	cleanTitle, subtitle := splitTitle(bookmark.Title)
	fullTitle := cleanTitle
	if subtitle != "" {
		fullTitle = fmt.Sprintf("%s - %s", cleanTitle, subtitle)
	}

	// Fetch content
	var content string
	if mdContent, err := fetchMarkdownContent(bookmark.URI, llmClient, model); err == nil {
		content = mdContent
	} else {
		slog.Error("Error fetching content", "url", bookmark.URI, "error", err)
		content = bookmark.Title // Fallback to title if fetch fails
	}

	// Parse URL to determine if it's YouTube
	parsedURL, err := url.Parse(bookmark.URI)
	isYouTube := err == nil && (parsedURL.Host == "youtube.com" || parsedURL.Host == "www.youtube.com" || parsedURL.Host == "youtu.be")

	// Generate markdown content based on URL type
	var markdownContent string
	if isYouTube {
		markdownContent = fmt.Sprintf("%s\n# %s\n%s\n",
			FrontMatter{
				CreatedAt: time.Unix(bookmark.AddedUnix, 0).Format("2006-01-02"),
				Path:      currentPath,
				URL:       bookmark.URI,
				ID:        bookmark.ID,
				Title:     fullTitle,
			}.String(),
			fullTitle,
			content)
	} else {
		// Generate screenshot URL for non-YouTube content
		screenshotURL := fmt.Sprintf("%s/screenshots/%s.jpeg", screenshotAPI, urlToScreenshotPath(bookmark.URI))
		markdownContent = fmt.Sprintf("%s\n![Screenshot](%s)\n%s\n",
			FrontMatter{
				CreatedAt: time.Unix(bookmark.AddedUnix, 0).Format("2006-01-02"),
				Path:      currentPath,
				URL:       bookmark.URI,
				ID:        bookmark.ID,
				Title:     fullTitle,
			}.String(),
			screenshotURL,
			content)
	}

	// Write actual file
	if err := os.WriteFile(filePath, []byte(markdownContent), 0644); err != nil {
		slog.Error("failed to write markdown file",
			"path", filePath,
			"error", err)
		return
	}
	slog.Debug("wrote markdown file", "path", filePath)
}

// Update createLinkTree to use helper
func createLinkTree(outputDir string, folder Bookmark, currentPath string, ignoredFolders []string) error {
	slog.Info("creating link tree from bookmarks", "folder", folder.Title)

	var createLinks func(b Bookmark, path string) error
	createLinks = func(b Bookmark, path string) error {
		// Create folder path
		if path != "" {
			folderPath := filepath.Join(outputDir, path)
			// Create directory if it doesn't exist
			if err := os.MkdirAll(folderPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", folderPath, err)
			}
			// Create folder index
			createFolderIndex(b, outputDir, path)
		}

		for _, bookmark := range b.Children {
			if bookmark.Type == "bookmark" && !bookmark.Deleted {
				sourcePath := getMarkdownFilename(bookmark, outputDir)
				symlinkPath := filepath.Join(outputDir, path, sanitizeFilename(bookmark.Title, bookmark.URI))

				if err := createSymlink(sourcePath, symlinkPath); err != nil {
					slog.Error("failed to create symlink",
						"source", sourcePath,
						"target", symlinkPath,
						"error", err)
				}
			} else if bookmark.Type == "folder" {
				// Skip ignored folders
				if shouldIgnoreFolder(bookmark.Title, ignoredFolders) {
					slog.Info("skipping ignored folder", "folder", bookmark.Title)
					continue
				}

				// Process nested folders
				newPath := bookmark.Title
				if path != "" {
					newPath = filepath.Join(path, bookmark.Title)
				}
				if err := createLinks(bookmark, newPath); err != nil {
					return err
				}
			}
		}
		return nil
	}

	return createLinks(folder, currentPath)
}

func main() {
	baseFolder := flag.String("folder", "", "Base folder name to sync from Firefox bookmarks")
	outputDir := flag.String("output", "bookmarks", "Output directory for markdown files")
	listFolders := flag.Bool("list", false, "List all available folders")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	ignoreFolders := flag.String("ignore", "", "Comma-separated list of folder names to ignore")
	screenshotAPI := flag.String("screenshot-api", "https://gowitness.cloud.x-truder.net", "Screenshot API base URL")
	llmAPIKey := flag.String("llm-key", "", "API key for LLM service")
	llmBaseURL := flag.String("llm-url", "https://generativelanguage.googleapis.com/v1beta/openai/", "Base URL for LLM service")
	llmModel := flag.String("llm-model", "gemini-2.0-flash", "Model to use for LLM service")
	recreateLinks := flag.Bool("recreate-symlinks", false, "Recreate all symlinks while processing bookmarks")
	flag.Parse()

	if *llmAPIKey == "" {
		*llmAPIKey = os.Getenv("GEMINI_API_KEY")
	}

	// Initialize logger with appropriate level based on verbose flag
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Initialize retryable client
	retryClient = newRetryableClient()

	// Initialize cache
	if err := initCache(); err != nil {
		slog.Error("failed to initialize cache", "error", err)
		os.Exit(1)
	}

	// Parse ignored folders
	var ignoredFolders []string
	if *ignoreFolders != "" {
		ignoredFolders = strings.Split(*ignoreFolders, ",")
	}

	// Create LLM client if API key is provided
	var llmClient *openai.Client
	if *llmAPIKey != "" {
		llmClient = newLLMClient(LLMConfig{
			APIKey:  *llmAPIKey,
			BaseURL: *llmBaseURL,
			Model:   *llmModel,
		})
	}

	slog.Info("starting bookmark sync",
		"folder", *baseFolder,
		"output", *outputDir,
		"verbose", *verbose,
		"ignored_folders", ignoredFolders)

	// Get Firefox bookmarks using ffsclient
	bookmarks, err := getFirefoxBookmarks()
	if err != nil {
		fmt.Printf("Error getting Firefox bookmarks: %v\n", err)
		os.Exit(1)
	}

	// If list flag is set, display folders and exit
	if *listFolders {
		fmt.Println("Available folders:")
		displayFolders(bookmarks, 0)
		os.Exit(0)
	}

	if *baseFolder == "" {
		fmt.Println("Please specify a base folder name using -folder flag")
		os.Exit(1)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Build the bookmark cache
	cache, err := buildBookmarkCache(*outputDir)
	if err != nil {
		fmt.Printf("Error building bookmark cache: %v\n", err)
		os.Exit(1)
	}

	// Find the specified folder
	targetFolder := findFolder(bookmarks, *baseFolder)
	if targetFolder == nil {
		fmt.Printf("Folder '%s' not found in bookmarks\n", *baseFolder)
		os.Exit(1)
	}

	// Clean up existing structure if recreating
	if *recreateLinks {
		if err := cleanLinkStructure(*outputDir); err != nil {
			slog.Error("failed to clean link structure", "error", err)
			os.Exit(1)
		}
	}

	// Fetch existing screenshots
	screenshotCache, err := fetchExistingScreenshots(*screenshotAPI)
	if err != nil {
		slog.Error("failed to fetch screenshot cache", "error", err)
		os.Exit(1)
	}

	// Collect URLs that need screenshots
	newURLs := collectNewURLs(*targetFolder, cache)

	// Filter out URLs that already have screenshots
	var urlsToScreenshot []string
	for _, u := range newURLs {
		if !screenshotCache[u] {
			urlsToScreenshot = append(urlsToScreenshot, u)
		}
	}

	if len(urlsToScreenshot) > 0 {
		slog.Info("submitting batch screenshot request",
			"total", len(newURLs),
			"new", len(urlsToScreenshot),
			"cached", len(newURLs)-len(urlsToScreenshot))
		if err := submitScreenshot(*screenshotAPI, urlsToScreenshot); err != nil {
			slog.Error("failed to submit batch screenshots", "error", err)
		}
	} else {
		slog.Info("no new screenshots needed",
			"total", len(newURLs),
			"cached", len(newURLs))
	}

	// Process bookmarks and create link tree
	processBookmarksWithPath(*targetFolder, *outputDir, "", cache, ignoredFolders, *screenshotAPI, llmClient, *llmModel)
	if err := createLinkTree(*outputDir, *targetFolder, "", ignoredFolders); err != nil {
		slog.Error("failed to create link tree", "error", err)
		os.Exit(1)
	}
}

func getFirefoxBookmarks() ([]Bookmark, error) {
	cmd := exec.Command("ffsclient", "bookmarks", "list", "--format=json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute ffsclient: %v", err)
	}

	var root BookmarksRoot
	if err := json.Unmarshal(output, &root); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	// Convert the structure to a slice of bookmarks
	bookmarks := []Bookmark{
		root.Bookmarks.Menu,
		root.Bookmarks.Mobile,
		root.Bookmarks.Toolbar,
		root.Bookmarks.Unfiled,
	}

	return bookmarks, nil
}

func findFolder(bookmarks []Bookmark, name string) *Bookmark {
	for _, b := range bookmarks {
		if b.Type == "folder" {
			if b.Title == name {
				return &b
			}
			if len(b.Children) > 0 {
				if found := findFolder(b.Children, name); found != nil {
					return found
				}
			}
		}
	}
	return nil
}

// Add helper function to create symlink
func createSymlink(source, target string) error {
	// Ensure target directory exists
	targetDir := filepath.Dir(target)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %v", err)
	}

	// Create relative path for the symlink
	relPath, err := filepath.Rel(targetDir, source)
	if err != nil {
		return fmt.Errorf("failed to create relative path: %v", err)
	}

	// Remove existing symlink if it exists
	if _, err := os.Lstat(target); err == nil {
		os.Remove(target)
	}

	// Create symlink
	if err := os.Symlink(relPath, target); err != nil {
		return fmt.Errorf("failed to create symlink: %v", err)
	}

	return nil
}

// Update createFolderIndex to use String method
func createFolderIndex(folder Bookmark, outputDir string, currentPath string) {
	slog.Debug("creating folder index",
		"folder", folder.Title,
		"id", folder.ID,
		"path", currentPath)

	// Create frontmatter
	frontmatter := FrontMatter{
		CreatedAt: time.Unix(folder.AddedUnix, 0).Format("2006-01-02"),
		Path:      currentPath,
		ID:        folder.ID,
		Title:     folder.Title,
	}

	// Create markdown content
	content := fmt.Sprintf("%s\n\n# %s\n",
		frontmatter.String(),
		folder.Title)

	// Write index file
	indexPath := filepath.Join(outputDir, currentPath, "index.md")
	if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
		slog.Error("failed to write folder index",
			"path", indexPath,
			"error", err)
	} else {
		slog.Debug("wrote folder index", "path", indexPath)
	}
}

func urlToScreenshotPath(u string) string {
	return strings.NewReplacer(
		"/", "-",
		":", "-",
		"?", "-",
		"=", "-",
		"&", "-",
		"_", "-",
	).Replace(u)
}

// Add function to submit screenshot request
func submitScreenshot(apiBaseURL string, urls []string) error {
	slog.Info("submitting screenshot request", "count", len(urls))

	request := ScreenshotRequest{
		URLs: urls,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("error marshaling request: %v", err)
	}

	resp, err := http.Post(apiBaseURL+"/api/submit", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error submitting screenshot request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("screenshot submission failed with status: %d", resp.StatusCode)
	}

	slog.Debug("screenshot request submitted successfully")
	return nil
}

// Update processBookmarksWithPath to focus only on file creation
func processBookmarksWithPath(folder Bookmark, outputDir string, currentPath string, cache map[string]FrontMatter, ignoredFolders []string, screenshotAPI string, llmClient *openai.Client, model string) {
	for _, bookmark := range folder.Children {
		if bookmark.Type == "bookmark" && !bookmark.Deleted {
			// Check if bookmark exists in cache using ID
			if _, exists := cache[bookmark.ID]; !exists {
				createMarkdownFile(bookmark, outputDir, currentPath, screenshotAPI, llmClient, model)
				// Add to cache after creating the file
				cache[bookmark.ID] = FrontMatter{
					CreatedAt: time.Unix(bookmark.AddedUnix, 0).Format("2006-01-02"),
					Path:      currentPath,
					URL:       bookmark.URI,
					ID:        bookmark.ID,
					Title:     bookmark.Title,
				}
			}
		} else if bookmark.Type == "folder" {
			// Skip ignored folders
			if shouldIgnoreFolder(bookmark.Title, ignoredFolders) {
				slog.Info("skipping ignored folder", "folder", bookmark.Title)
				continue
			}

			// Process nested folders with updated path
			newPath := bookmark.Title
			if currentPath != "" {
				newPath = filepath.Join(currentPath, bookmark.Title)
			}
			processBookmarksWithPath(bookmark, outputDir, newPath, cache, ignoredFolders, screenshotAPI, llmClient, model)
		}
	}
}

// Update splitTitle to be simpler
func splitTitle(title string) (string, string) {
	title = strings.TrimSpace(title)
	separators := []string{":", "|", " - "}

	// Find first separator
	firstSep := ""
	firstIndex := -1
	for _, sep := range separators {
		if idx := strings.Index(title, sep); idx != -1 {
			if firstIndex == -1 || idx < firstIndex {
				firstIndex = idx
				firstSep = sep
			}
		}
	}

	// If no separator found, return whole title
	if firstIndex == -1 {
		return title, ""
	}

	// Split by first separator
	mainTitle := strings.TrimSpace(title[:firstIndex])
	subtitle := strings.TrimSpace(title[firstIndex+len(firstSep):])

	// Remove other separators from subtitle
	for _, sep := range separators {
		subtitle = strings.ReplaceAll(subtitle, sep, " ")
	}

	// Clean up spaces
	subtitle = strings.Join(strings.Fields(subtitle), " ")

	return mainTitle, subtitle
}

// Update fetchGitHubReadme to use cache
func fetchGitHubReadme(u *url.URL) (string, error) {
	// Use repo URL as cache key
	repoURL := fmt.Sprintf("https://github.com%s", u.Path)
	cacheKey := getCacheKey(repoURL)

	if content, ok := getCachedContent(cacheKey); ok {
		slog.Debug("using cached GitHub readme", "url", repoURL)
		return content, nil
	}

	// Get user and repo from path
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid GitHub URL format")
	}

	repo := fmt.Sprintf("%s/%s", parts[0], parts[1])
	baseURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/HEAD/", repo)

	// Try different readme filenames
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
		resp, err := retryablehttp.Get(rawURL)
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

		contentRaw, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read github readme: %w", err)
			continue
		}

		content := string(contentRaw)

		// Fix relative links using the GitHub blob URL as base
		blobBaseURL := fmt.Sprintf("https://github.com/%s/blob/HEAD/", repo)
		content = fixMarkdownLinks(content, blobBaseURL)

		// Cache the successful response
		if err := setCachedContent(cacheKey, content); err != nil {
			slog.Warn("failed to cache GitHub readme", "error", err)
		}

		return content, nil
	}

	return "", fmt.Errorf("failed to fetch any readme file: %w", lastErr)
}

// Update getYouTubeEmbed to accept parsed URL
func getYouTubeEmbed(u *url.URL) (string, error) {
	var videoID string
	switch u.Host {
	case "youtube.com", "www.youtube.com":
		if u.Path == "/watch" {
			if v := u.Query().Get("v"); v != "" {
				videoID = v
			}
		}
	case "youtu.be":
		videoID = strings.TrimPrefix(u.Path, "/")
	}

	if videoID == "" {
		return "", fmt.Errorf("could not extract video ID from URL")
	}

	return fmt.Sprintf(`<iframe width="560" height="315" src="https://www.youtube.com/embed/%s" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>`, videoID), nil
}

// Update fetchMarkdownContent to use switch
func fetchMarkdownContent(u string, llmClient *openai.Client, model string) (string, error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %v", err)
	}

	// Determine URL type and handle accordingly
	switch parsedURL.Host {
	case "youtube.com", "www.youtube.com", "youtu.be":
		slog.Info("generating YouTube embed", "url", u)
		return getYouTubeEmbed(parsedURL)

	case "github.com", "www.github.com":
		slog.Info("fetching GitHub README", "url", u)
		return fetchGitHubReadme(parsedURL)

	default:
		slog.Info("fetching markdown content", "url", u)
		return fetchGenericMarkdown(u, llmClient, model)
	}
}

// Fix fixMarkdownLinks to properly handle image links
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

// Update fetchGenericMarkdown to use cache
func fetchGenericMarkdown(u string, llmClient *openai.Client, model string) (string, error) {
	cacheKey := getCacheKey(u)

	if content, ok := getCachedContent(cacheKey); ok {
		slog.Info("using cached markdown content", "url", u)

		// Fix relative links using the original URL as base
		parsedURL, err := url.Parse(u)
		if err != nil {
			slog.Warn("failed to parse URL", "error", err)
		} else {
			baseURL := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, parsedURL.Path)
			content = fixMarkdownLinks(content, baseURL)
		}

		// Still run LLM cleaning on cached content if client is available
		if llmClient != nil {
			content, err := cleanMarkdownWithLLM(llmClient, content, model)
			if err != nil {
				slog.Warn("LLM cleaning failed for cached content", "error", err)
				return content, nil
			}
			return content, nil
		}
		return content, nil
	}

	encodedURL := fmt.Sprintf("https://md.dhr.wtf/?url=%s&enableDetailedResponse=true",
		url.QueryEscape(u))

	resp, err := retryablehttp.Get(encodedURL)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status: %d", resp.StatusCode)
	}

	content := string(body)

	// Cache the raw response before LLM cleaning
	if err := setCachedContent(cacheKey, content); err != nil {
		slog.Warn("failed to cache markdown content", "error", err)
	}

	// Fix relative links using the original URL as base
	parsedURL, err := url.Parse(u)
	if err != nil {
		slog.Warn("failed to parse URL", "error", err)
	} else {
		baseURL := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, parsedURL.Path)
		content = fixMarkdownLinks(content, baseURL)
	}

	// Clean content with LLM if client is available
	if llmClient != nil {
		content, err = cleanMarkdownWithLLM(llmClient, content, model)
		if err != nil {
			slog.Warn("LLM cleaning failed, using original content", "error", err)
			return content, nil
		}
	}

	return strings.TrimSpace(content), nil
}

func sanitizeFilename(filename string, url string) string {
	// Extract domain from URL
	domain := extractDomain(url)

	// Replace invalid characters with spaces
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	title := filename
	for _, char := range invalid {
		title = strings.ReplaceAll(title, char, " ")
	}

	// Truncate long titles
	const maxTitleLength = 50 // Maximum length for the title part
	if len(title) > maxTitleLength {
		// Try to find a word boundary to truncate at
		truncated := title[:maxTitleLength]
		lastSpace := strings.LastIndex(truncated, " ")
		if lastSpace > maxTitleLength/2 {
			// If we found a space in the latter half, truncate there
			truncated = truncated[:lastSpace]
		}
		title = truncated
	}

	// Remove consecutive spaces
	for strings.Contains(title, "  ") {
		title = strings.ReplaceAll(title, "  ", " ")
	}

	// Trim spaces from beginning and end
	title = strings.TrimSpace(title)

	// Check if title already starts with domain
	if domain != "" && !strings.HasPrefix(strings.ToLower(title), strings.ToLower(domain)) {
		return fmt.Sprintf("%s - %s.md", domain, title)
	}
	return title + ".md"
}

func extractDomain(url string) string {
	// Remove protocol
	url = strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")

	// Get domain part
	domain := strings.Split(url, "/")[0]

	// Remove www. prefix if present
	domain = strings.TrimPrefix(domain, "www.")

	// Remove any port numbers
	if colonIndex := strings.Index(domain, ":"); colonIndex != -1 {
		domain = domain[:colonIndex]
	}

	return domain
}

// displayFolders prints the folder structure with proper indentation
func displayFolders(bookmarks []Bookmark, level int) {
	indent := strings.Repeat("  ", level)
	for _, b := range bookmarks {
		if b.Type == "folder" {
			fmt.Printf("%s- %s\n", indent, b.Title)
			if len(b.Children) > 0 {
				displayFolders(b.Children, level+1)
			}
		}
	}
}

// Update buildBookmarkCache to use ID as key
func buildBookmarkCache(outputDir string) (map[string]FrontMatter, error) {
	slog.Info("building bookmark cache", "dir", outputDir)

	cache := make(map[string]FrontMatter)

	// Look in _years directory for actual files
	yearsPath := filepath.Join(outputDir, "_years")
	if _, err := os.Stat(yearsPath); os.IsNotExist(err) {
		slog.Debug("_years directory does not exist yet", "path", yearsPath)
		return cache, nil
	}

	err := filepath.Walk(yearsPath, func(path string, info os.FileInfo, err error) error {
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

			var matter FrontMatter
			_, err = frontmatter.Parse(strings.NewReader(string(content)), &matter)
			if err != nil {
				slog.Warn("failed to parse frontmatter", "path", path, "error", err)
				return nil
			}

			if matter.ID != "" {
				cache[matter.ID] = matter
			}
		}
		return nil
	})

	if err != nil {
		slog.Error("failed to build cache", "error", err)
		return nil, fmt.Errorf("error building cache: %v", err)
	}

	slog.Info("bookmark cache built", "entries", len(cache))
	return cache, nil
}

// Update collectNewURLs to use ID for cache lookup
func collectNewURLs(folder Bookmark, cache map[string]FrontMatter) []string {
	var urls []string

	var collect func(b Bookmark)
	collect = func(b Bookmark) {
		if b.Type == "bookmark" && !b.Deleted {
			if _, exists := cache[b.ID]; !exists {
				urls = append(urls, b.URI)
			}
		}
		for _, child := range b.Children {
			collect(child)
		}
	}

	collect(folder)
	return urls
}

// Add function to fetch existing screenshots
func fetchExistingScreenshots(apiBaseURL string) (map[string]bool, error) {
	slog.Info("fetching existing screenshots")

	resp, err := http.Get(apiBaseURL + "/api/results/gallery?limit=10000")
	if err != nil {
		return nil, fmt.Errorf("error fetching screenshot gallery: %v", err)
	}
	defer resp.Body.Close()

	var gallery ScreenshotGallery
	if err := json.NewDecoder(resp.Body).Decode(&gallery); err != nil {
		return nil, fmt.Errorf("error decoding gallery response: %v", err)
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
