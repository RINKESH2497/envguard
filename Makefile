.PHONY: all build test lint fmt install clean release

# Binary output directory
BIN_DIR := bin
BINARY  := $(BIN_DIR)/envguard

# Default target
all: fmt lint test build

## build: Compile the envguard binary
build:
	@echo "==> Building..."
	@mkdir -p $(BIN_DIR)
	go build -ldflags="-s -w -X main.version=0.1.0" -o $(BINARY) ./cmd/envguard

## test: Run all tests
test:
	@echo "==> Running tests..."
	go test ./...

## lint: Run go vet and golangci-lint if available
lint:
	@echo "==> Linting..."
	go vet ./...
	@which golangci-lint >/dev/null 2>&1 && golangci-lint run || true

## fmt: Format all Go source files
fmt:
	@echo "==> Formatting..."
	gofmt -w .

## install: Install envguard to $GOPATH/bin
install:
	@echo "==> Installing..."
	go install ./cmd/envguard

## clean: Remove build artifacts
clean:
	@echo "==> Cleaning..."
	rm -rf $(BIN_DIR)

## release: Create a release with goreleaser
release:
	@echo "==> Releasing..."
	goreleaser release --clean

## help: Show this help message
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
