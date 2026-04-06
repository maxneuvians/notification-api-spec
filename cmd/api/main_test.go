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
	organisationsRepo "github.com/maxneuvians/notification-api-spec/internal/repository/organisations"
	servicesRepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	usersRepo "github.com/maxneuvians/notification-api-spec/internal/repository/users"
	serviceauth "github.com/maxneuvians/notification-api-spec/internal/service/auth"
	serviceerrs "github.com/maxneuvians/notification-api-spec/internal/service/services"
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

type serviceAdminRepoStub struct {
	createAPIKeyFn                     func(context.Context, uuid.UUID, string, uuid.UUID, string) (*servicesRepo.CreatedAPIKey, error)
	listAPIKeysFn                      func(context.Context, uuid.UUID, *uuid.UUID) ([]apiKeysRepo.ApiKey, error)
	revokeAPIKeyFn                     func(context.Context, uuid.UUID, uuid.UUID) (*apiKeysRepo.ApiKey, error)
	getSmsSenderByIDFn                 func(context.Context, uuid.UUID, uuid.UUID) (*servicesRepo.ServiceSmsSender, error)
	getSmsSendersByServiceIDFn         func(context.Context, uuid.UUID) ([]servicesRepo.ServiceSmsSender, error)
	addSmsSenderForServiceFn           func(context.Context, servicesRepo.ServiceSmsSender) (*servicesRepo.ServiceSmsSender, error)
	updateServiceSmsSenderFn           func(context.Context, servicesRepo.ServiceSmsSender) (*servicesRepo.ServiceSmsSender, error)
	updateSmsSenderWithInboundNumberFn func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*servicesRepo.ServiceSmsSender, error)
	archiveSmsSenderFn                 func(context.Context, uuid.UUID, uuid.UUID) (*servicesRepo.ServiceSmsSender, error)
	fetchServiceByIDFn                 func(context.Context, uuid.UUID, bool) (*servicesRepo.Service, error)
	fetchServiceDataRetentionFn        func(context.Context, uuid.UUID) ([]servicesRepo.ServiceDataRetention, error)
	fetchServiceDataRetentionByIDFn    func(context.Context, uuid.UUID, uuid.UUID) (*servicesRepo.ServiceDataRetention, error)
	fetchDataRetentionByTypeFn         func(context.Context, uuid.UUID, servicesRepo.NotificationType) (*servicesRepo.ServiceDataRetention, error)
	insertServiceDataRetentionFn       func(context.Context, servicesRepo.ServiceDataRetention) (*servicesRepo.ServiceDataRetention, error)
	updateServiceDataRetentionFn       func(context.Context, uuid.UUID, uuid.UUID, int32) (int64, error)
	fetchServiceSafelistFn             func(context.Context, uuid.UUID) ([]servicesRepo.ServiceSafelist, error)
	addSafelistedContactsFn            func(context.Context, uuid.UUID, []string, []string) error
	removeServiceSafelistFn            func(context.Context, uuid.UUID) error
	getReplyTosByServiceIDFn           func(context.Context, uuid.UUID) ([]servicesRepo.ServiceEmailReplyTo, error)
	getReplyToByIDFn                   func(context.Context, uuid.UUID, uuid.UUID) (*servicesRepo.ServiceEmailReplyTo, error)
	addReplyToEmailAddressFn           func(context.Context, servicesRepo.ServiceEmailReplyTo) (*servicesRepo.ServiceEmailReplyTo, error)
	updateReplyToEmailAddressFn        func(context.Context, servicesRepo.ServiceEmailReplyTo) (*servicesRepo.ServiceEmailReplyTo, error)
	archiveReplyToEmailAddressFn       func(context.Context, uuid.UUID, uuid.UUID) (*servicesRepo.ServiceEmailReplyTo, error)
	fetchAllServicesFn                 func(context.Context, bool) ([]servicesRepo.Service, error)
	fetchServicesByUserIDFn            func(context.Context, uuid.UUID, bool) ([]servicesRepo.Service, error)
	createServiceFn                    func(context.Context, servicesRepo.Service, *uuid.UUID, []string) (*servicesRepo.Service, error)
	fetchUserByIDFn                    func(context.Context, uuid.UUID) (*usersRepo.User, error)
	fetchOrganisationByEmailFn         func(context.Context, string) (*organisationsRepo.Organisation, error)
	fetchNHSEmailBrandingIDFn          func(context.Context) (*uuid.UUID, error)
	fetchNHSLetterBrandingIDFn         func(context.Context) (*uuid.UUID, error)
	assignServiceBrandingFn            func(context.Context, uuid.UUID, *uuid.UUID, *uuid.UUID) error
	getServicesByPartialNameFn         func(context.Context, string) ([]servicesRepo.Service, error)
	fetchStatsForAllServicesFn         func(context.Context, bool, bool, time.Time, time.Time) ([]servicesRepo.TodayStatsForAllServicesRow, error)
	fetchServiceHistoryFn              func(context.Context, uuid.UUID) ([]servicesRepo.ServicesHistory, error)
	fetchAPIKeyHistoryFn               func(context.Context, uuid.UUID) ([]apiKeysRepo.APIKeyHistory, error)
	isServiceNameUniqueFn              func(context.Context, string, uuid.UUID) (bool, error)
	isServiceEmailFromUniqueFn         func(context.Context, string, uuid.UUID) (bool, error)
	fetchServiceOrganisationFn         func(context.Context, uuid.UUID) (*organisationsRepo.Organisation, error)
	fetchStatsForServiceFn             func(context.Context, uuid.UUID, int) ([]servicesRepo.ServiceStatsRow, error)
	fetchTodaysStatsForServiceFn       func(context.Context, uuid.UUID) ([]servicesRepo.ServiceStatsRow, error)
	fetchMonthlyUsageForServiceFn      func(context.Context, uuid.UUID, int) ([]servicesRepo.MonthlyUsageRow, error)
	fetchLiveServicesDataFn            func(context.Context) ([]servicesRepo.GetLiveServicesDataRow, error)
	fetchSensitiveServiceIDsFn         func(context.Context) ([]uuid.UUID, error)
	fetchAnnualLimitStatsFn            func(context.Context, uuid.UUID) (*servicesRepo.AnnualLimitStats, error)
	suspendServiceFn                   func(context.Context, uuid.UUID, *uuid.UUID) (*servicesRepo.Service, error)
	resumeServiceFn                    func(context.Context, uuid.UUID) (*servicesRepo.Service, error)
	archiveServiceFn                   func(context.Context, uuid.UUID) (*servicesRepo.Service, error)
	saveServiceCallbackApiFn           func(context.Context, uuid.UUID, string, string, string, uuid.UUID) (*servicesRepo.ServiceCallbackApi, error)
	resetServiceCallbackApiFn          func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, *string, *string) (*servicesRepo.ServiceCallbackApi, error)
	getServiceCallbackApiFn            func(context.Context, uuid.UUID, uuid.UUID) (*servicesRepo.ServiceCallbackApi, error)
	deleteServiceCallbackApiFn         func(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	suspendUnsuspendCallbackApiFn      func(context.Context, uuid.UUID, uuid.UUID, bool) (*servicesRepo.ServiceCallbackApi, error)
	saveServiceInboundApiFn            func(context.Context, uuid.UUID, string, string, uuid.UUID) (*servicesRepo.ServiceInboundApi, error)
	resetServiceInboundApiFn           func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, *string, *string) (*servicesRepo.ServiceInboundApi, error)
	getServiceInboundApiFn             func(context.Context, uuid.UUID, uuid.UUID) (*servicesRepo.ServiceInboundApi, error)
	deleteServiceInboundApiFn          func(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

func (s *serviceAdminRepoStub) CreateAPIKey(ctx context.Context, serviceID uuid.UUID, name string, createdByID uuid.UUID, keyType string) (*servicesRepo.CreatedAPIKey, error) {
	if s.createAPIKeyFn == nil {
		return nil, nil
	}
	return s.createAPIKeyFn(ctx, serviceID, name, createdByID, keyType)
}

func (s *serviceAdminRepoStub) ListAPIKeys(ctx context.Context, serviceID uuid.UUID, keyID *uuid.UUID) ([]apiKeysRepo.ApiKey, error) {
	if s.listAPIKeysFn == nil {
		return nil, nil
	}
	return s.listAPIKeysFn(ctx, serviceID, keyID)
}

func (s *serviceAdminRepoStub) RevokeAPIKey(ctx context.Context, serviceID uuid.UUID, keyID uuid.UUID) (*apiKeysRepo.ApiKey, error) {
	if s.revokeAPIKeyFn == nil {
		return nil, nil
	}
	return s.revokeAPIKeyFn(ctx, serviceID, keyID)
}

func (s *serviceAdminRepoStub) GetSmsSenderByID(ctx context.Context, serviceID uuid.UUID, senderID uuid.UUID) (*servicesRepo.ServiceSmsSender, error) {
	if s.getSmsSenderByIDFn == nil {
		return nil, nil
	}
	return s.getSmsSenderByIDFn(ctx, serviceID, senderID)
}

func (s *serviceAdminRepoStub) GetSmsSendersByServiceID(ctx context.Context, serviceID uuid.UUID) ([]servicesRepo.ServiceSmsSender, error) {
	if s.getSmsSendersByServiceIDFn == nil {
		return nil, nil
	}
	return s.getSmsSendersByServiceIDFn(ctx, serviceID)
}

func (s *serviceAdminRepoStub) AddSmsSenderForService(ctx context.Context, sender servicesRepo.ServiceSmsSender) (*servicesRepo.ServiceSmsSender, error) {
	if s.addSmsSenderForServiceFn == nil {
		return nil, nil
	}
	return s.addSmsSenderForServiceFn(ctx, sender)
}

func (s *serviceAdminRepoStub) UpdateServiceSmsSender(ctx context.Context, sender servicesRepo.ServiceSmsSender) (*servicesRepo.ServiceSmsSender, error) {
	if s.updateServiceSmsSenderFn == nil {
		return nil, nil
	}
	return s.updateServiceSmsSenderFn(ctx, sender)
}

func (s *serviceAdminRepoStub) UpdateSmsSenderWithInboundNumber(ctx context.Context, serviceID, senderID, inboundNumberID uuid.UUID) (*servicesRepo.ServiceSmsSender, error) {
	if s.updateSmsSenderWithInboundNumberFn == nil {
		return nil, nil
	}
	return s.updateSmsSenderWithInboundNumberFn(ctx, serviceID, senderID, inboundNumberID)
}

func (s *serviceAdminRepoStub) ArchiveSmsSender(ctx context.Context, serviceID, senderID uuid.UUID) (*servicesRepo.ServiceSmsSender, error) {
	if s.archiveSmsSenderFn == nil {
		return nil, nil
	}
	return s.archiveSmsSenderFn(ctx, serviceID, senderID)
}

func (s *serviceAdminRepoStub) FetchServiceByID(ctx context.Context, id uuid.UUID, onlyActive bool) (*servicesRepo.Service, error) {
	if s.fetchServiceByIDFn == nil {
		return nil, nil
	}
	return s.fetchServiceByIDFn(ctx, id, onlyActive)
}

func (s *serviceAdminRepoStub) FetchAllServices(ctx context.Context, onlyActive bool) ([]servicesRepo.Service, error) {
	if s.fetchAllServicesFn == nil {
		return nil, nil
	}
	return s.fetchAllServicesFn(ctx, onlyActive)
}

func (s *serviceAdminRepoStub) FetchServicesByUserID(ctx context.Context, userID uuid.UUID, onlyActive bool) ([]servicesRepo.Service, error) {
	if s.fetchServicesByUserIDFn == nil {
		return nil, nil
	}
	return s.fetchServicesByUserIDFn(ctx, userID, onlyActive)
}

func (s *serviceAdminRepoStub) CreateService(ctx context.Context, service servicesRepo.Service, userID *uuid.UUID, permissions []string) (*servicesRepo.Service, error) {
	if s.createServiceFn == nil {
		return nil, nil
	}
	return s.createServiceFn(ctx, service, userID, permissions)
}

func (s *serviceAdminRepoStub) FetchUserByID(ctx context.Context, userID uuid.UUID) (*usersRepo.User, error) {
	if s.fetchUserByIDFn == nil {
		return nil, nil
	}
	return s.fetchUserByIDFn(ctx, userID)
}

func (s *serviceAdminRepoStub) FetchOrganisationByEmailAddress(ctx context.Context, emailAddress string) (*organisationsRepo.Organisation, error) {
	if s.fetchOrganisationByEmailFn == nil {
		return nil, nil
	}
	return s.fetchOrganisationByEmailFn(ctx, emailAddress)
}

func (s *serviceAdminRepoStub) FetchNHSEmailBrandingID(ctx context.Context) (*uuid.UUID, error) {
	if s.fetchNHSEmailBrandingIDFn == nil {
		return nil, nil
	}
	return s.fetchNHSEmailBrandingIDFn(ctx)
}

func (s *serviceAdminRepoStub) FetchNHSLetterBrandingID(ctx context.Context) (*uuid.UUID, error) {
	if s.fetchNHSLetterBrandingIDFn == nil {
		return nil, nil
	}
	return s.fetchNHSLetterBrandingIDFn(ctx)
}

func (s *serviceAdminRepoStub) AssignServiceBranding(ctx context.Context, serviceID uuid.UUID, emailBrandingID *uuid.UUID, letterBrandingID *uuid.UUID) error {
	if s.assignServiceBrandingFn == nil {
		return nil
	}
	return s.assignServiceBrandingFn(ctx, serviceID, emailBrandingID, letterBrandingID)
}

func (s *serviceAdminRepoStub) GetServicesByPartialName(ctx context.Context, name string) ([]servicesRepo.Service, error) {
	if s.getServicesByPartialNameFn == nil {
		return nil, nil
	}
	return s.getServicesByPartialNameFn(ctx, name)
}

func (s *serviceAdminRepoStub) FetchStatsForAllServices(ctx context.Context, includeFromTestKey bool, onlyActive bool, startDate time.Time, endDate time.Time) ([]servicesRepo.TodayStatsForAllServicesRow, error) {
	if s.fetchStatsForAllServicesFn == nil {
		return nil, nil
	}
	return s.fetchStatsForAllServicesFn(ctx, includeFromTestKey, onlyActive, startDate, endDate)
}

func (s *serviceAdminRepoStub) FetchServiceHistory(ctx context.Context, serviceID uuid.UUID) ([]servicesRepo.ServicesHistory, error) {
	if s.fetchServiceHistoryFn == nil {
		return nil, nil
	}
	return s.fetchServiceHistoryFn(ctx, serviceID)
}

func (s *serviceAdminRepoStub) FetchAPIKeyHistory(ctx context.Context, serviceID uuid.UUID) ([]apiKeysRepo.APIKeyHistory, error) {
	if s.fetchAPIKeyHistoryFn == nil {
		return nil, nil
	}
	return s.fetchAPIKeyHistoryFn(ctx, serviceID)
}

func (s *serviceAdminRepoStub) IsServiceNameUnique(ctx context.Context, name string, serviceID uuid.UUID) (bool, error) {
	if s.isServiceNameUniqueFn == nil {
		return false, nil
	}
	return s.isServiceNameUniqueFn(ctx, name, serviceID)
}

func (s *serviceAdminRepoStub) IsServiceEmailFromUnique(ctx context.Context, emailFrom string, serviceID uuid.UUID) (bool, error) {
	if s.isServiceEmailFromUniqueFn == nil {
		return false, nil
	}
	return s.isServiceEmailFromUniqueFn(ctx, emailFrom, serviceID)
}

func (s *serviceAdminRepoStub) FetchServiceOrganisation(ctx context.Context, serviceID uuid.UUID) (*organisationsRepo.Organisation, error) {
	if s.fetchServiceOrganisationFn == nil {
		return nil, nil
	}
	return s.fetchServiceOrganisationFn(ctx, serviceID)
}

func (s *serviceAdminRepoStub) FetchStatsForService(ctx context.Context, serviceID uuid.UUID, limitDays int) ([]servicesRepo.ServiceStatsRow, error) {
	if s.fetchStatsForServiceFn == nil {
		return nil, nil
	}
	return s.fetchStatsForServiceFn(ctx, serviceID, limitDays)
}

func (s *serviceAdminRepoStub) FetchTodaysStatsForService(ctx context.Context, serviceID uuid.UUID) ([]servicesRepo.ServiceStatsRow, error) {
	if s.fetchTodaysStatsForServiceFn == nil {
		return nil, nil
	}
	return s.fetchTodaysStatsForServiceFn(ctx, serviceID)
}

func (s *serviceAdminRepoStub) FetchMonthlyUsageForService(ctx context.Context, serviceID uuid.UUID, fiscalYearStart int) ([]servicesRepo.MonthlyUsageRow, error) {
	if s.fetchMonthlyUsageForServiceFn == nil {
		return nil, nil
	}
	return s.fetchMonthlyUsageForServiceFn(ctx, serviceID, fiscalYearStart)
}

func (s *serviceAdminRepoStub) FetchLiveServicesData(ctx context.Context) ([]servicesRepo.GetLiveServicesDataRow, error) {
	if s.fetchLiveServicesDataFn == nil {
		return nil, nil
	}
	return s.fetchLiveServicesDataFn(ctx)
}

func (s *serviceAdminRepoStub) FetchSensitiveServiceIDs(ctx context.Context) ([]uuid.UUID, error) {
	if s.fetchSensitiveServiceIDsFn == nil {
		return nil, nil
	}
	return s.fetchSensitiveServiceIDsFn(ctx)
}

func (s *serviceAdminRepoStub) FetchAnnualLimitStats(ctx context.Context, serviceID uuid.UUID) (*servicesRepo.AnnualLimitStats, error) {
	if s.fetchAnnualLimitStatsFn == nil {
		return &servicesRepo.AnnualLimitStats{}, nil
	}
	return s.fetchAnnualLimitStatsFn(ctx, serviceID)
}

func (s *serviceAdminRepoStub) SuspendService(ctx context.Context, id uuid.UUID, userID *uuid.UUID) (*servicesRepo.Service, error) {
	if s.suspendServiceFn == nil {
		return nil, nil
	}
	return s.suspendServiceFn(ctx, id, userID)
}

func (s *serviceAdminRepoStub) ResumeService(ctx context.Context, id uuid.UUID) (*servicesRepo.Service, error) {
	if s.resumeServiceFn == nil {
		return nil, nil
	}
	return s.resumeServiceFn(ctx, id)
}

func (s *serviceAdminRepoStub) ArchiveService(ctx context.Context, id uuid.UUID) (*servicesRepo.Service, error) {
	if s.archiveServiceFn == nil {
		return nil, nil
	}
	return s.archiveServiceFn(ctx, id)
}

func (s *serviceAdminRepoStub) FetchServiceDataRetention(ctx context.Context, serviceID uuid.UUID) ([]servicesRepo.ServiceDataRetention, error) {
	if s.fetchServiceDataRetentionFn == nil {
		return nil, nil
	}
	return s.fetchServiceDataRetentionFn(ctx, serviceID)
}

func (s *serviceAdminRepoStub) FetchServiceDataRetentionByID(ctx context.Context, serviceID, retentionID uuid.UUID) (*servicesRepo.ServiceDataRetention, error) {
	if s.fetchServiceDataRetentionByIDFn == nil {
		return nil, nil
	}
	return s.fetchServiceDataRetentionByIDFn(ctx, serviceID, retentionID)
}

func (s *serviceAdminRepoStub) FetchDataRetentionByNotificationType(ctx context.Context, serviceID uuid.UUID, notificationType servicesRepo.NotificationType) (*servicesRepo.ServiceDataRetention, error) {
	if s.fetchDataRetentionByTypeFn == nil {
		return nil, nil
	}
	return s.fetchDataRetentionByTypeFn(ctx, serviceID, notificationType)
}

func (s *serviceAdminRepoStub) InsertServiceDataRetention(ctx context.Context, retention servicesRepo.ServiceDataRetention) (*servicesRepo.ServiceDataRetention, error) {
	if s.insertServiceDataRetentionFn == nil {
		return nil, nil
	}
	return s.insertServiceDataRetentionFn(ctx, retention)
}

func (s *serviceAdminRepoStub) UpdateServiceDataRetention(ctx context.Context, serviceID, retentionID uuid.UUID, daysOfRetention int32) (int64, error) {
	if s.updateServiceDataRetentionFn == nil {
		return 0, nil
	}
	return s.updateServiceDataRetentionFn(ctx, serviceID, retentionID, daysOfRetention)
}

func (s *serviceAdminRepoStub) FetchServiceSafelist(ctx context.Context, serviceID uuid.UUID) ([]servicesRepo.ServiceSafelist, error) {
	if s.fetchServiceSafelistFn == nil {
		return nil, nil
	}
	return s.fetchServiceSafelistFn(ctx, serviceID)
}

func (s *serviceAdminRepoStub) AddSafelistedContacts(ctx context.Context, serviceID uuid.UUID, emailAddresses, phoneNumbers []string) error {
	if s.addSafelistedContactsFn == nil {
		return nil
	}
	return s.addSafelistedContactsFn(ctx, serviceID, emailAddresses, phoneNumbers)
}

func (s *serviceAdminRepoStub) RemoveServiceSafelist(ctx context.Context, serviceID uuid.UUID) error {
	if s.removeServiceSafelistFn == nil {
		return nil
	}
	return s.removeServiceSafelistFn(ctx, serviceID)
}

func (s *serviceAdminRepoStub) GetReplyTosByServiceID(ctx context.Context, serviceID uuid.UUID) ([]servicesRepo.ServiceEmailReplyTo, error) {
	if s.getReplyTosByServiceIDFn == nil {
		return nil, nil
	}
	return s.getReplyTosByServiceIDFn(ctx, serviceID)
}

func (s *serviceAdminRepoStub) GetReplyToByID(ctx context.Context, serviceID uuid.UUID, replyToID uuid.UUID) (*servicesRepo.ServiceEmailReplyTo, error) {
	if s.getReplyToByIDFn == nil {
		return nil, nil
	}
	return s.getReplyToByIDFn(ctx, serviceID, replyToID)
}

func (s *serviceAdminRepoStub) AddReplyToEmailAddress(ctx context.Context, replyTo servicesRepo.ServiceEmailReplyTo) (*servicesRepo.ServiceEmailReplyTo, error) {
	if s.addReplyToEmailAddressFn == nil {
		return nil, nil
	}
	return s.addReplyToEmailAddressFn(ctx, replyTo)
}

func (s *serviceAdminRepoStub) UpdateReplyToEmailAddress(ctx context.Context, replyTo servicesRepo.ServiceEmailReplyTo) (*servicesRepo.ServiceEmailReplyTo, error) {
	if s.updateReplyToEmailAddressFn == nil {
		return nil, nil
	}
	return s.updateReplyToEmailAddressFn(ctx, replyTo)
}

func (s *serviceAdminRepoStub) ArchiveReplyToEmailAddress(ctx context.Context, serviceID, replyToID uuid.UUID) (*servicesRepo.ServiceEmailReplyTo, error) {
	if s.archiveReplyToEmailAddressFn == nil {
		return nil, nil
	}
	return s.archiveReplyToEmailAddressFn(ctx, serviceID, replyToID)
}

func (s *serviceAdminRepoStub) SaveServiceCallbackApi(ctx context.Context, serviceID uuid.UUID, callbackType string, url string, bearerToken string, updatedByID uuid.UUID) (*servicesRepo.ServiceCallbackApi, error) {
	if s.saveServiceCallbackApiFn == nil {
		return nil, nil
	}
	return s.saveServiceCallbackApiFn(ctx, serviceID, callbackType, url, bearerToken, updatedByID)
}

func (s *serviceAdminRepoStub) ResetServiceCallbackApi(ctx context.Context, serviceID, callbackID uuid.UUID, updatedByID uuid.UUID, url *string, bearerToken *string) (*servicesRepo.ServiceCallbackApi, error) {
	if s.resetServiceCallbackApiFn == nil {
		return nil, nil
	}
	return s.resetServiceCallbackApiFn(ctx, serviceID, callbackID, updatedByID, url, bearerToken)
}

func (s *serviceAdminRepoStub) GetServiceCallbackApi(ctx context.Context, serviceID, callbackID uuid.UUID) (*servicesRepo.ServiceCallbackApi, error) {
	if s.getServiceCallbackApiFn == nil {
		return nil, nil
	}
	return s.getServiceCallbackApiFn(ctx, serviceID, callbackID)
}

func (s *serviceAdminRepoStub) DeleteServiceCallbackApi(ctx context.Context, serviceID, callbackID uuid.UUID) (bool, error) {
	if s.deleteServiceCallbackApiFn == nil {
		return false, nil
	}
	return s.deleteServiceCallbackApiFn(ctx, serviceID, callbackID)
}

func (s *serviceAdminRepoStub) SuspendUnsuspendCallbackApi(ctx context.Context, serviceID uuid.UUID, updatedByID uuid.UUID, suspend bool) (*servicesRepo.ServiceCallbackApi, error) {
	if s.suspendUnsuspendCallbackApiFn == nil {
		return nil, nil
	}
	return s.suspendUnsuspendCallbackApiFn(ctx, serviceID, updatedByID, suspend)
}

func (s *serviceAdminRepoStub) SaveServiceInboundApi(ctx context.Context, serviceID uuid.UUID, url string, bearerToken string, updatedByID uuid.UUID) (*servicesRepo.ServiceInboundApi, error) {
	if s.saveServiceInboundApiFn == nil {
		return nil, nil
	}
	return s.saveServiceInboundApiFn(ctx, serviceID, url, bearerToken, updatedByID)
}

func (s *serviceAdminRepoStub) ResetServiceInboundApi(ctx context.Context, serviceID, inboundID uuid.UUID, updatedByID uuid.UUID, url *string, bearerToken *string) (*servicesRepo.ServiceInboundApi, error) {
	if s.resetServiceInboundApiFn == nil {
		return nil, nil
	}
	return s.resetServiceInboundApiFn(ctx, serviceID, inboundID, updatedByID, url, bearerToken)
}

func (s *serviceAdminRepoStub) GetServiceInboundApi(ctx context.Context, serviceID, inboundID uuid.UUID) (*servicesRepo.ServiceInboundApi, error) {
	if s.getServiceInboundApiFn == nil {
		return nil, nil
	}
	return s.getServiceInboundApiFn(ctx, serviceID, inboundID)
}

func (s *serviceAdminRepoStub) DeleteServiceInboundApi(ctx context.Context, serviceID, inboundID uuid.UUID) (bool, error) {
	if s.deleteServiceInboundApiFn == nil {
		return false, nil
	}
	return s.deleteServiceInboundApiFn(ctx, serviceID, inboundID)
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
		server := httptest.NewServer(newRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, authRepo, nil, nil, nil))
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
		server := httptest.NewServer(newRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, authRepo, nil, nil, nil))
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
		server := httptest.NewServer(newRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, authRepo, nil, nil, nil))
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
		server := httptest.NewServer(newRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, authRepo, nil, nil, nil))
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
		server := httptest.NewServer(newRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, authRepo, nil, nil, nil))
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
		server := httptest.NewServer(newRouter(&prodCfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, authRepo, nil, nil, nil))
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
		server := httptest.NewServer(newRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), cache, authRepo, nil, nil, nil))
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

