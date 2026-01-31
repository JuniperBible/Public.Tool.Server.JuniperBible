# Juniper Host - Makefile

VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)
BINARY := juniper-host

.PHONY: build clean test install release-local

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/juniper-host

clean:
	rm -f $(BINARY) juniper-host-*

test:
	go test -v ./...

install: build
	sudo cp $(BINARY) /usr/local/bin/

# Build for all platforms locally
release-local:
	@mkdir -p dist
	# Linux
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 ./cmd/juniper-host
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 ./cmd/juniper-host
	GOOS=linux GOARCH=386 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-386 ./cmd/juniper-host
	GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-armv7 ./cmd/juniper-host
	# macOS
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 ./cmd/juniper-host
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 ./cmd/juniper-host
	# Windows
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe ./cmd/juniper-host
	GOOS=windows GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-windows-arm64.exe ./cmd/juniper-host
	# FreeBSD
	GOOS=freebsd GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-freebsd-amd64 ./cmd/juniper-host
	GOOS=freebsd GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-freebsd-arm64 ./cmd/juniper-host
	@echo "Built binaries in dist/"
	@ls -lh dist/
