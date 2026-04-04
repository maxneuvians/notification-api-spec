# Go Architecture

## Overview

### Design Principles

- **Single binary** (or two closely related binaries, `api` + `worker`) compiled from one Go module.
- **Goroutines replace Celery**: all 53 Celery tasks and 31 beat schedule entries collapse into goroutine pools and scheduled goroutines backed by SQS or `time.Ticker`/`robfig/cron`.
- **sqlc for the data layer**: all SQL is hand-written in `.sql` query files; sqlc generates type-safe Go structs and repository functions. No ORM.
- **golang-migrate for schema management**: migrations are versioned files under `db/migrations/`; the seed migration is derived from `specs/out.sql`.
- **chi router**: lightweight, stdlib-compatible mux with route groups that map cleanly onto the existing Flask blueprint prefixes.
- **One HTTP server** for both the internal admin API and the public v2 API; route groups are separated by URL prefix and middleware stack.

### Deployment Topology

```
┌──────────────────────────────────────────────────────────┐
│  cmd/api/main.go                                         │
│  - chi router (admin routes + v2 routes + sre-tools)     │
│  - OTEL HTTP middleware                                   │
│  - auth middleware (per route group)                     │
└──────────────────────────────┬───────────────────────────┘
                               │
         ┌─────────────────────┼─────────────────────┐
         ▼                     ▼                     ▼
   PostgreSQL (writer)  PostgreSQL (reader)        Redis
         │
   golang-migrate on startup
```

```
┌───────────────────────────────────────────────────────────┐
│  cmd/worker/main.go  (or packaged with api as sub-command)│
│  - WorkerManager starts goroutine pools per SQS queue     │
│  - robfig/cron for scheduled tasks + time.Ticker for sub- │
│    minute intervals                                       │
└───────────────────────────────────────────────────────────┘
```

The `cmd/api` and `cmd/worker` entry points may be compiled into one binary and invoked with subcommands (`notify api`, `notify worker`) or kept as separate binaries — this is an operational choice. The internal package tree is shared.

---

## Project Structure

```
go.mod
go.sum
cmd/
  api/
    main.go           HTTP server entry point
  worker/
    main.go           Background goroutine worker entry point
db/
  migrations/
    0001_initial.sql  Seeded from specs/out.sql
    0002_...sql       Subsequent changes
  queries/
    notifications.sql
    services.sql
    templates.sql
    jobs.sql
    billing.sql
    users.sql
    organisations.sql
    inbound_sms.sql
    providers.sql
    api_keys.sql
    complaints.sql
    reports.sql
    annual_limits.sql
    template_categories.sql
sqlc.yaml
internal/
  handler/
    notifications/    POST /notifications, GET /notifications/:id, etc.
    services/         /service and /service/:id family
    templates/        /service/:id/template family
    jobs/             /service/:id/job family
    billing/          /service/:id/billing family
    users/            /user family
    organisations/    /organisations family
    inbound/          /inbound-number, /service/:id/inbound-sms
    providers/        /provider-details family
    admin/            /sre-tools, /cache-clear, /cypress, /events
    v2/
      notifications/  /v2/notifications family
      templates/      /v2/template and /v2/templates
      inbound/        /v2/received-text-messages
      openapi/        /v2/openapi-en, /v2/openapi-fr
    status/           /, /_status, /_status/live-service-and-organisation-counts
    invite/           /invite/:type/:token
    api_key/          /api-key family
    email_branding/   /email-branding family
    letter_branding/  /letter-branding family
    template_category//template-category family
    complaint/        /complaint family
    platform_stats/   /platform-stats family
    support/          /support/find-ids
    newsletter/       /newsletter family
    report/           /service/:id/report
  service/
    notifications/
    services/
    templates/
    jobs/
    billing/
    users/
    organisations/
    inbound/
    providers/
    annual_limits/
    reports/
  repository/
    notifications/    sqlc-generated
    services/         sqlc-generated
    templates/        sqlc-generated
    jobs/             sqlc-generated
    billing/          sqlc-generated
    users/            sqlc-generated
    organisations/    sqlc-generated
    inbound/          sqlc-generated
    providers/        sqlc-generated
    api_keys/         sqlc-generated
    complaints/       sqlc-generated
    reports/          sqlc-generated
  worker/
    delivery/         deliver_sms, deliver_email, deliver_throttled_sms
    savenotify/       save_smss, save_emails (batch DB persistence)
    jobs/             process_job, process_incomplete_jobs
    receipts/         process_ses_result, process_sns_result, process_pinpoint_result
    callbacks/        send_delivery_status, send_complaint
    reporting/        create_nightly_billing, create_nightly_notification_status, etc.
    scheduled/        inbox drain tickers, nightly tasks, quarterly tasks
    manager.go        WorkerManager — starts/stops all pools
  middleware/
    requestid.go
    otel.go
    logging.go
    cors.go
    ratelimit.go
    auth.go           admin_jwt, sre_jwt, cache_clear_jwt, cypress_jwt, service_auth
    sizelimit.go
  client/
    ses/              AWS SES send
    sns/              AWS SNS SMS
    pinpoint/         AWS Pinpoint SMS V2 (us-west-2)
    s3/               S3 upload/download/presign
    sqs/              SQS send/receive/delete wrapper
    redis/            bounce rate, annual limits, batch inbox, cache
    salesforce/       Salesforce SOAP/REST engagement + contact
    freshdesk/        Freshdesk ticket creation
    airtable/         Airtable newsletter subscriber CRUD
  config/
    config.go         flat Config struct loaded from environment
  queue/
    consumer.go       SQS long-poll loop abstraction
    producer.go       SQS send abstraction
pkg/
  crypto/             encrypt/decrypt (replaces app/encryption.py)
  smsutil/            SMS fragment counting (replaces app/sms_fragment_utils.py)
  emailutil/          email daily/annual limit helpers
  pagination/         cursor-based and page-based pagination helpers
  signing/            itsdangerous-compatible HMAC signing (for signed notification blobs and callbacks)
```

---

## HTTP Layer

### Router Structure

Use **chi** (`github.com/go-chi/chi/v5`).

Route registration follows the Flask blueprint pattern: each sub-package under `internal/handler/` registers its routes onto a `chi.Router` sub-tree, and the top-level router mounts these sub-trees at the appropriate prefix.

```
Router
├── GET  /                          handler/status
├── GET  /_status                   handler/status
├── POST /_status                   handler/status
├── GET  /_status/live-service-and-organisation-counts
│
├── /invite                         → handler/invite          (admin JWT)
├── /api-key                        → handler/api_key         (admin JWT)
├── /sre-tools                      → handler/admin/sre       (SRE JWT)
├── /cache-clear                    → handler/admin/cache     (cache-clear JWT)
├── /cypress                        → handler/admin/cypress   (cypress JWT + non-prod check)
├── /complaint                      → handler/complaint       (admin JWT)
├── /email-branding                 → handler/email_branding  (admin JWT)
├── /events                         → handler/admin/events    (admin JWT)
├── /inbound-number                 → handler/inbound         (admin JWT)
├── /letter-branding                → handler/letter_branding (admin JWT)
├── /newsletter                     → handler/newsletter      (admin JWT)
│   ├── POST /unconfirmed-subscriber
│   ├── GET  /confirm/{subscriber_id}
│   ├── GET  /unsubscribe/{subscriber_id}
│   ├── POST /update-language/{subscriber_id}
│   ├── GET  /send-latest/{subscriber_id}
│   └── GET  /find-subscriber
├── /notifications                  → handler/notifications   (service auth)
├── /organisation/{id}/invite       → handler/invite          (admin JWT)
├── /organisations                  → handler/organisations   (admin JWT)
├── /platform-stats                 → handler/platform_stats  (admin JWT)
│   ├── GET  /
│   ├── GET  /usage-for-trial-services
│   ├── GET  /usage-for-all-services
│   └── GET  /send-method-stats-by-service
├── /provider-details               → handler/providers       (admin JWT)
├── /service                        → handler/services        (admin JWT)
├── /support                        → handler/support         (admin JWT)
├── /template-category              → handler/template_category (admin JWT)
│
├── /v2
│   ├── GET /openapi-en             handler/v2/openapi        (no auth)
│   ├── GET /openapi-fr             handler/v2/openapi        (no auth)
│   ├── /notifications              → handler/v2/notifications (service auth)
│   ├── /template                   → handler/v2/templates    (service auth)
│   ├── /templates                  → handler/v2/templates    (service auth)
│   └── /received-text-messages     → handler/v2/inbound      (service auth)
```

