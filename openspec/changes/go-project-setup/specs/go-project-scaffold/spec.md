# Capability: go-project-scaffold

Project layout, Go module, config system, chi router with health endpoints, middleware stack (excluding auth), sqlc setup, golang-migrate with seed migration, SQS consumer/producer abstractions, `pkg/crypto` and `pkg/signing` packages, `WorkerManager` stub, CI Makefile.

---

## Requirement: Project directory layout

**R1** — The Go module SHALL implement the directory structure defined in `go-architecture.md`:
`cmd/api/`, `cmd/worker/`, `db/migrations/`, `db/queries/`, `internal/handler/`, `internal/service/`, `internal/repository/`, `internal/middleware/`, `internal/worker/`, `internal/client/`, `internal/config/`, `queue/`, `pkg/crypto/`, `pkg/smsutil/`, `pkg/emailutil/`, `pkg/pagination/`, `pkg/signing/`.

#### Scenario: Project builds from root
- **WHEN** `go build ./...` is run from the repository root
- **THEN** both `cmd/api` and `cmd/worker` compile without errors

#### Scenario: All packages importable via module path
- **WHEN** any `internal/` package imports another `internal/` package
- **THEN** the import path uses the module path defined in `go.mod`

#### Scenario: cmd/api and cmd/worker are separate entry points
- **WHEN** `go build ./cmd/api` is run
- **THEN** it produces an `api` binary without building `cmd/worker`, and vice versa

---

## Requirement: Configuration loaded from environment

**R2** — `internal/config/config.go` SHALL define a flat `Config` struct. All fields SHALL be populated from environment variables. A `Load()` function SHALL return an error listing all missing required variables.

Required variables include at minimum: `DATABASE_URI`, `ADMIN_CLIENT_SECRET`, `SECRET_KEY`, `DANGEROUS_SALT`.

Optional variables have documented defaults (see brief.md for complete list).

Well-known internal UUIDs (`NotifyServiceID`, `HeartbeatServiceID`, etc.) and `APIKeyPrefix` (`"gcntfy-"`) SHALL be hardcoded constants, not configurable.

#### Scenario: Missing required variable causes fast exit
- **WHEN** a required environment variable (e.g. `DATABASE_URI`) is absent at startup
- **THEN** `Config.Load()` returns an error containing the variable name; the process exits with non-zero status before binding any port

#### Scenario: Optional variable uses documented default
- **WHEN** an optional environment variable (e.g. `POOL_SIZE`) is absent
- **THEN** `Config.Load()` succeeds and the field is set to its default value (5)

#### Scenario: Config struct has no global state
- **WHEN** the Config struct is instantiated in tests
- **THEN** it can be constructed directly without side effects on global state

#### Scenario: SECRET_KEY supports multiple comma-separated keys
- **WHEN** `SECRET_KEY=key1,key2,key3` is set
- **THEN** `config.SecretKeys` is a `[]string` with 3 elements; `config.SecretKey[0]` is `"key1"`

#### Scenario: NotifyEnvironment defaults to "development"
- **WHEN** `NOTIFY_ENVIRONMENT` is absent
- **THEN** `config.NotifyEnvironment` is `"development"`

---

## Requirement: Health endpoints

**R3** — `internal/handler/status/` SHALL implement four routes with no authentication:
- `GET /` → HTTP 200 `{"status": "ok"}`
- `GET /_status` → HTTP 200 `{"status": "ok"}`
- `POST /_status` → HTTP 200 `{"status": "ok"}`
- `GET /_status/live-service-and-organisation-counts` → HTTP 200 (stub counts response)

#### Scenario: Health check responds while DB is unreachable
- **WHEN** `GET /_status` is called and the PostgreSQL connection pool is unavailable
- **THEN** the endpoint still returns HTTP 200 (liveness check, not readiness)

#### Scenario: Status routes require no auth header
- **WHEN** `GET /_status` is called without any `Authorization` header
- **THEN** the response is HTTP 200

#### Scenario: GET / returns ok
- **WHEN** `GET /` is called
- **THEN** the response is HTTP 200 with body `{"status": "ok"}`

#### Scenario: POST /_status returns ok
- **WHEN** `POST /_status` is called with an empty body
- **THEN** the response is HTTP 200 with body `{"status": "ok"}`

---

## Requirement: Middleware stack ordering

