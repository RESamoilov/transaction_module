package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/meindokuse/transaction-module/internal/config"
	"github.com/meindokuse/transaction-module/internal/domain"
	"github.com/segmentio/kafka-go"
)

type stubErrorHandler struct {
	err   error
	calls int
}

func (s *stubErrorHandler) HandleError(ctx context.Context, originalErr error, msgs ...kafka.Message) error {
	s.calls++
	return s.err
}

func newClosedReader(t *testing.T) *kafka.Reader {
	t.Helper()

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Topic:   "test-topic",
		GroupID: "test-group",
	})
	if err := reader.Close(); err != nil {
		t.Fatalf("reader.Close() error = %v", err)
	}
	return reader
}

func TestHandlePoisonPillCommitsOnSuccessfulDLQ(t *testing.T) {
	t.Parallel()

	c := &Consumer{
		reader:       newClosedReader(t),
		errorHandler: &stubErrorHandler{},
		cfg:          config.ConsumerConfig{MaxDlqRetries: 1},
	}
	tracker := NewPartitionWatermark(5)

	c.handlePoisonPill(kafka.Message{Partition: 0, Offset: 5}, tracker, errors.New("bad message"))

	if tracker.watermark != 6 {
		t.Fatalf("watermark = %d, want 6", tracker.watermark)
	}
}

func TestHandlePoisonPillDoesNotCommitOnFailedDLQ(t *testing.T) {
	t.Parallel()

	c := &Consumer{
		reader:       newClosedReader(t),
		errorHandler: &stubErrorHandler{err: errors.New("dlq unavailable")},
		cfg:          config.ConsumerConfig{MaxDlqRetries: 1},
	}
	tracker := NewPartitionWatermark(5)

	c.handlePoisonPill(kafka.Message{Partition: 0, Offset: 5}, tracker, errors.New("bad message"))

	if tracker.watermark != 5 {
		t.Fatalf("watermark = %d, want 5", tracker.watermark)
	}
}

func TestProcessAndCommitBatchReturnsFalseWhenContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &Consumer{
		reader: newClosedReader(t),
		handler: func(ctx context.Context, txs []*domain.Transaction) error {
			t.Fatal("handler should not be called when context is canceled")
			return nil
		},
		cfg: config.ConsumerConfig{MaxRetries: 1},
	}
	c.watermarks.Store(0, NewPartitionWatermark(0))

	ok := c.processAndCommitBatch(ctx, []*domain.Transaction{{ID: "tx-1"}}, []kafka.Message{{Partition: 0, Offset: 0}}, 0)
	if ok {
		t.Fatal("processAndCommitBatch() = true, want false")
	}
}

func TestProcessAndCommitBatchRoutesToDLQAfterRetries(t *testing.T) {
	t.Parallel()

	errHandler := &stubErrorHandler{}
	c := &Consumer{
		reader:       newClosedReader(t),
		errorHandler: errHandler,
		handler: func(ctx context.Context, txs []*domain.Transaction) error {
			return errors.New("db down")
		},
		cfg: config.ConsumerConfig{
			MaxRetries:    1,
			MaxDlqRetries: 1,
		},
	}
	tracker := NewPartitionWatermark(0)
	c.watermarks.Store(0, tracker)

	ok := c.processAndCommitBatch(
		context.Background(),
		[]*domain.Transaction{{ID: "tx-1"}},
		[]kafka.Message{{Partition: 0, Offset: 1}},
		0,
	)

	if !ok {
		t.Fatal("processAndCommitBatch() = false, want true after DLQ success")
	}
	if errHandler.calls != 1 {
		t.Fatalf("DLQ calls = %d, want 1", errHandler.calls)
	}
	if tracker.watermark != 0 || !tracker.completed[1] {
		t.Fatalf("tracker state = watermark:%d completed:%v, want watermark 0 with completed[1]=true", tracker.watermark, tracker.completed)
	}
}

func TestBatchWorkerLoopRetainsBatchForShutdownDrain(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	saved := make(chan int, 1)
	c := &Consumer{
		reader: newClosedReader(t),
		handler: func(ctx context.Context, txs []*domain.Transaction) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			saved <- len(txs)
			return nil
		},
		cfg: config.ConsumerConfig{
			BatchSize:      10,
			BatchTimeout:   10 * time.Millisecond,
			MaxRetries:     1,
			RetryMaxDelay:  time.Millisecond,
			MaxDlqRetries:  1,
			WorkersCount:   1,
		},
		shutdownCtx: context.Background(),
	}
	tracker := NewPartitionWatermark(0)
	c.watermarks.Store(0, tracker)

	tasks := make(chan WorkerTask, 1)
	c.wg.Add(1)
	go c.batchWorkerLoop(ctx, 0, tasks)

	tasks <- WorkerTask{
		Msg: kafka.Message{Partition: 0, Offset: 1},
		Tx:  &domain.Transaction{ID: "tx-1", UserID: "user-1", Amount: 10, Type: domain.TxTypeBet, Timestamp: time.Now().UTC()},
	}

	cancel()
	time.Sleep(25 * time.Millisecond)
	close(tasks)
	c.wg.Wait()

	select {
	case count := <-saved:
		if count != 1 {
			t.Fatalf("saved batch size = %d, want 1", count)
		}
	default:
		t.Fatal("shutdown drain did not persist retained batch")
	}
}
