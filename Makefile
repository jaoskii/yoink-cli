BINARY    := yoink
PKG       := ./cmd/yoink
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -s -w -X main.version=$(VERSION)
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64

GOBIN_DIR := $(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)

.PHONY: help deps build install uninstall run test race vet lint cross clean

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  make /'

## deps: download Go module dependencies
deps:
	go mod download

## build: compile for this machine into bin/
build: deps
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)

## install: build and install yoink into Go's bin folder (on PATH after setup)
install: deps
	CGO_ENABLED=0 go install -trimpath -ldflags "$(LDFLAGS)" $(PKG)
	@echo "Installed $(GOBIN_DIR)/$(BINARY)"

## uninstall: remove the installed binary
uninstall:
	rm -f $(GOBIN_DIR)/$(BINARY) $(GOBIN_DIR)/$(BINARY).exe

## run: go run with ARGS, e.g. make run ARGS="./README.md --once"
run:
	go run $(PKG) $(ARGS)

## test: run all tests
test:
	go test ./...

## race: run tests with the race detector
race:
	go test -race ./...

## vet: static checks
vet:
	go vet ./...

## lint: golangci-lint (install: https://golangci-lint.run)
lint:
	golangci-lint run

## cross: static binaries for every target into dist/
cross:
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=""; \
		if [ "$$os" = windows ]; then ext=.exe; fi; \
		echo "  $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" \
			-o dist/$(BINARY)_$${os}_$${arch}$$ext $(PKG) || exit 1; \
	done

## clean: remove bin/ and dist/
clean:
	rm -rf bin dist
