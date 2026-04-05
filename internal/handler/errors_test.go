package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteAdminError(t *testing.T) {
	res := httptest.NewRecorder()

	WriteAdminError(res, http.StatusBadRequest, "bad request")

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	if got := res.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}

	var body adminErrorBody
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Result != "error" || body.Message != "bad request" {
		t.Fatalf("body = %#v, want error/bad request", body)
	}
}

func TestWriteV2Error(t *testing.T) {
	res := httptest.NewRecorder()

	WriteV2Error(res, http.StatusUnauthorized, "AuthError", "missing token")

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}

	var body v2ErrorBody
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status_code = %d, want %d", body.StatusCode, http.StatusUnauthorized)
	}
	if len(body.Errors) != 1 || body.Errors[0].Error != "AuthError" || body.Errors[0].Message != "missing token" {
		t.Fatalf("errors = %#v, want single AuthError entry", body.Errors)
	}
}

func TestPanicMiddleware(t *testing.T) {
	t.Run("recovers panic", func(t *testing.T) {
		wrapped := PanicMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("boom")
		}))

		res := httptest.NewRecorder()
		wrapped.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))

		if res.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
		}
	})

	t.Run("passes through when no panic", func(t *testing.T) {
		wrapped := PanicMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		}))

		res := httptest.NewRecorder()
		wrapped.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))

		if res.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusAccepted)
		}
	})
}
