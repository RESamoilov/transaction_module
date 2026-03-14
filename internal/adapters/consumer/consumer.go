package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/meindokuse/transaction-module/internal/config"
	"github.com/meindokuse/transaction-module/internal/domain"
	"github.com/meindokuse/transaction-module/internal/dto"
	"github.com/segmentio/kafka-go"
)

// BatchHandler интерфейс для сохранения пачки транзакций в БД
type BatchHandler func(ctx context.Context, txs []*domain.Transaction) error

// ErrorHandler абстрагирует логику работы с мертвыми письмами (DLQ).
// Принимает оригинальную ошибку и одно ИЛИ несколько сообщений.
type ErrorHandler interface {
	HandleError(ctx context.Context, originalErr error, msgs ...kafka.Message) error
}

type WorkerTask struct {
	Msg kafka.Message
	Tx  *domain.Transaction
}

type Consumer struct {
	reader       *kafka.Reader
	handler      BatchHandler
	errorHandler ErrorHandler
	cfg          config.ConsumerConfig
	validate     *validator.Validate

	workers []chan WorkerTask
	wg      sync.WaitGroup

	watermarks sync.Map

	shutdownCtx    context.Context
	setShutdownCtx sync.Once
}

func NewConsumer(cfg config.ConsumerConfig, handler BatchHandler, errHandler ErrorHandler, validate *validator.Validate) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		MinBytes:       cfg.MinBytes,
		MaxBytes:       cfg.MaxBytes,
		MaxWait:        cfg.MaxWait,
		ReadBackoffMin: cfg.ReadBackoffMin,
		ReadBackoffMax: cfg.ReadBackoffMax,
		CommitInterval: 0,
		StartOffset:    kafka.FirstOffset,
	})

	workers := make([]chan WorkerTask, cfg.WorkersCount)

	for i := 0; i < cfg.WorkersCount; i++ {
		workers[i] = make(chan WorkerTask, cfg.BatchSize*2)
	}

	return &Consumer{
		reader:       r,
		handler:      handler,
		errorHandler: errHandler,
		cfg:          cfg,
		workers:      workers,
		validate:     validate,
	}
}

func (c *Consumer) Start(ctx context.Context) {
	slog.InfoContext(ctx, "starting batch consumer", "topic", c.cfg.Topic, "workers", c.cfg.WorkersCount)

	for i := 0; i < c.cfg.WorkersCount; i++ {
		c.wg.Add(1)
		go c.batchWorkerLoop(ctx, i, c.workers[i])
	}

	defer func() {
		slog.InfoContext(ctx, "stopping consumer: closing worker channels from producer")
		for _, w := range c.workers {
			close(w)
		}
	}()

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				slog.InfoContext(ctx, "context canceled, stopping fetch loop")
				break
			}
			slog.ErrorContext(ctx, "failed to fetch message", "err", err)
			continue
		}

		trackerIface, _ := c.watermarks.LoadOrStore(msg.Partition, NewPartitionWatermark(msg.Offset))
		tracker := trackerIface.(*PartitionWatermark)

		var event dto.TransactionEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.ErrorContext(ctx, "poison pill: unmarshal failed", "err", err, "partition", msg.Partition, "offset", msg.Offset)
			c.handlePoisonPill(msg, tracker, fmt.Errorf("unmarshal error: %w", err))
			continue
		}

		if err := c.validate.Struct(event); err != nil {
			slog.ErrorContext(ctx, "poison pill: validation failed", "err", err, "partition", msg.Partition, "offset", msg.Offset, "transaction_id", event.ID)
			c.handlePoisonPill(msg, tracker, fmt.Errorf("validation error: %w", err))
			continue
		}

		parsedTime, err := time.Parse(time.RFC3339, event.Timestamp)
		if err != nil {
			slog.ErrorContext(ctx, "poison pill: invalid time format", "err", err, "partition", msg.Partition, "offset", msg.Offset, "transaction_id", event.ID)
			c.handlePoisonPill(msg, tracker, fmt.Errorf("invalid time format: %w", err))
			continue
		}

		tx := &domain.Transaction{
			ID:        event.ID,
			UserID:    event.UserID,
			Amount:    event.Amount,
			Type:      domain.TransactionType(event.Type),
			Timestamp: parsedTime,
		}

		workerID := hashUserID(tx.UserID) % uint32(c.cfg.WorkersCount)

		c.workers[workerID] <- WorkerTask{
			Msg: msg,
			Tx:  tx,
		}
	}
}

func (c *Consumer) handlePoisonPill(msg kafka.Message, tracker *PartitionWatermark, err error) {
	safeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if dlqErr := c.sendToDlq(safeCtx, err, msg); dlqErr == nil {
		c.commitProgress(safeCtx, msg, tracker)
	}
}

