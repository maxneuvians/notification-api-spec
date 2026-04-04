## Context

The legacy v0 notification endpoints (`POST /notifications/email`, `GET /notifications/{id}`, `GET /notifications`) are used by the GC Notify admin UI. The public v2 endpoints (`POST /v2/notifications/email`, `GET /v2/notifications/{id}`, `GET /v2/notifications`) are used by external government service teams. Both share the same internal service layer but have different request/response schemas.

## Goals / Non-Goals

**Goals:**
- Implement email send with full validation, template rendering, and queue dispatch
- Implement GET endpoints for single and listed notifications
- Handle simulated email addresses without persistence
- Enforce per-minute, daily, and annual email limits
- Support `email_reply_to_id` override and document attachments

**Non-Goals:**
- SMS send endpoints (separate change)
- Bulk CSV jobs (separate change)
- Actual SES delivery (delivery pipeline change)
- HTML email rendering/template compilation

## Decisions

### Single service layer, two handler layers
The `internal/service/notifications/` package contains the shared business logic. `internal/handler/notifications/` and `internal/handler/v2/notifications/` are thin HTTP adapters that translate request schemas and call the service layer. This keeps validation and routing logic out of the service layer.

### Email address validation: `net/mail.ParseAddress`
Go's stdlib `net/mail.ParseAddress` handles RFC 5322 address parsing. The validation spec requires rejecting bracketed addresses and non-string types, which stdlib handles correctly.

### Personalisation size check before rendering
Personalisation is checked at `json.Marshal` size after parsing, before template rendering. This catches large blobs before wasting CPU on rendering.

### Document attachment validation order
Validate: (1) service has `UPLOAD_DOCUMENT` permission, (2) document count ≤ 10, (3) each document: `sending_method` in `[attach, link]`, `filename` 2–255 chars, file is valid base64, decoded size ≤ 10 MB. All errors are collected and returned as a `ValidationError` list.

### Simulated address check before any DB write
Simulated email addresses are checked before template lookup, personalisation validation, and DB insert. On detection: return 201 with a mock response body; no persist, no queue publish.

### Limit check order: rate → annual → daily
Matching the Python order: per-minute rate limit checked first, then annual, then daily. On any limit breach, 429 is returned and no notification is persisted.

### Reply-to resolution: default if not supplied
If `email_reply_to_id` is not in the request, the service's default email reply-to is fetched and stored on the notification at creation time. Later changes to the service reply-to do NOT retroactively update stored notifications.

### v1/v2 error response schema differentiation
v2 endpoint errors use `{"errors": [{"error": "ValidationError", "message": "..."}], "status_code": 400}`. v0 (legacy) errors use `{"result": "error", "message": {...}}`. The service layer returns typed errors; each handler layer maps them to the correct format independently. This preserves backward compatibility with external clients parsing v0 error shapes.

### Notification rollback on queue publish failure
If the Redis queue publish call fails after the DB insert, the notification row is deleted from the DB and an error is returned to the caller. This prevents orphaned notifications in `created` status that will never be delivered.

### `scheduled_for` permission and window
`scheduled_for` requires the `SCHEDULE_NOTIFICATIONS` service permission (invite-only). The value must be a non-past ISO8601 datetime and may not be more than 24 h in the future (single sends; bulk allows 96 h). Scheduled notifications are persisted but not enqueued; the `dao_get_scheduled_notifications` periodic task dispatches them.

### `one_click_unsubscribe_url` — stored verbatim, passed through
If supplied, `one_click_unsubscribe_url` is stored on the notification row and included in the 201 response. No URL format validation is applied in the service layer; the value is passed verbatim to the delivery task, which includes it in the `List-Unsubscribe` email header.

### Key-type visibility isolation in list endpoints
`GET /v2/notifications` filters by the requesting API key's `key_type`: `normal`-key callers see only `key_type = 'normal'` notifications; `test`-key callers see only `key_type = 'test'`. Prevents cross-key-type notification visibility within the same service.

## Risks / Trade-offs

- **Personalisation injection** → Template rendering replaces `((placeholder))` variables. The renderer must sanitise values to prevent any attempt to inject additional placeholders. Use exact string replacement, not recursive rendering.
- **Document upload race** → S3 upload for attachment documents happens synchronously in the request path; large payloads increase latency. This matches the Python behaviour — deferred upload is a future improvement.
- **Limit check TOCTOU** → Rate/daily/annual limits use Redis counters incremented after the DB insert. A burst of concurrent requests can transiently exceed limits; this matches the Python implementation.
