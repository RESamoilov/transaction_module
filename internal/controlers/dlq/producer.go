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
		Balancer:     &kafka.Murmur2Balancer{}, // Классический алгоритм хэширования партиций
		MaxAttempts:  cfg.MaxAttempts,
		BatchSize:    cfg.BatchSize,
		BatchTimeout: cfg.BatchTimeout,
		RequiredAcks: kafka.RequireAll, // Гарантия записи на все реплики брокера (-1)
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Async:        false, // Мы хотим знать, точно ли записался батч
	}

	if cfg.Compression == "snappy" {
		w.Compression = kafka.Compression(snappy.NewCompressionCodec().Compression)
	}

	return &Producer{writer: w}
}

// WriteBatch отправляет пачку сообщений в Kafka
func (p *Producer) WriteBatch(ctx context.Context, msgs []kafka.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	start := time.Now()

	err := p.writer.WriteMessages(ctx, msgs...)

	if err != nil {
		return fmt.Errorf("failed to write messages to kafka: %w", err)
	}

	slog.Debug("Batch produced to Kafka",
		"count", len(msgs),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil
}

func (p *Producer) Close() error {
	slog.Info("Closing Kafka Producer")
	return p.writer.Close()
}
