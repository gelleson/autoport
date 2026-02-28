# Examples

## Start a service normally

```bash
autoport npm start
```

## Run Go service with deterministic ports

```bash
autoport go run ./cmd/api
```

## Export ports into current shell

```bash
eval "$(autoport)"
```

Then run tools that read environment variables:

```bash
docker compose up
```

## Use a custom range

```bash
autoport -r 7000-7099 npm run dev
```

## Ignore specific prefixes

Ignore database variables while still managing app ports:

```bash
autoport -i DB_ -i REDIS_ npm start
```

## Use built-in database preset

```bash
autoport -p db npm start
```

## Use custom preset from config

`~/.autoport.json` or `./.autoport.json`:

```json
{
  "presets": {
    "web": {
      "ignore": ["STRIPE_", "AWS_"],
      "range": "8000-8099"
    }
  }
}
```

Run:

```bash
autoport -p web npm run dev
```

## Multiple services in one machine

In each project directory, run your normal command through `autoport`:

```bash
# service-a
cd ~/work/service-a
autoport npm start

# service-b
cd ~/work/service-b
autoport go run .
```

Each service gets deterministic ports based on its own directory path.

## CI usage

```bash
autoport go test ./...
```

Useful when parallel jobs share a host and default ports might collide.
