package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	apiKeysRepo "github.com/maxneuvians/notification-api-spec/internal/repository/api_keys"
	servicesRepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	serviceauth "github.com/maxneuvians/notification-api-spec/internal/service/auth"
	"github.com/maxneuvians/notification-api-spec/pkg/signing"
)

type authRepoStub struct {
	service           servicesRepo.Service
	serviceErr        error
	permissions       []string
	permissionsErr    error
	apiKeys           []apiKeysRepo.ApiKey
	apiKeysErr        error
	secretLookup      map[string]apiKeysRepo.ApiKey
	secretLookupErr   error
	serviceCalls      int
	permissionsCalls  int
	serviceKeysCalls  int
	secretLookupCalls int
}

func (s *authRepoStub) GetServiceByIDWithAPIKeys(_ context.Context, id uuid.UUID) (servicesRepo.Service, error) {
	s.serviceCalls++
	if s.serviceErr != nil {
		return servicesRepo.Service{}, s.serviceErr
	}
	if s.service.ID != id {
		return servicesRepo.Service{}, sql.ErrNoRows
	}
	return s.service, nil
}

func (s *authRepoStub) GetServicePermissions(_ context.Context, _ uuid.UUID) ([]string, error) {
	s.permissionsCalls++
	if s.permissionsErr != nil {
		return nil, s.permissionsErr
	}
	return append([]string(nil), s.permissions...), nil
}

func (s *authRepoStub) GetAPIKeysByServiceID(_ context.Context, _ uuid.UUID) ([]apiKeysRepo.ApiKey, error) {
	s.serviceKeysCalls++
	if s.apiKeysErr != nil {
		return nil, s.apiKeysErr
	}
	return append([]apiKeysRepo.ApiKey(nil), s.apiKeys...), nil
}

func (s *authRepoStub) GetAPIKeyBySecret(_ context.Context, secret string) (apiKeysRepo.ApiKey, error) {
	s.secretLookupCalls++
	if s.secretLookupErr != nil {
		return apiKeysRepo.ApiKey{}, s.secretLookupErr
	}
	apiKey, ok := s.secretLookup[secret]
	if !ok {
		return apiKeysRepo.ApiKey{}, sql.ErrNoRows
	}
	return apiKey, nil
}

func TestDecodeBearerJWTRequest(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	validToken := makeJWT(t, "secret", map[string]any{"iss": "issuer", "iat": now.Unix(), "exp": now.Add(2 * time.Minute).Unix()})
	withinSkew := makeJWT(t, "secret", map[string]any{"iss": "issuer", "iat": now.Add(20 * time.Second).Unix(), "exp": now.Add(-20 * time.Second).Unix()})

	tests := []struct {
		name       string
		authHeader string
		wantReason string
		wantOK     bool
	}{
		{name: "valid token passes", authHeader: "Bearer " + validToken, wantOK: true},
		{name: "expired rejected", authHeader: "Bearer " + makeJWT(t, "secret", map[string]any{"iss": "issuer", "iat": now.Add(-2 * time.Minute).Unix(), "exp": now.Add(-31 * time.Second).Unix()}), wantReason: "token expired"},
		{name: "within skew accepted", authHeader: "bearer " + withinSkew, wantOK: true},
		{name: "wrong signature deferred to verification", authHeader: "Bearer " + makeJWT(t, "wrong", map[string]any{"iss": "issuer", "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()}), wantOK: true},
		{name: "missing issuer rejected", authHeader: "Bearer " + makeJWT(t, "secret", map[string]any{"iat": now.Unix(), "exp": now.Add(time.Minute).Unix()}), wantReason: "issuer missing"},
		{name: "missing expiry rejected", authHeader: "Bearer " + makeJWT(t, "secret", map[string]any{"iss": "issuer", "iat": now.Unix()}), wantReason: "expiry missing"},
		{name: "missing issued at rejected", authHeader: "Bearer " + makeJWT(t, "secret", map[string]any{"iss": "issuer", "exp": now.Add(time.Minute).Unix()}), wantReason: "issued-at missing"},
		{name: "future issued at rejected", authHeader: "Bearer " + makeJWT(t, "secret", map[string]any{"iss": "issuer", "iat": now.Add(31 * time.Second).Unix(), "exp": now.Add(time.Minute).Unix()}), wantReason: "token issued in the future"},
		{name: "malformed base64 rejected", authHeader: "Bearer a.!?.c", wantReason: "jwt header is malformed"},
		{name: "fewer than three parts rejected", authHeader: "Bearer a.b", wantReason: "jwt must contain three parts"},
		{name: "missing authorization header rejected", wantReason: "authorization header missing"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			token, _, err := decodeBearerJWTRequest(req, now)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("decodeBearerJWTRequest() error = %v, want nil", err)
				}
				if tc.name == "wrong signature deferred to verification" {
					if verifyErr := verifyJWTSignature(token, "secret"); verifyErr == nil {
						t.Fatal("verifyJWTSignature() error = nil, want signature mismatch")
					}
				}
				return
			}

			if err == nil {
				t.Fatal("decodeBearerJWTRequest() error = nil, want error")
			}
			if err.Error() != tc.wantReason {
				t.Fatalf("error = %q, want %q", err.Error(), tc.wantReason)
			}
		})
	}
}