### Middleware Stack (in order, applied at top-level router)

1. **RequestID** — generate/propagate `X-Request-ID`
2. **OTEL tracing** — `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`
3. **Structured logging** — log method, path, status, latency, request ID
4. **CORS** — allowed origins: `ADMIN_BASE_URL`; allowed methods: GET, POST, PUT, DELETE; allowed headers: `Authorization`, `Content-Type`, `X-Request-ID`
5. **Rate limiting** — per-IP token bucket (configurable); per-service rate limiting for service-auth routes is enforced inside the service layer
6. **Auth** — applied per route group, not globally (see below)
7. **Request size limiting** — configurable max body size (default matches `ATTACHMENT_SIZE_LIMIT` × `ATTACHMENT_NUM_LIMIT`)

### Authentication Middleware

Each middleware is a standard `func(http.Handler) http.Handler`. Successful validation injects values into `context.Context` using typed keys.

#### AdminJWT (`middleware.RequireAdminAuth`)
- Reads `Authorization: Bearer <jwt>` header.
- Decodes JWT without verification first to extract the `iss` (issuer) claim.
- Requires `iss == config.AdminClientUserName` (`ADMIN_CLIENT_USER_NAME`, default `"notify-admin"`).
- Verifies HMAC-SHA256 signature against `ADMIN_CLIENT_SECRET`.
- Validates `exp` and `iat` claims (clock skew tolerance: 30 s).
- On failure: HTTP 401 `{"token": ["Invalid token: ...message..."]}`.
- Injects: nothing beyond verifying the caller is the admin UI.

#### SREAuth (`middleware.RequireSREAuth`)
- Same flow as AdminJWT but `iss == SRE_USER_NAME`, secret == `SRE_CLIENT_SECRET`.
- Applied only to routes under `/sre-tools`.

#### CacheClearAuth (`middleware.RequireCacheClearAuth`)
- `iss == CACHE_CLEAR_USER_NAME`, secret == `CACHE_CLEAR_CLIENT_SECRET`.
- Applied only to `/cache-clear`.

#### CypressAuth (`middleware.RequireCypressAuth`)
- `iss == CYPRESS_AUTH_USER_NAME`, secret == `CYPRESS_AUTH_CLIENT_SECRET`.
- Applied only to `/cypress`.
- At request time (not just middleware init): checks `config.NotifyEnvironment != "production"`; returns HTTP 403 if running in production.

#### ServiceAuth (`middleware.RequireAuth`)
Supports two sub-schemes selected by the `Authorization` header prefix:

**JWT path** (`Authorization: Bearer <jwt>`):
1. Decode JWT to extract `iss` (expected to be a service ID UUID).
2. Look up service and its API keys from the DB (via read replica; cache with Redis TTL).
3. Try each non-expired API key's secret (`HMAC-SHA256`) to find a match.
4. On match: inject `AuthenticatedService` (full service record) and `ApiUser` (the matched API key) into context.
5. Errors: 401 no/malformed token; 403 service not found, archived, no valid API keys, token expired or invalid.

**API key path** (`Authorization: ApiKey-v1 <plaintext_secret>`):
1. Hash the supplied secret.
2. Look up API key record by hashed secret.
3. Verify key is not expired and service is active.
4. Inject `AuthenticatedService` and `ApiUser` into context.

#### RequireNoAuth (`middleware.RequireNoAuth`)
No-op pass-through. Used for health check and OpenAPI spec endpoints.

---

## Data Layer (sqlc + golang-migrate)

### sqlc Setup

`sqlc.yaml`:
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

Query files are split one-per-domain under `db/queries/` (e.g. `db/queries/notifications.sql`, `db/queries/services.sql`). Generated output lands in `internal/repository/{domain}/` — one package per domain.

For **encrypted columns** (`notifications._personalisation`, `inbound_sms._content`, `service_callback_api.bearer_token`, `service_inbound_api.bearer_token`, `verify_codes._code`, `users._password`), sqlc generates the raw encrypted bytes. Encryption and decryption are called explicitly in the service layer using `pkg/crypto` — there are no automatic ORM hooks.

> **Note**: `inbound_sms._content` is the physical column `content` in the DB, encrypted via `signer_inbound_sms` in Python. The Go repository must decrypt on read and encrypt on write, same as the other encrypted columns.

For **history tables** (`api_keys_history`, `services_history`, `templates_history`, etc.), the repository layer contains explicit `InsertXxxHistory(...)` functions. These are called from the service layer after every mutating operation on the parent table, replicating the SQLAlchemy event-hook behaviour.

### golang-migrate Setup

- Migration directory: `db/migrations/`
- Seed migration: `db/migrations/0001_initial.sql` is derived from `specs/out.sql`.
- Subsequent schema changes are added as sequentially numbered files (`0002_...sql`, `0003_...sql`).
- CLI usage: `migrate -path db/migrations -database "$SQLALCHEMY_DATABASE_URI" up`
- Go library usage: on application startup the `cmd/api/main.go` and `cmd/worker/main.go` entry points call `runMigrations(db)` before accepting traffic. The function uses `github.com/golang-migrate/migrate/v4` with the `postgres` driver.

### Repository Package Structure

Function signatures are shown without error returns for brevity; all functions return `(T, error)` or `error`.

#### `internal/repository/notifications`
```go
GetLastTemplateUsage(ctx, templateID, templateType, serviceID uuid.UUID) (*Notification, error)
CreateNotification(ctx, n *Notification) error
BulkInsertNotifications(ctx, ns []*Notification) error
UpdateNotificationStatusByID(ctx, id uuid.UUID, status string, sentBy *string, feedbackReason *string) (*Notification, error)
UpdateNotificationStatusByReference(ctx, reference, status string) (*Notification, error)
BulkUpdateNotificationStatuses(ctx, updates []StatusUpdate) error
GetNotificationByID(ctx, id uuid.UUID) (*Notification, error)
GetNotificationsByServiceID(ctx, serviceID uuid.UUID, filters NotificationFilters) ([]*Notification, int, error)
GetNotificationsForJob(ctx, jobID uuid.UUID, filters NotificationFilters) ([]*Notification, int, error)
GetNotificationsCreatedSince(ctx, since time.Time, status string) ([]*Notification, error)
TimeoutSendingNotifications(ctx, timeout time.Duration) ([]uuid.UUID, error)
DeleteNotificationsOlderThanRetention(ctx, notificationType string) (int64, error)
GetLastNotificationAddedForJobID(ctx, jobID uuid.UUID) (*Notification, error)
GetNotificationsByReference(ctx, reference string) ([]*Notification, error)
GetHardBouncesForService(ctx, serviceID uuid.UUID, since time.Time) ([]BounceRow, error)
GetMonthlyNotificationStats(ctx, serviceID uuid.UUID, year int) ([]MonthlyStats, error)
GetTemplateUsageMonthly(ctx, serviceID uuid.UUID, year int) ([]TemplateUsageRow, error)
InsertNotificationHistory(ctx, n *NotificationHistory) error
GetNotificationFromHistory(ctx, id uuid.UUID) (*NotificationHistory, error)
GetBounceRateTimeSeries(ctx, serviceID uuid.UUID, since time.Time) ([]BounceTimeRow, error)
```

