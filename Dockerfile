# Build stage
FROM --platform=$BUILDPLATFORM golang:1.23-bullseye AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with static linking
ARG TARGETARCH
RUN GOARCH=$TARGETARCH make install

# Final stage
FROM --platform=$TARGETPLATFORM gcr.io/distroless/static-debian12:nonroot

# Copy the binary and ffsclient
COPY --from=builder /build/build/ffbookmarks-to-markdown-linux-* /ffbookmarks-to-markdown
COPY --from=builder /build/build/ffsclient-* /usr/local/bin/ffsclient

# Use nonroot user
USER nonroot:nonroot

ENTRYPOINT ["/ffbookmarks-to-markdown"] 
