# zenflow-devices-api

A maintainable MVP REST API for managing **device profiles** used by a web-scraping service.
Each profile records `device_type`, viewport, user-agent, proxy country, and custom headers.
Users authenticate with HTTP Basic, may build profiles from scratch or from predefined
templates, and only see their own profiles.

Built for the ZenRows Go technical assessment.

---

## Quickstart

### Docker (recommended)

```bash
docker compose up --build
# api on http://localhost:8080, postgres on :5432
```

A seeded user `alice` / `secret` is configured in `docker-compose.yml`.

```bash
# health
curl http://localhost:8080/healthz

# list seeded templates
curl -u alice:secret http://localhost:8080/templates

# create a profile from scratch
curl -u alice:secret -X POST http://localhost:8080/profiles \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "my-desktop",
    "device_type": "desktop",
    "window_width": 1920,
    "window_height": 1080,
    "user_agent": "Mozilla/5.0",
    "country_code": "US",
    "custom_headers": [{"key":"Accept-Language","value":"en-US"}]
  }'

# create from a template (overrides win)
curl -u alice:secret -X POST http://localhost:8080/profiles \
  -H 'Content-Type: application/json' \
  -d '{"name":"phone-br","template_slug":"mobile-iphone-us","country_code":"BR"}'

# list own profiles
curl -u alice:secret http://localhost:8080/profiles

# patch
curl -u alice:secret -X PATCH http://localhost:8080/profiles/<id> \
  -H 'Content-Type: application/json' -d '{"window_width":800}'

# delete
curl -u alice:secret -X DELETE http://localhost:8080/profiles/<id>
```

### Local (with Postgres running somewhere)

```bash
export DATABASE_URL='postgres://zenflow:zenflow@localhost:5432/zenflow?sslmode=disable'
export BASIC_AUTH_USERS="alice:$(htpasswd -bnBC 10 '' secret | tr -d ':\n')"
export PORT=8080
go run ./cmd/api
```

### Tests

```bash
make test                               # unit tests (fast, no deps)
go test -tags=integration ./test/...    # requires Docker — spins up Postgres via testcontainers
```

---

## Endpoints

All endpoints except `/healthz` require HTTP Basic credentials.

| Method | Path                | Purpose                                        |
|--------|---------------------|------------------------------------------------|
| GET    | `/healthz`          | Liveness (unauthenticated)                     |
| POST   | `/profiles`         | Create (optionally `template_slug` to prefill) |
| GET    | `/profiles`         | List caller's profiles (`?limit`, `?offset`)   |
| GET    | `/profiles/{id}`    | Get one (404 if not owned — no existence leak) |
| PATCH  | `/profiles/{id}`    | Partial update                                 |
| DELETE | `/profiles/{id}`    | Delete                                         |
| GET    | `/templates`        | List predefined templates                      |
| GET    | `/templates/{slug}` | Get one template                               |

Errors use `{"error":"<code>","message":"..."}`.
Statuses: `400` invalid input · `401` unauthorized · `404` not found · `409` duplicate name · `500` unexpected.

---

## Architecture

The codebase follows a lightweight **ports-and-adapters** (hexagonal) layout.
Domain code imports neither `net/http` nor `pgx` — both are adapters that plug
into interfaces the domain owns.

```
cmd/api            → thin entry point
internal/app       → bootstrap: App struct with init steps, LIFO cleanups
internal/config    → env parsing
internal/auth      → transport-agnostic credential resolver (bcrypt)
internal/domains/  → PURE: profiles, templates (models, errors, Repo interfaces, services)
internal/transport → adapters grouped by kind
  └─ httprest/     → chi router, middleware, handlers, DTOs, error mapping
internal/storage/  → adapters grouped by kind
  ├─ postgresql/   → pgx pool, migrations (embedded), row DTOs, pg-error translation
  └─ memory/       → in-memory reference impl (used in tests)
pkg/               → project-agnostic utilities
  ├─ logging/      → slog setup
  └─ httpx/        → generic JSON/error helpers
migrations exist at internal/storage/postgresql/migrations/ and are embedded.
```

### Dependency direction (enforceable with `go list -deps`)

```
cmd/api  →  internal/app  →  { config, domains/*, transport/httprest, storage/* }
transport/httprest  →  domains/*
storage/postgresql  →  domains/*
domains/*           →  (stdlib + pkg/ only)   ← the critical rule
```

Verify: `go list -deps ./internal/domains/... | grep -E 'net/http|pgx|chi'` should be empty.

---

## Key design decisions

