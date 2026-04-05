package worker

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	"github.com/maxneuvians/notification-api-spec/queue"
)

type MessageHandler func(ctx context.Context, msg *queue.Message) error

type ConsumerFactory interface {
	New(queueURL string) queue.Consumer
}

type WorkerHandlers struct {
	SaveSMS             MessageHandler
	SaveEmail           MessageHandler
	DeliverSMS          MessageHandler
	DeliverEmail        MessageHandler
	DeliverThrottledSMS MessageHandler
	ResearchMode        MessageHandler
}

type WorkerManager struct {
	cfg             *config.Config
	consumerFactory ConsumerFactory
	handlers        WorkerHandlers
	shutdownTimeout time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type Option func(*WorkerManager)

func WithConsumerFactory(factory ConsumerFactory) Option {
	return func(m *WorkerManager) {
		m.consumerFactory = factory
	}
}

func WithHandlers(handlers WorkerHandlers) Option {
	return func(m *WorkerManager) {
		m.handlers = handlers
	}
}

func NewWorkerManager(cfg *config.Config, options ...Option) *WorkerManager {
	manager := &WorkerManager{
		cfg:             cfg,
		consumerFactory: noopConsumerFactory{},
		handlers: WorkerHandlers{
			SaveSMS:             noopHandler,
			SaveEmail:           noopHandler,
			DeliverSMS:          noopHandler,
			DeliverEmail:        noopHandler,
			DeliverThrottledSMS: noopHandler,
			ResearchMode:        noopHandler,
		},
	}
	if cfg != nil && cfg.WorkerShutdownTimeout > 0 {
		manager.shutdownTimeout = cfg.WorkerShutdownTimeout
	} else {
		manager.shutdownTimeout = 5 * time.Second
	}
	for _, option := range options {
		option(manager)
	}
	return manager
}

func (m *WorkerManager) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.cancel = cancel
	m.mu.Unlock()

	concurrency := 1
	if m.cfg != nil && m.cfg.SMSWorkerConcurrency > 0 {
		concurrency = m.cfg.SMSWorkerConcurrency
	}

	for _, spec := range m.queueSpecs(concurrency) {
		for workerIndex := 0; workerIndex < spec.concurrency; workerIndex++ {
			consumer := m.consumerFactory.New(spec.queueURL)
			m.wg.Add(1)
			go func(consumer queue.Consumer, handler MessageHandler) {
				defer m.wg.Done()
				consumer.Start(ctx, handler)
			}(consumer, spec.handler)
		}
	}

	return nil
}

func (m *WorkerManager) Stop() {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Unlock()

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(m.shutdownTimeout):
	}
}

type queueSpec struct {
	queueURL    string
	handler     MessageHandler
	concurrency int
}

func (m *WorkerManager) queueSpecs(concurrency int) []queueSpec {
	return []queueSpec{
		{queueURL: prefixedQueueName(m.cfg, "-priority-database-tasks.fifo"), handler: m.handlers.SaveSMS, concurrency: concurrency},
		{queueURL: prefixedQueueName(m.cfg, "-normal-database-tasks"), handler: m.handlers.SaveSMS, concurrency: concurrency},
		{queueURL: prefixedQueueName(m.cfg, "-bulk-database-tasks"), handler: m.handlers.SaveSMS, concurrency: concurrency},
		{queueURL: prefixedQueueName(m.cfg, "-priority-email-database-tasks.fifo"), handler: m.handlers.SaveEmail, concurrency: concurrency},
		{queueURL: prefixedQueueName(m.cfg, "-normal-email-database-tasks"), handler: m.handlers.SaveEmail, concurrency: concurrency},
		{queueURL: prefixedQueueName(m.cfg, "-bulk-email-database-tasks"), handler: m.handlers.SaveEmail, concurrency: concurrency},
		{queueURL: prefixedQueueName(m.cfg, "send-sms-high"), handler: m.handlers.DeliverSMS, concurrency: concurrency},
		{queueURL: prefixedQueueName(m.cfg, "send-sms-medium"), handler: m.handlers.DeliverSMS, concurrency: concurrency},
		{queueURL: prefixedQueueName(m.cfg, "send-sms-low"), handler: m.handlers.DeliverSMS, concurrency: concurrency},
		{queueURL: prefixedQueueName(m.cfg, "send-email-high"), handler: m.handlers.DeliverEmail, concurrency: concurrency},
		{queueURL: prefixedQueueName(m.cfg, "send-email-medium"), handler: m.handlers.DeliverEmail, concurrency: concurrency},
		{queueURL: prefixedQueueName(m.cfg, "send-email-low"), handler: m.handlers.DeliverEmail, concurrency: concurrency},
		{queueURL: prefixedQueueName(m.cfg, "send-throttled-sms-tasks"), handler: m.handlers.DeliverThrottledSMS, concurrency: 1},
		{queueURL: prefixedQueueName(m.cfg, "research-mode-tasks"), handler: m.handlers.ResearchMode, concurrency: 1},
	}
}

func prefixedQueueName(cfg *config.Config, suffix string) string {
	if cfg == nil {
		return strings.TrimPrefix(suffix, "-")
	}
	prefix := strings.TrimSpace(cfg.NotificationQueuePrefix)
	if prefix == "" {
		return strings.TrimPrefix(suffix, "-")
	}
	return prefix + suffix
}

type noopConsumerFactory struct{}

func (noopConsumerFactory) New(string) queue.Consumer {
	return noopConsumer{}
}

type noopConsumer struct{}

func (noopConsumer) Start(ctx context.Context, _ func(ctx context.Context, msg *queue.Message) error) {
	<-ctx.Done()
}

func (noopConsumer) Stop() {}

func noopHandler(context.Context, *queue.Message) error { return nil }
