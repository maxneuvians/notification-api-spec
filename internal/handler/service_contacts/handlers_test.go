package service_contacts

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	appmiddleware "github.com/maxneuvians/notification-api-spec/internal/middleware"
	apiKeysRepo "github.com/maxneuvians/notification-api-spec/internal/repository/api_keys"
	servicesRepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	serviceauth "github.com/maxneuvians/notification-api-spec/internal/service/auth"
	serviceerrs "github.com/maxneuvians/notification-api-spec/internal/service/services"
	"github.com/maxneuvians/notification-api-spec/pkg/signing"
)

type stubRepository struct {
	smsSender         *servicesRepo.ServiceSmsSender
	smsSenders        []servicesRepo.ServiceSmsSender
	service           *servicesRepo.Service
	dataRetention     *servicesRepo.ServiceDataRetention
	dataRetentions    []servicesRepo.ServiceDataRetention
	replyTo           *servicesRepo.ServiceEmailReplyTo
	replyTos          []servicesRepo.ServiceEmailReplyTo
	safelist          []servicesRepo.ServiceSafelist
	err               error
	updateSafelistHit bool
	updateInboundHit  bool
}

func (s *stubRepository) GetSmsSenderByID(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*servicesRepo.ServiceSmsSender, error) {
	return s.smsSender, s.err
}
func (s *stubRepository) GetSmsSendersByServiceID(_ context.Context, _ uuid.UUID) ([]servicesRepo.ServiceSmsSender, error) {
	return s.smsSenders, s.err
}
func (s *stubRepository) AddSmsSenderForService(_ context.Context, sender servicesRepo.ServiceSmsSender) (*servicesRepo.ServiceSmsSender, error) {
	s.smsSender = &sender
	return s.smsSender, s.err
}
func (s *stubRepository) UpdateServiceSmsSender(_ context.Context, sender servicesRepo.ServiceSmsSender) (*servicesRepo.ServiceSmsSender, error) {
	s.smsSender = &sender
	return s.smsSender, s.err
}
func (s *stubRepository) UpdateSmsSenderWithInboundNumber(_ context.Context, serviceID, senderID, inboundNumberID uuid.UUID) (*servicesRepo.ServiceSmsSender, error) {
	s.updateInboundHit = true
	result := &servicesRepo.ServiceSmsSender{ID: senderID, ServiceID: serviceID, InboundNumberID: uuid.NullUUID{UUID: inboundNumberID, Valid: true}}
	return result, s.err
}
func (s *stubRepository) ArchiveSmsSender(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*servicesRepo.ServiceSmsSender, error) {
	return s.smsSender, s.err
}
func (s *stubRepository) FetchServiceByID(_ context.Context, _ uuid.UUID, _ bool) (*servicesRepo.Service, error) {
	return s.service, s.err
}
func (s *stubRepository) FetchServiceDataRetention(_ context.Context, _ uuid.UUID) ([]servicesRepo.ServiceDataRetention, error) {
	return s.dataRetentions, s.err
}
func (s *stubRepository) FetchServiceDataRetentionByID(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*servicesRepo.ServiceDataRetention, error) {
	return s.dataRetention, s.err
}
func (s *stubRepository) FetchDataRetentionByNotificationType(_ context.Context, _ uuid.UUID, _ servicesRepo.NotificationType) (*servicesRepo.ServiceDataRetention, error) {
	return s.dataRetention, s.err
}
func (s *stubRepository) InsertServiceDataRetention(_ context.Context, retention servicesRepo.ServiceDataRetention) (*servicesRepo.ServiceDataRetention, error) {
	s.dataRetention = &retention
	return s.dataRetention, s.err
}
func (s *stubRepository) UpdateServiceDataRetention(_ context.Context, _ uuid.UUID, _ uuid.UUID, daysOfRetention int32) (int64, error) {
	if s.dataRetention == nil {
		return 0, s.err
	}
	s.dataRetention.DaysOfRetention = daysOfRetention
	return 1, s.err
}
func (s *stubRepository) FetchServiceSafelist(_ context.Context, _ uuid.UUID) ([]servicesRepo.ServiceSafelist, error) {
	return s.safelist, s.err
}
func (s *stubRepository) AddSafelistedContacts(_ context.Context, _ uuid.UUID, emailAddresses, phoneNumbers []string) error {
	s.updateSafelistHit = true
	s.safelist = nil
	for _, value := range emailAddresses {
		s.safelist = append(s.safelist, servicesRepo.ServiceSafelist{RecipientType: servicesRepo.RecipientTypeEmail, Recipient: value})
	}
	for _, value := range phoneNumbers {
		s.safelist = append(s.safelist, servicesRepo.ServiceSafelist{RecipientType: servicesRepo.RecipientTypeMobile, Recipient: value})
	}
	return s.err
}
func (s *stubRepository) RemoveServiceSafelist(_ context.Context, _ uuid.UUID) error {
	s.safelist = nil
	return s.err
}
func (s *stubRepository) GetReplyTosByServiceID(_ context.Context, _ uuid.UUID) ([]servicesRepo.ServiceEmailReplyTo, error) {
	return s.replyTos, s.err
}
func (s *stubRepository) GetReplyToByID(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*servicesRepo.ServiceEmailReplyTo, error) {
	return s.replyTo, s.err
}
func (s *stubRepository) AddReplyToEmailAddress(_ context.Context, replyTo servicesRepo.ServiceEmailReplyTo) (*servicesRepo.ServiceEmailReplyTo, error) {
	s.replyTo = &replyTo
	return s.replyTo, s.err
}
func (s *stubRepository) UpdateReplyToEmailAddress(_ context.Context, replyTo servicesRepo.ServiceEmailReplyTo) (*servicesRepo.ServiceEmailReplyTo, error) {
	s.replyTo = &replyTo
	return s.replyTo, s.err
}
func (s *stubRepository) ArchiveReplyToEmailAddress(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*servicesRepo.ServiceEmailReplyTo, error) {
	return s.replyTo, s.err
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

func TestServiceContactRoutesRequireAuthorization(t *testing.T) {
	serviceID := uuid.New()
	router := newProtectedRouter(t, serviceID, &stubRepository{})
	server := httptest.NewServer(router)
	defer server.Close()

	paths := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/service/" + serviceID.String() + "/sms-sender"},
		{method: http.MethodPost, path: "/service/" + serviceID.String() + "/sms-sender", body: `{"sms_sender":"Notify","is_default":true}`},
		{method: http.MethodPost, path: "/service/delete_service_sms_sender", body: `{"service_id":"` + serviceID.String() + `","sms_sender_id":"` + uuid.NewString() + `"}`},
		{method: http.MethodGet, path: "/service/" + serviceID.String() + "/data-retention"},
		{method: http.MethodPost, path: "/service/" + serviceID.String() + "/data-retention", body: `{"notification_type":"email","days_of_retention":7}`},
		{method: http.MethodGet, path: "/service/" + serviceID.String() + "/safelist"},
		{method: http.MethodPut, path: "/service/" + serviceID.String() + "/safelist", body: `{"email_addresses":[],"phone_numbers":[]}`},
		{method: http.MethodGet, path: "/service/" + serviceID.String() + "/email-reply-to"},
		{method: http.MethodPost, path: "/service/add_service_reply_to_email_address", body: `{"service_id":"` + serviceID.String() + `","email_address":"reply@example.com","is_default":true}`},
		{method: http.MethodGet, path: "/service/" + serviceID.String() + "/letter-contact"},
	}

	for _, tc := range paths {
		req, err := http.NewRequest(tc.method, server.URL+"/v2"+tc.path, strings.NewReader(tc.body))
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

func TestLetterContactRoutesReturnNotImplemented(t *testing.T) {
	repo := &stubRepository{}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)
	serviceID := uuid.New()
	contactID := uuid.New()

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/" + serviceID.String() + "/letter-contact"},
		{method: http.MethodGet, path: "/" + serviceID.String() + "/letter-contact/" + contactID.String()},
		{method: http.MethodPost, path: "/" + serviceID.String() + "/letter-contact"},
		{method: http.MethodPost, path: "/" + serviceID.String() + "/letter-contact/" + contactID.String()},
		{method: http.MethodPost, path: "/" + serviceID.String() + "/letter-contact/" + contactID.String() + "/archive"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"contact_block":"Example","is_default":true}`))
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)

		if res.Code != http.StatusNotImplemented {
			t.Fatalf("%s %s status = %d, want 501", tc.method, tc.path, res.Code)
		}
	}
}

func TestSMSSenderValidationErrorsReturnStructuredJSON(t *testing.T) {
	serviceID := uuid.New()
	repo := &stubRepository{err: serviceerrs.InvalidRequestError{Message: "You must have at least one SMS sender as the default.", StatusCode: http.StatusBadRequest}}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/"+serviceID.String()+"/sms-sender", strings.NewReader(`{"sms_sender":"Notify","is_default":false}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	if got := res.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if body["result"] != "error" || body["message"] != repo.err.Error() {
		t.Fatalf("body = %v", body)
	}
}

func TestUpdateSMSSenderWithInboundNumberUsesBindingPath(t *testing.T) {
	serviceID := uuid.New()
	senderID := uuid.New()
	inboundID := uuid.New()
	repo := &stubRepository{}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/"+serviceID.String()+"/sms-sender/"+senderID.String(), strings.NewReader(`{"inbound_number_id":"`+inboundID.String()+`"}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if !repo.updateInboundHit {
		t.Fatal("expected inbound binding path to be used")
	}
}

