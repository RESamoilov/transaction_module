package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

type LogLevel string

const (
	DebugLevel LogLevel = "DEBUG"
	InfoLevel  LogLevel = "INFO"
	WarnLevel  LogLevel = "WARN"
	ErrorLevel LogLevel = "ERROR"
)

const (
	TraceIDKey string = "trace_id"
	UserIDKey  string = "user_id"
)

type LogEntry struct {
	UserID  string `json:"user_id"`
	TraceID string `json:"trace_id"`
	ReqPath string `json:"req_path"`

	Extra map[string]interface{} `json:"extra,omitempty"`
}

const (
	LogFieldsKey string = "log_fields"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func Init() {
	var baseHandler slog.Handler
	env := getEnv("LOGGING", "dev")

	if env == "dev" {
		baseHandler = NewPrettyHandler(os.Stdout, WithMaxLength(20))
	} else {
		baseHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	handler := &ContextMiddleware{next: baseHandler}
	slog.SetDefault(slog.New(handler))
}

type ContextMiddleware struct {
	next slog.Handler
}

func (m *ContextMiddleware) Enabled(ctx context.Context, level slog.Level) bool {
	return m.next.Enabled(ctx, level)
}

func (m *ContextMiddleware) Handle(ctx context.Context, rec slog.Record) error {
	if fields, ok := ctx.Value(LogFieldsKey).(*LogEntry); ok && fields != nil {
		if fields.TraceID != "" {
			rec.Add("trace_id", fields.TraceID)
		}
		if fields.UserID != "" {
			rec.Add("user_id", fields.UserID)
		}
		if fields.ReqPath != "" {
			rec.Add("path", fields.ReqPath)
		}

		for key, value := range fields.Extra {
			rec.Add(key, value)
		}
	}

	return m.next.Handle(ctx, rec)
}

func (m *ContextMiddleware) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextMiddleware{next: m.next.WithAttrs(attrs)}
}

func (m *ContextMiddleware) WithGroup(name string) slog.Handler {
	return &ContextMiddleware{next: m.next.WithGroup(name)}
}

func WithField(ctx context.Context, key string, value interface{}) context.Context {
	return WithFields(ctx, map[string]interface{}{key: value})
}

func WithFields(ctx context.Context, newFields map[string]interface{}) context.Context {
	if fields, ok := ctx.Value(LogFieldsKey).(*LogEntry); ok && fields != nil {
		newExtra := make(map[string]interface{})
		for k, v := range fields.Extra {
			newExtra[k] = v
		}
		for k, v := range newFields {
			newExtra[k] = v
		}

		newEntry := &LogEntry{
			UserID:  fields.UserID,
			TraceID: fields.TraceID,
			ReqPath: fields.ReqPath,
			Extra:   newExtra,
		}

		return context.WithValue(ctx, LogFieldsKey, newEntry)
	}

	return context.WithValue(ctx, LogFieldsKey, &LogEntry{
		Extra: newFields,
	})
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

func WithUserID(ctx context.Context, userID interface{}) context.Context {
	return context.WithValue(ctx, UserIDKey, fmt.Sprintf("%v", userID))
}

func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}
