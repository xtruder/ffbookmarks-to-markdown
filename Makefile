# Build parameters
BINARY_NAME=ffbookmarks-to-markdown
MAIN_FILE=cmd/main.go
BUILD_DIR=build

# Default OS/ARCH
GOOS?=linux
GOARCH?=amd64

# Version from git tag
VERSION?=$(shell git describe --tags --always --dirty)

# ffsclient parameters
FFSCLIENT_VERSION=v1.8.0

# Build flags
LDFLAGS=-w -s -X main.version=$(VERSION) -extldflags "-static"

# Get all Go source files
GO_FILES=$(shell go list -f '{{range .GoFiles}}{{$$.Dir}}/{{.}} {{end}} {{range .TestGoFiles}}{{$$.Dir}}/{{.}} {{end}} {{range .XTestGoFiles}}{{$$.Dir}}/{{.}} {{end}}' ./...)

OUT?=.
IMG?=$(BINARY_NAME):$(VERSION)
TARGET=$(BUILD_DIR)/$(BINARY_NAME)-$(GOOS)-$(GOARCH)
RELEASE_TARGET=$(BUILD_DIR)/$(BINARY_NAME)-$(GOOS)-$(GOARCH).tar.gz
FFSCLIENT_TARGET=$(BUILD_DIR)/ffsclient-$(GOOS)-$(GOARCH)-$(FFSCLIENT_VERSION)

.PHONY: build clean release ffsclient install container

build: $(TARGET)
	ln -fs $(TARGET) $(BINARY_NAME)
release: $(RELEASE_TARGET)
ffsclient: $(FFSCLIENT_TARGET)
install: $(TARGET) $(FFSCLIENT_TARGET)
	install -m 755 $(TARGET) $(OUT)/$(BINARY_NAME)
	install -m 755 $(FFSCLIENT_TARGET) $(OUT)/ffsclient

# Download and install ffsclient
$(FFSCLIENT_TARGET):
	@echo "Downloading ffsclient $(FFSCLIENT_VERSION) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BUILD_DIR)
	@case "$(GOOS)-$(GOARCH)" in \
		"linux-amd64") \
			URL="https://github.com/Mikescher/firefox-sync-client/releases/download/$(FFSCLIENT_VERSION)/ffsclient_linux-amd64-static" ;; \
		"linux-arm64") \
			URL="https://github.com/Mikescher/firefox-sync-client/releases/download/$(FFSCLIENT_VERSION)/ffsclient_linux-arm64-static" ;; \
		"darwin-amd64") \
			URL="https://github.com/Mikescher/firefox-sync-client/releases/download/$(FFSCLIENT_VERSION)/ffsclient_macos-amd64" ;; \
		*) echo "Unsupported OS/ARCH: $(GOOS)-$(GOARCH)" && exit 1 ;; \
	esac && curl --fail -L -o $(FFSCLIENT_TARGET) $$URL
	@chmod +x $(FFSCLIENT_TARGET)

# Default build
$(TARGET): $(GO_FILES)
	@echo "Building for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -ldflags='$(LDFLAGS)' -o $(TARGET) $(MAIN_FILE)

# Create release archive
$(RELEASE_TARGET): $(TARGET) $(FFSCLIENT_TARGET)
	@echo "Creating release archive for $(GOOS)/$(GOARCH)..."
	@tar -C $(BUILD_DIR) \
		--transform='s|-$(GOOS)-$(GOARCH)-v[0-9.]*||' \
		--transform='s|-$(GOOS)-$(GOARCH)||' \
		-cvzf $(RELEASE_TARGET) $(notdir $(TARGET)) $(notdir $(FFSCLIENT_TARGET))
	@sha256sum $(RELEASE_TARGET) > $(RELEASE_TARGET).sha256

container:
	if [ -n "$(shell which docker)" ]; then \
		podman build --platform linux/amd64,linux/arm64  -t $(IMG) .; \
	elif [ -n "$(shell which podman)" ]; then \
		docker buildx build --platform linux/amd64,linux/arm64  -t $(IMG) .; \
	else \
		echo "No container tool found"; \
	fi

# Clean build directory
clean:
	@rm -rf $(BUILD_DIR)
