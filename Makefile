BIN := ekiben
VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)

.PHONY: build test fmt install

build:
	CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/KewinGit/ekiben/internal/version.Version=$(VERSION)" -o $(BIN) ./cmd/ekiben

test:
	go test ./...

fmt:
	gofmt -w .

install: build
	install -m 0755 $(BIN) $(HOME)/.local/bin/$(BIN)
