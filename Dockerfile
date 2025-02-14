# Build stage
FROM golang:1.23-bullseye AS builder

WORKDIR /app

# Install ffsclient
ADD https://github.com/Mikescher/firefox-sync-client/releases/download/v1.8.0/ffsclient_linux-amd64-static /usr/local/bin/ffsclient
RUN chmod +x /usr/local/bin/ffsclient

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with static linking
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags='-w -s -extldflags "-static"' -o /ffbookmarks-to-markdown

# Final stage
FROM gcr.io/distroless/static-debian12:nonroot

# Copy the binary and ffsclient
COPY --from=builder /ffbookmarks-to-markdown /ffbookmarks-to-markdown
COPY --from=builder /usr/local/bin/ffsclient /usr/local/bin/ffsclient

# Use nonroot user
USER nonroot:nonroot

ENTRYPOINT ["/ffbookmarks-to-markdown"] 
