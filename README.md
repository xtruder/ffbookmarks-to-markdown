# Firefox Bookmarks to Markdown

A tool that syncs Firefox bookmarks to markdown files for use with tools like Obsidian.

- **Firefox Sync Integration**: Syncs bookmarks directly from Firefox Sync service using [ffsclient](https://github.com/Mikescher/firefox-sync-client)
- **Content download**:
  - Downloads content from the web using [markdowner](https://md.dhr.wtf/dashboard) service
  - Special handing for github repositories and youtube videos
- **Content Processing**: 
  - Markdown cleanup using LLM (gemini)
  - Fixes relative links
  - Preserves code blocks and technical content
- **Screenshots**: Captures website screenshots using [gowitness](https://github.com/sensepost/gowitness)
- **Caching**: Caches web content and LLM responses for efficiency
- **Obsidian Integration**: 
  - Creates year-based index files
  - Adds proper frontmatter
  - Compatible with Dataview plugin

## Usage

### Basic Usage

+++ bash
# Sync bookmarks from Firefox toolbar folder
ffbookmarks-to-markdown -folder toolbar -output bookmarks

# List available bookmarks
ffbookmarks-to-markdown -list

# Enable verbose logging
ffbookmarks-to-markdown -verbose
+++

### Advanced Options

+++ bash
# Ignore specific folders
ffbookmarks-to-markdown -ignore "Archive,Old Stuff"

# Use custom LLM settings
ffbookmarks-to-markdown -llm-key "your-key" -llm-model "your-model"

# Use custom screenshot API
ffbookmarks-to-markdown -screenshot-api "https://your-screenshot-service"
+++

## Running Locally

1. Install Go 1.21 or later
2. Clone the repository
3. Install ffsclient:
   +++ bash
   # Download ffsclient
   curl -L -o ffsclient https://github.com/Mikescher/firefox-sync-client/releases/download/v1.8.0/ffsclient_linux-amd64-static
   chmod +x ffsclient
   sudo mv ffsclient /usr/local/bin/
   +++
4. Build and run:
   +++ bash
   go build -o ffbookmarks-to-markdown ./cmd/main.go
   ./ffbookmarks-to-markdown -folder toolbar -output bookmarks
   +++

## Running with Podman

1. Create required volumes:
   +++ bash
   # Create volume for Firefox Sync credentials
   podman volume create firefox-sync-creds
   
   # Create volume for bookmarks output
   podman volume create bookmarks
   
   # Create volume for cache
   podman volume create ffbookmarks-cache
   +++

2. Copy Firefox Sync credentials:
   +++ bash
   # Get Firefox Sync credentials from your browser
   # ~/.mozilla/firefox/PROFILE/logins.json
   # Copy credentials to volume
   podman cp ~/.mozilla/firefox/YOUR_PROFILE/logins.json \
     firefox-sync-creds:/root/.mozilla/firefox/PROFILE/
   +++

3. Run the container:
   +++ bash
   podman run -it --rm \
     -v firefox-sync-creds:/root/.mozilla/firefox:ro \
     -v bookmarks:/bookmarks \
     -v ffbookmarks-cache:/root/.cache \
     -e GEMINI_API_KEY="your-key" \
     ghcr.io/xtruder/ffbookmarks-to-markdown:latest \
     -folder toolbar -output /bookmarks
   +++

## Environment Variables

- `GEMINI_API_KEY`: API key for Gemini LLM service (optional)

## Output Structure

The tool creates the following structure in your output directory:

+++ text
bookmarks/
├── 2024.md           # Year index
├── 2023.md           # Year index
└── folder/           # Bookmark folders
    └── bookmark.md   # Bookmark files
+++

Each bookmark file contains:
- Frontmatter with metadata
- Cleaned markdown content
- Screenshot (if available)
- Original URL and creation date

## License

MIT License 
