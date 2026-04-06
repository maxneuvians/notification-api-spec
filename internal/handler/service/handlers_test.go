package service

import (
	"context"
	"database/sql"
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
	organisationsRepo "github.com/maxneuvians/notification-api-spec/internal/repository/organisations"
	servicesRepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	usersRepo "github.com/maxneuvians/notification-api-spec/internal/repository/users"
	serviceauth "github.com/maxneuvians/notification-api-spec/internal/service/auth"
	"github.com/maxneuvians/notification-api-spec/pkg/signing"
)

type stubRepository struct {
	fetchAllServicesFn       func(context.Context, bool) ([]servicesRepo.Service, error)
	fetchServicesByUserIDFn  func(context.Context, uuid.UUID, bool) ([]servicesRepo.Service, error)
	createServiceFn          func(context.Context, servicesRepo.Service, *uuid.UUID, []string) (*servicesRepo.Service, error)
	fetchUserByIDFn          func(context.Context, uuid.UUID) (*usersRepo.User, error)
	fetchOrgByEmailFn        func(context.Context, string) (*organisationsRepo.Organisation, error)
	fetchNHSEmailBrandingFn  func(context.Context) (*uuid.UUID, error)
	fetchNHSLetterBrandingFn func(context.Context) (*uuid.UUID, error)
	assignBrandingFn         func(context.Context, uuid.UUID, *uuid.UUID, *uuid.UUID) error
	findByNameFn             func(context.Context, string) ([]servicesRepo.Service, error)
	fetchStatsForAllFn       func(context.Context, bool, bool, time.Time, time.Time) ([]servicesRepo.TodayStatsForAllServicesRow, error)
	fetchServiceByIDFn       func(context.Context, uuid.UUID, bool) (*servicesRepo.Service, error)
	fetchServiceHistoryFn    func(context.Context, uuid.UUID) ([]servicesRepo.ServicesHistory, error)
	fetchAPIKeyHistoryFn     func(context.Context, uuid.UUID) ([]apiKeysRepo.APIKeyHistory, error)
	isServiceNameUniqueFn    func(context.Context, string, uuid.UUID) (bool, error)
	isEmailFromUniqueFn      func(context.Context, string, uuid.UUID) (bool, error)
	fetchServiceOrgFn        func(context.Context, uuid.UUID) (*organisationsRepo.Organisation, error)
	fetchStatsFn             func(context.Context, uuid.UUID, int) ([]servicesRepo.ServiceStatsRow, error)
	fetchTodaysStatsFn       func(context.Context, uuid.UUID) ([]servicesRepo.ServiceStatsRow, error)
	fetchLiveServicesDataFn  func(context.Context) ([]servicesRepo.GetLiveServicesDataRow, error)
	fetchSensitiveIDsFn      func(context.Context) ([]uuid.UUID, error)
	fetchAnnualLimitStatsFn  func(context.Context, uuid.UUID) (*servicesRepo.AnnualLimitStats, error)
	fetchMonthlyUsageFn      func(context.Context, uuid.UUID, int) ([]servicesRepo.MonthlyUsageRow, error)
	suspendFn                func(context.Context, uuid.UUID, *uuid.UUID) (*servicesRepo.Service, error)
	resumeFn                 func(context.Context, uuid.UUID) (*servicesRepo.Service, error)
	archiveFn                func(context.Context, uuid.UUID) (*servicesRepo.Service, error)
}

func (s *stubRepository) FetchAllServices(ctx context.Context, onlyActive bool) ([]servicesRepo.Service, error) {
	if s.fetchAllServicesFn == nil {
		return nil, nil
	}
	return s.fetchAllServicesFn(ctx, onlyActive)
}

func (s *stubRepository) FetchServicesByUserID(ctx context.Context, userID uuid.UUID, onlyActive bool) ([]servicesRepo.Service, error) {
	if s.fetchServicesByUserIDFn == nil {
		return nil, nil
	}
	return s.fetchServicesByUserIDFn(ctx, userID, onlyActive)
}

