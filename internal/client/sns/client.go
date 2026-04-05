package sns

import (
	"context"
	"fmt"
	"strings"

	clientpkg "github.com/maxneuvians/notification-api-spec/internal/client"
)

const transactionalMessageType = "Transactional"

type PublishInput struct {
	PhoneNumber       string
	Message           string
	MessageAttributes map[string]string
}

type publisherAPI interface {
	Publish(ctx context.Context, input PublishInput) error
}

type Client struct {
	client publisherAPI
}

var _ clientpkg.SMSSender = (*Client)(nil)

func NewClient(client publisherAPI) *Client {
	return &Client{client: client}
}

func (c *Client) SendSMS(ctx context.Context, request clientpkg.SMSRequest) (string, error) {
	normalized, err := normalizeDestination(request.To)
	if err != nil {
		return "", err
	}

	err = c.client.Publish(ctx, PublishInput{
		PhoneNumber: normalized,
		Message:     request.Content,
		MessageAttributes: map[string]string{
			"AWS.SNS.SMS.SMSType": transactionalMessageType,
			"reference":          request.Reference,
		},
	})
	if err != nil {
		return "", err
	}

	return "", nil
}

func normalizeDestination(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("No valid numbers found for SMS delivery")
	}

	digits := onlyDigits(trimmed)
	if len(digits) == 10 {
		return "+1" + digits, nil
	}
	if strings.HasPrefix(trimmed, "+") {
		return "+" + digits, nil
	}
	if len(digits) == 11 && digits[0] == '1' {
		return "+" + digits, nil
	}
	return "+" + digits, nil
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