package delivery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

type stubThrottledDeliverer struct {
	called         bool
	notificationID uuid.UUID
	processType    string
	retries        int
	err            error
}

func (s *stubThrottledDeliverer) Deliver(_ context.Context, notificationID uuid.UUID, processType string, retries int) error {
	s.called = true
	s.notificationID = notificationID
	s.processType = processType
	s.retries = retries
	return s.err
}

func TestThrottledSMSWorkerUsesSingleConcurrency(t *testing.T) {
	worker := NewThrottledSMSWorker(&stubThrottledDeliverer{})
	if worker.Concurrency() != 1 {
		t.Fatalf("Concurrency() = %d, want 1", worker.Concurrency())
	}
}

func TestThrottledSMSWorkerDelegatesToSMSWorker(t *testing.T) {
	deliverer := &stubThrottledDeliverer{}
	worker := newThrottledSMSWorker(deliverer, rate.NewLimiter(rate.Every(time.Millisecond), 1), 1)
	id := uuid.New()

	if err := worker.Deliver(context.Background(), id, ProcessTypePriority, 3); err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if !deliverer.called || deliverer.notificationID != id || deliverer.processType != ProcessTypePriority || deliverer.retries != 3 {
		t.Fatalf("deliverer = %#v", deliverer)
	}
}

func TestThrottledSMSWorkerRateLimitsCalls(t *testing.T) {
	deliverer := &stubThrottledDeliverer{}
	worker := newThrottledSMSWorker(deliverer, rate.NewLimiter(rate.Every(100*time.Millisecond), 1), 1)
	ctx := context.Background()

	start := time.Now()
	if err := worker.Deliver(ctx, uuid.New(), ProcessTypeNormal, 0); err != nil {
		t.Fatalf("first Deliver() error = %v", err)
	}
	if err := worker.Deliver(ctx, uuid.New(), ProcessTypeNormal, 0); err != nil {
		t.Fatalf("second Deliver() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("elapsed = %v, want at least 100ms", elapsed)
	}
}

func TestThrottledSMSWorkerReturnsUnderlyingErrors(t *testing.T) {
	deliverer := &stubThrottledDeliverer{err: errors.New("boom")}
	worker := newThrottledSMSWorker(deliverer, rate.NewLimiter(rate.Every(time.Millisecond), 1), 1)
	if err := worker.Deliver(context.Background(), uuid.New(), ProcessTypeBulk, 0); err == nil || err.Error() != "boom" {
		t.Fatalf("Deliver() error = %v, want boom", err)
	}
}
