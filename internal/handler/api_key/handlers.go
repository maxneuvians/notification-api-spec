package api_key

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apphandler "github.com/maxneuvians/notification-api-spec/internal/handler"
	apiKeysRepo "github.com/maxneuvians/notification-api-spec/internal/repository/api_keys"
	servicesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
)

type Repository interface {
	CreateAPIKey(ctx context.Context, serviceID uuid.UUID, name string, createdByID uuid.UUID, keyType string) (*servicesrepo.CreatedAPIKey, error)
	ListAPIKeys(ctx context.Context, serviceID uuid.UUID, keyID *uuid.UUID) ([]apiKeysRepo.ApiKey, error)
	RevokeAPIKey(ctx context.Context, serviceID uuid.UUID, keyID uuid.UUID) (*apiKeysRepo.ApiKey, error)
}

type Handler struct {
	repo Repository
}

type createAPIKeyRequest struct {
	Name      string `json:"name"`
	CreatedBy string `json:"created_by"`
	KeyType   string `json:"key_type"`
}

type apiKeyJSON struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	ServiceID  uuid.UUID  `json:"service_id"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiryDate *time.Time `json:"expiry_date,omitempty"`
	Key        string     `json:"key,omitempty"`
}

type createAPIKeyResponse struct {
	Key     string `json:"key"`
	KeyName string `json:"key_name"`
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/{serviceID}/api-key", h.createAPIKey)
	r.Get("/{serviceID}/api-keys", h.listAPIKeys)
	r.Post("/{serviceID}/api-key/{keyID}/revoke", h.revokeAPIKey)
}

func (h *Handler) createAPIKey(w http.ResponseWriter, r *http.Request) {
	serviceID, err := parseUUIDParam(chi.URLParam(r, "serviceID"))
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid service id")
		return
	}

	var payload createAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	createdByID, err := parseUUIDParam(payload.CreatedBy)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid created_by")
		return
	}
	if payload.KeyType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"key_type": []string{"Missing data for required field."}})
		return
	}

	created, err := h.repo.CreateAPIKey(r.Context(), serviceID, payload.Name, createdByID, payload.KeyType)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": createAPIKeyResponse{Key: created.Key, KeyName: created.APIKey.Name}})
}

func (h *Handler) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	serviceID, err := parseUUIDParam(chi.URLParam(r, "serviceID"))
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid service id")
		return
	}

	var keyID *uuid.UUID
	if raw := r.URL.Query().Get("key_id"); raw != "" {
		parsed, err := parseUUIDParam(raw)
		if err != nil {
			apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid key id")
			return
		}
		keyID = &parsed
	}

	items, err := h.repo.ListAPIKeys(r.Context(), serviceID, keyID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"apiKeys": marshalAPIKeys(items)})
}

func (h *Handler) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	serviceID, err := parseUUIDParam(chi.URLParam(r, "serviceID"))
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid service id")
		return
	}
	keyID, err := parseUUIDParam(chi.URLParam(r, "keyID"))
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid key id")
		return
	}

	revoked, err := h.repo.RevokeAPIKey(r.Context(), serviceID, keyID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if revoked == nil {
		apphandler.WriteAdminError(w, http.StatusNotFound, "API key not found")
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func marshalAPIKeys(items []apiKeysRepo.ApiKey) []apiKeyJSON {
	out := make([]apiKeyJSON, 0, len(items))
	for _, item := range items {
		out = append(out, marshalAPIKey(item))
	}
	return out
}

func marshalAPIKey(item apiKeysRepo.ApiKey) apiKeyJSON {
	var expiryDate *time.Time
	if item.ExpiryDate.Valid {
		expiryDate = &item.ExpiryDate.Time
	}
	return apiKeyJSON{
		ID:         item.ID,
		Name:       item.Name,
		ServiceID:  item.ServiceID,
		CreatedAt:  item.CreatedAt,
		ExpiryDate: expiryDate,
	}
}

func parseUUIDParam(value string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.UUID{}, err
	}
	return parsed, nil
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
