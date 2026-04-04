# Tasks: go-project-setup

## 1. Repository scaffold and Go module
- [ ] 1.1 Create repository; write `go.mod` with module path and Go version; create all top-level directories: `cmd/api/`, `cmd/worker/`, `db/migrations/`, `db/queries/`, `internal/handler/`, `internal/service/`, `internal/repository/`, `internal/middleware/`, `internal/worker/`, `internal/client/`, `internal/config/`, `queue/`, `pkg/crypto/`, `pkg/smsutil/`, `pkg/emailutil/`, `pkg/pagination/`, `pkg/signing/`
- [ ] 1.2 Add core dependencies to `go.mod`/`go.sum`: `github.com/go-chi/chi/v5`, `github.com/golang-migrate/migrate/v4`, `github.com/google/uuid`, `github.com/caarlos0/env/v11`, `github.com/joho/godotenv`; run `go mod tidy`; verify `go build ./...` succeeds on the empty scaffold

## 2. Configuration system
- [ ] 2.1 Implement `internal/config/config.go`: flat `Config` struct with all fields from `go-architecture.md` (database, Redis, AWS, SQS/worker, auth, feature flags, application); implement `Load() (*Config, error)` that reads env vars, validates all required fields, and returns an error listing any missing required variables
- [ ] 2.2 Add hardcoded constants to `internal/config/constants.go`: `NotifyServiceID`, `HeartbeatServiceID`, `NewsletterServiceID`, `NotifyUserID`, `CypressServiceID`, `CypressTestUserID`, `CypressTestUserAdminID`, `APIKeyPrefix = "gcntfy-"`, internal template ID constants
- [ ] 2.3 Write unit tests for `Config.Load()`: missing required variable returns error containing var name; optional variables use documented defaults; SECRET_KEY comma-split into `[]string`

## 3. pkg/crypto — encrypted column helpers
- [ ] 3.1 Implement `pkg/crypto/crypto.go`: `Encrypt(plaintext, secret string) (string, error)` and `Decrypt(ciphertext string, secrets []string) (string, error)` using HMAC-based encryption compatible with Python `app/encryption.py`; key rotation: encrypt uses `secrets[0]`; decrypt tries each secret in order
- [ ] 3.2 Write unit tests: Encrypt→Decrypt round-trip; Decrypt with second key succeeds; Decrypt with no matching key returns error
- [ ] 3.3 Write integration test or reference test: use a known ciphertext+key pair from the Python implementation to verify interoperability (document the expected ciphertext value as a fixture)

## 4. pkg/signing — itsdangerous-compatible HMAC
- [ ] 4.1 Implement `pkg/signing/signing.go`: `Sign`, `Unsign`, `Dumps`, `Loads` with HMAC-SHA256 format compatible with Python `itsdangerous.TimestampSigner` / `itsdangerous.URLSafeTimedSerializer`
- [ ] 4.2 Write unit tests: Sign→Unsign round-trip; Unsign fails with wrong secret; Loads returns correct map from Dumps output
- [ ] 4.3 Write cross-language compatibility test using a fixture token signed by the Python library: `Unsign(pythonToken, []string{secret}, salt)` must return original payload without error

## 5. Seed migration
- [ ] 5.1 Create `db/migrations/0001_initial.sql` by converting `spec/out.sql` into a golang-migrate–compatible migration file: remove `pg_dump` headers/comments, keep all `CREATE TYPE`, `CREATE TABLE`, `CREATE INDEX`, `ALTER TABLE ADD CONSTRAINT` statements; verify the file applies cleanly to an empty PostgreSQL database using `make migrate-up`

## 6. sqlc configuration
- [ ] 6.1 Create `sqlc.yaml` with `version: "2"`, `engine: "postgresql"`, `queries: "db/queries/"`, `schema: "db/migrations/"`, `out: "internal/repository/"`, all 5 type overrides, and all 4 emit flags set to `true`
- [ ] 6.2 Run `make sqlc` (set up in task 14); confirm it exits with no error against an empty `db/queries/` directory

