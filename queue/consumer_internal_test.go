package queue

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

type queueCaptureHandler struct {
	records []string
}

func (h *queueCaptureHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *queueCaptureHandler) Handle(_ context.Context, record slog.Record) error {
	h.records = append(h.records, record.Message)
	return nil
}

func (h *queueCaptureHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *queueCaptureHandler) WithGroup(string) slog.Handler {
	return h
}

func TestTransientErrorMethods(t *testing.T) {
	inner := errors.New("temporary")
	err := &transientError{err: inner}

	if got := err.Error(); got != inner.Error() {
		t.Fatalf("Error() = %q, want %q", got, inner.Error())
	}
	if !errors.Is(err.Unwrap(), inner) {
		t.Fatal("Unwrap() did not return inner error")
	}
}

func TestConsumerLog(t *testing.T) {
	consumer := &SQSConsumer{}
	consumer.log("ignored")

	handler := &queueCaptureHandler{}
	consumer.logger = slog.New(handler)
	consumer.log("logged")

	if len(handler.records) != 1 || handler.records[0] != "logged" {
		t.Fatalf("log records = %#v, want [logged]", handler.records)
	}
}
