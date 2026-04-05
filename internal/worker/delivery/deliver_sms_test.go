package delivery

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	clientpkg "github.com/maxneuvians/notification-api-spec/internal/client"
	"github.com/maxneuvians/notification-api-spec/internal/client/pinpoint"
	"github.com/maxneuvians/notification-api-spec/internal/config"
	notificationsrepo "github.com/maxneuvians/notification-api-spec/internal/repository/notifications"
	providersrepo "github.com/maxneuvians/notification-api-spec/internal/repository/providers"
	servicesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	providersservice "github.com/maxneuvians/notification-api-spec/internal/service/providers"
)

type stubNotificationStore struct {
	notification notificationsrepo.Notification
	err          error
	updates      []NotificationUpdate
}

func (s *stubNotificationStore) GetNotificationByID(_ context.Context, _ uuid.UUID) (notificationsrepo.Notification, error) {
	if s.err != nil {
		return notificationsrepo.Notification{}, s.err
	}
	return s.notification, nil
}

func (s *stubNotificationStore) UpdateDelivery(_ context.Context, update NotificationUpdate) error {
	s.updates = append(s.updates, update)
	return nil
}

type stubServiceStore struct {
	service servicesrepo.Service
	err     error
}

func (s *stubServiceStore) GetServiceByID(_ context.Context, _ servicesrepo.GetServiceByIDParams) (servicesrepo.Service, error) {
	if s.err != nil {
		return servicesrepo.Service{}, s.err
	}
	return s.service, nil
}

type stubSelector struct {
	selection providersservice.Selection
	err       error
}

func (s *stubSelector) ProviderToUse(_ context.Context, _ providersrepo.NotificationType, _ string, _ string, _ uuid.UUID, _ bool) (providersservice.Selection, error) {
	if s.err != nil {
		return providersservice.Selection{}, s.err
	}
	return s.selection, nil
}

type stubToggler struct {
	called     bool
	identifier string
	err        error
}

func (s *stubToggler) ToggleSMSProviderByIdentifier(_ context.Context, identifier string) error {
	s.called = true
	s.identifier = identifier
	return s.err
}

type stubRenderer struct {
	body string
	err  error
}

func (s *stubRenderer) RenderSMS(_ context.Context, _ notificationsrepo.Notification, _ servicesrepo.Service) (string, error) {
	return s.body, s.err
}

type stubResponder struct {
	called    bool
	provider  string
	to        string
	reference string
	err       error
}

func (s *stubResponder) SendSMSResponse(providerName, to, reference string) error {
	s.called = true
	s.provider = providerName
	s.to = to
	s.reference = reference
	return s.err
}

type stubCallback struct {
	called bool
	ids    []uuid.UUID
}

func (s *stubCallback) EnqueueDeliveryStatusCallback(_ context.Context, notificationID uuid.UUID) error {
	s.called = true
	s.ids = append(s.ids, notificationID)
	return nil
}

type stubStats struct {
	timingCalled bool
	incrCalled   bool
	incrKey      string
}

func (s *stubStats) TimingWithDates(_ string, _, _ time.Time) { s.timingCalled = true }
func (s *stubStats) Incr(name string)                         { s.incrCalled = true; s.incrKey = name }

type stubSMSSender struct {
	response string
	err      error
	called   bool
	request  clientpkg.SMSRequest
}

func (s *stubSMSSender) SendSMS(_ context.Context, request clientpkg.SMSRequest) (string, error) {
	s.called = true
	s.request = request
	return s.response, s.err
}