func TestSMSSenderValidationErrorsAreReturnedAsStructuredJSON(t *testing.T) {
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
	serviceID := uuid.New()
	senderID := uuid.New()
	authRepo, plaintextKey := newAuthRepoStub(t, cfg, serviceID)
	server := httptest.NewServer(newRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, authRepo, nil, nil, &serviceAdminRepoStub{
		updateServiceSmsSenderFn: func(context.Context, servicesRepo.ServiceSmsSender) (*servicesRepo.ServiceSmsSender, error) {
			return nil, serviceerrs.InvalidRequestError{Message: "You must have at least one SMS sender as the default.", StatusCode: http.StatusBadRequest}
		},
		addSmsSenderForServiceFn: func(context.Context, servicesRepo.ServiceSmsSender) (*servicesRepo.ServiceSmsSender, error) {
			return nil, serviceerrs.InvalidRequestError{Message: "You must have at least one SMS sender as the default.", StatusCode: http.StatusBadRequest}
		},
	}))
	defer server.Close()

	for _, tc := range []struct {
		name   string
		path   string
		body   string
		method string
	}{
		{name: "update sole default", method: http.MethodPost, path: "/v2/service/" + serviceID.String() + "/sms-sender/" + senderID.String(), body: `{"sms_sender":"Notify","is_default":false}`},
		{name: "add without default", method: http.MethodPost, path: "/v2/service/" + serviceID.String() + "/sms-sender", body: `{"sms_sender":"Notify","is_default":false}`},
	} {
		req, err := http.NewRequest(tc.method, server.URL+tc.path, strings.NewReader(tc.body))
		if err != nil {
			t.Fatalf("%s NewRequest() error = %v", tc.name, err)
		}
		req.Header.Set("Authorization", "ApiKey-v1 "+plaintextKey)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s Do() error = %v", tc.name, err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", tc.name, res.StatusCode)
		}
		if got := res.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
			t.Fatalf("%s content-type = %q, want application/json", tc.name, got)
		}
		var body map[string]string
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatalf("%s decode error = %v", tc.name, err)
		}
		if body["result"] != "error" || body["message"] != "You must have at least one SMS sender as the default." {
			t.Fatalf("%s body = %v", tc.name, body)
		}
	}
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

func newAuthRepoStub(t *testing.T, cfg *config.Config, serviceID uuid.UUID) (*authRepoStub, string) {
	t.Helper()
	plaintextToken := uuid.New().String()
	apiKeySecret, err := signing.SignAPIKeyToken(plaintextToken, cfg.SecretKeys[0])
	if err != nil {
		t.Fatalf("SignAPIKeyToken() error = %v", err)
	}
	apiKey := apiKeysRepo.ApiKey{ID: uuid.New(), ServiceID: serviceID, Secret: apiKeySecret, KeyType: "normal"}
	return &authRepoStub{
		service:      servicesRepo.Service{ID: serviceID, Name: "service", Active: true},
		permissions:  []string{"manage_settings"},
		apiKeys:      []apiKeysRepo.ApiKey{apiKey},
		secretLookup: map[string]apiKeysRepo.ApiKey{apiKeySecret: apiKey},
	}, cfg.APIKeyPrefix + serviceID.String() + plaintextToken
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
