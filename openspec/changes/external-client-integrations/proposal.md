## Why

GC Notify integrates with several external systems: Airtable (newsletter subscribers), Salesforce (account and contact records), Freshdesk (support tickets), and Cronitor/Performance Platform (operational metrics). This change implements all four external client packages and the endpoint/worker code that calls them.

## What Changes

- `internal/client/airtable/` — Airtable API client for newsletter subscriber CRUD
- `internal/client/salesforce/` — Salesforce SOAP/REST contact and engagement API
- `internal/client/freshdesk/` — Freshdesk ticket creation
- `internal/client/cronitor/` — Cronitor heartbeat pings
- `internal/handler/newsletter/` mounts (except the 4 undocumented routes — those are in `newsletter-endpoints`)

## Capabilities

### New Capabilities

- `external-client-integrations`: Airtable, Salesforce, Freshdesk, and Cronitor external API client packages and their associated endpoint/worker wiring

### Modified Capabilities

## Non-goals

- The 4 undocumented newsletter routes (C2 fix) — covered in `newsletter-endpoints`
- Inbound email handling (no-reply worker) — covered in `notification-delivery-pipeline`

## Impact

- Requires `authentication-middleware` (admin JWT for newsletter routes)
- All four client packages introduce new external HTTP dependencies; each must be mockable for tests
