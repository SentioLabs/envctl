.PHONY: build test lint install clean tidy fmt

# Build the binary
build:
	go build -o bin/envctl ./cmd/envctl

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
	go install ./cmd/envctl

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html

# Tidy dependencies
tidy:
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Run all checks
check: fmt tidy lint test
