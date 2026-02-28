# Contributing

## Prerequisites

- Go 1.21+
- Git

## Setup

```bash
git clone https://github.com/gelleson/autoport.git
cd autoport
go build -o autoport ./main.go
```

Optional task runner:

```bash
# https://github.com/casey/just
just build
just install-dev
```

`just install-dev` installs `/usr/local/bin/autoport` from the local checkout and embeds `version=dev` with current UTC build time.

## Quality checks

Run before opening a PR:

```bash
gofmt -w .
go test ./...
go test -cover ./...
go vet ./...
```

Or run equivalent `just` tasks:

```bash
just fmt
just test
just test-cover
just vet
```

## Project map

- `main.go`: CLI wiring and argument parsing
- `internal/app`: application orchestration
- `internal/scanner`: env + `.env` key discovery
- `internal/config`: preset loading/merge logic
- `internal/env`: `.env` parsing helpers
- `pkg/port`: deterministic range allocation primitives
- `docs/`: user and architecture docs

## Pull requests

1. Create a branch from `main`
2. Keep commits focused and clearly titled
3. Add or update tests for behavior changes
4. Update docs when CLI behavior or public API changes
5. Open PR to `main`

## Notes

- Avoid changing behavior unintentionally; this tool is used as a wrapper in scripts.
- Preserve deterministic semantics when modifying allocation logic.

## License

By contributing, you agree your contributions are licensed under the MIT License.
