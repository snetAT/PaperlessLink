BINARY   := paperlesslink
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -ldflags "-X main.version=$(VERSION)"
BUILD_DIR := bin

.PHONY: all build build-linux build-windows build-darwin clean run help

## help: Show this help
help:
	@grep -E '^##' Makefile | sed 's/## //'

## all: Build for Linux, Windows, and macOS (amd64)
all: build-linux build-windows build-darwin

## build: Build for the current platform
build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) .

## build-linux: Cross-compile for Linux amd64
build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 .

## build-linux-arm64: Cross-compile for Linux arm64 (e.g. Raspberry Pi)
build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64 .

## build-windows: Cross-compile for Windows amd64
build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe .

## build-darwin: Cross-compile for macOS amd64
build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 .

## build-darwin-arm64: Cross-compile for macOS Apple Silicon
build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 .

## run: Run locally (set DIR, URL, TOKEN env vars or pass flags manually)
run:
	go run . \
	  -dir  "$${DIR:-/tmp/watch}" \
	  -url  "$${PAPERLESS_URL:-http://localhost:8000}" \
	  -token "$${PAPERLESS_TOKEN:-changeme}" \
	  -ext  "$${EXT:-pdf}" \
	  -after-upload "$${AFTER_UPLOAD:-delete}" \
	  -log-file "$${LOG_FILE:-}"

## clean: Remove compiled binaries
clean:
	rm -rf $(BUILD_DIR)

## tidy: Run go mod tidy
tidy:
	go mod tidy
