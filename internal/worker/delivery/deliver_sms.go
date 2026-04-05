package delivery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	clientpkg "github.com/maxneuvians/notification-api-spec/internal/client"
	"github.com/maxneuvians/notification-api-spec/internal/client/pinpoint"
	"github.com/maxneuvians/notification-api-spec/internal/config"
	notificationsrepo "github.com/maxneuvians/notification-api-spec/internal/repository/notifications"
	providersrepo "github.com/maxneuvians/notification-api-spec/internal/repository/providers"
	servicesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	providersservice "github.com/maxneuvians/notification-api-spec/internal/service/providers"
	"github.com/maxneuvians/notification-api-spec/pkg/smsutil"
)

const (
	ProcessTypePriority = "priority"
	ProcessTypeNormal   = "normal"
	ProcessTypeBulk     = "bulk"
	maxSMSRetries       = 48
)

type NotificationStore interface {
	GetNotificationByID(ctx context.Context, id uuid.UUID) (notificationsrepo.Notification, error)
	UpdateDelivery(ctx context.Context, update NotificationUpdate) error
}

type ServiceStore interface {
	GetServiceByID(ctx context.Context, arg servicesrepo.GetServiceByIDParams) (servicesrepo.Service, error)
}

type ProviderSelector interface {
	ProviderToUse(ctx context.Context, notificationType providersrepo.NotificationType, sender, to string, templateID uuid.UUID, international bool) (providersservice.Selection, error)
}

type ProviderToggler interface {
	ToggleSMSProviderByIdentifier(ctx context.Context, identifier string) error
}

type SMSContentRenderer interface {
	RenderSMS(ctx context.Context, notification notificationsrepo.Notification, service servicesrepo.Service) (string, error)
}

type SMSResponseSender interface {
	SendSMSResponse(providerName, to, reference string) error
}

type DeliveryCallbackEnqueuer interface {
	EnqueueDeliveryStatusCallback(ctx context.Context, notificationID uuid.UUID) error
}

type StatsClient interface {
	TimingWithDates(name string, end, start time.Time)
	Incr(name string)
}

type NotificationUpdate struct {
	ID               uuid.UUID
	Status           string
	SentBy           *string
	FeedbackReason   *string
	ProviderResponse *string
	BillableUnits    *int32
}

type RetryableError struct {
	After time.Duration
	Err   error
}