func TestJWTHelpersAndPanics(t *testing.T) {
	t.Run("bearer token requires bearer scheme", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Basic abc")
		if _, err := bearerTokenFromRequest(req); err == nil || err.Error() != "authorization header must use Bearer scheme" {
			t.Fatalf("bearerTokenFromRequest() error = %v", err)
		}
	})

	t.Run("api key requires exact scheme", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "apikey-v1 secret")
		if _, err := apiKeyFromRequest(req); err == nil || err.Error() != "authorization header must use ApiKey-v1 scheme" {
			t.Fatalf("apiKeyFromRequest() error = %v", err)
		}
	})

	t.Run("api key requires token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", apiKeySchemePrefix)
		if _, err := apiKeyFromRequest(req); err == nil || err.Error() != "authorization token missing" {
			t.Fatalf("apiKeyFromRequest() error = %v", err)
		}
	})

	t.Run("parse api key rejects invalid values", func(t *testing.T) {
		cases := []string{
			"wrongprefix-123",
			"gcntfy-short",
			"gcntfy-not-a-uuid123e4567-e89b-12d3-a456-426614174000",
			"gcntfy-123e4567-e89b-12d3-a456-426614174000not-a-uuid-token-value-123456",
		}
		for _, plaintext := range cases {
			if _, _, err := parseAPIKeyValue("gcntfy-", plaintext); err == nil || err.Error() != "invalid api key format" {
				t.Fatalf("parseAPIKeyValue(%q) error = %v", plaintext, err)
			}
		}
	})

	t.Run("service auth secrets fall back to secret key", func(t *testing.T) {
		secrets := serviceAuthSecrets(Config{SecretKey: []string{"fallback"}})
		if len(secrets) != 1 || secrets[0] != "fallback" {
			t.Fatalf("serviceAuthSecrets() = %v, want [fallback]", secrets)
		}
	})

	t.Run("admin auth panics without secret", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic")
			}
		}()
		_ = RequireAdminAuth(Config{AdminClientUserName: "notify-admin"})
	})

	t.Run("cypress auth panics without secret", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic")
			}
		}()
		_ = RequireCypressAuth(Config{CypressAuthUserName: "cypress"})
	})

	t.Run("unsupported algorithm rejected", func(t *testing.T) {
		token := makeJWTWithHeader(t, map[string]any{"alg": "HS512", "typ": "JWT"}, "secret", map[string]any{"iss": "issuer", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix()})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		if _, _, err := decodeBearerJWTRequest(req, time.Now()); err == nil || err.Error() != "unsupported signing algorithm" {
			t.Fatalf("decodeBearerJWTRequest() error = %v", err)
		}
	})
}