#### `internal/repository/services`
```go
GetAllServices(ctx, onlyActive bool) ([]*Service, error)
GetServicesByPartialName(ctx, name string) ([]*Service, error)
CountLiveServices(ctx) (int64, error)
GetLiveServicesData(ctx, filterHeartbeats bool) ([]LiveServiceRow, error)
GetServiceByID(ctx, id uuid.UUID, onlyActive bool) (*Service, error)
GetServiceByInboundNumber(ctx, number string) (*Service, error)
GetServiceByIDWithAPIKeys(ctx, id uuid.UUID) (*Service, error)
GetServicesByUserID(ctx, userID uuid.UUID, onlyActive bool) ([]*Service, error)
CreateService(ctx, s *Service) error
UpdateService(ctx, id uuid.UUID, fields map[string]any) error
ArchiveService(ctx, id uuid.UUID) (string, error)
SuspendService(ctx, id uuid.UUID) error
ResumeService(ctx, id uuid.UUID) error
GetServiceByIDAndUser(ctx, serviceID, userID uuid.UUID) (*Service, error)
AddUserToService(ctx, serviceID, userID uuid.UUID, permissions []string, folderPermissions []uuid.UUID) error
RemoveUserFromService(ctx, serviceID, userID uuid.UUID) error
GetServicePermissions(ctx, serviceID uuid.UUID) ([]string, error)
SetServicePermissions(ctx, serviceID uuid.UUID, permissions []string) error
GetSafelist(ctx, serviceID uuid.UUID) (*Safelist, error)
UpdateSafelist(ctx, serviceID uuid.UUID, emails, phones []string) error
GetDataRetention(ctx, serviceID uuid.UUID) ([]*ServiceDataRetention, error)
UpsertDataRetention(ctx, serviceID uuid.UUID, notificationType string, days int) error
GetSMSSenders(ctx, serviceID uuid.UUID) ([]*ServiceSmsSender, error)
CreateSMSSender(ctx, sender *ServiceSmsSender) error
UpdateSMSSender(ctx, senderID uuid.UUID, fields map[string]any) error
GetEmailReplyTo(ctx, serviceID uuid.UUID) ([]*ServiceEmailReplyTo, error)
CreateEmailReplyTo(ctx, replyTo *ServiceEmailReplyTo) error
UpdateEmailReplyTo(ctx, id uuid.UUID, fields map[string]any) error
GetCallbackAPIs(ctx, serviceID uuid.UUID, callbackType string) ([]*ServiceCallbackAPI, error)
UpsertCallbackAPI(ctx, cb *ServiceCallbackAPI) error
DeleteCallbackAPI(ctx, id uuid.UUID) error
GetInboundAPI(ctx, serviceID uuid.UUID) (*ServiceInboundAPI, error)
UpsertInboundAPI(ctx, api *ServiceInboundAPI) error
DeleteInboundAPI(ctx, id uuid.UUID) error
InsertServicesHistory(ctx, h *ServicesHistory) error
GetSensitiveServiceIDs(ctx) ([]uuid.UUID, error)
GetMonthlyDataByService(ctx, start, end time.Time) ([]MonthlyServiceRow, error)
```

#### `internal/repository/api_keys`
```go
CreateAPIKey(ctx, key *APIKey) error
GetAPIKeysByServiceID(ctx, serviceID uuid.UUID) ([]*APIKey, error)
GetAPIKeyByID(ctx, id uuid.UUID) (*APIKey, error)
RevokeAPIKey(ctx, id uuid.UUID) error
GetAPIKeyBySecret(ctx, hashedSecret string) (*APIKey, error)
UpdateAPIKeyLastUsed(ctx, id uuid.UUID, ts time.Time) error
RecordAPIKeyCompromise(ctx, id uuid.UUID, info json.RawMessage) error
GetAPIKeySummaryStats(ctx, id uuid.UUID) (*APIKeySummaryStats, error)
GetAPIKeysRankedByNotifications(ctx, nDaysBack int) ([]APIKeyRankedRow, error)
InsertAPIKeyHistory(ctx, h *APIKeyHistory) error
```

#### `internal/repository/templates`
```go
CreateTemplate(ctx, t *Template) error
GetTemplateByID(ctx, id, serviceID uuid.UUID) (*Template, error)
GetTemplateByIDAndVersion(ctx, id uuid.UUID, version int) (*TemplateHistory, error)
GetTemplatesByServiceID(ctx, serviceID uuid.UUID) ([]*Template, error)
UpdateTemplate(ctx, id uuid.UUID, fields map[string]any) error
ArchiveTemplate(ctx, id uuid.UUID) error
GetTemplateVersions(ctx, id uuid.UUID) ([]*TemplateHistory, error)
GetPrecompiledLetterTemplate(ctx, serviceID uuid.UUID) (*Template, error)
GetTemplateFolders(ctx, serviceID uuid.UUID) ([]*TemplateFolder, error)
CreateTemplateFolder(ctx, f *TemplateFolder) error
UpdateTemplateFolder(ctx, id uuid.UUID, fields map[string]any) error
DeleteTemplateFolder(ctx, id uuid.UUID) error
MoveTemplateContents(ctx, targetFolderID *uuid.UUID, folderIDs, templateIDs []uuid.UUID) error
GetTemplateCategories(ctx, templateType *string, hidden *bool) ([]*TemplateCategory, error)
GetTemplateCategoryByID(ctx, id uuid.UUID) (*TemplateCategory, error)
CreateTemplateCategory(ctx, c *TemplateCategory) error
UpdateTemplateCategory(ctx, id uuid.UUID, fields map[string]any) error
DeleteTemplateCategory(ctx, id uuid.UUID, cascade bool) error
InsertTemplateHistory(ctx, h *TemplateHistory) error
```

#### `internal/repository/jobs`
```go
CreateJob(ctx, j *Job) error
GetJobByID(ctx, id uuid.UUID) (*Job, error)
GetJobsByServiceID(ctx, serviceID uuid.UUID, filters JobFilters) ([]*Job, int, error)
UpdateJob(ctx, id uuid.UUID, fields map[string]any) error
SetScheduledJobsToPending(ctx) ([]*Job, error)
GetInProgressJobs(ctx) ([]*Job, error)
GetStalledJobs(ctx, minAge, maxAge time.Duration) ([]*Job, error)
ArchiveOldJobs(ctx, olderThan time.Duration) (int64, error)
HasJobs(ctx, serviceID uuid.UUID) (bool, error)
```

#### `internal/repository/billing`
```go
GetMonthlyBillingUsage(ctx, serviceID uuid.UUID, year int) ([]BillingRow, error)
GetYearlyBillingUsage(ctx, serviceID uuid.UUID, year int) ([]YearlyBillingRow, error)
GetFreeSMSFragmentLimit(ctx, serviceID uuid.UUID, financialYear int) (*AnnualBilling, error)
UpsertFreeSMSFragmentLimit(ctx, ab *AnnualBilling) error
UpsertFactBillingForDay(ctx, day time.Time, data []FactBillingRow) error
GetAnnualLimitsData(ctx, serviceID uuid.UUID) ([]AnnualLimitsDataRow, error)
InsertQuarterData(ctx, rows []AnnualLimitsDataRow) error
GetPlatformStatsByDateRange(ctx, start, end time.Time) ([]PlatformStatsRow, error)
GetDeliveredNotificationsByMonth(ctx, filterHeartbeats bool) ([]MonthlyDeliveryRow, error)
GetUsageForTrialServices(ctx) ([]TrialServiceUsageRow, error)
GetUsageForAllServices(ctx, start, end time.Time) ([]AllServiceUsageRow, error)
GetFactNotificationStatusForDay(ctx, day time.Time, serviceIDs []uuid.UUID) ([]FactNotifStatusRow, error)
UpsertFactNotificationStatus(ctx, day time.Time, rows []FactNotifStatusRow) error
UpsertMonthlyNotificationStatsSummary(ctx, month time.Time) error
```