func (e *RetryableError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "retry requested"
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

type NotificationTechnicalFailureException struct {
	NotificationID uuid.UUID
}

func (e *NotificationTechnicalFailureException) Error() string {
	return e.NotificationID.String()
}

type SMSWorker struct {
	notifications NotificationStore
	services      ServiceStore
	selector      ProviderSelector
	toggler       ProviderToggler
	renderer      SMSContentRenderer
	responder     SMSResponseSender
	callbacks     DeliveryCallbackEnqueuer
	stats         StatsClient
}

func NewSMSWorker(notifications NotificationStore, services ServiceStore, selector ProviderSelector, toggler ProviderToggler, renderer SMSContentRenderer, responder SMSResponseSender, callbacks DeliveryCallbackEnqueuer, stats StatsClient) *SMSWorker {
	return &SMSWorker{notifications: notifications, services: services, selector: selector, toggler: toggler, renderer: renderer, responder: responder, callbacks: callbacks, stats: stats}
}

func (w *SMSWorker) Deliver(ctx context.Context, notificationID uuid.UUID, processType string, retries int) error {
	notification, err := w.notifications.GetNotificationByID(ctx, notificationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &RetryableError{After: 25 * time.Second, Err: err}
		}
		return err
	}

	if notification.NotificationStatus.Valid && notification.NotificationStatus.String != "created" {
		return nil
	}
	if !notification.ServiceID.Valid {
		return fmt.Errorf("notification %s missing service id", notification.ID)
	}

	service, err := w.services.GetServiceByID(ctx, servicesrepo.GetServiceByIDParams{ID: notification.ServiceID.UUID, OnlyActive: false})
	if err != nil {
		return err
	}

	if !service.Active {
		return w.technicalFailure(ctx, notification.ID)
	}

	body, err := w.renderer.RenderSMS(ctx, notification, service)
	if err != nil {
		return err
	}
	body = smsutil.Normalize(body)
	if strings.TrimSpace(body) == "" {
		return w.technicalFailure(ctx, notification.ID)
	}
	body = smsutil.ApplyPrefix(service.Name, body, service.PrefixSms)

	if service.ResearchMode || notification.KeyType == config.KeyTypeTest || notification.To == config.InternalTestNumber {
		if err := w.responder.SendSMSResponse("sns", notification.To, notification.Reference.String); err != nil {
			return err
		}
		zero := int32(0)
		sentBy := "sns"
		return w.notifications.UpdateDelivery(ctx, NotificationUpdate{ID: notification.ID, Status: "sent", SentBy: &sentBy, BillableUnits: &zero})
	}

	selection, err := w.selector.ProviderToUse(ctx, providersrepo.NotificationTypeSms, "", notification.To, nullUUID(notification.TemplateID), notification.International.Bool)
	if err != nil {
		return err
	}

	sender, ok := selection.Client.(clientpkg.SMSSender)
	if !ok {
		return fmt.Errorf("selected client does not implement SMSSender")
	}

	response, sendErr := sender.SendSMS(ctx, clientpkg.SMSRequest{
		To:             notification.To,
		Content:        body,
		Reference:      notification.Reference.String,
		TemplateID:     nullUUID(notification.TemplateID),
		ServiceID:      notification.ServiceID.UUID,
		SendingVehicle: strings.ToLower(string(selection.SendingVehicle)),
	})
	if response == "opted_out" {
		message := "Phone number is opted out"
		return w.notifications.UpdateDelivery(ctx, NotificationUpdate{ID: notification.ID, Status: "permanent-failure", ProviderResponse: &message})
	}
	if sendErr == nil {
		sentBy := selection.Provider.Identifier
		if err := w.notifications.UpdateDelivery(ctx, NotificationUpdate{ID: notification.ID, Status: "sent", SentBy: &sentBy}); err != nil {
			return err
		}
		if w.stats != nil {
			w.stats.TimingWithDates("sms.total-time", time.Now().UTC(), notification.CreatedAt)
			w.stats.Incr("sms.process_type-" + normalizeProcessType(processType))
		}
		return nil
	}

	var validationErr *pinpoint.PinpointValidationException
	if errors.As(sendErr, &validationErr) {
		if err := w.notifications.UpdateDelivery(ctx, NotificationUpdate{ID: notification.ID, Status: "provider-failure", FeedbackReason: &validationErr.Reason}); err != nil {
			return err
		}
		if w.callbacks != nil {
			return w.callbacks.EnqueueDeliveryStatusCallback(ctx, notification.ID)
		}
		return nil
	}

	var conflictErr *pinpoint.PinpointConflictException
	if errors.As(sendErr, &conflictErr) {
		if err := w.notifications.UpdateDelivery(ctx, NotificationUpdate{ID: notification.ID, Status: "provider-failure"}); err != nil {
			return err
		}
		if w.callbacks != nil {
			return w.callbacks.EnqueueDeliveryStatusCallback(ctx, notification.ID)
		}
		return nil
	}

	one := int32(1)
	if err := w.notifications.UpdateDelivery(ctx, NotificationUpdate{ID: notification.ID, BillableUnits: &one}); err != nil {
		return err
	}
	if w.toggler != nil {
		if err := w.toggler.ToggleSMSProviderByIdentifier(ctx, selection.Provider.Identifier); err != nil {
			return err
		}
	}

	if retries >= maxSMSRetries {
		return w.technicalFailure(ctx, notification.ID)
	}

	return &RetryableError{After: retryDelay(processType), Err: sendErr}
}

func (w *SMSWorker) technicalFailure(ctx context.Context, notificationID uuid.UUID) error {
	if err := w.notifications.UpdateDelivery(ctx, NotificationUpdate{ID: notificationID, Status: "technical-failure"}); err != nil {
		return err
	}
	if w.callbacks != nil {
		if err := w.callbacks.EnqueueDeliveryStatusCallback(ctx, notificationID); err != nil {
			return err
		}
	}
	return &NotificationTechnicalFailureException{NotificationID: notificationID}
}

func retryDelay(processType string) time.Duration {
	if normalizeProcessType(processType) == ProcessTypePriority {
		return 25 * time.Second
	}
	return 300 * time.Second
}

func normalizeProcessType(processType string) string {
	switch strings.ToLower(strings.TrimSpace(processType)) {
	case ProcessTypePriority:
		return ProcessTypePriority
	case ProcessTypeBulk:
		return ProcessTypeBulk
	default:
		return ProcessTypeNormal
	}
}

func nullUUID(value uuid.NullUUID) uuid.UUID {
	if value.Valid {
		return value.UUID
	}
	return uuid.Nil
}
