package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/maxneuvians/notification-api-spec/internal/config"
)

func TestWorkerOpenDBReturnsError(t *testing.T) {
	if _, err := openDB("postgresql://127.0.0.1:1/notify?connect_timeout=1&sslmode=disable", 1); err == nil {
		t.Fatal("expected error, got nil")
	}
}

type stubWorkerManager struct {
	startErr error
	started  bool
	stopped  bool
	onStart  func(context.Context)
}

func (m *stubWorkerManager) Start(ctx context.Context) error {
	m.started = true
	if m.onStart != nil {
		m.onStart(ctx)
	}
	return m.startErr
}

func (m *stubWorkerManager) Stop() {
	m.stopped = true
}

func TestRun(t *testing.T) {
	originalLoadConfig := loadWorkerConfig
	originalOpenDB := openWorkerDB
	originalRunMigrations := runWorkerMigrations
	originalNewManager := newWorkerManager
	originalNewSignalContext := newSignalContext
	defer func() {
		loadWorkerConfig = originalLoadConfig
		openWorkerDB = originalOpenDB
		runWorkerMigrations = originalRunMigrations
		newWorkerManager = originalNewManager
		newSignalContext = originalNewSignalContext
	}()

	t.Run("load config failure", func(t *testing.T) {
		loadWorkerConfig = func() (*config.Config, error) { return nil, errors.New("boom") }
		if err := run(); err == nil || !strings.Contains(err.Error(), "load config") {
			t.Fatalf("run() error = %v, want load config failure", err)
		}
	})

	t.Run("open db failure", func(t *testing.T) {
		loadWorkerConfig = func() (*config.Config, error) { return &config.Config{DatabaseURI: "writer", DBPoolSize: 1}, nil }
		openWorkerDB = func(string, int) (*sql.DB, error) { return nil, errors.New("open failed") }
		if err := run(); err == nil || !strings.Contains(err.Error(), "open database") {
			t.Fatalf("run() error = %v, want open database failure", err)
		}
	})

	t.Run("migration failure", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer db.Close()
		mock.ExpectClose()

		loadWorkerConfig = func() (*config.Config, error) { return &config.Config{DatabaseURI: "writer", DBPoolSize: 1}, nil }
		openWorkerDB = func(string, int) (*sql.DB, error) { return db, nil }
		runWorkerMigrations = func(*sql.DB, string) error { return errors.New("migrate failed") }
		if err := run(); err == nil || !strings.Contains(err.Error(), "run migrations") {
			t.Fatalf("run() error = %v, want migration failure", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("db expectations: %v", err)
		}
	})

	t.Run("start failure", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer db.Close()
		mock.ExpectClose()

		manager := &stubWorkerManager{startErr: errors.New("start failed")}
		loadWorkerConfig = func() (*config.Config, error) { return &config.Config{DatabaseURI: "writer", DBPoolSize: 1}, nil }
		openWorkerDB = func(string, int) (*sql.DB, error) { return db, nil }
		runWorkerMigrations = func(*sql.DB, string) error { return nil }
		newWorkerManager = func(*config.Config) workerManager { return manager }
		newSignalContext = func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) }
		if err := run(); err == nil || !strings.Contains(err.Error(), "start workers") {
			t.Fatalf("run() error = %v, want start failure", err)
		}
		if !manager.started {
			t.Fatal("expected manager to start")
		}
		if manager.stopped {
			t.Fatal("did not expect manager.Stop on start failure")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("db expectations: %v", err)
		}
	})

	t.Run("success stops manager after context cancellation", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer db.Close()
		mock.ExpectClose()

		ctx, cancel := context.WithCancel(context.Background())
		manager := &stubWorkerManager{onStart: func(context.Context) { cancel() }}
		loadWorkerConfig = func() (*config.Config, error) { return &config.Config{DatabaseURI: "writer", DBPoolSize: 1}, nil }
		openWorkerDB = func(string, int) (*sql.DB, error) { return db, nil }
		runWorkerMigrations = func(*sql.DB, string) error { return nil }
		newWorkerManager = func(*config.Config) workerManager { return manager }
		newSignalContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
		if err := run(); err != nil {
			t.Fatalf("run() error = %v, want nil", err)
		}
		if !manager.started || !manager.stopped {
			t.Fatalf("manager state = started:%v stopped:%v, want true/true", manager.started, manager.stopped)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("db expectations: %v", err)
		}
	})
}
