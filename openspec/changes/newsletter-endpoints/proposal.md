## Why

Four newsletter-related routes exist in the Python codebase (`POST /newsletter/update-language/{subscriber_id}`, `GET /newsletter/send-latest/{subscriber_id}`, `GET /newsletter/find-subscriber`, `GET /platform-stats/send-method-stats-by-service`) but were missing from the `api-surface.md` specification (validation finding C2). This change adds these four documented endpoints to the Go implementation.

## What Changes

- `internal/handler/newsletter/update_language.go` — `POST /newsletter/update-language/{subscriber_id}`: update subscriber language preference in Airtable
- `internal/handler/newsletter/send_latest.go` — `GET /newsletter/send-latest/{subscriber_id}`: trigger sending the latest newsletter to a subscriber
- `internal/handler/newsletter/find_subscriber.go` — `GET /newsletter/find-subscriber`: look up subscriber by email address
- `internal/handler/platform_stats/send_method_stats.go` — `GET /platform-stats/send-method-stats-by-service`: return send method breakdown per service

## Capabilities

### New Capabilities

- `newsletter-endpoints`: The 4 undocumented routes from the C2 validation finding, with auth, request/response shapes, and Airtable client wiring

### Modified Capabilities

## Non-goals

- Other newsletter endpoints (`POST /unconfirmed-subscriber`, `GET /confirm`, `GET /unsubscribe`) — covered in `external-client-integrations`
- Airtable client implementation itself — covered in `external-client-integrations` (only the endpoint wiring is here)

## Impact

- Requires `external-client-integrations` (Airtable client), `authentication-middleware` (admin JWT)
- **C2 fix**: all 4 routes from the validation report are documented here with auth scheme and request/response shapes
