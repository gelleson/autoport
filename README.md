# autoport

`autoport` is a deterministic port wrapper for local development.

It discovers `PORT` and `*_PORT` environment keys, assigns free ports based on your project path, and either:
- runs your command with overrides, or
- prints shell exports when no command is provided.

## Why use it

- Prevents port collisions across multiple local projects
- Keeps port assignment stable for the same project path
- Works with existing workflows (`npm`, `go run`, Docker, test commands)
- Requires no database, daemon, or lock file

## Installation

```bash
go install github.com/gelleson/autoport@latest
```

Single-line installer (macOS/Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/gelleson/autoport/main/scripts/install.sh | sh
```

Install script options:

```bash
# Install a specific release tag
curl -fsSL https://raw.githubusercontent.com/gelleson/autoport/main/scripts/install.sh | sh -s -- v1.0.0

# Install to a custom directory
INSTALL_DIR="$HOME/.local/bin" curl -fsSL https://raw.githubusercontent.com/gelleson/autoport/main/scripts/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/gelleson/autoport/main/scripts/install.ps1 | iex
```

## Quick Start

Run an app with deterministic ports:

```bash
autoport npm start
```

Export ports into your current shell:

```bash
eval "$(autoport)"
```

Use a custom range:

```bash
autoport -r 8000-8999 npm start
```

Apply a preset and ignore prefixes:

```bash
autoport -p db -i REDIS_ npm start
```

## CLI

```text
autoport [flags] [command ...]
```

Flags:
- `-r <start-end>`: Port range (default: `10000-20000`)
- `-p <name>`: Preset name (repeatable)
- `-i <prefix>`: Ignore env keys starting with prefix (repeatable)
- `-k <env_key>`: Include a port env key manually (repeatable), e.g. `WEB_PORT`
- `-f, -format <shell|json>`: Output format (default: `shell`)
- `-q, -quiet`: Suppress command-mode override summaries
- `-n, -dry-run`: Preview resolved overrides without executing the command

Behavior:
- With `command`: executes command with port overrides in process env and prints a summary to `stderr`
- Without `command`: prints `export KEY=value` lines sorted by key
- With `-f json`: emits structured JSON instead of shell exports/table summaries
- With `-n`: prints preview output and exits `0` without running the command

## Configuration

`autoport` loads presets from:
1. `~/.autoport.json`
2. `./.autoport.json` (overrides same preset names from home config)

Example:

```json
{
  "presets": {
    "web": {
      "ignore": ["STRIPE_", "AWS_"],
      "range": "8000-9000"
    }
  }
}
```

Built-in presets:
- `db`: ignores database-style prefixes (`DB`, `DATABASE`, `POSTGRES`, `MYSQL`, `MONGO`, `REDIS`, `MEMCACHED`, `ES`, `CLICKHOUSE`, `INFLUX`)

## How it works

1. Hashes absolute current working directory path (FNV-1a, 32-bit)
2. Discovers target keys from current env and `.env` / `.env.*` files
3. Computes deterministic candidate per key index inside selected range
4. Probes forward until a free port is found
5. Exports or executes with overrides

## Documentation

- [Architecture](docs/ARCHITECTURE.md)
- [Examples](docs/EXAMPLES.md)
- [Contributing](CONTRIBUTING.md)

## CI/CD

- CI runs on every pull request and on pushes to `main`:
  - `gofmt` check
  - `go vet ./...`
  - `go test ./...`
  - `go test -tags e2e ./e2e -v`
  - `go build ./...`
- CD runs on tags matching `v*` (for example `v1.0.0`) and creates a GitHub Release with binaries for:
  - Linux (`amd64`, `arm64`)
  - macOS (`amd64`, `arm64`)
  - Windows (`amd64`)

## Project Layout

- `main.go`: CLI entrypoint and arg parsing
- `internal/app`: workflow orchestration
- `internal/scanner`: key discovery from env and files
- `internal/config`: preset loading/merging
- `internal/env`: `.env` key extraction
- `pkg/port`: range parsing, hashing, deterministic allocation
