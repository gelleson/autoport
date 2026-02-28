# autoport

`autoport` is a deterministic port wrapper for local development.

It discovers `PORT` and `*_PORT` environment keys, assigns free ports based on your project path (and optional namespace), and can:
- run your command with overrides,
- print exports for your shell,
- explain how assignments were produced,
- diagnose configuration/environment issues,
- and persist/consume lockfiles.

## Why use it

- Prevents port collisions across multiple local projects
- Keeps assignment stable for the same project path + namespace
- Works with existing workflows (`npm`, `go run`, Docker, test commands)
- Supports opt-in reproducibility via lockfile

## Installation

```bash
go install github.com/gelleson/autoport@latest
```

Install the local development checkout:

```bash
go install .
```

Single-line installer (macOS/Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/gelleson/autoport/main/scripts/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/gelleson/autoport/main/scripts/install.ps1 | iex
```

Using `just` tasks:

```bash
just install      # latest release from module path
just install-dev  # local checkout -> /usr/local/bin/autoport (version=dev, build time embedded)
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

Explain key discovery and allocation:

```bash
autoport explain
```

Run health checks:

```bash
autoport doctor
```

Create and consume lockfile:

```bash
autoport lock
autoport --use-lock npm start
```

## CLI

```text
autoport [flags] [command ...]
autoport explain [flags]
autoport doctor [flags]
autoport lock [flags]
autoport version
```

Selection flags:
- `-r <start-end>`: Port range (default: `10000-20000`)
- `-p <name>`: Preset name (repeatable)
- `-i <prefix>`: Ignore env keys starting with prefix (repeatable)
- `--include <env_key>`: Include exact key (repeatable)
- `--exclude <env_key>`: Exclude exact key (repeatable)
- `-k <env_key>`: Include a port env key manually (repeatable)

Execution flags:
- `-q, -quiet`: Suppress command-mode override summary
- `-n, -dry-run`: Preview overrides without executing
- `--namespace <name>`: Namespace salt for deterministic seed
- `--seed <uint32>`: Explicit deterministic seed
- `--use-lock`: Use `.autoport.lock.json` assignments (opt-in)

Formats:
- Run/export mode (`autoport`): `-f shell|json|dotenv|yaml` (default: `shell`)
- Explain/doctor modes: `-f text|json` (default: `text`)

## Commands

### `autoport` (run/export)
- With `command`: executes command with port overrides in process env
- Without `command`: prints exports in selected format
- With `-n`: prints preview and exits without running command

### `autoport explain`
Shows:
- effective inputs (range/presets/filters/seed),
- discovered keys and source (`env`, `.env`, `.env.local`, `default`, `manual`),
- inclusion/exclusion decisions,
- final assignments (`preferred`, `assigned`, `probes`).

### `autoport doctor`
Runs diagnostics for:
- config parse/compat,
- unknown preset behavior,
- range sanity,
- scan stats,
- sampled port availability,
- lockfile compatibility.

Exit codes:
- `0` healthy
- `1` warnings only
- `2` fatal issues

### `autoport lock`
Writes `.autoport.lock.json` with:
- `version`
- `cwd_fingerprint`
- `range`
- `assignments`
- `created_at`

## Configuration

`autoport` loads presets from:
1. `~/.autoport.json`
2. `./.autoport.json` (overrides home config)

### v2 schema

```json
{
  "version": 2,
  "strict": false,
  "scanner": {
    "ignore_dirs": ["node_modules", "vendor"],
    "max_depth": 4
  },
  "presets": {
    "web": {
      "range": "8000-9000",
      "ignore_prefixes": ["AWS_", "STRIPE_"],
      "include_keys": ["PORT", "WEB_PORT"],
      "exclude_keys": ["DB_PORT"]
    }
  }
}
```

Built-in presets:
- `db`: ignores database-style prefixes (`DB`, `DATABASE`, `POSTGRES`, `MYSQL`, `MONGO`, `REDIS`, `MEMCACHED`, `ES`, `CLICKHOUSE`, `INFLUX`)
- `queues`: excludes common broker ports (`RABBITMQ_PORT`, `AMQP_PORT`, `NATS_PORT`, `KAFKA_PORT`, `PULSAR_PORT`, `ACTIVEMQ_PORT`, `ARTEMIS_PORT`, `SQS_PORT`, `NSQ_PORT`, `RSMQ_PORT`, `BEANSTALKD_PORT`)

### Migration compatibility

Legacy v1 preset field `ignore` is still accepted in this release and auto-mapped to `ignore_prefixes` with warnings.
See [Migration Guide](docs/MIGRATION_V1.md).

## Documentation

- [Architecture](docs/ARCHITECTURE.md)
- [Examples](docs/EXAMPLES.md)
- [Migration Guide](docs/MIGRATION_V1.md)
- [Contributing](CONTRIBUTING.md)

## CI/CD

- CI runs on pull requests and pushes to `main`:
  - `gofmt` check
  - `go vet ./...`
  - `go test ./...`
  - `go test -tags e2e ./e2e -v`
  - `go build ./...`
- CD runs on tags matching `v*` and publishes binaries for Linux/macOS/Windows.

## Project Layout

- `main.go`: CLI parsing and process exit behavior
- `internal/app`: orchestration for run/explain/doctor/lock
- `internal/scanner`: key discovery + scan stats + source tracking
- `internal/config`: v2 config loading, merging, migration warnings
- `internal/lockfile`: lockfile read/write/fingerprint
- `pkg/port`: range parsing, seed derivation, deterministic allocation
