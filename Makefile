BIN := claude-atlas

.PHONY: build install test vet

build:
	go build -o bin/$(BIN) ./cmd/$(BIN)

install:
	go install ./cmd/$(BIN)

test:
	go test ./...

vet:
	go vet ./...
