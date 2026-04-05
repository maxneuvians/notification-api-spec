## Context

The notification-api is a Python 3 / Flask / Celery / SQLAlchemy application running in production. The Go rewrite produces a single Go module compiled to two binary entry points (`cmd/api`, `cmd/worker`) that share all `internal/` packages. This change bootstraps the skeleton: the correct directory layout, build toolchain decisions, base infrastructure libraries, non-domain-specific packages, and the seed migration. All 19 subsequent changes layer domain logic on top of what is built here, meaning any design error in the project layout or shared packages propagates to every downstream change.

No Go code currently exists. The module path established here (`go.mod`) is the import root for all subsequent packages. The seed migration (`db/migrations/0001_initial.sql`) is the only migration file at this stage; every domain change adds its own numbered migration files.

## Goals / Non-Goals

**Goals:**
- Create the Go module (`go.mod`) establishing the canonical module path for all 19 subsequent changes
- Implement the full directory structure from `go-architecture.md`: `cmd/api/`, `cmd/worker/`, `db/migrations/`, `db/queries/`, `internal/handler/`, `internal/service/`, `internal/repository/`, `internal/middleware/`, `internal/worker/`, `internal/client/`, `internal/config/`, `queue/`, `pkg/crypto/`, `pkg/smsutil/`, `pkg/emailutil/`, `pkg/pagination/`, `pkg/signing/`
- Wire the chi router at `cmd/api/main.go` with the full non-auth middleware stack in exact order: RequestID → OTEL → logging → CORS → rate limiting → request size limiting
- Implement the 4 health/status endpoints (`GET /`, `GET /_status`, `POST /_status`, `GET /_status/live-service-and-organisation-counts`) with no authentication
- Implement `internal/config/config.go`: flat `Config` struct, all fields from env vars, `Load()` returns error if any required variable is absent, process exits before binding port
- Set up `sqlc.yaml` with all type overrides; seed migration `db/migrations/0001_initial.sql` derived from `spec/out.sql`
- Both `cmd/api/main.go` and `cmd/worker/main.go` call `runMigrations(db)` on startup using `golang-migrate`
- Implement `pkg/crypto`: `Encrypt`/`Decrypt` replacing `app/encryption.py`, with multi-key rotation support
- Implement `pkg/signing`: `Sign`/`Unsign`/`Dumps`/`Loads` compatible with Python itsdangerous HMAC-SHA256 format
- Implement `queue/consumer.go` and `queue/producer.go` SQS long-poll and send abstractions
- Implement `WorkerManager` stub (`internal/worker/manager.go`) that compiles and integrates with `cmd/worker/main.go`
- Provide `Makefile` with `build`, `test`, `lint`, `sqlc`, `migrate-up`, `migrate-down` targets
- Define HTTP error response format sentinel types in `internal/handler/errors.go`

**Non-Goals:**
- Authentication middleware (separate change: `authentication-middleware`)
- Any domain handler, service, or repository beyond the health endpoint
- Worker pool goroutines (separate change: `notification-delivery-pipeline`)
- External API client packages: SES, SNS, Pinpoint, S3, Airtable, Salesforce, Freshdesk, Redis (each domain change or external-client-integrations adds these)
- Database query files (each domain change adds its own under `db/queries/`)
- Per-service Redis rate limiting (domain changes add this where needed)

## Decisions

### D1 — chi over gorilla/mux
chi is stdlib-compatible (`net/http`), actively maintained, and its `Router.Group()` / `Mount()` model maps directly onto Flask's blueprint prefix pattern (e.g. `/service` → `handler/services`, `/v2/notifications` → `handler/v2/notifications`). gorilla/mux is in maintenance mode. Middleware composition at the group level (e.g. all `/service/{id}` routes share admin JWT middleware) is cleaner with chi.

### D2 — sqlc over GORM/ent
sqlc generates type-safe Go structs from hand-written SQL. This matches the requirement for explicit query control — the Python app uses SQLAlchemy with many raw SQL expressions. No ORM magic, no N+1 surprises from lazy-loaded relationships, and generated code is auditable. ent requires a schema DSL that diverges from the existing 68-table SQL schema and would require a parallel schema definition.

### D3 — golang-migrate over goose
`golang-migrate` is specified in `go-architecture.md`. It supports both CLI and in-process Go API, enabling startup auto-migration. Migration files are plain `.sql` — no Go code in migrations, matching the spirit of sqlc's explicit-SQL philosophy.