func TestSimpleJWTMiddleware(t *testing.T) {
	adminCfg := Config{AdminClientUserName: "notify-admin", AdminClientSecret: "admin-secret"}
	sreCfg := Config{SREUserName: "notify-sre", SREClientSecret: "sre-secret"}
	cacheCfg := Config{CacheClearUserName: "cache-clear", CacheClearClientSecret: "cache-secret"}
	now := time.Now()

	adminToken := makeJWT(t, adminCfg.AdminClientSecret, map[string]any{"iss": adminCfg.AdminClientUserName, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})
	adminWrongIssuerToken := makeJWT(t, adminCfg.AdminClientSecret, map[string]any{"iss": sreCfg.SREUserName, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})
	sreToken := makeJWT(t, sreCfg.SREClientSecret, map[string]any{"iss": sreCfg.SREUserName, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})
	cacheToken := makeJWT(t, cacheCfg.CacheClearClientSecret, map[string]any{"iss": cacheCfg.CacheClearUserName, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})
	cacheWrongIssuerToken := makeJWT(t, cacheCfg.CacheClearClientSecret, map[string]any{"iss": adminCfg.AdminClientUserName, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})

	t.Run("sre token on sre routes passes", func(t *testing.T) {
		res := exerciseMiddleware(t, RequireSREAuth(sreCfg), "/sre-tools", "Bearer "+sreToken)
		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", res.Code)
		}
	})

	t.Run("admin token on sre routes rejected", func(t *testing.T) {
		res := exerciseMiddleware(t, RequireSREAuth(sreCfg), "/sre-tools", "Bearer "+makeJWT(t, sreCfg.SREClientSecret, map[string]any{"iss": adminCfg.AdminClientUserName, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()}))
		assertTokenErrorBody(t, res, http.StatusUnauthorized, "issuer mismatch")
	})

	t.Run("cache clear wrong issuer rejected", func(t *testing.T) {
		res := exerciseMiddleware(t, RequireCacheClearAuth(cacheCfg), "/cache-clear", "Bearer "+cacheWrongIssuerToken)
		assertTokenErrorBody(t, res, http.StatusUnauthorized, "issuer mismatch")
	})

	t.Run("shared jwt middleware cases", func(t *testing.T) {
		cases := []struct {
			name       string
			authHeader string
			wantStatus int
			reason     string
		}{
			{name: "valid token", authHeader: "Bearer " + adminToken, wantStatus: http.StatusOK},
			{name: "expired token", authHeader: "Bearer " + makeJWT(t, adminCfg.AdminClientSecret, map[string]any{"iss": adminCfg.AdminClientUserName, "iat": now.Add(-2 * time.Minute).Unix(), "exp": now.Add(-time.Minute).Unix()}), wantStatus: http.StatusUnauthorized, reason: "token expired"},
			{name: "wrong issuer", authHeader: "Bearer " + adminWrongIssuerToken, wantStatus: http.StatusUnauthorized, reason: "issuer mismatch"},
			{name: "tampered signature", authHeader: "Bearer " + tamperJWTPayload(t, adminToken), wantStatus: http.StatusUnauthorized, reason: "signature mismatch"},
			{name: "missing header", wantStatus: http.StatusUnauthorized, reason: "authorization header missing"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				res := exerciseMiddleware(t, RequireAdminAuth(adminCfg), "/admin", tc.authHeader)
				if tc.wantStatus == http.StatusOK {
					if res.Code != http.StatusOK {
						t.Fatalf("status = %d, want 200", res.Code)
					}
					return
				}
				assertTokenErrorBody(t, res, tc.wantStatus, tc.reason)
			})
		}
	})

	t.Run("cache clear valid token accepted", func(t *testing.T) {
		res := exerciseMiddleware(t, RequireCacheClearAuth(cacheCfg), "/cache-clear", "Bearer "+cacheToken)
		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", res.Code)
		}
	})
}

