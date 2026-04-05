package savenotify

import (
	"log/slog"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	notificationsrepo "github.com/maxneuvians/notification-api-spec/internal/repository/notifications"
	"github.com/maxneuvians/notification-api-spec/queue"
)

func NewSMSWorker(cfg *config.Config, notifications NotificationRepository, producer queue.Producer, acknowledger ReceiptAcknowledger, logger *slog.Logger) *SaveWorker {
	return newWorker(cfg, notifications, nil, producer, acknowledger, logger, notificationsrepo.NotificationTypeSms)
}
