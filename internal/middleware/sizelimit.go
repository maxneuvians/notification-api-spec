package middleware

import (
	"net/http"
	"strconv"

	apphandler "github.com/maxneuvians/notification-api-spec/internal/handler"
)

func SizeLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if contentLength := r.Header.Get("Content-Length"); contentLength != "" {
				if parsed, err := strconv.ParseInt(contentLength, 10, 64); err == nil && parsed > maxBytes {
					apphandler.WriteAdminError(w, http.StatusRequestEntityTooLarge, "Request entity too large")
					return
				}
			}

			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