func (s *stubRepository) CreateService(ctx context.Context, service servicesRepo.Service, userID *uuid.UUID, permissions []string) (*servicesRepo.Service, error) {
	if s.createServiceFn == nil {
		return nil, nil
	}
	return s.createServiceFn(ctx, service, userID, permissions)
}

func (s *stubRepository) FetchUserByID(ctx context.Context, userID uuid.UUID) (*usersRepo.User, error) {
	if s.fetchUserByIDFn == nil {
		return nil, nil
	}
	return s.fetchUserByIDFn(ctx, userID)
}

func (s *stubRepository) FetchOrganisationByEmailAddress(ctx context.Context, emailAddress string) (*organisationsRepo.Organisation, error) {
	if s.fetchOrgByEmailFn == nil {
		return nil, nil
	}
	return s.fetchOrgByEmailFn(ctx, emailAddress)
}

func (s *stubRepository) FetchNHSEmailBrandingID(ctx context.Context) (*uuid.UUID, error) {
	if s.fetchNHSEmailBrandingFn == nil {
		return nil, nil
	}
	return s.fetchNHSEmailBrandingFn(ctx)
}

func (s *stubRepository) FetchNHSLetterBrandingID(ctx context.Context) (*uuid.UUID, error) {
	if s.fetchNHSLetterBrandingFn == nil {
		return nil, nil
	}
	return s.fetchNHSLetterBrandingFn(ctx)
}

func (s *stubRepository) AssignServiceBranding(ctx context.Context, serviceID uuid.UUID, emailBrandingID *uuid.UUID, letterBrandingID *uuid.UUID) error {
	if s.assignBrandingFn == nil {
		return nil
	}
	return s.assignBrandingFn(ctx, serviceID, emailBrandingID, letterBrandingID)
}

func (s *stubRepository) GetServicesByPartialName(ctx context.Context, name string) ([]servicesRepo.Service, error) {
	if s.findByNameFn == nil {
		return nil, nil
	}
	return s.findByNameFn(ctx, name)
}

func (s *stubRepository) FetchStatsForAllServices(ctx context.Context, includeFromTestKey bool, onlyActive bool, startDate time.Time, endDate time.Time) ([]servicesRepo.TodayStatsForAllServicesRow, error) {
	if s.fetchStatsForAllFn == nil {
		return nil, nil
	}
	return s.fetchStatsForAllFn(ctx, includeFromTestKey, onlyActive, startDate, endDate)
}

func (s *stubRepository) FetchServiceByID(ctx context.Context, id uuid.UUID, onlyActive bool) (*servicesRepo.Service, error) {
	if s.fetchServiceByIDFn == nil {
		return nil, nil
	}
	return s.fetchServiceByIDFn(ctx, id, onlyActive)
}

func (s *stubRepository) FetchServiceHistory(ctx context.Context, serviceID uuid.UUID) ([]servicesRepo.ServicesHistory, error) {
	if s.fetchServiceHistoryFn == nil {
		return nil, nil
	}
	return s.fetchServiceHistoryFn(ctx, serviceID)
}

func (s *stubRepository) FetchAPIKeyHistory(ctx context.Context, serviceID uuid.UUID) ([]apiKeysRepo.APIKeyHistory, error) {
	if s.fetchAPIKeyHistoryFn == nil {
		return nil, nil
	}
	return s.fetchAPIKeyHistoryFn(ctx, serviceID)
}

func (s *stubRepository) IsServiceNameUnique(ctx context.Context, name string, serviceID uuid.UUID) (bool, error) {
	if s.isServiceNameUniqueFn == nil {
		return false, nil
	}
	return s.isServiceNameUniqueFn(ctx, name, serviceID)
}

func (s *stubRepository) IsServiceEmailFromUnique(ctx context.Context, emailFrom string, serviceID uuid.UUID) (bool, error) {
	if s.isEmailFromUniqueFn == nil {
		return false, nil
	}
	return s.isEmailFromUniqueFn(ctx, emailFrom, serviceID)
}

