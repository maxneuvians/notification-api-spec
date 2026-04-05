package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	apiKeysRepo "github.com/maxneuvians/notification-api-spec/internal/repository/api_keys"
	servicesRepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	serviceauth "github.com/maxneuvians/notification-api-spec/internal/service/auth"
	"github.com/maxneuvians/notification-api-spec/pkg/signing"
)

func TestListenAddr(t *testing.T) {
	tests := []struct {
		port string
		want string
	}{
		{port: "8080", want: ":8080"},
		{port: ":9090", want: ":9090"},
		{port: "127.0.0.1:7000", want: "127.0.0.1:7000"},
	}

	for _, tc := range tests {
		if got := listenAddr(tc.port); got != tc.want {
			t.Fatalf("listenAddr(%q) = %q, want %q", tc.port, got, tc.want)
		}
	}
}

func TestOpenDBReturnsError(t *testing.T) {
	if _, err := openDB("postgresql://127.0.0.1:1/notify?connect_timeout=1&sslmode=disable", 1); err == nil {
		t.Fatal("expected error, got nil")
	}
}

type fakeServer struct {
	listenErr error
	called    bool
}

func (s *fakeServer) ListenAndServe() error {
	s.called = true
	return s.listenErr
}

type stubServiceQueries struct {
	service     servicesRepo.Service
	serviceErr  error
	permissions []string
	permErr     error
}

func (s *stubServiceQueries) GetServiceByIDWithAPIKeys(_ context.Context, _ uuid.UUID) (servicesRepo.Service, error) {
	return s.service, s.serviceErr
}

func (s *stubServiceQueries) GetServicePermissions(_ context.Context, _ uuid.UUID) ([]string, error) {
	return s.permissions, s.permErr
}

type stubAPIKeyQueries struct {
	apiKeys []apiKeysRepo.ApiKey
	keysErr error
	apiKey  apiKeysRepo.ApiKey
	keyErr  error
}

func (s *stubAPIKeyQueries) GetAPIKeysByServiceID(_ context.Context, _ uuid.UUID) ([]apiKeysRepo.ApiKey, error) {
	return s.apiKeys, s.keysErr
}

func (s *stubAPIKeyQueries) GetAPIKeyBySecret(_ context.Context, _ string) (apiKeysRepo.ApiKey, error) {
	return s.apiKey, s.keyErr
}

type authRepoStub struct {
	service      servicesRepo.Service
	permissions  []string
	apiKeys      []apiKeysRepo.ApiKey
	secretLookup map[string]apiKeysRepo.ApiKey
}

func (s *authRepoStub) GetServiceByIDWithAPIKeys(_ context.Context, id uuid.UUID) (servicesRepo.Service, error) {
	if s.service.ID != id {
		return servicesRepo.Service{}, sql.ErrNoRows
	}
	return s.service, nil
}

func (s *authRepoStub) GetServicePermissions(_ context.Context, _ uuid.UUID) ([]string, error) {
	return append([]string(nil), s.permissions...), nil
}

func (s *authRepoStub) GetAPIKeysByServiceID(_ context.Context, _ uuid.UUID) ([]apiKeysRepo.ApiKey, error) {
	return append([]apiKeysRepo.ApiKey(nil), s.apiKeys...), nil
}

func (s *authRepoStub) GetAPIKeyBySecret(_ context.Context, secret string) (apiKeysRepo.ApiKey, error) {
	apiKey, ok := s.secretLookup[secret]
	if !ok {
		return apiKeysRepo.ApiKey{}, sql.ErrNoRows
	}
	return apiKey, nil
}