**R4** — `cmd/api/main.go` SHALL apply the following middleware to the top-level chi router in this exact order: RequestID, OTEL tracing, structured logging, CORS, rate limiting, request size limiting. Authentication middleware SHALL NOT be applied globally.

#### Scenario: Request ID propagated when present in request
- **WHEN** a request arrives with an `X-Request-ID: abc-123` header
- **THEN** the same value `abc-123` is present in the structured log entry AND in the `X-Request-ID` response header

#### Scenario: Request ID generated when absent
- **WHEN** a request arrives without an `X-Request-ID` header
- **THEN** a new UUID v4 is generated, injected into the request context, logged, and returned in the `X-Request-ID` response header

#### Scenario: Oversized request rejected before handler is invoked
- **WHEN** a request body exceeds the configured maximum size (`AttachmentSizeLimit × AttachmentNumLimit`)
- **THEN** the middleware returns HTTP 413 and the handler function is NOT called

#### Scenario: CORS allows admin origin
- **WHEN** a request arrives with `Origin: <value of AdminBaseURL>`
- **THEN** the response contains appropriate `Access-Control-Allow-Origin` header

#### Scenario: OTEL tracing is a no-op when disabled
- **WHEN** `config.FFEnableOtel == false`
- **THEN** no OpenTelemetry spans are created; the middleware is still in the chain but does not add latency

#### Scenario: Auth middleware not applied to health routes
- **WHEN** `GET /_status` is called without an `Authorization` header
- **THEN** no 401/403 is returned (auth middleware is not on this route)

---

## Requirement: Database migrations run on startup

**R5** — Both `cmd/api/main.go` and `cmd/worker/main.go` SHALL call `runMigrations(db)` using `github.com/golang-migrate/migrate/v4` with the `postgres` driver before the application starts serving traffic or processing queues.

`db/migrations/0001_initial.sql` is the seed migration derived from `spec/out.sql`.

#### Scenario: Clean startup applies all pending migrations
- **WHEN** the application starts against a database with no migration history
- **THEN** all migration files in `db/migrations/` are applied in ascending numeric order and the application proceeds to start

#### Scenario: Already-migrated database is a no-op
- **WHEN** the application starts against a database already at the latest migration version
- **THEN** `runMigrations` returns without error and startup continues normally

#### Scenario: Migration failure aborts startup
- **WHEN** a migration SQL statement fails (e.g. constraint violation, syntax error)
- **THEN** the application logs the error and exits with a non-zero status code; no port is bound

#### Scenario: Concurrent pod startup uses advisory lock
- **WHEN** two application pods start simultaneously against the same database
- **THEN** only one pod applies the migration; the other waits for the advisory lock and then proceeds with startup (golang-migrate behaviour)

---

## Requirement: sqlc configuration

**R6** — `sqlc.yaml` SHALL configure the `postgresql` engine, point at `db/queries/` for queries and `db/migrations/` for schema, and output to `internal/repository/`. Type overrides SHALL include all five entries from `go-architecture.md`:
- `uuid` → `github.com/google/uuid.UUID`
- nullable `uuid` → `github.com/google/uuid.NullUUID`
- `jsonb` → `encoding/json.RawMessage`
- `pg_catalog.timestamptz` → `time.Time`
- nullable `pg_catalog.timestamptz` → `*time.Time`

All emit flags SHALL be `true`: `emit_json_tags`, `emit_pointers_for_null_types`, `emit_enum_valid_method`, `emit_all_enum_values`.

#### Scenario: sqlc generate succeeds with empty db/queries/
- **WHEN** `sqlc generate` is run with `db/queries/` containing no `.sql` files
- **THEN** it exits with no error (generates an empty or minimal `internal/repository/` package)

#### Scenario: sqlc generate uses all five type overrides
- **WHEN** `sqlc generate` is run after a query file with a UUID column is added
- **THEN** the generated Go type for a non-nullable UUID column is `github.com/google/uuid.UUID`, not `string`

---

## Requirement: pkg/crypto — encrypt/decrypt with key rotation

**R7** — `pkg/crypto` SHALL provide:
```go
func Encrypt(plaintext string, secret string) (string, error)
func Decrypt(ciphertext string, secrets []string) (string, error)
```

`Encrypt` uses the provided secret to produce a ciphertext string compatible with the Python `app/encryption.py` format. `Decrypt` tries each secret in `secrets` in order, returning the plaintext when decryption succeeds.

