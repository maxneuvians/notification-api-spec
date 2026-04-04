## Why

The Python application has 7 letter-related endpoints that are stubs (they exist but return fixed responses or minimal behaviour). The Go rewrite must expose the same routes returning the correct status codes so that API clients and integration tests don't encounter 404s. Full letter implementation is explicitly out of scope for this project.

## What Changes

- 5 letter-contact routes: `GET/POST /service/{id}/letter-contact`, `GET/PUT /service/{id}/letter-contact/{contact_id}`, `POST /service/{id}/letter-contact/{contact_id}/archive` — return 200 with empty list / stub object
- `POST /service/{id}/send-pdf-letter` — return 400 `"Letters as PDF feature is not enabled"` (consistent with Python stub)
- `GET /letters/returned` — return 200 with empty list

## Capabilities

### New Capabilities

- `letter-stub-endpoints`: 7 letter routes mounted with correct status codes and minimal response bodies, marked as stub implementations

### Modified Capabilities

## Non-goals

- Actual letter generation, printing, or delivery (not in scope for this rewrite)
- DVLa integration

## Impact

- Requires `authentication-middleware` (admin JWT)
- **H1 fix** from validation reports: enumerates all 7 stub endpoints with expected responses
