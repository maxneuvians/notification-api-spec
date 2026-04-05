package delivery

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	clientpkg "github.com/maxneuvians/notification-api-spec/internal/client"
	sesclient "github.com/maxneuvians/notification-api-spec/internal/client/ses"
	"github.com/maxneuvians/notification-api-spec/internal/config"
	notificationsrepo "github.com/maxneuvians/notification-api-spec/internal/repository/notifications"
	providersrepo "github.com/maxneuvians/notification-api-spec/internal/repository/providers"
	servicesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	providersservice "github.com/maxneuvians/notification-api-spec/internal/service/providers"
)

type stubEmailRenderer struct {
	content EmailContent
	err     error
}

func (s *stubEmailRenderer) RenderEmail(_ context.Context, _ notificationsrepo.Notification, _ servicesrepo.Service) (EmailContent, error) {
	return s.content, s.err
}

type stubEmailResponder struct {
	called bool
	to     string
	err    error
}

func (s *stubEmailResponder) SendEmailResponse(to string) error {
	s.called = true
	s.to = to
	return s.err
}

type stubBounceRate struct {
	setCalled   bool
	checkCalled bool
	status      BounceRateStatus
	err         error
}

func (s *stubBounceRate) SetSlidingNotifications(_ context.Context, _, _ uuid.UUID) error {
	s.setCalled = true
	return s.err
}

func (s *stubBounceRate) CheckBounceRateStatus(_ context.Context, _ uuid.UUID) (BounceRateStatus, error) {
	s.checkCalled = true
	return s.status, s.err
}

type stubScanner struct {
	statusCode int
	err        error
	called     bool
}

func (s *stubScanner) CheckScanVerdict(_ context.Context, _ string) (int, error) {
	s.called = true
	return s.statusCode, s.err
}

type stubDownloader struct {
	attachment clientpkg.Attachment
	statusCode int
	err        error
	callCount  int
	failUntil  int
}

func (s *stubDownloader) Download(_ context.Context, _ string) (clientpkg.Attachment, int, error) {
	s.callCount++
	if s.failUntil > 0 && s.callCount < s.failUntil {
		return clientpkg.Attachment{}, httpStatusServiceUnavailable, errors.New("temporary")
	}
	if s.err != nil {
		return clientpkg.Attachment{}, s.statusCode, s.err
	}
	return s.attachment, s.statusCode, nil
}

const httpStatusServiceUnavailable = 503

type stubEmailSender struct {
	err     error
	called  bool
	request clientpkg.EmailRequest
}

func (s *stubEmailSender) SendEmail(_ context.Context, request clientpkg.EmailRequest) error {
	s.called = true
	s.request = request
	return s.err
}