## 7. Middleware implementations
- [ ] 7.1 Implement `internal/middleware/requestid.go`: read `X-Request-ID` header; generate UUID if absent; inject into context; write to response header
- [ ] 7.2 Implement `internal/middleware/otel.go`: wrap `otelhttp.NewHandler`; enabled only when `config.FFEnableOtel == true`; otherwise pass through
- [ ] 7.3 Implement `internal/middleware/logging.go`: structured log (JSON) per request with method, path, status, latency, request ID
- [ ] 7.4 Implement `internal/middleware/cors.go`: allow `config.AdminBaseURL`; allowed methods GET/POST/PUT/DELETE; allowed headers `Authorization`, `Content-Type`, `X-Request-ID`
- [ ] 7.5 Implement `internal/middleware/ratelimit.go`: per-IP token bucket using `golang.org/x/time/rate`; rate and burst configurable from config
- [ ] 7.6 Implement `internal/middleware/sizelimit.go`: max body `config.AttachmentSizeLimit × config.AttachmentNumLimit`; return HTTP 413 if exceeded before handler is called

## 8. Health endpoints
- [ ] 8.1 Implement `internal/handler/status/handler.go`: `GET /` → 200 `{"status": "ok"}`; `GET /_status` → 200 `{"status": "ok"}`; `POST /_status` → 200 `{"status": "ok"}`; `GET /_status/live-service-and-organisation-counts` → 200 with stub counts
- [ ] 8.2 Write handler tests: each route returns 200 with correct body; no auth required

## 9. HTTP error format helpers
- [ ] 9.1 Implement `internal/handler/errors.go`: define `APIError` struct; write `WriteAdminError(w, statusCode, message)` helper that writes `{"result": "error", "message": ...}`; write `WriteV2Error(w, statusCode, errorType, message)` helper that writes `{"status_code": N, "errors": [{"error": "...", "message": "..."}]}`

## 10. cmd/api main entry point
- [ ] 10.1 Implement `cmd/api/main.go`: call `config.Load()` (exit on error); open writer `*sql.DB` and reader `*sql.DB` (if `DatabaseReaderURI` set); call `runMigrations(db)`; construct chi router; apply middleware stack in exact order (RequestID → OTEL → logging → CORS → ratelimit → sizelimit); register status routes; implement `runMigrations(db)` using `golang-migrate` postgres driver with `db/migrations/`; listen on configured port
- [ ] 10.2 Implement `internal/handler/errors.go` global chi middleware for unhandled panics returning HTTP 500 in admin error format

## 11. cmd/worker main entry point
- [ ] 11.1 Implement `internal/worker/manager.go`: `WorkerManager` struct; `NewWorkerManager(cfg *config.Config) *WorkerManager`; `Start(ctx context.Context) error` (returns nil immediately); `Stop()`
- [ ] 11.2 Implement `cmd/worker/main.go`: call `config.Load()` (exit on error); open `*sql.DB`; call `runMigrations(db)`; construct `WorkerManager`; call `wm.Start(ctx)`; block on `SIGTERM`/`SIGINT`; call `wm.Stop()`; exit 0

## 12. SQS abstractions
- [ ] 12.1 Implement `queue/consumer.go`: define `Message` struct and `Consumer` interface; implement `SQSConsumer` with `Start(ctx, handler)` long-poll loop (`WaitTimeSeconds=20`, `MaxMessages=10`, `VisibilityTimeout=310`); on success delete message; on transient error extend visibility with exponential backoff; on context cancellation drain in-progress batch and exit
- [ ] 12.2 Implement `queue/producer.go`: define `Producer` interface; implement `SQSProducer` with `Send` and `SendFIFO` methods; `SendFIFO` includes `MessageGroupId` and `MessageDeduplicationId`
- [ ] 12.3 Write unit tests for consumer: successful handler triggers DeleteMessage; transient error triggers ChangeMessageVisibility; context cancellation stops the loop

## 13. golang-migrate runMigrations helper
- [ ] 13.1 Extract `runMigrations(db *sql.DB) error` into `internal/migrate/migrate.go` (or inline in main.go): instantiate `migrate.New("file://db/migrations", connURL)` using `golang-migrate/migrate/v4` postgres driver; call `m.Up()`; if `err == migrate.ErrNoChange` return nil; otherwise return error

## 14. Makefile
- [ ] 14.1 Write `Makefile` with targets: `build` (`go build ./cmd/api ./cmd/worker`), `test` (`go test ./...`), `lint` (`golangci-lint run`), `sqlc` (`sqlc generate`), `migrate-up` (`migrate -path db/migrations -database "$(DB_URL)" up`), `migrate-down` (`migrate -path db/migrations -database "$(DB_URL)" down 1`); verify `make build` succeeds