func TestRequireCypressAuth(t *testing.T) {
	validCfg := Config{
		NotifyEnvironment:       "staging",
		CypressAuthUserName:     "cypress",
		CypressAuthClientSecret: "cypress-secret",
	}
	productionCfg := validCfg
	productionCfg.NotifyEnvironment = "production"
	now := time.Now()
	validToken := makeJWT(t, validCfg.CypressAuthClientSecret, map[string]any{"iss": validCfg.CypressAuthUserName, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})
	invalidToken := makeJWT(t, "wrong", map[string]any{"iss": validCfg.CypressAuthUserName, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})

	t.Run("production blocks all requests", func(t *testing.T) {
		for _, authHeader := range []string{"", "Bearer " + validToken, "Bearer " + invalidToken} {
			res := exerciseMiddleware(t, RequireCypressAuth(productionCfg), "/cypress", authHeader)
			if res.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", res.Code)
			}
		}
	})

	t.Run("non production valid token passes", func(t *testing.T) {
		res := exerciseMiddleware(t, RequireCypressAuth(validCfg), "/cypress", "Bearer "+validToken)
		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", res.Code)
		}
	})

	t.Run("non production invalid token returns 401", func(t *testing.T) {
		res := exerciseMiddleware(t, RequireCypressAuth(validCfg), "/cypress", "Bearer "+invalidToken)
		assertTokenErrorBody(t, res, http.StatusUnauthorized, "signature mismatch")
	})

	t.Run("non production missing token returns 401", func(t *testing.T) {
		res := exerciseMiddleware(t, RequireCypressAuth(validCfg), "/cypress", "")
		assertTokenErrorBody(t, res, http.StatusUnauthorized, "authorization header missing")
	})
}

func TestAuthContextGetters(t *testing.T) {
	service := &AuthenticatedService{Service: servicesRepo.Service{ID: uuid.New(), Name: "svc"}, Permissions: []string{"send_sms"}}
	apiUser := &ApiUser{ApiKey: apiKeysRepo.ApiKey{ID: uuid.New(), KeyType: "normal"}}

	t.Run("returns values when present", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), authenticatedServiceContextKey, service)
		ctx = context.WithValue(ctx, apiUserContextKey, apiUser)

		gotService, err := GetAuthenticatedService(ctx)
		if err != nil {
			t.Fatalf("GetAuthenticatedService() error = %v", err)
		}
		if gotService.Service.ID != service.Service.ID {
			t.Fatalf("service id = %v, want %v", gotService.Service.ID, service.Service.ID)
		}

		gotUser, err := GetApiUser(ctx)
		if err != nil {
			t.Fatalf("GetApiUser() error = %v", err)
		}
		if gotUser.ApiKey.ID != apiUser.ApiKey.ID {
			t.Fatalf("api key id = %v, want %v", gotUser.ApiKey.ID, apiUser.ApiKey.ID)
		}
	})

	t.Run("missing values return typed error", func(t *testing.T) {
		if _, err := GetAuthenticatedService(context.Background()); !errors.Is(err, ErrNoAuthContext) {
			t.Fatalf("GetAuthenticatedService() error = %v, want ErrNoAuthContext", err)
		}
		if _, err := GetApiUser(context.Background()); !errors.Is(err, ErrNoAuthContext) {
			t.Fatalf("GetApiUser() error = %v, want ErrNoAuthContext", err)
		}
	})

	t.Run("nil context returns typed error", func(t *testing.T) {
		var nilCtx context.Context
		if _, err := GetAuthenticatedService(nilCtx); !errors.Is(err, ErrNoAuthContext) {
			t.Fatalf("GetAuthenticatedService(nil context) error = %v, want ErrNoAuthContext", err)
		}
		if _, err := GetApiUser(nilCtx); !errors.Is(err, ErrNoAuthContext) {
			t.Fatalf("GetApiUser(nil context) error = %v, want ErrNoAuthContext", err)
		}
	})
}

