package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	"github.com/maxneuvians/notification-api-spec/queue"
)

type fakeManagerConsumerFactory struct {
	mu        sync.Mutex
	queueURLs []string
	consumers []*fakeManagerConsumer
	release   chan struct{}
}

func (f *fakeManagerConsumerFactory) New(queueURL string) queue.Consumer {
	f.mu.Lock()
	defer f.mu.Unlock()
	consumer := &fakeManagerConsumer{queueURL: queueURL, started: make(chan struct{}), stopped: make(chan struct{}), release: f.release}
	f.queueURLs = append(f.queueURLs, queueURL)
	f.consumers = append(f.consumers, consumer)
	return consumer
}

type fakeManagerConsumer struct {
	queueURL string
	started  chan struct{}
	stopped  chan struct{}
	release  chan struct{}
}

func (f *fakeManagerConsumer) Start(ctx context.Context, _ func(ctx context.Context, msg *queue.Message) error) {
	close(f.started)
	<-ctx.Done()
	if f.release != nil {
		<-f.release
	}
	close(f.stopped)
}

func (*fakeManagerConsumer) Stop() {}

func TestWorkerManager(t *testing.T) {
	cfg := &config.Config{Port: "8080", SMSWorkerConcurrency: 2, WorkerShutdownTimeout: 250 * time.Millisecond, NotificationQueuePrefix: "notify"}
	factory := &fakeManagerConsumerFactory{}
	manager := NewWorkerManager(cfg, WithConsumerFactory(factory))

	if manager.cfg != cfg {
		t.Fatal("manager did not retain config reference")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	wantConsumers := 26
	eventually(t, 2*time.Second, func() bool {
		factory.mu.Lock()
		defer factory.mu.Unlock()
		return len(factory.queueURLs) == wantConsumers
	})
	if len(factory.queueURLs) != wantConsumers {
		t.Fatalf("consumer count = %d, want %d", len(factory.queueURLs), wantConsumers)
	}

	manager.Stop()
}

func TestWorkerManagerStopWaitsForConsumers(t *testing.T) {
	cfg := &config.Config{SMSWorkerConcurrency: 1, WorkerShutdownTimeout: 500 * time.Millisecond}
	release := make(chan struct{})
	factory := &fakeManagerConsumerFactory{release: release}
	manager := NewWorkerManager(cfg, WithConsumerFactory(factory))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	eventually(t, 2*time.Second, func() bool {
		factory.mu.Lock()
		defer factory.mu.Unlock()
		return len(factory.consumers) > 0
	})

	stopped := make(chan struct{})
	go func() {
		manager.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		t.Fatal("Stop() returned before consumers finished")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return after consumers exited")
	}
}

func eventually(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !fn() {
		t.Fatal("condition not satisfied before timeout")
	}
}