#### Scenario: Encrypt → Decrypt round-trip succeeds with same key
- **WHEN** `Encrypt("hello", "mysecret")` is called and the result is passed to `Decrypt(ciphertext, []string{"mysecret"})`
- **THEN** the returned plaintext is `"hello"` with no error

#### Scenario: Decrypt succeeds with second key after rotation
- **WHEN** data was encrypted with `oldSecret` and `Decrypt(ciphertext, []string{"newSecret", "oldSecret"})` is called
- **THEN** decryption succeeds using `oldSecret` (second element) and returns the plaintext

#### Scenario: Decrypt fails if no key matches
- **WHEN** `Decrypt` is called with a ciphertext and a list of secrets that do not include the encrypting key
- **THEN** an error is returned and no plaintext is produced

#### Scenario: crypto.Encrypt is not called from repository layer
- **WHEN** all repository-layer code is reviewed
- **THEN** no call to `pkg/crypto.Encrypt` or `pkg/crypto.Decrypt` exists in `internal/repository/` packages

---

## Requirement: pkg/signing — itsdangerous-compatible HMAC

**R8** — `pkg/signing` SHALL provide:
```go
func Sign(payload string, secret string, salt string) (string, error)
func Unsign(token string, secrets []string, salt string) (string, error)
func Dumps(data any, secret string, salt string) (string, error)
func Loads(token string, secrets []string, salt string) (map[string]any, error)
```

Tokens produced by `Sign`/`Dumps` SHALL be verifiable by Python's `itsdangerous.TimestampSigner` / `itsdangerous.URLSafeTimedSerializer` using the same secret and salt. Tokens produced by the Python library SHALL be verifiable by `Unsign`/`Loads`.

#### Scenario: Go-signed token verifiable by Python
- **WHEN** `pkg/signing.Sign(payload, secret, salt)` produces a token
- **THEN** the Python itsdangerous library can verify and unsign that token using the same secret and salt

#### Scenario: Python-signed token verifiable by Go
- **WHEN** Python produces a signed token with itsdangerous
- **THEN** `pkg/signing.Unsign(token, []string{secret}, salt)` returns the original payload without error

#### Scenario: Unsign fails with wrong secret
- **WHEN** `Unsign` is called with a token and a secrets list that does not include the signing secret
- **THEN** an error is returned; no payload is extracted

#### Scenario: Loads returns typed map
- **WHEN** `Dumps(map[string]any{"user_id": "abc"}, secret, salt)` is called and `Loads` unsigns the result
- **THEN** the returned map contains `"user_id"` → `"abc"`

---

## Requirement: SQS Consumer abstraction

**R9** — `queue/consumer.go` SHALL define an interface and implementation for SQS long-polling:

```go
type Consumer interface {
    Start(ctx context.Context, handler func(ctx context.Context, msg *Message) error)
    Stop()
}
```

Long-poll parameters: `WaitTimeSeconds=20`, `MaxMessages=10`, `VisibilityTimeout=310`.

On handler success: call `DeleteMessage`. On transient error: extend visibility timeout with exponential backoff. On permanent/unrecoverable error: do not extend visibility; let SQS move to DLQ after the configured max-receive count.

#### Scenario: Successful message processing deletes the message
- **WHEN** the handler function returns `nil`
- **THEN** `DeleteMessage` is called for that message's receipt handle

#### Scenario: Transient error extends visibility timeout
- **WHEN** the handler function returns a transient error
- **THEN** `ChangeMessageVisibility` is called to extend the timeout (exponential backoff), and the message remains in the queue for retry

#### Scenario: Consumer stops on context cancellation
- **WHEN** the context passed to `Start` is cancelled
- **THEN** the long-poll loop exits cleanly after the in-progress batch completes

---

## Requirement: SQS Producer abstraction

**R10** — `queue/producer.go` SHALL define:

```go
type Producer interface {
    Send(ctx context.Context, queueURL, body string, attrs map[string]string) error
    SendFIFO(ctx context.Context, queueURL, body, groupID, deduplicationID string) error
}
```

#### Scenario: Send enqueues a message
- **WHEN** `Send` is called with a valid queue URL and message body
- **THEN** the message is delivered to the SQS queue without error