#### `internal/repository/users`
```go
CreateUser(ctx, u *User) error
GetUserByID(ctx, id uuid.UUID) (*User, error)
GetUserByEmail(ctx, email string) (*User, error)
FindUsersByEmail(ctx, partialEmail string) ([]*User, error)
GetAllUsers(ctx) ([]*User, error)
UpdateUser(ctx, id uuid.UUID, fields map[string]any) error
ArchiveUser(ctx, id uuid.UUID) error
DeactivateUser(ctx, id uuid.UUID) error
ActivateUser(ctx, id uuid.UUID) error
GetUsersByServiceID(ctx, serviceID uuid.UUID) ([]*User, error)
SetUserPermissions(ctx, userID, serviceID uuid.UUID, permissions []string) error
GetUserPermissions(ctx, userID, serviceID uuid.UUID) ([]string, error)
SetFolderPermissions(ctx, userID, serviceID uuid.UUID, folderIDs []uuid.UUID) error
CreateVerifyCode(ctx, code *VerifyCode) error
GetVerifyCode(ctx, userID uuid.UUID, codeType string) (*VerifyCode, error)
MarkVerifyCodeUsed(ctx, id uuid.UUID) error
DeleteExpiredVerifyCodes(ctx) (int64, error)
CreateLoginEvent(ctx, e *LoginEvent) error
GetLoginEventsByUserID(ctx, userID uuid.UUID) ([]*LoginEvent, error)
GetFido2KeysByUserID(ctx, userID uuid.UUID) ([]*Fido2Key, error)
CreateFido2Key(ctx, key *Fido2Key) error
DeleteFido2Key(ctx, id uuid.UUID) error
CreateFido2Session(ctx, s *Fido2Session) error
GetFido2Session(ctx, sessionID uuid.UUID) (*Fido2Session, error)
```

#### `internal/repository/organisations`
```go
GetAllOrganisations(ctx) ([]*Organisation, error)
GetOrganisationByID(ctx, id uuid.UUID) (*Organisation, error)
GetOrganisationByDomain(ctx, domain string) (*Organisation, error)
CreateOrganisation(ctx, o *Organisation) error
UpdateOrganisation(ctx, id uuid.UUID, fields map[string]any) error
LinkServiceToOrganisation(ctx, orgID, serviceID uuid.UUID) error
GetServicesByOrganisationID(ctx, orgID uuid.UUID) ([]*Service, error)
AddUserToOrganisation(ctx, orgID, userID uuid.UUID) error
GetUsersByOrganisationID(ctx, orgID uuid.UUID) ([]*User, error)
IsOrganisationNameUnique(ctx, orgID uuid.UUID, name string) (bool, error)
GetInvitedOrgUsers(ctx, orgID uuid.UUID) ([]*InvitedOrganisationUser, error)
CreateInvitedOrgUser(ctx, i *InvitedOrganisationUser) error
UpdateInvitedOrgUser(ctx, id uuid.UUID, status string) error
```

#### `internal/repository/inbound`
```go
GetInboundNumbers(ctx) ([]*InboundNumber, error)
GetAvailableInboundNumbers(ctx) ([]*InboundNumber, error)
GetInboundNumberByServiceID(ctx, serviceID uuid.UUID) (*InboundNumber, error)
AddInboundNumber(ctx, number string) error
DisableInboundNumberForService(ctx, serviceID uuid.UUID) error
CreateInboundSMS(ctx, sms *InboundSMS) error
GetInboundSMSForService(ctx, serviceID uuid.UUID, phoneNumber *string, limitDays int) ([]*InboundSMS, error)
GetMostRecentInboundSMS(ctx, serviceID uuid.UUID, page int) ([]*InboundSMS, bool, error)
GetInboundSMSSummary(ctx, serviceID uuid.UUID) (*InboundSMSSummary, error)
GetInboundSMSByID(ctx, id uuid.UUID) (*InboundSMS, error)
DeleteInboundSMSOlderThan(ctx, olderThan time.Duration) (int64, error)
```

#### `internal/repository/providers`
```go
GetAllProviders(ctx) ([]*ProviderDetails, error)
GetProviderByID(ctx, id uuid.UUID) (*ProviderDetails, error)
GetProviderVersions(ctx, id uuid.UUID) ([]*ProviderDetailsHistory, error)
UpdateProvider(ctx, id uuid.UUID, fields map[string]any) error
ToggleSMSProvider(ctx) error
InsertProviderHistory(ctx, h *ProviderDetailsHistory) error
```

#### `internal/repository/complaints`
```go
CreateOrUpdateComplaint(ctx, c *Complaint) error
GetComplaintsPage(ctx, page int) ([]*Complaint, *Pagination, error)
CountComplaintsByDateRange(ctx, start, end time.Time) (int64, error)
```

#### `internal/repository/reports`
```go
CreateReport(ctx, r *Report) error
GetReportByID(ctx, id uuid.UUID) (*Report, error)
GetReportsByServiceID(ctx, serviceID uuid.UUID, limitDays int) ([]*Report, error)
UpdateReport(ctx, id uuid.UUID, fields map[string]any) error
```

---

## Background Workers (Goroutines replacing Celery)

### Worker Architecture

The `WorkerManager` struct in `internal/worker/manager.go` is responsible for the entire background processing lifecycle:

- On startup it creates goroutine pools for each SQS queue (or queue group) and registers all scheduled tasks with `robfig/cron`.
- Each pool is a configurable-size group of goroutines running the same SQS long-poll loop.
- The `WorkerManager.Start(ctx context.Context)` method launches all goroutines and returns. Calling `WorkerManager.Stop()` (or cancel of the context) drains in-progress work and shuts down cleanly.

**SQS consumer pattern** (each goroutine in a pool):
```
long-poll loop (WaitTimeSeconds=20, MaxMessages=10, VisibilityTimeout=310):
  receive batch → for each message:
    dispatch to handler function
    on success  → DeleteMessage
    on transient error → extend visibility timeout (exponential backoff)
    on permanent error → let SQS move to DLQ after max-receive count
```

**Scheduled task pattern**:
- Sub-minute tasks (inbox drain, in-flight recovery): `time.NewTicker(interval)`; each ticker runs in a dedicated goroutine.
- Minute and longer tasks: `robfig/cron` (`github.com/robfig/cron/v3`) with standard cron expressions.
- Each task function signature: `func(ctx context.Context) error`.

**Chained pipelines** (replacing Celery `chain`/`apply_async`):
- Instead of `process_job` → `save_smss` → `deliver_sms`, Go code follows this pattern within a single goroutine: read S3 CSV → bulk INSERT to DB → enqueue SQS messages to the delivery queues. There is no intermediate queue between stages: the chain only crosses queue boundaries at the points the Python code does (CSV → DB, DB → delivery).

### Worker Groups (from async-tasks.md)

| Celery worker script | Go goroutine group | Queue(s) consumed |
|---|---|---|
| `run_celery_beat.sh` | `scheduledWorker` (in-process cron + tickers) | no SQS queue; fires tasks directly |
| `run_celery.sh` (general) | `periodicWorkerPool` | `periodic-tasks` |
| `run_celery.sh` (general) | `normalTasksPool` | `normal-tasks`, `priority-tasks`, `bulk-tasks` |
| `run_celery.sh` (general) | `dbSaveWorkerPool` | `-priority-database-tasks.fifo`, `-normal-database-tasks`, `-bulk-database-tasks` |
| `run_celery.sh` (general) | `jobWorkerPool` | `job-tasks` |
| `run_celery.sh` (general) | `callbackWorkerPool` | `service-callbacks`, `service-callbacks-retry` |
| `run_celery.sh` (general) | `reportingWorkerPool` | `reporting-tasks` |
| `run_celery.sh` (general) | `retryWorkerPool` | `retry-tasks` |
| `run_celery.sh` (general) | `internalEmailPool` | `notify-internal-tasks` |
| `run_celery.sh` (general) | `researchModePool` | `research-mode-tasks` |
| `run_celery_no_sms_sending.sh` | same general pools minus SMS delivery | excludes `send-sms-*`, `send-throttled-sms-tasks` |
| `run_celery_core_tasks.sh` | `periodicWorkerPool` + `dbSaveWorkerPool` + `jobWorkerPool` | excludes all send queues |
| `run_celery_send_sms.sh` | `smsWorkerPool` | `send-sms-high`, `send-sms-medium`, `send-sms-low` |
| `run_celery_sms.sh` | `throttledSMSWorkerPool` (1 goroutine + rate limiter) | `send-throttled-sms-tasks` |
| `run_celery_send_email.sh` | `emailWorkerPool` | `send-email-high`, `send-email-medium`, `send-email-low` |
| `run_celery_delivery.sh` / `run_celery_receipts.sh` | `receiptWorkerPool` | `delivery-receipts` |
| `run_celery_report_tasks.sh` | `generateReportsPool` | `generate-reports` |
| `run_celery_local.sh` | all pools combined + embedded scheduler | all queues |