func TestSMSWorkerPreconditionsAndSuccessPaths(t *testing.T) {
	baseNotification := notificationsrepo.Notification{
		ID:                 uuid.New(),
		To:                 "+16135550100",
		ServiceID:          uuid.NullUUID{UUID: uuid.New(), Valid: true},
		TemplateID:         uuid.NullUUID{UUID: uuid.New(), Valid: true},
		CreatedAt:          time.Now().Add(-time.Minute),
		Reference:          sql.NullString{String: "ref-1", Valid: true},
		NotificationStatus: sql.NullString{String: "created", Valid: true},
	}
	baseService := servicesrepo.Service{ID: baseNotification.ServiceID.UUID, Active: true}

	t.Run("service inactive becomes technical failure", func(t *testing.T) {
		store := &stubNotificationStore{notification: baseNotification}
		callbacks := &stubCallback{}
		worker := NewSMSWorker(store, &stubServiceStore{service: servicesrepo.Service{Active: false}}, &stubSelector{}, &stubToggler{}, &stubRenderer{body: "hello"}, &stubResponder{}, callbacks, nil)

		err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0)
		var target *NotificationTechnicalFailureException
		if !errors.As(err, &target) {
			t.Fatalf("error = %v, want NotificationTechnicalFailureException", err)
		}
		if len(store.updates) == 0 || store.updates[0].Status != "technical-failure" {
			t.Fatalf("updates = %#v, want technical-failure", store.updates)
		}
		if !callbacks.called {
			t.Fatal("expected callback enqueue")
		}
	})

	t.Run("empty body becomes technical failure", func(t *testing.T) {
		store := &stubNotificationStore{notification: baseNotification}
		worker := NewSMSWorker(store, &stubServiceStore{service: baseService}, &stubSelector{}, &stubToggler{}, &stubRenderer{body: "   "}, &stubResponder{}, &stubCallback{}, nil)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err == nil {
			t.Fatal("expected error")
		}
		if len(store.updates) == 0 || store.updates[0].Status != "technical-failure" {
			t.Fatalf("updates = %#v, want technical-failure", store.updates)
		}
	})

	t.Run("wrong status is skipped", func(t *testing.T) {
		notification := baseNotification
		notification.NotificationStatus = sql.NullString{String: "sending", Valid: true}
		store := &stubNotificationStore{notification: notification}
		worker := NewSMSWorker(store, &stubServiceStore{service: baseService}, &stubSelector{}, &stubToggler{}, &stubRenderer{body: "hello"}, &stubResponder{}, &stubCallback{}, nil)
		if err := worker.Deliver(context.Background(), notification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if len(store.updates) != 0 {
			t.Fatalf("updates = %#v, want none", store.updates)
		}
	})

	t.Run("research mode uses send_sms_response", func(t *testing.T) {
		store := &stubNotificationStore{notification: baseNotification}
		responder := &stubResponder{}
		worker := NewSMSWorker(store, &stubServiceStore{service: servicesrepo.Service{ID: baseService.ID, Active: true, ResearchMode: true}}, &stubSelector{}, &stubToggler{}, &stubRenderer{body: "hello"}, responder, &stubCallback{}, nil)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if !responder.called {
			t.Fatal("expected send_sms_response call")
		}
		if len(store.updates) == 0 || store.updates[0].Status != "sent" {
			t.Fatalf("updates = %#v, want sent", store.updates)
		}
	})

	t.Run("test key uses send_sms_response", func(t *testing.T) {
		notification := baseNotification
		notification.KeyType = config.KeyTypeTest
		store := &stubNotificationStore{notification: notification}
		responder := &stubResponder{}
		worker := NewSMSWorker(store, &stubServiceStore{service: baseService}, &stubSelector{}, &stubToggler{}, &stubRenderer{body: "hello"}, responder, &stubCallback{}, nil)
		if err := worker.Deliver(context.Background(), notification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if !responder.called {
			t.Fatal("expected send_sms_response call")
		}
	})

	t.Run("internal test number uses send_sms_response", func(t *testing.T) {
		notification := baseNotification
		notification.To = config.InternalTestNumber
		store := &stubNotificationStore{notification: notification}
		responder := &stubResponder{}
		worker := NewSMSWorker(store, &stubServiceStore{service: baseService}, &stubSelector{}, &stubToggler{}, &stubRenderer{body: "hello"}, responder, &stubCallback{}, nil)
		if err := worker.Deliver(context.Background(), notification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if !responder.called {
			t.Fatal("expected send_sms_response call")
		}
	})

	t.Run("successful send emits stats", func(t *testing.T) {
		sender := &stubSMSSender{}
		stats := &stubStats{}
		store := &stubNotificationStore{notification: baseNotification}
		worker := NewSMSWorker(store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "sns"}, Client: sender, SendingVehicle: providersservice.SmsSendingVehicleLongCode}}, &stubToggler{}, &stubRenderer{body: "hello"}, &stubResponder{}, &stubCallback{}, stats)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeBulk, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if !sender.called {
			t.Fatal("expected provider send")
		}
		if !stats.timingCalled || !stats.incrCalled || stats.incrKey != "sms.process_type-bulk" {
			t.Fatalf("stats = %#v, want timing and bulk increment", stats)
		}
	})

	t.Run("normalizes content and prefixes service name", func(t *testing.T) {
		sender := &stubSMSSender{}
		service := baseService
		service.Name = "Sample service"
		service.PrefixSms = true
		store := &stubNotificationStore{notification: baseNotification}
		worker := NewSMSWorker(store, &stubServiceStore{service: service}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "sns"}, Client: sender}}, &stubToggler{}, &stubRenderer{body: "Hello\t\u2026\u200b\U0001F347 <b>tag</b>"}, &stubResponder{}, &stubCallback{}, nil)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if sender.request.Content != "Sample service: Hello ...? <b>tag</b>" {
			t.Fatalf("content = %q", sender.request.Content)
		}
	})

	t.Run("opted out becomes permanent failure", func(t *testing.T) {
		sender := &stubSMSSender{response: "opted_out"}
		store := &stubNotificationStore{notification: baseNotification}
		worker := NewSMSWorker(store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "pinpoint"}, Client: sender}}, &stubToggler{}, &stubRenderer{body: "hello"}, &stubResponder{}, &stubCallback{}, nil)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if len(store.updates) == 0 || store.updates[0].Status != "permanent-failure" {
			t.Fatalf("updates = %#v, want permanent-failure", store.updates)
		}
	})
}

