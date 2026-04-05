package status

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type okResponse struct {
	Status string `json:"status"`
}

type liveCountsResponse struct {
	Organisations int `json:"organisations"`
	Services      int `json:"services"`
}

func RegisterRoutes(r chi.Router) {
	r.Get("/", okHandler)
	r.Get("/_status", okHandler)
	r.Post("/_status", okHandler)
	r.Get("/_status/live-service-and-organisation-counts", liveCountsHandler)
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, okResponse{Status: "ok"})
}

func liveCountsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, liveCountsResponse{Organisations: 0, Services: 0})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