**Throttled SMS pool detail**: uses `golang.org/x/time/rate.NewLimiter(rate.Every(2*time.Second), 1)` (≤30/min) with a single goroutine. The rate limiter call blocks before each SQS `ReceiveMessage` to honour the ≤30/min cap.

**SMS delivery concurrency**: The default `CELERY_CONCURRENCY` value of 4 per pod (with multiple pods) translates to a configurable `SMS_WORKER_CONCURRENCY` env var (default 4) per binary instance. Horizontal scaling is handled at the container/pod level.

### Scheduled Tasks

| Beat task | Schedule (UTC) | Go goroutine function |
|---|---|---|
| `beat-inbox-sms-normal` | every 10 s | `ticker.InboxDrainSMSNormal()` |
| `beat-inbox-sms-bulk` | every 10 s | `ticker.InboxDrainSMSBulk()` |
| `beat-inbox-sms-priority` | every 10 s | `ticker.InboxDrainSMSPriority()` |
| `beat-inbox-email-normal` | every 10 s | `ticker.InboxDrainEmailNormal()` |
| `beat-inbox-email-bulk` | every 10 s | `ticker.InboxDrainEmailBulk()` |
| `beat-inbox-email-priority` | every 10 s | `ticker.InboxDrainEmailPriority()` |
| `in-flight-to-inbox` | every 60 s | `ticker.RecoverExpiredNotifications()` |
| `run-scheduled-jobs` | `* * * * *` | `cron.RunScheduledJobs()` |
| `mark-jobs-complete` | `* * * * *` | `cron.MarkJobsComplete()` |
| `check-job-status` | `* * * * *` | `cron.CheckJobStatus()` |
| `replay-created-notifications` | `0,15,30,45 * * * *` | `cron.ReplayCreatedNotifications()` |
| `delete-verify-codes` | `@every 63m` | `cron.DeleteVerifyCodes()` |
| `delete-invitations` | `@every 66m` | `cron.DeleteInvitations()` |
| `timeout-sending-notifications` | `5 5 * * *` | `cron.TimeoutSendingNotifications()` |
| `create-nightly-billing` | `15 5 * * *` | `cron.CreateNightlyBilling()` |
| `create-nightly-notification-status` | `30 5 * * *` | `cron.CreateNightlyNotificationStatus()` |
| `create-monthly-notification-stats-summary` | `30 6 * * *` | `cron.CreateMonthlyNotificationStatsSummary()` |
| `delete-inbound-sms` | `40 6 * * *` | `cron.DeleteInboundSMS()` |
| `send-daily-performance-platform-stats` | `0 7 * * *` | `cron.SendDailyPerformancePlatformStats()` |
| `remove_transformed_dvla_files` | `40 8 * * *` | `cron.RemoveTransformedDVLAFiles()` |
| `remove_sms_email_jobs` | `0 9 * * *` | `cron.RemoveSMSEmailJobs()` |
| `delete-sms-notifications` | `15 9 * * *` | `cron.DeleteSMSNotifications()` |
| `delete-email-notifications` | `30 9 * * *` | `cron.DeleteEmailNotifications()` |
| `delete-letter-notifications` | `45 9 * * *` | `cron.DeleteLetterNotifications()` |
| `insert-quarter-data-q1` | `0 23 1 7 *` | `cron.InsertQuarterDataForAnnualLimits()` |
| `insert-quarter-data-q2` | `0 23 1 10 *` | `cron.InsertQuarterDataForAnnualLimits()` |
| `insert-quarter-data-q3` | `0 23 1 1 *` | `cron.InsertQuarterDataForAnnualLimits()` |
| `insert-quarter-data-q4` | `0 23 1 4 *` | `cron.InsertQuarterDataForAnnualLimits()` |
| `send-quarterly-email-q1` | `0 23 2 7 *` | `cron.SendQuarterlyEmail()` |
| `send-quarterly-email-q2` | `0 23 2 10 *` | `cron.SendQuarterlyEmail()` |
| `send-quarterly-email-q3` | `0 23 3 1 *` | `cron.SendQuarterlyEmail()` |

> **Note**: `send-quarterly-email-q4` (April) is absent from the Python beat schedule — this appears to be a bug. The Go implementation should add `0 23 2 4 *` for consistency.

---

## External Service Clients

### AWS SES
- **Go package**: `internal/client/ses`
- **Interface**:
  ```go
  type SESClient interface {
      SendRawEmail(ctx context.Context, input *sesv2.SendEmailInput) (string, error)
      SendEmail(ctx context.Context, input *sesv2.SendEmailInput) (string, error)
  }
  ```
- **Config**: `AWS_SES_REGION` (default `us-east-1`), `AWS_SES_ACCESS_KEY`, `AWS_SES_SECRET_KEY`

### AWS SNS
- **Go package**: `internal/client/sns`
- **Interface**:
  ```go
  type SNSClient interface {
      PublishSMS(ctx context.Context, phoneNumber, message string, attrs map[string]string) (string, error)
  }
  ```
- **Config**: `AWS_REGION` (default `us-east-1`)

### AWS Pinpoint SMS V2
- **Go package**: `internal/client/pinpoint`
- **Interface**:
  ```go
  type PinpointClient interface {
      SendTextMessage(ctx context.Context, input *PinpointSMSInput) (*PinpointSMSOutput, error)
  }
  ```
- **Config**: `AWS_PINPOINT_REGION` (default `us-west-2`), `AWS_PINPOINT_SC_POOL_ID`, `AWS_PINPOINT_DEFAULT_POOL_ID`, `AWS_PINPOINT_CONFIGURATION_SET_NAME`, `AWS_PINPOINT_SC_TEMPLATE_IDS`, `AWS_US_TOLL_FREE_NUMBER`

### AWS S3
- **Go package**: `internal/client/s3`
- **Interface**:
  ```go
  type S3Client interface {
      GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error)
      PutObject(ctx context.Context, bucket, key string, body io.Reader) error
      DeleteObject(ctx context.Context, bucket, key string) error
      GeneratePresignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
      StreamCSVToWriter(ctx context.Context, bucket, key string, w io.Writer) error
  }
  ```
- **Config**: `AWS_REGION`, `CSV_UPLOAD_BUCKET_NAME`, `REPORTS_BUCKET_NAME`, `GC_ORGANISATIONS_BUCKET_NAME`

### AWS SQS
- **Go package**: `internal/client/sqs`
- **Interface**:
  ```go
  type SQSClient interface {
      SendMessage(ctx context.Context, queueURL, body string, attrs map[string]string) error
      SendMessageFIFO(ctx context.Context, queueURL, body, groupID, deduplicationID string) error
      ReceiveMessages(ctx context.Context, queueURL string, maxMessages, waitSeconds int32) ([]sqstypes.Message, error)
      DeleteMessage(ctx context.Context, queueURL, receiptHandle string) error
      ChangeMessageVisibility(ctx context.Context, queueURL, receiptHandle string, timeout int32) error
  }
  ```
- **Config**: `AWS_REGION`, `NOTIFICATION_QUEUE_PREFIX`