### Why Postgres (with a JSONB escape hatch)
The spec called out that the schema must “support future changes gracefully
without breaking existing profiles.” Postgres gives that via **typed core columns
+ `custom_headers` / `extra` JSONB columns**: strict validation where the shape
is stable, document flexibility where it isn’t. When a JSONB field stabilizes,
promote it to a typed column with a backfill migration. Mongo was a defensible
alternative, but JSONB dissolves its main advantage here while keeping the
`UNIQUE (user_id, name)` and FK constraints we actually want.

### Why hexagonal, and why adapter-kind folders
Transport- and storage-agnosticism were explicit requirements. Grouping adapters
by kind (`transport/httprest`, `storage/postgresql`) rather than by domain keeps
cross-cutting concerns (middleware, row scanning, error translation) in one place
and makes adding a new domain to an existing adapter a **file, not a subtree**.
Adding gRPC = new `internal/transport/grpc/`; domain untouched.

### Why HTTP Basic Auth
The spec asked for a simple mechanism that restricts access to each user’s own
profiles — not full user management. Basic auth with a server-side
`user → bcrypt(hash)` map from env config is the smallest thing that meets that.
The `auth.Resolver` is HTTP-agnostic; only the middleware lives in the transport
package. Swapping to JWT/OAuth later is additive.

### Why `chi`, `pgx`, and hand-rolled validation
- **chi** — `net/http`-native router with idiomatic middleware. Stdlib `http.ServeMux`
  (1.22+) was considered; chi won on URL params and ergonomics.
- **pgx/v5** — best-in-class Postgres driver, native JSONB, no ORM overhead.
- **hand-rolled validation** — the ruleset is small (6 rules) and lives where it
  matters: on the domain object (`Profile.Validate()`). Avoids a heavy validator
  dep, keeps error messages explicit, and domain tests exercise it directly.
- **golang-migrate** with `iofs` source — migrations embedded into the binary via
  `go:embed`, so deploys don’t need a separate migrations copy.
- **testify** was considered but not adopted — stdlib `testing` sufficed.
- **testcontainers-go** for integration — self-contained `go test` with a real
  Postgres; no CI-specific shim required.

### Bootstrap (`internal/app`)
The `App` struct owns process lifecycle. `Init` runs `initConfig → initLogger →
initDB → initDomains → initHTTP` sequentially with explicit error wrapping.
Each step may register a **cleanup function** via `addCleanup`; `Shutdown` runs
them in **LIFO** order. This keeps teardown logic next to the setup that
produced the resource, and avoids a DI framework at this scale.

### Ownership & error model
Ownership is enforced at the repo level (`WHERE user_id = ? AND id = ?`). Missing
rows return `ErrNotFound` whether the profile doesn’t exist or belongs to another
user — deliberate, to avoid leaking existence. Domain errors
(`ErrNotFound`, `ErrDuplicateName`, `ErrInvalidInput`, `ErrTemplate`) are the
currency between layers; each adapter translates at its edge (pg `23505` →
`ErrDuplicateName` → HTTP 409).

---

## Configuration

| Env var             | Default | Notes |
|---------------------|---------|-------|
| `PORT`              | `8080`  | HTTP listen port |
| `DATABASE_URL`      | —       | required, pgx-compatible |
| `LOG_LEVEL`         | `info`  | `debug` / `info` / `warn` / `error` |
| `BASIC_AUTH_USERS`  | —       | required, `user1:bcrypt_hash1,user2:bcrypt_hash2` |

Generate a bcrypt hash:
```bash
htpasswd -bnBC 10 "" secret | tr -d ':\n'
```

---

## Testing approach

- **Unit (`internal/domains/profiles/service_test.go`)** — the service is tested
  against an inline fake repo. Covers: validation, template merge + overrides,
  ownership enforcement, duplicate-name rejection, patch merge + revalidation.
  Pure Go, no Docker.
- **Integration (`test/integration_test.go`, tag `integration`)** — spins up
  Postgres via `testcontainers-go/modules/postgres`, runs migrations, wires the
  real HTTP stack, and exercises `POST /profiles` happy path + 401 + 409 + 400 +
  cross-user 404 + template creation. Asserts the seed templates are present.

---

## What I'd do next given more time

- **OpenAPI / Swagger** spec generated from handler tags; Swagger UI for browsing.
- **Rate limiting** per-user (token bucket) via middleware.
- **OAuth/JWT** replacing Basic auth — isolated to `internal/auth` + a new middleware.
- **Cursor pagination** instead of `limit/offset` once profile counts grow.
- **Structured audit log** of write operations (who/what/when).
- **Template catalog expansion** + a `/profiles/{id}/validate` endpoint that
  simulates a request shape before persistence.
- **CI** — GitHub Actions matrix: `go test ./...` + `go test -tags=integration`.
- **Profile versioning** — `schema_version` on JSONB so migrations can safely
  rewrite older shapes.
