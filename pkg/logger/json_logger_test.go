package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestGetEnvReturnsDefaultAndOverride(t *testing.T) {
	t.Setenv("LOGGER_TEST_ENV", "")
	if got := getEnv("LOGGER_TEST_ENV", "default"); got != "default" {
		t.Fatalf("getEnv() = %q, want default", got)
	}

	t.Setenv("LOGGER_TEST_ENV", "custom")
	if got := getEnv("LOGGER_TEST_ENV", "default"); got != "custom" {
		t.Fatalf("getEnv() = %q, want custom", got)
	}
}

func TestWithFieldsMergesExtras(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), LogFieldsKey, &LogEntry{
		UserID:  "u1",
		TraceID: "t1",
		ReqPath: "/x",
		Extra: map[string]interface{}{
			"a": 1,
		},
	})

	ctx = WithFields(ctx, map[string]interface{}{"b": 2})
	entry := ctx.Value(LogFieldsKey).(*LogEntry)

	if entry.Extra["a"] != 1 || entry.Extra["b"] != 2 {
		t.Fatalf("merged extra fields = %#v", entry.Extra)
	}
}

func TestContextMiddlewareHandleWritesContextFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	handler := &ContextMiddleware{
		next: slog.NewJSONHandler(&buf, nil),
	}

	ctx := context.WithValue(context.Background(), LogFieldsKey, &LogEntry{
		UserID:  "user-1",
		TraceID: "trace-1",
		ReqPath: "/api/v1/transactions",
		Extra: map[string]interface{}{
			"foo": "bar",
		},
	})

	rec := slog.NewRecord(testTime(), slog.LevelInfo, "hello", 0)
	if err := handler.Handle(ctx, rec); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	out := buf.String()
	for _, want := range []string{"trace-1", "user-1", "/api/v1/transactions", "foo", "bar"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output %q does not contain %q", out, want)
		}
	}
}

func TestWithFieldAndTraceHelpers(t *testing.T) {
	t.Parallel()

	ctx := WithField(context.Background(), "foo", "bar")
	ctx = WithTraceID(ctx, "trace-42")
	ctx = WithUserID(ctx, 7)

	if GetTraceID(ctx) != "trace-42" {
		t.Fatalf("GetTraceID() = %q, want trace-42", GetTraceID(ctx))
	}
	if ctx.Value(UserIDKey).(string) != "7" {
		t.Fatalf("user id = %v, want 7", ctx.Value(UserIDKey))
	}
}

func TestInitDoesNotPanic(t *testing.T) {
	t.Setenv("LOGGING", "dev")
	Init()
	t.Setenv("LOGGING", "prod")
	Init()
}

func TestContextMiddlewareHelpers(t *testing.T) {
	t.Parallel()

	base := slog.NewJSONHandler(&bytes.Buffer{}, nil)
	mw := &ContextMiddleware{next: base}

	if !mw.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("Enabled() = false, want true")
	}
	if mw.WithAttrs(nil) == nil {
		t.Fatal("WithAttrs() returned nil")
	}
	if mw.WithGroup("test") == nil {
		t.Fatal("WithGroup() returned nil")
	}
}

func TestWithFieldsCreatesEntryWhenMissing(t *testing.T) {
	t.Parallel()

	ctx := WithFields(context.Background(), map[string]interface{}{"foo": "bar"})
	entry, ok := ctx.Value(LogFieldsKey).(*LogEntry)
	if !ok || entry == nil {
		t.Fatal("log entry was not stored in context")
	}
	if entry.Extra["foo"] != "bar" {
		t.Fatalf("entry.Extra = %#v, want foo=bar", entry.Extra)
	}
}

func TestGetTraceIDReturnsEmptyStringWhenMissing(t *testing.T) {
	t.Parallel()

	if got := GetTraceID(context.Background()); got != "" {
		t.Fatalf("GetTraceID() = %q, want empty string", got)
	}
}

func testTime() time.Time {
	return time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
}
