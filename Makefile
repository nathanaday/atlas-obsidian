BIN := claude-atlas
# The binary reports the plugin's version so `doctor` and `status` can tell when the two drift.
VERSION ?= $(shell sed -n 's/.*"version": "\([^"]*\)".*/\1/p' .claude-plugin/plugin.json | head -1)
LDFLAGS := -ldflags "-X github.com/nathanaday/claude-atlas/internal/cli.Version=$(VERSION)"

.PHONY: build install test vet

build:
	go build $(LDFLAGS) -o build/$(BIN) ./cmd/$(BIN)

install:
	go install $(LDFLAGS) ./cmd/$(BIN)

test:
	go test ./...

vet:
	go vet ./...
