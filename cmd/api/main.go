package main

import (
	"context"
	"database/sql"
	"encoding/json"
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
	statushandler "github.com/maxneuvians/notification-api-spec/internal/handler/status"
	appmiddleware "github.com/maxneuvians/notification-api-spec/internal/middleware"
	internalmigrate "github.com/maxneuvians/notification-api-spec/internal/migrate"
	apiKeysRepo "github.com/maxneuvians/notification-api-spec/internal/repository/api_keys"
	servicesRepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	serviceauth "github.com/maxneuvians/notification-api-spec/internal/service/auth"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	writerDB, err := openDB(cfg.DatabaseURI, cfg.DBPoolSize)
	if err != nil {
		log.Fatalf("open writer database: %v", err)
	}
	defer writerDB.Close()

	// Default to the writer and override with a read replica when one is configured.
	// Auth-path repository lookups such as services.GetServiceByIDWithAPIKeys and
	// api_keys.GetAPIKeyBySecret should receive readerDB, while all writes stay on writerDB.
	readerDB := writerDB
	if cfg.DatabaseReaderURI != "" {
		readerDB, err = openDB(cfg.DatabaseReaderURI, cfg.DBPoolSize)
		if err != nil {
			log.Fatalf("open reader database: %v", err)
		}
		defer readerDB.Close()
	}
	authRepo := &serviceAuthRepository{
		services: servicesRepo.New(readerDB),
		apiKeys:  apiKeysRepo.New(readerDB),
	}

	var authCache *serviceauth.ServiceAuthCache
	if cfg.RedisEnabled && cfg.CacheOpsURL != "" {
		store, err := serviceauth.NewRedisStore(cfg.CacheOpsURL)
		if err != nil {
			log.Fatalf("open redis store: %v", err)
		}
		authCache = serviceauth.NewServiceAuthCache(store)
	}

	if err := internalmigrate.Run(writerDB, cfg.DatabaseURI); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r := newRouter(cfg, logger, authCache, authRepo)

	server := &http.Server{
		Addr:    listenAddr(cfg.Port),
		Handler: r,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve http: %v", err)
	}
}

type serviceAuthRepository struct {
	services *servicesRepo.Queries
	apiKeys  *apiKeysRepo.Queries
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

func newRouter(cfg *config.Config, logger *slog.Logger, authCache *serviceauth.ServiceAuthCache, authRepo appmiddleware.ServiceAuthRepository) http.Handler {
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
