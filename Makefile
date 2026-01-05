# Makefile for router-brute project

# Default target
all: build

# Build the project
build:
	@echo "Building router-brute..."
	go build -o router-brute cmd/router-brute/main.go
	@echo "Build complete: router-brute"

# Clean build artifacts
clean:
	rm -f router-brute
	@echo "Cleaned build artifacts"

# Run tests
test:
	@echo "Running tests..."
		go test ./internal/... ./pkg/... ./cmd/...
	@echo "Tests complete"

# Format code
fmt:
	@echo "Formatting code..."
	gofmt -w .
	@echo "Code formatted"

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run ./internal/... ./pkg/... ./cmd/...
	@echo "Linting complete"

# Run the application
run:
	@echo "Running router-brute..."
	./router-brute

# Install dependencies
install:
	@echo "Installing dependencies..."
	go mod tidy
	@echo "Dependencies installed"

# Generate documentation
docs:
	@echo "Generating documentation..."
	go doc -all . > docs/generated_docs.md
	@echo "Documentation generated"

# Build for release
release:
	@echo "Building release..."
	GOOS=linux GOARCH=amd64 go build -o release/router-brute-linux-amd64 cmd/router-brute/main.go
	GOOS=windows GOARCH=amd64 go build -o release/router-brute-windows-amd64.exe cmd/router-brute/main.go
	GOOS=darwin GOARCH=amd64 go build -o release/router-brute-darwin-amd64 cmd/router-brute/main.go
	@echo "Release build complete"

# Show help
help:
	@echo "Available targets:"
	@echo "  all         - Build the project (default)"
	@echo "  build       - Build the project"
	@echo "  clean       - Clean build artifacts"
	@echo "  test        - Run tests"
	@echo "  fmt         - Format code"
	@echo "  lint        - Lint code"
	@echo "  run         - Run the application"
	@echo "  install     - Install dependencies"
	@echo "  docs        - Generate documentation"
	@echo "  release     - Build for release"
	@echo "  help        - Show this help message"

.PHONY: all build clean test fmt lint run install docs release help
