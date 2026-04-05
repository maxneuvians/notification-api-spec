package queue

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type Message struct {
	ID                string
	Body              string
	ReceiptHandle     string
	Attributes        map[string]string
	MessageAttributes map[string]string
}

type Consumer interface {
	Start(ctx context.Context, handler func(ctx context.Context, msg *Message) error)
	Stop()
}

type transientError struct {
	err error
}

func (e *transientError) Error() string {
	return e.err.Error()
}

func (e *transientError) Unwrap() error {
	return e.err
}

func Transient(err error) error {
	if err == nil {
		return nil
	}

	return &transientError{err: err}
}

type sqsReceiveClient interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
}

type SQSConsumer struct {
	client            sqsReceiveClient
	queueURL          string
	waitTimeSeconds   int32
	maxMessages       int32
	visibilityTimeout int32
	logger            *slog.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewSQSConsumer(client sqsReceiveClient, queueURL string, logger *slog.Logger) *SQSConsumer {
	return &SQSConsumer{
		client:            client,
		queueURL:          queueURL,
		waitTimeSeconds:   20,
		maxMessages:       10,
		visibilityTimeout: 310,
		logger:            logger,
	}
}

func (c *SQSConsumer) Start(ctx context.Context, handler func(ctx context.Context, msg *Message) error) {
	ctx, cancel := context.WithCancel(ctx)
	c.setCancel(cancel)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		out, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(c.queueURL),
			WaitTimeSeconds:     c.waitTimeSeconds,
			MaxNumberOfMessages: c.maxMessages,
			VisibilityTimeout:   c.visibilityTimeout,
			MessageAttributeNames: []string{
				"All",
			},
			AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameAll},
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			c.log("receive_error", slog.String("error", err.Error()))
			continue
		}

		for _, raw := range out.Messages {
			msg := &Message{
				ID:                aws.ToString(raw.MessageId),
				Body:              aws.ToString(raw.Body),
				ReceiptHandle:     aws.ToString(raw.ReceiptHandle),
				Attributes:        raw.Attributes,
				MessageAttributes: flattenAttributes(raw.MessageAttributes),
			}

			err := handler(ctx, msg)
			switch {
			case err == nil:
				_, deleteErr := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(c.queueURL),
					ReceiptHandle: raw.ReceiptHandle,
				})
				if deleteErr != nil {
					c.log("delete_error", slog.String("error", deleteErr.Error()))
				}
			case isTransient(err):
				_, visErr := c.client.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
					QueueUrl:          aws.String(c.queueURL),
					ReceiptHandle:     raw.ReceiptHandle,
					VisibilityTimeout: backoffSeconds(raw.Attributes),
				})
				if visErr != nil {
					c.log("visibility_error", slog.String("error", visErr.Error()))
				}
			default:
				c.log("handler_error", slog.String("error", err.Error()))
			}
		}
	}
}

func (c *SQSConsumer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *SQSConsumer) setCancel(cancel context.CancelFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancel = cancel
}

func (c *SQSConsumer) log(message string, args ...any) {
	if c.logger == nil {
		return
	}
	c.logger.Warn(message, args...)
}

func isTransient(err error) bool {
	var target *transientError
	return errors.As(err, &target)
}

func backoffSeconds(attrs map[string]string) int32 {
	receiveCount, _ := strconv.Atoi(attrs[string(types.MessageSystemAttributeNameApproximateReceiveCount)])
	if receiveCount <= 0 {
		receiveCount = 1
	}

	backoff := time.Duration(1<<min(receiveCount-1, 5)) * time.Minute
	seconds := int32(backoff / time.Second)
	if seconds > 900 {
		return 900
	}
	return seconds
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func flattenAttributes(attrs map[string]types.MessageAttributeValue) map[string]string {
	if len(attrs) == 0 {
		return nil
	}

	flattened := make(map[string]string, len(attrs))
	for key, value := range attrs {
		flattened[key] = aws.ToString(value.StringValue)
	}
	return flattened
}
