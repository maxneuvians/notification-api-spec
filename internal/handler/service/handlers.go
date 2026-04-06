package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apphandler "github.com/maxneuvians/notification-api-spec/internal/handler"
	apiKeysRepo "github.com/maxneuvians/notification-api-spec/internal/repository/api_keys"
	organisationsRepo "github.com/maxneuvians/notification-api-spec/internal/repository/organisations"
	servicesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	usersRepo "github.com/maxneuvians/notification-api-spec/internal/repository/users"
)

type Repository interface {
	FetchAllServices(ctx context.Context, onlyActive bool) ([]servicesrepo.Service, error)
	FetchServicesByUserID(ctx context.Context, userID uuid.UUID, onlyActive bool) ([]servicesrepo.Service, error)
	CreateService(ctx context.Context, service servicesrepo.Service, userID *uuid.UUID, permissions []string) (*servicesrepo.Service, error)
	FetchUserByID(ctx context.Context, userID uuid.UUID) (*usersRepo.User, error)
	FetchOrganisationByEmailAddress(ctx context.Context, emailAddress string) (*organisationsRepo.Organisation, error)
	FetchNHSEmailBrandingID(ctx context.Context) (*uuid.UUID, error)
	FetchNHSLetterBrandingID(ctx context.Context) (*uuid.UUID, error)
	AssignServiceBranding(ctx context.Context, serviceID uuid.UUID, emailBrandingID *uuid.UUID, letterBrandingID *uuid.UUID) error
	GetServicesByPartialName(ctx context.Context, nameQuery string) ([]servicesrepo.Service, error)
	FetchStatsForAllServices(ctx context.Context, includeFromTestKey bool, onlyActive bool, startDate time.Time, endDate time.Time) ([]servicesrepo.TodayStatsForAllServicesRow, error)
	FetchServiceByID(ctx context.Context, id uuid.UUID, onlyActive bool) (*servicesrepo.Service, error)
	FetchServiceHistory(ctx context.Context, serviceID uuid.UUID) ([]servicesrepo.ServicesHistory, error)
	FetchAPIKeyHistory(ctx context.Context, serviceID uuid.UUID) ([]apiKeysRepo.APIKeyHistory, error)
	IsServiceNameUnique(ctx context.Context, name string, serviceID uuid.UUID) (bool, error)
	IsServiceEmailFromUnique(ctx context.Context, emailFrom string, serviceID uuid.UUID) (bool, error)
	FetchServiceOrganisation(ctx context.Context, serviceID uuid.UUID) (*organisationsRepo.Organisation, error)
	FetchStatsForService(ctx context.Context, serviceID uuid.UUID, limitDays int) ([]servicesrepo.ServiceStatsRow, error)
	FetchTodaysStatsForService(ctx context.Context, serviceID uuid.UUID) ([]servicesrepo.ServiceStatsRow, error)
	FetchLiveServicesData(ctx context.Context) ([]servicesrepo.GetLiveServicesDataRow, error)
	FetchSensitiveServiceIDs(ctx context.Context) ([]uuid.UUID, error)
	FetchAnnualLimitStats(ctx context.Context, serviceID uuid.UUID) (*servicesrepo.AnnualLimitStats, error)
	FetchMonthlyUsageForService(ctx context.Context, serviceID uuid.UUID, fiscalYearStart int) ([]servicesrepo.MonthlyUsageRow, error)
	SuspendService(ctx context.Context, id uuid.UUID, userID *uuid.UUID) (*servicesrepo.Service, error)
	ResumeService(ctx context.Context, id uuid.UUID) (*servicesrepo.Service, error)
	ArchiveService(ctx context.Context, id uuid.UUID) (*servicesrepo.Service, error)
}

type Handler struct {
	repo Repository
}

type suspendRequest struct {
	UserID string `json:"user_id"`
}

type createServiceRequest struct {
	Name         *string `json:"name"`
	UserID       *string `json:"user_id"`
	MessageLimit *int64  `json:"message_limit"`
	Restricted   *bool   `json:"restricted"`
	EmailFrom    *string `json:"email_from"`
	CreatedBy    *string `json:"created_by"`
}

