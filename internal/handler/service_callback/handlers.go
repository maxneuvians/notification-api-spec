package service_callback

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apphandler "github.com/maxneuvians/notification-api-spec/internal/handler"
	servicesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
)

type Repository interface {
	SaveServiceCallbackApi(ctx context.Context, serviceID uuid.UUID, callbackType string, url string, bearerToken string, updatedByID uuid.UUID) (*servicesrepo.ServiceCallbackApi, error)
	ResetServiceCallbackApi(ctx context.Context, serviceID, callbackID uuid.UUID, updatedByID uuid.UUID, url *string, bearerToken *string) (*servicesrepo.ServiceCallbackApi, error)
	GetServiceCallbackApi(ctx context.Context, serviceID, callbackID uuid.UUID) (*servicesrepo.ServiceCallbackApi, error)
	DeleteServiceCallbackApi(ctx context.Context, serviceID, callbackID uuid.UUID) (bool, error)
	SuspendUnsuspendCallbackApi(ctx context.Context, serviceID uuid.UUID, updatedByID uuid.UUID, suspend bool) (*servicesrepo.ServiceCallbackApi, error)
	SaveServiceInboundApi(ctx context.Context, serviceID uuid.UUID, url string, bearerToken string, updatedByID uuid.UUID) (*servicesrepo.ServiceInboundApi, error)
	ResetServiceInboundApi(ctx context.Context, serviceID, inboundID uuid.UUID, updatedByID uuid.UUID, url *string, bearerToken *string) (*servicesrepo.ServiceInboundApi, error)
	GetServiceInboundApi(ctx context.Context, serviceID, inboundID uuid.UUID) (*servicesrepo.ServiceInboundApi, error)
	DeleteServiceInboundApi(ctx context.Context, serviceID, inboundID uuid.UUID) (bool, error)
}

type Handler struct {
	repo Repository
}

type createAPIRequest struct {
	URL         string `json:"url"`
	BearerToken string `json:"bearer_token"`
	UpdatedByID string `json:"updated_by_id"`
}

type updateAPIRequest struct {
	URL         *string `json:"url"`
	BearerToken *string `json:"bearer_token"`
	UpdatedByID string  `json:"updated_by_id"`
}

type suspendCallbackRequest struct {
	UpdatedByID      string `json:"updated_by_id"`
	SuspendUnsuspend bool   `json:"suspend_unsuspend"`
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/{serviceID}/inbound-api", h.createInboundAPI)
	r.Post("/{serviceID}/inbound-api/{inboundAPIID}", h.updateInboundAPI)
	r.Get("/{serviceID}/inbound-api/{inboundAPIID}", h.getInboundAPI)
	r.Delete("/{serviceID}/inbound-api/{inboundAPIID}", h.deleteInboundAPI)

	r.Post("/{serviceID}/delivery-receipt-api", h.createDeliveryReceiptAPI)
	r.Post("/{serviceID}/delivery-receipt-api/{callbackAPIID}", h.updateDeliveryReceiptAPI)
	r.Get("/{serviceID}/delivery-receipt-api/{callbackAPIID}", h.getDeliveryReceiptAPI)
	r.Delete("/{serviceID}/delivery-receipt-api/{callbackAPIID}", h.deleteDeliveryReceiptAPI)
	r.Post("/{serviceID}/delivery-receipt-api/suspend-callback", h.suspendDeliveryReceiptAPI)
}