func TestSMSWorkerErrorHandling(t *testing.T) {
	baseNotification := notificationsrepo.Notification{
		ID:                 uuid.New(),
		To:                 "+16135550100",
		ServiceID:          uuid.NullUUID{UUID: uuid.New(), Valid: true},
		TemplateID:         uuid.NullUUID{UUID: uuid.New(), Valid: true},
		CreatedAt:          time.Now().Add(-time.Minute),
		Reference:          sql.NullString{String: "ref-1", Valid: true},
		NotificationStatus: sql.NullString{String: "created", Valid: true},
	}
	baseService := servicesrepo.Service{ID: baseNotification.ServiceID.UUID, Active: true}

	t.Run("not found retries in 25 seconds", func(t *testing.T) {
		worker := NewSMSWorker(&stubNotificationStore{err: sql.ErrNoRows}, &stubServiceStore{}, &stubSelector{}, &stubToggler{}, &stubRenderer{}, &stubResponder{}, &stubCallback{}, nil)
		err := worker.Deliver(context.Background(), uuid.New(), ProcessTypePriority, 0)
		var retryErr *RetryableError
		if !errors.As(err, &retryErr) || retryErr.After != 25*time.Second {
			t.Fatalf("error = %v, want 25 second retry", err)
		}
	})

	t.Run("PinpointValidationException sets provider failure and no toggle", func(t *testing.T) {
		sender := &stubSMSSender{err: &pinpoint.PinpointValidationException{Reason: "NO_ORIGINATION_IDENTITIES_FOUND"}}
		store := &stubNotificationStore{notification: baseNotification}
		toggler := &stubToggler{}
		callbacks := &stubCallback{}
		worker := NewSMSWorker(store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "pinpoint"}, Client: sender}}, toggler, &stubRenderer{body: "hello"}, &stubResponder{}, callbacks, nil)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if toggler.called {
			t.Fatal("toggle should not be called for validation exception")
		}
		if !callbacks.called || len(store.updates) == 0 || store.updates[0].Status != "provider-failure" || store.updates[0].FeedbackReason == nil || *store.updates[0].FeedbackReason != "NO_ORIGINATION_IDENTITIES_FOUND" {
			t.Fatalf("updates = %#v callbacks=%v", store.updates, callbacks.called)
		}
	})

	t.Run("PinpointConflictException sets provider failure and no toggle", func(t *testing.T) {
		sender := &stubSMSSender{err: &pinpoint.PinpointConflictException{Reason: "OTHER"}}
		store := &stubNotificationStore{notification: baseNotification}
		toggler := &stubToggler{}
		callbacks := &stubCallback{}
		worker := NewSMSWorker(store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "pinpoint"}, Client: sender}}, toggler, &stubRenderer{body: "hello"}, &stubResponder{}, callbacks, nil)
		if err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0); err != nil {
			t.Fatalf("Deliver() error = %v", err)
		}
		if toggler.called {
			t.Fatal("toggle should not be called for conflict exception")
		}
		if !callbacks.called || len(store.updates) == 0 || store.updates[0].Status != "provider-failure" {
			t.Fatalf("updates = %#v callbacks=%v", store.updates, callbacks.called)
		}
	})

	t.Run("generic exception retries, sets billable units, and toggles provider", func(t *testing.T) {
		sender := &stubSMSSender{err: errors.New("boom")}
		store := &stubNotificationStore{notification: baseNotification}
		toggler := &stubToggler{}
		worker := NewSMSWorker(store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "sns"}, Client: sender}}, toggler, &stubRenderer{body: "hello"}, &stubResponder{}, &stubCallback{}, nil)
		err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, 0)
		var retryErr *RetryableError
		if !errors.As(err, &retryErr) || retryErr.After != 300*time.Second {
			t.Fatalf("error = %v, want 300 second retry", err)
		}
		if !toggler.called || toggler.identifier != "sns" {
			t.Fatalf("toggle = %#v, want sns", toggler)
		}
		if len(store.updates) == 0 || store.updates[0].BillableUnits == nil || *store.updates[0].BillableUnits != 1 {
			t.Fatalf("updates = %#v, want billable_units=1", store.updates)
		}
	})

	t.Run("max retries becomes technical failure", func(t *testing.T) {
		sender := &stubSMSSender{err: errors.New("boom")}
		store := &stubNotificationStore{notification: baseNotification}
		callbacks := &stubCallback{}
		worker := NewSMSWorker(store, &stubServiceStore{service: baseService}, &stubSelector{selection: providersservice.Selection{Provider: providersrepo.ProviderDetail{Identifier: "sns"}, Client: sender}}, &stubToggler{}, &stubRenderer{body: "hello"}, &stubResponder{}, callbacks, nil)
		err := worker.Deliver(context.Background(), baseNotification.ID, ProcessTypeNormal, maxSMSRetries)
		var target *NotificationTechnicalFailureException
		if !errors.As(err, &target) {
			t.Fatalf("error = %v, want NotificationTechnicalFailureException", err)
		}
		if !callbacks.called || len(store.updates) < 2 || store.updates[len(store.updates)-1].Status != "technical-failure" {
			t.Fatalf("updates = %#v callbacks=%v", store.updates, callbacks.called)
		}
	})
}
