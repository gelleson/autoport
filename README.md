# autoport

`autoport` is a lightweight CLI tool that helps you manage port collisions by deterministically assigning free ports based on your project's directory path.

## Features

- **Deterministic Port Selection**: The same project directory will always target the same set of ports.
- **Collision Avoidance**: If a target port is in use, it automatically finds the next available one.
- **Automatic Discovery**: Scans your environment variables and `.env` files for `PORT` and `*_PORT` keys.
- **Presets**: Built-in and custom presets to ignore specific environment variables or set default ranges.
- **Transparent Execution**: Seamlessly wraps your existing commands with the calculated environment variables.

## Installation

```bash
go install github.com/gelleson/autoport@latest
```

## Usage

### Run a command with auto-assigned ports

```bash
autoport npm start
```

If your project has `PORT=3000` in a `.env` file, `autoport` will override it with a deterministic port (e.g., `14253`) and run your command.

### Export ports to the current shell

If no command is provided, `autoport` prints the export statements:

```bash
eval $(autoport)
```

### Options

- `-r <range>`: Specify a port range (e.g., `3000-4000`). Default is `10000-20000`.
- `-p <preset>`: Apply a preset (e.g., `-p db`).
- `-i <prefix>`: Ignore environment variables starting with this prefix (can be used multiple times).

## Configuration

### Presets

Presets allow you to quickly apply common configurations. 

#### Built-in Presets:
- `db`: Ignores common database port keys like `DB`, `DATABASE`, `POSTGRES`, `MYSQL`, `MONGO`, `REDIS`, `MEMCACHED`, `ES`, `CLICKHOUSE`, `INFLUX` prefixes, to prevent them from being accidentally reassigned.

#### Custom Presets:
You can define custom presets in `~/.autoport.json` or a local `.autoport.json`:

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

## Documentation

- [Architecture](docs/ARCHITECTURE.md) - Deep dive into how it works.
- [Examples](docs/EXAMPLES.md) - Advanced usage and integration patterns.
- [Contributing](CONTRIBUTING.md) - How to help improve the project.

## Project Structure

- `main.go`: CLI entry point.
- `internal/app/`: Application orchestration.
- `internal/config/`: Configuration and presets.
- `internal/scanner/`: Discovery of port environment variables.
- `pkg/port/`: Hashing and port assignment logic.

## How it works

1. **Hashing**: It generates a 32-bit hash of your current working directory's absolute path.
2. **Discovery**: It scans the environment and local `.env` files for any keys ending in `_PORT` or exactly `PORT`.
3. **Assignment**: For each key found, it uses the hash and the key's index to pick a "home" port within the specified range.
4. **Validation**: It checks if the port is free. If not, it probes the range until a free one is found.
5. **Execution**: It injects these ports into the environment and spawns your sub-process.
