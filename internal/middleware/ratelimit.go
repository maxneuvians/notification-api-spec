package middleware

import (
	"net"
	"net/http"
	"sync"

	apphandler "github.com/maxneuvians/notification-api-spec/internal/handler"
	"golang.org/x/time/rate"
)

type ipLimiters struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	limit    rate.Limit
	burst    int
}

func RateLimit(limitPerSecond float64, burst int) func(http.Handler) http.Handler {
	store := &ipLimiters{
		limiters: make(map[string]*rate.Limiter),
		limit:    rate.Limit(limitPerSecond),
		burst:    burst,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limiter := store.getLimiter(clientIP(r))
			if !limiter.Allow() {
				apphandler.WriteAdminError(w, http.StatusTooManyRequests, "Rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (s *ipLimiters) getLimiter(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	limiter, ok := s.limiters[ip]
	if !ok {
		limiter = rate.NewLimiter(s.limit, s.burst)
		s.limiters[ip] = limiter
	}

	return limiter
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
