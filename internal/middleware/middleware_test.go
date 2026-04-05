package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type capturedRecord struct {
	message string
	attrs   map[string]any
}

type captureHandler struct {
	records []capturedRecord
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := make(map[string]any)
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()
		return true
	})

	h.records = append(h.records, capturedRecord{
		message: record.Message,
		attrs:   attrs,
	})
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *captureHandler) WithGroup(string) slog.Handler {
	return h
}

func TestRequestID(t *testing.T) {
	t.Run("preserves header", func(t *testing.T) {
		wrapped := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := GetRequestID(r.Context()); got != "req-123" {
				t.Fatalf("GetRequestID() = %q, want req-123", got)
			}
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(RequestIDHeader, "req-123")
		res := httptest.NewRecorder()

		wrapped.ServeHTTP(res, req)

		if got := res.Header().Get(RequestIDHeader); got != "req-123" {
			t.Fatalf("response request id = %q, want req-123", got)
		}
	})

	t.Run("creates header when missing", func(t *testing.T) {
		wrapped := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := GetRequestID(r.Context()); got == "" {
				t.Fatal("expected generated request id")
			}
			w.WriteHeader(http.StatusNoContent)
		}))

		res := httptest.NewRecorder()
		wrapped.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))

		if got := res.Header().Get(RequestIDHeader); got == "" {
			t.Fatal("expected response request id header")
		}
	})

	if got := GetRequestID(context.Background()); got != "" {
		t.Fatalf("GetRequestID(background) = %q, want empty", got)
	}
}

func TestRateLimit(t *testing.T) {
	wrapped := RateLimit(1, 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	firstReq := httptest.NewRequest(http.MethodGet, "/", nil)
	firstReq.RemoteAddr = "192.0.2.1:1234"
	firstRes := httptest.NewRecorder()
	wrapped.ServeHTTP(firstRes, firstReq)

	if firstRes.Code != http.StatusAccepted {
		t.Fatalf("first status = %d, want %d", firstRes.Code, http.StatusAccepted)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/", nil)
	secondReq.RemoteAddr = "192.0.2.1:5678"
	secondRes := httptest.NewRecorder()
	wrapped.ServeHTTP(secondRes, secondReq)

	if secondRes.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", secondRes.Code, http.StatusTooManyRequests)
	}

	if got := secondRes.Body.String(); got == "" {
		t.Fatal("expected rate limit response body")
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	if got := clientIP(req); got != "203.0.113.10" {
		t.Fatalf("clientIP(forwarded) = %q, want 203.0.113.10", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.12:8080"
	if got := clientIP(req); got != "198.51.100.12" {
		t.Fatalf("clientIP(remote) = %q, want 198.51.100.12", got)
	}

	req.RemoteAddr = "malformed"
	if got := clientIP(req); got != "malformed" {
		t.Fatalf("clientIP(malformed) = %q, want malformed", got)
	}
}

func TestSizeLimit(t *testing.T) {
	t.Run("rejects oversized content length", func(t *testing.T) {
		called := false
		wrapped := SizeLimit(4)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodPost, "/", io.NopCloser(io.Reader(nil)))
		req.Header.Set("Content-Length", "5")
		res := httptest.NewRecorder()

		wrapped.ServeHTTP(res, req)

		if called {
			t.Fatal("handler should not be called")
		}
		if res.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusRequestEntityTooLarge)
		}
	})

	t.Run("allows smaller content length", func(t *testing.T) {
		called := false
		wrapped := SizeLimit(4)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusCreated)
		}))

		req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
		req.Header.Set("Content-Length", "4")
		res := httptest.NewRecorder()

		wrapped.ServeHTTP(res, req)

		if !called {
			t.Fatal("handler should be called")
		}
		if res.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusCreated)
		}
	})
}

func TestCORS(t *testing.T) {
	t.Run("sets headers for allowed origin", func(t *testing.T) {
		wrapped := CORS("https://admin.example.com")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "https://admin.example.com")
		res := httptest.NewRecorder()

		wrapped.ServeHTTP(res, req)

		if got := res.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example.com" {
			t.Fatalf("allow origin = %q, want https://admin.example.com", got)
		}
		if res.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusAccepted)
		}
	})

	t.Run("options short circuits", func(t *testing.T) {
		called := false
		wrapped := CORS("https://admin.example.com")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))

		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "https://admin.example.com")
		res := httptest.NewRecorder()

		wrapped.ServeHTTP(res, req)

		if called {
			t.Fatal("handler should not be called for options")
		}
		if res.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
		}
	})

	t.Run("ignores mismatched origin", func(t *testing.T) {
		wrapped := CORS("https://admin.example.com")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "https://other.example.com")
		res := httptest.NewRecorder()

		wrapped.ServeHTTP(res, req)

		if got := res.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("allow origin = %q, want empty", got)
		}
	})
}

func TestLogging(t *testing.T) {
	handler := &captureHandler{}
	logger := slog.New(handler)
	wrapped := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/widgets", nil)
	req = req.WithContext(context.WithValue(req.Context(), requestIDContextKey{}, "req-999"))
	res := httptest.NewRecorder()

	wrapped.ServeHTTP(res, req)

	if len(handler.records) != 1 {
		t.Fatalf("log records = %d, want 1", len(handler.records))
	}

	record := handler.records[0]
	if record.message != "http_request" {
		t.Fatalf("log message = %q, want http_request", record.message)
	}
	if got := record.attrs["method"]; got != http.MethodPost {
		t.Fatalf("method = %#v, want %q", got, http.MethodPost)
	}
	if got := record.attrs["path"]; got != "/widgets" {
		t.Fatalf("path = %#v, want /widgets", got)
	}
	if got := record.attrs["request_id"]; got != "req-999" {
		t.Fatalf("request_id = %#v, want req-999", got)
	}

	Logging(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestOTEL(t *testing.T) {
	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	disabled := OTEL(false)(base)
	disabledRes := httptest.NewRecorder()
	disabled.ServeHTTP(disabledRes, httptest.NewRequest(http.MethodGet, "/", nil))
	if disabledRes.Code != http.StatusAccepted {
		t.Fatalf("disabled status = %d, want %d", disabledRes.Code, http.StatusAccepted)
	}

	enabled := OTEL(true)(base)
	enabledRes := httptest.NewRecorder()
	enabled.ServeHTTP(enabledRes, httptest.NewRequest(http.MethodGet, "/", nil))
	if enabledRes.Code != http.StatusAccepted {
		t.Fatalf("enabled status = %d, want %d", enabledRes.Code, http.StatusAccepted)
	}
}
