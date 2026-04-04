## Context
Platform admin features span multiple protected route groups (SRE JWT, cache-clear JWT, Cypress JWT, admin JWT). This change wires all cross-cutting endpoints that don't belong to a specific domain: complaint management, email/letter branding CRUD, system event logging, platform usage statistics, SRE inspection tools, Redis cache-clear, Cypress test helpers (non-production only), healthcheck, and the support entity-lookup utility.

## Goals / Non-Goals
**Goals:** Cache-clear endpoint, healthcheck and live-counts endpoints, SRE JWT tool endpoints, Cypress helpers (non-production), complaint list and count, email branding CRUD, letter branding CRUD, event logging, platform stats (aggregate + usage + trial + send-method), and support find-id lookup.
**Non-Goals:** Per-service reporting (in `billing-tracking` and `bulk-send-jobs`), newsletter endpoints (in `newsletter-endpoints`), inbound number CRUD (in `service-management`).

## Decisions

### Cache-clear: delete Redis keys by pattern
`POST /cache/clear` calls `redis_store.delete_cache_keys_by_pattern` for every pattern in `CACHE_KEYS_ALL` and returns `201 {"result": "ok"}`. Any Redis exception becomes a `500`. Auth uses a dedicated cache-clear JWT header that is distinct from the standard admin JWT.

### SRE tools: RequireSRE middleware
Endpoints under `/sre-tools/` require the SRE JWT (`RequireSRE` middleware). This is a separate auth token from the standard admin JWT; an admin JWT on an SRE endpoint returns `401`. The SRE live-service-and-organisation-counts endpoint delegates to the same DB counting logic as `GET /status/live-service-and-organisation-counts`.

### Cypress helpers: production block at request time before auth
`POST /cypress/create_user/<suffix>` and `GET /cypress/cleanup` are blocked in production at request handling time — **before** the Cypress auth header is checked. If `NOTIFY_ENVIRONMENT == "production"`, the handler returns `403` immediately. In non-production environments the dedicated Cypress JWT is required and verified. This prevents accidental test data creation regardless of whether a valid auth header is present.

### Email branding CRUD: name uniqueness at API layer, falsy-to-null coercion on update
`POST /email-branding` creates an `EmailBranding` record; `POST /email-branding/<id>` performs a partial update. Uniqueness is checked by calling `dao_get_email_branding_by_name` before insert/update; a match returns `400 CannotSaveDuplicateEmailBrandingError`. The `dao_update_email_branding` function coerces any falsy kwarg value (empty string, zero, False) to `NULL` before storing — callers must pass the existing value or `None` explicitly (not an empty string) when they want to preserve or clear a field.

### Letter branding CRUD: IntegrityError mapped to 400 on update
`POST /letter-branding` creates a `LetterBranding` record; `POST /letter-branding/<id>` updates it. Name collisions on update are detected via a caught `IntegrityError` and returned as `400 {"message": {"name": ["Name already in use"]}}` rather than a pre-check lookup. All letter branding records are globally visible (no org scoping). `dao_get_all_letter_branding` returns records ordered `name ASC`.

### Complaint management: paginated admin list and timezone-aware count
`GET /complaint` returns all complaints paginated (page size from `PAGE_SIZE` config) with `.service` eager-loaded to avoid N+1 during serialisation. The count endpoint `GET /complaint/count` uses `fetch_count_of_complaints` whose date window boundaries are computed as `America/Toronto` timezone midnight (TIMEZONE env var). Go must replicate this timezone conversion for start/end date midnight computation — not UTC midnight.

### Platform stats: financial-year enforcement on usage endpoint
`GET /platform-stats/usage-for-all-services` validates that the requested date range falls within a single financial year (April 1 → March 31, local `America/Toronto` time). Cross-year ranges return `400`. The response merges three independent datasets (SMS billing, letter cost totals, letter line items) into per-service records keyed by `service_id`; services present only in letter data are still included. Sorting: blank `organisation_name` last, then org name alphabetically, then service name.

### Events: wrap in caller transaction (no @transactional on dao_create_event)
`POST /events` persists an `Event` row via `dao_create_event`, which calls `db.session.commit()` directly and does **not** use the `@transactional` decorator. In Go the event insert must be wrapped inside the caller's transaction — not issued as a separate commit — to maintain rollback safety and avoid orphan audit rows if the surrounding operation fails.

### Support find-id: fixed resolution order, non-UUID tokens inlined
`GET /support/find-id` resolves each submitted UUID in a fixed order: user → service → template → job → notification. The first match wins. Non-UUID tokens are not rejected with 400; they are returned inline as `{"type": "not a uuid"}`. UUIDs matching no entity return `{"type": "no result found"}`. The endpoint is not paginated.
