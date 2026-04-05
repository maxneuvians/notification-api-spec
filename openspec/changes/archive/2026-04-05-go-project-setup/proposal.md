## Why

The notification-api is currently a Python/Flask/Celery application. This change bootstraps the Go rewrite: establishing the project layout, build toolchain, configuration system, and base infrastructure so all subsequent domain changes have a working foundation to build on.

## What Changes

- New Go module (`go.mod`) with core dependencies: `chi`, `sqlc`, `golang-migrate`, `google/uuid`, `go-chi/chi`, `opentelemetry`, `robfig/cron`, `golang-migrate/migrate`
- `cmd/api/main.go` — HTTP server entry point: chi router, middleware stack, run migrations on startup
- `cmd/worker/main.go` — Worker entry point: `WorkerManager` stub, SQS consumer abstraction
- `db/migrations/0001_initial.sql` — derived from `spec/out.sql` (68-table schema)
- `sqlc.yaml` — configured for PostgreSQL, `db/queries/`, `db/migrations/`, with UUID/JSONB/timestamptz overrides
- `internal/config/config.go` — flat `Config` struct loaded from environment variables
- `internal/middleware/` — `requestid`, `otel`, `logging`, `cors`, `ratelimit`, `sizelimit` (auth middleware is a separate change)
- `internal/handler/status/` — `GET /`, `GET /_status`, `POST /_status` health endpoints
- `pkg/crypto/` — encrypt/decrypt wrappers replacing `app/encryption.py`
- `pkg/signing/` — itsdangerous-compatible HMAC signing for notification blobs and callbacks
- `queue/consumer.go`, `queue/producer.go` — SQS long-poll and send abstractions
- CI: `Makefile` targets for `build`, `test`, `lint`, `sqlc generate`, `migrate up/down`

## Capabilities

### New Capabilities

- `go-project-scaffold`: Project layout, Go module, config system, chi router with health endpoints, middleware stack (excluding auth), sqlc setup, golang-migrate with seed migration, SQS queue consumer/producer abstractions, `pkg/crypto` and `pkg/signing` packages, CI Makefile

### Modified Capabilities

## Non-goals

- No domain-specific handlers, services, or repository functions (handled in subsequent changes)
- No authentication middleware (handled in `authentication-middleware`)
- No database query files beyond the seed migration (each domain change adds its own queries)
- No worker pool implementations (handled in `notification-delivery-pipeline`)
- No external API client packages (Airtable, Salesforce, Freshdesk — handled in `external-client-integrations`)

## Impact

- New repository; no existing code is modified
- Establishes the Go module path that all subsequent changes import
- All 19 subsequent OpenSpec changes depend on the project layout defined here
