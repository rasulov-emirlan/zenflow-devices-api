# Seeds

Reference data that is *not* part of the schema. Seed data lives under
`internal/seed/data/` so that the Go `//go:embed` directive can pick it up at
compile time. This directory holds operator-facing docs only; the canonical
data files live next to the seeder code.

## Sources

| Source name | File |
|-------------|------|
| `templates` | `internal/seed/data/templates.json` |

Edit the JSON, rebuild, and re-run the seeder.

## Applying

One-shot via CLI:

```bash
DATABASE_URL=postgres://... go run ./cmd/seed run --source=templates --on-conflict=skip
```

On boot (dev only):

```bash
APP_ENV=dev SEED_ON_BOOT=true go run ./cmd/api
```

`SEED_ON_BOOT=true` is rejected in `prod` at config load.

## On-conflict policy

- `skip` (default): leave existing rows untouched.
- `update`: overwrite by primary key (`slug`).
- `fail`: error if any row collides.
