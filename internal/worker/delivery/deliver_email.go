package delivery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	clientpkg "github.com/maxneuvians/notification-api-spec/internal/client"
	sesclient "github.com/maxneuvians/notification-api-spec/internal/client/ses"
	"github.com/maxneuvians/notification-api-spec/internal/config"
	notificationsrepo "github.com/maxneuvians/notification-api-spec/internal/repository/notifications"
	providersrepo "github.com/maxneuvians/notification-api-spec/internal/repository/providers"
	servicesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
)

const (
	maxEmailRetries       = 48
	scanRetryBackoff      = 30 * time.Second
	scanMaxBackoffRetries = 5
)

type EmailContent struct {
	Source         string
	Subject        string
	Body           string
	HTMLBody       string
	ReplyToAddress *string
	SendingMethod  string
	FileURL        string
	DirectFileURL  string
	Attachments    []clientpkg.Attachment
}

type EmailContentRenderer interface {
	RenderEmail(ctx context.Context, notification notificationsrepo.Notification, service servicesrepo.Service) (EmailContent, error)
}

type EmailResponseSender interface {
	SendEmailResponse(to string) error
}

type BounceRateStatus string

const (
	BounceRateStatusNormal   BounceRateStatus = "NORMAL"
	BounceRateStatusWarning  BounceRateStatus = "WARNING"
	BounceRateStatusCritical BounceRateStatus = "CRITICAL"
)

type BounceRateClient interface {
	SetSlidingNotifications(ctx context.Context, serviceID, notificationID uuid.UUID) error
	CheckBounceRateStatus(ctx context.Context, serviceID uuid.UUID) (BounceRateStatus, error)
}

type MalwareScanner interface {
	CheckScanVerdict(ctx context.Context, directFileURL string) (int, error)
}

type FileDownloader interface {
	Download(ctx context.Context, directFileURL string) (clientpkg.Attachment, int, error)
}

type InvalidUrlException struct {
	URL string
}

func (e *InvalidUrlException) Error() string {
	if e.URL == "" {
		return "invalid url"
	}
	return e.URL
}

type MalwareDetectedException struct {
	URL string
}

func (e *MalwareDetectedException) Error() string {
	if e.URL == "" {
		return "malware detected"
	}
	return e.URL
}

type MalwareScanInProgressException struct {
	URL string
}

func (e *MalwareScanInProgressException) Error() string {
	if e.URL == "" {
		return "malware scan in progress"
	}
	return e.URL
}

type DocumentDownloadException struct {
	URL string
}

func (e *DocumentDownloadException) Error() string {
	if e.URL == "" {
		return "document unavailable"
	}
	return e.URL
}

type EmailWorker struct {
	cfg           *config.Config
	notifications NotificationStore
	services      ServiceStore
	selector      ProviderSelector
	renderer      EmailContentRenderer
	responder     EmailResponseSender
	callbacks     DeliveryCallbackEnqueuer
	bounceRate    BounceRateClient
	scanner       MalwareScanner
	downloader    FileDownloader
	stats         StatsClient
	logger        *slog.Logger
}

func NewEmailWorker(cfg *config.Config, notifications NotificationStore, services ServiceStore, selector ProviderSelector, renderer EmailContentRenderer, responder EmailResponseSender, callbacks DeliveryCallbackEnqueuer, bounceRate BounceRateClient, scanner MalwareScanner, downloader FileDownloader, stats StatsClient, logger *slog.Logger) *EmailWorker {
	return &EmailWorker{cfg: cfg, notifications: notifications, services: services, selector: selector, renderer: renderer, responder: responder, callbacks: callbacks, bounceRate: bounceRate, scanner: scanner, downloader: downloader, stats: stats, logger: logger}
}