func (s *stubRepository) FetchServiceOrganisation(ctx context.Context, serviceID uuid.UUID) (*organisationsRepo.Organisation, error) {
	if s.fetchServiceOrgFn == nil {
		return nil, nil
	}
	return s.fetchServiceOrgFn(ctx, serviceID)
}

func (s *stubRepository) FetchStatsForService(ctx context.Context, serviceID uuid.UUID, limitDays int) ([]servicesRepo.ServiceStatsRow, error) {
	if s.fetchStatsFn == nil {
		return nil, nil
	}
	return s.fetchStatsFn(ctx, serviceID, limitDays)
}

func (s *stubRepository) FetchTodaysStatsForService(ctx context.Context, serviceID uuid.UUID) ([]servicesRepo.ServiceStatsRow, error) {
	if s.fetchTodaysStatsFn == nil {
		return nil, nil
	}
	return s.fetchTodaysStatsFn(ctx, serviceID)
}

func (s *stubRepository) FetchLiveServicesData(ctx context.Context) ([]servicesRepo.GetLiveServicesDataRow, error) {
	if s.fetchLiveServicesDataFn == nil {
		return nil, nil
	}
	return s.fetchLiveServicesDataFn(ctx)
}

func (s *stubRepository) FetchSensitiveServiceIDs(ctx context.Context) ([]uuid.UUID, error) {
	if s.fetchSensitiveIDsFn == nil {
		return nil, nil
	}
	return s.fetchSensitiveIDsFn(ctx)
}

func (s *stubRepository) FetchAnnualLimitStats(ctx context.Context, serviceID uuid.UUID) (*servicesRepo.AnnualLimitStats, error) {
	if s.fetchAnnualLimitStatsFn == nil {
		return &servicesRepo.AnnualLimitStats{}, nil
	}
	return s.fetchAnnualLimitStatsFn(ctx, serviceID)
}

func (s *stubRepository) FetchMonthlyUsageForService(ctx context.Context, serviceID uuid.UUID, fiscalYearStart int) ([]servicesRepo.MonthlyUsageRow, error) {
	if s.fetchMonthlyUsageFn == nil {
		return nil, nil
	}
	return s.fetchMonthlyUsageFn(ctx, serviceID, fiscalYearStart)
}

func (s *stubRepository) SuspendService(ctx context.Context, id uuid.UUID, userID *uuid.UUID) (*servicesRepo.Service, error) {
	if s.suspendFn == nil {
		return nil, nil
	}
	return s.suspendFn(ctx, id, userID)
}

func (s *stubRepository) ResumeService(ctx context.Context, id uuid.UUID) (*servicesRepo.Service, error) {
	if s.resumeFn == nil {
		return nil, nil
	}
	return s.resumeFn(ctx, id)
}

func (s *stubRepository) ArchiveService(ctx context.Context, id uuid.UUID) (*servicesRepo.Service, error) {
	if s.archiveFn == nil {
		return nil, nil
	}
	return s.archiveFn(ctx, id)
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

func TestServiceLifecycleRoutesRequireAuthorization(t *testing.T) {
	serviceID := uuid.New()
	router := newProtectedRouter(t, serviceID)
	server := httptest.NewServer(router)
	defer server.Close()

	paths := []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/service"},
		{method: http.MethodGet, path: "/service"},
		{method: http.MethodGet, path: "/service/find-by-name?service_name=Alpha"},
		{method: http.MethodGet, path: "/service/live-services"},
		{method: http.MethodGet, path: "/service/sensitive-service-ids"},
		{method: http.MethodGet, path: "/service/is-name-unique?name=Alpha&service_id=" + serviceID.String()},
		{method: http.MethodGet, path: "/service/is-email-from-unique?email_from=alpha@example.com&service_id=" + serviceID.String()},
		{method: http.MethodGet, path: "/service/" + serviceID.String() + "/history"},
		{method: http.MethodGet, path: "/service/" + serviceID.String() + "/organisation"},
		{method: http.MethodGet, path: "/service/" + serviceID.String() + "/statistics"},
		{method: http.MethodGet, path: "/service/" + serviceID.String() + "/monthly-usage?year=2026"},
		{method: http.MethodGet, path: "/service/" + serviceID.String() + "/annual-limit-stats"},
		{method: http.MethodPost, path: "/service/" + serviceID.String() + "/archive"},
		{method: http.MethodPost, path: "/service/" + serviceID.String() + "/suspend"},
		{method: http.MethodPost, path: "/service/" + serviceID.String() + "/resume"},
	}

	for _, item := range paths {
		req, err := http.NewRequest(item.method, server.URL+"/v2"+item.path, nil)
		if err != nil {
			t.Fatalf("NewRequest(%s %s) error = %v", item.method, item.path, err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Do(%s %s) error = %v", item.method, item.path, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want 401", item.method, item.path, res.StatusCode)
		}
	}
}

