package service_contacts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apphandler "github.com/maxneuvians/notification-api-spec/internal/handler"
	servicesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	serviceerrs "github.com/maxneuvians/notification-api-spec/internal/service/services"
)

type Repository interface {
	GetSmsSenderByID(ctx context.Context, serviceID uuid.UUID, senderID uuid.UUID) (*servicesrepo.ServiceSmsSender, error)
	GetSmsSendersByServiceID(ctx context.Context, serviceID uuid.UUID) ([]servicesrepo.ServiceSmsSender, error)
	AddSmsSenderForService(ctx context.Context, sender servicesrepo.ServiceSmsSender) (*servicesrepo.ServiceSmsSender, error)
	UpdateServiceSmsSender(ctx context.Context, sender servicesrepo.ServiceSmsSender) (*servicesrepo.ServiceSmsSender, error)
	UpdateSmsSenderWithInboundNumber(ctx context.Context, serviceID, senderID, inboundNumberID uuid.UUID) (*servicesrepo.ServiceSmsSender, error)
	ArchiveSmsSender(ctx context.Context, serviceID, senderID uuid.UUID) (*servicesrepo.ServiceSmsSender, error)
	FetchServiceByID(ctx context.Context, id uuid.UUID, onlyActive bool) (*servicesrepo.Service, error)
	FetchServiceDataRetention(ctx context.Context, serviceID uuid.UUID) ([]servicesrepo.ServiceDataRetention, error)
	FetchServiceDataRetentionByID(ctx context.Context, serviceID, retentionID uuid.UUID) (*servicesrepo.ServiceDataRetention, error)
	FetchDataRetentionByNotificationType(ctx context.Context, serviceID uuid.UUID, notificationType servicesrepo.NotificationType) (*servicesrepo.ServiceDataRetention, error)
	InsertServiceDataRetention(ctx context.Context, retention servicesrepo.ServiceDataRetention) (*servicesrepo.ServiceDataRetention, error)
	UpdateServiceDataRetention(ctx context.Context, serviceID, retentionID uuid.UUID, daysOfRetention int32) (int64, error)
	FetchServiceSafelist(ctx context.Context, serviceID uuid.UUID) ([]servicesrepo.ServiceSafelist, error)
	AddSafelistedContacts(ctx context.Context, serviceID uuid.UUID, emailAddresses, phoneNumbers []string) error
	RemoveServiceSafelist(ctx context.Context, serviceID uuid.UUID) error
	GetReplyTosByServiceID(ctx context.Context, serviceID uuid.UUID) ([]servicesrepo.ServiceEmailReplyTo, error)
	GetReplyToByID(ctx context.Context, serviceID uuid.UUID, replyToID uuid.UUID) (*servicesrepo.ServiceEmailReplyTo, error)
	AddReplyToEmailAddress(ctx context.Context, replyTo servicesrepo.ServiceEmailReplyTo) (*servicesrepo.ServiceEmailReplyTo, error)
	UpdateReplyToEmailAddress(ctx context.Context, replyTo servicesrepo.ServiceEmailReplyTo) (*servicesrepo.ServiceEmailReplyTo, error)
	ArchiveReplyToEmailAddress(ctx context.Context, serviceID, replyToID uuid.UUID) (*servicesrepo.ServiceEmailReplyTo, error)
}

type Handler struct {
	repo Repository
}

type smsSenderRequest struct {
	SmsSender       string `json:"sms_sender"`
	IsDefault       bool   `json:"is_default"`
	InboundNumberID string `json:"inbound_number_id"`
	ServiceID       string `json:"service_id"`
	SmsSenderID     string `json:"sms_sender_id"`
}

type replyToRequest struct {
	ServiceID    string `json:"service_id"`
	ReplyToID    string `json:"reply_to_id"`
	EmailAddress string `json:"email_address"`
	IsDefault    bool   `json:"is_default"`
}

type dataRetentionRequest struct {
	NotificationType string `json:"notification_type"`
	DaysOfRetention  *int32 `json:"days_of_retention"`
}

type safelistRequest struct {
	EmailAddresses []string `json:"email_addresses"`
	PhoneNumbers   []string `json:"phone_numbers"`
}