func TestUpdateSMSSenderReturnsStructuredInboundValidationError(t *testing.T) {
	serviceID := uuid.New()
	senderID := uuid.New()
	repo := &stubRepository{err: serviceerrs.InvalidRequestError{Message: "You cannot update an inbound number", StatusCode: http.StatusBadRequest}}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/"+serviceID.String()+"/sms-sender/"+senderID.String(), strings.NewReader(`{"sms_sender":"Changed","is_default":true}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if body["message"] != "You cannot update an inbound number" {
		t.Fatalf("body = %v", body)
	}
}

func TestArchiveSMSSenderReturnsStructuredValidationError(t *testing.T) {
	serviceID := uuid.New()
	senderID := uuid.New()
	repo := &stubRepository{err: serviceerrs.InvalidRequestError{Message: "You cannot delete an inbound number", StatusCode: http.StatusBadRequest}}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/delete_service_sms_sender", strings.NewReader(`{"service_id":"`+serviceID.String()+`","sms_sender_id":"`+senderID.String()+`"}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if body["message"] != "You cannot delete an inbound number" {
		t.Fatalf("body = %v", body)
	}
}

func TestAddReplyToReturnsStructuredValidationError(t *testing.T) {
	serviceID := uuid.New()
	repo := &stubRepository{err: serviceerrs.InvalidRequestError{Message: "You must have at least one reply to email address as the default.", StatusCode: http.StatusBadRequest}}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/add_service_reply_to_email_address", strings.NewReader(`{"service_id":"`+serviceID.String()+`","email_address":"reply@example.com","is_default":false}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if body["message"] != "You must have at least one reply to email address as the default." {
		t.Fatalf("body = %v", body)
	}
}

