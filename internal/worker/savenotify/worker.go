package savenotify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	notificationsrepo "github.com/maxneuvians/notification-api-spec/internal/repository/notifications"
	servicesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
	"github.com/maxneuvians/notification-api-spec/pkg/signing"
	"github.com/maxneuvians/notification-api-spec/queue"
)

const maxSaveWorkerRetries = 5

type NotificationRepository interface {
	BulkInsertNotifications(ctx context.Context, items json.RawMessage) error
}

type ServiceRepository interface {
	GetEmailReplyTo(ctx context.Context, serviceID uuid.UUID) ([]servicesrepo.ServiceEmailReplyTo, error)
}

type ReceiptAcknowledger interface {
	AcknowledgeReceipt(ctx context.Context, receipt string) error
}

type saveTask struct {
	ServiceID           *uuid.UUID `json:"service_id,omitempty"`
	SignedNotifications []string   `json:"signed_notifications"`
	Receipt             string     `json:"receipt,omitempty"`
}

type SaveWorker struct {
	cfg           *config.Config
	notifications NotificationRepository
	services      ServiceRepository
	producer      queue.Producer
	acknowledger  ReceiptAcknowledger
	logger        *slog.Logger
	kind          notificationsrepo.NotificationType
}

func newWorker(cfg *config.Config, notifications NotificationRepository, services ServiceRepository, producer queue.Producer, acknowledger ReceiptAcknowledger, logger *slog.Logger, kind notificationsrepo.NotificationType) *SaveWorker {
	return &SaveWorker{cfg: cfg, notifications: notifications, services: services, producer: producer, acknowledger: acknowledger, logger: logger, kind: kind}
}

func (w *SaveWorker) Handle(ctx context.Context, msg *queue.Message) error {
	var task saveTask
	if err := json.Unmarshal([]byte(msg.Body), &task); err != nil {
		return err
	}

	verified, err := w.verifyAndPrepare(ctx, task)
	if err != nil {
		if errors.Is(err, errBadSignature) {
			w.warn("dropping invalid signed notification batch")
			return nil
		}
		return err
	}

	payload, err := json.Marshal(verified)
	if err != nil {
		return err
	}

	if err := w.notifications.BulkInsertNotifications(ctx, payload); err != nil {
		w.acknowledge(ctx, task.Receipt)
		if receiveCount(msg) < maxSaveWorkerRetries {
			return queue.Transient(err)
		}
		return nil
	}

	w.acknowledge(ctx, task.Receipt)
	for _, item := range verified {
		if err := w.enqueueNotification(ctx, item); err != nil {
			if isRateLimitError(err) {
				w.warn("skipping delivery enqueue because service limit has been exceeded", slog.String("error", err.Error()))
				continue
			}
			return err
		}
	}

	return nil
}

func (w *SaveWorker) verifyAndPrepare(ctx context.Context, task saveTask) ([]map[string]any, error) {
	verified := make([]map[string]any, 0, len(task.SignedNotifications))
	for _, signedNotification := range task.SignedNotifications {
		decoded, err := signing.Loads(signedNotification, w.cfg.SecretKeys, w.cfg.DangerousSalt)
		if err != nil {
			return nil, errBadSignature
		}
		if task.ServiceID != nil {
			decoded["service_id"] = task.ServiceID.String()
		}
		decoded["notification_type"] = string(w.kind)
		if w.kind == notificationsrepo.NotificationTypeEmail {
			if err := w.resolveEmailReplyTo(ctx, task.ServiceID, decoded); err != nil {
				return nil, err
			}
		}
		verified = append(verified, decoded)
	}
	return verified, nil
}

