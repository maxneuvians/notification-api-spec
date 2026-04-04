## ADDED Requirements

### Requirement: Admin JWT middleware
`middleware.RequireAdminAuth` SHALL validate a `Bearer <jwt>` token where the JWT is HMAC-SHA256 signed, `iss` equals `config.AdminClientUserName`, and the signature is verified against `config.AdminClientSecret`. Clock skew of ±30 s SHALL be tolerated. On failure, the middleware SHALL return HTTP 401 with body `{"token": ["Invalid token: <reason>"]}`.

#### Scenario: Valid admin token passes
- **WHEN** a request carries a valid, non-expired admin JWT with the correct issuer and signature
- **THEN** the middleware calls the next handler with status 200

#### Scenario: Wrong issuer rejected
- **WHEN** a request carries an SRE JWT on an admin route
- **THEN** the middleware returns HTTP 401

#### Scenario: Tampered signature rejected
- **WHEN** the JWT payload is modified but the original signature is kept
- **THEN** the middleware returns HTTP 401

#### Scenario: Expired JWT rejected
- **WHEN** a JWT with `exp` more than 30 seconds in the past is presented
- **THEN** the middleware returns HTTP 401

---

### Requirement: SRE JWT middleware
`middleware.RequireSREAuth` SHALL validate a Bearer JWT where `iss` equals `config.SREUserName` and the signature is verified against `config.SREClientSecret`. Applied only to routes under `/sre-tools`. Same clock skew and error format as admin JWT.

#### Scenario: Valid SRE token accepted on /sre-tools
- **WHEN** a valid SRE JWT is sent to a `/sre-tools` route
- **THEN** the handler is invoked

#### Scenario: Admin token rejected on /sre-tools
- **WHEN** an admin JWT is sent to a `/sre-tools` route
- **THEN** HTTP 401 is returned

---

### Requirement: Cache-clear JWT middleware
`middleware.RequireCacheClearAuth` SHALL validate a Bearer JWT where `iss` equals `config.CacheClearUserName` and the signature is verified against `config.CacheClearClientSecret`. Applied only to routes under `/cache-clear`.

#### Scenario: Valid cache-clear token accepted
- **WHEN** a valid cache-clear JWT is sent to `/cache-clear`
- **THEN** the handler is invoked

---

### Requirement: Cypress JWT middleware with production guard
`middleware.RequireCypressAuth` SHALL validate a Bearer JWT where `iss` equals `config.CypressAuthUserName` and the signature is verified against `config.CypressAuthClientSecret`. Applied only to routes under `/cypress`. At request time, before JWT validation, if `config.NotifyEnvironment == "production"` the middleware SHALL return HTTP 403.

#### Scenario: Cypress route blocked in production
- **WHEN** any request reaches a `/cypress` route and the environment is `production`
- **THEN** HTTP 403 is returned regardless of token validity

#### Scenario: Cypress route allowed in non-production
- **WHEN** a valid Cypress JWT is sent to `/cypress` in a non-production environment
- **THEN** the handler is invoked

---

### Requirement: Service auth middleware — JWT path
When `Authorization: Bearer <jwt>` is presented, `middleware.RequireAuth` SHALL: (1) decode the JWT without verifying to extract `iss` (expected to be a service ID UUID); (2) fetch the service and its non-expired API keys from cache or DB; (3) attempt HMAC-SHA256 signature verification against each API key secret; (4) on first match, inject `AuthenticatedService` and `ApiUser` into the request context. Failure conditions: missing/malformed token → HTTP 401; service not found or archived → HTTP 403; no matching key / expired token → HTTP 403.

#### Scenario: Valid service JWT accepted
- **WHEN** a valid JWT signed with a service's API key secret is presented
- **THEN** `AuthenticatedService` and `ApiUser` are injected into context and the handler is invoked

#### Scenario: Archived service rejected
- **WHEN** the JWT issuer is a service ID of an archived service
- **THEN** HTTP 403 is returned

#### Scenario: Key rotation: both old and new keys accepted during overlap
- **WHEN** a service has two non-expired API keys and a JWT is signed with the second key
- **THEN** the middleware tries both keys and accepts the request on the second match

---

### Requirement: Service auth middleware — API key path
When `Authorization: ApiKey-v1 <plaintext_secret>` is presented, `middleware.RequireAuth` SHALL: (1) hash the supplied plaintext using the same hash algorithm used on create; (2) look up the API key record by hashed secret; (3) verify the key is not expired and the service is active; (4) inject `AuthenticatedService` and `ApiUser` into context. Failure: key not found or expired → HTTP 403.

#### Scenario: Valid legacy API key accepted
- **WHEN** `Authorization: ApiKey-v1 <valid_plaintext>` is presented
- **THEN** the service is authenticated and the handler is invoked

#### Scenario: Expired API key rejected
- **WHEN** an API key with a past `expiry_date` is presented
- **THEN** HTTP 403 is returned

---

### Requirement: Redis API key cache
Service and API key records retrieved from the DB during `RequireAuth` SHALL be cached in Redis keyed by `service_id` with a TTL of 30 seconds. Subsequent requests within the TTL window SHALL use the cached record without a DB query. On API key revocation or compromise, the cache entry for that service MUST be invalidated immediately.

#### Scenario: Cache hit avoids DB query
- **WHEN** two requests for the same service arrive within the 30 s TTL window
- **THEN** the second request retrieves the service+keys from Redis, not from the database

#### Scenario: Cache miss falls back to DB
- **WHEN** no cache entry exists for a service ID
- **THEN** the middleware queries the database and populates the cache

#### Scenario: Revoked key invalidates cache
- **WHEN** an API key is revoked via the service layer
- **THEN** the Redis cache key for that service is deleted synchronously

---

### Requirement: Auth context values
`middleware.RequireAuth` SHALL inject the following typed values into the request context on successful service authentication: `AuthenticatedService` (full service record including permissions) and `ApiUser` (the matched API key record including key_type). Handlers SHALL retrieve these values using typed getter functions `middleware.GetAuthenticatedService(ctx)` and `middleware.GetApiUser(ctx)`.

#### Scenario: Handler can read authenticated service
- **WHEN** a handler calls `middleware.GetAuthenticatedService(ctx)` after `RequireAuth` passes
- **THEN** the full service record is returned, not nil

#### Scenario: Missing context value returns typed error
- **WHEN** `middleware.GetAuthenticatedService(ctx)` is called on a context without auth (e.g. in a test)
- **THEN** a typed error is returned, not a nil-pointer panic