func (c *Consumer) batchWorkerLoop(ctx context.Context, workerID int, tasks <-chan WorkerTask) {
	defer c.wg.Done()

	batchMsg := make([]kafka.Message, 0, c.cfg.BatchSize)
	batchTx := make([]*domain.Transaction, 0, c.cfg.BatchSize)

	ticker := time.NewTicker(c.cfg.BatchTimeout)
	defer ticker.Stop()

	for {
		select {
		case task, ok := <-tasks:
			if !ok {
				if len(batchTx) > 0 {
					finalCtx := c.getShutdownCtx()
					c.processAndCommitBatch(finalCtx, batchTx, batchMsg, workerID)
				}
				return
			}

			batchMsg = append(batchMsg, task.Msg)
			batchTx = append(batchTx, task.Tx)

			if len(batchTx) >= c.cfg.BatchSize {
				if c.processAndCommitBatch(ctx, batchTx, batchMsg, workerID) {
					batchTx = batchTx[:0]
					batchMsg = batchMsg[:0]
				}
				ticker.Reset(c.cfg.BatchTimeout)
			}

		case <-ticker.C:
			if len(batchTx) > 0 {
				if c.processAndCommitBatch(ctx, batchTx, batchMsg, workerID) {
					batchTx = batchTx[:0]
					batchMsg = batchMsg[:0]
				}
			}
		}
	}
}

func (c *Consumer) processAndCommitBatch(ctx context.Context, batchTx []*domain.Transaction, batchMsg []kafka.Message, workerID int) bool {
	attempts := 0
	delay := 100 * time.Millisecond
	maxAttempts := c.cfg.MaxRetries
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	var lastErr error
	batchSaved := false

	for attempts < maxAttempts {
		if ctx.Err() != nil {
			slog.WarnContext(ctx, "context canceled during batch process, deferring to shutdown drain", "worker", workerID)
			return false
		}

		err := c.handler(ctx, batchTx)
		if err == nil {
			lastErr = nil
			batchSaved = true
			break
		}

		attempts++
		lastErr = err
		slog.ErrorContext(ctx, "batch save failed, retrying", "err", err, "attempt", attempts, "worker", workerID, "batch_size", len(batchMsg))

		time.Sleep(delay)
		delay *= 2
		if delay > c.cfg.RetryMaxDelay {
			delay = c.cfg.RetryMaxDelay
		}
	}

	if lastErr != nil && !batchSaved {
		slog.ErrorContext(ctx, "max retries reached, routing entire batch to DLQ", "worker", workerID, "batch_size", len(batchMsg))

		if ctx.Err() != nil {
			slog.WarnContext(ctx, "context canceled before routing to DLQ, deferring to shutdown drain", "worker", workerID)
			return false
		}

		dlqErr := c.sendToDlq(ctx, fmt.Errorf("db batch save error: %w", lastErr), batchMsg...)
		if dlqErr == nil {
			batchSaved = true
		}
	}

	if batchSaved {
		slog.InfoContext(ctx, "batch processed successfully", "worker", workerID, "batch_size", len(batchMsg))
		for _, msg := range batchMsg {
			trackerIface, _ := c.watermarks.Load(msg.Partition)
			tracker := trackerIface.(*PartitionWatermark)
			c.commitProgress(context.Background(), msg, tracker)
		}
		return true
	}

	slog.ErrorContext(ctx, "batch was neither saved to DB nor DLQ; offsets will not be committed",
		"worker", workerID,
		"batch_size", len(batchMsg),
	)
	return false
}

func (c *Consumer) sendToDlq(ctx context.Context, originalErr error, msgs ...kafka.Message) error {
	if c.errorHandler == nil {
		slog.WarnContext(ctx, "DLQ handler not configured, skipping messages", "count", len(msgs))
		return errors.New("DLQ handler not configured")
	}

	for i := 0; i < c.cfg.MaxDlqRetries; i++ {
		err := c.errorHandler.HandleError(ctx, originalErr, msgs...)
		if err == nil {
			return nil
		}

		slog.ErrorContext(ctx, "failed to write to DLQ, retrying",
			"err", err,
			"count", len(msgs),
			"attempt", i+1,
		)

		select {
		case <-ctx.Done():
			return fmt.Errorf("DLQ retry aborted due to context cancellation: %w", ctx.Err())
		case <-time.After(2 * time.Second):

		}
	}

	slog.ErrorContext(ctx, "failed to write to DB and DLQ", "count", len(msgs))
	return fmt.Errorf("exhausted all %d retries to send to DLQ", c.cfg.MaxDlqRetries)
}

func (c *Consumer) commitProgress(ctx context.Context, msg kafka.Message, tracker *PartitionWatermark) {
	_, shouldCommit := tracker.MarkDone(msg.Offset)
	if shouldCommit {
		err := c.reader.CommitMessages(ctx, msg)
		if err != nil {
			slog.ErrorContext(ctx, "failed to commit offset", "err", err, "partition", msg.Partition, "offset", msg.Offset)
		}
	}
}

func (c *Consumer) Stop(ctx context.Context) error {
	var shutdownErr error

	c.setShutdownCtx.Do(func() {
		c.shutdownCtx = ctx
	})

	slog.InfoContext(ctx, "waiting for workers to finish in-flight batches")

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.InfoContext(ctx, "all workers finished successfully")
	case <-ctx.Done():
		shutdownErr = fmt.Errorf("consumer shutdown timeout exceeded: %w", ctx.Err())
		slog.ErrorContext(ctx, "partial shutdown: some in-flight batches were dropped", "err", shutdownErr)
	}

	if err := c.reader.Close(); err != nil {
		slog.ErrorContext(ctx, "failed to close kafka reader", "err", err)
		if shutdownErr == nil {
			shutdownErr = err
		}
	}

	return shutdownErr
}

func hashUserID(userID string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(userID))
	return h.Sum32()
}

func (c *Consumer) getShutdownCtx() context.Context {
	if c.shutdownCtx != nil {
		return c.shutdownCtx
	}
	return context.Background()
}
