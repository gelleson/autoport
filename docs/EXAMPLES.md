# Examples

## Start a service normally

```bash
autoport npm start
```

## Export into current shell

```bash
eval "$(autoport)"
```

## Use YAML for scripting

```bash
autoport -f yaml
```

## Explain why a key was or wasn't used

```bash
autoport explain
```

JSON explain payload:

```bash
autoport explain -f json
```

## Run diagnostics

```bash
autoport doctor
```

Machine-readable diagnostics:

```bash
autoport doctor -f json
```

## Use namespace for monorepo services

```bash
autoport --namespace api npm run dev
autoport --namespace worker npm run dev
```

## Use explicit include/exclude policy

```bash
autoport --include PORT --include WEB_PORT --exclude DB_PORT npm start
```

## Generate and consume lockfile

```bash
autoport lock
autoport --use-lock npm start
```

## Use custom preset from config

```json
{
  "version": 2,
  "presets": {
    "web": {
      "ignore_prefixes": ["AWS_", "STRIPE_"],
      "range": "8000-8099"
    }
  }
}
```

Run:

```bash
autoport -p web npm run dev
```

## CI usage with JSON

```bash
autoport -f json go test ./...
```

## Preview without executing command

```bash
autoport -n npm start
```
