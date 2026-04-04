## Why

Government service teams need user accounts to access the GC Notify admin UI. This change implements user CRUD, invite flows, permission management, MFA (FIDO2/U2F), and login event tracking. Authentication middleware itself is in `authentication-middleware` — this change covers the user management surface.

## What Changes

- `internal/handler/users/` — GET/PUT user, archive/deactivate/activate user, permissions, verify codes, login events, FIDO2 key registration and authentication
- `internal/handler/invite/` — invite user to service, invite to organisation, accept/reject invite, get invited user
- `internal/service/users/` — user CRUD, password management (with `pkg/crypto`), MFA token generation and verification, FIDO2 session management

## Capabilities

### New Capabilities

- `user-management`: User admin CRUD, service invite flows, password and verify-code management, FIDO2 MFA key registration and login

### Modified Capabilities

## Non-goals

- JWT issuance (that is the admin UI's responsibility — GC Notify API only validates tokens)
- Organisation invite flows (some overlap; organisation invites are covered in `organisation-management`)

## Impact

- Requires `authentication-middleware`, `data-model-migrations` (users, verify_codes, fido2_keys repositories)
