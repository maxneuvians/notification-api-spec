# Brief: go-project-setup

## Source Files
- `spec/go-architecture.md`
- `openspec/changes/go-project-setup/proposal.md`

---

## Overview

This change bootstraps the Go rewrite of the Python/Flask/Celery notification-api. It establishes:
- The Go module layout
- Chi HTTP router with the full non-auth middleware stack
- Health endpoints
- Configuration system (flat env-var struct)
- sqlc code-generation setup with the seed migration
- golang-migrate database migration runner (at startup)
- `pkg/crypto` — encrypted column helpers (replaces `app/encryption.py`)
- `pkg/signing` — itsdangerous-compatible HMAC signing
- SQS consumer/producer abstractions (`queue/consumer.go`, `queue/producer.go`)
- `WorkerManager` stub (`internal/worker/manager.go`)
- CI Makefile with `build`, `test`, `lint`, `sqlc`, `migrate` targets

**All 19 subsequent changes depend on the project layout, module path, and base packages defined here.**

---

## Project Directory Layout

```
go.mod
go.sum
cmd/
  api/
    main.go              HTTP server entry point
  worker/
    main.go              Background goroutine worker entry point
db/
  migrations/
    0001_initial.sql     Derived from spec/out.sql (68 tables)
  queries/               (empty at this stage — populated by data-model-migrations)
sqlc.yaml
internal/
  handler/
    status/              GET /, GET /_status, POST /_status
  middleware/
    requestid.go
    otel.go
    logging.go
    cors.go
    ratelimit.go
    sizelimit.go
  config/
    config.go
  worker/
    manager.go           WorkerManager stub
  client/                (empty dirs, populated by later changes)
queue/
  consumer.go
  producer.go
pkg/
  crypto/
  signing/
  smsutil/               (stub/empty)
  emailutil/             (stub/empty)
  pagination/            (stub/empty)
Makefile
```

---

## HTTP Router: chi

**Library**: `github.com/go-chi/chi/v5`

**Top-level mounts** (all domain sub-trees are empty stubs or omitted at this stage):
```
GET  /                → handler/status
GET  /_status         → handler/status
POST /_status         → handler/status
GET  /_status/live-service-and-organisation-counts → handler/status
```

**Routing pattern**: Each sub-package under `internal/handler/` registers its routes onto a `chi.Router` sub-tree. The top-level router mounts these sub-trees at appropriate URL prefixes matching Flask blueprint prefixes.

---

## Middleware Stack (exact order at top-level router)

1. **RequestID** (`internal/middleware/requestid.go`)
   - Reads `X-Request-ID` header; generates a new UUID if absent
   - Injects into request context
   - Returns in `X-Request-ID` response header

2. **OTEL tracing** (`internal/middleware/otel.go`)
   - `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`
   - Enabled when `config.FFEnableOtel == true`; otherwise no-op

3. **Structured logging** (`internal/middleware/logging.go`)
   - Logs method, path, status, latency, request ID per request

4. **CORS** (`internal/middleware/cors.go`)
   - Allowed origins: `config.AdminBaseURL`
   - Allowed methods: GET, POST, PUT, DELETE
   - Allowed headers: `Authorization`, `Content-Type`, `X-Request-ID`

5. **Rate limiting** (`internal/middleware/ratelimit.go`)
   - Per-IP token bucket (configurable rate)

6. **Request size limiting** (`internal/middleware/sizelimit.go`)
   - Default max body: `config.AttachmentSizeLimit × config.AttachmentNumLimit`
   - Returns HTTP 413 if exceeded

**Auth middleware is NOT applied globally** — it is applied per route group in domain handler registration. Auth middleware is implemented in the `authentication-middleware` change.

---

## Health Endpoints

`internal/handler/status/` handles:

| Method | Path | Response | Auth |
|---|---|---|---|
| GET | `/` | 200 `{"status": "ok"}` | None |
| GET | `/_status` | 200 `{"status": "ok"}` | None |
| POST | `/_status` | 200 `{"status": "ok"}` | None |
| GET | `/_status/live-service-and-organisation-counts` | 200 (live counts stub) | None |

