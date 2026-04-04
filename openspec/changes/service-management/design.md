## Context

Service configuration is the operational backbone of notification-api. Every notification, template, API key, job, and billing record belongs to a service. The service admin API (~80+ endpoints) manages the complete service lifecycle: CRUD, permissions, user membership, API key lifecycle, SMS senders, email reply-to addresses, letter contacts, callback webhooks, inbound SMS webhook, data retention policies, and safelist.

Two bugs identified in Python source are corrected in this Go implementation:

- **C7 fix**: `raise Exception("You must have at least one SMS sender as the default.", 400)` in `dao_add_sms_sender_for_service` and `dao_update_service_sms_sender` — a bare Python Exception with a tuple as args, which produces a garbled 500 response instead of a structured 400. Go must consistently use `InvalidRequestError{Message, StatusCode}`.
- **Truthiness bug**: `reset_service_inbound_api` and `reset_service_callback_api` use `if url:` instead of `if url is not None:`, preventing callers from clearing a URL/token to an empty string. Go must use `!= nil`.

All versioned entities (`Service`, `ServiceCallbackApi`, `ServiceInboundApi`, `ApiKey`) write history rows on every mutation within the same transaction.

---

## Goals / Non-Goals

**Goals**

- Implement all 80+ service-family REST endpoint handlers
- Service CRUD with mandatory history writes (create, update, archive, suspend, resume)
- API key lifecycle: create (hash secret, return plaintext once), list, revoke
- SMS sender management with default invariant enforcement and C7 fix
- Email reply-to management with default invariant enforcement
- Letter contact management (data layer; not the letter delivery pipeline)
- Callback API config for delivery-status and complaint types with bearer token signing
- Inbound SMS API config with bearer token; truthiness-bug fix
- Data retention CRUD
- Safelist (whitelist) CRUD with atomic replace-all semantics
- Service user membership management
- Full DAO layer (sqlc queries covering all operations in the query inventory)

**Non-Goals**

- User CRUD (covered by `user-management` change)
- Organisation–service linkage (covered by `organisation-management` change)
- Template management (separate change)
- Notification dispatch or delivery pipeline
- Statistics aggregation workers (covered by `billing-tracking` change)

---

## Decisions

### C7 fix: use InvalidRequestError for all service-layer guard failures

The SMS sender default guard raises `Exception("You must have at least one SMS sender as the default.", 400)` in Python — the tuple `("message", 400)` is stored as the exception's args, which produces a garbled 500 response body containing the stringified tuple instead of a structured JSON object. In Go, every guard failure in the service layer SHALL return an `InvalidRequestError{Message: "...", StatusCode: 400}` that maps to `{"result":"error","message":"..."}` at the HTTP handler level. This pattern is already used consistently by email reply-to guards in Python (`InvalidRequest`) and must be extended to SMS sender guards.

### API key secret: hash on create, return plaintext once, never store

On `POST /service/{id}/api-key`:
1. Generate a random secret of sufficient entropy.
2. Hash it using HMAC-SHA256 keyed with the service API key secret config value.
3. Store the hashed value in `api_keys.secret`.
4. Return the plaintext secret in the 201 response body (prefixed with `API_KEY_PREFIX` as the `key` field).

Subsequent `GET /api-keys` calls never reveal the plaintext. The `key_name` response field contains the `API_KEY_PREFIX` prefix. Two keys generated for the same service always produce distinct values.

### Service suspension does NOT expire API keys

`dao_suspend_service_no_transaction` sets `active=False` and records `suspended_at` (and optionally `suspended_by_id`). It does NOT set `expiry_date` on any API key. Only `dao_archive_service_no_transaction` expires keys (those without an existing expiry date). This preserves the ability to resume the service and have all previously-valid keys work immediately. Go must mirror this exactly — tests must assert API key `expiry_date` remains null after suspend and that pre-revoked keys remain revoked after resume.

### Service history writes are mandatory for every mutation

Every function that mutates a `Service` row is decorated with `@version_class(Service)` in Python, writing a row to `services_history` with an incremented `version` within the same transaction. In Go, calling `CreateService`, `UpdateService`, `SuspendService`, `ResumeService`, and `ArchiveService` must each write a `services_history` row in the same transaction. The same applies to `ServiceCallbackApi` and `ServiceInboundApi` mutations (both have `_history` tables with `@version_class` decoration).

### Safelist update is always full-replace

The canonical safelist update pattern is:
1. Validate all entries upfront (email or phone). On any invalid entry → HTTP 400, return immediately without touching the DB.
2. `dao_remove_service_safelist(service_id)` — delete all existing entries.
3. `dao_add_and_commit_safelisted_contacts(objs)` — bulk-insert new entries.

Both operations must be wrapped in a single transaction (the Python implementation uses separate commits, creating a window where the safelist is empty; Go must close this gap).

### Callback bearer tokens are signed with itsdangerous Signer, not encrypted

Callback bearer tokens are stored as itsdangerous-signed values in the `_bearer_token` column. The `bearer_token` property strips the signature to return plaintext. Key rotation is handled by `resign_service_callbacks(resign=True/False)`. Go must implement equivalent HMAC signing, using the same secret and salt, to remain interoperable with any existing signed rows in the DB.

### Truthiness bug: use strict nil checks in reset functions

Python's `if url:` silently skips updating a field when passed an empty string. Go must use pointer parameters (`*string`) and check `!= nil` in `ResetServiceInboundApi` and `ResetServiceCallbackApi`. This is a correctness fix: callers that explicitly pass an empty string to clear a URL or bearer token must have their intent honoured.

### Letter contacts are data-layer only in this scope

Letter contact CRUD endpoints (create, list, get, update, archive) are in scope. The archive operation must cascade `Template.service_letter_contact_id` to `NULL` for all templates referencing the archived contact. The full letter-sending pipeline integration (rendering, dispatch) is out of scope for this change.

---

## Risks / Trade-offs

- **Safelist atomicity gap in Python**: the existing Python implementation commits the DELETE and INSERT separately. Go closes this gap by wrapping both in a single transaction, which is a strict improvement in correctness but a deliberate behavioural divergence from Python.
- **Service name uniqueness is advisory**: `GET /service/is-name-unique` is a pre-flight check only; a race condition between check and create is possible. The DB unique constraint on `services.name` is the authoritative guard. Handlers must translate `UniqueConstraintError` to HTTP 400.
- **Organisation auto-assignment at creation**: longest-domain-match is computed at creation only. Subsequent org changes go through `organisation-management` endpoints. If the matching logic changes, existing services are unaffected.
- **Callback URL HTTPS enforcement**: the HTTPS-only check must be enforced in the handler before persisting. Bearer token minimum length (10 chars) is validated by the schema layer. Both checks must return structured `{"result":"error","message":"..."}` bodies, not plain-text 400 responses.