### Redis
- **Go package**: `internal/client/redis`
- **Interface**:
  ```go
  type RedisClient interface {
      Get(ctx context.Context, key string) (string, error)
      Set(ctx context.Context, key string, value any, ttl time.Duration) error
      Delete(ctx context.Context, keys ...string) error
      DeleteByPattern(ctx context.Context, pattern string) error
      ZAdd(ctx context.Context, key string, members ...redis.Z) error
      ZRangeByScore(ctx context.Context, key, min, max string) ([]string, error)
      Incr(ctx context.Context, key string) (int64, error)
      IncrBy(ctx context.Context, key string, value int64) (int64, error)
      Expire(ctx context.Context, key string, ttl time.Duration) error
      // Inbox (batch-save) operations
      LPush(ctx context.Context, key string, values ...any) error
      LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
      LTrim(ctx context.Context, key string, start, stop int64) error
      // In-flight tracking
      SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
      Keys(ctx context.Context, pattern string) ([]string, error)
  }
  ```
- **Config**: `REDIS_URL`, `REDIS_ENABLED`, `CACHE_OPS_URL`

### Salesforce
- **Go package**: `internal/client/salesforce`
- **Interface**:
  ```go
  type SalesforceClient interface {
      CreateEngagement(ctx context.Context, service *ServiceEngagement) error
      UpdateEngagement(ctx context.Context, serviceID string, fields map[string]any) error
      CloseEngagement(ctx context.Context, serviceID string) error
      CreateContact(ctx context.Context, user *UserContact) error
      UpdateContact(ctx context.Context, userID string, fields map[string]any) error
  }
  ```
- **Config**: `SALESFORCE_DOMAIN`, `SALESFORCE_CLIENT_ID`, `SALESFORCE_USERNAME`, `SALESFORCE_PASSWORD`, `SALESFORCE_SECURITY_TOKEN`, `SALESFORCE_ENGAGEMENT_PRODUCT_ID`, `SALESFORCE_ENGAGEMENT_RECORD_TYPE`, `SALESFORCE_ENGAGEMENT_STANDARD_PRICEBOOK_ID`, `SALESFORCE_GENERIC_ACCOUNT_ID`

### Freshdesk
- **Go package**: `internal/client/freshdesk`
- **Interface**:
  ```go
  type FreshdeskClient interface {
      CreateTicket(ctx context.Context, ticket *FreshdeskTicket) (*FreshdeskTicketResponse, error)
  }
  ```
- **Config**: `FRESH_DESK_API_URL`, `FRESH_DESK_API_KEY`, `FRESH_DESK_PRODUCT_ID`, `FRESH_DESK_ENABLED`

### Airtable
- **Go package**: `internal/client/airtable`
- **Interface**:
  ```go
  type AirtableClient interface {
      FindSubscriberByEmail(ctx context.Context, email string) (*AirtableRecord, error)
      CreateSubscriber(ctx context.Context, email, language string) (*AirtableRecord, error)
      ConfirmSubscriber(ctx context.Context, recordID string) (*AirtableRecord, error)
      UnsubscribeSubscriber(ctx context.Context, recordID string) (*AirtableRecord, error)
  }
  ```
- **Config**: `AIRTABLE_API_KEY`, `AIRTABLE_NEWSLETTER_BASE_ID`, `AIRTABLE_NEWSLETTER_TABLE_NAME`, `AIRTABLE_CURRENT_NEWSLETTER_TEMPLATES_TABLE_NAME`

---

## Configuration

All configuration is loaded at startup into a single flat `Config` struct in `internal/config/config.go`. Use `github.com/caarlos0/env/v11` (or equivalent) to parse environment variables with default values. The struct is passed through the application via dependency injection (not global state).

### Database
| Go field | Env var | Type | Notes |
|---|---|---|---|
| `DatabaseURI` | `SQLALCHEMY_DATABASE_URI` | `string` | Primary writer DSN |
| `DatabaseReaderURI` | `SQLALCHEMY_DATABASE_READER_URI` | `string` | Optional read replica DSN |
| `DBPoolSize` | `SQLALCHEMY_POOL_SIZE` | `int` | Default 5 |
| `DBPoolRecycle` | — | `time.Duration` | Default 300 s |

### Redis
| Go field | Env var | Type | Notes |
|---|---|---|---|
| `RedisURL` | `REDIS_URL` | `string` | |
| `CacheOpsURL` | `CACHE_OPS_URL` | `string` | Defaults to `REDIS_URL` |
| `RedisEnabled` | `REDIS_ENABLED` | `bool` | Default false |

### AWS
| Go field | Env var | Type | Notes |
|---|---|---|---|
| `AWSRegion` | `AWS_REGION` | `string` | Default `us-east-1` |
| `AWSSESRegion` | `AWS_SES_REGION` | `string` | Default `us-east-1` |
| `AWSSESAccessKey` | `AWS_SES_ACCESS_KEY` | `string` | |
| `AWSSESSecretKey` | `AWS_SES_SECRET_KEY` | `string` | |
| `AWSPinpointRegion` | `AWS_PINPOINT_REGION` | `string` | Default `us-west-2` |
| `AWSPinpointSCPoolID` | `AWS_PINPOINT_SC_POOL_ID` | `string` | |
| `AWSPinpointDefaultPoolID` | `AWS_PINPOINT_DEFAULT_POOL_ID` | `string` | |
| `AWSPinpointConfigSet` | `AWS_PINPOINT_CONFIGURATION_SET_NAME` | `string` | Default `pinpoint-configuration` |
| `AWSPinpointSCTemplateIDs` | `AWS_PINPOINT_SC_TEMPLATE_IDS` | `[]string` | Comma-separated |
| `AWSUSTollFreeNumber` | `AWS_US_TOLL_FREE_NUMBER` | `string` | |
| `CSVUploadBucket` | `CSV_UPLOAD_BUCKET_NAME` | `string` | |
| `ReportsBucket` | `REPORTS_BUCKET_NAME` | `string` | |
| `GCOrganisationsBucket` | `GC_ORGANISATIONS_BUCKET_NAME` | `string` | |
| `GCOrganisationsFilename` | `GC_ORGANISATIONS_FILENAME` | `string` | Default `all.json` |

### SQS / Worker
| Go field | Env var | Type | Notes |
|---|---|---|---|
| `NotificationQueuePrefix` | `NOTIFICATION_QUEUE_PREFIX` | `string` | Prepended to all queue names |
| `CeleryDeliverSMSRateLimit` | `CELERY_DELIVER_SMS_RATE_LIMIT` | `string` | e.g. `"1/s"` |
| `BatchInsertionChunkSize` | `BATCH_INSERTION_CHUNK_SIZE` | `int` | Default 500 |
| `SMSWorkerConcurrency` | `CELERY_CONCURRENCY` | `int` | Default 4 |
| `SendingNotificationsTimeout` | — | `time.Duration` | Default 259200 s (3 days) |

### Auth
| Go field | Env var | Type | Notes |
|---|---|---|---|
| `AdminBaseURL` | `ADMIN_BASE_URL` | `string` | Default `http://localhost:6012` |
| `AdminClientSecret` | `ADMIN_CLIENT_SECRET` | `string` | Required |
| `AdminClientUserName` | — | `string` | Hardcoded `"notify-admin"` |
| `SecretKey` | `SECRET_KEY` | `[]string` | Comma-separated list |
| `DangerousSalt` | `DANGEROUS_SALT` | `string` | Required |
| `SREClientSecret` | `SRE_CLIENT_SECRET` | `string` | |
| `SREUserName` | `SRE_USER_NAME` | `string` | |
| `CacheClearClientSecret` | `CACHE_CLEAR_CLIENT_SECRET` | `string` | |
| `CacheClearUserName` | `CACHE_CLEAR_USER_NAME` | `string` | |
| `CypressAuthClientSecret` | `CYPRESS_AUTH_CLIENT_SECRET` | `string` | |
| `CypressAuthUserName` | `CYPRESS_AUTH_USER_NAME` | `string` | |
| `CypressUserPWSecret` | `CYPRESS_USER_PW_SECRET` | `string` | |
| `APIKeyPrefix` | — | `string` | Hardcoded `"gcntfy-"` |