These endpoints MUST respond even when the DB is unreachable (liveness, not readiness check).

---

## Configuration: `internal/config/config.go`

**Pattern**: Single flat `Config` struct, all fields from environment variables. Uses `github.com/caarlos0/env/v11` or equivalent. Passed via dependency injection — no global state.

**`Load()` function**: Returns error listing all missing required variables. Process exits before binding any port if Load() fails.

### Database
| Go field | Env var | Type | Default |
|---|---|---|---|
| `DatabaseURI` | `DATABASE_URI` | `string` | required |
| `DatabaseReaderURI` | `DATABASE_READER_URI` | `string` | optional |
| `DBPoolSize` | `POOL_SIZE` | `int` | 5 |

### Redis
| Go field | Env var | Default |
|---|---|---|
| `RedisURL` | `REDIS_URL` | — |
| `CacheOpsURL` | `CACHE_OPS_URL` | defaults to REDIS_URL |
| `RedisEnabled` | `REDIS_ENABLED` | false |

### AWS
| Go field | Env var | Default |
|---|---|---|
| `AWSRegion` | `AWS_REGION` | `us-east-1` |
| `AWSSESRegion` | `AWS_SES_REGION` | `us-east-1` |
| `AWSSESAccessKey` | `AWS_SES_ACCESS_KEY` | — |
| `AWSSESSecretKey` | `AWS_SES_SECRET_KEY` | — |
| `AWSPinpointRegion` | `AWS_PINPOINT_REGION` | `us-west-2` |
| `AWSPinpointSCPoolID` | `AWS_PINPOINT_SC_POOL_ID` | — |
| `AWSPinpointDefaultPoolID` | `AWS_PINPOINT_DEFAULT_POOL_ID` | — |
| `AWSPinpointConfigSet` | `AWS_PINPOINT_CONFIGURATION_SET_NAME` | `pinpoint-configuration` |
| `AWSPinpointSCTemplateIDs` | `AWS_PINPOINT_SC_TEMPLATE_IDS` | `[]string` comma-sep |
| `AWSUSTollFreeNumber` | `AWS_US_TOLL_FREE_NUMBER` | — |
| `CSVUploadBucket` | `CSV_UPLOAD_BUCKET_NAME` | — |
| `ReportsBucket` | `REPORTS_BUCKET_NAME` | — |
| `GCOrganisationsBucket` | `GC_ORGANISATIONS_BUCKET_NAME` | — |
| `GCOrganisationsFilename` | `GC_ORGANISATIONS_FILENAME` | `all.json` |

### SQS / Worker
| Go field | Env var | Default |
|---|---|---|
| `NotificationQueuePrefix` | `NOTIFICATION_QUEUE_PREFIX` | — |
| `CeleryDeliverSMSRateLimit` | `CELERY_DELIVER_SMS_RATE_LIMIT` | `"1/s"` |
| `BatchInsertionChunkSize` | `BATCH_INSERTION_CHUNK_SIZE` | 500 |
| `SMSWorkerConcurrency` | `CELERY_CONCURRENCY` | 4 |

### Auth
| Go field | Env var | Notes |
|---|---|---|
| `AdminBaseURL` | `ADMIN_BASE_URL` | default `http://localhost:6012` |
| `AdminClientSecret` | `ADMIN_CLIENT_SECRET` | required |
| `AdminClientUserName` | — | hardcoded `"notify-admin"` |
| `SecretKey` | `SECRET_KEY` | `[]string` comma-sep; required |
| `DangerousSalt` | `DANGEROUS_SALT` | required |
| `SREClientSecret` | `SRE_CLIENT_SECRET` | — |
| `SREUserName` | `SRE_USER_NAME` | — |
| `CacheClearClientSecret` | `CACHE_CLEAR_CLIENT_SECRET` | — |
| `CacheClearUserName` | `CACHE_CLEAR_USER_NAME` | — |
| `CypressAuthClientSecret` | `CYPRESS_AUTH_CLIENT_SECRET` | — |
| `CypressAuthUserName` | `CYPRESS_AUTH_USER_NAME` | — |
| `CypressUserPWSecret` | `CYPRESS_USER_PW_SECRET` | — |
| `APIKeyPrefix` | — | hardcoded `"gcntfy-"` |

