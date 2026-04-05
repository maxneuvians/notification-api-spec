package worker

import (
	"context"

	"github.com/maxneuvians/notification-api-spec/internal/config"
)

type WorkerManager struct {
	cfg *config.Config
}

func NewWorkerManager(cfg *config.Config) *WorkerManager {
	return &WorkerManager{cfg: cfg}
}

func (m *WorkerManager) Start(context.Context) error {
	return nil
}

func (m *WorkerManager) Stop() {}
