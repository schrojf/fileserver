# Simple Web File Server Makefile

BINARY_NAME=fileserver
VERSION=1.0.0
BUILD_DIR=build

# Default target
.PHONY: all
all: clean build

# Clean build directory
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	mkdir -p $(BUILD_DIR)

# Build for current platform
.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) .

# Build for Linux AMD64
.PHONY: build-linux-amd64
build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .

# Build for Linux ARM64 (Raspberry Pi 4)
.PHONY: build-linux-arm64
build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .

# Build for Linux ARM (Raspberry Pi 2/3)
.PHONY: build-linux-arm
build-linux-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm .

# Build all Linux targets
.PHONY: build-all
build-all: clean build-linux-amd64 build-linux-arm64 build-linux-arm
	@echo "All builds completed:"
	@ls -la $(BUILD_DIR)/

# Run tests
.PHONY: test
test:
	go test -v ./...

# Run with default settings
.PHONY: run
run:
	go run . -root ./test -port 8080

# Install dependencies and verify
.PHONY: deps
deps:
	go mod tidy
	go mod verify

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Lint code (requires golangci-lint)
.PHONY: lint
lint:
	golangci-lint run

# Create directory structure for templates
.PHONY: init
init:
	mkdir -p templates

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all             - Clean and build for current platform"
	@echo "  clean           - Clean build directory"
	@echo "  build           - Build for current platform"
	@echo "  build-linux-amd64 - Build for Linux x64"
	@echo "  build-linux-arm64 - Build for Linux ARM64 (RPi 4)"
	@echo "  build-linux-arm   - Build for Linux ARM (RPi 2/3)"
	@echo "  build-all       - Build for all Linux targets"
	@echo "  test            - Run tests"
	@echo "  run             - Run with default settings"
	@echo "  deps            - Install and verify dependencies"
	@echo "  fmt             - Format code"
	@echo "  lint            - Lint code"
	@echo "  help            - Show this help"