func TestUpdateReplyToReturnsStructuredValidationError(t *testing.T) {
	serviceID := uuid.New()
	replyToID := uuid.New()
	repo := &stubRepository{err: serviceerrs.InvalidRequestError{Message: "You must have at least one reply to email address as the default.", StatusCode: http.StatusBadRequest}}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/update_service_reply_to_email_address", strings.NewReader(`{"service_id":"`+serviceID.String()+`","reply_to_id":"`+replyToID.String()+`","email_address":"reply@example.com","is_default":false}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if body["message"] != "You must have at least one reply to email address as the default." {
		t.Fatalf("body = %v", body)
	}
}

func TestArchiveReplyToReturnsStructuredValidationError(t *testing.T) {
	serviceID := uuid.New()
	replyToID := uuid.New()
	repo := &stubRepository{err: serviceerrs.InvalidRequestError{Message: "You cannot delete a default email reply to address if other reply to addresses exist", StatusCode: http.StatusBadRequest}}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/delete_service_reply_to_email_address", strings.NewReader(`{"service_id":"`+serviceID.String()+`","reply_to_id":"`+replyToID.String()+`"}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if body["message"] != "You cannot delete a default email reply to address if other reply to addresses exist" {
		t.Fatalf("body = %v", body)
	}
}

func TestArchiveReplyToReturnsUpdatedRecordForSoleDefault(t *testing.T) {
	serviceID := uuid.New()
	replyToID := uuid.New()
	repo := &stubRepository{replyTo: &servicesRepo.ServiceEmailReplyTo{ID: replyToID, ServiceID: serviceID, EmailAddress: "reply@example.com", Archived: true, IsDefault: false}}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/delete_service_reply_to_email_address", strings.NewReader(`{"service_id":"`+serviceID.String()+`","reply_to_id":"`+replyToID.String()+`"}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, ok := body["data"]; !ok {
		t.Fatalf("body = %s, want data envelope", res.Body.String())
	}
}

