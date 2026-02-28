# Architecture

`autoport` is a stateless CLI that deterministically assigns free ports and can also explain/diagnose/runtime-lock those decisions.

## Design goals

- Stateless by default: no daemon or runtime registry
- Deterministic: same cwd + namespace + branch + inputs -> same preferred candidates
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
        -> resolve cross-repo URL links (-e/config links)
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
  - `--seed` > hash(`cwd|namespace|branch`) > hash(`cwd|namespace`) > hash(`cwd`)
- Executes mode-specific behavior:
  - run/export
  - explain
  - doctor
  - lockfile write
  - link rewrites (loopback URL values)

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
- Supports `links` rewrite rules (`source_key`, `target_repo`, optional `target_port_key`, `target_namespace`, `same_branch`)

### `internal/lockfile`
- Reads/writes `.autoport.lock.json`
- Validates lockfile version
- Uses cwd fingerprint for compatibility checks

### `pkg/port`
- `ParseRange`: validates syntax and bounds
- `SeedFor`: deterministic seed for path + namespace
- `Allocator.PortForWithStats`: preferred + probe-aware assignment

### `internal/linkspec`
- Parses `-e/--target-env` specs
- Supports smart and explicit modes

### `internal/gitbranch`
- Resolves current branch name from git for branch-aware seed/link checks

## Selection model

Key selection is decided in this order:
1. Scanner discovers port-shaped keys (`PORT`, `*_PORT`) from env and files (case-insensitive).
2. Prefix ignores are applied during scan.
3. Exact excludes are applied.
4. Exact includes (if provided) become an allow-list.
5. Manual `-k` keys are always included.

## Link rewrite model

Rule precedence:
1. Explicit `-e SOURCE_KEY=...` specs
2. Config `links`
3. Smart `-e <env-file>` inference

Resolution details:
- Rewrites apply only to loopback URLs (`localhost`, `127.0.0.1`).
- Target key defaults to `APP_PORT` then `PORT` (case-insensitive) when not provided.
- Target port source-of-truth:
  - lockfile assignment if available,
  - otherwise deterministic preferred port (no free-port probing).
- If `same_branch=true`, source and target branches must match.
- Unresolved links are warnings and keep original value.

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
- E2E tests for CLI behavior, namespace/branch determinism, lockfile usage, scanner controls, and link rewrites
