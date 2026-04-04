## Why

Platform administrators need cross-cutting tooling: complaint management, branding CRUD, event logging, performance stats, SRE tools, cache management, and Cypress test helpers. This change implements all endpoints under `/sre-tools`, `/cache-clear`, `/cypress`, `/complaint`, `/email-branding`, `/letter-branding`, `/events`, `/platform-stats`, and `/support`.

## What Changes

- `internal/handler/admin/` — SRE tools (feature flags, functional tests), cache-clear endpoints, Cypress test helpers (non-production only)
- `internal/handler/complaint/` — complaint list and GET
- `internal/handler/email_branding/` and `letter_branding/` — branding CRUD
- `internal/handler/admin/events/` — event logging endpoints
- `internal/handler/platform_stats/` — platform statistics (usage by service, send-method stats)
- `internal/handler/support/` — find-ids lookup

## Capabilities

### New Capabilities

- `platform-admin-features`: SRE tooling, cache management, complaint management, email/letter branding CRUD, event logging, platform statistics, support utilities

### Modified Capabilities

## Non-goals

- Service-level reporting (covered in `billing-tracking` and `bulk-send-jobs`)
- Newsletter endpoints (covered in `newsletter-endpoints`)

## Impact

- Requires `authentication-middleware` (SRE, cache-clear, and cypress middleware), `data-model-migrations`
