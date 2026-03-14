package dlq

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/meindokuse/transaction-module/internal/config"
	"github.com/segmentio/kafka-go"
)

type DLQHandler struct {
	producer *Producer
	dlqTopic string
}

func NewDLQHandler(cfg config.ProducerConfig) *DLQHandler {
	p := NewProducer(cfg)

	return &DLQHandler{
		producer: p,
		dlqTopic: cfg.DLQTopic,
	}
}

// HandleError обрабатывает одно или несколько битых сообщений, обогащает их и отправляет в DLQ
func (d *DLQHandler) HandleError(ctx context.Context, originalErr error, msgs ...kafka.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	dlqMsgs := make([]kafka.Message, 0, len(msgs))
	errStr := originalErr.Error()
	timestamp := time.Now().UTC().Format(time.RFC3339)

	for _, msg := range msgs {
		headers := make([]kafka.Header, 0, len(msg.Headers)+5)
		headers = append(headers, msg.Headers...)

		headers = append(headers,
			kafka.Header{Key: "dlq_error_reason", Value: []byte(errStr)},
			kafka.Header{Key: "dlq_original_topic", Value: []byte(msg.Topic)},
			kafka.Header{Key: "dlq_original_partition", Value: []byte(strconv.Itoa(msg.Partition))},
			kafka.Header{Key: "dlq_original_offset", Value: []byte(strconv.FormatInt(msg.Offset, 10))},
			kafka.Header{Key: "dlq_timestamp", Value: []byte(timestamp)},
		)

		dlqMsgs = append(dlqMsgs, kafka.Message{
			Key:     msg.Key,
			Value:   msg.Value,
			Headers: headers,
		})
	}

	// Отправляем все сообщения одним батчем в Kafka
	err := d.producer.WriteBatch(ctx, dlqMsgs)
	if err != nil {
		return fmt.Errorf("failed to write %d messages to DLQ %s: %w", len(msgs), d.dlqTopic, err)
	}

	slog.Info("successfully routed messages to DLQ",
		"topic", d.dlqTopic,
		"count", len(msgs),
		"reason", errStr,
	)

	return nil
}

func (d *DLQHandler) Close() error {
	return d.producer.Close()
}
