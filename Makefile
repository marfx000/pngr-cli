.PHONY: help build install test test-smoke lint tidy snapshot

VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

help:
	@echo "pngr CLI — available make targets:"
	@echo ""
	@echo "  build        Build ./bin/pngr"
	@echo "  install      go install the pngr binary into GOBIN"
	@echo "  test         Run unit tests"
	@echo "  test-smoke   Run pkg/client smoke test against a live server"
	@echo "  tidy         go mod tidy"
	@echo "  snapshot     GoReleaser local dry-run (builds dist/, no publish)"

build:
	go build -ldflags "$(LDFLAGS)" -o bin/pngr .

install:
	go install -ldflags "$(LDFLAGS)" .

test:
	go test ./...

# Exercises pkg/client against a live server. Requires PNGR_BASE_URL and (after
# register) PNGR_VERIFY_TOKEN from the server log. See pkg/client/client_smoke_test.go.
test-smoke:
	go test -tags=smoke -v -run TestSmoke ./pkg/client

tidy:
	go mod tidy

snapshot:
	goreleaser release --snapshot --clean
