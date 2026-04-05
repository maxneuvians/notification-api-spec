package queue_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/maxneuvians/notification-api-spec/queue"
)

type fakeConsumerClient struct {
	mu                    sync.Mutex
	receiveOutputs        []*sqs.ReceiveMessageOutput
	receiveErr            error
	deleteCalls           int
	changeVisibilityCalls int
	done                  chan struct{}
}

func (f *fakeConsumerClient) ReceiveMessage(ctx context.Context, _ *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.receiveErr != nil {
		return nil, f.receiveErr
	}
	if len(f.receiveOutputs) == 0 {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	out := f.receiveOutputs[0]
	f.receiveOutputs = f.receiveOutputs[1:]
	return out, nil
}

func (f *fakeConsumerClient) DeleteMessage(context.Context, *sqs.DeleteMessageInput, ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteCalls++
	if f.done != nil {
		close(f.done)
		f.done = nil
	}
	return &sqs.DeleteMessageOutput{}, nil
}

func (f *fakeConsumerClient) ChangeMessageVisibility(context.Context, *sqs.ChangeMessageVisibilityInput, ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.changeVisibilityCalls++
	if f.done != nil {
		close(f.done)
		f.done = nil
	}
	return &sqs.ChangeMessageVisibilityOutput{}, nil
}

func TestConsumerDeletesMessageOnSuccess(t *testing.T) {
	client := &fakeConsumerClient{
		receiveOutputs: []*sqs.ReceiveMessageOutput{{
			Messages: []types.Message{{
				MessageId:     aws.String("1"),
				Body:          aws.String("hello"),
				ReceiptHandle: aws.String("rh-1"),
			}},
		}},
		done: make(chan struct{}),
	}

	consumer := queue.NewSQSConsumer(client, "queue-url", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go consumer.Start(ctx, func(context.Context, *queue.Message) error {
		return nil
	})

	select {
	case <-client.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for delete")
	}

	consumer.Stop()

	if client.deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", client.deleteCalls)
	}
}

func TestConsumerExtendsVisibilityOnTransientError(t *testing.T) {
	client := &fakeConsumerClient{
		receiveOutputs: []*sqs.ReceiveMessageOutput{{
			Messages: []types.Message{{
				MessageId:     aws.String("1"),
				Body:          aws.String("hello"),
				ReceiptHandle: aws.String("rh-1"),
				Attributes: map[string]string{
					string(types.MessageSystemAttributeNameApproximateReceiveCount): "2",
				},
			}},
		}},
		done: make(chan struct{}),
	}

	consumer := queue.NewSQSConsumer(client, "queue-url", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go consumer.Start(ctx, func(context.Context, *queue.Message) error {
		return queue.Transient(errors.New("temporary"))
	})

	select {
	case <-client.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for visibility change")
	}

	consumer.Stop()

	if client.changeVisibilityCalls != 1 {
		t.Fatalf("change visibility calls = %d, want 1", client.changeVisibilityCalls)
	}
}

func TestConsumerStopsOnContextCancellation(t *testing.T) {
	client := &fakeConsumerClient{}
	consumer := queue.NewSQSConsumer(client, "queue-url", nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		consumer.Start(ctx, func(context.Context, *queue.Message) error { return nil })
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("consumer did not stop after cancellation")
	}
}
