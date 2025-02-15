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

```bash
# Sync bookmarks from Firefox toolbar folder
ffbookmarks-to-markdown -folder toolbar -output bookmarks

# List available bookmarks
ffbookmarks-to-markdown -list

# Enable verbose logging
ffbookmarks-to-markdown -verbose
```

### Advanced Options

``` bash
# Ignore specific folders
ffbookmarks-to-markdown -ignore "Archive,Old Stuff"

# Use custom LLM settings
ffbookmarks-to-markdown -llm-key "your-key" -llm-model "your-model"

# Use custom screenshot API
ffbookmarks-to-markdown -screenshot-api "https://your-screenshot-service"
```

## Installing

Here is a single-liner to install the binary to your local bin directory:

```bash
mkdir -p ~/.local/bin
curl -L https://github.com/xtruder/ffbookmarks-to-markdown/releases/download/v0.1.0/ffbookmarks-to-markdown-linux-amd64.tar.gz | tar -xz -C ~/.local/bin
```

## Running

First you need to login to your Firefox Sync account using `ffsclient` tool:

```bash
ffsclient login
```

It will ask you for your username and password.

After that you can run the tool, which will sync bookmarks from your Firefox Sync account and save them to the specified directory:

```bash
ffbookmarks-to-markdown -folder toolbar -output bookmarks
```

If you want to use LLM to clean up the markdown content, you need to set `GEMINI_API_KEY` environment variable:

```bash
export GEMINI_API_KEY="your-key"
```

## Running with Podman

1. Create required volumes:
   ```bash
   # Create volume for Firefox Sync credentials
   podman volume create firefox-sync-creds
   
   # Create volume for bookmarks output
   podman volume create bookmarks
   
   # Create volume for cache
   podman volume create ffbookmarks-cache
   ```

2. Copy Firefox Sync credentials:
   ```bash
   # Get Firefox Sync credentials you generated using ffsclient tool
   # ~/.config/firefox-sync-client.secret
   # Copy credentials to volume
   podman cp ~/.config/firefox-sync-client.secret \
     firefox-sync-creds:/home/nonroot/.config/firefox-sync-client.secret
   ```

3. Run the container:
   ```bash
   podman run -it --rm \
     -v firefox-sync-creds:/home/nonroot/.config:ro \
     -v bookmarks:/bookmarks \
     -v ffbookmarks-cache:/home/nonroot/.cache \
     -e GEMINI_API_KEY="your-key" \
     ghcr.io/xtruder/ffbookmarks-to-markdown:v0.1.0 \
     -folder toolbar -output /bookmarks
   ```

## Usage

```
Usage of ./ffbookmarks-to-markdown:
  -folder string
        Base folder name to sync from Firefox bookmarks (default "toolbar")
  -ignore string
        Comma-separated list of folder names to ignore
  -list
        List all available bookmarks
  -llm-key string
        API key for LLM service
  -llm-model string
        Model to use for LLM service (default "gemini-2.0-flash")
  -llm-url string
        Base URL for LLM service (default "https://generativelanguage.googleapis.com/v1beta/openai/")
  -output string
        Output directory for markdown files (default "bookmarks")
  -screenshot-api string
        Screenshot API base URL (default "https://gowitness.cloud.x-truder.net")
  -verbose
        Enable verbose logging
```

## Environment Variables

- `GEMINI_API_KEY`: API key for Gemini LLM service (optional)

## Output Structure

The tool creates the following structure in your output directory:

```text
bookmarks/
├── 2024.md           # Year index
├── 2023.md           # Year index
└── folder/           # Bookmark folders
    └── bookmark.md   # Bookmark files
```

Each bookmark file contains:
- Frontmatter with metadata
- Cleaned markdown content
- Screenshot (if available)
- Original URL and creation date

## License

MIT License 