func TestRequireAuthJWTPath(t *testing.T) {
	now := time.Now()
	serviceID := uuid.New()
	firstKey := apiKeysRepo.ApiKey{ID: uuid.New(), ServiceID: serviceID, Secret: "wrong-secret", KeyType: "normal"}
	secondKey := apiKeysRepo.ApiKey{ID: uuid.New(), ServiceID: serviceID, Secret: "matching-secret", KeyType: "team"}
	repo := &authRepoStub{
		service:     servicesRepo.Service{ID: serviceID, Name: "service", Active: true},
		permissions: []string{"send_emails"},
		apiKeys:     []apiKeysRepo.ApiKey{firstKey, secondKey},
	}
	cache := serviceauth.NewServiceAuthCache(nil)
	middleware := RequireAuth(Config{APIKeyPrefix: "gcntfy-"}, cache, repo)

	t.Run("valid jwt with second key populates context", func(t *testing.T) {
		jwt := makeJWT(t, secondKey.Secret, map[string]any{"iss": serviceID.String(), "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})
		res := httptest.NewRecorder()
		var gotService *AuthenticatedService
		var gotUser *ApiUser

		middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			gotService, err = GetAuthenticatedService(r.Context())
			if err != nil {
				t.Fatalf("GetAuthenticatedService() error = %v", err)
			}
			gotUser, err = GetApiUser(r.Context())
			if err != nil {
				t.Fatalf("GetApiUser() error = %v", err)
			}
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(res, withAuthHeader(httptest.NewRequest(http.MethodGet, "/v2/notifications", nil), "Bearer "+jwt))

		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", res.Code)
		}
		if gotService == nil || gotService.Service.ID != serviceID {
			t.Fatalf("authenticated service = %#v, want service id %v", gotService, serviceID)
		}
		if gotUser == nil || gotUser.ApiKey.ID != secondKey.ID {
			t.Fatalf("api user = %#v, want api key id %v", gotUser, secondKey.ID)
		}
	})

	t.Run("archived service rejected", func(t *testing.T) {
		repo.service.Active = false
		jwt := makeJWT(t, secondKey.Secret, map[string]any{"iss": serviceID.String(), "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})
		res := exerciseMiddleware(t, middleware, "/v2/notifications", "Bearer "+jwt)
		if res.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", res.Code)
		}
		repo.service.Active = true
	})

	t.Run("service not found rejected", func(t *testing.T) {
		jwt := makeJWT(t, secondKey.Secret, map[string]any{"iss": uuid.New().String(), "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})
		res := exerciseMiddleware(t, middleware, "/v2/notifications", "Bearer "+jwt)
		if res.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", res.Code)
		}
	})

	t.Run("no matching key rejected", func(t *testing.T) {
		jwt := makeJWT(t, "no-match", map[string]any{"iss": serviceID.String(), "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})
		res := exerciseMiddleware(t, middleware, "/v2/notifications", "Bearer "+jwt)
		if res.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", res.Code)
		}
	})

	t.Run("expired jwt rejected", func(t *testing.T) {
		jwt := makeJWT(t, secondKey.Secret, map[string]any{"iss": serviceID.String(), "iat": now.Add(-2 * time.Minute).Unix(), "exp": now.Add(-time.Minute).Unix()})
		res := exerciseMiddleware(t, middleware, "/v2/notifications", "Bearer "+jwt)
		if res.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", res.Code)
		}
	})

	t.Run("cache hit avoids second db call", func(t *testing.T) {
		cacheData := &serviceauth.CachedServiceAuth{Service: repo.service, Permissions: repo.permissions, APIKeys: repo.apiKeys}
		store := &serviceauthTestStore{values: map[string]string{}}
		cache := serviceauth.NewServiceAuthCache(store)
		cache.Set(context.Background(), serviceID, cacheData, time.Minute)
		cachedRepo := &authRepoStub{service: repo.service, permissions: repo.permissions, apiKeys: repo.apiKeys}
		mw := RequireAuth(Config{APIKeyPrefix: "gcntfy-"}, cache, cachedRepo)
		jwt := makeJWT(t, secondKey.Secret, map[string]any{"iss": serviceID.String(), "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})

		res := exerciseMiddleware(t, mw, "/v2/notifications", "Bearer "+jwt)
		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", res.Code)
		}
		if cachedRepo.serviceCalls != 0 || cachedRepo.permissionsCalls != 0 || cachedRepo.serviceKeysCalls != 0 {
			t.Fatalf("unexpected db calls: service=%d permissions=%d keys=%d", cachedRepo.serviceCalls, cachedRepo.permissionsCalls, cachedRepo.serviceKeysCalls)
		}
	})
}

func TestRequireAuthAPIKeyPath(t *testing.T) {
	serviceID := uuid.New()
	plaintextToken := uuid.New().String()
	plaintextKey := "gcntfy-" + serviceID.String() + plaintextToken
	variant, err := signing.SignAPIKeyToken(plaintextToken, "current-secret")
	if err != nil {
		t.Fatalf("SignAPIKeyToken() error = %v", err)
	}
	matchedKey := apiKeysRepo.ApiKey{ID: uuid.New(), ServiceID: serviceID, Secret: variant, KeyType: "normal"}
	repo := &authRepoStub{
		service:      servicesRepo.Service{ID: serviceID, Name: "service", Active: true},
		permissions:  []string{"send_sms"},
		apiKeys:      []apiKeysRepo.ApiKey{matchedKey},
		secretLookup: map[string]apiKeysRepo.ApiKey{variant: matchedKey},
	}
	cache := serviceauth.NewServiceAuthCache(nil)
	mw := RequireAuth(Config{APIKeyPrefix: "gcntfy-", SecretKeys: []string{"old-secret", "current-secret"}}, cache, repo)

	t.Run("valid api key populates context", func(t *testing.T) {
		res := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := GetAuthenticatedService(r.Context()); err != nil {
				t.Fatalf("GetAuthenticatedService() error = %v", err)
			}
			if _, err := GetApiUser(r.Context()); err != nil {
				t.Fatalf("GetApiUser() error = %v", err)
			}
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(res, withAuthHeader(httptest.NewRequest(http.MethodGet, "/v2/notifications", nil), apiKeySchemePrefix+plaintextKey))

		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", res.Code)
		}
	})

	t.Run("expired key rejected", func(t *testing.T) {
		expired := matchedKey
		expired.ExpiryDate = sql.NullTime{Time: time.Now().Add(-time.Minute), Valid: true}
		res := exerciseMiddleware(t, RequireAuth(Config{APIKeyPrefix: "gcntfy-", SecretKeys: []string{"current-secret"}}, cache, &authRepoStub{service: repo.service, permissions: repo.permissions, apiKeys: []apiKeysRepo.ApiKey{expired}, secretLookup: map[string]apiKeysRepo.ApiKey{variant: expired}}), "/v2/notifications", apiKeySchemePrefix+plaintextKey)
		if res.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", res.Code)
		}
	})

	t.Run("soft revoked key rejected", func(t *testing.T) {
		revoked := matchedKey
		revoked.ExpiryDate = sql.NullTime{Time: time.Now(), Valid: true}
		res := exerciseMiddleware(t, RequireAuth(Config{APIKeyPrefix: "gcntfy-", SecretKeys: []string{"current-secret"}}, cache, &authRepoStub{service: repo.service, permissions: repo.permissions, apiKeys: []apiKeysRepo.ApiKey{revoked}, secretLookup: map[string]apiKeysRepo.ApiKey{variant: revoked}}), "/v2/notifications", apiKeySchemePrefix+plaintextKey)
		if res.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", res.Code)
		}
	})

	t.Run("nonexistent key rejected", func(t *testing.T) {
		res := exerciseMiddleware(t, mw, "/v2/notifications", apiKeySchemePrefix+"gcntfy-"+uuid.New().String()+uuid.New().String())
		if res.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", res.Code)
		}
	})

	t.Run("service archived rejected", func(t *testing.T) {
		archivedRepo := &authRepoStub{service: servicesRepo.Service{ID: serviceID, Name: "service", Active: false}, permissions: repo.permissions, apiKeys: repo.apiKeys, secretLookup: repo.secretLookup}
		res := exerciseMiddleware(t, RequireAuth(Config{APIKeyPrefix: "gcntfy-", SecretKeys: []string{"current-secret"}}, cache, archivedRepo), "/v2/notifications", apiKeySchemePrefix+plaintextKey)
		if res.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", res.Code)
		}
	})

	t.Run("unrecognised auth scheme rejected with 401", func(t *testing.T) {
		res := exerciseMiddleware(t, mw, "/v2/notifications", "Basic abc")
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", res.Code)
		}
	})

	t.Run("service mismatch rejected", func(t *testing.T) {
		other := matchedKey
		other.ServiceID = uuid.New()
		res := exerciseMiddleware(t, RequireAuth(Config{APIKeyPrefix: "gcntfy-", SecretKeys: []string{"current-secret"}}, cache, &authRepoStub{service: repo.service, permissions: repo.permissions, apiKeys: []apiKeysRepo.ApiKey{other}, secretLookup: map[string]apiKeysRepo.ApiKey{variant: other}}), "/v2/notifications", apiKeySchemePrefix+plaintextKey)
		if res.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", res.Code)
		}
	})

	t.Run("lookup error rejected", func(t *testing.T) {
		res := exerciseMiddleware(t, RequireAuth(Config{APIKeyPrefix: "gcntfy-", SecretKeys: []string{"current-secret"}}, cache, &authRepoStub{secretLookupErr: errors.New("lookup failed")}), "/v2/notifications", apiKeySchemePrefix+plaintextKey)
		if res.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", res.Code)
		}
	})
}

