package ses

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"

	"golang.org/x/net/idna"

	clientpkg "github.com/maxneuvians/notification-api-spec/internal/client"
)

type SendRawEmailInput struct {
	RawMessage []byte
}

type senderAPI interface {
	SendRawEmail(ctx context.Context, input SendRawEmailInput) error
}

type ClientError struct {
	Code    string
	Message string
	Err     error
}

func (e *ClientError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *ClientError) Unwrap() error {
	return e.Err
}

type InvalidEmailError struct {
	Message string
	Err     error
}

func (e *InvalidEmailError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "invalid email"
}

func (e *InvalidEmailError) Unwrap() error {
	return e.Err
}

type AwsSesClientException struct {
	Message string
	Err     error
}

func (e *AwsSesClientException) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "ses client error"
}

func (e *AwsSesClientException) Unwrap() error {
	return e.Err
}

type Client struct {
	client senderAPI
}

var _ clientpkg.EmailSender = (*Client)(nil)

func NewClient(client senderAPI) *Client {
	return &Client{client: client}
}

func (c *Client) SendEmail(ctx context.Context, request clientpkg.EmailRequest) error {
	rawMessage, err := buildRawMessage(request)
	if err != nil {
		return err
	}

	err = c.client.SendRawEmail(ctx, SendRawEmailInput{RawMessage: rawMessage})
	if err == nil {
		return nil
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		return err
	}

	if clientErr.Code == "InvalidParameterValue" {
		return &InvalidEmailError{Message: clientErr.Message, Err: err}
	}

	return &AwsSesClientException{Message: clientErr.Message, Err: err}
}

func buildRawMessage(request clientpkg.EmailRequest) ([]byte, error) {
	from, err := punycodeEncodeEmail(strings.TrimSpace(request.Source))
	if err != nil {
		return nil, err
	}

	toAddresses := make([]string, 0, len(request.ToAddresses))
	for _, address := range request.ToAddresses {
		encoded, err := punycodeEncodeEmail(strings.TrimSpace(address))
		if err != nil {
			return nil, err
		}
		toAddresses = append(toAddresses, encoded)
	}

	replyTo, includeReplyTo, err := formatReplyToHeader(request.ReplyToAddress)
	if err != nil {
		return nil, err
	}

	body, contentType, err := buildMessageBody(request)
	if err != nil {
		return nil, err
	}

	var raw bytes.Buffer
	writeHeader(&raw, "From", from)
	writeHeader(&raw, "To", strings.Join(toAddresses, ", "))
	writeHeader(&raw, "Subject", request.Subject)
	writeHeader(&raw, "MIME-Version", "1.0")
	if includeReplyTo {
		writeHeader(&raw, "Reply-To", replyTo)
	}
	writeHeader(&raw, "Content-Type", contentType)
	raw.WriteString("\r\n")
	raw.Write(body)
	return raw.Bytes(), nil
}

func buildMessageBody(request clientpkg.EmailRequest) ([]byte, string, error) {
	var body bytes.Buffer

	if len(request.Attachments) == 0 {
		alternativeWriter := multipart.NewWriter(&body)
		if err := writeTextPart(alternativeWriter, "text/plain; charset=UTF-8", request.Body); err != nil {
			return nil, "", err
		}
		if err := writeTextPart(alternativeWriter, "text/html; charset=UTF-8", request.HTMLBody); err != nil {
			return nil, "", err
		}
		if err := alternativeWriter.Close(); err != nil {
			return nil, "", err
		}
		return body.Bytes(), fmt.Sprintf("multipart/alternative; boundary=%q", alternativeWriter.Boundary()), nil
	}

	mixedWriter := multipart.NewWriter(&body)
	alternativeBuffer := &bytes.Buffer{}
	alternativeWriter := multipart.NewWriter(alternativeBuffer)
	alternativeHeader := textproto.MIMEHeader{}
	alternativeHeader.Set("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", alternativeWriter.Boundary()))
	alternativePart, err := mixedWriter.CreatePart(alternativeHeader)
	if err != nil {
		return nil, "", err
	}

	if err := writeTextPart(alternativeWriter, "text/plain; charset=UTF-8", request.Body); err != nil {
		return nil, "", err
	}
	if err := writeTextPart(alternativeWriter, "text/html; charset=UTF-8", request.HTMLBody); err != nil {
		return nil, "", err
	}
	if err := alternativeWriter.Close(); err != nil {
		return nil, "", err
	}
	if _, err := alternativePart.Write(alternativeBuffer.Bytes()); err != nil {
		return nil, "", err
	}

	for _, attachment := range request.Attachments {
		if err := writeAttachmentPart(mixedWriter, attachment); err != nil {
			return nil, "", err
		}
	}

	if err := mixedWriter.Close(); err != nil {
		return nil, "", err
	}

	return body.Bytes(), fmt.Sprintf("multipart/mixed; boundary=%q", mixedWriter.Boundary()), nil
}

func writeTextPart(writer *multipart.Writer, contentType, value string) error {
	header := textproto.MIMEHeader{}
	header.Set("Content-Type", contentType)
	header.Set("Content-Transfer-Encoding", "quoted-printable")
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = part.Write([]byte(value))
	return err
}

func writeAttachmentPart(writer *multipart.Writer, attachment clientpkg.Attachment) error {
	header := textproto.MIMEHeader{}
	mimeType := strings.TrimSpace(attachment.MimeType)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	header.Set("Content-Type", mimeType)
	header.Set("Content-Transfer-Encoding", "base64")
	header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", attachment.Name))
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	encoder := base64.NewEncoder(base64.StdEncoding, part)
	if _, err := encoder.Write(attachment.Data); err != nil {
		return err
	}
	return encoder.Close()
}

func formatReplyToHeader(replyToAddress *string) (string, bool, error) {
	if replyToAddress == nil {
		return "", false, nil
	}

	trimmed := strings.TrimSpace(*replyToAddress)
	if trimmed == "" {
		return "", false, nil
	}

	encoded, err := punycodeEncodeEmail(trimmed)
	if err != nil {
		return "", false, err
	}
	if isASCII(trimmed) {
		return encoded, true, nil
	}
	return fmt.Sprintf("=?utf-8?b?%s?=", base64.StdEncoding.EncodeToString([]byte(encoded))), true, nil
}

func punycodeEncodeEmail(address string) (string, error) {
	trimmed := strings.TrimSpace(address)
	parts := strings.Split(trimmed, "@")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", fmt.Errorf("invalid email address: %s", address)
	}

	domain, err := idna.Lookup.ToASCII(parts[1])
	if err != nil {
		return "", err
	}

	return parts[0] + "@" + domain, nil
}

func isASCII(value string) bool {
	for _, r := range value {
		if r > 127 {
			return false
		}
	}
	return true
}

func writeHeader(buffer *bytes.Buffer, key, value string) {
	buffer.WriteString(key)
	buffer.WriteString(": ")
	buffer.WriteString(value)
	buffer.WriteString("\r\n")
}