func (w *EmailWorker) Deliver(ctx context.Context, notificationID uuid.UUID, processType string, retries int) error {
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
		return w.technicalFailureWithCallback(ctx, notification.ID)
	}

	content, err := w.renderer.RenderEmail(ctx, notification, service)
	if err != nil {
		return err
	}

	if strings.EqualFold(notification.To, config.InternalTestEmailAddress) || service.ResearchMode || notification.KeyType == config.KeyTypeTest {
		if err := w.responder.SendEmailResponse(notification.To); err != nil {
			return err
		}
		zero := int32(0)
		sentBy := "ses"
		return w.notifications.UpdateDelivery(ctx, NotificationUpdate{ID: notification.ID, Status: "sending", SentBy: &sentBy, BillableUnits: &zero})
	}

	if w.cfg != nil && w.cfg.ScanForPII && containsValidSIN(content.Body) {
		return w.notifications.UpdateDelivery(ctx, NotificationUpdate{ID: notification.ID, Status: "pii-check-failed"})
	}

	if err := w.prepareAttachments(ctx, &content, retries); err != nil {
		var invalidURL *InvalidUrlException
		if errors.As(err, &invalidURL) {
			return w.technicalFailureWithCallback(ctx, notification.ID)
		}

		var malwareDetected *MalwareDetectedException
		if errors.As(err, &malwareDetected) {
			if updateErr := w.notifications.UpdateDelivery(ctx, NotificationUpdate{ID: notification.ID, Status: "virus-scan-failed"}); updateErr != nil {
				return updateErr
			}
			if w.callbacks != nil {
				return w.callbacks.EnqueueDeliveryStatusCallback(ctx, notification.ID)
			}
			return nil
		}

		var scanInProgress *MalwareScanInProgressException
		if errors.As(err, &scanInProgress) {
			if retries < scanMaxBackoffRetries {
				return &RetryableError{After: scanRetryBackoff * time.Duration(retries+1), Err: err}
			}
			return &RetryableError{After: retryDelay(processType), Err: err}
		}

		var downloadErr *DocumentDownloadException
		if errors.As(err, &downloadErr) {
			return w.technicalFailureWithCallback(ctx, notification.ID)
		}

		return err
	}

	selection, err := w.selector.ProviderToUse(ctx, providersrepo.NotificationTypeEmail, "", notification.To, nullUUID(notification.TemplateID), notification.International.Bool)
	if err != nil {
		return err
	}

	sender, ok := selection.Client.(clientpkg.EmailSender)
	if !ok {
		return fmt.Errorf("selected client does not implement EmailSender")
	}

	source := strings.TrimSpace(content.Source)
	if source == "" {
		source = service.EmailFrom
	}

	sendErr := sender.SendEmail(ctx, clientpkg.EmailRequest{
		Source:         source,
		ToAddresses:    []string{notification.To},
		Subject:        content.Subject,
		Body:           content.Body,
		HTMLBody:       content.HTMLBody,
		ReplyToAddress: content.ReplyToAddress,
		Attachments:    content.Attachments,
	})
	if sendErr == nil {
		sentBy := selection.Provider.Identifier
		if err := w.notifications.UpdateDelivery(ctx, NotificationUpdate{ID: notification.ID, Status: "sending", SentBy: &sentBy}); err != nil {
			return err
		}
		if w.bounceRate != nil {
			if err := w.bounceRate.SetSlidingNotifications(ctx, notification.ServiceID.UUID, notification.ID); err != nil {
				return err
			}
			status, err := w.bounceRate.CheckBounceRateStatus(ctx, notification.ServiceID.UUID)
			if err != nil {
				return err
			}
			switch status {
			case BounceRateStatusCritical:
				w.warn("critical bounce rate threshold of 10% reached", slog.String("service_id", notification.ServiceID.UUID.String()))
			case BounceRateStatusWarning:
				w.warn("warning bounce rate threshold of 5% reached", slog.String("service_id", notification.ServiceID.UUID.String()))
			}
		}
		if w.stats != nil {
			w.stats.TimingWithDates("email.total-time", time.Now().UTC(), notification.CreatedAt)
			w.stats.Incr("email.process_type-" + normalizeProcessType(processType))
			if len(content.Attachments) > 0 {
				w.stats.Incr("email.with_attachments")
			}
		}
		return nil
	}

	var invalidEmailErr *sesclient.InvalidEmailError
	if errors.As(sendErr, &invalidEmailErr) {
		w.info(fmt.Sprintf("Cannot send notification %s, got an invalid email address: %s.", notification.ID, invalidEmailErr.Error()))
		return w.technicalFailureWithCallback(ctx, notification.ID)
	}

	var invalidURL *InvalidUrlException
	if errors.As(sendErr, &invalidURL) {
		return w.technicalFailureWithCallback(ctx, notification.ID)
	}

	var sesErr *sesclient.AwsSesClientException
	if errors.As(sendErr, &sesErr) {
		if retries >= maxEmailRetries {
			return w.technicalFailureWithCallback(ctx, notification.ID)
		}
		return &RetryableError{After: retryDelay(processType), Err: sendErr}
	}

	if retries >= maxEmailRetries {
		return w.technicalFailureWithCallback(ctx, notification.ID)
	}

	return &RetryableError{After: retryDelay(processType), Err: sendErr}
}