func TestEmailWorkerPreconditionsAndSuccess(t *testing.T) {
	baseNotification := notificationsrepo.Notification{
		ID:                 uuid.New(),
		To:                 "recipient@example.com",
		ServiceID:          uuid.NullUUID{UUID: uuid.New(), Valid: true},
		TemplateID:         uuid.NullUUID{UUID: uuid.New(), Valid: true},
		CreatedAt:          time.Now().Add(-time.Minute),
		Reference:          sql.NullString{String: "ref-1", Valid: true},
		NotificationStatus: sql.NullString{String: "created", Valid: true},
	}
	baseService := servicesrepo.Service{ID: baseNotification.ServiceID.UUID, Active: true, EmailFrom: "sender@example.com"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("service inactive becomes technical failure", func(t *testing.T) {
		store := &stubNotificationStore{notification: baseNotification}
		callbacks := &stubCallback{}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: servicesrepo.Service{Active: false}}, &stubSelector{}, &stubEmailRenderer{content: EmailContent{Body: "body", HTMLBody: "<p>body</p>"}}, &stubEmailResponder{}, callbacks, nil, nil, nil, nil, logger)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if len(store.updates) == 0 || store.updates[0].Status != "technical-failure" || !callbacks.called {
			t.Fatalf("updates = %#v callbacks=%v", store.updates, callbacks.called)
		}
	})

	t.Run("wrong status is skipped", func(t *testing.T) {
		notification := baseNotification
		notification.NotificationStatus = sql.NullString{String: "sending", Valid: true}
		store := &stubNotificationStore{notification: notification}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: baseService}, &stubSelector{}, &stubEmailRenderer{content: EmailContent{Body: "body", HTMLBody: "<p>body</p>"}}, &stubEmailResponder{}, &stubCallback{}, nil, nil, nil, nil, logger)
		if err := worker.Deliver(context.Background(), notification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if len(store.updates) != 0 {
			t.Fatalf("updates = %#v, want none", store.updates)
		}
	})

	t.Run("internal test email uses send_email_response", func(t *testing.T) {
		notification := baseNotification
		notification.To = config.InternalTestEmailAddress
		store := &stubNotificationStore{notification: notification}
		responder := &stubEmailResponder{}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: baseService}, &stubSelector{}, &stubEmailRenderer{content: EmailContent{Body: "body", HTMLBody: "<p>body</p>"}}, responder, &stubCallback{}, nil, nil, nil, nil, logger)
		if err := worker.Deliver(context.Background(), notification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if !responder.called || len(store.updates) == 0 || store.updates[0].Status != "sending" {
			t.Fatalf("responder=%#v updates=%#v", responder, store.updates)
		}
	})

	t.Run("research mode uses send_email_response", func(t *testing.T) {
		store := &stubNotificationStore{notification: baseNotification}
		responder := &stubEmailResponder{}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: servicesrepo.Service{ID: baseService.ID, Active: true, EmailFrom: baseService.EmailFrom, ResearchMode: true}}, &stubSelector{}, &stubEmailRenderer{content: EmailContent{Body: "body", HTMLBody: "<p>body</p>"}}, responder, &stubCallback{}, nil, nil, nil, nil, logger)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if !responder.called {
			t.Fatal("expected send_email_response call")
		}
	})

	t.Run("pii scan failure sets pii-check-failed", func(t *testing.T) {
		store := &stubNotificationStore{notification: baseNotification}
		worker := NewEmailWorker(&config.Config{ScanForPII: true}, store, &stubServiceStore{service: baseService}, &stubSelector{}, &stubEmailRenderer{content: EmailContent{Body: "SIN 046 454 005", HTMLBody: "<p>body</p>"}}, &stubEmailResponder{}, &stubCallback{}, nil, nil, nil, nil, logger)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if len(store.updates) == 0 || store.updates[0].Status != "pii-check-failed" {
			t.Fatalf("updates = %#v", store.updates)
		}
	})

	t.Run("success updates status and bounce rate", func(t *testing.T) {
		sender := &stubEmailSender{}
		bounceRate := &stubBounceRate{status: BounceRateStatusNormal}
		store := &stubNotificationStore{notification: baseNotification}
		stats := &stubStats{}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "ses"}, Client: sender}}, &stubEmailRenderer{content: EmailContent{Subject: "subject", Body: "body", HTMLBody: "<p>body</p>"}}, &stubEmailResponder{}, &stubCallback{}, bounceRate, nil, nil, stats, logger)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeBulk, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if !sender.called || !bounceRate.setCalled || !bounceRate.checkCalled || !stats.timingCalled || !stats.incrCalled {
			t.Fatalf("sender=%#v bounce=%#v stats=%#v", sender, bounceRate, stats)
		}
		if len(store.updates) == 0 || store.updates[0].Status != "sending" {
			t.Fatalf("updates = %#v", store.updates)
		}
	})

	t.Run("link sending embeds url without download", func(t *testing.T) {
		sender := &stubEmailSender{}
		store := &stubNotificationStore{notification: baseNotification}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "ses"}, Client: sender}}, &stubEmailRenderer{content: EmailContent{Subject: "subject", Body: "body", HTMLBody: "<p>body</p>", SendingMethod: "link", FileURL: "https://example.com/file.pdf"}}, &stubEmailResponder{}, &stubCallback{}, &stubBounceRate{status: BounceRateStatusNormal}, nil, nil, nil, logger)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if !strings.Contains(sender.request.HTMLBody, "https://example.com/file.pdf") {
			t.Fatalf("html body = %q, want embedded url", sender.request.HTMLBody)
		}
	})

	t.Run("attach downloads file after clean scan", func(t *testing.T) {
		sender := &stubEmailSender{}
		store := &stubNotificationStore{notification: baseNotification}
		scanner := &stubScanner{statusCode: 200}
		downloader := &stubDownloader{attachment: clientpkg.Attachment{Name: "file.pdf", Data: []byte("pdf"), MimeType: "application/pdf"}, statusCode: 200}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "ses"}, Client: sender}}, &stubEmailRenderer{content: EmailContent{Subject: "subject", Body: "body", HTMLBody: "<p>body</p>", SendingMethod: "attach", DirectFileURL: "https://example.com/file.pdf"}}, &stubEmailResponder{}, &stubCallback{}, &stubBounceRate{status: BounceRateStatusNormal}, scanner, downloader, nil, logger)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if !scanner.called || downloader.callCount != 1 || len(sender.request.Attachments) != 1 {
			t.Fatalf("scanner=%#v downloader=%#v request=%#v", scanner, downloader, sender.request)
		}
	})

	t.Run("attach retries download on 5xx until success", func(t *testing.T) {
		sender := &stubEmailSender{}
		store := &stubNotificationStore{notification: baseNotification}
		scanner := &stubScanner{statusCode: 200}
		downloader := &stubDownloader{attachment: clientpkg.Attachment{Name: "file.pdf", Data: []byte("pdf")}, statusCode: 200, failUntil: 5}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "ses"}, Client: sender}}, &stubEmailRenderer{content: EmailContent{Subject: "subject", Body: "body", HTMLBody: "<p>body</p>", SendingMethod: "attach", DirectFileURL: "https://example.com/file.pdf"}}, &stubEmailResponder{}, &stubCallback{}, &stubBounceRate{status: BounceRateStatusNormal}, scanner, downloader, nil, logger)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if downloader.callCount != 5 {
			t.Fatalf("download call count = %d, want 5", downloader.callCount)
		}
	})
}

