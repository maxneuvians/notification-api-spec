package savenotify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	servicesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	"github.com/maxneuvians/notification-api-spec/pkg/signing"
	"github.com/maxneuvians/notification-api-spec/queue"
)

type stubNotificationsRepo struct {
	payload json.RawMessage
	err     error
	called  bool
}

func (s *stubNotificationsRepo) BulkInsertNotifications(_ context.Context, items json.RawMessage) error {
	s.called = true
	s.payload = append(json.RawMessage(nil), items...)
	return s.err
}

type stubServicesRepo struct {
	replyTos []servicesrepo.ServiceEmailReplyTo
	err      error
}

func (s *stubServicesRepo) GetEmailReplyTo(_ context.Context, _ uuid.UUID) ([]servicesrepo.ServiceEmailReplyTo, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.replyTos, nil
}

type producerCall struct {
	queueURL string
	body     string
	fifo     bool
}

type stubProducer struct {
	err   error
	calls []producerCall
}

func (s *stubProducer) Send(_ context.Context, queueURL, body string, _ map[string]string) error {
	s.calls = append(s.calls, producerCall{queueURL: queueURL, body: body})
	return s.err
}

func (s *stubProducer) SendFIFO(_ context.Context, queueURL, body, _, _ string) error {
	s.calls = append(s.calls, producerCall{queueURL: queueURL, body: body, fifo: true})
	return s.err
}

type stubAcknowledger struct {
	receipts []string
	err      error
}

func (s *stubAcknowledger) AcknowledgeReceipt(_ context.Context, receipt string) error {
	s.receipts = append(s.receipts, receipt)
	return s.err
}