type serviceauthTestStore struct {
	values map[string]string
}

func (s *serviceauthTestStore) Get(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *serviceauthTestStore) Set(_ context.Context, key string, value string, _ time.Duration) error {
	if s.values == nil {
		s.values = make(map[string]string)
	}
	s.values[key] = value
	return nil
}

func (s *serviceauthTestStore) Del(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func exerciseMiddleware(t *testing.T, middleware func(http.Handler) http.Handler, path, authHeader string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	res := httptest.NewRecorder()
	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(res, req)
	return res
}

func assertTokenErrorBody(t *testing.T, res *httptest.ResponseRecorder, wantStatus int, reason string) {
	t.Helper()
	if res.Code != wantStatus {
		t.Fatalf("status = %d, want %d", res.Code, wantStatus)
	}

	var body map[string][]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	want := "Invalid token: " + reason
	if len(body["token"]) != 1 || body["token"][0] != want {
		t.Fatalf("body token = %#v, want [%q]", body["token"], want)
	}
}

func makeJWT(t *testing.T, secret string, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString(mustJSON(t, map[string]any{"alg": "HS256", "typ": "JWT"}))
	payload := base64.RawURLEncoding.EncodeToString(mustJSON(t, claims))
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + signature
}

func makeJWTWithHeader(t *testing.T, headerClaims map[string]any, secret string, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString(mustJSON(t, headerClaims))
	payload := base64.RawURLEncoding.EncodeToString(mustJSON(t, claims))
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + signature
}

func tamperJWTPayload(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token parts = %d, want 3", len(parts))
	}

	payload := map[string]any{}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	payload["iss"] = "tampered-issuer"
	parts[1] = base64.RawURLEncoding.EncodeToString(mustJSON(t, payload))
	return strings.Join(parts, ".")
}

func withAuthHeader(req *http.Request, authHeader string) *http.Request {
	req.Header.Set("Authorization", authHeader)
	return req
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return encoded
}
