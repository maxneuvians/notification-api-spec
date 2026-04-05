package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	internalmigrate "github.com/maxneuvians/notification-api-spec/internal/migrate"
	"github.com/maxneuvians/notification-api-spec/internal/worker"
)

var (
	loadWorkerConfig    = config.Load
	openWorkerDB        = openDB
	runWorkerMigrations = internalmigrate.Run
	newWorkerManager    = func(cfg *config.Config) workerManager {
		return worker.NewWorkerManager(cfg)
	}
	newSignalContext = func() (context.Context, context.CancelFunc) {
		return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	}
)

type workerManager interface {
	Start(context.Context) error
	Stop()
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := loadWorkerConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := openWorkerDB(cfg.DatabaseURI, cfg.DBPoolSize)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := runWorkerMigrations(db, cfg.DatabaseURI); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	ctx, stop := newSignalContext()
	defer stop()

	manager := newWorkerManager(cfg)
	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("start workers: %w", err)
	}

	<-ctx.Done()
	manager.Stop()
	return nil
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
