## Why

The notification-api has five distinct authentication schemes applied to different route groups. Without this middleware, no route can verify the identity of its caller. This change implements all five schemes as standalone `func(http.Handler) http.Handler` middleware, enabling every subsequent domain change to simply mount its routes under the correct auth middleware.

## What Changes

- `internal/middleware/auth.go` — five middleware functions: `RequireAdminAuth`, `RequireSREAuth`, `RequireCacheClearAuth`, `RequireCypressAuth`, `RequireAuth` (service auth, JWT + API key sub-schemes)
- `internal/middleware/auth_context.go` — typed context keys and `AuthenticatedService`, `ApiUser` structs injected into `context.Context` by `RequireAuth`
- Route group auth wiring in `cmd/api/main.go` — each route group is mounted with its correct auth middleware
- Redis-backed API key cache in `internal/service/auth/` — TTL-based caching of service + API key lookups used by `RequireAuth`
- Tests for all five schemes: valid token accepted, expired token rejected, wrong issuer rejected, tampered signature rejected

## Capabilities

### New Capabilities

- `auth-middleware`: The five authentication middleware functions, context injection types, API key cache, and the JWT validation logic for all four internal issuer roles

### Modified Capabilities

## Non-goals

- User session management, login flows, MFA — handled in `user-management`
- API key CRUD (create/revoke) — handled in `service-management`
- Per-service rate limiting (enforced in service layer, not auth middleware)
- Authorization (permission checking beyond authentication) — handled per-domain

## Impact

- Requires `go-project-setup` (chi router and middleware stack) and `data-model-migrations` (api_keys and services repository) to be complete
- All 17 domain-handling changes depend on at least one of these middleware functions being present
