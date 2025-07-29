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
VERSION := 0.2.0
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

all: build

build:
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(SERVER_BINARY) ./cmd/godoctor
	$(GOBUILD) $(LDFLAGS) -o $(CLIENT_BINARY) ./cmd/godoctor-cli

clean:
	@rm -rf $(BINARY_DIR)

test:
	$(GOTEST) -v ./...

test-cov:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	@echo "to view the coverage report, run: go tool cover -html=coverage.out"

gofmt:
	$(GOCMD) fmt ./...

goimports:
	$(GOCMD) run golang.org/x/tools/cmd/goimports@latest -w $(shell pwd)

integration-test: build
	@echo "--- Running Integration Test: go-doc ---"
	$(CLIENT_BINARY) -server $(SERVER_BINARY) fmt Println
	@echo "\n--- Running Integration Test: code_review ---"
	$(CLIENT_BINARY) -server $(SERVER_BINARY) -review cmd/godoctor/main.go

install: build
	@echo "Installing $(SERVER_BINARY_NAME) and $(CLIENT_BINARY_NAME) using go install..."
	$(GOCMD) install $(LDFLAGS) ./cmd/godoctor
	$(GOCMD) install $(LDFLAGS) ./cmd/godoctor-cli
	@echo "Installation complete. Binaries typically installed to $GOPATH/bin or $GOBIN."

.PHONY: all build clean test test-cov gofmt goimports integration-test install
