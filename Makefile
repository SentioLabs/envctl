.PHONY: build test lint install clean tidy fmt release-snapshot

# Version info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS = -X github.com/sentiolabs/envctl/internal/version.Version=$(VERSION) \
          -X github.com/sentiolabs/envctl/internal/version.GitCommit=$(GIT_COMMIT) \
          -X github.com/sentiolabs/envctl/internal/version.BuildDate=$(BUILD_DATE)

# Build the binary
build:
	go build -ldflags "$(LDFLAGS)" -o bin/envctl ./cmd/envctl

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	golangci-lint run

# Install to GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/envctl

# Clean build artifacts
clean:
	rm -rf bin/ dist/ coverage.out coverage.html

# Tidy dependencies
tidy:
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Run all checks
check: fmt tidy lint test

# GoReleaser snapshot (local dry-run)
release-snapshot:
	goreleaser release --snapshot --clean
