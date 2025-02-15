# Firefox Bookmarks to Markdown

[![Latest Release](https://img.shields.io/github/v/release/xtruder/ffbookmarks-to-markdown?style=flat-square)](https://github.com/xtruder/ffbookmarks-to-markdown/releases/latest)
[![Docker Image](https://img.shields.io/badge/container-ghcr.io-blue?style=flat-square)](https://github.com/xtruder/ffbookmarks-to-markdown/pkgs/container/ffbookmarks-to-markdown)

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

```shell
# Sync bookmarks from Firefox toolbar folder
ffbookmarks-to-markdown -folder toolbar -output bookmarks

# List available bookmarks
ffbookmarks-to-markdown -list

# Enable verbose logging
ffbookmarks-to-markdown -verbose
```

### Advanced Options

```shell
# Ignore specific folders
ffbookmarks-to-markdown -ignore "Archive,Old Stuff"

# Use custom LLM settings
ffbookmarks-to-markdown -llm-key "your-key" -llm-model "your-model"

# Use custom screenshot API
ffbookmarks-to-markdown -screenshot-api "https://your-screenshot-service"
```

## Installing

Here is a single-liner to install the binary to your local bin directory:

```shell
mkdir -p ~/.local/bin
curl -L https://github.com/xtruder/ffbookmarks-to-markdown/releases/download/v0.2.0/ffbookmarks-to-markdown-linux-amd64.tar.gz | tar -xz -C ~/.local/bin
```

## Running

First you need to login to your Firefox Sync account using `ffsclient` tool:

```shell
ffsclient login
```

It will ask you for your username and password.

After that you can run the tool, which will sync bookmarks from your Firefox Sync account and save them to the specified directory:

```shell
ffbookmarks-to-markdown -folder toolbar -output bookmarks
```

If you want to use LLM to clean up the markdown content, you need to set `GEMINI_API_KEY` environment variable:

```shell
export GEMINI_API_KEY="your-key"
```

## Full docker-compose example

Here is a full example of docker-compose file that will run gowitness, obsidian and ffbookmarks-to-markdown.
You must make sure to copy your Firefox Sync credentials to the `obsidian-config` volume.

```yaml
version: '3.9'
name: obsidian-stack
services:
  obsidian:
    container_name: obsidian
    image: linuxserver/obsidian:1.8.4
    networks:
      - net
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=UTC
      - DOCKER_MODS=linuxserver/mods:universal-package-installer
    volumes:
      - obsidian-config:/config
    shm_size: "1gb"
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [compute,video,graphics,utility]
    restart: unless-stopped
  gowitness:
    image: ghcr.io/sensepost/gowitness:latest
    restart: unless-stopped
    command: gowitness report server --host 0.0.0.0 --screenshot-path /data/screenshots --db-uri sqlite:///data/gowitness.sqlite3
    volumes:
      - gowitness-storage:/data
    networks:
      - net
  ffbookmarks-to-markdown:
    image: ghcr.io/xtruder/ffbookmarks-to-markdown:0.2.0
    command: -output /data/my-vault/Bookmarks -screenshot-api http://gowitness
    environment:
      - TZ=UTC
      - GEMINI_API_KEY=${GEMINI_API_KEY}
      - HOME=/home/user
    working_dir: /home/user
    user: 1000:1000
    restart: never
    volumes:
      - obsidian-config:/data
      - ffbookmarks-home:/home/user
    networks:
      - net

volumes:
  gowitness-storage:
  obsidian-config:
  ffbookmarks-home:

networks:
  net:
```

## Usage

```shell
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
