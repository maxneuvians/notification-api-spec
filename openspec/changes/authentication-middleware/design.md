## Context

The Python application uses Flask's `@requires_auth` decorator backed by JWT validation. There are four internal JWT issuers (admin UI, SRE tooling, cache-clear tooling, Cypress test tooling) and one service-facing auth scheme that accepts either a JWT (with service ID as issuer and matched API key as secret) or a plaintext API key header. All schemes use HMAC-SHA256.

## Goals / Non-Goals

**Goals:**
- Implement all five middleware as standalone `func(http.Handler) http.Handler` wrappers
- Inject typed caller identity into `context.Context` for use by handlers
- Cache service + API key lookups in Redis to avoid per-request DB hits
- Block Cypress-auth routes in production environment at request time

**Non-Goals:**
- Permission checking beyond identity verification (role-based access is handler responsibility)
- Login/logout flows, token issuance (see `user-management`)
- API key lifecycle (create, revoke, compromise) — see `service-management`

## Decisions

### JWT validation: decode issuer first, then verify signature
All five JWT schemes extract the `iss` claim without signature verification, compare it to the configured issuer name, then verify the HMAC-SHA256 signature. This matches the Python implementation and is safe because the issuer check is always followed by signature verification using the corresponding secret. An attacker cannot use a valid admin token on an SRE route because the secrets differ.

### ServiceAuth: Redis TTL cache keyed by service ID
`RequireAuth` fetches a service and its API keys from the read-replica DB, then caches the result in Redis for a short TTL (default 30 s). This avoids a DB round-trip on every API request. Cache invalidation on API key revoke is handled by deleting the Redis key; the TTL provides a fallback safety window.

### ServiceAuth JWT path: try all non-expired keys
When the auth header is a Bearer JWT, `iss` is expected to be a service ID UUID. The middleware fetches all non-expired API keys for that service (from cache) and attempts HMAC-SHA256 verification against each key's secret until one succeeds. This mirrors the Python `validate_client_token` function and allows a service to rotate keys without a gap in service.

### ServiceAuth API key path: hash-based lookup
When the auth header is `ApiKey-v1 <plaintext>`, the middleware hashes the supplied secret and looks up the API key record by the hashed value. This avoids storing or comparing plaintext secrets.

### Cypress auth: non-production check in middleware, not at startup
The non-production guard for Cypress routes is evaluated at request time (not at startup) so that a production binary that receives a misconfigured Cypress route request gets a 403, not a startup failure. This matches the Python behaviour.

### Clock skew tolerance: 30 seconds for JWT `exp`/`iat`
All four internal JWT schemes allow ±30 s clock skew, matching the Python `ALLOW_DEBUG_CREATIONS_FLAG` logic.

### Error body format differentiation
Admin/SRE/cache-clear/Cypress middleware returns `{"token": ["Invalid token: <reason>"]}` on JWT failure (HTTP 401). Service auth (`RequireAuth`) returns `{"result": "error", "message": "Unauthorized, authentication token must be provided"}` for a missing/malformed auth header (401), and a bare 403 body for service-not-found or key-not-matched. This matches the Python Flask-JWT error serialisation per route group.

### Context key type safety — unexported struct type
Context keys for `AuthenticatedService` and `ApiUser` are defined as values of an unexported `type contextKey struct{ name string }`. This prevents any external package from accidentally shadowing the same key and avoids the string-key anti-pattern. The exported getter functions (`GetAuthenticatedService`, `GetApiUser`) return a typed `(nil, ErrNoAuthContext)` on missing value rather than a nil-pointer panic.

### Middleware config captured at init time — fail fast on missing secrets
Each middleware constructor receives a `Config` value at startup. If a required secret field (e.g. `AdminClientSecret`) is empty, the constructor panics. This surfaces misconfiguration at startup rather than silently accepting all JWT validation at runtime.

### No-auth routes mounted outside all auth groups
`GET /_status`, `GET /version`, and any metrics endpoints are mounted on the root router before any auth group. This ensures load-balancer health probes and service-discovery scrapes are never gated by authentication.

### `Authorization` scheme prefix matching
The `Bearer ` prefix is detected case-insensitively (`strings.EqualFold`). The `ApiKey-v1 ` prefix is matched exactly (case-sensitive), matching the Python reference implementation. An unrecognised auth scheme on a service-auth route returns HTTP 401.

## Risks / Trade-offs

- **Redis cache window allows stale revocation** → If an API key is revoked, up to 30 s of requests using the revoked key may succeed. Mitigated by the compromised-key revocation path (immediate cache flush) handled in `service-management`.
- **Timing side-channel in API key iteration** → Iterating over all keys and using `hmac.Equal` avoids timing leaks per key; the total iteration time varies with key count but this is not a security concern given key counts are small (< 10 per service typically).
- **JWT without RS256** → Both admin and service JWTs use HMAC-SHA256 with shared secrets (not RSA). This is inherited from the Python design and cannot be changed without coordinating with the admin UI and all external API clients.

## Migration Plan

No existing Go system to migrate. New middleware is wired in `cmd/api/main.go` route group mounting:

```go
r.Group(func(r chi.Router) {
    r.Use(middleware.RequireAdminAuth(cfg))
    r.Mount("/service", handler.NewServiceRouter(...))
    // ...
})
r.Group(func(r chi.Router) {
    r.Use(middleware.RequireAuth(cfg, svcAuthCache))
    r.Mount("/v2/notifications", handler.NewV2NotificationsRouter(...))
    // ...
})
```
