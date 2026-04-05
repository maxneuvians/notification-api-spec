package worker

import (
	"context"
	"testing"

	"github.com/maxneuvians/notification-api-spec/internal/config"
)

func TestWorkerManager(t *testing.T) {
	cfg := &config.Config{Port: "8080"}
	manager := NewWorkerManager(cfg)

	if manager.cfg != cfg {
		t.Fatal("manager did not retain config reference")
	}

	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}

	manager.Stop()
}
