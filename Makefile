.PHONY: build test clean

# Build variables
BINARY_NAME=anhinga
BUILD_DIR=.
GO_FILES=$(shell find . -name '*.go')
LDFLAGS=-ldflags "-s -w"

# Default target
all: build

# Build the application
build: $(GO_FILES)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(LDFLAGS) ./cmd/anhinga

# Run tests
test:
	go test -v ./...

# Install the application
install:
	go install $(LDFLAGS) ./cmd/anhinga

# Clean build artifacts
clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)
	go clean

# Run the application
run:
	go run ./cmd/anhinga