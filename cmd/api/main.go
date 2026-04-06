package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	apphandler "github.com/maxneuvians/notification-api-spec/internal/handler"
	apiKeyHandler "github.com/maxneuvians/notification-api-spec/internal/handler/api_key"
	providerhandler "github.com/maxneuvians/notification-api-spec/internal/handler/providers"
	serviceHandler "github.com/maxneuvians/notification-api-spec/internal/handler/service"
	serviceCallbackHandler "github.com/maxneuvians/notification-api-spec/internal/handler/service_callback"
	serviceContactsHandler "github.com/maxneuvians/notification-api-spec/internal/handler/service_contacts"
	statushandler "github.com/maxneuvians/notification-api-spec/internal/handler/status"
	appmiddleware "github.com/maxneuvians/notification-api-spec/internal/middleware"
	internalmigrate "github.com/maxneuvians/notification-api-spec/internal/migrate"
	apiKeysRepo "github.com/maxneuvians/notification-api-spec/internal/repository/api_keys"
	providersRepo "github.com/maxneuvians/notification-api-spec/internal/repository/providers"
	servicesRepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	serviceauth "github.com/maxneuvians/notification-api-spec/internal/service/auth"
)

var (
	loadAPIConfig    = config.Load
	openAPIDB        = openDB
	runAPIMigrations = internalmigrate.Run
	newRedisStore    = serviceauth.NewRedisStore
	newAPIServer     = func(addr string, handler http.Handler) listenServer {
		return &http.Server{Addr: addr, Handler: handler}
	}
	newAPILogger = func() *slog.Logger {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
)

type listenServer interface {
	ListenAndServe() error
}

type serviceQueryReader interface {
	GetServiceByIDWithAPIKeys(ctx context.Context, id uuid.UUID) (servicesRepo.Service, error)
	GetServicePermissions(ctx context.Context, serviceID uuid.UUID) ([]string, error)
}

type apiKeyQueryReader interface {
	GetAPIKeysByServiceID(ctx context.Context, serviceID uuid.UUID) ([]apiKeysRepo.ApiKey, error)
	GetAPIKeyBySecret(ctx context.Context, secret string) (apiKeysRepo.ApiKey, error)
}

type serviceAdminRepository interface {
	apiKeyHandler.Repository
	serviceHandler.Repository
	serviceCallbackHandler.Repository
	serviceContactsHandler.Repository
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := loadAPIConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	writerDB, err := openAPIDB(cfg.DatabaseURI, cfg.DBPoolSize)
	if err != nil {
		return fmt.Errorf("open writer database: %w", err)
	}
	defer writerDB.Close()

	readerDB := writerDB
	closeReader := false
	if cfg.DatabaseReaderURI != "" {
		readerDB, err = openAPIDB(cfg.DatabaseReaderURI, cfg.DBPoolSize)
		if err != nil {
			return fmt.Errorf("open reader database: %w", err)
		}
		closeReader = true
	}
	if closeReader {
		defer readerDB.Close()
	}

	authRepo := newServiceAuthRepository(servicesRepo.New(readerDB), apiKeysRepo.New(readerDB))

	var authCache *serviceauth.ServiceAuthCache
	if cfg.RedisEnabled && cfg.CacheOpsURL != "" {
		store, err := newRedisStore(cfg.CacheOpsURL)
		if err != nil {
			return fmt.Errorf("open redis store: %w", err)
		}
		authCache = serviceauth.NewServiceAuthCache(store)
	}

	if err := runAPIMigrations(writerDB, cfg.DatabaseURI); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	providerReader := providersRepo.New(readerDB)
	providerWriter := providersRepo.New(writerDB)
	serviceAdminRepo := servicesRepo.NewRepository(readerDB, writerDB, servicesRepo.WithConfig(cfg))

	server := newAPIServer(listenAddr(cfg.Port), newRouter(cfg, newAPILogger(), authCache, authRepo, providerReader, providerWriter, serviceAdminRepo))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve http: %w", err)
	}

	return nil
}

type serviceAuthRepository struct {
	services serviceQueryReader
	apiKeys  apiKeyQueryReader
}

func newServiceAuthRepository(services serviceQueryReader, apiKeys apiKeyQueryReader) *serviceAuthRepository {
	return &serviceAuthRepository{services: services, apiKeys: apiKeys}
}

func (r *serviceAuthRepository) GetServiceByIDWithAPIKeys(ctx context.Context, id uuid.UUID) (servicesRepo.Service, error) {
	return r.services.GetServiceByIDWithAPIKeys(ctx, id)
}

func (r *serviceAuthRepository) GetServicePermissions(ctx context.Context, serviceID uuid.UUID) ([]string, error) {
	return r.services.GetServicePermissions(ctx, serviceID)
}

