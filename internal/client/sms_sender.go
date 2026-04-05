package client

import (
	"context"

	"github.com/google/uuid"
)

type SMSRequest struct {
	To             string
	Content        string
	Reference      string
	TemplateID     uuid.UUID
	ServiceID      uuid.UUID
	Sender         string
	SendingVehicle string
}

type SMSSender interface {
	SendSMS(ctx context.Context, request SMSRequest) (string, error)
}