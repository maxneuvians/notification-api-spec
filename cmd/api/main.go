package main

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	apphandler "github.com/maxneuvians/notification-api-spec/internal/handler"
	statushandler "github.com/maxneuvians/notification-api-spec/internal/handler/status"
	appmiddleware "github.com/maxneuvians/notification-api-spec/internal/middleware"
	internalmigrate "github.com/maxneuvians/notification-api-spec/internal/migrate"
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

	if cfg.DatabaseReaderURI != "" {
		readerDB, err := openDB(cfg.DatabaseReaderURI, cfg.DBPoolSize)
		if err != nil {
			log.Fatalf("open reader database: %v", err)
		}
		defer readerDB.Close()
	}

	if err := internalmigrate.Run(writerDB, cfg.DatabaseURI); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r := chi.NewRouter()
	r.Use(appmiddleware.RequestID)
	r.Use(appmiddleware.OTEL(cfg.FFEnableOtel))
	r.Use(appmiddleware.Logging(logger))
	r.Use(appmiddleware.CORS(cfg.AdminBaseURL))
	r.Use(appmiddleware.RateLimit(cfg.RateLimitPerSecond, cfg.RateLimitBurst))
	r.Use(appmiddleware.SizeLimit(int64(cfg.AttachmentNumLimit * cfg.AttachmentSizeLimit)))
	r.Use(apphandler.PanicMiddleware)

	statushandler.RegisterRoutes(r)

	server := &http.Server{
		Addr:    listenAddr(cfg.Port),
		Handler: r,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve http: %v", err)
	}
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