func (h *Handler) createInboundAPI(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	var payload createAPIRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	updatedByID, ok := parseUUIDParam(w, payload.UpdatedByID, "Invalid updated_by_id")
	if !ok {
		return
	}
	if !validateCreateAPIRequest(w, payload) {
		return
	}
	created, err := h.repo.SaveServiceInboundApi(r.Context(), serviceID, payload.URL, payload.BearerToken, updatedByID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) updateInboundAPI(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	inboundID, ok := parseUUIDParam(w, chi.URLParam(r, "inboundAPIID"), "Invalid inbound api id")
	if !ok {
		return
	}
	var payload updateAPIRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	updatedByID, ok := parseUUIDParam(w, payload.UpdatedByID, "Invalid updated_by_id")
	if !ok {
		return
	}
	if !validateUpdateAPIRequest(w, payload) {
		return
	}
	updated, err := h.repo.ResetServiceInboundApi(r.Context(), serviceID, inboundID, updatedByID, payload.URL, payload.BearerToken)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if updated == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) getInboundAPI(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	inboundID, ok := parseUUIDParam(w, chi.URLParam(r, "inboundAPIID"), "Invalid inbound api id")
	if !ok {
		return
	}
	item, err := h.repo.GetServiceInboundApi(r.Context(), serviceID, inboundID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if item == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) deleteInboundAPI(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	inboundID, ok := parseUUIDParam(w, chi.URLParam(r, "inboundAPIID"), "Invalid inbound api id")
	if !ok {
		return
	}
	deleted, err := h.repo.DeleteServiceInboundApi(r.Context(), serviceID, inboundID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if !deleted {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createDeliveryReceiptAPI(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	var payload createAPIRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	updatedByID, ok := parseUUIDParam(w, payload.UpdatedByID, "Invalid updated_by_id")
	if !ok {
		return
	}
	if !validateCreateAPIRequest(w, payload) {
		return
	}
	created, err := h.repo.SaveServiceCallbackApi(r.Context(), serviceID, "delivery_status", payload.URL, payload.BearerToken, updatedByID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) updateDeliveryReceiptAPI(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	callbackID, ok := parseUUIDParam(w, chi.URLParam(r, "callbackAPIID"), "Invalid callback api id")
	if !ok {
		return
	}
	var payload updateAPIRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	updatedByID, ok := parseUUIDParam(w, payload.UpdatedByID, "Invalid updated_by_id")
	if !ok {
		return
	}
	if !validateUpdateAPIRequest(w, payload) {
		return
	}
	updated, err := h.repo.ResetServiceCallbackApi(r.Context(), serviceID, callbackID, updatedByID, payload.URL, payload.BearerToken)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if updated == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) getDeliveryReceiptAPI(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	callbackID, ok := parseUUIDParam(w, chi.URLParam(r, "callbackAPIID"), "Invalid callback api id")
	if !ok {
		return
	}
	item, err := h.repo.GetServiceCallbackApi(r.Context(), serviceID, callbackID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if item == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) deleteDeliveryReceiptAPI(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	callbackID, ok := parseUUIDParam(w, chi.URLParam(r, "callbackAPIID"), "Invalid callback api id")
	if !ok {
		return
	}
	deleted, err := h.repo.DeleteServiceCallbackApi(r.Context(), serviceID, callbackID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if !deleted {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) suspendDeliveryReceiptAPI(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := parseUUIDParam(w, chi.URLParam(r, "serviceID"), "Invalid service id")
	if !ok {
		return
	}
	var payload suspendCallbackRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	updatedByID, ok := parseUUIDParam(w, payload.UpdatedByID, "Invalid updated_by_id")
	if !ok {
		return
	}
	updated, err := h.repo.SuspendUnsuspendCallbackApi(r.Context(), serviceID, updatedByID, payload.SuspendUnsuspend)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if updated == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "No result found")
		return
	}
	writeJSON(w, http.StatusOK, updated)
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

func validateCreateAPIRequest(w http.ResponseWriter, payload createAPIRequest) bool {
	if !isValidHTTPSURL(payload.URL) {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "url is not a valid https url")
		return false
	}
	if len(payload.BearerToken) < 10 {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "bearer_token "+payload.BearerToken+" is too short")
		return false
	}
	return true
}

func validateUpdateAPIRequest(w http.ResponseWriter, payload updateAPIRequest) bool {
	if payload.URL != nil && !isValidHTTPSURL(*payload.URL) && *payload.URL != "" {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "url is not a valid https url")
		return false
	}
	if payload.BearerToken != nil && len(*payload.BearerToken) < 10 && *payload.BearerToken != "" {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "bearer_token "+*payload.BearerToken+" is too short")
		return false
	}
	return true
}

func isValidHTTPSURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return parsed.Scheme == "https" && parsed.Host != ""
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