func TestEmailWorkerErrorsAndRetries(t *testing.T) {
	baseNotification := notificationsrepo.Notification{
		ID:                 uuid.New(),
		To:                 "recipient@example.com",
		ServiceID:          uuid.NullUUID{UUID: uuid.New(), Valid: true},
		TemplateID:         uuid.NullUUID{UUID: uuid.New(), Valid: true},
		CreatedAt:          time.Now().Add(-time.Minute),
		Reference:          sql.NullString{String: "ref-1", Valid: true},
		NotificationStatus: sql.NullString{String: "created", Valid: true},
	}
	baseService := servicesrepo.Service{ID: baseNotification.ServiceID.UUID, Active: true, EmailFrom: "sender@example.com"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("not found retries in 25 seconds", func(t *testing.T) {
		worker := NewEmailWorker(&config.Config{}, &stubNotificationStore{err: sql.ErrNoRows}, &stubServiceStore{}, &stubSelector{}, &stubEmailRenderer{}, &stubEmailResponder{}, &stubCallback{}, nil, nil, nil, nil, logger)
		err := worker.Deliver(context.Background(), uuid.New(), ProcessTypePriority, 0)
		var retryErr *RetryableError
		if !errors.As(err, &retryErr) || retryErr.After != 25*time.Second {
			t.Fatalf("error = %v, want 25 second retry", err)
		}
	})

	t.Run("invalid email causes technical failure and callback", func(t *testing.T) {
		sender := &stubEmailSender{err: &sesclient.InvalidEmailError{Message: "bad address"}}
		store := &stubNotificationStore{notification: baseNotification}
		callbacks := &stubCallback{}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "ses"}, Client: sender}}, &stubEmailRenderer{content: EmailContent{Subject: "subject", Body: "body", HTMLBody: "<p>body</p>"}}, &stubEmailResponder{}, callbacks, nil, nil, nil, nil, logger)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if !callbacks.called || len(store.updates) == 0 || store.updates[0].Status != "technical-failure" {
			t.Fatalf("updates=%#v callbacks=%v", store.updates, callbacks.called)
		}
	})

	t.Run("invalid url causes technical failure", func(t *testing.T) {
		sender := &stubEmailSender{}
		store := &stubNotificationStore{notification: baseNotification}
		callbacks := &stubCallback{}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "ses"}, Client: sender}}, &stubEmailRenderer{content: EmailContent{Subject: "subject", Body: "body", HTMLBody: "<p>body</p>", SendingMethod: "link", DirectFileURL: "file:///tmp/secret"}}, &stubEmailResponder{}, callbacks, nil, nil, nil, nil, logger)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if !callbacks.called || len(store.updates) == 0 || store.updates[0].Status != "technical-failure" {
			t.Fatalf("updates=%#v callbacks=%v", store.updates, callbacks.called)
		}
	})

	t.Run("ses exception retries with status unchanged", func(t *testing.T) {
		sender := &stubEmailSender{err: &sesclient.AwsSesClientException{Message: "throttle"}}
		store := &stubNotificationStore{notification: baseNotification}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "ses"}, Client: sender}}, &stubEmailRenderer{content: EmailContent{Subject: "subject", Body: "body", HTMLBody: "<p>body</p>"}}, &stubEmailResponder{}, &stubCallback{}, nil, nil, nil, nil, logger)
		err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0)
		var retryErr *RetryableError
		if !errors.As(err, &retryErr) || retryErr.After != 300*time.Second || len(store.updates) != 0 {
			t.Fatalf("error=%v updates=%#v", err, store.updates)
		}
	})

	t.Run("malware 423 sets virus-scan-failed", func(t *testing.T) {
		sender := &stubEmailSender{}
		store := &stubNotificationStore{notification: baseNotification}
		callbacks := &stubCallback{}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "ses"}, Client: sender}}, &stubEmailRenderer{content: EmailContent{Subject: "subject", Body: "body", HTMLBody: "<p>body</p>", SendingMethod: "attach", DirectFileURL: "https://example.com/file.pdf"}}, &stubEmailResponder{}, callbacks, nil, &stubScanner{statusCode: 423}, &stubDownloader{}, nil, logger)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if !callbacks.called || len(store.updates) == 0 || store.updates[0].Status != "virus-scan-failed" {
			t.Fatalf("updates=%#v callbacks=%v", store.updates, callbacks.called)
		}
	})

	t.Run("malware 428 uses exponential backoff", func(t *testing.T) {
		sender := &stubEmailSender{}
		store := &stubNotificationStore{notification: baseNotification}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "ses"}, Client: sender}}, &stubEmailRenderer{content: EmailContent{Subject: "subject", Body: "body", HTMLBody: "<p>body</p>", SendingMethod: "attach", DirectFileURL: "https://example.com/file.pdf"}}, &stubEmailResponder{}, &stubCallback{}, nil, &stubScanner{statusCode: 428}, &stubDownloader{}, nil, logger)
		err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 1)
		var retryErr *RetryableError
		if !errors.As(err, &retryErr) || retryErr.After != 60*time.Second {
			t.Fatalf("error = %v, want 60 second retry", err)
		}
	})

	t.Run("malware 404 sets technical failure", func(t *testing.T) {
		sender := &stubEmailSender{}
		store := &stubNotificationStore{notification: baseNotification}
		callbacks := &stubCallback{}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "ses"}, Client: sender}}, &stubEmailRenderer{content: EmailContent{Subject: "subject", Body: "body", HTMLBody: "<p>body</p>", SendingMethod: "attach", DirectFileURL: "https://example.com/file.pdf"}}, &stubEmailResponder{}, callbacks, nil, &stubScanner{statusCode: 404}, &stubDownloader{}, nil, logger)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if !callbacks.called || len(store.updates) == 0 || store.updates[0].Status != "technical-failure" {
			t.Fatalf("updates=%#v callbacks=%v", store.updates, callbacks.called)
		}
	})

	t.Run("max retries becomes technical failure", func(t *testing.T) {
		sender := &stubEmailSender{err: errors.New("boom")}
		store := &stubNotificationStore{notification: baseNotification}
		callbacks := &stubCallback{}
		worker := NewEmailWorker(&config.Config{}, store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "ses"}, Client: sender}}, &stubEmailRenderer{content: EmailContent{Subject: "subject", Body: "body", HTMLBody: "<p>body</p>"}}, &stubEmailResponder{}, callbacks, nil, nil, nil, nil, logger)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, maxEmailRetries); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if !callbacks.called || len(store.updates) == 0 || store.updates[0].Status != "technical-failure" {
			t.Fatalf("updates=%#v callbacks=%v", store.updates, callbacks.called)
		}
	})
}
