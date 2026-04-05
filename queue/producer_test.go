package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type fakeSendClient struct {
	input *sqs.SendMessageInput
	err   error
}

func (f *fakeSendClient) SendMessage(ctx context.Context, params *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.input = params
	if f.err != nil {
		return nil, f.err
	}
	return &sqs.SendMessageOutput{}, nil
}

func TestSQSProducerSend(t *testing.T) {
	client := &fakeSendClient{}
	producer := NewSQSProducer(client)

	err := producer.Send(context.Background(), "queue-url", "hello", map[string]string{"kind": "welcome"})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}

	if got := aws.ToString(client.input.QueueUrl); got != "queue-url" {
		t.Fatalf("queue url = %q, want queue-url", got)
	}
	if got := aws.ToString(client.input.MessageBody); got != "hello" {
		t.Fatalf("message body = %q, want hello", got)
	}
	if got := aws.ToString(client.input.MessageAttributes["kind"].StringValue); got != "welcome" {
		t.Fatalf("kind attr = %q, want welcome", got)
	}

	client.err = errors.New("send failed")
	if err := producer.Send(context.Background(), "queue-url", "hello", nil); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSQSProducerSendFIFO(t *testing.T) {
	client := &fakeSendClient{}
	producer := NewSQSProducer(client)

	err := producer.SendFIFO(context.Background(), "fifo-url", "hello", "group-1", "dedupe-1")
	if err != nil {
		t.Fatalf("SendFIFO() error = %v, want nil", err)
	}

	if got := aws.ToString(client.input.MessageGroupId); got != "group-1" {
		t.Fatalf("group id = %q, want group-1", got)
	}
	if got := aws.ToString(client.input.MessageDeduplicationId); got != "dedupe-1" {
		t.Fatalf("dedupe id = %q, want dedupe-1", got)
	}
}

func TestExpandMessageAttributes(t *testing.T) {
	if got := expandMessageAttributes(nil); got != nil {
		t.Fatalf("expandMessageAttributes(nil) = %#v, want nil", got)
	}

	attrs := expandMessageAttributes(map[string]string{"a": "b"})
	if got := aws.ToString(attrs["a"].DataType); got != "String" {
		t.Fatalf("data type = %q, want String", got)
	}
	if got := aws.ToString(attrs["a"].StringValue); got != "b" {
		t.Fatalf("string value = %q, want b", got)
	}
}

func TestTransientHelpers(t *testing.T) {
	if got := Transient(nil); got != nil {
		t.Fatalf("Transient(nil) = %#v, want nil", got)
	}

	err := Transient(errors.New("temporary"))
	if err == nil {
		t.Fatal("expected wrapped error")
	}
	if !isTransient(err) {
		t.Fatal("expected transient error")
	}
	if isTransient(errors.New("plain")) {
		t.Fatal("did not expect plain error to be transient")
	}
}

func TestBackoffSeconds(t *testing.T) {
	if got := backoffSeconds(nil); got != 60 {
		t.Fatalf("backoffSeconds(nil) = %d, want 60", got)
	}

	attrs := map[string]string{string(types.MessageSystemAttributeNameApproximateReceiveCount): "2"}
	if got := backoffSeconds(attrs); got != 120 {
		t.Fatalf("backoffSeconds(receiveCount=2) = %d, want 120", got)
	}

	attrs[string(types.MessageSystemAttributeNameApproximateReceiveCount)] = "99"
	if got := backoffSeconds(attrs); got != 900 {
		t.Fatalf("backoffSeconds(receiveCount=99) = %d, want 900", got)
	}
}

func TestMin(t *testing.T) {
	if got := min(1, 2); got != 1 {
		t.Fatalf("min(1, 2) = %d, want 1", got)
	}
	if got := min(3, 2); got != 2 {
		t.Fatalf("min(3, 2) = %d, want 2", got)
	}
}

func TestFlattenAttributes(t *testing.T) {
	if got := flattenAttributes(nil); got != nil {
		t.Fatalf("flattenAttributes(nil) = %#v, want nil", got)
	}

	flattened := flattenAttributes(map[string]types.MessageAttributeValue{
		"kind": {StringValue: aws.String("notice")},
	})
	if got := flattened["kind"]; got != "notice" {
		t.Fatalf("flattened kind = %q, want notice", got)
	}
}