func TestMonthlyUsageHandlerValidatesYearAndUnknownService(t *testing.T) {
	serviceID := uuid.New()
	r := chi.NewRouter()
	NewHandler(&stubRepository{
		fetchServiceByIDFn: func(context.Context, uuid.UUID, bool) (*servicesRepo.Service, error) { return nil, nil },
	}).RegisterRoutes(r)

	for _, path := range []string{"/" + serviceID.String() + "/monthly-usage", "/" + serviceID.String() + "/monthly-usage?year=bad"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", path, rec.Code)
		}
	}

	unknownReq := httptest.NewRequest(http.MethodGet, "/"+serviceID.String()+"/monthly-usage?year=2026", nil)
	unknownRec := httptest.NewRecorder()
	r.ServeHTTP(unknownRec, unknownReq)
	if unknownRec.Code != http.StatusNotFound {
		t.Fatalf("unknown service status = %d, want 404", unknownRec.Code)
	}
}

func TestMonthlyUsageHandlerReturnsFiscalYearMap(t *testing.T) {
	serviceID := uuid.New()
	repo := &stubRepository{
		fetchServiceByIDFn: func(_ context.Context, id uuid.UUID, onlyActive bool) (*servicesRepo.Service, error) {
			if id != serviceID || onlyActive {
				t.Fatalf("FetchServiceByID(%v,%v)", id, onlyActive)
			}
			return &servicesRepo.Service{ID: serviceID, Name: "Alpha"}, nil
		},
		fetchMonthlyUsageFn: func(_ context.Context, id uuid.UUID, fiscalYearStart int) ([]servicesRepo.MonthlyUsageRow, error) {
			if id != serviceID || fiscalYearStart != 2026 {
				t.Fatalf("FetchMonthlyUsageForService(%v,%d)", id, fiscalYearStart)
			}
			return []servicesRepo.MonthlyUsageRow{
				{Month: "2026-04", NotificationType: servicesRepo.NullNotificationType{NotificationType: servicesRepo.NotificationTypeEmail, Valid: true}, NotificationStatus: servicesRepo.NullNotifyStatusType{NotifyStatusType: servicesRepo.NotifyStatusTypeDelivered, Valid: true}, Count: 3},
				{Month: "2026-04", NotificationType: servicesRepo.NullNotificationType{NotificationType: servicesRepo.NotificationTypeEmail, Valid: true}, NotificationStatus: servicesRepo.NullNotifyStatusType{NotifyStatusType: servicesRepo.NotifyStatusTypeFailed, Valid: true}, Count: 1},
			}, nil
		},
	}

	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/"+serviceID.String()+"/monthly-usage?year=2026", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"2026-04":{"email":{"requested":4,"delivered":3,"failed":1}`) {
		t.Fatalf("body = %s", body)
	}
	if !strings.Contains(body, `"2027-03":{}`) {
		t.Fatalf("body = %s", body)
	}
}

