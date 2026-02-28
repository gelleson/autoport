# Architecture

`autoport` is designed to be a simple, deterministic, and stateless tool for managing port collisions. This document outlines its internal design and key components.

## Core Principles

1.  **Statelessness**: No local database or registry is required. Port assignment is derived from the project's directory path.
2.  **Determinism**: The same project will always attempt to use the same ports, facilitating consistent local development environments.
3.  **Transparency**: It should behave like a thin wrapper around existing CLI tools.

## Component Overview

### 1. Port Discovery (`internal/scanner`)
The scanner is responsible for identifying which environment variables need to be reassigned.
- It scans the current process environment.
- It walks the directory tree (ignoring hidden directories) to find `.env` and `.env.*` files.
- It extracts keys that match `PORT` or end with `_PORT`.

### 2. Hashing Mechanism (`pkg/port`)
To achieve determinism, `autoport` hashes the absolute path of the current working directory using the FNV-1a 32-bit algorithm. This hash serves as the seed for port selection.

### 3. Port Selection Strategy (`pkg/port`)
For each discovered key, `autoport` calculates a "home" port within the configured range:
```
port = start + (hash + index) % range_size
```
- **Collision Handling**: If the target port is already in use, it probes the next port in the range sequentially until a free one is found.
- **IsFree Check**: It attempts to open a TCP listener on the port to verify its availability.
- **Allocator API**: `pkg/port` exposes `Range` and `Allocator` types so callers can parse once and allocate many deterministic ports cleanly.

### 4. Configuration and Presets (`internal/config`)
Presets allow users to ignore specific environment variables (like `DB_PORT`) that shouldn't be managed by `autoport`.
- **Built-in Presets**: Common database prefixes are ignored by default when using `-p db`.
- **Custom Presets**: Users can define their own presets in `~/.autoport.json` or a local `.autoport.json`.

### 5. Application Lifecycle (`internal/app`)
The `App` struct orchestrates the entire workflow:
1.  Loads configuration and merges presets.
2.  Initializes the scanner and finds port keys.
3.  Calculates and validates free ports.
4.  If a command is provided, it spawns a sub-process with updated environment variables.
5.  If no command is provided, it prints shell export statements.

## Data Flow

```
User Command -> main.go -> app.Run()
                   |
                   v
           config.Load() (Presets)
                   |
                   v
           scanner.Scan() (Environment & .env files)
                   |
                   v
           port.Allocator.PortFor() (Hashing & Probing)
                   |
                   v
           executor.Run() (Sub-process execution)
```

`main.go` is intentionally thin and delegates argument parsing to a dedicated parser function before handing off to `app.Run`.