### Features Flags (see also next section)
| Go field | Env var | Type | Default |
|---|---|---|---|
| `FFUseBillableUnits` | `FF_USE_BILLABLE_UNITS` | `bool` | false |
| `FFSalesforceContact` | `FF_SALESFORCE_CONTACT` | `bool` | false |
| `FFUsePinpointForDedicated` | `FF_USE_PINPOINT_FOR_DEDICATED` | `bool` | false |
| `FFBounceRateSeedEpochMs` | `FF_BOUNCE_RATE_SEED_EPOCH_MS` | `int64` | 0 |
| `FFPTServiceSkipFreshdesk` | `FF_PT_SERVICE_SKIP_FRESHDESK` | `bool` | false |
| `FFEnableOtel` | `FF_ENABLE_OTEL` | `bool` | false |

### External Services
| Go field | Env var | Type |
|---|---|---|
| `FreshDeskAPIURL` | `FRESH_DESK_API_URL` | `string` |
| `FreshDeskAPIKey` | `FRESH_DESK_API_KEY` | `string` |
| `FreshDeskProductID` | `FRESH_DESK_PRODUCT_ID` | `string` |
| `FreshDeskEnabled` | `FRESH_DESK_ENABLED` | `bool` |
| `AirtableAPIKey` | `AIRTABLE_API_KEY` | `string` |
| `AirtableNewsletterBaseID` | `AIRTABLE_NEWSLETTER_BASE_ID` | `string` |
| `AirtableNewsletterTableName` | `AIRTABLE_NEWSLETTER_TABLE_NAME` | `string` |
| `AirtableCurrentTemplatesTable` | `AIRTABLE_CURRENT_NEWSLETTER_TEMPLATES_TABLE_NAME` | `string` |
| `SalesforceDomain` | `SALESFORCE_DOMAIN` | `string` |
| `SalesforceClientID` | `SALESFORCE_CLIENT_ID` | `string` |
| `SalesforceUsername` | `SALESFORCE_USERNAME` | `string` |
| `SalesforcePassword` | `SALESFORCE_PASSWORD` | `string` |
| `SalesforceSecurityToken` | `SALESFORCE_SECURITY_TOKEN` | `string` |

### Application
| Go field | Env var | Type | Default |
|---|---|---|---|
| `NotifyEnvironment` | `NOTIFY_ENVIRONMENT` | `string` | `"development"` |
| `APIHostName` | `API_HOST_NAME` | `string` | |
| `DocumentationDomain` | `DOCUMENTATION_DOMAIN` | `string` | `"documentation.notification.canada.ca"` |
| `InvitationExpirationDays` | — | `int` | 2 |
| `PageSize` | — | `int` | 50 |
| `APIPageSize` | — | `int` | 250 |
| `MaxVerifyCodeCount` | — | `int` | 10 |
| `JobsMaxScheduleHoursAhead` | — | `int` | 96 |
| `FailedLoginLimit` | `FAILED_LOGIN_LIMIT` | `int` | 10 |
| `AttachmentNumLimit` | `ATTACHMENT_NUM_LIMIT` | `int` | 10 |
| `AttachmentSizeLimit` | `ATTACHMENT_SIZE_LIMIT` | `int` | 10485760 (10 MB) |
| `PersonalisationSizeLimit` | `PERSONALISATION_SIZE_LIMIT` | `int` | 51200 (50 KB) |
| `AllowHTMLServiceIDs` | `ALLOW_HTML_SERVICE_IDS` | `[]string` | `[]` |
| `DaysBeforeReportsExpire` | `DAYS_BEFORE_REPORTS_EXPIRE` | `int` | 3 |
| `StatsDHost` | `STATSD_HOST` | `string` | |
| `StatsDPort` | — | `int` | 8125 |
| `StatsDEnabled` | — | `bool` | derived from StatsDHost != "" |
| `OtelRequestMetricsEnabled` | `OTEL_REQUEST_METRICS_ENABLED` | `bool` | false |
| `CronitorEnabled` | — | `bool` | false |
| `CronitorKeys` | `CRONITOR_KEYS` | `map[string]string` | JSON |
| `ScanForPII` | `SCAN_FOR_PII` | `bool` | false |
| `CSVBulkRedirectThreshold` | `CSV_BULK_REDIRECT_THRESHOLD` | `int` | configurable |