func TestNewRouterAuthGroups(t *testing.T) {
	cfg := &config.Config{
		AdminBaseURL:            "https://admin.example.com",
		AttachmentNumLimit:      1,
		AttachmentSizeLimit:     1024,
		RateLimitPerSecond:      10,
		RateLimitBurst:          20,
		APIKeyPrefix:            "gcntfy-",
		AdminClientUserName:     "notify-admin",
		AdminClientSecret:       "admin-secret",
		SREUserName:             "notify-sre",
		SREClientSecret:         "sre-secret",
		CacheClearUserName:      "cache-clear",
		CacheClearClientSecret:  "cache-secret",
		CypressAuthUserName:     "cypress",
		CypressAuthClientSecret: "cypress-secret",
		SecretKeys:              []string{"current-secret"},
	}
	now := time.Now()
	serviceID := uuid.New()
	serviceJWTSecret := "service-jwt-secret"
	serviceToken := makeJWT(t, serviceJWTSecret, map[string]any{"iss": serviceID.String(), "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})
	adminToken := makeJWT(t, cfg.AdminClientSecret, map[string]any{"iss": cfg.AdminClientUserName, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})
	sreToken := makeJWT(t, cfg.SREClientSecret, map[string]any{"iss": cfg.SREUserName, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})
	cacheToken := makeJWT(t, cfg.CacheClearClientSecret, map[string]any{"iss": cfg.CacheClearUserName, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})
	cypressToken := makeJWT(t, cfg.CypressAuthClientSecret, map[string]any{"iss": cfg.CypressAuthUserName, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})

	plaintextToken := uuid.New().String()
	plaintextKey := cfg.APIKeyPrefix + serviceID.String() + plaintextToken
	apiKeySecret, err := signing.SignAPIKeyToken(plaintextToken, cfg.SecretKeys[0])
	if err != nil {
		t.Fatalf("SignAPIKeyToken() error = %v", err)
	}

	apiKey := apiKeysRepo.ApiKey{ID: uuid.New(), ServiceID: serviceID, Secret: apiKeySecret, KeyType: "normal"}
	authRepo := &authRepoStub{
		service:      servicesRepo.Service{ID: serviceID, Name: "service", Active: true},
		permissions:  []string{"send_emails"},
		apiKeys:      []apiKeysRepo.ApiKey{{ID: uuid.New(), ServiceID: serviceID, Secret: "wrong-secret", KeyType: "normal"}, {ID: apiKey.ID, ServiceID: serviceID, Secret: serviceJWTSecret, KeyType: apiKey.KeyType}},
		secretLookup: map[string]apiKeysRepo.ApiKey{apiKeySecret: apiKey},
	}

	t.Run("status route is public", func(t *testing.T) {
		server := httptest.NewServer(newRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, authRepo, nil, nil))
		defer server.Close()

		res, err := http.Get(server.URL + "/_status")
		if err != nil {
			t.Fatalf("GET /_status: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", res.StatusCode)
		}
	})

	t.Run("version route is public", func(t *testing.T) {
		server := httptest.NewServer(newRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, authRepo, nil, nil))
		defer server.Close()

		res, err := http.Get(server.URL + "/version")
		if err != nil {
			t.Fatalf("GET /version: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", res.StatusCode)
		}
	})

	t.Run("no auth header rejected for protected routes", func(t *testing.T) {
		server := httptest.NewServer(newRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, authRepo, nil, nil))
		defer server.Close()

		cases := []struct {
			path       string
			wantStatus int
		}{
			{path: "/admin/ping", wantStatus: http.StatusUnauthorized},
			{path: "/sre-tools/ping", wantStatus: http.StatusUnauthorized},
			{path: "/cache-clear/ping", wantStatus: http.StatusUnauthorized},
			{path: "/cypress/ping", wantStatus: http.StatusUnauthorized},
			{path: "/v2/ping", wantStatus: http.StatusUnauthorized},
		}

		for _, tc := range cases {
			res, err := http.Get(server.URL + tc.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tc.path, err)
			}
			res.Body.Close()
			if res.StatusCode != tc.wantStatus {
				t.Fatalf("%s status = %d, want %d", tc.path, res.StatusCode, tc.wantStatus)
			}
		}
	})

	t.Run("valid token of correct type passes", func(t *testing.T) {
		server := httptest.NewServer(newRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, authRepo, nil, nil))
		defer server.Close()

		cases := []struct {
			path       string
			authHeader string
		}{
			{path: "/admin/ping", authHeader: "Bearer " + adminToken},
			{path: "/sre-tools/ping", authHeader: "Bearer " + sreToken},
			{path: "/cache-clear/ping", authHeader: "Bearer " + cacheToken},
			{path: "/cypress/ping", authHeader: "Bearer " + cypressToken},
			{path: "/v2/ping", authHeader: "Bearer " + serviceToken},
			{path: "/v2/ping", authHeader: "ApiKey-v1 " + plaintextKey},
		}

		for _, tc := range cases {
			req, err := http.NewRequest(http.MethodGet, server.URL+tc.path, nil)
			if err != nil {
				t.Fatalf("NewRequest(%s): %v", tc.path, err)
			}
			req.Header.Set("Authorization", tc.authHeader)

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("GET %s: %v", tc.path, err)
			}
			res.Body.Close()
			if res.StatusCode != http.StatusOK {
				t.Fatalf("%s status = %d, want 200", tc.path, res.StatusCode)
			}
		}
	})

	t.Run("cross issuer token on sre route returns 401", func(t *testing.T) {
		server := httptest.NewServer(newRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, authRepo, nil, nil))
		defer server.Close()

		req, err := http.NewRequest(http.MethodGet, server.URL+"/sre-tools/ping", nil)
		if err != nil {
			t.Fatalf("NewRequest(): %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+adminToken)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /sre-tools/ping: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", res.StatusCode)
		}
	})

	t.Run("production cypress guard returns 403", func(t *testing.T) {
		prodCfg := *cfg
		prodCfg.NotifyEnvironment = "production"
		server := httptest.NewServer(newRouter(&prodCfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, authRepo, nil, nil))
		defer server.Close()

		req, err := http.NewRequest(http.MethodGet, server.URL+"/cypress/ping", nil)
		if err != nil {
			t.Fatalf("NewRequest(): %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+cypressToken)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /cypress/ping: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", res.StatusCode)
		}
	})

	t.Run("service auth cache hit still succeeds", func(t *testing.T) {
		store := &serviceAuthTestStore{values: map[string]string{}}
		cache := serviceauth.NewServiceAuthCache(store)
		cache.Set(context.Background(), serviceID, &serviceauth.CachedServiceAuth{Service: authRepo.service, Permissions: authRepo.permissions, APIKeys: authRepo.apiKeys}, time.Minute)
		server := httptest.NewServer(newRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), cache, authRepo, nil, nil))
		defer server.Close()

		req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/ping", nil)
		if err != nil {
			t.Fatalf("NewRequest(): %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+serviceToken)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /v2/ping: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", res.StatusCode)
		}
	})
}

