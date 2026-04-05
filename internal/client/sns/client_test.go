package sns

import (
	"context"
	"testing"

	clientpkg "github.com/maxneuvians/notification-api-spec/internal/client"
)

type stubPublisher struct {
	input  PublishInput
	err    error
	called bool
}

func (s *stubPublisher) Publish(_ context.Context, input PublishInput) error {
	s.called = true
	s.input = input
	return s.err
}

func TestSendSMSNormalizesPhoneAndSetsTransactionalAttributes(t *testing.T) {
	publisher := &stubPublisher{}
	client := NewClient(publisher)

	if _, err := client.SendSMS(context.Background(), clientpkg.SMSRequest{To: "6135550100", Content: "hello", Reference: "ref-1"}); err != nil {
		t.Fatalf("SendSMS() error = %v", err)
	}
	if !publisher.called {
		t.Fatal("expected Publish call")
	}
	if publisher.input.PhoneNumber != "+16135550100" {
		t.Fatalf("phone = %q, want +16135550100", publisher.input.PhoneNumber)
	}
	if publisher.input.MessageAttributes["AWS.SNS.SMS.SMSType"] != transactionalMessageType {
		t.Fatalf("message type = %q, want %q", publisher.input.MessageAttributes["AWS.SNS.SMS.SMSType"], transactionalMessageType)
	}
	if publisher.input.MessageAttributes["reference"] != "ref-1" {
		t.Fatalf("reference = %q, want ref-1", publisher.input.MessageAttributes["reference"])
	}
}

func TestSendSMSEmptyDestinationFails(t *testing.T) {
	client := NewClient(&stubPublisher{})
	if _, err := client.SendSMS(context.Background(), clientpkg.SMSRequest{}); err == nil || err.Error() != "No valid numbers found for SMS delivery" {
		t.Fatalf("SendSMS() error = %v, want destination validation error", err)
	}
}