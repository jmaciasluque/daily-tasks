.PHONY: all build run test test-verbose clean lint fmt help

# Default target
all: test build

# Build the binary
build:
	go build -o bin/daily-tasks .

# Run the application
run:
	go run .

# Run tests
test:
	go test ./... -cover

# Run tests with verbose output
test-verbose:
	go test ./... -v -cover

# Run tests with race detection
test-race:
	go test ./... -race

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...
	goimports -w .

# Show coverage report
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Install development tools
dev-tools:
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Show help
help:
	@echo "Available targets:"
	@echo "  all          - Run tests and build (default)"
	@echo "  build        - Build the binary to bin/daily-tasks"
	@echo "  run          - Run the application directly"
	@echo "  test         - Run tests with coverage"
	@echo "  test-verbose - Run tests with verbose output"
	@echo "  test-race    - Run tests with race detection"
	@echo "  clean        - Remove build artifacts"
	@echo "  lint         - Run golangci-lint"
	@echo "  fmt          - Format code"
	@echo "  coverage     - Generate HTML coverage report"
	@echo "  dev-tools    - Install development tools"
	@echo "  help         - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  DAILY_TASKS_PATH         - Custom path for data file"
	@echo "  DAILY_TASKS_WEBDAV_URL   - WebDAV URL for sync"
	@echo "  DAILY_TASKS_WEBDAV_USER  - WebDAV username"
	@echo "  DAILY_TASKS_WEBDAV_PASS  - WebDAV password"