func TestServiceAuthRepositoryDelegates(t *testing.T) {
	serviceID := uuid.New()
	service := servicesRepo.Service{ID: serviceID, Name: "svc"}
	apiKey := apiKeysRepo.ApiKey{ID: uuid.New(), ServiceID: serviceID, Secret: "secret"}
	repo := newServiceAuthRepository(
		&stubServiceQueries{service: service, permissions: []string{"send_emails"}},
		&stubAPIKeyQueries{apiKeys: []apiKeysRepo.ApiKey{apiKey}, apiKey: apiKey},
	)

	gotService, err := repo.GetServiceByIDWithAPIKeys(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("GetServiceByIDWithAPIKeys() error = %v", err)
	}
	if gotService.ID != service.ID {
		t.Fatalf("service id = %v, want %v", gotService.ID, service.ID)
	}

	permissions, err := repo.GetServicePermissions(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("GetServicePermissions() error = %v", err)
	}
	if len(permissions) != 1 || permissions[0] != "send_emails" {
		t.Fatalf("permissions = %v, want [send_emails]", permissions)
	}

	apiKeys, err := repo.GetAPIKeysByServiceID(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("GetAPIKeysByServiceID() error = %v", err)
	}
	if len(apiKeys) != 1 || apiKeys[0].ID != apiKey.ID {
		t.Fatalf("api keys = %v, want [%v]", apiKeys, apiKey.ID)
	}

	gotAPIKey, err := repo.GetAPIKeyBySecret(context.Background(), apiKey.Secret)
	if err != nil {
		t.Fatalf("GetAPIKeyBySecret() error = %v", err)
	}
	if gotAPIKey.ID != apiKey.ID {
		t.Fatalf("api key id = %v, want %v", gotAPIKey.ID, apiKey.ID)
	}
}

