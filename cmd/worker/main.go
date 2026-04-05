package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	internalmigrate "github.com/maxneuvians/notification-api-spec/internal/migrate"
	"github.com/maxneuvians/notification-api-spec/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := openDB(cfg.DatabaseURI, cfg.DBPoolSize)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	// Worker flows currently stay on the writer handle because they mutate state or
	// depend on write-after-read consistency. Add a separate reader only for future
	// explicitly eventual-consistent reporting paths.

	if err := internalmigrate.Run(db, cfg.DatabaseURI); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	manager := worker.NewWorkerManager(cfg)
	if err := manager.Start(ctx); err != nil {
		log.Fatalf("start workers: %v", err)
	}

	<-ctx.Done()
	manager.Stop()
	os.Exit(0)
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
