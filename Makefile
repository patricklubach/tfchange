# Makefile for tfchange
#
# This Makefile provides convenience targets for building, testing, linting,
# formatting, and cleaning the tfchange Go project.

# Binary name
BINARY_NAME := tfchange

# Go build flags (set -v for verbose)
GOFLAGS     := -v

# Ensure golangci-lint is available
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null)

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@go build $(GOFLAGS) -o $(BINARY_NAME) ./...

# Run unit tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test $(GOFLAGS) ./...

# Lint using golangci-lint
.PHONY: lint
lint:
ifeq ($(GOLANGCI_LINT),)
	@echo "golangci-lint not found in PATH. Please install it to run linting." >&2
	@exit 1
else
	@echo "Running golangci-lint..."
	@golangci-lint run ./...
endif

# Format and vet the code
.PHONY: fmt
fmt:
	@echo "Formatting and vetting..."
	@go fmt ./...
	@go vet ./...

# Clean up generated files
.PHONY: clean
clean:
	@echo "Cleaning up..."
	@rm -f $(BINARY_NAME)

# Run all quality checks
.PHONY: check
check: fmt lint test

# Help target
.PHONY: help
help:
	@echo "Makefile targets:"
	@echo "  build   - Build the tfchange binary"
	@echo "  test    - Run unit tests"
	@echo "  lint    - Run golangci-lint (requires installation)"
	@echo "  fmt     - Format with gofmt and run go vet"
	@echo "  clean   - Remove generated binary"
	@echo "  check   - Run fmt, lint, and test"
	@echo "  help    - Show this help message"