type safelistResponse struct {
	EmailAddresses []string `json:"email_addresses"`
	PhoneNumbers   []string `json:"phone_numbers"`
}

var safelistPhonePattern = regexp.MustCompile(`^\+\d{8,15}$`)

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/{serviceID}/sms-sender", h.listSMSSenders)
	r.Get("/{serviceID}/sms-sender/{senderID}", h.getSMSSender)
	r.Post("/{serviceID}/sms-sender", h.addSMSSender)
	r.Post("/{serviceID}/sms-sender/{senderID}", h.updateSMSSender)
	r.Post("/delete_service_sms_sender", h.archiveSMSSender)

	r.Get("/{serviceID}/data-retention", h.listDataRetention)
	r.Get("/{serviceID}/data-retention/notification-type/{notificationType}", h.getDataRetentionByType)
	r.Get("/{serviceID}/data-retention/{retentionID}", h.getDataRetention)
	r.Post("/{serviceID}/data-retention", h.addDataRetention)
	r.Post("/{serviceID}/data-retention/{retentionID}", h.updateDataRetention)
	r.Get("/{serviceID}/safelist", h.getSafelist)
	r.Put("/{serviceID}/safelist", h.updateSafelist)

	r.Get("/{serviceID}/email-reply-to", h.listReplyTos)
	r.Get("/{serviceID}/email-reply-to/{replyToID}", h.getReplyTo)
	r.Post("/add_service_reply_to_email_address", h.addReplyTo)
	r.Post("/update_service_reply_to_email_address", h.updateReplyTo)
	r.Post("/delete_service_reply_to_email_address", h.archiveReplyTo)

	r.Get("/{serviceID}/letter-contact", h.notImplemented)
	r.Get("/{serviceID}/letter-contact/{letterContactID}", h.notImplemented)
	r.Post("/{serviceID}/letter-contact", h.notImplemented)
	r.Post("/{serviceID}/letter-contact/{letterContactID}", h.notImplemented)
	r.Post("/{serviceID}/letter-contact/{letterContactID}/archive", h.notImplemented)
}

func (h *Handler) listSMSSenders(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseURLUUID(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	items, err := h.repo.GetSmsSendersByServiceID(r.Context(), serviceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) getSMSSender(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseURLUUID(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	senderID, ok := parseURLUUID(w, chi.URLParam(r, "senderID"), "Invalid sender id")
	if !ok {
		return
	}
	item, err := h.repo.GetSmsSenderByID(r.Context(), serviceID, senderID)
	if err != nil {
		writeError(w, err)
		return
	}
	if item == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "SMS sender not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) addSMSSender(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseURLUUID(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	var payload smsSenderRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	sender := servicesrepo.ServiceSmsSender{ServiceID: serviceID, SmsSender: payload.SmsSender, IsDefault: payload.IsDefault}
	if payload.InboundNumberID != "" {
		inboundID, ok := parseBodyUUID(w, payload.InboundNumberID, "Invalid inbound_number_id")
		if !ok {
			return
		}
		sender.InboundNumberID = uuid.NullUUID{UUID: inboundID, Valid: true}
	}
	created, err := h.repo.AddSmsSenderForService(r.Context(), sender)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": created})
}

func (h *Handler) updateSMSSender(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseURLUUID(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	senderID, ok := parseURLUUID(w, chi.URLParam(r, "senderID"), "Invalid sender id")
	if !ok {
		return
	}
	var payload smsSenderRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	if payload.InboundNumberID != "" {
		inboundID, ok := parseBodyUUID(w, payload.InboundNumberID, "Invalid inbound_number_id")
		if !ok {
			return
		}
		updated, err := h.repo.UpdateSmsSenderWithInboundNumber(r.Context(), serviceID, senderID, inboundID)
		if err != nil {
			writeError(w, err)
			return
		}
		if updated == nil {
			apphandler.WriteAdminError(w, http.StatusNotFound, "SMS sender not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": updated})
		return
	}
	updated, err := h.repo.UpdateServiceSmsSender(r.Context(), servicesrepo.ServiceSmsSender{ID: senderID, ServiceID: serviceID, SmsSender: payload.SmsSender, IsDefault: payload.IsDefault})
	if err != nil {
		writeError(w, err)
		return
	}
	if updated == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "SMS sender not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": updated})
}

func (h *Handler) archiveSMSSender(w http.ResponseWriter, r *http.Request) {
	var payload smsSenderRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	serviceID, ok := parseBodyUUID(w, payload.ServiceID, "Invalid service id")
	if !ok {
		return
	}
	senderID, ok := parseBodyUUID(w, payload.SmsSenderID, "Invalid sender id")
	if !ok {
		return
	}
	updated, err := h.repo.ArchiveSmsSender(r.Context(), serviceID, senderID)
	if err != nil {
		writeError(w, err)
		return
	}
	if updated == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "SMS sender not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": updated})
}