### Well-known internal UUIDs (hardcoded constants, not env vars)
```go
const (
    NotifyServiceID        = "d6aa2c68-a2d9-4437-ab19-3ae8eb202553"
    HeartbeatServiceID     = "30b2fb9c-f8ad-49ad-818a-ed123fc00758"
    NewsletterServiceID    = "143806ca-3068-4f5d-9c6d-276b4151a395"
    NotifyUserID           = "6af522d0-2915-4e52-83a3-3690455a5fe6"

    CypressServiceID       = "d4e8a7f4-2b8a-4c9a-8b3f-9c2d4e8a7f4b"
    CypressTestUserID      = "e5f9d8c7-3a9b-4d8c-9b4f-8d3e5f9d8c7a"
    CypressTestUserAdminID = "4f8b8b1e-9c4f-4d8b-8b1e-4f8b8b1e9c4f"

    // Notify internal template IDs (subset — add remaining from config.py)
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

## Feature Flags

| Flag | Go config field | Env var | Purpose |
|---|---|---|---|
| `FF_USE_BILLABLE_UNITS` | `FFUseBillableUnits` | `FF_USE_BILLABLE_UNITS` | When true, SMS daily/annual limit checks count SMS fragments instead of messages; affects POST /v2/notifications/sms, POST /v2/notifications/bulk, and the v1 SMS endpoint |
| `FF_SALESFORCE_CONTACT` | `FFSalesforceContact` | `FF_SALESFORCE_CONTACT` | Mirror service lifecycle events (create, go-live, update name, archive) and user events (activate, update) to Salesforce |
| `FF_USE_PINPOINT_FOR_DEDICATED` | `FFUsePinpointForDedicated` | `FF_USE_PINPOINT_FOR_DEDICATED` | Route dedicated long-number SMS through AWS Pinpoint SMS V2 (us-west-2) and the throttled worker instead of SNS |
| `FF_BOUNCE_RATE_SEED_EPOCH_MS` | `FFBounceRateSeedEpochMs` | `FF_BOUNCE_RATE_SEED_EPOCH_MS` | When set to a Unix epoch (ms), triggers bounce-rate Redis seeding for a service within 24 hours of that timestamp |
| `FF_PT_SERVICE_SKIP_FRESHDESK` | `FFPTServiceSkipFreshdesk` | `FF_PT_SERVICE_SKIP_FRESHDESK` | For province/territory services: route contact requests to a secure email address instead of Freshdesk |
| `FF_ENABLE_OTEL` | `FFEnableOtel` | `FF_ENABLE_OTEL` | Use OpenTelemetry for distributed tracing/metrics; when false, falls back to AWS X-Ray SDK |

---

## Key Divergences from Python Implementation

### No SQLAlchemy ORM
All queries are explicit SQL written in `db/queries/*.sql` files, processed by sqlc to generate type-safe Go code. There are no lazy-loaded relationships, no `SELECT *`, and no N+1 query risks from relationship traversal.

### No Celery
The 53 Celery tasks and 31 beat entries are replaced by goroutines. Goroutine pools read directly from SQS queues; the `robfig/cron` scheduler and `time.Ticker` replace beat. There is no separate broker process, no task serialisation format to maintain, and no Celery retry state — retry logic is handled by SQS visibility timeout extension and dead-letter queues.

### No Flask Blueprints
Flask blueprints → chi router groups. Each handler sub-package registers its own routes. Middleware is composed at the group level (e.g. all routes under `/service/{service_id}` share the admin JWT middleware).

### Marshmallow → `encoding/json` with struct tags
Request bodies are decoded with `json.NewDecoder` into typed request structs. Validation is performed by explicit Go code (or a validation library such as `github.com/go-playground/validator/v10`) rather than Marshmallow schemas. Response bodies are encoded with `json.NewEncoder`. Field names match the Python schema field names exactly via `json:"..."` struct tags.

### `notifications_utils` → Internal Go packages
The Python `notifications_utils` shared library is replaced by `pkg/` packages:
- `pkg/smsutil` — SMS fragment counting (GSM7/UCS-2 detection, fragmentation math)
- `pkg/emailutil` — email daily/annual limit computation helpers
- `pkg/crypto` — `itsdangerous`-compatible HMAC signing for notification blobs and callback tokens
- `pkg/pagination` — cursor-based and page-based pagination

### History / Versioning Tables
SQLAlchemy's `history_meta.py` event hooks automatically wrote to `*_history` tables on every UPDATE. In Go, history inserts are explicit calls in the service layer: every `service.UpdateTemplate(...)` call also calls `repository.InsertTemplateHistory(...)` inside the same database transaction. This is more explicit and easier to audit.

### Encrypted Columns
Python used SQLAlchemy column type wrappers (`Encryption`) to transparently encrypt/decrypt `_personalisation`, `bearer_token`, `_code`, and `_password`. In Go, sqlc returns plain `[]byte` for these columns. Encryption is applied explicitly in the service layer:
- Before INSERT: `crypto.Encrypt(value, config.SecretKey[0])` → store ciphertext
- After SELECT: `crypto.Decrypt(ciphertext, config.SecretKeys)` → use plaintext
- Key rotation: `SecretKey` is a list; encryption always uses the first key; decryption tries all keys in order.

> **⚠️ Validation gap**: `inbound_sms._content` (physical column: `content`) was missing from earlier encrypted-column lists. It must also go through `crypto.Decrypt`/`crypto.Encrypt` in the inbound-SMS repository functions.

### Read Replica Routing
Python used `db.on_reader()` context manager for queries explicitly directed to the read replica. In Go this is achieved by injecting two `*sql.DB` instances (writer and reader) into the repository layer. Functions that must use the reader (e.g. `GetServiceByIDWithAPIKeys` called during request authentication) receive the reader `*sql.DB`; all writes and reads that require up-to-date data use the writer.

**Guidance for Go repository design — use reader for:**
- Request-time service/API-key lookup (`GetServiceByIDWithAPIKeys`, `GetAPIKeyBySecret`)
- Read-only list/fetch endpoints where eventual consistency is acceptable (e.g. `ListTemplates`, `GetNotificationsByService`)
- `ft_billing` / `ft_notification_status` fact-table queries (nightly-aggregated, no write path)
- `annual_billing`, `annual_limits_data` reads during limit-check flows

**Always use writer for:**
- Any INSERT, UPDATE, or DELETE
- Reads immediately following a write within the same request (e.g. fetching a row just inserted)
- Auth-critical reads where stale cache would be a security risk (token verification, permission checks)
- Rate-limit counter reads/writes (Redis preferred; DB fallback must use writer)

### Simulated Phone Numbers / Email Addresses
Python had a list of hard-coded simulated addresses that skip actual delivery. In Go these are constants in `internal/service/notifications/simulate.go`. Notifications for simulated recipients are persisted to the DB but not enqueued to SQS delivery queues.

### Letter Features
The Python codebase contains substantial letter-related code that is **either dead (PDF stubs) or not active in the Canadian deployment**. The initial Go rewrite should omit letter delivery (`create-letters-pdf`, DVLA pipeline, precompiled letter S3 flow) entirely, retaining only the stub endpoints required to return correct HTTP status codes for any clients that call them.

**Letter stub endpoints — implement as 501 Not Implemented in Go:**

| Method | Path | Handler decision |
|---|---|---|
| `GET` | `/service/{service_id}/letter-contact` | 501 stub |
| `GET` | `/service/{service_id}/letter-contact/{letter_contact_id}` | 501 stub |
| `POST` | `/service/{service_id}/letter-contact` | 501 stub |
| `POST` | `/service/{service_id}/letter-contact/{letter_contact_id}` | 501 stub |
| `POST` | `/service/{service_id}/letter-contact/{letter_contact_id}/archive` | 501 stub |
| `POST` | `/service/{service_id}/send-pdf-letter` | 501 stub |
| `POST` | `/letters/returned` | 501 stub |

All 7 routes must be registered (so clients receive 501 rather than 404) but no business logic is required.

---

## Service Layer

The `internal/service/{domain}/` packages sit between HTTP handlers and the repository layer. Handlers parse and validate requests, call service functions, then encode responses. Service functions own transaction boundaries, orchestrate cross-repository operations, and apply business rules from `specs/business-rules/{domain}.md`.

### Pattern

```go
// internal/service/notifications/service.go
type Service struct {
    db       *sql.DB        // writer
    reader   *sql.DB        // read replica
    notifRepo  repository.NotificationsQuerier
    jobRepo    repository.JobsQuerier
    svcRepo    repository.ServicesQuerier
    sqs      queue.Producer
    redis    client.RedisClient
    cfg      *config.Config
}

func NewService(db, reader *sql.DB, ..., cfg *config.Config) *Service { ... }
```

Services are constructed once in `cmd/api/main.go` (or `cmd/worker/main.go`) via dependency injection. There is no global state; all dependencies flow in through struct fields.

### Transaction Boundaries

Repository functions each accept a `*sql.Tx` or use the injected `*sql.DB` directly. For multi-step operations that must be atomic (e.g. create template + insert template history), the service opens a `db.BeginTx(ctx, nil)`, calls the relevant repository functions passing the `*sql.Tx`, then `Commit()` or `Rollback()` on error.

```go
tx, err := s.db.BeginTx(ctx, nil)
// ...
if err := s.notifRepo.WithTx(tx).CreateNotification(ctx, n); err != nil { tx.Rollback(); return err }
if err := s.notifRepo.WithTx(tx).InsertNotificationHistory(ctx, n.toHistory()); err != nil { tx.Rollback(); return err }
return tx.Commit()
```

### Responsibility Boundary

| Layer | Owns |
|---|---|
| `internal/handler/` | HTTP decode, input validation, HTTP encode, HTTP status selection |
| `internal/service/` | Business rules, rate/limit checks, transaction coordination, SQS enqueue decisions, history writes, Salesforce/Freshdesk side-effects |
| `internal/repository/` | SQL execution only — no business logic |
| `internal/worker/` | SQS message decode, call service functions, SQS ack/nack |

The full per-domain business logic each service package must implement is documented in `specs/business-rules/{domain}.md`.

---

## HTTP Error Response Format

The Go implementation must preserve the existing wire format exactly, because the admin UI and v2 API clients depend on it.

### Admin API (internal routes — non-`/v2/`)

**Standard error body** (`{"result": "error", "message": ...}`)

```json
{ "result": "error", "message": "No result found" }
```

`message` is a string for simple errors, or an object/array (field → message list) for validation errors:

```json
{ "result": "error", "message": { "name": ["Missing data for required field."] } }
```

### Public v2 API (`/v2/` routes)

**v2 error body** (`{"status_code": N, "errors": [...]}`)

```json
{
  "status_code": 400,
  "errors": [
    { "error": "ValidationError", "message": "template_id is not a valid UUID" }
  ]
}
```

### Status Code Mapping

| Condition | Status | Body format |
|---|---|---|
| `NoResultFound` / `DataError` | 404 | admin format |
| `InvalidRequest` subclass | varies (400/403/409) | admin format via `to_dict()` |
| Marshmallow/jsonschema validation | 400 | admin format |
| `AuthError` | 401/403 | admin format |
| `ArchiveValidationError` | 400 | admin format |
| Rate limit exceeded | 429 | admin format |
| Unhandled exception | 500 | `{"result": "error", "message": "Internal server error"}` |
| v2 validation error | 400 | v2 format |
| v2 auth error | 401/403 | v2 format |

### Go Implementation

Define a small set of sentinel error types in `internal/handler/errors.go`:

```go
type APIError struct {
    StatusCode int
    Message    any    // string or map[string][]string
}

type V2Error struct {
    StatusCode int
    ErrorType  string // class name e.g. "ValidationError", "AuthError"
    Message    string
}
```

Each handler sub-package registers a `chi` middleware (or uses a shared `WriteError` helper) that inspects the returned error type and writes the correct JSON body + status code. Admin routes use `APIError`; v2 routes use `V2Error`. Unrecognised errors log at ERROR level and return a generic 500.
