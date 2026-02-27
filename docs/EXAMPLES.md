# Examples

This document provides advanced usage examples for `autoport`.

## 1. Multi-Service Development

If you are running multiple microservices locally, `autoport` ensures they don't collide without you having to manually manage a spreadsheet of ports.

```bash
# In service-a directory
autoport go run main.go

# In service-b directory
autoport npm start
```

Each service will get a deterministic port based on its path.

## 2. Using with Docker Compose

You can use `autoport` to dynamically assign ports to services in a Docker Compose setup by passing them as environment variables.

```bash
# Set ports for the current session
eval $(autoport)

# Run docker-compose
docker-compose up
```

Ensure your `docker-compose.yml` uses the variables:
```yaml
services:
  web:
    build: .
    ports:
      - "${PORT}:${PORT}"
```

## 3. CI/CD Pipelines

In CI environments where multiple builds might run on the same agent, `autoport` helps avoid collisions during integration tests.

```bash
# Run tests with a random but consistent port based on the build directory
autoport go test ./...
```

## 4. Custom Range for Specific Projects

If you have a project that requires ports in the 8000 range:

```bash
autoport -r 8000-8100 npm start
```

## 5. Ignoring Specific Variables

If your application uses `REDIS_PORT` and you want to keep it at the default `6379`, use the ignore flag:

```bash
autoport -i REDIS_ npm start
```

Or use the built-in database preset:

```bash
autoport -p db npm start
```

## 6. Shell Integration

Add an alias to your `.zshrc` or `.bashrc` for even faster usage:

```bash
alias ap='autoport'
```

Then run:
```bash
ap npm start
```