func TestCreateServiceHandlerRequiresFields(t *testing.T) {
	r := chi.NewRouter()
	NewHandler(&stubRepository{}).RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Alpha"}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"email_from":["Missing data for required field."]`) {
		t.Fatalf("body = %s", body)
	}
}

func TestCreateServiceHandlerRejectsDuplicates(t *testing.T) {
	userID := uuid.New()
	user := &usersRepo.User{ID: userID, EmailAddress: "user@example.com"}

	for _, tc := range []struct {
		name        string
		repo        *stubRepository
		wantMessage string
	}{
		{
			name: "duplicate name",
			repo: &stubRepository{
				isServiceNameUniqueFn: func(context.Context, string, uuid.UUID) (bool, error) { return false, nil },
				fetchUserByIDFn:       func(context.Context, uuid.UUID) (*usersRepo.User, error) { return user, nil },
			},
			wantMessage: `Duplicate service name 'Alpha'`,
		},
		{
			name: "duplicate email",
			repo: &stubRepository{
				isServiceNameUniqueFn: func(context.Context, string, uuid.UUID) (bool, error) { return true, nil },
				isEmailFromUniqueFn:   func(context.Context, string, uuid.UUID) (bool, error) { return false, nil },
				fetchUserByIDFn:       func(context.Context, uuid.UUID) (*usersRepo.User, error) { return user, nil },
			},
			wantMessage: `Duplicate service name 'alpha@example.com'`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := chi.NewRouter()
			NewHandler(tc.repo).RegisterRoutes(r)
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Alpha","user_id":"`+userID.String()+`","message_limit":1000,"restricted":false,"email_from":"alpha@example.com","created_by":"`+userID.String()+`"}`))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.wantMessage) {
				t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCreateServiceHandlerSetsCountAsLiveForPlatformAdmin(t *testing.T) {
	userID := uuid.New()
	createdServiceID := uuid.New()
	repo := &stubRepository{
		isServiceNameUniqueFn: func(context.Context, string, uuid.UUID) (bool, error) { return true, nil },
		isEmailFromUniqueFn:   func(context.Context, string, uuid.UUID) (bool, error) { return true, nil },
		fetchUserByIDFn: func(_ context.Context, id uuid.UUID) (*usersRepo.User, error) {
			if id != userID {
				t.Fatalf("userID = %v, want %v", id, userID)
			}
			return &usersRepo.User{ID: userID, EmailAddress: "admin@example.com", PlatformAdmin: true}, nil
		},
		createServiceFn: func(_ context.Context, service servicesRepo.Service, gotUserID *uuid.UUID, permissions []string) (*servicesRepo.Service, error) {
			if gotUserID == nil || *gotUserID != userID {
				t.Fatalf("gotUserID = %v, want %v", gotUserID, userID)
			}
			if service.CountAsLive {
				t.Fatal("expected count_as_live=false for platform admin")
			}
			service.ID = createdServiceID
			return &service, nil
		},
	}

	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Alpha","user_id":"`+userID.String()+`","message_limit":1000,"restricted":false,"email_from":"alpha@example.com","created_by":"`+userID.String()+`"}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"count_as_live":false`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestListServicesHandlerUsesFilters(t *testing.T) {
	serviceID := uuid.New()
	userID := uuid.New()
	repo := &stubRepository{
		fetchServicesByUserIDFn: func(_ context.Context, gotUserID uuid.UUID, onlyActive bool) ([]servicesRepo.Service, error) {
			if gotUserID != userID {
				t.Fatalf("userID = %v, want %v", gotUserID, userID)
			}
			if !onlyActive {
				t.Fatal("expected onlyActive=true")
			}
			return []servicesRepo.Service{{ID: serviceID, Name: "Alpha", Active: true}}, nil
		},
	}

	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/?only_active=true&user_id="+userID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Alpha") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestListServicesHandlerDetailedUsesDateRangeAndStats(t *testing.T) {
	serviceID := uuid.New()
	startDate := "2026-04-01"
	endDate := "2026-04-03"
	repo := &stubRepository{
		fetchAllServicesFn: func(_ context.Context, onlyActive bool) ([]servicesRepo.Service, error) {
			if onlyActive {
				t.Fatal("expected onlyActive=false")
			}
			return []servicesRepo.Service{{ID: serviceID, Name: "Alpha", Active: true}}, nil
		},
		fetchStatsForAllFn: func(_ context.Context, includeFromTestKey bool, onlyActive bool, gotStartDate time.Time, gotEndDate time.Time) ([]servicesRepo.TodayStatsForAllServicesRow, error) {
			if includeFromTestKey {
				t.Fatal("expected includeFromTestKey=false")
			}
			if onlyActive {
				t.Fatal("expected onlyActive=false")
			}
			if gotStartDate.Format("2006-01-02") != startDate || gotEndDate.Format("2006-01-02") != endDate {
				t.Fatalf("date range = %s..%s", gotStartDate.Format("2006-01-02"), gotEndDate.Format("2006-01-02"))
			}
			return []servicesRepo.TodayStatsForAllServicesRow{
				{ServiceID: serviceID, NotificationType: servicesRepo.NullNotificationType{NotificationType: servicesRepo.NotificationTypeEmail, Valid: true}, NotificationStatus: servicesRepo.NullNotifyStatusType{NotifyStatusType: servicesRepo.NotifyStatusTypeDelivered, Valid: true}, Count: 2},
				{ServiceID: serviceID, NotificationType: servicesRepo.NullNotificationType{NotificationType: servicesRepo.NotificationTypeEmail, Valid: true}, NotificationStatus: servicesRepo.NullNotifyStatusType{NotifyStatusType: servicesRepo.NotifyStatusTypeFailed, Valid: true}, Count: 1},
			}, nil
		},
	}

	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/?detailed=true&include_from_test_key=false&start_date="+startDate+"&end_date="+endDate, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"statistics":{"sms":{"requested":0,"delivered":0,"failed":0},"email":{"requested":3,"delivered":2,"failed":1}`) {
		t.Fatalf("body = %s", body)
	}
}

func TestListServicesHandlerRejectsInvalidParams(t *testing.T) {
	r := chi.NewRouter()
	NewHandler(&stubRepository{}).RegisterRoutes(r)

	for _, path := range []string{
		"/?only_active=bad",
		"/?user_id=bad-uuid",
		"/?detailed=true&start_date=bad-date",
		"/?detailed=true&start_date=2026-04-03&end_date=2026-04-01",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", path, rec.Code)
		}
	}
}

func TestStatisticsHandlerBuildsGroupedResponse(t *testing.T) {
	serviceID := uuid.New()
	repo := &stubRepository{
		fetchStatsFn: func(_ context.Context, id uuid.UUID, limitDays int) ([]servicesRepo.ServiceStatsRow, error) {
			if id != serviceID {
				t.Fatalf("serviceID = %v, want %v", id, serviceID)
			}
			if limitDays != 7 {
				t.Fatalf("limitDays = %d, want 7", limitDays)
			}
			return []servicesRepo.ServiceStatsRow{
				{NotificationType: servicesRepo.NotificationTypeEmail, NotificationStatus: servicesRepo.NullNotifyStatusType{NotifyStatusType: servicesRepo.NotifyStatusTypeDelivered, Valid: true}, Count: 2},
				{NotificationType: servicesRepo.NotificationTypeEmail, NotificationStatus: servicesRepo.NullNotifyStatusType{NotifyStatusType: servicesRepo.NotifyStatusTypeFailed, Valid: true}, Count: 1},
				{NotificationType: servicesRepo.NotificationTypeSms, NotificationStatus: servicesRepo.NullNotifyStatusType{NotifyStatusType: servicesRepo.NotifyStatusTypeSent, Valid: true}, Count: 4},
			}, nil
		},
	}

	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/"+serviceID.String()+"/statistics", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"email":{"requested":3,"delivered":2,"failed":1}`) {
		t.Fatalf("body = %s", body)
	}
	if !strings.Contains(body, `"sms":{"requested":4,"delivered":0,"failed":0}`) {
		t.Fatalf("body = %s", body)
	}
}

