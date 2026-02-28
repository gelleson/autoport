# Architecture

`autoport` is a small stateless CLI that deterministically assigns free ports.

## Design goals

- Stateless: no registry or persistent runtime state
- Deterministic: same path, same preferred ports
- Transparent: behaves as a thin wrapper around your existing command

## Runtime flow

```text
CLI args -> parse flags -> resolve presets/range/ignores
        -> scan env + .env files for port keys
        -> allocate deterministic free ports
        -> print exports OR execute subcommand with env overrides
```

## Components

### `main.go`
- Parses CLI flags into `app.Options`
- Delegates execution to `app.Run`

### `internal/app`
- Resolves effective ignores and range from flags + presets
- Invokes scanner to discover keys
- Uses `pkg/port.Allocator` to assign ports
- Either prints sorted exports or executes subcommand

### `internal/scanner`
- Reads current process environment
- Walks project tree for `.env` and `.env.*`
- Skips hidden directories
- Selects keys matching `PORT` or `*_PORT`
- Applies ignore prefixes

### `internal/config`
- Loads JSON config from home and project
- Merges preset maps with later files overriding earlier entries

### `internal/env`
- Extracts port-related keys from `.env`-style content

### `pkg/port`
- `ParseRange`: validates `start-end` syntax
- `HashPath`: generates deterministic seed from absolute path
- `Allocator.PortFor(index)`: chooses and probes deterministic ports

## Deterministic allocation

For each discovered key at position `index`:

```text
home = start + (seed + index) % size
```

If `home` is in use, allocation probes sequentially until a free port is found.
If none are free, allocation returns an error.

## Error model

- Invalid range format or bounds -> immediate CLI error
- Scanner cancellation (`context canceled` / timeout) -> propagated
- No free ports in range -> propagated with key context
- Missing/invalid preset files -> ignored
- Unknown preset name -> warning log

## Test strategy

- Unit tests per package
- App-level orchestration tests with mock executor
- Scanner tests for env files, ignores, hidden-dir behavior, cancellation
- Port tests for range parsing and allocator probing behavior
