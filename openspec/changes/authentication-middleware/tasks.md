## 1. JWT Validation Core

- [ ] 1.1 Implement JWT decode-then-verify helper in `internal/middleware/auth.go`: parse Authorization header, strip Bearer prefix (case-insensitive), base64-decode the JWT without signature validation to extract `iss`/`exp`/`iat` claims, then verify HMAC-SHA256 signature using the provided secret; apply ±30 s clock skew; return typed error with reason string on any failure
- [ ] 1.2 Write table-driven unit tests for the JWT helper covering: valid token passes, expired (> 30 s) rejected, within 30 s skew window accepted, wrong signature rejected, missing `iss` rejected, malformed base64 rejected, JWT with fewer than 3 parts rejected, missing Authorization header rejected

## 2. Admin, SRE, and Cache-Clear Middleware

- [ ] 2.1 Implement `RequireAdminAuth(cfg Config) func(http.Handler) http.Handler` using the JWT helper with `cfg.AdminClientUserName` / `cfg.AdminClientSecret`; on failure return HTTP 401 with JSON body `{"token": ["Invalid token: <reason>"]}`; panic at construction if `cfg.AdminClientSecret` is empty
- [ ] 2.2 Implement `RequireSREAuth(cfg)` following the same pattern with `cfg.SREUserName` / `cfg.SREClientSecret`; write unit tests: valid SRE token on `/sre-tools` passes, admin token on `/sre-tools` returns 401
- [ ] 2.3 Implement `RequireCacheClearAuth(cfg)` with `cfg.CacheClearUserName` / `cfg.CacheClearClientSecret`; write unit tests: valid token accepted, wrong issuer rejected
- [ ] 2.4 Write shared table-driven tests for the three simple JWT middleware covering: valid token 200, expired token 401, wrong issuer 401, tampered signature 401, missing Authorization header 401, correct error body format `{"token": [...]}`

## 3. Cypress JWT Middleware

- [ ] 3.1 Implement `RequireCypressAuth(cfg)`: add production guard — if `cfg.NotifyEnvironment == "production"` return HTTP 403 immediately before any JWT parsing; then validate JWT with `cfg.CypressAuthUserName` / `cfg.CypressAuthClientSecret`
- [ ] 3.2 Write unit tests: production environment blocks all requests (valid token 403, invalid token 403, no token 403); non-production with valid Cypress token returns 200; non-production with invalid token returns 401; non-production with no token returns 401

## 4. Service Auth Context Types

- [ ] 4.1 Define `AuthenticatedService` and `ApiUser` structs in `internal/middleware/auth_context.go`; define unexported `contextKey` struct type; define package-level context key variables; implement `GetAuthenticatedService(ctx context.Context) (*AuthenticatedService, error)` and `GetApiUser(ctx context.Context) (*ApiUser, error)` returning `(nil, ErrNoAuthContext)` on missing value
- [ ] 4.2 Write unit tests for context getters: returns correct value when set by `RequireAuth`, returns typed error (not panic) when key absent, returns typed error (not panic) on nil context

## 5. Redis API Key Cache

- [ ] 5.1 Implement `internal/service/auth/cache.go` — `ServiceAuthCache` struct with `Get(ctx, serviceID uuid.UUID) (*CachedServiceAuth, bool)`, `Set(ctx, serviceID, data *CachedServiceAuth, ttl time.Duration)`, `Invalidate(ctx, serviceID)` backed by Redis client; serialise with `encoding/json`; `CachedServiceAuth` contains the full service record and its non-expired API keys
- [ ] 5.2 Write unit tests for `ServiceAuthCache` using a mock Redis client: cache miss returns false, cache hit returns data, Set populates the key with correct TTL, Invalidate deletes the key, JSON round-trip preserves all fields, expired entry (nil return from Redis) treated as miss

## 6. Service Auth Middleware — JWT Path

- [ ] 6.1 Implement `RequireAuth` JWT path in `internal/middleware/auth.go`: detect `Bearer ` prefix (case-insensitive), decode JWT to extract `iss`, validate UUID format, call `ServiceAuthCache.Get` fallback to `repository.GetServiceByIDWithAPIKeys`, populate cache on miss, iterate non-expired keys and verify HMAC-SHA256; inject `AuthenticatedService` and `ApiUser` on first match; return appropriate 401/403 codes per failure case
- [ ] 6.2 Write unit tests: valid JWT + matching key → 200 + context populated; archived service → 403; service not found → 403; no matching key (all wrong) → 403; JWT `exp` expired → 403; two non-expired keys, second matches → 200 (key rotation); cache hit avoids second DB call

## 7. Service Auth Middleware — API Key Path

- [ ] 7.1 Implement `RequireAuth` API key path: detect `ApiKey-v1 ` prefix (exact-match, case-sensitive), hash supplied plaintext, call `repository.GetAPIKeyBySecret`, verify key not expired (`expiry_date IS NULL` or future) and service `active = true`; inject context values on success; return 403 on any failure
- [ ] 7.2 Write unit tests: valid API key → 200 + context populated; expired key (expiry_date in past) → 403; soft-revoked key (expiry_date set to now) → 403; nonexistent key → 403; service archived → 403; unrecognised auth scheme → 401

## 8. Router Wiring and Integration Tests

- [ ] 8.1 Wire all five auth middleware into `cmd/api/main.go` route groups: admin routes under `RequireAdminAuth`, `/sre-tools` under `RequireSREAuth`, `/cache-clear` under `RequireCacheClearAuth`, `/cypress` under `RequireCypressAuth`, `/v2/` and service routes under `RequireAuth`; mount `GET /_status`, `GET /version` outside all auth groups
- [ ] 8.2 Write integration tests (using `httptest.NewServer` + real chi router) for each auth group: no auth header → 401/403; valid token of correct type → 200; cross-issuer token (admin token on SRE route) → 401; confirm `/_status` reachable with no auth; confirm production environment guard on `/cypress`
