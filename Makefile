# Makefile for GoDoctor

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_DIR=bin
SERVER_BINARY_NAME=godoctor
CLIENT_BINARY_NAME=godoctor-cli
SERVER_BINARY=$(BINARY_DIR)/$(SERVER_BINARY_NAME)
CLIENT_BINARY=$(BINARY_DIR)/$(CLIENT_BINARY_NAME)

# Version
VERSION := 0.1.4
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

all: build

build:
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(SERVER_BINARY) ./cmd/godoctor
	$(GOBUILD) $(LDFLAGS) -o $(CLIENT_BINARY) ./cmd/godoctor-cli

install:
	$(GOCMD) install $(LDFLAGS) ./...

clean:
	@rm -rf $(BINARY_DIR)

test:
	$(GOTEST) -v ./...

test-cov:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	@echo "to view the coverage report, run: go tool cover -html=coverage.out"

integration-test: build
	@echo "--- Running Integration Test: go-doc ---"
	$(CLIENT_BINARY) -server $(SERVER_BINARY) fmt Println
	@echo "\n--- Running Integration Test: code_review ---"
	$(CLIENT_BINARY) -server $(SERVER_BINARY) -review cmd/godoctor/main.go

.PHONY: all build install clean test test-cov integration-test
