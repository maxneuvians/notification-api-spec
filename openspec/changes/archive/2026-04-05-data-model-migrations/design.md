## Context

The seed migration (`db/migrations/0001_initial.sql`) captured in `go-project-setup` creates the full 68-table schema across 10 domain groups. This change adds SQL query files for all 14 domain query modules under `db/queries/` so that `sqlc generate` produces typed Go repository packages in `internal/repository/`. The generated packages are the only permitted data-access path — raw `database/sql` calls outside the repository layer are forbidden in all domain service and handler code.

The 68 tables span: notifications (6), services (16), users (10), templates (7), billing (7), organisations (5), providers (3), inbound SMS (2), auth/security (6), branding (3), analytics/reporting (4). Key structural concerns are: 6 versioned entity families with history tables, 8 encrypted columns across 5 tables, 6 soft-delete entity types, and 4 denormalised fact tables whose write pattern requires `INSERT ... ON CONFLICT DO UPDATE`.

## Goals / Non-Goals

**Goals:**
- Produce 14 `db/queries/*.sql` files covering all 14 domain query modules
- `sqlc generate` produces compilable `internal/repository/<domain>/` packages for all 14 domains
- Document and enforce the history-write pattern for all 6 versioned entities
- Document and enforce the encrypted-column read/write protocol for all 8 encrypted columns
- Document and enforce the soft-delete convention for all 6 soft-delete entity types
- Define idempotent upsert queries for all 4 denormalised fact tables
- Confirm uuid/jsonb/timestamptz sqlc type overrides work correctly end-to-end
- Establish read-replica routing guidance for repository functions

**Non-Goals:**
- Implementing any handler or service-layer logic
- Adding Redis caching over repository calls (domain changes add caching where needed)
- Configuring connection pool sizing or read-replica DSN parsing (config concerns in go-project-setup)
- Future schema migrations beyond the seed (each domain change adds its own .sql migration files)

## Decisions

### D1 — 14 query files, one per domain module under `db/queries/`
Query files: `notifications.sql`, `services.sql`, `api_keys.sql`, `templates.sql`, `jobs.sql`, `billing.sql`, `users.sql`, `organisations.sql`, `inbound_sms.sql`, `providers.sql`, `complaints.sql`, `reports.sql`, `annual_limits.sql`, `template_categories.sql`. sqlc generates one Go package per domain in `internal/repository/<domain>/`, keeping package sizes manageable and preventing import cycles.

### D2 — History tables: explicit `InsertXxxHistory` functions, never implicit
Python SQLAlchemy fires event hooks after every UPDATE. Go has no equivalent. Each service-layer mutating function on a versioned entity MUST call the corresponding history function as the next statement within the same `*sql.Tx`. If the mutation succeeds but history insert fails, the transaction rolls back. The 6 versioned entities are: `api_keys` → `api_keys_history`, `provider_details` → `provider_details_history`, `service_callback_api` → `service_callback_api_history`, `service_inbound_api` → `service_inbound_api_history`, `services` → `services_history`, `templates` → `templates_history`. All history tables have composite PK `(id, version)` and carry no FK constraints.

### D3 — Encrypted columns: explicit service-layer calls, no repository-layer magic
sqlc surfaces encrypted columns as `[]byte` or `string`. Repository functions receive and return raw encrypted bytes. Callers (service layer) call `pkg/crypto.Decrypt` on read and `pkg/crypto.Encrypt` on write. The 8 encrypted columns are: `notifications._personalisation`, `notifications.to` (SensitiveString), `notifications.normalised_to` (SensitiveString), `inbound_sms._content` (physical column: `content`), `service_callback_api.bearer_token`, `service_inbound_api.bearer_token`, `verify_codes._code`, `users._password`. Key rotation: `SecretKey` is a list; encryption always uses `SecretKey[0]`; decryption tries all keys in order.

### D4 — `notify_status_type` native enum is dead code; `job_status_types` enum defers to lookup table
`notify_status_type` is defined in DDL but never referenced by application code. The generated `NotifyStatusType` Go type MUST NOT appear in any production code path. `job_status_types` DDL enum has only 4 values; the `job_status` lookup table has 9. Production code always uses the 9-value lookup table string constants; the generated `JobStatusTypes` Go type must not be used for job status fields.

### D5 — Soft-delete: filtered by default; explicit opt-in to include archived
Six entities soft-delete rather than physically delete. Default list queries exclude soft-deleted records: `api_keys` filters `WHERE expiry_date IS NULL`; `jobs`, `service_email_reply_to`, `service_letter_contacts`, `service_sms_senders`, `templates` filter `WHERE archived = false`. Functions may accept an `IncludeArchived`/`IncludeExpired` option to include them explicitly.

### D6 — Fact tables use `INSERT ... ON CONFLICT DO UPDATE`
`ft_billing`, `ft_notification_status`, `monthly_notification_stats_summary`, and `annual_limits_data` are written by nightly batch workers that may be re-run. All write functions (`UpsertFactBillingForDay`, `UpsertFactNotificationStatus`, `UpsertMonthlyNotificationStatsSummary`) MUST use upsert semantics so a second run updates rather than fails with a duplicate-key error.

### D7 — Read-replica routing: reader for auth-path lookups and reporting; writer for all mutations
`GetServiceByIDWithAPIKeys` and `GetAPIKeyBySecret` (called on every authenticated request) use the reader `*sql.DB`. Read-only listing and reporting queries use the reader where eventual consistency is acceptable. All INSERTs, UPDATEs, DELETEs, and reads immediately following a write use the writer `*sql.DB`.

## Risks / Trade-offs

- **sqlc version drift** → Pin the sqlc binary version in `Makefile` and commit the lockfile. Regenerating with a different version may produce different struct names or method signatures, breaking callers.
- **Encrypted column type confusion** → If a developer queries an encrypted column and returns it to a handler without service-layer decryption, ciphertext appears in API responses. Mitigation: code review checklist; repository unit tests that assert the returned encrypted field is non-empty and not equal to any known plaintext.
- **Nullable UUID handling** → sqlc generates `github.com/google/uuid.NullUUID` for nullable FK columns. Any code that assumes `uuid.UUID` on a nullable column will fail at `sqlc generate` time — caught before runtime.
- **Fact table composite PK width** → `ft_billing` has a 10-column composite PK. The `INSERT ... ON CONFLICT (col1, ..., col10) DO UPDATE` query is verbose; confirm with `EXPLAIN` that the planner uses the PK index.
- **inbound_sms._content gap (C1)** → The physical column name is `content`, not `_content`. All repository queries for `inbound_sms` must reference the physical `content` column; the Go struct field will be named `Content` by sqlc. Service-layer code must decrypt `Content` on read, matching the other encrypted columns.

## Migration Plan

1. Write all 14 query files in `db/queries/`
2. Run `sqlc generate` — fix any type-mismatch errors iteratively until all packages compile
3. Write a smoke test for each repository package: at minimum one `Create*` + one `Get*` round-trip against a test database
4. Validate: no queries reference `notify_status_type` enum; no queries reference `job_status_types` enum for status fields
5. No rollback needed: this is additive query-file authorship only; no schema changes
