## Source Files

- `openspec/changes/authentication-middleware/specs/auth-middleware/spec.md`
- `spec/behavioral-spec/users-auth.md` §authentication section

## Requirements

### R1: Admin JWT Middleware

- `middleware.RequireAdminAuth(cfg)` validates `Authorization: Bearer <jwt>`
- JWT: HMAC-SHA256 signed, `iss == cfg.AdminClientUserName`, signature verified against `cfg.AdminClientSecret`
- Clock skew: ±30 s on `exp` (reject if `exp + 30 s < now`)
- On failure: HTTP 401, body `{"token": ["Invalid token: <reason>"]}`
- Mounted on the admin UI route group (`/service/`, `/user/`, `/organisation/`, etc.)

### R2: SRE JWT Middleware

- `middleware.RequireSREAuth(cfg)` validates Bearer JWT
- `iss == cfg.SREUserName`, secret `cfg.SREClientSecret`
- Same ±30 s skew and `{"token": [...]}` error format
- Applied ONLY to routes under `/sre-tools`

### R3: Cache-Clear JWT Middleware

- `middleware.RequireCacheClearAuth(cfg)` validates Bearer JWT
- `iss == cfg.CacheClearUserName`, secret `cfg.CacheClearClientSecret`
- Applied ONLY to routes under `/cache-clear`

### R4: Cypress JWT Middleware — Production Guard

- `middleware.RequireCypressAuth(cfg)` validates Bearer JWT
- `iss == cfg.CypressAuthUserName`, secret `cfg.CypressAuthClientSecret`
- Applied ONLY to routes under `/cypress`
- **Before JWT validation**: if `cfg.NotifyEnvironment == "production"` → HTTP 403 immediately (any token)
- Non-production with valid token → delegate to next handler

### R5: Service Auth Middleware — JWT Path

- Auth header: `Authorization: Bearer <jwt>`
- Step 1: Decode JWT header+payload WITHOUT signature verification; extract `iss` (expect service UUID)
- Step 2: Fetch service + all non-expired API keys from `ServiceAuthCache`; on miss query `api_keys.GetServiceByIDWithAPIKeys` then populate cache
- Step 3: Iterate non-expired keys; attempt HMAC-SHA256 verify against each key's secret
- Step 4: On first match → inject `AuthenticatedService` + `ApiUser` into request context; call next handler
- Missing or malformed Authorization header → HTTP 401 `{"result": "error", "message": "Unauthorized, authentication token must be provided"}`
- JWT parse failure (invalid base64, < 3 parts) → HTTP 401
- `iss` is not a valid UUID → HTTP 401
- Service not found in DB → HTTP 403
- Service archived (`active=false`) → HTTP 403
- No key matches / all expired → HTTP 403
- JWT `exp` expired (> 30 s ago) → HTTP 403

### R6: Service Auth Middleware — API Key Path

- Auth header: `Authorization: ApiKey-v1 <plaintext_secret>` (prefix match is exact, case-sensitive)
- Hash the plaintext using same algorithm as key creation (SHA-512 with salt, per DB schema)
- Lookup `api_keys` table by `hashed_secret = hash(plaintext)`
- Verify: key's `expiry_date` is null or in the future; service `active = true`
- On success: inject `AuthenticatedService` + `ApiUser` into context
- Key not found → HTTP 403
- Key expired → HTTP 403
- Service archived → HTTP 403

### R7: Redis API Key Cache

- Key format: string keyed by `service_id`
- Value: JSON-serialised `{ service: Service, api_keys: []ApiKey }` (non-expired keys only)
- TTL: 30 seconds
- On cache MISS: query DB, populate cache, proceed
- On cache HIT within TTL: deserialise, proceed; no DB query
- On API key revocation: call `ServiceAuthCache.Invalidate(ctx, serviceID)` — synchronous DELETE of the Redis key
- Both JWT path and API key path go through the same cache object

### R8: Auth Context Values

- `middleware.RequireAuth` injects on success:
  - `AuthenticatedService` — full `Service` struct including permissions, `research_mode`, `active`, `message_limit`, etc.
  - `ApiUser` — matched `ApiKey` struct including `key_type` (`normal`, `team`, `test`), `expiry_date`
- Context keys: unexported typed struct keys to prevent collision
- Getters: `middleware.GetAuthenticatedService(ctx) (*Service, error)` and `middleware.GetApiUser(ctx) (*ApiKey, error)`
- If called on context without auth values → returns `(nil, ErrNoAuthContext)`; never panics

## Error Conditions

| Condition | HTTP Status | Body |
|---|---|---|
| Missing `Authorization` header on any protected route | 401 | `{"result": "error", "message": "Unauthorized, authentication token must be provided"}` |
| Bearer token on admin/SRE/cache-clear/Cypress route: malformed JWT | 401 | `{"token": ["Invalid token: malformed JWT"]}` |
| Bearer token on admin route: expired (> 30 s) | 401 | `{"token": ["Invalid token: token is expired by …"]}` |
| Bearer token on admin route: wrong issuer | 401 | `{"token": ["Invalid token: wrong issuer"]}` |
| Bearer token on admin route: signature mismatch | 401 | `{"token": ["Invalid token: signature is invalid"]}` |
| Cypress route: any request in production | 403 | empty |
| Service JWT: `iss` not a valid UUID | 401 | `{"result": "error", "message": "Unauthorized…"}` |
| Service JWT: service not found / archived | 403 | — |
| Service JWT: no matching key | 403 | — |
| API key: not found / expired | 403 | — |

## Business Rules

- JWT decode order: extract `iss` without verifying, select the issuer-specific secret, verify HMAC-SHA256 — safe because attacker cannot change `iss` after bypass (signature still fails)
- All four internal middleware share the same JWT structure (HS256, `iss` + `exp`/`iat`); secrets differ per route group
- Service JWT: `iss` is a service UUID; service's API keys are the candidate secrets
- Key rotation: service may have > 1 non-expired key; try each sequentially until one verifies
- Redis TTL 30 s: acceptable stale-revocation window; immediate flush handles key compromise
- No per-route authorization: middleware only verifies identity; per-resource permission checks are handler responsibility
- Routes outside all auth groups (no auth required): `GET /_status`, `GET /version`, `GET /_metrics`
- `ApiKey-v1 ` prefix detection is case-sensitive (Python reference uses exact match)
