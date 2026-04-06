package service_callback

import (
	"context"
	"database/sql"
	"encoding/json"
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
	"github.com/maxneuvians/notification-api-spec/pkg/signing"
)

type stubRepository struct {
	callback *servicesRepo.ServiceCallbackApi
	inbound  *servicesRepo.ServiceInboundApi
	deleted  bool
	err      error
	lastURL  *string
}

func (s *stubRepository) SaveServiceCallbackApi(_ context.Context, _ uuid.UUID, _ string, _ string, _ string, _ uuid.UUID) (*servicesRepo.ServiceCallbackApi, error) {
	return s.callback, s.err
}
func (s *stubRepository) ResetServiceCallbackApi(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ uuid.UUID, url *string, _ *string) (*servicesRepo.ServiceCallbackApi, error) {
	s.lastURL = url
	return s.callback, s.err
}
func (s *stubRepository) GetServiceCallbackApi(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*servicesRepo.ServiceCallbackApi, error) {
	return s.callback, s.err
}
func (s *stubRepository) DeleteServiceCallbackApi(_ context.Context, _ uuid.UUID, _ uuid.UUID) (bool, error) {
	return s.deleted, s.err
}
func (s *stubRepository) SuspendUnsuspendCallbackApi(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ bool) (*servicesRepo.ServiceCallbackApi, error) {
	return s.callback, s.err
}
func (s *stubRepository) SaveServiceInboundApi(_ context.Context, _ uuid.UUID, _ string, _ string, _ uuid.UUID) (*servicesRepo.ServiceInboundApi, error) {
	return s.inbound, s.err
}
func (s *stubRepository) ResetServiceInboundApi(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ uuid.UUID, url *string, _ *string) (*servicesRepo.ServiceInboundApi, error) {
	s.lastURL = url
	return s.inbound, s.err
}
func (s *stubRepository) GetServiceInboundApi(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*servicesRepo.ServiceInboundApi, error) {
	return s.inbound, s.err
}
func (s *stubRepository) DeleteServiceInboundApi(_ context.Context, _ uuid.UUID, _ uuid.UUID) (bool, error) {
	return s.deleted, s.err
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

func TestCallbackRoutesRequireAuthorization(t *testing.T) {
	serviceID := uuid.New()
	server := httptest.NewServer(newProtectedRouter(t, serviceID, &stubRepository{}))
	defer server.Close()

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/v2/service/" + serviceID.String() + "/delivery-receipt-api", body: `{"url":"https://callback.example.com","bearer_token":"0123456789","updated_by_id":"` + uuid.NewString() + `"}`},
		{method: http.MethodPost, path: "/v2/service/" + serviceID.String() + "/inbound-api", body: `{"url":"https://inbound.example.com","bearer_token":"0123456789","updated_by_id":"` + uuid.NewString() + `"}`},
	} {
		req, err := http.NewRequest(tc.method, server.URL+tc.path, strings.NewReader(tc.body))
		if err != nil {
			t.Fatalf("NewRequest(%s) error = %v", tc.path, err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Do(%s) error = %v", tc.path, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", tc.path, res.StatusCode)
		}
	}
}

func TestValidationErrorsReturnStructuredJSON(t *testing.T) {
	repo := &stubRepository{}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)
	serviceID := uuid.New()
	updatedByID := uuid.New()

	for _, tc := range []struct {
		name string
		path string
		body string
	}{
		{name: "invalid url", path: "/" + serviceID.String() + "/delivery-receipt-api", body: `{"url":"http://bad.example.com","bearer_token":"0123456789","updated_by_id":"` + updatedByID.String() + `"}`},
		{name: "short bearer", path: "/" + serviceID.String() + "/inbound-api", body: `{"url":"https://good.example.com","bearer_token":"short","updated_by_id":"` + updatedByID.String() + `"}`},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)

		if res.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", tc.name, res.Code)
		}
		var body map[string]string
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s decode error = %v", tc.name, err)
		}
		if body["result"] != "error" || body["message"] == "" {
			t.Fatalf("%s body = %v", tc.name, body)
		}
	}
}

func TestUpdateInboundAPIAllowsExplicitEmptyURL(t *testing.T) {
	serviceID := uuid.New()
	apiID := uuid.New()
	repo := &stubRepository{inbound: &servicesRepo.ServiceInboundApi{ID: apiID, ServiceID: serviceID, Url: "", CreatedAt: time.Now().UTC()}}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/"+serviceID.String()+"/inbound-api/"+apiID.String(), strings.NewReader(`{"url":"","updated_by_id":"`+uuid.NewString()+`"}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if repo.lastURL == nil || *repo.lastURL != "" {
		t.Fatalf("lastURL = %#v, want explicit empty string", repo.lastURL)
	}
}

func newProtectedRouter(t *testing.T, serviceID uuid.UUID, repo Repository) http.Handler {
	t.Helper()
	cfg := config.Config{
		AdminBaseURL:        "https://admin.example.com",
		AttachmentNumLimit:  1,
		AttachmentSizeLimit: 1024,
		RateLimitPerSecond:  10,
		RateLimitBurst:      20,
		APIKeyPrefix:        "gcntfy-",
		SecretKeys:          []string{"current-secret"},
	}
	plaintextToken := uuid.New().String()
	plaintextKey := cfg.APIKeyPrefix + serviceID.String() + plaintextToken
	apiKeySecret, err := signing.SignAPIKeyToken(plaintextToken, cfg.SecretKeys[0])
	if err != nil {
		t.Fatalf("SignAPIKeyToken() error = %v", err)
	}
	apiKey := apiKeysRepo.ApiKey{ID: uuid.New(), ServiceID: serviceID, Secret: apiKeySecret, KeyType: "normal"}
	authRepo := &authRepoStub{
		service:      servicesRepo.Service{ID: serviceID, Name: "service", Active: true},
		permissions:  []string{"manage_settings"},
		apiKeys:      []apiKeysRepo.ApiKey{apiKey},
		secretLookup: map[string]apiKeysRepo.ApiKey{apiKeySecret: apiKey},
	}

	r := chi.NewRouter()
	r.Use(appmiddleware.RequireAuth(cfg, nil, authRepo))
	r.Route("/service", func(r chi.Router) {
		NewHandler(repo).RegisterRoutes(r)
	})
	outer := chi.NewRouter()
	outer.Mount("/v2", r)
	_ = plaintextKey
	return outer
}
