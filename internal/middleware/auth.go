package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	apiKeysRepo "github.com/maxneuvians/notification-api-spec/internal/repository/api_keys"
	servicesRepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	serviceauth "github.com/maxneuvians/notification-api-spec/internal/service/auth"
	"github.com/maxneuvians/notification-api-spec/pkg/signing"
)

type Config = config.Config

const (
	jwtClockSkew        = 30 * time.Second
	serviceAuthCacheTTL = 30 * time.Second
	apiKeySchemePrefix  = "ApiKey-v1 "
)

type ServiceAuthRepository interface {
	GetServiceByIDWithAPIKeys(ctx context.Context, id uuid.UUID) (servicesRepo.Service, error)
	GetServicePermissions(ctx context.Context, serviceID uuid.UUID) ([]string, error)
	GetAPIKeysByServiceID(ctx context.Context, serviceID uuid.UUID) ([]apiKeysRepo.ApiKey, error)
	GetAPIKeyBySecret(ctx context.Context, secret string) (apiKeysRepo.ApiKey, error)
}

type tokenValidationError struct {
	reason string
}

func (e *tokenValidationError) Error() string {
	return e.reason
}

func (e *tokenValidationError) Reason() string {
	return e.reason
}

type jwtHeader struct {
	Alg string `json:"alg"`
}

type jwtClaims struct {
	Iss string `json:"iss"`
	Exp *int64 `json:"exp"`
	Iat *int64 `json:"iat"`
}

func RequireAdminAuth(cfg Config) func(http.Handler) http.Handler {
	return requireFixedIssuerJWT(cfg.AdminClientUserName, cfg.AdminClientSecret)
}

func RequireSREAuth(cfg Config) func(http.Handler) http.Handler {
	return requireFixedIssuerJWT(cfg.SREUserName, cfg.SREClientSecret)
}

func RequireCacheClearAuth(cfg Config) func(http.Handler) http.Handler {
	return requireFixedIssuerJWT(cfg.CacheClearUserName, cfg.CacheClearClientSecret)
}

