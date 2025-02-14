// Main command logic, flag parsing, and orchestration

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/xtruder/ffbookmarks-to-markdown/internal/bookmarks"
	"github.com/xtruder/ffbookmarks-to-markdown/internal/firefox"
	"github.com/xtruder/ffbookmarks-to-markdown/internal/llm"
	"github.com/xtruder/ffbookmarks-to-markdown/internal/markdown"
	"github.com/xtruder/ffbookmarks-to-markdown/internal/web"
	"github.com/xtruder/ffbookmarks-to-markdown/internal/x"
)

var (
	// Command line flags
	baseFolder    string
	outputDir     string
	listBookmarks bool
	verbose       bool
	ignoreFolders string
	screenshotAPI string
	llmAPIKey     string
	llmBaseURL    string
	llmModel      string
)

func main() {
	// Define command line flags
	flag.StringVar(&baseFolder, "folder", "toolbar", "Base folder name to sync from Firefox bookmarks")
	flag.StringVar(&outputDir, "output", "bookmarks", "Output directory for markdown files")
	flag.BoolVar(&listBookmarks, "list", false, "List all available bookmarks")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&ignoreFolders, "ignore", "", "Comma-separated list of folder names to ignore")
	flag.StringVar(&screenshotAPI, "screenshot-api", "https://gowitness.cloud.x-truder.net", "Screenshot API base URL")
	flag.StringVar(&llmAPIKey, "llm-key", "", "API key for LLM service")
	flag.StringVar(&llmBaseURL, "llm-url", "https://generativelanguage.googleapis.com/v1beta/openai/", "Base URL for LLM service")
	flag.StringVar(&llmModel, "llm-model", "gemini-2.0-flash", "Model to use for LLM service")
	flag.Parse()

	// Get API key from environment if not provided
	if llmAPIKey == "" {
		llmAPIKey = os.Getenv("GEMINI_API_KEY")
	}

	// Initialize logger
	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Initialize HTTP client
	client := retryablehttp.NewClient()
	client.RetryMax = 3
	client.Logger = nil // Disable retryable client logging

	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Error("failed to get home directory", "error", err)
		os.Exit(1)
	}

	cacheDir := filepath.Join(homeDir, ".cache", "ffbookmarks-to-markdown")

	// Initialize cache
	cache, err := x.NewFileCache(cacheDir)
	if err != nil {
		slog.Warn("failed to initialize cache", "error", err)
	}

	llmClient, err := llm.NewOpenAIClient(llmAPIKey, llmBaseURL, llmModel, client.StandardClient(), cache)
	if err != nil {
		slog.Error("failed to initialize LLM client", "error", err)
		os.Exit(1)
	}

	// Initialize services
	ffFetcher := firefox.NewFirefoxFetcher()
	contentService := web.NewContentService(client.StandardClient(), web.FetchOptions{
		BaseURL:        "https://md.dhr.wtf",
		ContentCleaner: llmClient,
		Cache:          cache,
	})
	screenshotService := web.NewScreenshotService(client.StandardClient(), screenshotAPI)

	// Get Firefox bookmarkRoot
	bookmarkRoot, err := ffFetcher.GetBookmarks()
	if err != nil {
		slog.Error("failed to get Firefox bookmarks", "error", err)
		os.Exit(1)
	}

	// Find target folder
	targetFolder := bookmarkRoot.Path(baseFolder)
	if targetFolder == nil {
		fmt.Printf("Folder '%s' not found in bookmarks\n", baseFolder)
		os.Exit(1)
	}

	// Parse ignored folders
	var ignoredFoldersList []string
	if ignoreFolders != "" {
		ignoredFoldersList = strings.Split(ignoreFolders, ",")
	}

	// Collect new URLs for screenshots
	allBookmarks := x.Filter2(
		targetFolder.All(),
		func(path string, v *bookmarks.Bookmark) bool {
			for _, ignorePath := range ignoredFoldersList {
				if strings.HasPrefix(path, ignorePath) {
					return false
				}
			}

			return v.Type == "bookmark" && !v.Deleted
		},
	)

	if listBookmarks {
		for path := range allBookmarks {
			fmt.Println(path)
		}

		os.Exit(0)
	}

	// Get existing screenshots
	screenshots, err := screenshotService.GetExistingScreenshots()
	if err != nil {
		slog.Error("failed to get existing screenshots", "error", err)
		os.Exit(1)
	}

	mdCache, err := markdown.BuildCache(outputDir)
	if err != nil {
		slog.Error("failed to build markdown cache", "error", err)
		os.Exit(1)
	}

	newURLs := mdCache.CollectNewURLs(x.Values(allBookmarks))

	// Filter URLs that need screenshots
	var urlsToScreenshot []string
	for _, u := range newURLs {
		if !screenshots[u] {
			urlsToScreenshot = append(urlsToScreenshot, u)
		}
	}

	// Submit new screenshots
	if len(urlsToScreenshot) > 0 {
		slog.Info("submitting batch screenshot request",
			"total", len(newURLs),
			"new", len(urlsToScreenshot),
			"cached", len(newURLs)-len(urlsToScreenshot))
		if err := screenshotService.SubmitScreenshots(urlsToScreenshot); err != nil {
			slog.Error("failed to submit screenshots", "error", err)
		}
	} else {
		slog.Info("no new screenshots needed",
			"total", len(newURLs),
			"cached", len(newURLs))
	}

	// Process bookmarks
	mdProcessor := markdown.NewProcessor(
		markdown.ProcessorOptions{
			OutputDir:      outputDir,
			IgnoredFolders: ignoredFoldersList,
		},
		contentService,
		screenshotService,
		mdCache,
	)

	// Process bookmarks and create indexes
	if err := mdProcessor.ProcessBookmarks(*targetFolder, ""); err != nil {
		slog.Error("failed to process bookmarks", "error", err)
		os.Exit(1)
	}

	if err := mdProcessor.CreateYearIndexes(x.Values(allBookmarks)); err != nil {
		slog.Error("failed to create year indexes", "error", err)
		os.Exit(1)
	}
}
