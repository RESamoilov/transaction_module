package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

type stubWriter struct {
	mu   sync.Mutex
	msgs []kafka.Message
	err  error
}

func (s *stubWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	if s.err != nil {
		return s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msgs = append(s.msgs, msgs...)
	return nil
}

func (s *stubWriter) Close() error { return nil }

func TestLoadHandlerRejectsNonPost(t *testing.T) {
	t.Parallel()

	handler := newLoadHandler(&stubWriter{}, 5*time.Millisecond, 1)
	req := httptest.NewRequest(http.MethodGet, "/load", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestLoadHandlerSendsValidMessages(t *testing.T) {
	t.Parallel()

	writer := &stubWriter{}
	handler := newLoadHandler(writer, 15*time.Millisecond, 1)
	req := httptest.NewRequest(http.MethodPost, "/load", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	writer.mu.Lock()
	defer writer.mu.Unlock()

	if len(writer.msgs) == 0 {
		t.Fatal("expected at least one Kafka message to be produced")
	}

	var event TransactionEvent
	if err := json.Unmarshal(writer.msgs[0].Value, &event); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if event.ID == "" || event.UserID == "" || event.Timestamp == "" {
		t.Fatalf("event = %+v, expected required fields", event)
	}
	if event.Type != "bet" && event.Type != "win" {
		t.Fatalf("transaction type = %q, want bet or win", event.Type)
	}
	if event.Amount <= 0 {
		t.Fatalf("amount = %v, want > 0", event.Amount)
	}
}

func TestLoadHandlerReturnsOKWhenWriterFails(t *testing.T) {
	t.Parallel()

	writer := &stubWriter{err: context.DeadlineExceeded}
	handler := newLoadHandler(writer, 10*time.Millisecond, 1)
	req := httptest.NewRequest(http.MethodPost, "/load", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
