package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestPrettyHandlerHandleWritesFormattedLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	h := NewPrettyHandler(&buf, WithColors(false), WithMaxLength(8))

	rec := slog.NewRecord(time.Date(2026, 3, 15, 1, 2, 3, 0, time.UTC), slog.LevelInfo, "message", 0)
	rec.Add("key", "very-long-value")

	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "message") || !strings.Contains(out, "very-...") || !strings.Contains(out, "key") {
		t.Fatalf("formatted output = %q, missing expected fragments", out)
	}
}

func TestPrettyHandlerWithAttrsAndGroupsReturnsHandler(t *testing.T) {
	t.Parallel()

	h := NewPrettyHandler(&bytes.Buffer{})
	if h.WithAttrs(nil) == nil {
		t.Fatal("WithAttrs() returned nil")
	}
	if h.WithGroup("group") == nil {
		t.Fatal("WithGroup() returned nil")
	}
	if !h.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("Enabled() = false, want true for debug")
	}
}

func TestPrettyHandlerColorizeLevelAndFormatValue(t *testing.T) {
	t.Parallel()

	h := NewPrettyHandler(&bytes.Buffer{}, WithColors(true), WithMaxLength(5))

	levelCases := []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError, slog.Level(99)}
	for _, level := range levelCases {
		if got := h.colorizeLevel(level); got == "" {
			t.Fatalf("colorizeLevel(%v) returned empty string", level)
		}
	}

	valueCases := []slog.Value{
		slog.StringValue("long-string"),
		slog.Int64Value(42),
		slog.BoolValue(true),
		slog.Float64Value(12.5),
		slog.TimeValue(time.Date(2026, 3, 15, 1, 2, 3, 0, time.UTC)),
		slog.AnyValue(struct{ Name string }{Name: "x"}),
	}

	for _, value := range valueCases {
		if got := h.formatValue(value); got == "" {
			t.Fatalf("formatValue(%v) returned empty string", value)
		}
	}
}