func TestUniquenessHandlersValidateAndReturnResult(t *testing.T) {
	serviceID := uuid.New()
	repo := &stubRepository{
		isServiceNameUniqueFn: func(_ context.Context, name string, id uuid.UUID) (bool, error) {
			if name != "Alpha" || id != serviceID {
				t.Fatalf("unexpected name uniqueness inputs: %q %v", name, id)
			}
			return true, nil
		},
		isEmailFromUniqueFn: func(_ context.Context, emailFrom string, id uuid.UUID) (bool, error) {
			if emailFrom != "alpha@example.com" || id != serviceID {
				t.Fatalf("unexpected email uniqueness inputs: %q %v", emailFrom, id)
			}
			return false, nil
		},
	}

	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	nameReq := httptest.NewRequest(http.MethodGet, "/is-name-unique?name=Alpha&service_id="+serviceID.String(), nil)
	nameRec := httptest.NewRecorder()
	r.ServeHTTP(nameRec, nameReq)
	if nameRec.Code != http.StatusOK || !strings.Contains(nameRec.Body.String(), `"result":true`) {
		t.Fatalf("name response = %d %s", nameRec.Code, nameRec.Body.String())
	}

	emailReq := httptest.NewRequest(http.MethodGet, "/is-email-from-unique?email_from=alpha@example.com&service_id="+serviceID.String(), nil)
	emailRec := httptest.NewRecorder()
	r.ServeHTTP(emailRec, emailReq)
	if emailRec.Code != http.StatusOK || !strings.Contains(emailRec.Body.String(), `"result":false`) {
		t.Fatalf("email response = %d %s", emailRec.Code, emailRec.Body.String())
	}

	badReq := httptest.NewRequest(http.MethodGet, "/is-name-unique", nil)
	badRec := httptest.NewRecorder()
	r.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("bad request status = %d, want 400", badRec.Code)
	}
}