func (w *SaveWorker) resolveEmailReplyTo(ctx context.Context, serviceID *uuid.UUID, payload map[string]any) error {
	if replyTo, ok := payload["reply_to_text"].(string); ok && strings.TrimSpace(replyTo) != "" {
		return nil
	}
	if serviceID == nil || w.services == nil {
		return nil
	}
	replyTos, err := w.services.GetEmailReplyTo(ctx, *serviceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	for _, replyTo := range replyTos {
		if replyTo.IsDefault {
			payload["reply_to_text"] = replyTo.EmailAddress
			return nil
		}
	}
	if len(replyTos) > 0 {
		payload["reply_to_text"] = replyTos[0].EmailAddress
	}
	return nil
}

func (w *SaveWorker) enqueueNotification(ctx context.Context, payload map[string]any) error {
	if w.producer == nil {
		return nil
	}

	notificationID, err := notificationIDFromPayload(payload)
	if err != nil {
		return err
	}
	queueName := queueNameForPayload(w.kind, payload)
	if queueName == "" {
		return fmt.Errorf("missing queue name for notification %s", notificationID)
	}
	body, err := json.Marshal(map[string]string{"notification_id": notificationID.String()})
	if err != nil {
		return err
	}
	if strings.HasSuffix(queueName, ".fifo") {
		return w.producer.SendFIFO(ctx, queueName, string(body), notificationID.String(), notificationID.String())
	}
	return w.producer.Send(ctx, queueName, string(body), nil)
}

func queueNameForPayload(kind notificationsrepo.NotificationType, payload map[string]any) string {
	if queueName, ok := payload["queue_name"].(string); ok && strings.TrimSpace(queueName) != "" {
		return queueName
	}
	processType, _ := payload["process_type"].(string)
	switch strings.ToLower(strings.TrimSpace(processType)) {
	case "priority":
		if kind == notificationsrepo.NotificationTypeEmail {
			return "send-email-high"
		}
		return "send-sms-high"
	case "bulk":
		if kind == notificationsrepo.NotificationTypeEmail {
			return "send-email-low"
		}
		return "send-sms-low"
	default:
		if kind == notificationsrepo.NotificationTypeEmail {
			return "send-email-medium"
		}
		return "send-sms-medium"
	}
}

func notificationIDFromPayload(payload map[string]any) (uuid.UUID, error) {
	value, ok := payload["id"].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return uuid.Nil, fmt.Errorf("notification payload missing id")
	}
	return uuid.Parse(value)
}

func receiveCount(msg *queue.Message) int {
	if msg == nil || msg.Attributes == nil {
		return 1
	}
	value := strings.TrimSpace(msg.Attributes[string(types.MessageSystemAttributeNameApproximateReceiveCount)])
	if value == "" {
		return 1
	}
	count := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 1
		}
		count = count*10 + int(r-'0')
	}
	if count <= 0 {
		return 1
	}
	return count
}

func (w *SaveWorker) acknowledge(ctx context.Context, receipt string) {
	if w.acknowledger == nil || strings.TrimSpace(receipt) == "" {
		return
	}
	if err := w.acknowledger.AcknowledgeReceipt(ctx, receipt); err != nil {
		w.warn("acknowledge receipt failed", slog.String("error", err.Error()))
	}
}

func (w *SaveWorker) warn(message string, args ...any) {
	if w.logger != nil {
		w.logger.Warn(message, args...)
	}
}

var errBadSignature = errors.New("bad signature")

type LiveServiceTooManyRequestsError struct{ Err error }

func (e *LiveServiceTooManyRequestsError) Error() string {
	return errorString(e.Err, "live service limit exceeded")
}
func (e *LiveServiceTooManyRequestsError) Unwrap() error { return e.Err }

type TrialServiceTooManyRequestsError struct{ Err error }

func (e *TrialServiceTooManyRequestsError) Error() string {
	return errorString(e.Err, "trial service limit exceeded")
}
func (e *TrialServiceTooManyRequestsError) Unwrap() error { return e.Err }

func isRateLimitError(err error) bool {
	var liveErr *LiveServiceTooManyRequestsError
	var trialErr *TrialServiceTooManyRequestsError
	return errors.As(err, &liveErr) || errors.As(err, &trialErr)
}

func errorString(err error, fallback string) string {
	if err != nil {
		return err.Error()
	}
	return fallback
}
