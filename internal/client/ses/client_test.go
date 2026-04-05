package ses

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	clientpkg "github.com/maxneuvians/notification-api-spec/internal/client"
)

type stubSESAPI struct {
	input SendRawEmailInput
	err   error

	called bool
}

func (s *stubSESAPI) SendRawEmail(_ context.Context, input SendRawEmailInput) error {
	s.called = true
	s.input = input
	return s.err
}

func TestSendEmailMultipartWithoutAttachments(t *testing.T) {
	api := &stubSESAPI{}
	client := NewClient(api)

	err := client.SendEmail(context.Background(), clientpkg.EmailRequest{
		Source:      "sender@example.com",
		ToAddresses: []string{"recipient@example.com"},
		Subject:     "Subject",
		Body:        "plain body",
		HTMLBody:    "<p>html body</p>",
	})
	if err != nil {
		t.Fatalf("SendEmail() error = %v", err)
	}
	raw := string(api.input.RawMessage)
	if !strings.Contains(raw, "Content-Type: multipart/alternative;") {
		t.Fatalf("raw message missing multipart/alternative: %s", raw)
	}
	if !strings.Contains(raw, "Content-Type: text/plain; charset=UTF-8") || !strings.Contains(raw, "Content-Type: text/html; charset=UTF-8") {
		t.Fatalf("raw message missing text parts: %s", raw)
	}
	if !api.called {
		t.Fatal("expected SendRawEmail call")
	}
}

func TestSendEmailMultipartWithAttachments(t *testing.T) {
	api := &stubSESAPI{}
	client := NewClient(api)

	err := client.SendEmail(context.Background(), clientpkg.EmailRequest{
		Source:      "sender@example.com",
		ToAddresses: []string{"recipient@example.com"},
		Subject:     "Subject",
		Body:        "plain body",
		HTMLBody:    "<p>html body</p>",
		Attachments: []clientpkg.Attachment{{Name: "test.txt", Data: []byte("hello"), MimeType: "text/plain"}},
	})
	if err != nil {
		t.Fatalf("SendEmail() error = %v", err)
	}
	raw := string(api.input.RawMessage)
	if !strings.Contains(raw, "Content-Type: multipart/mixed;") {
		t.Fatalf("raw message missing multipart/mixed: %s", raw)
	}
	if !strings.Contains(raw, "Content-Disposition: attachment; filename=\"test.txt\"") {
		t.Fatalf("raw message missing attachment disposition: %s", raw)
	}
	if !strings.Contains(raw, "Content-Transfer-Encoding: base64") || !strings.Contains(raw, "aGVsbG8=") {
		t.Fatalf("raw message missing base64 attachment: %s", raw)
	}
}

func TestReplyToHandling(t *testing.T) {
	api := &stubSESAPI{}
	client := NewClient(api)

	err := client.SendEmail(context.Background(), clientpkg.EmailRequest{
		Source:         "sender@example.com",
		ToAddresses:    []string{"recipient@example.com"},
		Subject:        "Subject",
		Body:           "plain body",
		HTMLBody:       "<p>html body</p>",
		ReplyToAddress: nil,
	})
	if err != nil {
		t.Fatalf("SendEmail() error = %v", err)
	}
	if strings.Contains(string(api.input.RawMessage), "Reply-To:") {
		t.Fatal("did not expect Reply-To header")
	}

	asciiReplyTo := "support@example.com"
	err = client.SendEmail(context.Background(), clientpkg.EmailRequest{
		Source:         "sender@example.com",
		ToAddresses:    []string{"recipient@example.com"},
		Subject:        "Subject",
		Body:           "plain body",
		HTMLBody:       "<p>html body</p>",
		ReplyToAddress: &asciiReplyTo,
	})
	if err != nil {
		t.Fatalf("SendEmail() error = %v", err)
	}
	if !strings.Contains(string(api.input.RawMessage), "Reply-To: support@example.com") {
		t.Fatalf("expected ascii Reply-To header, got %s", string(api.input.RawMessage))
	}

	idnReplyTo := "user@例え.jp"
	err = client.SendEmail(context.Background(), clientpkg.EmailRequest{
		Source:         "sender@example.com",
		ToAddresses:    []string{"recipient@example.com"},
		Subject:        "Subject",
		Body:           "plain body",
		HTMLBody:       "<p>html body</p>",
		ReplyToAddress: &idnReplyTo,
	})
	if err != nil {
		t.Fatalf("SendEmail() error = %v", err)
	}
	if !strings.Contains(string(api.input.RawMessage), "Reply-To: =?utf-8?b?") && !strings.Contains(string(api.input.RawMessage), "Reply-To: =?UTF-8?B?") {
		t.Fatalf("expected encoded Reply-To header, got %s", string(api.input.RawMessage))
	}
	if !bytes.Contains(api.input.RawMessage, []byte("dXNlckB4bi0tcjhqejQ1Zy5qcA==")) {
		t.Fatalf("expected encoded punycode address in raw message, got %s", string(api.input.RawMessage))
	}
}

func TestToAddressPunycode(t *testing.T) {
	api := &stubSESAPI{}
	client := NewClient(api)

	err := client.SendEmail(context.Background(), clientpkg.EmailRequest{
		Source:      "sender@example.com",
		ToAddresses: []string{"user@例え.jp"},
		Subject:     "Subject",
		Body:        "plain body",
		HTMLBody:    "<p>html body</p>",
	})
	if err != nil {
		t.Fatalf("SendEmail() error = %v", err)
	}
	if !bytes.Contains(api.input.RawMessage, []byte("To: user@xn--r8jz45g.jp")) {
		t.Fatalf("expected punycoded To header, got %s", string(api.input.RawMessage))
	}
}

func TestClientErrorMapping(t *testing.T) {
	t.Run("invalid parameter raises InvalidEmailError", func(t *testing.T) {
		api := &stubSESAPI{err: &ClientError{Code: "InvalidParameterValue", Message: "bad address"}}
		client := NewClient(api)
		err := client.SendEmail(context.Background(), clientpkg.EmailRequest{Source: "sender@example.com", ToAddresses: []string{"recipient@example.com"}, Subject: "Subject", Body: "plain", HTMLBody: "html"})
		var target *InvalidEmailError
		if !errors.As(err, &target) || target.Message != "bad address" {
			t.Fatalf("error = %v, want InvalidEmailError", err)
		}
	})

	t.Run("other client errors raise AwsSesClientException", func(t *testing.T) {
		api := &stubSESAPI{err: &ClientError{Code: "Throttling", Message: "slow down"}}
		client := NewClient(api)
		err := client.SendEmail(context.Background(), clientpkg.EmailRequest{Source: "sender@example.com", ToAddresses: []string{"recipient@example.com"}, Subject: "Subject", Body: "plain", HTMLBody: "html"})
		var target *AwsSesClientException
		if !errors.As(err, &target) || target.Message != "slow down" {
			t.Fatalf("error = %v, want AwsSesClientException", err)
		}
	})
}

func TestPunycodeEncodeEmail(t *testing.T) {
	got, err := punycodeEncodeEmail("user@example.com")
	if err != nil {
		t.Fatalf("punycodeEncodeEmail() error = %v", err)
	}
	if got != "user@example.com" {
		t.Fatalf("got %q, want unchanged ascii", got)
	}

	got, err = punycodeEncodeEmail("user@例え.jp")
	if err != nil {
		t.Fatalf("punycodeEncodeEmail() error = %v", err)
	}
	if got != "user@xn--r8jz45g.jp" {
		t.Fatalf("got %q, want punycoded domain", got)
	}
}
