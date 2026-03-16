package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/meindokuse/transaction-module/internal/config"
	"github.com/segmentio/kafka-go"
)

func TestSendToDlqWithoutHandlerReturnsError(t *testing.T) {
	t.Parallel()

	c := &Consumer{cfg: config.ConsumerConfig{MaxDlqRetries: 1}}
	err := c.sendToDlq(context.Background(), errors.New("boom"), kafka.Message{})
	if err == nil {
		t.Fatal("sendToDlq() error = nil, want error")
	}
}

func TestSendToDlqReturnsContextErrorWhenCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &Consumer{
		errorHandler: &stubErrorHandler{err: errors.New("dlq down")},
		cfg:          config.ConsumerConfig{MaxDlqRetries: 1},
	}

	err := c.sendToDlq(ctx, errors.New("boom"), kafka.Message{})
	if err == nil {
		t.Fatal("sendToDlq() error = nil, want error")
	}
}

func TestGetShutdownCtxFallsBackToBackground(t *testing.T) {
	t.Parallel()

	c := &Consumer{}
	if got := c.getShutdownCtx(); got == nil {
		t.Fatal("getShutdownCtx() = nil, want background context")
	}
}

func TestStopReturnsTimeoutWhenWorkersDoNotFinish(t *testing.T) {
	t.Parallel()

	c := &Consumer{reader: newClosedReader(t)}
	c.wg.Add(1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := c.Stop(ctx)
	c.wg.Done()

	if err == nil {
		t.Fatal("Stop() error = nil, want timeout error")
	}
}

func TestHashUserIDIsDeterministic(t *testing.T) {
	t.Parallel()

	a := hashUserID("user-1")
	b := hashUserID("user-1")
	c := hashUserID("user-2")

	if a != b {
		t.Fatalf("hashUserID() mismatch for same input: %d != %d", a, b)
	}
	if a == c {
		t.Fatalf("hashUserID() collision in test inputs: %d == %d", a, c)
	}
}