func RequireCypressAuth(cfg Config) func(http.Handler) http.Handler {
	if strings.TrimSpace(cfg.CypressAuthClientSecret) == "" {
		panic("cypress auth secret is required")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.NotifyEnvironment == "production" {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			if err := validateFixedIssuerJWTRequest(r, cfg.CypressAuthUserName, cfg.CypressAuthClientSecret, time.Now()); err != nil {
				writeTokenFailure(w, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequireAuth(cfg Config, cache *serviceauth.ServiceAuthCache, repo ServiceAuthRepository) func(http.Handler) http.Handler {
	if repo == nil {
		panic("service auth repository is required")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authorization := strings.TrimSpace(r.Header.Get("Authorization"))
			if authorization == "" {
				writeServiceUnauthorized(w)
				return
			}

			switch {
			case hasBearerPrefix(authorization):
				handleServiceJWTAuth(w, r, next, cache, repo)
			case strings.HasPrefix(authorization, apiKeySchemePrefix):
				handleServiceAPIKeyAuth(w, r, next, cfg, cache, repo)
			default:
				writeServiceUnauthorized(w)
			}
		})
	}
}

func requireFixedIssuerJWT(expectedIssuer, secret string) func(http.Handler) http.Handler {
	if strings.TrimSpace(secret) == "" {
		panic("auth secret is required")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := validateFixedIssuerJWTRequest(r, expectedIssuer, secret, time.Now()); err != nil {
				writeTokenFailure(w, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func validateFixedIssuerJWTRequest(r *http.Request, expectedIssuer, secret string, now time.Time) error {
	token, claims, err := decodeBearerJWTRequest(r, now)
	if err != nil {
		return err
	}
	if err := verifyJWTSignature(token, secret); err != nil {
		return err
	}
	if claims.Iss != expectedIssuer {
		return newTokenValidationError("issuer mismatch")
	}
	return nil
}

func handleServiceJWTAuth(w http.ResponseWriter, r *http.Request, next http.Handler, cache *serviceauth.ServiceAuthCache, repo ServiceAuthRepository) {
	token, claims, err := decodeBearerJWTRequest(r, time.Now())
	if err != nil {
		writeServiceForbidden(w)
		return
	}

	serviceID, err := uuid.Parse(claims.Iss)
	if err != nil {
		writeServiceForbidden(w)
		return
	}

	authData, err := loadServiceAuth(r.Context(), serviceID, cache, repo)
	if err != nil || !authData.Service.Active || len(authData.APIKeys) == 0 {
		writeServiceForbidden(w)
		return
	}

	for _, apiKey := range authData.APIKeys {
		if err := verifyJWTSignature(token, apiKey.Secret); err == nil {
			next.ServeHTTP(w, withServiceAuthContext(r, authData, apiKey))
			return
		}
	}

	writeServiceForbidden(w)
}

func handleServiceAPIKeyAuth(w http.ResponseWriter, r *http.Request, next http.Handler, cfg Config, cache *serviceauth.ServiceAuthCache, repo ServiceAuthRepository) {
	plaintext, err := apiKeyFromRequest(r)
	if err != nil {
		writeServiceForbidden(w)
		return
	}

	serviceID, token, err := parseAPIKeyValue(cfg.APIKeyPrefix, plaintext)
	if err != nil {
		writeServiceForbidden(w)
		return
	}

	variants, err := signing.SignAPIKeyTokenWithAllKeys(token, serviceAuthSecrets(cfg))
	if err != nil {
		writeServiceForbidden(w)
		return
	}

	var matched apiKeysRepo.ApiKey
	found := false
	for _, variant := range variants {
		apiKey, lookupErr := repo.GetAPIKeyBySecret(r.Context(), variant)
		if errors.Is(lookupErr, sql.ErrNoRows) {
			continue
		}
		if lookupErr != nil {
			writeServiceForbidden(w)
			return
		}
		if apiKey.ServiceID != serviceID {
			continue
		}
		if apiKey.ExpiryDate.Valid && !apiKey.ExpiryDate.Time.After(time.Now()) {
			continue
		}
		matched = apiKey
		found = true
		break
	}

	if !found {
		writeServiceForbidden(w)
		return
	}

	authData, err := loadServiceAuth(r.Context(), matched.ServiceID, cache, repo)
	if err != nil || !authData.Service.Active {
		writeServiceForbidden(w)
		return
	}

	next.ServeHTTP(w, withServiceAuthContext(r, authData, matched))
}

func loadServiceAuth(ctx context.Context, serviceID uuid.UUID, cache *serviceauth.ServiceAuthCache, repo ServiceAuthRepository) (*serviceauth.CachedServiceAuth, error) {
	if cached, ok := cache.Get(ctx, serviceID); ok {
		return cached, nil
	}

	service, err := repo.GetServiceByIDWithAPIKeys(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	permissions, err := repo.GetServicePermissions(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	apiKeys, err := repo.GetAPIKeysByServiceID(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	cached := &serviceauth.CachedServiceAuth{
		Service:     service,
		Permissions: permissions,
		APIKeys:     apiKeys,
	}
	cache.Set(ctx, serviceID, cached, serviceAuthCacheTTL)

	return cached, nil
}

func withServiceAuthContext(r *http.Request, authData *serviceauth.CachedServiceAuth, apiKey apiKeysRepo.ApiKey) *http.Request {
	ctx := context.WithValue(r.Context(), authenticatedServiceContextKey, &AuthenticatedService{
		Service:     authData.Service,
		Permissions: append([]string(nil), authData.Permissions...),
	})
	ctx = context.WithValue(ctx, apiUserContextKey, &ApiUser{ApiKey: apiKey})
	return r.WithContext(ctx)
}

func decodeBearerJWTRequest(r *http.Request, now time.Time) (string, jwtClaims, error) {
	token, err := bearerTokenFromRequest(r)
	if err != nil {
		return "", jwtClaims{}, err
	}

	header, claims, err := decodeJWTWithoutVerification(token)
	if err != nil {
		return "", jwtClaims{}, err
	}
	if header.Alg != "" && header.Alg != "HS256" {
		return "", jwtClaims{}, newTokenValidationError("unsupported signing algorithm")
	}
	if strings.TrimSpace(claims.Iss) == "" {
		return "", jwtClaims{}, newTokenValidationError("issuer missing")
	}
	if claims.Exp == nil {
		return "", jwtClaims{}, newTokenValidationError("expiry missing")
	}
	if claims.Iat == nil {
		return "", jwtClaims{}, newTokenValidationError("issued-at missing")
	}

	expiration := time.Unix(*claims.Exp, 0)
	if now.Add(-jwtClockSkew).After(expiration) {
		return "", jwtClaims{}, newTokenValidationError("token expired")
	}

	issuedAt := time.Unix(*claims.Iat, 0)
	if now.Add(jwtClockSkew).Before(issuedAt) {
		return "", jwtClaims{}, newTokenValidationError("token issued in the future")
	}

	return token, claims, nil
}

func bearerTokenFromRequest(r *http.Request) (string, error) {
	if r == nil {
		return "", newTokenValidationError("authorization header missing")
	}

	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if authorization == "" {
		return "", newTokenValidationError("authorization header missing")
	}

	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", newTokenValidationError("authorization header must use Bearer scheme")
	}
	if strings.TrimSpace(parts[1]) == "" {
		return "", newTokenValidationError("authorization token missing")
	}

	return parts[1], nil
}

func apiKeyFromRequest(r *http.Request) (string, error) {
	if r == nil {
		return "", newTokenValidationError("authorization header missing")
	}

	authorization := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, apiKeySchemePrefix) {
		return "", newTokenValidationError("authorization header must use ApiKey-v1 scheme")
	}

	plaintext := strings.TrimSpace(authorization[len(apiKeySchemePrefix):])
	if plaintext == "" {
		return "", newTokenValidationError("authorization token missing")
	}

	return plaintext, nil
}

func parseAPIKeyValue(prefix, plaintext string) (uuid.UUID, string, error) {
	if !strings.HasPrefix(plaintext, prefix) {
		return uuid.UUID{}, "", newTokenValidationError("invalid api key format")
	}
	if len(plaintext) < len(prefix)+72 {
		return uuid.UUID{}, "", newTokenValidationError("invalid api key format")
	}

	serviceSegment := plaintext[len(plaintext)-72 : len(plaintext)-36]
	tokenSegment := plaintext[len(plaintext)-36:]

	serviceID, err := uuid.Parse(serviceSegment)
	if err != nil {
		return uuid.UUID{}, "", newTokenValidationError("invalid api key format")
	}
	if _, err := uuid.Parse(tokenSegment); err != nil {
		return uuid.UUID{}, "", newTokenValidationError("invalid api key format")
	}

	return serviceID, tokenSegment, nil
}

func decodeJWTWithoutVerification(token string) (jwtHeader, jwtClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtHeader{}, jwtClaims{}, newTokenValidationError("jwt must contain three parts")
	}

	var header jwtHeader
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return jwtHeader{}, jwtClaims{}, newTokenValidationError("jwt header is malformed")
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return jwtHeader{}, jwtClaims{}, newTokenValidationError("jwt header is malformed")
	}

	var claims jwtClaims
	claimBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtHeader{}, jwtClaims{}, newTokenValidationError("jwt payload is malformed")
	}
	if err := json.Unmarshal(claimBytes, &claims); err != nil {
		return jwtHeader{}, jwtClaims{}, newTokenValidationError("jwt payload is malformed")
	}

	return header, claims, nil
}

func verifyJWTSignature(token, secret string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return newTokenValidationError("jwt must contain three parts")
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return newTokenValidationError("jwt signature is malformed")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return newTokenValidationError("signature mismatch")
	}

	return nil
}

func hasBearerPrefix(authorization string) bool {
	parts := strings.Fields(authorization)
	return len(parts) > 0 && strings.EqualFold(parts[0], "Bearer")
}

func serviceAuthSecrets(cfg Config) []string {
	if len(cfg.SecretKeys) > 0 {
		return cfg.SecretKeys
	}
	return cfg.SecretKey
}

func writeTokenFailure(w http.ResponseWriter, err error) {
	reason := err.Error()
	var tokenErr *tokenValidationError
	if errors.As(err, &tokenErr) {
		reason = tokenErr.Reason()
	}

	writeJSON(w, http.StatusUnauthorized, map[string][]string{
		"token": {"Invalid token: " + reason},
	})
}

func writeServiceUnauthorized(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, map[string]string{
		"result":  "error",
		"message": "Unauthorized, authentication token must be provided",
	})
}

func writeServiceForbidden(w http.ResponseWriter) {
	w.WriteHeader(http.StatusForbidden)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func newTokenValidationError(reason string) error {
	return &tokenValidationError{reason: reason}
}
