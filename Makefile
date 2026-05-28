# Makefile for GoDoctor

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_DIR=bin
SERVER_BINARY_NAME=godoctor
SERVER_BINARY=$(BINARY_DIR)/$(SERVER_BINARY_NAME)

# Version derived dynamically from Git tags
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
ifeq ($(VERSION),)
  VERSION := dev
endif
LDFLAGS=-ldflags "-X main.version=$(VERSION)"


all: build

build:
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(SERVER_BINARY) ./cmd/godoctor

install:
	$(GOCMD) install $(LDFLAGS) ./...

clean:
	@rm -rf $(BINARY_DIR)

test:
	$(GOTEST) -v ./...

test-cov:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	@echo "to view the coverage report, run: go tool cover -html=coverage.out"

snapshot:
	goreleaser release --snapshot --clean

release:
	goreleaser release --clean

# Usage: make bump-version VERSION=0.16.2
bump-version:
	@if [ "$(origin VERSION)" != "command line" ]; then \
		echo "Error: VERSION must be explicitly specified on the command line. Usage: make bump-version VERSION=0.16.2"; \
		exit 1; \
	fi
	@python3 -c "import re; f = 'gemini-extension.json'; content = open(f).read(); new_content = re.sub(r'\"version\":\s*\"[^\"]+\"', '\"version\": \"$(VERSION)\"', content); open(f, 'w').write(new_content);"
	@echo "Successfully bumped version to $(VERSION) in gemini-extension.json"

.PHONY: all build install clean test test-cov snapshot release bump-version