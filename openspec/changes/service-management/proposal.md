## Why

Service configuration (permissions, API keys, SMS senders, email reply-tos, callback URLs, data retention, safelists) is managed via the service admin API. This change implements all 80+ service-family endpoints, including the C7 fix for the malformed exception in the SMS sender DAO.

## What Changes

- `internal/handler/services/` — complete service CRUD, permissions, archive/suspend/resume, user membership, API key management, SMS senders, email reply-tos, letter contacts, callback APIs, inbound API config, data retention, safelists, bounced emails
- `internal/service/services/` — business logic for service operations; **C7 fix**: replace `raise Exception("You must have at least one SMS sender as the default.", 400)` pattern with structured `InvalidRequestError` (consistent with rest of codebase)
- `internal/handler/api_key/` — API key CRUD endpoints (create, revoke, list, compromise reporting)

## Capabilities

### New Capabilities

- `service-management`: Service admin CRUD, permissions management, API key lifecycle, SMS sender and email reply-to management, callback API configuration, data retention, safelists

### Modified Capabilities

## Non-goals

- User-to-service relationship management beyond what is part of the service endpoints (user CRUD is in `user-management`)
- Organisation-service linkage (covered in `organisation-management`)

## Impact

- Requires `authentication-middleware`, `data-model-migrations` (services, api_keys repositories)
- **C7 fix**: `InvalidRequestError` pattern prevents garbled 500 responses from the SMS sender default guard
