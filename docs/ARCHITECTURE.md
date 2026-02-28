# Architecture

`autoport` is a stateless CLI that deterministically assigns free ports and can also explain/diagnose/runtime-lock those decisions.

## Design goals

- Stateless by default: no daemon or runtime registry
- Deterministic: same cwd + namespace + inputs -> same preferred candidates
- Transparent: explainable decisions and diagnostics
- Reproducible when needed: optional lockfile workflow

## Runtime flow

```text
CLI args -> parse flags/subcommand
        -> load+merge config (home then project)
        -> resolve presets/filters/range/seed
        -> scan env + .env files (with stats/sources)
        -> apply include/exclude/manual key policy
        -> assign ports (dynamic allocator or lockfile)
        -> render output / execute command / write lockfile
```

## Components

### `main.go`
- Parses global flags + subcommands (`run`, `explain`, `doctor`, `lock`, `version`)
- Maps doctor-specific exit codes through `app.ExitError`

### `internal/app`
- Central orchestration
- Resolves effective policy from CLI + config + presets
- Applies deterministic seed precedence:
  - `--seed` > hash(`cwd|namespace`) > hash(`cwd`)
- Executes mode-specific behavior:
  - run/export
  - explain
  - doctor
  - lockfile write

### `internal/scanner`
- Reads process environment
- Walks project tree for `.env` / `.env.*`
- Skips hidden dirs by default
- Supports `scanner.ignore_dirs` and `scanner.max_depth`
- Produces source-aware discoveries and scan stats

### `internal/config`
- Loads JSON config from home and project
- Merges later files over earlier files
- Supports v2 schema and strict mode
- Maps legacy v1 `ignore` to `ignore_prefixes` with warnings

### `internal/lockfile`
- Reads/writes `.autoport.lock.json`
- Validates lockfile version
- Uses cwd fingerprint for compatibility checks

### `pkg/port`
- `ParseRange`: validates syntax and bounds
- `SeedFor`: deterministic seed for path + namespace
- `Allocator.PortForWithStats`: preferred + probe-aware assignment

## Selection model

Key selection is decided in this order:
1. Scanner discovers port-shaped keys (`PORT`, `*_PORT`) from env and files.
2. Prefix ignores are applied during scan.
3. Exact excludes are applied.
4. Exact includes (if provided) become an allow-list.
5. Manual `-k` keys are always included.

## Error/exit model

- Invalid config parse/version: fatal
- Unknown preset:
  - warning in non-strict mode
  - fatal in strict mode
- Doctor exit codes:
  - `0` healthy
  - `1` warnings
  - `2` fatal

## Test strategy

- Unit tests for config migration, scanner policy, allocator behavior, lockfile schema
- App-level tests for run/explain/doctor/lock orchestration
- E2E tests for CLI behavior, namespace determinism, lockfile usage, and scanner controls