### D4 — Single repo, two binary entry points (`cmd/api`, `cmd/worker`)
`cmd/api` and `cmd/worker` share all `internal/` packages via the same module. They may be compiled as separate binaries or as one binary with subcommands — this is a CI/CD deployment decision and does not affect the package design. The project structure supports both without code changes.

### D5 — Flat `Config` struct loaded from env; no Viper or YAML
A single flat struct populated via env-var parsing (e.g. `github.com/caarlos0/env/v11`). The Python app is 100% env-var-driven; this pattern requires no config file management in deployment. A startup check (`Config.Load()`) asserts all required variables are present before the process binds any port or connects to any database.

### D6 — `pkg/crypto` is explicit service-layer calls; no ORM hooks
Python uses SQLAlchemy column-type wrappers to transparently encrypt/decrypt. Go has no ORM to intercept. Encryption and decryption are explicit calls in the service layer, consistent with sqlc's philosophy and the no-ORM constraint. Repository functions accept and return raw ciphertext; the service layer is responsible for calling `crypto.Encrypt`/`crypto.Decrypt`. SecretKey is a list — encryption uses `SecretKey[0]`; decryption tries all keys for rotation support.

### D7 — `pkg/signing` uses itsdangerous-compatible HMAC-SHA256
Notification callbacks and signed blobs that interoperate with the Python `itsdangerous` library during the transition period must use the same signing format. `pkg/signing` implements HMAC-SHA256 with the same serialisation scheme so tokens generated by Go are verifiable by Python and vice versa. A round-trip test (Go-signed → Python-verify, Python-signed → Go-verify) is required.

### D8 — SQS abstractions in `queue/` package (not `internal/`)
`queue/Consumer` and `queue/Producer` are in `queue/` (not `internal/`) to make the boundary explicit: these are infrastructure primitives that could theoretically be used by tooling outside the main application. Worker implementations in `internal/worker/` depend on `queue/`, not the reverse.

### D9 — WorkerManager stub only; full implementation deferred
The full goroutine pool implementation (15+ SQS consumer groups + cron scheduler) is out-of-scope. This change delivers a compilable `WorkerManager` stub with `Start(ctx) error` and `Stop()` methods so `cmd/worker/main.go` compiles and integrates cleanly. The real implementation is delivered in `notification-delivery-pipeline`.

### D10 — Two wire error formats preserved exactly
The admin API (non-/v2/) uses `{"result": "error", "message": ...}`. The public v2 API uses `{"status_code": N, "errors": [...]}`. Both formats must be preserved because the admin UI and v2 API clients depend on them. Sentinel error types in `internal/handler/errors.go` centralise format selection.

## Risks / Trade-offs

- **sqlc schema drift** → `sqlc generate` runs in CI (`make sqlc`) — build fails if queries are inconsistent with the migration schema. Pinning the sqlc binary version in `Makefile` prevents unexpected output changes.
- **Seed migration size** → `0001_initial.sql` derived from `spec/out.sql` covers 68 tables. golang-migrate handles large migrations in one transaction; no sharding needed.
- **itsdangerous compatibility** → `pkg/signing` must produce byte-for-byte identical MAC values to the Python library for existing signed tokens to remain valid. Required: a round-trip integration test using a secret and token pair generated by the Python app.
- **Environment variable sprawl** → 50+ configuration variables. The flat `Config` struct with `Load()` makes all required variables enumerable and verifiable at startup; missing variables are caught before any traffic is accepted.
- **startup migration on heavy load** → Running `golang-migrate` as a blocking call before accepting traffic is safe for sequential pod restarts. If multiple pods start simultaneously against a migration that hasn't been applied, only one will succeed; others will error and restart. This is acceptable and expected behaviour for golang-migrate's postgres advisory lock.

## Migration Plan

This is a new repository. No existing system is modified.

1. Create repository and commit project scaffold (`go.mod`, directory structure, empty packages)
2. Implement `internal/config/config.go` and run `go build ./...` to verify the module structure
3. Implement `pkg/crypto` and `pkg/signing`; run `make test` with round-trip tests
4. Implement chi router with middleware stack and health endpoints; verify `GET /_status` returns 200
5. Add `db/migrations/0001_initial.sql` (seed from `spec/out.sql`); run `make migrate-up` against a test DB
6. Confirm `sqlc.yaml` and `make sqlc` work with empty `db/queries/` directory (no errors expected)
7. Implement `queue/consumer.go`, `queue/producer.go`, `WorkerManager` stub; run `go build ./cmd/worker`
8. Subsequent changes build on this foundation — no rollback procedure needed for the initial setup
