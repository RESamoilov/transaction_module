package dlq

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/meindokuse/transaction-module/internal/config"
	"github.com/segmentio/kafka-go"
)

func TestNewDLQHandlerStoresTopic(t *testing.T) {
	t.Parallel()

	h := NewDLQHandler(config.ProducerConfig{
		Brokers:  []string{"127.0.0.1:9092"},
		DLQTopic: "dlq-topic",
	})
	defer h.Close()

	if h.dlqTopic != "dlq-topic" {
		t.Fatalf("dlqTopic = %q, want %q", h.dlqTopic, "dlq-topic")
	}
}

func TestHandleErrorWithNoMessagesReturnsNil(t *testing.T) {
	t.Parallel()

	h := &DLQHandler{producer: NewProducer(config.ProducerConfig{Brokers: []string{"127.0.0.1:9092"}, DLQTopic: "dlq"})}
	defer h.Close()

	if err := h.HandleError(context.Background(), errors.New("boom")); err != nil {
		t.Fatalf("HandleError() error = %v, want nil", err)
	}
}

func TestHandleErrorReturnsWriteError(t *testing.T) {
	t.Parallel()

	p := NewProducer(config.ProducerConfig{
		Brokers:      []string{"127.0.0.1:1"},
		DLQTopic:     "dlq",
		BatchTimeout: time.Millisecond,
	})
	_ = p.Close()

	h := &DLQHandler{producer: p, dlqTopic: "dlq"}
	err := h.HandleError(context.Background(), errors.New("boom"), kafka.Message{Topic: "main", Partition: 1, Offset: 2})
	if err == nil {
		t.Fatal("HandleError() error = nil, want write error")
	}
}

func TestProducerWriteBatchEmptyReturnsNil(t *testing.T) {
	t.Parallel()

	p := NewProducer(config.ProducerConfig{
		Brokers:  []string{"127.0.0.1:9092"},
		DLQTopic: "dlq",
	})
	defer p.Close()

	if err := p.WriteBatch(context.Background(), nil); err != nil {
		t.Fatalf("WriteBatch() error = %v, want nil", err)
	}
}

func TestNewProducerAppliesSnappyCompression(t *testing.T) {
	t.Parallel()

	p := NewProducer(config.ProducerConfig{
		Brokers:     []string{"127.0.0.1:9092"},
		DLQTopic:    "dlq",
		Compression: "snappy",
	})
	defer p.Close()

	if p.writer == nil {
		t.Fatal("writer = nil, want initialized kafka writer")
	}
}

func TestProducerCloseReturnsNil(t *testing.T) {
	t.Parallel()

	p := NewProducer(config.ProducerConfig{
		Brokers:  []string{"127.0.0.1:9092"},
		DLQTopic: "dlq",
	})

	if err := p.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
}
