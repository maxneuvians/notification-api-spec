## Why

Organisations are the top-level grouping of government services. This change implements organisation CRUD, domain mapping, branding assignment, agreement tracking, and service linking.

## What Changes

- `internal/handler/organisations/` — organisation CRUD, service listing, user listing, domain management, branding association, agreement status, organisation invite flows
- `internal/service/organisations/` — domain-based organisation lookup, service-org linking

## Capabilities

### New Capabilities

- `organisation-management`: Organisation CRUD, domain mapping, service linking, branding, agreement tracking, invite flows

### Modified Capabilities

## Non-goals

- Email branding and letter branding CRUD (separate from organisation branding association — covered in `platform-admin-features`)
- User CRUD (covered in `user-management`)

## Impact

- Requires `authentication-middleware`, `data-model-migrations` (organisation, domain repositories)