func TestDataRetentionHandlersValidateAndReturnEmptyObjectOnMiss(t *testing.T) {
	serviceID := uuid.New()
	retentionID := uuid.New()
	repo := &stubRepository{}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	invalidReq := httptest.NewRequest(http.MethodPost, "/"+serviceID.String()+"/data-retention", strings.NewReader(`{"notification_type":"fax","days_of_retention":7}`))
	invalidRes := httptest.NewRecorder()
	r.ServeHTTP(invalidRes, invalidReq)
	if invalidRes.Code != http.StatusBadRequest {
		t.Fatalf("invalid create status = %d, want 400", invalidRes.Code)
	}
	var invalidBody map[string]string
	if err := json.Unmarshal(invalidRes.Body.Bytes(), &invalidBody); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if invalidBody["message"] != "notification_type fax is not one of [sms, letter, email]" {
		t.Fatalf("body = %v", invalidBody)
	}

	missReq := httptest.NewRequest(http.MethodGet, "/"+serviceID.String()+"/data-retention/"+retentionID.String(), nil)
	missRes := httptest.NewRecorder()
	r.ServeHTTP(missRes, missReq)
	if missRes.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", missRes.Code)
	}
	if strings.TrimSpace(missRes.Body.String()) != "{}" {
		t.Fatalf("body = %q, want {}", missRes.Body.String())
	}
}

func TestUpdateDataRetentionReturnsNotFoundWhenRowMissing(t *testing.T) {
	serviceID := uuid.New()
	retentionID := uuid.New()
	repo := &stubRepository{}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/"+serviceID.String()+"/data-retention/"+retentionID.String(), strings.NewReader(`{"days_of_retention":14}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.Code)
	}
}

func TestSafelistHandlersReturnEmptyArraysAndRejectInvalidEntries(t *testing.T) {
	serviceID := uuid.New()
	repo := &stubRepository{service: &servicesRepo.Service{ID: serviceID, Name: "service"}}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	getReq := httptest.NewRequest(http.MethodGet, "/"+serviceID.String()+"/safelist", nil)
	getRes := httptest.NewRecorder()
	r.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", getRes.Code)
	}
	var safelistBody map[string][]string
	if err := json.Unmarshal(getRes.Body.Bytes(), &safelistBody); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(safelistBody["email_addresses"]) != 0 || len(safelistBody["phone_numbers"]) != 0 {
		t.Fatalf("body = %v", safelistBody)
	}

	invalidReq := httptest.NewRequest(http.MethodPut, "/"+serviceID.String()+"/safelist", strings.NewReader(`{"email_addresses":[""],"phone_numbers":[]}`))
	invalidRes := httptest.NewRecorder()
	r.ServeHTTP(invalidRes, invalidReq)
	if invalidRes.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", invalidRes.Code)
	}
	if repo.updateSafelistHit {
		t.Fatal("expected invalid safelist request to skip repository update")
	}
	var invalidBody map[string]string
	if err := json.Unmarshal(invalidRes.Body.Bytes(), &invalidBody); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if invalidBody["message"] != "Invalid safelist: \"\" is not a valid email address or phone number" {
		t.Fatalf("body = %v", invalidBody)
	}
}

func TestSafelistGetReturnsNotFoundForUnknownService(t *testing.T) {
	serviceID := uuid.New()
	repo := &stubRepository{}
	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/"+serviceID.String()+"/safelist", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.Code)
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
	r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	contacts := NewHandler(repo)
	r.Route("/service", func(r chi.Router) {
		contacts.RegisterRoutes(r)
	})

	outer := chi.NewRouter()
	outer.Mount("/v2", r)

	_ = plaintextKey
	_ = serviceauth.CachedServiceAuth{}
	return outer
}
