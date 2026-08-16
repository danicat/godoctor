.PHONY: build test install clean

build:
	@go build -o bin/godoctor ./cmd/godoctor

test:
	@go test ./...

install:
	@./install.sh

clean:
	@rm -rf bin/