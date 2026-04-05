package savenotify

import (
	"log/slog"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	notificationsrepo "github.com/maxneuvians/notification-api-spec/internal/repository/notifications"
	"github.com/maxneuvians/notification-api-spec/queue"
)

func NewEmailWorker(cfg *config.Config, notifications NotificationRepository, services ServiceRepository, producer queue.Producer, acknowledger ReceiptAcknowledger, logger *slog.Logger) *SaveWorker {
	return newWorker(cfg, notifications, services, producer, acknowledger, logger, notificationsrepo.NotificationTypeEmail)
}
