package client

import "context"

type Attachment struct {
	Name     string
	Data     []byte
	MimeType string
}

type EmailRequest struct {
	Source         string
	ToAddresses    []string
	Subject        string
	Body           string
	HTMLBody       string
	ReplyToAddress *string
	Attachments    []Attachment
}

type EmailSender interface {
	SendEmail(ctx context.Context, request EmailRequest) error
}