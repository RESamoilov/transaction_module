package consumer

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/meindokuse/transaction-module/internal/config"
	"github.com/meindokuse/transaction-module/internal/domain"
	"github.com/meindokuse/transaction-module/internal/dto"
	"github.com/meindokuse/transaction-module/pkg/validate"
	"github.com/segmentio/kafka-go"
)

type stubReader struct {
	mu      sync.Mutex
	msgs    []kafka.Message
	commits []kafka.Message
	closed  bool
}

func (s *stubReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.msgs) > 0 {
		msg := s.msgs[0]
		s.msgs = s.msgs[1:]
		return msg, nil
	}

	<-ctx.Done()
	return kafka.Message{}, ctx.Err()
}

func (s *stubReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commits = append(s.commits, msgs...)
	return nil
}

func (s *stubReader) Close() error {
	s.closed = true
	return nil
}

func TestStartProcessesValidMessage(t *testing.T) {
	t.Parallel()

	event := dto.TransactionEvent{
		ID:        "tx-1",
		UserID:    "user-1",
		Amount:    10,
		Type:      "bet",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	reader := &stubReader{
		msgs: []kafka.Message{{Partition: 0, Offset: 0, Value: payload}},
	}

	processed := make(chan []*domain.Transaction, 1)
	c := &Consumer{
		reader:       reader,
		handler:      func(ctx context.Context, txs []*domain.Transaction) error { processed <- txs; return nil },
		errorHandler: &stubErrorHandler{},
		cfg: config.ConsumerConfig{
			Topic:        "topic",
			WorkersCount: 1,
			BatchSize:    1,
			BatchTimeout: time.Millisecond,
			MaxRetries:   1,
		},
		validate: validate.Get(),
		workers:  []chan WorkerTask{make(chan WorkerTask, 2)},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.Start(ctx)
		close(done)
	}()

	select {
	case got := <-processed:
		if len(got) != 1 || got[0].ID != "tx-1" {
			t.Fatalf("processed batch = %+v, want tx-1", got)
		}
	case <-time.After(time.Second):
		t.Fatal("valid message was not processed")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start() did not stop after context cancellation")
	}
}

func TestStartRoutesInvalidMessageToDlq(t *testing.T) {
	t.Parallel()

	reader := &stubReader{
		msgs: []kafka.Message{{Topic: "main", Partition: 0, Offset: 5, Value: []byte(`{"id":"tx-1","user_id":"user-1","amount":10}`)}},
	}
	errHandler := &stubErrorHandler{}
	c := &Consumer{
		reader:       reader,
		errorHandler: errHandler,
		cfg: config.ConsumerConfig{
			Topic:         "topic",
			WorkersCount:  1,
			BatchSize:     1,
			BatchTimeout:  time.Millisecond,
			MaxRetries:    1,
			MaxDlqRetries: 1,
		},
		validate: validate.Get(),
		workers:  []chan WorkerTask{make(chan WorkerTask, 1)},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.Start(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start() did not stop after context cancellation")
	}

	if errHandler.calls == 0 {
		t.Fatal("expected invalid message to be routed to DLQ handler")
	}
}
