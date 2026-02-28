# Migration Guide (v1 compatibility release)

This release introduces config v2 and new command surfaces while keeping one-cycle compatibility for common v1 config.

## Summary

- New config shape: `version: 2`
- Legacy preset field `ignore` is deprecated
- New replacement: `ignore_prefixes`
- New capabilities: `include_keys`, `exclude_keys`, `scanner.ignore_dirs`, `scanner.max_depth`, `strict`

## Field mapping

| Legacy | New | Behavior |
|---|---|---|
| `presets.<name>.ignore` | `presets.<name>.ignore_prefixes` | Auto-mapped in this release with warnings |

## Example conversion

### Before

```json
{
  "presets": {
    "web": {
      "ignore": ["AWS_", "STRIPE_"],
      "range": "8000-9000"
    }
  }
}
```

### After

```json
{
  "version": 2,
  "presets": {
    "web": {
      "ignore_prefixes": ["AWS_", "STRIPE_"],
      "range": "8000-9000"
    }
  }
}
```

## Strict mode

If `strict: true` is enabled:
- unknown presets become fatal errors
- malformed/unsupported config is fatal

## New workflows

- `autoport explain` for traceable decision output
- `autoport doctor` for diagnostics and exit code health checks
- `autoport lock` + `--use-lock` for reproducible assignments

## Deprecation window

Legacy compatibility is intended for this release cycle only. A following release may remove deprecated aliases and legacy field handling.
