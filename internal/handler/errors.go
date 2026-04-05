package handler

import (
	"encoding/json"
	"net/http"
)

type APIError struct {
	StatusCode int
	Body       any
}

type adminErrorBody struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

type v2ErrorEntry struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type v2ErrorBody struct {
	StatusCode int            `json:"status_code"`
	Errors     []v2ErrorEntry `json:"errors"`
}

func WriteAdminError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, adminErrorBody{
		Result:  "error",
		Message: message,
	})
}

func WriteV2Error(w http.ResponseWriter, statusCode int, errorType, message string) {
	writeJSON(w, statusCode, v2ErrorBody{
		StatusCode: statusCode,
		Errors: []v2ErrorEntry{{
			Error:   errorType,
			Message: message,
		}},
	})
}

func PanicMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				WriteAdminError(w, http.StatusInternalServerError, "Internal server error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
