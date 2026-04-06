package api_key

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	appmiddleware "github.com/maxneuvians/notification-api-spec/internal/middleware"
	apiKeysRepo "github.com/maxneuvians/notification-api-spec/internal/repository/api_keys"
	servicesRepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	serviceauth "github.com/maxneuvians/notification-api-spec/internal/service/auth"
	"github.com/maxneuvians/notification-api-spec/pkg/signing"
)

type stubRepository struct {
	created *servicesRepo.CreatedAPIKey
	items   []apiKeysRepo.ApiKey
	revoked *apiKeysRepo.ApiKey
	err     error
}

func (s *stubRepository) CreateAPIKey(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID, _ string) (*servicesRepo.CreatedAPIKey, error) {
	return s.created, s.err
}

func (s *stubRepository) ListAPIKeys(_ context.Context, _ uuid.UUID, _ *uuid.UUID) ([]apiKeysRepo.ApiKey, error) {
	return s.items, s.err
}

func (s *stubRepository) RevokeAPIKey(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*apiKeysRepo.ApiKey, error) {
	return s.revoked, s.err
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
	item, ok := s.secretLookup[secret]
	if !ok {
		return apiKeysRepo.ApiKey{}, sql.ErrNoRows
	}
	return item, nil
}

func TestAPIKeyRoutesRequireAuthorization(t *testing.T) {
	serviceID := uuid.New()
	router := newProtectedRouter(t, serviceID)
	server := httptest.NewServer(router)
	defer server.Close()

	paths := []string{
		"/service/" + serviceID.String() + "/api-key",
		"/service/" + serviceID.String() + "/api-keys",
		"/service/" + serviceID.String() + "/api-key/" + uuid.NewString() + "/revoke",
	}

	for _, path := range paths {
		method := http.MethodGet
		body := io.Reader(nil)
		if strings.Contains(path, "/api-key") && !strings.Contains(path, "/api-keys") {
			method = http.MethodPost
			body = strings.NewReader(`{"name":"primary","created_by":"` + uuid.NewString() + `","key_type":"normal"}`)
		}
		if strings.HasSuffix(path, "/revoke") {
			method = http.MethodPost
			body = nil
		}

		req, err := http.NewRequest(method, server.URL+"/v2"+path, body)
		if err != nil {
			t.Fatalf("NewRequest(%s) error = %v", path, err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Do(%s) error = %v", path, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", path, res.StatusCode)
		}
	}
}

func TestListAPIKeysDoesNotExposeStoredSecret(t *testing.T) {
	serviceID := uuid.New()
	secret := "stored-secret"
	handler := NewHandler(&stubRepository{items: []apiKeysRepo.ApiKey{{
		ID:        uuid.New(),
		Name:      "primary",
		Secret:    secret,
		ServiceID: serviceID,
		CreatedAt: time.Now().UTC(),
	}}})

	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/"+serviceID.String()+"/api-keys", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("response leaked stored secret: %s", rec.Body.String())
	}
}

func TestCreateAndRevokeAPIKeyHandlers(t *testing.T) {
	serviceID := uuid.New()
	createdByID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)
	created := &servicesRepo.CreatedAPIKey{
		APIKey: apiKeysRepo.ApiKey{
			ID:        uuid.New(),
			Name:      "primary",
			ServiceID: serviceID,
			CreatedAt: createdAt,
		},
		Key: "gcntfy-" + serviceID.String() + uuid.NewString(),
	}
	revoked := &apiKeysRepo.ApiKey{
		ID:         created.APIKey.ID,
		Name:       created.APIKey.Name,
		ServiceID:  serviceID,
		CreatedAt:  createdAt,
		ExpiryDate: sql.NullTime{Time: createdAt.Add(time.Minute), Valid: true},
	}
	handler := NewHandler(&stubRepository{created: created, revoked: revoked})

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	createReq := httptest.NewRequest(http.MethodPost, "/"+serviceID.String()+"/api-key", strings.NewReader(`{"name":"primary","created_by":"`+createdByID.String()+`","key_type":"normal"}`))
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", createRec.Code)
	}
	var createBody map[string]map[string]string
	if err := json.Unmarshal(createRec.Body.Bytes(), &createBody); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}
	if createBody["data"]["key"] == "" || createBody["data"]["key_name"] != "primary" {
		t.Fatalf("create body = %#v, want plaintext key", createBody)
	}

	revokeReq := httptest.NewRequest(http.MethodPost, "/"+serviceID.String()+"/api-key/"+created.APIKey.ID.String()+"/revoke", nil)
	revokeRec := httptest.NewRecorder()
	r.ServeHTTP(revokeRec, revokeReq)
	if revokeRec.Code != http.StatusAccepted {
		t.Fatalf("revoke status = %d, want 202", revokeRec.Code)
	}
	if revokeRec.Body.Len() != 0 {
		t.Fatalf("revoke body = %q, want empty body", revokeRec.Body.String())
	}
}

func newProtectedRouter(t *testing.T, serviceID uuid.UUID) http.Handler {
	t.Helper()
	cfg := &config.Config{
		AdminBaseURL:        "https://admin.example.com",
		AttachmentNumLimit:  1,
		AttachmentSizeLimit: 1024,
		RateLimitPerSecond:  10,
		RateLimitBurst:      20,
		APIKeyPrefix:        "gcntfy-",
		SecretKeys:          []string{"current-secret"},
	}
	plaintextToken := uuid.NewString()
	secret, err := signing.SignAPIKeyToken(plaintextToken, cfg.SecretKeys[0])
	if err != nil {
		t.Fatalf("SignAPIKeyToken() error = %v", err)
	}
	apiKey := apiKeysRepo.ApiKey{ID: uuid.New(), ServiceID: serviceID, Secret: secret, KeyType: "normal"}
	authRepo := &authRepoStub{
		service:      servicesRepo.Service{ID: serviceID, Name: "service", Active: true},
		permissions:  []string{"manage_api_keys"},
		apiKeys:      []apiKeysRepo.ApiKey{apiKey},
		secretLookup: map[string]apiKeysRepo.ApiKey{secret: apiKey},
	}

	r := chi.NewRouter()
	r.Use(appmiddleware.RequireAuth(*cfg, (*serviceauth.ServiceAuthCache)(nil), authRepo))
	NewHandler(&stubRepository{}).RegisterRoutes(r)
	outer := chi.NewRouter()
	outer.Mount("/v2/service", r)
	return outer
}