func (h *Handler) listDataRetention(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseURLUUID(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	items, err := h.repo.FetchServiceDataRetention(r.Context(), serviceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) getDataRetention(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseURLUUID(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	retentionID, ok := parseURLUUID(w, chi.URLParam(r, "retentionID"), "Invalid retention id")
	if !ok {
		return
	}
	item, err := h.repo.FetchServiceDataRetentionByID(r.Context(), serviceID, retentionID)
	if err != nil {
		writeError(w, err)
		return
	}
	if item == nil {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) getDataRetentionByType(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseURLUUID(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	notificationType, ok := parseNotificationType(w, chi.URLParam(r, "notificationType"))
	if !ok {
		return
	}
	item, err := h.repo.FetchDataRetentionByNotificationType(r.Context(), serviceID, notificationType)
	if err != nil {
		writeError(w, err)
		return
	}
	if item == nil {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) addDataRetention(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseURLUUID(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	var payload dataRetentionRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	if payload.DaysOfRetention == nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "days_of_retention is required")
		return
	}
	notificationType, ok := parseNotificationType(w, payload.NotificationType)
	if !ok {
		return
	}
	created, err := h.repo.InsertServiceDataRetention(r.Context(), servicesrepo.ServiceDataRetention{ServiceID: serviceID, NotificationType: notificationType, DaysOfRetention: *payload.DaysOfRetention})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"result": created})
}

func (h *Handler) updateDataRetention(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseURLUUID(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	retentionID, ok := parseURLUUID(w, chi.URLParam(r, "retentionID"), "Invalid retention id")
	if !ok {
		return
	}
	var payload dataRetentionRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	if payload.DaysOfRetention == nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "days_of_retention is required")
		return
	}
	count, err := h.repo.UpdateServiceDataRetention(r.Context(), serviceID, retentionID, *payload.DaysOfRetention)
	if err != nil {
		writeError(w, err)
		return
	}
	if count == 0 {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getSafelist(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseURLUUID(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	service, err := h.repo.FetchServiceByID(r.Context(), serviceID, false)
	if err != nil {
		writeError(w, err)
		return
	}
	if service == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}
	items, err := h.repo.FetchServiceSafelist(r.Context(), serviceID)
	if err != nil {
		writeError(w, err)
		return
	}
	response := safelistResponse{EmailAddresses: []string{}, PhoneNumbers: []string{}}
	for _, item := range items {
		switch item.RecipientType {
		case servicesrepo.RecipientTypeEmail:
			response.EmailAddresses = append(response.EmailAddresses, item.Recipient)
		case servicesrepo.RecipientTypeMobile:
			response.PhoneNumbers = append(response.PhoneNumbers, item.Recipient)
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) updateSafelist(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseURLUUID(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	service, err := h.repo.FetchServiceByID(r.Context(), serviceID, false)
	if err != nil {
		writeError(w, err)
		return
	}
	if service == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}
	var payload safelistRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	if invalid, ok := invalidSafelistValue(payload.EmailAddresses, isValidEmailAddress); ok {
		apphandler.WriteAdminError(w, http.StatusBadRequest, invalidSafelistMessage(invalid))
		return
	}
	if invalid, ok := invalidSafelistValue(payload.PhoneNumbers, isValidPhoneNumber); ok {
		apphandler.WriteAdminError(w, http.StatusBadRequest, invalidSafelistMessage(invalid))
		return
	}
	if err := h.repo.AddSafelistedContacts(r.Context(), serviceID, payload.EmailAddresses, payload.PhoneNumbers); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listReplyTos(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseURLUUID(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	items, err := h.repo.GetReplyTosByServiceID(r.Context(), serviceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) getReplyTo(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseURLUUID(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	replyToID, ok := parseURLUUID(w, chi.URLParam(r, "replyToID"), "Invalid reply_to id")
	if !ok {
		return
	}
	item, err := h.repo.GetReplyToByID(r.Context(), serviceID, replyToID)
	if err != nil {
		writeError(w, err)
		return
	}
	if item == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "Reply to email address not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) addReplyTo(w http.ResponseWriter, r *http.Request) {
	var payload replyToRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	serviceID, ok := parseBodyUUID(w, payload.ServiceID, "Invalid service id")
	if !ok {
		return
	}
	created, err := h.repo.AddReplyToEmailAddress(r.Context(), servicesrepo.ServiceEmailReplyTo{ServiceID: serviceID, EmailAddress: payload.EmailAddress, IsDefault: payload.IsDefault})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": created})
}

func (h *Handler) updateReplyTo(w http.ResponseWriter, r *http.Request) {
	var payload replyToRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	serviceID, ok := parseBodyUUID(w, payload.ServiceID, "Invalid service id")
	if !ok {
		return
	}
	replyToID, ok := parseBodyUUID(w, payload.ReplyToID, "Invalid reply_to id")
	if !ok {
		return
	}
	updated, err := h.repo.UpdateReplyToEmailAddress(r.Context(), servicesrepo.ServiceEmailReplyTo{ID: replyToID, ServiceID: serviceID, EmailAddress: payload.EmailAddress, IsDefault: payload.IsDefault})
	if err != nil {
		writeError(w, err)
		return
	}
	if updated == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "Reply to email address not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": updated})
}

func (h *Handler) archiveReplyTo(w http.ResponseWriter, r *http.Request) {
	var payload replyToRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	serviceID, ok := parseBodyUUID(w, payload.ServiceID, "Invalid service id")
	if !ok {
		return
	}
	replyToID, ok := parseBodyUUID(w, payload.ReplyToID, "Invalid reply_to id")
	if !ok {
		return
	}
	updated, err := h.repo.ArchiveReplyToEmailAddress(r.Context(), serviceID, replyToID)
	if err != nil {
		writeError(w, err)
		return
	}
	if updated == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "Reply to email address not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": updated})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid JSON")
		return false
	}
	return true
}

func parseURLUUID(w http.ResponseWriter, value string, message string) (uuid.UUID, bool) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, message)
		return uuid.UUID{}, false
	}
	return parsed, true
}

func parseBodyUUID(w http.ResponseWriter, value string, message string) (uuid.UUID, bool) {
	return parseURLUUID(w, value, message)
}

func parseNotificationType(w http.ResponseWriter, value string) (servicesrepo.NotificationType, bool) {
	notificationType := servicesrepo.NotificationType(strings.TrimSpace(value))
	if !notificationType.Valid() {
		apphandler.WriteAdminError(w, http.StatusBadRequest, invalidNotificationTypeMessage(value))
		return "", false
	}
	return notificationType, true
}

func writeError(w http.ResponseWriter, err error) {
	var invalidErr serviceerrs.InvalidRequestError
	if errors.As(err, &invalidErr) {
		writeJSON(w, invalidErr.StatusCode, invalidErr.Body())
		return
	}
	apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
}

func (h *Handler) notImplemented(w http.ResponseWriter, _ *http.Request) {
	apphandler.WriteAdminError(w, http.StatusNotImplemented, "Not implemented")
}

func invalidNotificationTypeMessage(value string) string {
	return "notification_type " + value + " is not one of [sms, letter, email]"
}

func invalidSafelistMessage(value string) string {
	return "Invalid safelist: \"" + value + "\" is not a valid email address or phone number"
}

func invalidSafelistValue(values []string, validate func(string) bool) (string, bool) {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if !validate(trimmed) {
			return trimmed, true
		}
	}
	return "", false
}

func isValidEmailAddress(value string) bool {
	if value == "" {
		return false
	}
	_, err := mail.ParseAddress(value)
	return err == nil
}

func isValidPhoneNumber(value string) bool {
	if value == "" {
		return false
	}
	return safelistPhonePattern.MatchString(value)
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