func (w *EmailWorker) prepareAttachments(ctx context.Context, content *EmailContent, _ int) error {
	sendingMethod := strings.ToLower(strings.TrimSpace(content.SendingMethod))
	switch sendingMethod {
	case "", "none":
		return nil
	case "attach":
		directFileURL := strings.TrimSpace(content.DirectFileURL)
		if strings.HasPrefix(strings.ToLower(directFileURL), "file://") {
			return &InvalidUrlException{URL: directFileURL}
		}
		if w.scanner != nil {
			statusCode, err := w.scanner.CheckScanVerdict(ctx, directFileURL)
			if err != nil {
				return err
			}
			switch statusCode {
			case http.StatusLocked:
				return &MalwareDetectedException{URL: directFileURL}
			case http.StatusPreconditionRequired:
				return &MalwareScanInProgressException{URL: directFileURL}
			case http.StatusNotFound:
				return &DocumentDownloadException{URL: directFileURL}
			case http.StatusOK, http.StatusRequestTimeout, http.StatusUnprocessableEntity:
			}
		}
		if w.downloader == nil {
			return nil
		}
		attachment, err := w.downloadAttachment(ctx, directFileURL)
		if err != nil {
			return err
		}
		content.Attachments = append(content.Attachments, attachment)
		return nil
	case "link":
		linkURL := strings.TrimSpace(content.FileURL)
		if linkURL == "" {
			linkURL = strings.TrimSpace(content.DirectFileURL)
		}
		if strings.HasPrefix(strings.ToLower(linkURL), "file://") {
			return &InvalidUrlException{URL: linkURL}
		}
		if content.HTMLBody == "" {
			content.HTMLBody = html.EscapeString(content.Body)
		}
		if linkURL != "" {
			content.HTMLBody += fmt.Sprintf("<p><a href=%q>Download file</a></p>", html.EscapeString(linkURL))
		}
		return nil
	default:
		return nil
	}
}

func (w *EmailWorker) downloadAttachment(ctx context.Context, directFileURL string) (clientpkg.Attachment, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		attachment, statusCode, err := w.downloader.Download(ctx, directFileURL)
		if err == nil {
			return attachment, nil
		}
		lastErr = err
		if statusCode < 500 || statusCode > 599 {
			return clientpkg.Attachment{}, err
		}
	}
	w.warn("Max retries exceeded downloading attachment", slog.String("url", directFileURL))
	if lastErr == nil {
		lastErr = fmt.Errorf("Max retries exceeded")
	}
	return clientpkg.Attachment{}, lastErr
}

func (w *EmailWorker) technicalFailureWithCallback(ctx context.Context, notificationID uuid.UUID) error {
	if err := w.notifications.UpdateDelivery(ctx, NotificationUpdate{ID: notificationID, Status: "technical-failure"}); err != nil {
		return err
	}
	if w.callbacks != nil {
		if err := w.callbacks.EnqueueDeliveryStatusCallback(ctx, notificationID); err != nil {
			return err
		}
	}
	return nil
}

func containsValidSIN(value string) bool {
	var token strings.Builder
	flush := func() bool {
		if token.Len() == 0 {
			return false
		}
		var digits strings.Builder
		for _, r := range token.String() {
			if r >= '0' && r <= '9' {
				digits.WriteRune(r)
			}
		}
		matched := digits.Len() == 9 && luhnValid(digits.String())
		token.Reset()
		return matched
	}

	for _, r := range value {
		if (r >= '0' && r <= '9') || r == ' ' || r == '-' {
			token.WriteRune(r)
		} else if flush() {
			return true
		}
	}
	return flush()
}

func luhnValid(value string) bool {
	if len(value) != 9 {
		return false
	}
	sum := 0
	double := false
	for index := len(value) - 1; index >= 0; index-- {
		digit := int(value[index] - '0')
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}
	return sum%10 == 0
}

func (w *EmailWorker) info(message string, args ...any) {
	if w.logger != nil {
		w.logger.Info(message, args...)
	}
}

func (w *EmailWorker) warn(message string, args ...any) {
	if w.logger != nil {
		w.logger.Warn(message, args...)
	}
}