### Feature Flags
| Go field | Env var | Default |
|---|---|---|
| `FFUseBillableUnits` | `FF_USE_BILLABLE_UNITS` | false |
| `FFSalesforceContact` | `FF_SALESFORCE_CONTACT` | false |
| `FFUsePinpointForDedicated` | `FF_USE_PINPOINT_FOR_DEDICATED` | false |
| `FFBounceRateSeedEpochMs` | `FF_BOUNCE_RATE_SEED_EPOCH_MS` | 0 |
| `FFPTServiceSkipFreshdesk` | `FF_PT_SERVICE_SKIP_FRESHDESK` | false |
| `FFEnableOtel` | `FF_ENABLE_OTEL` | false |

### Application
| Go field | Env var | Default |
|---|---|---|
| `NotifyEnvironment` | `NOTIFY_ENVIRONMENT` | `"development"` |
| `APIHostName` | `API_HOST_NAME` | — |
| `DocumentationDomain` | `DOCUMENTATION_DOMAIN` | `"documentation.notification.canada.ca"` |
| `InvitationExpirationDays` | — | 2 |
| `PageSize` | — | 50 |
| `APIPageSize` | — | 250 |
| `MaxVerifyCodeCount` | — | 10 |
| `JobsMaxScheduleHoursAhead` | — | 96 |
| `FailedLoginLimit` | `FAILED_LOGIN_LIMIT` | 10 |
| `AttachmentNumLimit` | `ATTACHMENT_NUM_LIMIT` | 10 |
| `AttachmentSizeLimit` | `ATTACHMENT_SIZE_LIMIT` | 10485760 (10 MB) |
| `PersonalisationSizeLimit` | `PERSONALISATION_SIZE_LIMIT` | 51200 (50 KB) |
| `AllowHTMLServiceIDs` | `ALLOW_HTML_SERVICE_IDS` | `[]string` |
| `DaysBeforeReportsExpire` | `DAYS_BEFORE_REPORTS_EXPIRE` | 3 |
| `ScanForPII` | `SCAN_FOR_PII` | false |
| `CSVBulkRedirectThreshold` | `CSV_BULK_REDIRECT_THRESHOLD` | configurable |
| `OtelRequestMetricsEnabled` | `OTEL_REQUEST_METRICS_ENABLED` | false |

### Well-known Internal UUIDs (hardcoded constants)
```go
const (
    NotifyServiceID        = "d6aa2c68-a2d9-4437-ab19-3ae8eb202553"
    HeartbeatServiceID     = "30b2fb9c-f8ad-49ad-818a-ed123fc00758"
    NewsletterServiceID    = "143806ca-3068-4f5d-9c6d-276b4151a395"
    NotifyUserID           = "6af522d0-2915-4e52-83a3-3690455a5fe6"
    CypressServiceID       = "d4e8a7f4-2b8a-4c9a-8b3f-9c2d4e8a7f4b"
    CypressTestUserID      = "e5f9d8c7-3a9b-4d8c-9b4f-8d3e5f9d8c7a"
    CypressTestUserAdminID = "4f8b8b1e-9c4f-4d8b-8b1e-4f8b8b1e9c4f"
)
```

