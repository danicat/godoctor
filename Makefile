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

# Usage: make bump-version VERSION=0.21.0
bump-version:
	@if [ "$(origin VERSION)" != "command line" ]; then \
		echo "Error: VERSION must be explicitly specified on the command line. Usage: make bump-version VERSION=0.21.0"; \
		exit 1; \
	fi
	@git tag v$(VERSION)
	@git push origin main --tags
	@echo "🚀 Successfully tagged v$(VERSION) and pushed to remote!"

.PHONY: all build clean test test-cov snapshot release bump-version