package queue

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type Producer interface {
	Send(ctx context.Context, queueURL, body string, attrs map[string]string) error
	SendFIFO(ctx context.Context, queueURL, body, groupID, deduplicationID string) error
}

type sqsSendClient interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

type SQSProducer struct {
	client sqsSendClient
}

func NewSQSProducer(client sqsSendClient) *SQSProducer {
	return &SQSProducer{client: client}
}

func (p *SQSProducer) Send(ctx context.Context, queueURL, body string, attrs map[string]string) error {
	_, err := p.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:          aws.String(queueURL),
		MessageBody:       aws.String(body),
		MessageAttributes: expandMessageAttributes(attrs),
	})
	return err
}

func (p *SQSProducer) SendFIFO(ctx context.Context, queueURL, body, groupID, deduplicationID string) error {
	_, err := p.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:               aws.String(queueURL),
		MessageBody:            aws.String(body),
		MessageGroupId:         aws.String(groupID),
		MessageDeduplicationId: aws.String(deduplicationID),
	})
	return err
}

func expandMessageAttributes(attrs map[string]string) map[string]types.MessageAttributeValue {
	if len(attrs) == 0 {
		return nil
	}

	result := make(map[string]types.MessageAttributeValue, len(attrs))
	for key, value := range attrs {
		result[key] = types.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(value),
		}
	}

	return result
}
