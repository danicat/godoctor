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
VERSION := 0.1.0
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

.PHONY: all build clean test test-cov