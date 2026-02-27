# Contributing to autoport

Thank you for your interest in contributing! This document provides guidelines for contributing to `autoport`.

## Development Setup

1.  **Prerequisites**: Go 1.21 or later.
2.  **Clone the repository**:
    ```bash
    git clone https://github.com/gelleson/autoport.git
    cd autoport
    ```
3.  **Build**:
    ```bash
    go build -o autoport main.go
    ```

## Running Tests

We value thorough testing. Please ensure all tests pass before submitting a PR.

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

## Project Structure

- `main.go`: Entry point and CLI flag parsing.
- `internal/app/`: Core application logic and orchestration.
- `internal/config/`: Configuration loading and presets.
- `internal/env/`: Utilities for parsing `.env` files.
- `internal/scanner/`: Logic for discovering port-related environment variables.
- `pkg/port/`: Core port calculation, hashing, and validation logic.

## Pull Request Process

1.  Create a new branch for your feature or bug fix.
2.  Write clear, concise commit messages.
3.  Include tests for any new functionality.
4.  Update documentation if you change the API or add new features.
5.  Open a PR against the `main` branch.

## Code Style

We follow standard Go formatting and idioms. Please run `go fmt` and `go vet` on your code.

```bash
go fmt ./...
go vet ./...
```

## License

By contributing to `autoport`, you agree that your contributions will be licensed under the project's MIT License.
