package delivery

import (
	"context"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

type throttledSMSDeliverer interface {
	Deliver(ctx context.Context, notificationID uuid.UUID, processType string, retries int) error
}

type waitLimiter interface {
	Wait(ctx context.Context) error
}

type ThrottledSMSWorker struct {
	deliverer    throttledSMSDeliverer
	limiter      waitLimiter
	concurrency  int
	ratePerToken time.Duration
}

func NewThrottledSMSWorker(deliverer throttledSMSDeliverer) *ThrottledSMSWorker {
	return newThrottledSMSWorker(deliverer, rate.NewLimiter(rate.Every(2*time.Second), 1), 1)
}

func newThrottledSMSWorker(deliverer throttledSMSDeliverer, limiter waitLimiter, concurrency int) *ThrottledSMSWorker {
	return &ThrottledSMSWorker{deliverer: deliverer, limiter: limiter, concurrency: concurrency, ratePerToken: 2 * time.Second}
}

func (w *ThrottledSMSWorker) Deliver(ctx context.Context, notificationID uuid.UUID, processType string, retries int) error {
	if err := w.limiter.Wait(ctx); err != nil {
		return err
	}
	return w.deliverer.Deliver(ctx, notificationID, processType, retries)
}

func (w *ThrottledSMSWorker) Concurrency() int {
	return w.concurrency
}