func TestSMSWorkerValidSignedBlobPersistsAndEnqueues(t *testing.T) {
	cfg := testConfig()
	repo := &stubNotificationsRepo{}
	producer := &stubProducer{}
	ack := &stubAcknowledger{}
	worker := NewSMSWorker(cfg, repo, producer, ack, testLogger())
	notificationID := uuid.New()
	message := saveMessageBody(t, cfg, map[string]any{
		"id":           notificationID.String(),
		"recipient":    "+16135550100",
		"service_id":   uuid.New().String(),
		"notification_type": "sms",
		"process_type": "priority",
	})

	if err := worker.Handle(context.Background(), &queue.Message{Body: message}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !repo.called {
		t.Fatal("expected bulk insert")
	}
	if len(producer.calls) != 1 || producer.calls[0].queueURL != "send-sms-high" {
		t.Fatalf("producer calls = %#v", producer.calls)
	}
	if len(ack.receipts) != 0 {
		t.Fatalf("ack receipts = %#v, want none", ack.receipts)
	}
}

func TestEmailWorkerResolvesReplyToAndAcknowledgesReceipt(t *testing.T) {
	cfg := testConfig()
	repo := &stubNotificationsRepo{}
	producer := &stubProducer{}
	ack := &stubAcknowledger{}
	serviceID := uuid.New()
	worker := NewEmailWorker(cfg, repo, &stubServicesRepo{replyTos: []servicesrepo.ServiceEmailReplyTo{{EmailAddress: "default@example.com", IsDefault: true}}}, producer, ack, testLogger())
	notificationID := uuid.New()
	message := saveMessageBody(t, cfg, map[string]any{
		"id":         notificationID.String(),
		"recipient":  "user@example.com",
		"service_id": serviceID.String(),
		"process_type": "normal",
	}, withServiceID(serviceID), withReceipt("receipt-1"))

	if err := worker.Handle(context.Background(), &queue.Message{Body: message}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(ack.receipts) != 1 || ack.receipts[0] != "receipt-1" {
		t.Fatalf("ack receipts = %#v", ack.receipts)
	}
	var inserted []map[string]any
	if err := json.Unmarshal(repo.payload, &inserted); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if inserted[0]["reply_to_text"] != "default@example.com" {
		t.Fatalf("reply_to_text = %#v, want default@example.com", inserted[0]["reply_to_text"])
	}
	if len(producer.calls) != 1 || producer.calls[0].queueURL != "send-email-medium" {
		t.Fatalf("producer calls = %#v", producer.calls)
	}
}

func TestWorkerDropsInvalidSignatureWithoutInsert(t *testing.T) {
	cfg := testConfig()
	repo := &stubNotificationsRepo{}
	producer := &stubProducer{}
	worker := NewSMSWorker(cfg, repo, producer, nil, testLogger())
	body, err := json.Marshal(saveTask{SignedNotifications: []string{"not-a-valid-token"}})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	if err := worker.Handle(context.Background(), &queue.Message{Body: string(body)}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if repo.called || len(producer.calls) != 0 {
		t.Fatalf("repo.called=%v producer.calls=%#v", repo.called, producer.calls)
	}
}

func TestWorkerReturnsTransientOnDatabaseErrorUntilRetryLimit(t *testing.T) {
	cfg := testConfig()
	repo := &stubNotificationsRepo{err: errors.New("db failure")}
	ack := &stubAcknowledger{}
	worker := NewSMSWorker(cfg, repo, &stubProducer{}, ack, testLogger())
	message := &queue.Message{
		Body: saveMessageBody(t, cfg, map[string]any{"id": uuid.New().String(), "recipient": "+16135550100"}),
		Attributes: map[string]string{string(types.MessageSystemAttributeNameApproximateReceiveCount): "2"},
	}
	if err := worker.Handle(context.Background(), message); err == nil || !strings.Contains(err.Error(), "db failure") {
		t.Fatalf("Handle() error = %v, want transient db error", err)
	}
	if len(ack.receipts) != 0 {
		t.Fatalf("ack receipts = %#v, want none", ack.receipts)
	}

	message.Attributes[string(types.MessageSystemAttributeNameApproximateReceiveCount)] = "5"
	if err := worker.Handle(context.Background(), message); err != nil {
		t.Fatalf("Handle() at retry limit error = %v, want nil", err)
	}
}

func TestWorkerAcknowledgesReceiptOnFailure(t *testing.T) {
	cfg := testConfig()
	repo := &stubNotificationsRepo{err: errors.New("db failure")}
	ack := &stubAcknowledger{}
	worker := NewSMSWorker(cfg, repo, &stubProducer{}, ack, testLogger())
	message := &queue.Message{
		Body: saveMessageBody(t, cfg, map[string]any{"id": uuid.New().String(), "recipient": "+16135550100"}, withReceipt("receipt-1")),
		Attributes: map[string]string{string(types.MessageSystemAttributeNameApproximateReceiveCount): "1"},
	}
	_ = worker.Handle(context.Background(), message)
	if len(ack.receipts) != 1 || ack.receipts[0] != "receipt-1" {
		t.Fatalf("ack receipts = %#v", ack.receipts)
	}
}

func TestWorkerSkipsEnqueueOnRateLimitError(t *testing.T) {
	cfg := testConfig()
	repo := &stubNotificationsRepo{}
	producer := &stubProducer{err: &LiveServiceTooManyRequestsError{Err: errors.New("too many")}}
	worker := NewSMSWorker(cfg, repo, producer, nil, testLogger())
	message := saveMessageBody(t, cfg, map[string]any{"id": uuid.New().String(), "recipient": "+16135550100"})

	if err := worker.Handle(context.Background(), &queue.Message{Body: message}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !repo.called || len(producer.calls) != 1 {
		t.Fatalf("repo.called=%v producer.calls=%#v", repo.called, producer.calls)
	}
}

func testConfig() *config.Config {
	return &config.Config{SecretKeys: []string{"secret"}, DangerousSalt: "notify"}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type saveMessageOption func(*saveTask)

func withServiceID(id uuid.UUID) saveMessageOption {
	return func(task *saveTask) { task.ServiceID = &id }
}

func withReceipt(receipt string) saveMessageOption {
	return func(task *saveTask) { task.Receipt = receipt }
}

func saveMessageBody(t *testing.T, cfg *config.Config, payload map[string]any, options ...saveMessageOption) string {
	t.Helper()
	signed, err := signing.Dumps(payload, cfg.SecretKeys[0], cfg.DangerousSalt)
	if err != nil {
		t.Fatalf("sign payload: %v", err)
	}
	task := saveTask{SignedNotifications: []string{signed}}
	for _, option := range options {
		option(&task)
	}
	body, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}
	return string(body)
}
