## Context
Four newsletter-related routes exist in the Python production codebase (`POST /newsletter/update-language/{subscriber_id}`, `GET /newsletter/send-latest/{subscriber_id}`, `GET /newsletter/find-subscriber`, `GET /platform-stats/send-method-stats-by-service`) but were absent from `api-surface.md` (validation finding C2). This change documents and implements them in the Go rewrite.

## Goals
- Implement all 4 C2 routes with auth, request/response validation, and Airtable/DB client wiring
- Provide full request/response shape documentation so these routes are no longer undocumented
- Ensure all 4 routes enforce admin JWT authentication

## Non-Goals
- Other newsletter endpoints (`POST /newsletter/unconfirmed-subscriber`, `GET /newsletter/confirm/{id}`, `GET /newsletter/unsubscribe/{id}`) — covered in `external-client-integrations`
- Airtable client implementation itself — covered in `external-client-integrations`

## Decisions

### All four routes require admin JWT
`POST /newsletter/update-language/{subscriber_id}`, `GET /newsletter/send-latest/{subscriber_id}`, `GET /newsletter/find-subscriber`, and `GET /platform-stats/send-method-stats-by-service` all require `requires_admin_auth`. Unauthenticated requests → 401.

### update-language returns 204 (not 200)
The update operation modifies a record and returns no body. HTTP 204 No Content is semantically correct and matches typical REST convention for update-without-response-body.

### language validation: strict enum
Only `"en"` and `"fr"` are valid language values. Any other value → HTTP 400 with a validation error. The check happens in the handler before any Airtable call.

### find-subscriber: email query param is required
Missing or empty `email` query parameter → 400. The endpoint does not support listing all subscribers.

### platform-stats: queries ft_billing, grouped by service_id and key_type
The `key_type` column in `ft_billing` distinguishes API sends (`"normal"`), team sends (`"team"`), and test key sends (`"test"`). The response aggregates these per service into `api_count`, `bulk_count`, and `team_count`.

### send-latest: uses LatestNewsletterTemplate, not a request body template ID
The latest newsletter templates are fetched from Airtable's `LatestNewsletterTemplate` table (sort by `-Created at`, limit 1). No template ID is accepted in the request.

## Risks
- `send-latest` involves both an Airtable subscriber lookup and a template lookup; either can independently return 404. Handler must produce distinct error messages to aid debugging.
- `platform-stats` query on `ft_billing` may be slow without a covering index on `key_type`; add a query timeout context.