type serviceListItem struct {
	servicesrepo.Service
	Statistics *statisticsResponse `json:"statistics,omitempty"`
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.createService)
	r.Get("/", h.listServices)
	r.Get("/find-by-name", h.findByName)
	r.Get("/find-services-by-name", h.findByName)
	r.Get("/live-services", h.liveServices)
	r.Get("/sensitive-service-ids", h.sensitiveServiceIDs)
	r.Get("/is-name-unique", h.isNameUnique)
	r.Get("/is-email-from-unique", h.isEmailFromUnique)
	r.Get("/{serviceID}/history", h.serviceHistory)
	r.Get("/{serviceID}/organisation", h.serviceOrganisation)
	r.Get("/{serviceID}/statistics", h.serviceStatistics)
	r.Get("/{serviceID}/monthly-usage", h.monthlyUsage)
	r.Get("/{serviceID}/annual-limit-stats", h.annualLimitStats)
	r.Post("/{serviceID}/archive", h.archiveService)
	r.Post("/{serviceID}/suspend", h.suspendService)
	r.Post("/{serviceID}/resume", h.resumeService)
}

func (h *Handler) createService(w http.ResponseWriter, r *http.Request) {
	var payload createServiceRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	if missing := validateCreateServiceRequest(payload); len(missing) > 0 {
		writeJSON(w, http.StatusBadRequest, missing)
		return
	}

	userID, ok := parseUUIDParam(w, *payload.UserID, "Invalid user_id")
	if !ok {
		return
	}
	if _, ok := parseUUIDParam(w, *payload.CreatedBy, "Invalid created_by"); !ok {
		return
	}

	name := strings.TrimSpace(*payload.Name)
	emailFrom := strings.TrimSpace(*payload.EmailFrom)
	nameUnique, err := h.repo.IsServiceNameUnique(r.Context(), name, uuid.Nil)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if !nameUnique {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Duplicate service name '"+name+"'")
		return
	}
	emailUnique, err := h.repo.IsServiceEmailFromUnique(r.Context(), emailFrom, uuid.Nil)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if !emailUnique {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Duplicate service name '"+emailFrom+"'")
		return
	}

	user, err := h.repo.FetchUserByID(r.Context(), userID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if user == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}

	service := servicesrepo.Service{
		Name:             name,
		Active:           true,
		MessageLimit:     *payload.MessageLimit,
		Restricted:       *payload.Restricted,
		EmailFrom:        emailFrom,
		Version:          1,
		ResearchMode:     false,
		PrefixSms:        true,
		RateLimit:        1000,
		CountAsLive:      !user.PlatformAdmin,
		SmsDailyLimit:    1000,
		EmailAnnualLimit: 20000000,
		SmsAnnualLimit:   100000,
	}

	organisation, err := h.repo.FetchOrganisationByEmailAddress(r.Context(), user.EmailAddress)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	var nhsEmailBrandingID *uuid.UUID
	var nhsLetterBrandingID *uuid.UUID
	if organisation != nil {
		service.OrganisationID = uuid.NullUUID{UUID: organisation.ID, Valid: true}
		service.OrganisationType = organisation.OrganisationType
		service.Crown = organisation.Crown
	} else if isNHSEmail(user.EmailAddress) {
		nhsEmailBrandingID, err = h.repo.FetchNHSEmailBrandingID(r.Context())
		if err != nil {
			apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		nhsLetterBrandingID, err = h.repo.FetchNHSLetterBrandingID(r.Context())
		if err != nil {
			apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
	}

	created, err := h.repo.CreateService(r.Context(), service, &userID, nil)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if created == nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if nhsEmailBrandingID != nil || nhsLetterBrandingID != nil {
		if err := h.repo.AssignServiceBranding(r.Context(), created.ID, nhsEmailBrandingID, nhsLetterBrandingID); err != nil {
			apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": created})
}

func (h *Handler) listServices(w http.ResponseWriter, r *http.Request) {
	onlyActive, ok := parseBoolQuery(w, r.URL.Query().Get("only_active"), "Invalid only_active")
	if !ok {
		return
	}
	detailed, ok := parseBoolQuery(w, r.URL.Query().Get("detailed"), "Invalid detailed")
	if !ok {
		return
	}
	includeFromTestKey, ok := parseBoolWithDefault(w, r.URL.Query().Get("include_from_test_key"), true, "Invalid include_from_test_key")
	if !ok {
		return
	}
	userID, hasUserID, ok := parseOptionalUUIDQuery(w, r.URL.Query().Get("user_id"), "Invalid user_id")
	if !ok {
		return
	}

	var services []servicesrepo.Service
	var err error
	if hasUserID {
		services, err = h.repo.FetchServicesByUserID(r.Context(), userID, onlyActive)
	} else {
		services, err = h.repo.FetchAllServices(r.Context(), onlyActive)
	}
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if !detailed {
		writeJSON(w, http.StatusOK, map[string]any{"data": services})
		return
	}

	startDate, endDate, ok := parseDateRangeQuery(w, r)
	if !ok {
		return
	}
	rows, err := h.repo.FetchStatsForAllServices(r.Context(), includeFromTestKey, onlyActive, startDate, endDate)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	statsByService := buildServiceStatistics(rows)
	items := make([]serviceListItem, 0, len(services))
	for _, service := range services {
		stats := statsByService[service.ID]
		statsCopy := stats
		items = append(items, serviceListItem{Service: service, Statistics: &statsCopy})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) findByName(w http.ResponseWriter, r *http.Request) {
	serviceName := strings.TrimSpace(r.URL.Query().Get("service_name"))
	if serviceName == "" {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Missing service_name")
		return
	}
	items, err := h.repo.GetServicesByPartialName(r.Context(), serviceName)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) liveServices(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.FetchLiveServicesData(r.Context())
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) sensitiveServiceIDs(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.FetchSensitiveServiceIDs(r.Context())
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) isNameUnique(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	serviceIDText := strings.TrimSpace(r.URL.Query().Get("service_id"))
	if name == "" || serviceIDText == "" {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Missing name or service_id")
		return
	}
	serviceID, ok := parseUUIDParam(w, serviceIDText, "Invalid service_id")
	if !ok {
		return
	}
	unique, err := h.repo.IsServiceNameUnique(r.Context(), name, serviceID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": unique})
}

func (h *Handler) isEmailFromUnique(w http.ResponseWriter, r *http.Request) {
	emailFrom := strings.TrimSpace(r.URL.Query().Get("email_from"))
	serviceIDText := strings.TrimSpace(r.URL.Query().Get("service_id"))
	if emailFrom == "" || serviceIDText == "" {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Missing email_from or service_id")
		return
	}
	serviceID, ok := parseUUIDParam(w, serviceIDText, "Invalid service_id")
	if !ok {
		return
	}
	unique, err := h.repo.IsServiceEmailFromUnique(r.Context(), emailFrom, serviceID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": unique})
}

func (h *Handler) serviceHistory(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	serviceHistory, err := h.repo.FetchServiceHistory(r.Context(), serviceID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	apiKeyHistory, err := h.repo.FetchAPIKeyHistory(r.Context(), serviceID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"service_history":  serviceHistory,
		"api_key_history":  apiKeyHistory,
		"template_history": []any{},
		"events":           []any{},
	}})
}

func (h *Handler) serviceOrganisation(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	item, err := h.repo.FetchServiceOrganisation(r.Context(), serviceID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if item == nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": item})
}

func (h *Handler) serviceStatistics(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	todayOnly, ok := parseBoolQuery(w, r.URL.Query().Get("today_only"), "Invalid today_only")
	if !ok {
		return
	}
	var rows []servicesrepo.ServiceStatsRow
	var err error
	if todayOnly {
		rows, err = h.repo.FetchTodaysStatsForService(r.Context(), serviceID)
	} else {
		rows, err = h.repo.FetchStatsForService(r.Context(), serviceID, 7)
	}
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": buildStatisticsResponse(rows)})
}

func (h *Handler) monthlyUsage(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	yearText := strings.TrimSpace(r.URL.Query().Get("year"))
	if yearText == "" {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Year must be a number")
		return
	}
	year, err := strconv.Atoi(yearText)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Year must be a number")
		return
	}
	service, err := h.repo.FetchServiceByID(r.Context(), serviceID, false)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if service == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}
	rows, err := h.repo.FetchMonthlyUsageForService(r.Context(), serviceID, year)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": buildMonthlyUsageResponse(year, rows)})
}

func (h *Handler) annualLimitStats(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	stats, err := h.repo.FetchAnnualLimitStats(r.Context(), serviceID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if stats == nil {
		stats = &servicesrepo.AnnualLimitStats{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": stats})
}

func (h *Handler) archiveService(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	updated, err := h.repo.ArchiveService(r.Context(), serviceID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if updated == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) suspendService(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	payload, ok := decodeSuspendRequest(w, r)
	if !ok {
		return
	}
	var userID *uuid.UUID
	if payload.UserID != "" {
		parsed, parsedOK := parseUUIDParam(w, payload.UserID, "Invalid user_id")
		if !parsedOK {
			return
		}
		userID = &parsed
	}
	updated, err := h.repo.SuspendService(r.Context(), serviceID, userID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if updated == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) resumeService(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	updated, err := h.repo.ResumeService(r.Context(), serviceID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if updated == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeSuspendRequest(w http.ResponseWriter, r *http.Request) (suspendRequest, bool) {
	if r.Body == nil || r.ContentLength == 0 {
		return suspendRequest{}, true
	}
	defer r.Body.Close()
	var payload suspendRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		if err == io.EOF {
			return suspendRequest{}, true
		}
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid JSON")
		return suspendRequest{}, false
	}
	return payload, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid JSON")
		return false
	}
	return true
}

func parseUUIDParam(w http.ResponseWriter, value string, message string) (uuid.UUID, bool) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, message)
		return uuid.UUID{}, false
	}
	return parsed, true
}

func parseBoolQuery(w http.ResponseWriter, value string, message string) (bool, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false, true
	}
	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, message)
		return false, false
	}
	return parsed, true
}

func parseBoolWithDefault(w http.ResponseWriter, value string, defaultValue bool, message string) (bool, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultValue, true
	}
	return parseBoolQuery(w, trimmed, message)
}

func parseOptionalUUIDQuery(w http.ResponseWriter, value string, message string) (uuid.UUID, bool, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return uuid.UUID{}, false, true
	}
	parsed, ok := parseUUIDParam(w, trimmed, message)
	if !ok {
		return uuid.UUID{}, false, false
	}
	return parsed, true, true
}

func parseDateRangeQuery(w http.ResponseWriter, r *http.Request) (time.Time, time.Time, bool) {
	location := loadTorontoLocation()
	now := time.Now().In(location)
	startDate, ok := parseDateOrDefault(w, r.URL.Query().Get("start_date"), now, "start_date")
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	endDate, ok := parseDateOrDefault(w, r.URL.Query().Get("end_date"), now, "end_date")
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	if endDate.Before(startDate) {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Start date must be before end date")
		return time.Time{}, time.Time{}, false
	}
	return startDate, endDate, true
}

func parseDateOrDefault(w http.ResponseWriter, value string, defaultDate time.Time, fieldName string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Date(defaultDate.Year(), defaultDate.Month(), defaultDate.Day(), 0, 0, 0, 0, defaultDate.Location()), true
	}
	parsed, err := time.ParseInLocation("2006-01-02", trimmed, loadTorontoLocation())
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, fieldName+" time data "+trimmed+" does not match format %Y-%m-%d")
		return time.Time{}, false
	}
	return parsed, true
}

