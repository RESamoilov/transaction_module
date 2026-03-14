package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
)

type PrettyHandler struct {
	output        io.Writer
	maxLength     int
	colorsEnabled bool
}

func NewPrettyHandler(w io.Writer, opts ...PrettyHandlerOption) *PrettyHandler {
	h := &PrettyHandler{
		output:        w,
		maxLength:     -1,
		colorsEnabled: true,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

func WithMaxLength(max int) PrettyHandlerOption {
	return func(h *PrettyHandler) {
		h.maxLength = max
	}
}

func WithColors(enabled bool) PrettyHandlerOption {
	return func(h *PrettyHandler) {
		h.colorsEnabled = enabled
	}
}

type PrettyHandlerOption func(*PrettyHandler)

func (h *PrettyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= slog.LevelDebug
}

func (h *PrettyHandler) Handle(ctx context.Context, rec slog.Record) error {
	var builder strings.Builder

	builder.WriteString(h.gray(rec.Time.Format("15:04:05.000")))
	builder.WriteString(" ")

	if h.colorsEnabled {
		builder.WriteString(h.colorizeLevel(rec.Level))
	}

	builder.WriteString(" ")

	builder.WriteString(h.cyan(rec.Message))

	rec.Attrs(func(attr slog.Attr) bool {
		builder.WriteString(" ")
		builder.WriteString(h.magenta(attr.Key))
		builder.WriteString("=")
		builder.WriteString(h.formatValue(attr.Value))
		return true
	})

	builder.WriteString("\n")

	_, err := h.output.Write([]byte(builder.String()))
	return err
}

func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	return h
}

func (h *PrettyHandler) colorizeLevel(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return h.gray("DEBUG")
	case slog.LevelInfo:
		return h.cyan("INFO ")
	case slog.LevelWarn:
		return h.yellow("WARN ")
	case slog.LevelError:
		return h.red("ERROR")
	default:
		return h.gray("?????")
	}
}

func (h *PrettyHandler) formatValue(value slog.Value) string {
	var str string
	switch value.Kind() {
	case slog.KindString:
		str = value.String()
	case slog.KindInt64:
		str = strconv.FormatInt(value.Int64(), 10)
	case slog.KindBool:
		str = strconv.FormatBool(value.Bool())
	case slog.KindFloat64:
		str = strconv.FormatFloat(value.Float64(), 'f', -1, 64)
	case slog.KindTime:
		str = value.Time().Format("15:04:05")
	default:
		str = fmt.Sprintf("%v", value.Any())
	}

	if h.maxLength > 0 && len(str) > h.maxLength {
		str = str[:h.maxLength-3] + "..."
	}

	switch value.Kind() {
	case slog.KindString:
		return h.green(str)
	case slog.KindInt64, slog.KindFloat64:
		return h.yellow(str)
	case slog.KindBool:
		return h.magenta(str)
	case slog.KindTime:
		return h.blue(str)
	default:
		return h.gray(str)
	}
}

const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorGray    = "\033[90m"
)

func (h *PrettyHandler) red(s string) string     { return colorRed + s + colorReset }
func (h *PrettyHandler) green(s string) string   { return colorGreen + s + colorReset }
func (h *PrettyHandler) yellow(s string) string  { return colorYellow + s + colorReset }
func (h *PrettyHandler) blue(s string) string    { return colorBlue + s + colorReset }
func (h *PrettyHandler) magenta(s string) string { return colorMagenta + s + colorReset }
func (h *PrettyHandler) cyan(s string) string    { return colorCyan + s + colorReset }
func (h *PrettyHandler) gray(s string) string    { return colorGray + s + colorReset }