### Internal Template ID Constants (subset)
```go
const (
    InvitationEmailTemplateID          = "4f46df42-f795-4cc4-83bb-65ca312f49cc"
    SMSCodeTemplateID                  = "36fb0730-6259-4da1-8a80-c8de22ad4246"
    NoReplyTemplateID                  = "86950840-6da4-4865-841b-16028110e980"
    OrganisationInvitationTemplateID   = "203566f0-d835-47c5-aa06-932439c86573"
    PasswordResetTemplateID            = "474e9242-823b-4f99-813d-ed392e7f1201"
    ReportDownloadTemplateID           = "8b5c14e1-2c78-4b87-9797-5b8cc8d9a86c"
    ReachedAnnualLimitTemplateID       = "ca6d9205-d923-4198-acdd-d0aa37725c37"
    NearAnnualLimitTemplateID          = "1a7a1f01-7fd0-43e5-93a4-982e25a78816"
    AnnualLimitQuarterlyUsageTemplateID= "f66a1025-17f5-471c-a7ab-37d6b9e9d304"
    APIKeyRevokeTemplateID             = "a0a4e7b8-8a6a-4eaa-9f4e-9c3a5b2dbcf3"
)
```

---

## golang-migrate Setup

- **Library**: `github.com/golang-migrate/migrate/v4` with `postgres` driver
- **Migration directory**: `db/migrations/`
- **Seed migration**: `db/migrations/0001_initial.sql` derived from `spec/out.sql`
- **CLI usage**: `migrate -path db/migrations -database "$DATABASE_URI" up`
- **Startup behaviour**: Both `cmd/api/main.go` and `cmd/worker/main.go` call `runMigrations(db)` before any port binding or queue consumption. If the migration fails, the process logs the error and exits with non-zero status.
- **Already-current database**: `runMigrations` is idempotent — already-applied migrations are skipped without error.

---

## sqlc Configuration (`sqlc.yaml`)

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "db/queries/"
    schema: "db/migrations/"
    gen:
      go:
        package: "repository"
        out: "internal/repository"
        emit_json_tags: true
        emit_pointers_for_null_types: true
        emit_enum_valid_method: true
        emit_all_enum_values: true
        overrides:
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
          - db_type: "uuid"
            nullable: true
            go_type: "github.com/google/uuid.NullUUID"
          - db_type: "jsonb"
            go_type: "encoding/json.RawMessage"
          - db_type: "pg_catalog.timestamptz"
            go_type: "time.Time"
          - db_type: "pg_catalog.timestamptz"
            nullable: true
            go_type: "*time.Time"
```

---

## pkg/crypto

Replaces `app/encryption.py` (Python itsdangerous + cryptography).

**Interface**:
```go
func Encrypt(plaintext string, secret string) (string, error)
func Decrypt(ciphertext string, secrets []string) (string, error)
```

- Encryption uses `config.SecretKey[0]` (current key).
- Decryption tries each key in `config.SecretKeys` in order — key rotation support.
- Used explicitly by service layer before INSERT (Encrypt) and after SELECT (Decrypt) for all 8 encrypted columns.
- **NOT called by repository layer** — repository functions return raw ciphertext.

---

## pkg/signing

Replaces Python `itsdangerous` HMAC-SHA256 signing used for:
- Notification callback blobs
- Signed invitation tokens
- Other signed URL parameters

**Interface**:
```go
func Sign(payload string, secret string, salt string) (string, error)
func Unsign(token string, secrets []string, salt string) (string, error)
func Dumps(data any, secret string, salt string) (string, error)
func Loads(token string, secrets []string, salt string) (map[string]any, error)
```

**Compatibility requirement**: Tokens generated by `pkg/signing` must be verifiable by the Python itsdangerous library and vice versa during the transition period. A round-trip test must be included.

---

## SQS Queue Abstractions (`queue/`)

### `queue/consumer.go`

```go
type Consumer interface {
    Start(ctx context.Context, handler func(ctx context.Context, msg *Message) error)
    Stop()
}

