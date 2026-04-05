# GC Notify API — Spec Summary

This directory contains the reverse-engineered specification for the `notification-api` Python application. The spec is the input to a Go rewrite. No Go code exists yet; all documents here describe what the Python application currently does.

---

## What This Application Is

**GC Notify** is the Government of Canada's multi-tenant notification platform. It allows government departments ("services") to send email and SMS messages to Canadians at scale. It is the Canadian fork of the UK Government's [GOV.UK Notify](https://www.notifications.service.gov.uk/).

The application has two user-facing interfaces:

| Surface | Who uses it | URL prefix |
|---|---|---|
| **Admin API** | The GC Notify web UI (`notify-admin`) — used by government service teams to manage their service configuration, templates, users, billing | All non-`/v2/` routes |
| **Public v2 API** | External government service teams' code — used to send notifications programmatically to their end users | `/v2/` routes |

The admin UI is a separate application. This repository is the backend API only.

### Core Flow

```
Government service team's code
        │ POST /v2/notifications/email (or sms)
        ▼
   notification-api  ──── saves to PostgreSQL ──── enqueues to SQS
        │
   Celery workers (→ Go goroutines in rewrite)
        │
        ├── save-notifications worker  → bulk INSERT to notifications table
        ├── deliver-email worker       → calls AWS SES
        ├── deliver-sms worker         → calls AWS SNS / Pinpoint
        └── receipts worker            → processes SES/SNS delivery callbacks
                                         → updates notification status in DB
```

### Key Concepts

- **Service**: A government department or team. Each service has its own API keys, templates, send limits, and notification history.
- **Template**: A reusable email or SMS message with `((placeholder))` variables. Templates belong to a service.
- **Notification**: A single send of a template to one recipient. Persisted in `notifications`, then archived to `notification_history` after the service's retention period.
- **Job**: A bulk send — a CSV file uploaded to S3 with one row per recipient. A job produces N notifications.
- **Organisation**: A group of services (e.g. "Health Canada" contains multiple services for different programs).
- **Provider**: An external messaging provider — AWS SES (email), AWS SNS (SMS), AWS Pinpoint SMS V2 (dedicated long-number SMS). Provider selection and failover is managed internally.

---

## Spec File Index

### Foundation

| File | Contents |
|---|---|
| [out.sql](out.sql) | Raw `pg_dump --schema-only` of the fully migrated database. Ground truth for all table structures. 68 tables. |
| `test-manifest.txt` | Complete list of every test file used as source for the behavioral spec. Used for completeness verification. **⚠️ File does not exist on disk** — reference only; was not generated during spec authoring. |

### Layer 1 — Data Model

| File | Contents |
|---|---|
| [data-model.md](data-model.md) | All 68 tables: columns, types, nullability, defaults, indexes, FK relationships, enum types, encrypted columns, history/audit table pattern. Derived from `out.sql`, corroborated against `app/models.py`. |

Key patterns to know before reading:
- `*_history` tables (e.g. `templates_history`, `services_history`) are append-only audit mirrors with composite `(id, version)` PKs. Written on every UPDATE to the parent table via SQLAlchemy event hooks — must be replicated explicitly in Go.
- Columns prefixed with `_` (e.g. `_personalisation`, `_password`, `_code`) are **encrypted at rest** using `itsdangerous`-style HMAC encryption. sqlc will surface these as `[]byte`; decryption is applied in the service layer.
- Enum lookup tables (`auth_type`, `key_types`, `notification_status_types`, etc.) are single-column `name` PK tables — they are essentially DB-enforced enums and are seeded with fixed values.
- `ft_billing` and `ft_notification_status` are denormalised fact tables written by nightly Celery tasks; they have no FK constraints and wide composite PKs.

### Layer 2 — API Surface

| File | Contents |
|---|---|
| [api-surface.md](api-surface.md) | All ~210 endpoints across 36 blueprints: method, full path, auth scheme, request schema reference, response shape, HTTP status codes. Divided into admin routes (~195) and public v2 routes (~15). |

### Layer 3 — Async Tasks

| File | Contents |
|---|---|
| [async-tasks.md](async-tasks.md) | All 53 Celery tasks: task name, queue, trigger (SQS-driven vs beat-scheduled), input payload shape, side effects, retry policy. 22 queues. 31 beat schedule entries. Worker script → queue mapping. |

In the Go rewrite, Celery workers become goroutine pools (`internal/worker/`) consuming SQS queues. The beat scheduler becomes `robfig/cron` + `time.Ticker`. See [go-architecture.md](go-architecture.md) for the mapping.

### Layer 4 — Business Rules

Per-domain documents in `business-rules/`. Each file covers: data access patterns (every DAO function with its query type, filters, return value, side effects), domain invariants, validation rules, status machines, and cross-domain interactions.

| File | Domain |
|---|---|
| [business-rules/notifications.md](business-rules/notifications.md) | Notification create/update/query, status transitions, rate/daily/annual limit enforcement, retention and archival, bounce rate calculation |
| [business-rules/services.md](business-rules/services.md) | Service CRUD, permissions, safelist, SMS senders, email reply-to, callbacks, data retention config, archive/suspend/resume |
| [business-rules/templates.md](business-rules/templates.md) | Template CRUD, versioning, folder management, template categories, redaction, process type assignment |
| [business-rules/jobs.md](business-rules/jobs.md) | CSV job lifecycle: creation, S3 upload, processing (CSV parse → notification bulk insert → SQS enqueue), stall detection, archival |
| [business-rules/billing.md](business-rules/billing.md) | Annual SMS/email limits, free fragment limits, fact-billing nightly aggregation, quarterly limit data, platform stats |
| [business-rules/users-auth.md](business-rules/users-auth.md) | User CRUD, FIDO2 keys, verify codes, login events, permissions, invites, auth flow |
| [business-rules/organisations.md](business-rules/organisations.md) | Organisation CRUD, service linkage, domain management, org user invites |
| [business-rules/inbound-sms.md](business-rules/inbound-sms.md) | Inbound number allocation, inbound SMS receive and forwarding to service inbound API webhook |
| [business-rules/providers.md](business-rules/providers.md) | Provider record management, priority/load weighting, history, toggle between providers |
| [business-rules/platform-admin.md](business-rules/platform-admin.md) | SRE tools, cache clearing, platform stats, Cypress test helpers, complaint handling, newsletter management |

### Layer 5 — Behavioral Spec

Per-domain documents in `behavioral-spec/`. Each file contains: per-endpoint happy-path contracts (request shape, response shape, status code), validation rules with exact error messages, and edge cases.

| File | Source tests |
|---|---|
| [behavioral-spec/notifications.md](behavioral-spec/notifications.md) | `tests/app/notifications/`, `tests/app/dao/notification_dao/`, `tests/app/v2/notifications/`, `tests/app/public_contracts/`, `tests/app/test_annual_limit_utils.py`, `tests/app/test_email_limit_utils.py` |
| [behavioral-spec/services.md](behavioral-spec/services.md) | `tests/app/service/`, `tests/app/dao/` (service DAOs) |
| [behavioral-spec/templates.md](behavioral-spec/templates.md) | `tests/app/template/`, `tests/app/v2/template/`, `tests/app/v2/templates/`, `tests/app/template_folder/` |
| [behavioral-spec/jobs.md](behavioral-spec/jobs.md) | `tests/app/job/`, `tests/app/celery/` (job processing tasks) |
| [behavioral-spec/billing.md](behavioral-spec/billing.md) | `tests/app/billing/`, `tests/app/platform_stats/` |
| [behavioral-spec/users-auth.md](behavioral-spec/users-auth.md) | `tests/app/user/`, `tests/app/authentication/`, `tests/app/invite/`, `tests/app/accept_invite/`, `tests/app/api_key/` |
| [behavioral-spec/organisations.md](behavioral-spec/organisations.md) | `tests/app/organisation/` |
| [behavioral-spec/inbound-sms.md](behavioral-spec/inbound-sms.md) | `tests/app/inbound_sms/`, `tests/app/v2/inbound_sms/`, `tests/app/inbound_number/` |
| [behavioral-spec/providers.md](behavioral-spec/providers.md) | `tests/app/provider_details/`, `tests/app/delivery/` |
| [behavioral-spec/platform-admin.md](behavioral-spec/platform-admin.md) | `tests/app/status/`, `tests/app/complaint/`, `tests/app/events/`, `tests/app/newsletter/`, `tests/app/support/`, `tests/app/cache/`, `tests/app/cypress/` |
| [behavioral-spec/external-clients.md](behavioral-spec/external-clients.md) | `tests/app/clients/` — AWS SES, SNS, Pinpoint, S3, Salesforce, Freshdesk, Airtable |
| [behavioral-spec/smoke-tests.md](behavioral-spec/smoke-tests.md) | `tests_smoke/` — 9 smoke test files |
| [behavioral-spec/misc-gaps.md](behavioral-spec/misc-gaps.md) | Remaining files not covered by other domain files |

### Phase C — Go Rewrite Architecture

| File | Contents |
|---|---|
| [go-architecture.md](go-architecture.md) | Full Go project structure, package layout, router groups, middleware stack (auth, OTEL, CORS, rate limiting), sqlc config, repository function signatures for every domain, goroutine worker architecture replacing Celery, scheduled task mapping, external client interfaces, full config struct with all env vars, feature flags, well-known UUIDs, and key divergences from the Python implementation. |

---

## Reading Order for a New Agent

If you are implementing the Go rewrite, read in this order:

1. **This file** — understand the application's purpose and spec structure.
2. **[data-model.md](data-model.md)** — understand the database schema before reading any logic.
3. **[go-architecture.md](go-architecture.md)** — understand the target Go structure: packages, layers, sqlc setup, worker architecture, config.
4. **[api-surface.md](api-surface.md)** — the complete endpoint inventory; use this to know what handlers to implement.
5. **[async-tasks.md](async-tasks.md)** — the complete worker/task inventory; use this to know what goroutine workers to implement.
6. For each domain you are implementing, read:
   - `business-rules/{domain}.md` — the DAO query patterns and invariants to implement in the service layer.
   - `behavioral-spec/{domain}.md` — the exact HTTP contracts (request/response shapes, error messages) to implement in the handler layer.

---

## Notable Constraints and Gotchas

## Code Review Checklist

- For every service-layer mutating function on a versioned entity (`services`, `api_keys`, `templates`, `provider_details`, `service_callback_api`, `service_inbound_api`), confirm the matching `InsertXxxHistory` call happens inside the same transaction.
- For all 8 encrypted columns, verify the service layer calls `crypto.Encrypt` before repository writes and `crypto.Decrypt` after repository reads: `notifications._personalisation`, `notifications.to`, `notifications.normalised_to`, `inbound_sms.content`, `service_callback_api.bearer_token`, `service_inbound_api.bearer_token`, `verify_codes._code`, `users._password`.
- Confirm auth-path repository lookups use the reader DB handle when available, specifically `GetServiceByIDWithAPIKeys` and `GetAPIKeyBySecret`.

- **Wire format compatibility**: The Go API must preserve exact JSON field names and error response shapes. Admin errors use `{"result":"error","message":...}`; v2 errors use `{"status_code":N,"errors":[{"error":"...","message":"..."}]}`. See `go-architecture.md` § HTTP Error Response Format.
- **Encryption**: Five columns are encrypted at rest (`_personalisation`, `_password`, `_code`, `bearer_token` ×2). sqlc returns them as `[]byte`. Decrypt in the service layer via `pkg/crypto`. Key rotation: `SECRET_KEY` is a list; encrypt with `[0]`, decrypt by trying all. See `data-model.md` and `business-rules/users-auth.md`.
- **History tables**: Every UPDATE to a versioned entity (`services`, `templates`, `api_keys`, `provider_details`, `service_callback_api`, `service_inbound_api`) must also insert into its `*_history` table within the same transaction. This was automatic in Python via SQLAlchemy event hooks; in Go it is explicit in the service layer.
- **Letters are dead code**: Letter delivery features (`create-letters-pdf`, DVLA pipeline, precompiled letter templates) are stubs in the Canadian deployment. The Go rewrite omits them; letter endpoints return appropriate stub responses only.
- **Feature flags**: Six feature flags (`FF_USE_BILLABLE_UNITS`, `FF_SALESFORCE_CONTACT`, `FF_USE_PINPOINT_FOR_DEDICATED`, `FF_BOUNCE_RATE_SEED_EPOCH_MS`, `FF_PT_SERVICE_SKIP_FRESHDESK`, `FF_ENABLE_OTEL`) change significant behaviour. See `go-architecture.md` § Feature Flags.
- **Read replica**: Some queries (especially auth-path lookups) are explicitly routed to a read replica. The architecture uses two separate `*sql.DB` instances (writer + reader). See `go-architecture.md` § Read Replica Routing.
- **Simulated recipients**: A hardcoded list of phone numbers and email addresses bypass actual delivery (used for testing). Notifications are persisted but not enqueued to delivery queues. See `business-rules/notifications.md`.
- **Quarterly email bug**: The Python beat schedule is missing the Q4 quarterly usage email (April). The Go implementation should add it. See `go-architecture.md` § Scheduled Tasks note.
