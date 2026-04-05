package providers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apphandler "github.com/maxneuvians/notification-api-spec/internal/handler"
	providersrepo "github.com/maxneuvians/notification-api-spec/internal/repository/providers"
)

type Reader interface {
	GetDaoProviderStats(ctx context.Context) ([]providersrepo.ProviderDetailStat, error)
	GetProviderStatByID(ctx context.Context, id uuid.UUID) (providersrepo.ProviderDetailStat, error)
	GetProviderVersions(ctx context.Context, id uuid.UUID) ([]providersrepo.ProviderDetailsHistory, error)
}

type Writer interface {
	UpdateProviderDetails(ctx context.Context, arg providersrepo.UpdateProviderDetailsParams) (providersrepo.ProviderDetail, error)
}

type Handler struct {
	reader Reader
	writer Writer
}

func NewHandler(reader Reader, writer Writer) *Handler {
	return &Handler{reader: reader, writer: writer}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.getProviders)
	r.Get("/{providerID}", h.getProviderByID)
	r.Post("/{providerID}", h.updateProviderByID)
	r.Get("/{providerID}/versions", h.getProviderVersions)
}

func (h *Handler) getProviders(w http.ResponseWriter, r *http.Request) {
	stats, err := h.reader.GetDaoProviderStats(r.Context())
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"provider_details": marshalProviderStats(stats)})
}

func (h *Handler) getProviderByID(w http.ResponseWriter, r *http.Request) {
	providerID, err := parseProviderID(chi.URLParam(r, "providerID"))
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid provider id")
		return
	}

	provider, err := h.reader.GetProviderStatByID(r.Context(), providerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apphandler.WriteAdminError(w, http.StatusNotFound, "Provider not found")
			return
		}
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"provider_details": marshalProviderStat(provider)})
}

func (h *Handler) updateProviderByID(w http.ResponseWriter, r *http.Request) {
	providerID, err := parseProviderID(chi.URLParam(r, "providerID"))
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid provider id")
		return
	}

	var payload map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	validationErrors := map[string][]string{}
	allowed := map[string]struct{}{"priority": {}, "active": {}, "created_by": {}}
	for key := range payload {
		if _, ok := allowed[key]; !ok {
			validationErrors[key] = []string{"Not permitted to be updated"}
		}
	}
	if len(validationErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"result": "error", "message": validationErrors})
		return
	}

	params := providersrepo.UpdateProviderDetailsParams{ID: providerID}
	if raw, ok := payload["priority"]; ok {
		var priority int32
		if err := json.Unmarshal(raw, &priority); err != nil {
			apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid priority")
			return
		}
		params.Priority = &priority
	}
	if raw, ok := payload["active"]; ok {
		var active bool
		if err := json.Unmarshal(raw, &active); err != nil {
			apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid active value")
			return
		}
		params.Active = &active
	}
	if raw, ok := payload["created_by"]; ok {
		var createdBy string
		if err := json.Unmarshal(raw, &createdBy); err != nil {
			apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid created_by")
			return
		}
		createdByID, err := uuid.Parse(createdBy)
		if err != nil {
			apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid created_by")
			return
		}
		params.CreatedByID = &createdByID
	}

	if _, err := h.writer.UpdateProviderDetails(r.Context(), params); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apphandler.WriteAdminError(w, http.StatusNotFound, "Provider not found")
			return
		}
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	provider, err := h.reader.GetProviderStatByID(r.Context(), providerID)
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"provider_details": marshalProviderStat(provider)})
}

func (h *Handler) getProviderVersions(w http.ResponseWriter, r *http.Request) {
	providerID, err := parseProviderID(chi.URLParam(r, "providerID"))
	if err != nil {
		apphandler.WriteAdminError(w, http.StatusBadRequest, "Invalid provider id")
		return
	}

	versions, err := h.reader.GetProviderVersions(r.Context(), providerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apphandler.WriteAdminError(w, http.StatusNotFound, "Provider not found")
			return
		}
		apphandler.WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": marshalProviderVersions(versions)})
}

type providerDetailsJSON struct {
	ID                    uuid.UUID  `json:"id"`
	CreatedByName         string     `json:"created_by_name"`
	DisplayName           string     `json:"display_name"`
	Identifier            string     `json:"identifier"`
	Priority              int32      `json:"priority"`
	NotificationType      string     `json:"notification_type"`
	Active                bool       `json:"active"`
	UpdatedAt             *time.Time `json:"updated_at"`
	SupportsInternational bool       `json:"supports_international"`
	CurrentMonthBillable  int64      `json:"current_month_billable_sms"`
}

type providerVersionJSON struct {
	ID                    uuid.UUID  `json:"id"`
	CreatedBy             *uuid.UUID `json:"created_by,omitempty"`
	DisplayName           string     `json:"display_name"`
	Identifier            string     `json:"identifier"`
	Priority              int32      `json:"priority"`
	NotificationType      string     `json:"notification_type"`
	Active                bool       `json:"active"`
	Version               int32      `json:"version"`
	UpdatedAt             *time.Time `json:"updated_at"`
	SupportsInternational bool       `json:"supports_international"`
}

func marshalProviderStats(stats []providersrepo.ProviderDetailStat) []providerDetailsJSON {
	out := make([]providerDetailsJSON, 0, len(stats))
	for _, item := range stats {
		out = append(out, marshalProviderStat(item))
	}
	return out
}

func marshalProviderStat(item providersrepo.ProviderDetailStat) providerDetailsJSON {
	var updatedAt *time.Time
	if item.UpdatedAt.Valid {
		updatedAt = &item.UpdatedAt.Time
	}
	return providerDetailsJSON{
		ID:                    item.ID,
		CreatedByName:         item.CreatedByName.String,
		DisplayName:           item.DisplayName,
		Identifier:            item.Identifier,
		Priority:              item.Priority,
		NotificationType:      string(item.NotificationType),
		Active:                item.Active,
		UpdatedAt:             updatedAt,
		SupportsInternational: item.SupportsInternational,
		CurrentMonthBillable:  item.CurrentMonthBillableSMS,
	}
}

func marshalProviderVersions(versions []providersrepo.ProviderDetailsHistory) []providerVersionJSON {
	out := make([]providerVersionJSON, 0, len(versions))
	for _, item := range versions {
		var updatedAt *time.Time
		if item.UpdatedAt.Valid {
			updatedAt = &item.UpdatedAt.Time
		}
		var createdBy *uuid.UUID
		if item.CreatedByID.Valid {
			createdBy = &item.CreatedByID.UUID
		}
		out = append(out, providerVersionJSON{
			ID:                    item.ID,
			CreatedBy:             createdBy,
			DisplayName:           item.DisplayName,
			Identifier:            item.Identifier,
			Priority:              item.Priority,
			NotificationType:      string(item.NotificationType),
			Active:                item.Active,
			Version:               item.Version,
			UpdatedAt:             updatedAt,
			SupportsInternational: item.SupportsInternational,
		})
	}
	return out
}

func parseProviderID(value string) (uuid.UUID, error) {
	return uuid.Parse(value)
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