func TestFindByNameHandlerUsesPartialCaseInsensitiveQuery(t *testing.T) {
	firstID := uuid.New()
	secondID := uuid.New()
	repo := &stubRepository{
		findByNameFn: func(_ context.Context, name string) ([]servicesRepo.Service, error) {
			if name != "ALPHA" {
				t.Fatalf("name = %q, want ALPHA", name)
			}
			return []servicesRepo.Service{{ID: firstID, Name: "Alpha One"}, {ID: secondID, Name: "alpha-two"}}, nil
		},
	}

	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/find-by-name?service_name=ALPHA", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Alpha One") || !strings.Contains(rec.Body.String(), "alpha-two") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestFindByNameHandlerRequiresServiceName(t *testing.T) {
	r := chi.NewRouter()
	NewHandler(&stubRepository{}).RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/find-by-name", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestArchiveSuspendResumeHandlers(t *testing.T) {
	serviceID := uuid.New()
	userID := uuid.New()
	service := &servicesRepo.Service{ID: serviceID, Active: true}
	repo := &stubRepository{}

	repo.archiveFn = func(_ context.Context, id uuid.UUID) (*servicesRepo.Service, error) {
		if id != serviceID {
			t.Fatalf("archive serviceID = %v, want %v", id, serviceID)
		}
		return service, nil
	}
	repo.suspendFn = func(_ context.Context, id uuid.UUID, gotUserID *uuid.UUID) (*servicesRepo.Service, error) {
		if id != serviceID {
			t.Fatalf("suspend serviceID = %v, want %v", id, serviceID)
		}
		if gotUserID == nil || *gotUserID != userID {
			t.Fatalf("suspend userID = %v, want %v", gotUserID, userID)
		}
		return &servicesRepo.Service{ID: serviceID, Active: false}, nil
	}
	repo.resumeFn = func(_ context.Context, id uuid.UUID) (*servicesRepo.Service, error) {
		if id != serviceID {
			t.Fatalf("resume serviceID = %v, want %v", id, serviceID)
		}
		return service, nil
	}

	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)

	archiveReq := httptest.NewRequest(http.MethodPost, "/"+serviceID.String()+"/archive", nil)
	archiveRec := httptest.NewRecorder()
	r.ServeHTTP(archiveRec, archiveReq)
	if archiveRec.Code != http.StatusNoContent {
		t.Fatalf("archive status = %d, want 204", archiveRec.Code)
	}

	suspendReq := httptest.NewRequest(http.MethodPost, "/"+serviceID.String()+"/suspend", strings.NewReader(`{"user_id":"`+userID.String()+`"}`))
	suspendRec := httptest.NewRecorder()
	r.ServeHTTP(suspendRec, suspendReq)
	if suspendRec.Code != http.StatusNoContent {
		t.Fatalf("suspend status = %d, want 204", suspendRec.Code)
	}

	resumeReq := httptest.NewRequest(http.MethodPost, "/"+serviceID.String()+"/resume", nil)
	resumeRec := httptest.NewRecorder()
	r.ServeHTTP(resumeRec, resumeReq)
	if resumeRec.Code != http.StatusNoContent {
		t.Fatalf("resume status = %d, want 204", resumeRec.Code)
	}
}

