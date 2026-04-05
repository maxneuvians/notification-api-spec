package providers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	providersrepo "github.com/maxneuvians/notification-api-spec/internal/repository/providers"
)

type stubReader struct {
	stats    []providersrepo.ProviderDetailStat
	stat     providersrepo.ProviderDetailStat
	versions []providersrepo.ProviderDetailsHistory
	err      error
}

func (s *stubReader) GetDaoProviderStats(_ context.Context) ([]providersrepo.ProviderDetailStat, error) {
	return s.stats, s.err
}

func (s *stubReader) GetProviderStatByID(_ context.Context, id uuid.UUID) (providersrepo.ProviderDetailStat, error) {
	if s.err != nil {
		return providersrepo.ProviderDetailStat{}, s.err
	}
	if s.stat.ID != id {
		return providersrepo.ProviderDetailStat{}, sql.ErrNoRows
	}
	return s.stat, nil
}

func (s *stubReader) GetProviderVersions(_ context.Context, _ uuid.UUID) ([]providersrepo.ProviderDetailsHistory, error) {
	return s.versions, s.err
}

type stubWriter struct {
	params providersrepo.UpdateProviderDetailsParams
	err    error
}

func (s *stubWriter) UpdateProviderDetails(_ context.Context, arg providersrepo.UpdateProviderDetailsParams) (providersrepo.ProviderDetail, error) {
	s.params = arg
	if s.err != nil {
		return providersrepo.ProviderDetail{}, s.err
	}
	return providersrepo.ProviderDetail{ID: arg.ID}, nil
}

func TestGetProviders(t *testing.T) {
	reader := &stubReader{stats: []providersrepo.ProviderDetailStat{{
		ProviderDetail:          providersrepo.ProviderDetail{ID: uuid.New(), DisplayName: "SES", Identifier: "ses", Priority: 5, NotificationType: providersrepo.NotificationTypeEmail, Active: true},
		CurrentMonthBillableSMS: 0,
	}, {
		ProviderDetail:          providersrepo.ProviderDetail{ID: uuid.New(), DisplayName: "SNS", Identifier: "sns", Priority: 10, NotificationType: providersrepo.NotificationTypeSms, Active: true},
		CurrentMonthBillableSMS: 42,
	}}}
	h := NewHandler(reader, &stubWriter{})
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}

	var body map[string][]map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(body["provider_details"]) != 2 {
		t.Fatalf("provider_details len = %d, want 2", len(body["provider_details"]))
	}
	if got := body["provider_details"][1]["current_month_billable_sms"]; got.(float64) != 42 {
		t.Fatalf("current_month_billable_sms = %v, want 42", got)
	}
}

func TestGetProviderByID(t *testing.T) {
	id := uuid.New()
	reader := &stubReader{stat: providersrepo.ProviderDetailStat{ProviderDetail: providersrepo.ProviderDetail{ID: id, DisplayName: "SNS", Identifier: "sns", Priority: 10, NotificationType: providersrepo.NotificationTypeSms, Active: true}}}
	h := NewHandler(reader, &stubWriter{})
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/"+id.String(), nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
}

func TestUpdateProviderRejectsDisallowedFields(t *testing.T) {
	h := NewHandler(&stubReader{}, &stubWriter{})
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	id := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/"+id.String(), bytes.NewBufferString(`{"identifier":"x"}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	if got := res.Body.String(); got == "" || !bytes.Contains(res.Body.Bytes(), []byte("Not permitted to be updated")) {
		t.Fatalf("body = %s, want validation error", got)
	}
}

func TestUpdateProviderAcceptsAllowedFields(t *testing.T) {
	id := uuid.New()
	updatedAt := time.Now().UTC().Truncate(time.Second)
	reader := &stubReader{stat: providersrepo.ProviderDetailStat{ProviderDetail: providersrepo.ProviderDetail{ID: id, DisplayName: "SNS", Identifier: "sns", Priority: 20, NotificationType: providersrepo.NotificationTypeSms, Active: false, UpdatedAt: sql.NullTime{Time: updatedAt, Valid: true}}}}
	writer := &stubWriter{}
	h := NewHandler(reader, writer)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	userID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/"+id.String(), bytes.NewBufferString(`{"priority":20,"active":false,"created_by":"`+userID.String()+`"}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if writer.params.Priority == nil || *writer.params.Priority != 20 {
		t.Fatalf("priority params = %#v, want 20", writer.params.Priority)
	}
	if writer.params.Active == nil || *writer.params.Active {
		t.Fatalf("active params = %#v, want false", writer.params.Active)
	}
	if writer.params.CreatedByID == nil || *writer.params.CreatedByID != userID {
		t.Fatalf("created_by params = %#v, want %v", writer.params.CreatedByID, userID)
	}
}

func TestGetProviderVersionsOmitsCurrentMonthBillableSMS(t *testing.T) {
	id := uuid.New()
	reader := &stubReader{versions: []providersrepo.ProviderDetailsHistory{{ID: id, DisplayName: "SNS", Identifier: "sns", Priority: 10, NotificationType: providersrepo.NotificationTypeSms, Active: true, Version: 1}}}
	h := NewHandler(reader, &stubWriter{})
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/"+id.String()+"/versions", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if bytes.Contains(res.Body.Bytes(), []byte("current_month_billable_sms")) {
		t.Fatalf("body = %s, should not contain current_month_billable_sms", res.Body.String())
	}
}

func TestGetProviderByIDNotFound(t *testing.T) {
	h := NewHandler(&stubReader{err: errors.New("boom")}, &stubWriter{})
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/"+uuid.New().String(), nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", res.Code)
	}
}
