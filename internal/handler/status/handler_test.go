package status_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/maxneuvians/notification-api-spec/internal/handler/status"
)

func TestStatusRoutes(t *testing.T) {
	r := chi.NewRouter()
	status.RegisterRoutes(r)

	tests := []struct {
		name       string
		method     string
		path       string
		statusCode int
		body       string
	}{
		{name: "root", method: http.MethodGet, path: "/", statusCode: http.StatusOK, body: "{\"status\":\"ok\"}\n"},
		{name: "status get", method: http.MethodGet, path: "/_status", statusCode: http.StatusOK, body: "{\"status\":\"ok\"}\n"},
		{name: "status post", method: http.MethodPost, path: "/_status", statusCode: http.StatusOK, body: "{\"status\":\"ok\"}\n"},
		{name: "live counts", method: http.MethodGet, path: "/_status/live-service-and-organisation-counts", statusCode: http.StatusOK, body: "{\"organisations\":0,\"services\":0}\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			res := httptest.NewRecorder()

			r.ServeHTTP(res, req)

			if res.Code != tc.statusCode {
				t.Fatalf("status code = %d, want %d", res.Code, tc.statusCode)
			}

			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}

			if string(body) != tc.body {
				t.Fatalf("body = %q, want %q", string(body), tc.body)
			}
		})
	}
}
