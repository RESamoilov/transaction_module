package dlq

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/meindokuse/transaction-module/internal/config"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/snappy"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(cfg config.ProducerConfig) *Producer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.DLQTopic,
		Balancer:     &kafka.Murmur2Balancer{},
		MaxAttempts:  cfg.MaxAttempts,
		BatchSize:    cfg.BatchSize,
		BatchTimeout: cfg.BatchTimeout,
		RequiredAcks: kafka.RequireAll,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Async:        false,
	}

	if cfg.Compression == "snappy" {
		w.Compression = kafka.Compression(snappy.NewCompressionCodec().Compression)
	}

	return &Producer{writer: w}
}

// WriteBatch publishes a batch of messages to the configured Kafka topic.
func (p *Producer) WriteBatch(ctx context.Context, msgs []kafka.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	start := time.Now()

	err := p.writer.WriteMessages(ctx, msgs...)
	if err != nil {
		return fmt.Errorf("failed to write messages to kafka: %w", err)
	}

	slog.DebugContext(ctx, "batch produced to kafka",
		"count", len(msgs),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil
}

func (p *Producer) Close() error {
	slog.Info("Closing Kafka Producer")
	return p.writer.Close()
}