func loadTorontoLocation() *time.Location {
	location, err := time.LoadLocation("America/Toronto")
	if err != nil {
		return time.UTC
	}
	return location
}

func validateCreateServiceRequest(payload createServiceRequest) map[string]any {
	missing := map[string]any{}
	if payload.Name == nil {
		missing["name"] = []string{"Missing data for required field."}
	}
	if payload.UserID == nil {
		missing["user_id"] = []string{"Missing data for required field."}
	}
	if payload.MessageLimit == nil {
		missing["message_limit"] = []string{"Missing data for required field."}
	}
	if payload.Restricted == nil {
		missing["restricted"] = []string{"Missing data for required field."}
	}
	if payload.EmailFrom == nil {
		missing["email_from"] = []string{"Missing data for required field."}
	}
	if payload.CreatedBy == nil {
		missing["created_by"] = []string{"Missing data for required field."}
	}
	return missing
}

func isNHSEmail(value string) bool {
	domain := strings.ToLower(strings.TrimSpace(extractEmailDomain(value)))
	return domain == "nhs.uk" || strings.HasSuffix(domain, ".nhs.uk") || domain == "nhs.net" || strings.HasSuffix(domain, ".nhs.net")
}

func extractEmailDomain(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

type statisticsCounts struct {
	Requested int64 `json:"requested"`
	Delivered int64 `json:"delivered"`
	Failed    int64 `json:"failed"`
}

type statisticsResponse struct {
	SMS    statisticsCounts `json:"sms"`
	Email  statisticsCounts `json:"email"`
	Letter statisticsCounts `json:"letter"`
}

func buildStatisticsResponse(rows []servicesrepo.ServiceStatsRow) statisticsResponse {
	response := statisticsResponse{}
	for _, row := range rows {
		bucket := selectStatisticsBucket(&response, row.NotificationType)
		bucket.Requested += row.Count
		if !row.NotificationStatus.Valid {
			continue
		}
		switch row.NotificationStatus.NotifyStatusType {
		case servicesrepo.NotifyStatusTypeDelivered:
			bucket.Delivered += row.Count
		case servicesrepo.NotifyStatusTypeCreated,
			servicesrepo.NotifyStatusTypeSending,
			servicesrepo.NotifyStatusTypePending,
			servicesrepo.NotifyStatusTypeSent,
			servicesrepo.NotifyStatusTypeTechnicalFailure,
			servicesrepo.NotifyStatusTypeTemporaryFailure,
			servicesrepo.NotifyStatusTypePermanentFailure,
			servicesrepo.NotifyStatusTypeFailed:
			if isFailedStatus(row.NotificationStatus.NotifyStatusType) {
				bucket.Failed += row.Count
			}
		}
	}
	return response
}

func buildServiceStatistics(rows []servicesrepo.TodayStatsForAllServicesRow) map[uuid.UUID]statisticsResponse {
	items := make(map[uuid.UUID]statisticsResponse)
	for _, row := range rows {
		current := items[row.ServiceID]
		bucket := selectStatisticsBucket(&current, row.NotificationType.NotificationType)
		bucket.Requested += row.Count
		if row.NotificationStatus.Valid {
			switch row.NotificationStatus.NotifyStatusType {
			case servicesrepo.NotifyStatusTypeDelivered:
				bucket.Delivered += row.Count
			default:
				if isFailedStatus(row.NotificationStatus.NotifyStatusType) {
					bucket.Failed += row.Count
				}
			}
		}
		items[row.ServiceID] = current
	}
	return items
}

func buildMonthlyUsageResponse(fiscalYearStart int, rows []servicesrepo.MonthlyUsageRow) map[string]any {
	response := make(map[string]any, 12)
	monthBuckets := make(map[string]map[string]statisticsCounts, 12)
	start := time.Date(fiscalYearStart, time.April, 1, 0, 0, 0, 0, time.UTC)
	for offset := 0; offset < 12; offset++ {
		monthKey := start.AddDate(0, offset, 0).Format("2006-01")
		monthBuckets[monthKey] = map[string]statisticsCounts{}
		response[monthKey] = monthBuckets[monthKey]
	}
	for _, row := range rows {
		bucketByType, ok := monthBuckets[row.Month]
		if !ok {
			continue
		}
		channelKey := monthlyChannelKey(row.NotificationType.NotificationType)
		counts := bucketByType[channelKey]
		counts.Requested += row.Count
		if row.NotificationStatus.Valid {
			switch row.NotificationStatus.NotifyStatusType {
			case servicesrepo.NotifyStatusTypeDelivered:
				counts.Delivered += row.Count
			default:
				if isFailedStatus(row.NotificationStatus.NotifyStatusType) {
					counts.Failed += row.Count
				}
			}
		}
		bucketByType[channelKey] = counts
	}
	return response
}

func monthlyChannelKey(notificationType servicesrepo.NotificationType) string {
	switch notificationType {
	case servicesrepo.NotificationTypeEmail:
		return "email"
	case servicesrepo.NotificationTypeLetter:
		return "letter"
	default:
		return "sms"
	}
}

func selectStatisticsBucket(response *statisticsResponse, notificationType servicesrepo.NotificationType) *statisticsCounts {
	switch notificationType {
	case servicesrepo.NotificationTypeEmail:
		return &response.Email
	case servicesrepo.NotificationTypeLetter:
		return &response.Letter
	default:
		return &response.SMS
	}
}

func isFailedStatus(status servicesrepo.NotifyStatusType) bool {
	switch status {
	case servicesrepo.NotifyStatusTypeFailed,
		servicesrepo.NotifyStatusTypeTechnicalFailure,
		servicesrepo.NotifyStatusTypeTemporaryFailure,
		servicesrepo.NotifyStatusTypePermanentFailure:
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