func TestRun(t *testing.T) {
	originalLoadConfig := loadAPIConfig
	originalOpenDB := openAPIDB
	originalRunMigrations := runAPIMigrations
	originalNewRedisStore := newRedisStore
	originalNewServer := newAPIServer
	originalNewLogger := newAPILogger
	defer func() {
		loadAPIConfig = originalLoadConfig
		openAPIDB = originalOpenDB
		runAPIMigrations = originalRunMigrations
		newRedisStore = originalNewRedisStore
		newAPIServer = originalNewServer
		newAPILogger = originalNewLogger
	}()

	newAPILogger = func() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
	validCfg := &config.Config{
		DatabaseURI:             "writer",
		DBPoolSize:              1,
		AdminBaseURL:            "http://localhost",
		AttachmentNumLimit:      1,
		AttachmentSizeLimit:     1,
		RateLimitPerSecond:      10,
		RateLimitBurst:          20,
		APIKeyPrefix:            "gcntfy-",
		AdminClientUserName:     "notify-admin",
		AdminClientSecret:       "admin-secret",
		SREUserName:             "notify-sre",
		SREClientSecret:         "sre-secret",
		CacheClearUserName:      "cache-clear",
		CacheClearClientSecret:  "cache-secret",
		CypressAuthUserName:     "cypress",
		CypressAuthClientSecret: "cypress-secret",
		SecretKeys:              []string{"current-secret"},
	}

	t.Run("load config failure", func(t *testing.T) {
		loadAPIConfig = func() (*config.Config, error) { return nil, errors.New("boom") }
		if err := run(); err == nil || !strings.Contains(err.Error(), "load config") {
			t.Fatalf("run() error = %v, want load config failure", err)
		}
	})

	t.Run("open writer db failure", func(t *testing.T) {
		loadAPIConfig = func() (*config.Config, error) { return &config.Config{DatabaseURI: "writer", DBPoolSize: 1}, nil }
		openAPIDB = func(string, int) (*sql.DB, error) { return nil, errors.New("open failed") }
		if err := run(); err == nil || !strings.Contains(err.Error(), "open writer database") {
			t.Fatalf("run() error = %v, want writer db failure", err)
		}
	})

	t.Run("open reader db failure", func(t *testing.T) {
		writerDB, writerMock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer writerDB.Close()
		writerMock.ExpectClose()

		loadAPIConfig = func() (*config.Config, error) {
			return &config.Config{DatabaseURI: "writer", DatabaseReaderURI: "reader", DBPoolSize: 1}, nil
		}
		openCalls := 0
		openAPIDB = func(string, int) (*sql.DB, error) {
			openCalls++
			if openCalls == 1 {
				return writerDB, nil
			}
			return nil, errors.New("reader failed")
		}
		if err := run(); err == nil || !strings.Contains(err.Error(), "open reader database") {
			t.Fatalf("run() error = %v, want reader db failure", err)
		}
		if err := writerMock.ExpectationsWereMet(); err != nil {
			t.Fatalf("writer expectations: %v", err)
		}
	})

	t.Run("redis store failure", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer db.Close()
		mock.ExpectClose()

		loadAPIConfig = func() (*config.Config, error) {
			return &config.Config{DatabaseURI: "writer", DBPoolSize: 1, RedisEnabled: true, CacheOpsURL: "redis://bad"}, nil
		}
		openAPIDB = func(string, int) (*sql.DB, error) { return db, nil }
		newRedisStore = func(string) (serviceauth.RedisStore, error) { return nil, errors.New("redis failed") }
		if err := run(); err == nil || !strings.Contains(err.Error(), "open redis store") {
			t.Fatalf("run() error = %v, want redis store failure", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("db expectations: %v", err)
		}
	})

	t.Run("migration failure", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer db.Close()
		mock.ExpectClose()

		loadAPIConfig = func() (*config.Config, error) { return &config.Config{DatabaseURI: "writer", DBPoolSize: 1}, nil }
		openAPIDB = func(string, int) (*sql.DB, error) { return db, nil }
		newRedisStore = func(string) (serviceauth.RedisStore, error) { return nil, nil }
		runAPIMigrations = func(*sql.DB, string) error { return errors.New("migrate failed") }
		if err := run(); err == nil || !strings.Contains(err.Error(), "run migrations") {
			t.Fatalf("run() error = %v, want migration failure", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("db expectations: %v", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer db.Close()
		mock.ExpectClose()

		loadAPIConfig = func() (*config.Config, error) { return validCfg, nil }
		openAPIDB = func(string, int) (*sql.DB, error) { return db, nil }
		runAPIMigrations = func(*sql.DB, string) error { return nil }
		server := &fakeServer{listenErr: errors.New("serve failed")}
		newAPIServer = func(string, http.Handler) listenServer { return server }
		if err := run(); err == nil || !strings.Contains(err.Error(), "serve http") {
			t.Fatalf("run() error = %v, want server failure", err)
		}
		if !server.called {
			t.Fatal("expected server to be started")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("db expectations: %v", err)
		}
	})

	t.Run("http err server closed treated as success", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer db.Close()
		mock.ExpectClose()

		loadAPIConfig = func() (*config.Config, error) { return validCfg, nil }
		openAPIDB = func(string, int) (*sql.DB, error) { return db, nil }
		runAPIMigrations = func(*sql.DB, string) error { return nil }
		server := &fakeServer{listenErr: http.ErrServerClosed}
		newAPIServer = func(string, http.Handler) listenServer { return server }
		if err := run(); err != nil {
			t.Fatalf("run() error = %v, want nil", err)
		}
		if !server.called {
			t.Fatal("expected server to be started")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("db expectations: %v", err)
		}
	})
}

type serviceAuthTestStore struct {
	values map[string]string
}

func (s *serviceAuthTestStore) Get(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *serviceAuthTestStore) Set(_ context.Context, key string, value string, _ time.Duration) error {
	if s.values == nil {
		s.values = make(map[string]string)
	}
	s.values[key] = value
	return nil
}

func (s *serviceAuthTestStore) Del(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
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

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return encoded
}
