package middleware

import (
	"context"
	"errors"

	apiKeysRepo "github.com/maxneuvians/notification-api-spec/internal/repository/api_keys"
	servicesRepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
)

var ErrNoAuthContext = errors.New("authentication context missing")

type AuthenticatedService struct {
	servicesRepo.Service
	Permissions []string `json:"permissions"`
}

type ApiUser struct {
	apiKeysRepo.ApiKey
}

type contextKey struct {
	name string
}

var (
	authenticatedServiceContextKey = contextKey{name: "authenticated-service"}
	apiUserContextKey              = contextKey{name: "api-user"}
)

func GetAuthenticatedService(ctx context.Context) (*AuthenticatedService, error) {
	if ctx == nil {
		return nil, ErrNoAuthContext
	}

	service, ok := ctx.Value(authenticatedServiceContextKey).(*AuthenticatedService)
	if !ok || service == nil {
		return nil, ErrNoAuthContext
	}

	return service, nil
}

func GetApiUser(ctx context.Context) (*ApiUser, error) {
	if ctx == nil {
		return nil, ErrNoAuthContext
	}

	apiUser, ok := ctx.Value(apiUserContextKey).(*ApiUser)
	if !ok || apiUser == nil {
		return nil, ErrNoAuthContext
	}

	return apiUser, nil
}