func (r *serviceAuthRepository) GetAPIKeysByServiceID(ctx context.Context, serviceID uuid.UUID) ([]apiKeysRepo.ApiKey, error) {
	return r.apiKeys.GetAPIKeysByServiceID(ctx, serviceID)
}

func (r *serviceAuthRepository) GetAPIKeyBySecret(ctx context.Context, secret string) (apiKeysRepo.ApiKey, error) {
	return r.apiKeys.GetAPIKeyBySecret(ctx, secret)
}

type providerAdminRepository struct {
	reader *providersRepo.Queries
	writer *providersRepo.Queries
}

func (r *providerAdminRepository) GetDaoProviderStats(ctx context.Context) ([]providersRepo.ProviderDetailStat, error) {
	return r.reader.GetDaoProviderStats(ctx)
}

func (r *providerAdminRepository) GetProviderStatByID(ctx context.Context, id uuid.UUID) (providersRepo.ProviderDetailStat, error) {
	return r.reader.GetProviderStatByID(ctx, id)
}

func (r *providerAdminRepository) GetProviderVersions(ctx context.Context, id uuid.UUID) ([]providersRepo.ProviderDetailsHistory, error) {
	return r.reader.GetProviderVersions(ctx, id)
}

func (r *providerAdminRepository) UpdateProviderDetails(ctx context.Context, arg providersRepo.UpdateProviderDetailsParams) (providersRepo.ProviderDetail, error) {
	return r.writer.UpdateProviderDetails(ctx, arg)
}

func newRouter(cfg *config.Config, logger *slog.Logger, authCache *serviceauth.ServiceAuthCache, authRepo appmiddleware.ServiceAuthRepository, providerReader *providersRepo.Queries, providerWriter *providersRepo.Queries, serviceAdminRepo serviceAdminRepository) http.Handler {
	r := chi.NewRouter()
	r.Use(appmiddleware.RequestID)
	r.Use(appmiddleware.OTEL(cfg.FFEnableOtel))
	r.Use(appmiddleware.Logging(logger))
	r.Use(appmiddleware.CORS(cfg.AdminBaseURL))
	r.Use(appmiddleware.RateLimit(cfg.RateLimitPerSecond, cfg.RateLimitBurst))
	r.Use(appmiddleware.SizeLimit(int64(cfg.AttachmentNumLimit * cfg.AttachmentSizeLimit)))
	r.Use(apphandler.PanicMiddleware)

	statushandler.RegisterRoutes(r)
	r.Get("/version", okHandler)

	r.Route("/admin", func(r chi.Router) {
		r.Use(appmiddleware.RequireAdminAuth(*cfg))
		r.Get("/ping", okHandler)
	})

	if providerReader != nil && providerWriter != nil {
		providerRoutes := providerhandler.NewHandler(&providerAdminRepository{reader: providerReader, writer: providerWriter}, &providerAdminRepository{reader: providerReader, writer: providerWriter})
		r.Route("/provider-details", func(r chi.Router) {
			r.Use(appmiddleware.RequireAdminAuth(*cfg))
			providerRoutes.RegisterRoutes(r)
		})
	}

	r.Route("/sre-tools", func(r chi.Router) {
		r.Use(appmiddleware.RequireSREAuth(*cfg))
		r.Get("/ping", okHandler)
	})

	r.Route("/cache-clear", func(r chi.Router) {
		r.Use(appmiddleware.RequireCacheClearAuth(*cfg))
		r.Get("/ping", okHandler)
	})

	r.Route("/cypress", func(r chi.Router) {
		r.Use(appmiddleware.RequireCypressAuth(*cfg))
		r.Get("/ping", okHandler)
	})

	r.Route("/v2", func(r chi.Router) {
		r.Use(appmiddleware.RequireAuth(*cfg, authCache, authRepo))
		r.Get("/ping", okHandler)
		if serviceAdminRepo != nil {
			apiKeyRoutes := apiKeyHandler.NewHandler(serviceAdminRepo)
			serviceRoutes := serviceHandler.NewHandler(serviceAdminRepo)
			serviceCallbackRoutes := serviceCallbackHandler.NewHandler(serviceAdminRepo)
			serviceContactRoutes := serviceContactsHandler.NewHandler(serviceAdminRepo)
			r.Route("/service", func(r chi.Router) {
				apiKeyRoutes.RegisterRoutes(r)
				serviceRoutes.RegisterRoutes(r)
				serviceCallbackRoutes.RegisterRoutes(r)
				serviceContactRoutes.RegisterRoutes(r)
			})
		}
	})

	return r
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func openDB(dsn string, maxOpen int) (*sql.DB, error) {
	db, err := sql.Open("postgres", internalmigrate.NormalizeDatabaseURI(dsn))
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxOpen)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func listenAddr(port string) string {
	if _, _, err := net.SplitHostPort(port); err == nil {
		return port
	}

	if len(port) > 0 && port[0] == ':' {
		return port
	}

	return fmt.Sprintf(":%s", port)
}
