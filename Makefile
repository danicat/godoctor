# Version derived dynamically from Git tags
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
ifeq ($(VERSION),)
  VERSION := dev
endif

build:
	@go build -o bin/godoctor ./cmd/godoctor

test:
	@go test -v ./...

install:
	@./install.sh

uninstall:
	@./uninstall.sh

# Usage: make bump-version VERSION=0.21.0
bump-version:
	@if [ "$(origin VERSION)" != "command line" ]; then \
		echo "Error: VERSION must be explicitly specified on the command line. Usage: make bump-version VERSION=0.21.0"; \
		exit 1; \
	fi
	@git tag v$(VERSION)
	@git push origin main --tags
	@echo "🚀 Successfully tagged v$(VERSION) and pushed to remote!"

.PHONY: build test install uninstall bump-version