#### Scenario: SendFIFO includes deduplication ID
- **WHEN** `SendFIFO` is called with a deduplication ID
- **THEN** the SQS `SendMessage` request includes the `MessageDeduplicationId` attribute

---

## Requirement: WorkerManager stub

**R11** — `internal/worker/manager.go` SHALL provide a compilable `WorkerManager` struct with `NewWorkerManager`, `Start(ctx context.Context) error`, and `Stop()` methods. `Start` SHALL return `nil` immediately (no goroutines launched). `cmd/worker/main.go` SHALL instantiate `WorkerManager`, call `Start`, and block until SIGTERM/SIGINT.

#### Scenario: cmd/worker compiles and starts
- **WHEN** `go build ./cmd/worker` is run
- **THEN** the binary compiles without errors

#### Scenario: cmd/worker exits cleanly on SIGTERM
- **WHEN** the worker process receives `SIGTERM`
- **THEN** `WorkerManager.Stop()` is called, the process logs shutdown, and exits with status 0

---

## Requirement: HTTP error response format

**R12** — `internal/handler/errors.go` SHALL define sentinel types for the two wire error formats. The admin API (non-/v2/ routes) uses `{"result": "error", "message": ...}`. The public v2 API (/v2/ routes) uses `{"status_code": N, "errors": [...]}`.

```go
type APIError struct {
    StatusCode int
    Body       any
}
```

Standard status code mapping:
- `NoResultFound` / `DataError` → 404, admin format
- Validation error → 400, format varies by route group
- `AuthError` → 401 or 403
- Rate limit exceeded → 429
- Unhandled exception → 500, `{"result": "error", "message": "Internal server error"}`

#### Scenario: v2 error returns status_code in body
- **WHEN** a v2 handler returns a validation error
- **THEN** the response body is `{"status_code": 400, "errors": [{"error": "ValidationError", "message": "..."}]}`

#### Scenario: admin error returns result/message
- **WHEN** an admin (non-v2) handler returns a not-found error
- **THEN** the response body is `{"result": "error", "message": "No result found"}`

---

## Requirement: Makefile targets

**R13** — A `Makefile` at the repository root SHALL provide the following targets:

| Target | Command |
|---|---|
| `build` | `go build ./cmd/api ./cmd/worker` |
| `test` | `go test ./...` |
| `lint` | `golangci-lint run` |
| `sqlc` | `sqlc generate` |
| `migrate-up` | `migrate -path db/migrations -database "$(DB_URL)" up` |
| `migrate-down` | `migrate -path db/migrations -database "$(DB_URL)" down 1` |

#### Scenario: make build succeeds from clean checkout
- **WHEN** `make build` is run on a machine with Go installed and dependencies vendored or in module cache
- **THEN** both `cmd/api` and `cmd/worker` compile successfully

#### Scenario: make sqlc fails if queries are inconsistent with schema
- **WHEN** `make sqlc` is run and a query references a column that does not exist in the migration schema
- **THEN** `sqlc generate` exits with a non-zero status and describes the error

---

## Requirement: Seed migration derived from spec/out.sql

**R14** — `db/migrations/0001_initial.sql` SHALL create all 68 tables, all PostgreSQL ENUM types, all indexes, all foreign key constraints, and all check constraints as defined in `spec/out.sql`. No application logic or seed data is included in this migration.

#### Scenario: Seed migration applies cleanly to an empty database
- **WHEN** `migrate -path db/migrations -database "$DB_URL" up` is run against a fresh PostgreSQL database
- **THEN** all 68 tables are created without error

#### Scenario: Seed migration is idempotent via golang-migrate tracking
- **WHEN** the seed migration has already been applied and `runMigrations(db)` is called again
- **THEN** golang-migrate detects the migration is already at version 1 and skips it without error

---

## Requirement: No domain logic or auth in this change

**R15** — This change SHALL NOT implement any domain-specific handler, service, repository, or business rule. It SHALL NOT implement any authentication or authorization middleware.

#### Scenario: All /v2/ and /service/ routes are absent
- **WHEN** `GET /v2/notifications` is called against the running server
- **THEN** the router returns HTTP 404 (route is not registered yet)

#### Scenario: Authorization header is not checked on status route
- **WHEN** `GET /_status` is called with `Authorization: Bearer invalid_token`
- **THEN** the response is still HTTP 200 (no auth middleware on this route)
