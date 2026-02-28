set shell := ["bash", "-cu"]

default:
  @just --list

fmt:
  gofmt -w .

fmt-check:
  test -z "$$(gofmt -l .)"

test:
  go test ./...

test-cover:
  go test -cover ./...

test-e2e:
  go test -tags e2e ./e2e -v

vet:
  go vet ./...

build:
  go build -o autoport ./main.go

build-dev:
  go build -ldflags "-X main.version=dev -X main.buildTime=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o autoport ./main.go

install:
  go install github.com/gelleson/autoport@latest

install-dev:
  tmp_bin="$$(mktemp -t autoport-dev.XXXXXX)"
  go build -ldflags "-X main.version=dev -X main.buildTime=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o "$$tmp_bin" ./main.go
  if [ -w /usr/local/bin ]; then install -m 0755 "$$tmp_bin" /usr/local/bin/autoport; elif command -v sudo >/dev/null 2>&1; then sudo install -m 0755 "$$tmp_bin" /usr/local/bin/autoport; else echo "error: /usr/local/bin is not writable and sudo is unavailable"; rm -f "$$tmp_bin"; exit 1; fi
  rm -f "$$tmp_bin"

ci: fmt-check vet test test-e2e build
