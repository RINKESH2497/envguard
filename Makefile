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
	go build -o $(BINARY) ./cmd/envguard

## test: Run all tests
test:
	@echo "==> Running tests..."
	go test ./...

## lint: Run golangci-lint
lint:
	@echo "==> Linting..."
	golangci-lint run

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
