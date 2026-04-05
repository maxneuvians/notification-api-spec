package pinpoint

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/client"
	"github.com/maxneuvians/notification-api-spec/internal/config"
)

const transactionalMessageType = "TRANSACTIONAL"

type SendTextMessageInput struct {
	DestinationPhoneNumber string
	MessageBody            string
	MessageType            string
	ConfigurationSetName   string
	OriginationIdentity    string
	HasOriginationIdentity bool
	DryRun                 bool
	HasDryRun              bool
	Context                map[string]string
}

type senderAPI interface {
	SendTextMessage(ctx context.Context, input SendTextMessageInput) (string, error)
}

type APIError struct {
	Code    string
	Reason  string
	Message string
	Err     error
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *APIError) Unwrap() error {
	return e.Err
}

type PinpointConflictException struct {
	Reason string
	Err    error
}

func (e *PinpointConflictException) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	if e.Reason != "" {
		return e.Reason
	}
	return "pinpoint conflict"
}

func (e *PinpointConflictException) Unwrap() error {
	return e.Err
}

type PinpointValidationException struct {
	Reason string
	Err    error
}

func (e *PinpointValidationException) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	if e.Reason != "" {
		return e.Reason
	}
	return "pinpoint validation"
}

func (e *PinpointValidationException) Unwrap() error {
	return e.Err
}

type Client struct {
	cfg             *config.Config
	defaultClient   senderAPI
	dedicatedClient senderAPI
}

var _ client.SMSSender = (*Client)(nil)

func NewClient(cfg *config.Config, defaultClient senderAPI, dedicatedClient senderAPI) *Client {
	return &Client{cfg: cfg, defaultClient: defaultClient, dedicatedClient: dedicatedClient}
}

func (c *Client) SendSMS(ctx context.Context, request client.SMSRequest) (string, error) {
	normalized, international, err := normalizeDestination(request.To)
	if err != nil {
		return "", err
	}

	input := SendTextMessageInput{
		DestinationPhoneNumber: normalized,
		MessageBody:            request.Content,
		MessageType:            transactionalMessageType,
		ConfigurationSetName:   c.cfg.AWSPinpointConfigSet,
		Context: map[string]string{
			"reference": request.Reference,
		},
	}

	clientToUse := c.defaultClient
	hasDedicatedSender := strings.HasPrefix(strings.TrimSpace(request.Sender), "+1")
	if hasDedicatedSender && c.cfg.FFUsePinpointForDedicated && c.dedicatedClient != nil {
		clientToUse = c.dedicatedClient
		input.OriginationIdentity = request.Sender
		input.HasOriginationIdentity = true
	} else if !international {
		identity := c.originationIdentity(request.TemplateID, request.ServiceID, request.SendingVehicle)
		if identity != "" {
			input.OriginationIdentity = identity
			input.HasOriginationIdentity = true
		}
		if normalized == config.ExternalTestNumber {
			input.DryRun = true
			input.HasDryRun = true
		}
	}

	response, err := clientToUse.SendTextMessage(ctx, input)
	if err == nil {
		return response, nil
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return "", err
	}

	switch apiErr.Code {
	case "ConflictException":
		if apiErr.Reason == "DESTINATION_PHONE_NUMBER_OPTED_OUT" {
			return "opted_out", nil
		}
		return "", &PinpointConflictException{Reason: apiErr.Reason, Err: err}
	case "ValidationException":
		return "", &PinpointValidationException{Reason: apiErr.Reason, Err: err}
	default:
		return "", err
	}
}

func (c *Client) originationIdentity(templateID, serviceID uuid.UUID, sendingVehicle string) string {
	if strings.EqualFold(sendingVehicle, "short_code") {
		return c.cfg.AWSPinpointSCPoolID
	}
	if templateID != uuid.Nil && serviceID == uuid.MustParse(config.NotifyServiceID) {
		for _, configuredID := range c.cfg.AWSPinpointSCTemplateIDs {
			if strings.TrimSpace(configuredID) == templateID.String() {
				return c.cfg.AWSPinpointSCPoolID
			}
		}
	}
	return c.cfg.AWSPinpointDefaultPoolID
}

func normalizeDestination(value string) (string, bool, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false, fmt.Errorf("No valid numbers found for SMS delivery")
	}

	digits := onlyDigits(trimmed)
	if len(digits) == 10 {
		return "+1" + digits, false, nil
	}
	if strings.HasPrefix(trimmed, "+") {
		return "+" + digits, !strings.HasPrefix(trimmed, "+1"), nil
	}
	if len(digits) == 11 && digits[0] == '1' {
		return "+" + digits, false, nil
	}
	return "+" + digits, true, nil
}

func onlyDigits(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