type Message struct {
    ID            string
    Body          string
    ReceiptHandle string
    Attributes    map[string]string
}
```

**Long-poll loop**: `WaitTimeSeconds=20`, `MaxMessages=10`, `VisibilityTimeout=310`. On handler success: `DeleteMessage`. On transient error: extend visibility timeout (exponential backoff). On permanent error: do not extend; let SQS move to DLQ after max-receive count.

### `queue/producer.go`

```go
type Producer interface {
    Send(ctx context.Context, queueURL, body string, attrs map[string]string) error
    SendFIFO(ctx context.Context, queueURL, body, groupID, deduplicationID string) error
}
```

---

## WorkerManager Stub (`internal/worker/manager.go`)

The full worker pool implementation with SQS queues and scheduled goroutines is out-of-scope for this change (handled by `notification-delivery-pipeline`). This change delivers:

```go
type WorkerManager struct {
    cfg *config.Config
    // dependencies injected here in later changes
}

func NewWorkerManager(cfg *config.Config) *WorkerManager { ... }
func (wm *WorkerManager) Start(ctx context.Context) error { return nil }
func (wm *WorkerManager) Stop() {}
```

The stub MUST compile and integrate with `cmd/worker/main.go`. It returns immediately from `Start`  and is replaced by the real implementation in `notification-delivery-pipeline`.

---

## HTTP Error Response Format

Two wire formats must be preserved exactly (clients depend on them):

### Admin API (non-/v2/ routes)
```json
{ "result": "error", "message": "No result found" }
```
`message` may be a string or an object for validation errors:
```json
{ "result": "error", "message": { "name": ["Missing data for required field."] } }
```

### Public v2 API (/v2/ routes)
```json
{
  "status_code": 400,
  "errors": [
    { "error": "ValidationError", "message": "template_id is not a valid UUID" }
  ]
}
```

Define sentinel error types in `internal/handler/errors.go`:
```go
type APIError struct {
    StatusCode int
    Body       any
}
```

---

## Command Entry Points

### `cmd/api/main.go`
1. Load `config.Config` via `config.Load()` — exit on error
2. Open `*sql.DB` (writer) and optionally reader; call `runMigrations(db)`
3. Construct chi router; apply middleware stack; register status handler
4. Listen and serve on configured port

### `cmd/worker/main.go`
1. Load `config.Config` — exit on error
2. Open `*sql.DB`; call `runMigrations(db)`
3. Construct `WorkerManager`; call `wm.Start(ctx)`
4. Block until signal (SIGTERM/SIGINT); call `wm.Stop()`

Both binaries share all `internal/` packages via the same Go module.

---

## Makefile Targets
| Target | Command |
|---|---|
| `build` | `go build ./cmd/api ./cmd/worker` |
| `test` | `go test ./...` |
| `lint` | `golangci-lint run` |
| `sqlc` | `sqlc generate` |
| `migrate-up` | `migrate -path db/migrations -database "$(DB_URL)" up` |
| `migrate-down` | `migrate -path db/migrations -database "$(DB_URL)" down 1` |

---

## Key Dependencies (`go.mod`)

| Package | Purpose |
|---|---|
| `github.com/go-chi/chi/v5` | HTTP router |
| `github.com/golang-migrate/migrate/v4` | DB migrations |
| `github.com/google/uuid` | UUID type |
| `sqlc-dev/sqlc` (tool) | SQL code generation |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | OTEL tracing middleware |
| `github.com/robfig/cron/v3` | Scheduled task cron |
| `golang.org/x/time/rate` | Rate limiting |
| `github.com/caarlos0/env/v11` | Env var struct parsing |
| `github.com/joho/godotenv` | .env file loading (dev) |
| `github.com/aws/aws-sdk-go-v2` | AWS SDK (SQS, SES, SNS, etc.) |
| `github.com/go-playground/validator/v10` | Request validation |

---

## Single-Binary vs Two-Binary Decision (deferred)

`cmd/api` and `cmd/worker` may be compiled as one binary with `api`/`worker` subcommands or as two separate binaries. The project structure supports both. The deployment decision is deferred to CI/CD configuration.

---

## Letter Stub Endpoints (out of scope here; tracked in letter-stub-endpoints change)

7 letter-related routes SHALL be registered at the router returning HTTP 501 Not Implemented. They are implemented as stubs in the `letter-stub-endpoints` change, not here.
