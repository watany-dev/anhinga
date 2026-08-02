.PHONY: all build test test-race cover vet lint fmt tools install clean run

# Build variables
BINARY_NAME=anhinga
BUILD_DIR=.
GO_FILES=$(shell find . -name '*.go')
LDFLAGS=-ldflags "-s -w"
GOLANGCI_LINT_VERSION ?= v2.12.2

# Default target
all: build

# Build the application
build: $(GO_FILES)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(LDFLAGS) ./cmd/anhinga

# Run tests
test:
	go test -v ./...

# Run tests with the race detector
test-race:
	go test -race -count=1 ./...

# Run tests and report coverage
cover:
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out | tail -n 1

# Run go vet
vet:
	go vet ./...

# Lint (config in .golangci.yml)
lint:
	golangci-lint run ./...

# Apply formatters configured in .golangci.yml
fmt:
	golangci-lint fmt ./...

# Install development tooling
tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

# Install the application
install:
	go install $(LDFLAGS) ./cmd/anhinga

# Clean build artifacts
clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME) coverage.out
	go clean

# Run the application
run:
	go run ./cmd/anhinga