func TestSuspendHandlerAllowsEmptyBodyForIdempotentNoUserCall(t *testing.T) {
	serviceID := uuid.New()
	repo := &stubRepository{
		suspendFn: func(_ context.Context, id uuid.UUID, userID *uuid.UUID) (*servicesRepo.Service, error) {
			if id != serviceID {
				t.Fatalf("serviceID = %v, want %v", id, serviceID)
			}
			if userID != nil {
				t.Fatalf("userID = %v, want nil", *userID)
			}
			return &servicesRepo.Service{ID: serviceID, Active: false}, nil
		},
	}

	r := chi.NewRouter()
	NewHandler(repo).RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/"+serviceID.String()+"/suspend", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestServiceLifecycleHandlersReturnNotFoundAndValidationErrors(t *testing.T) {
	t.Run("not found from repo", func(t *testing.T) {
		serviceID := uuid.New()
		repo := &stubRepository{
			archiveFn: func(context.Context, uuid.UUID) (*servicesRepo.Service, error) { return nil, nil },
			resumeFn:  func(context.Context, uuid.UUID) (*servicesRepo.Service, error) { return nil, nil },
			suspendFn: func(context.Context, uuid.UUID, *uuid.UUID) (*servicesRepo.Service, error) { return nil, nil },
		}
		r := chi.NewRouter()
		NewHandler(repo).RegisterRoutes(r)

		for _, path := range []string{"/" + serviceID.String() + "/archive", "/" + serviceID.String() + "/resume", "/" + serviceID.String() + "/suspend"} {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s status = %d, want 404", path, rec.Code)
			}
		}
	})

	t.Run("invalid ids", func(t *testing.T) {
		serviceID := uuid.New()
		r := chi.NewRouter()
		NewHandler(&stubRepository{}).RegisterRoutes(r)

		badServiceReq := httptest.NewRequest(http.MethodPost, "/not-a-uuid/archive", nil)
		badServiceRec := httptest.NewRecorder()
		r.ServeHTTP(badServiceRec, badServiceReq)
		if badServiceRec.Code != http.StatusBadRequest {
			t.Fatalf("bad service status = %d, want 400", badServiceRec.Code)
		}

		badUserReq := httptest.NewRequest(http.MethodPost, "/"+serviceID.String()+"/suspend", strings.NewReader(`{"user_id":"not-a-uuid"}`))
		badUserRec := httptest.NewRecorder()
		r.ServeHTTP(badUserRec, badUserReq)
		if badUserRec.Code != http.StatusBadRequest {
			t.Fatalf("bad user status = %d, want 400", badUserRec.Code)
		}
	})
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
		permissions:  []string{"manage_settings"},
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
