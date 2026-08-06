package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

// NewLogger creates a logger using the formats supported by both the Meerkit
// host and plugin SDK: text, json, and simple.
func NewLogger(output io.Writer, format string, level slog.Leveler, addSource bool) *slog.Logger {
	options := &slog.HandlerOptions{Level: level, AddSource: addSource}
	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		handler = slog.NewJSONHandler(output, options)
	case "simple":
		handler = NewSimpleHandler(output, options)
	default:
		handler = slog.NewTextHandler(output, options)
	}
	return slog.New(handler)
}

// ParseLevel converts the external log level names accepted by Meerkit.
func ParseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported log level %q", value)
	}
}

type simpleHandler struct {
	output io.Writer
	level  slog.Leveler
	attrs  []slog.Attr
	groups []string
	mutex  *sync.Mutex
}

// NewSimpleHandler writes one compact line per record:
// [15:04:05] [INFO] message key=value
func NewSimpleHandler(output io.Writer, options *slog.HandlerOptions) slog.Handler {
	if options == nil {
		options = &slog.HandlerOptions{}
	}
	level := options.Level
	if level == nil {
		level = slog.LevelInfo
	}
	return &simpleHandler{output: output, level: level, mutex: &sync.Mutex{}}
}

func (h *simpleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *simpleHandler) Handle(_ context.Context, record slog.Record) error {
	timestamp := record.Time
	var line strings.Builder
	line.WriteByte('[')
	line.WriteString(timestamp.Format("15:04:05"))
	line.WriteString("] [")
	line.WriteString(record.Level.String())
	line.WriteString("] ")
	line.WriteString(singleLine(record.Message))
	for _, attr := range h.attrs {
		appendAttr(&line, h.groups, attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		appendAttr(&line, h.groups, attr)
		return true
	})
	line.WriteByte('\n')
	h.mutex.Lock()
	_, err := io.WriteString(h.output, line.String())
	h.mutex.Unlock()
	return err
}

func (h *simpleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &clone
}

func (h *simpleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.groups = append(append([]string(nil), h.groups...), name)
	return &clone
}

func appendAttr(line *strings.Builder, groups []string, attr slog.Attr) {
	value := attr.Value.Resolve()
	if value.Kind() == slog.KindGroup {
		nextGroups := groups
		if attr.Key != "" {
			nextGroups = append(append([]string(nil), groups...), attr.Key)
		}
		for _, child := range value.Group() {
			appendAttr(line, nextGroups, child)
		}
		return
	}
	if attr.Key == "" {
		return
	}
	key := attr.Key
	if len(groups) > 0 {
		key = strings.Join(groups, ".") + "." + key
	}
	line.WriteByte(' ')
	line.WriteString(key)
	line.WriteByte('=')
	line.WriteString(formatValue(value))
}

func formatValue(value slog.Value) string {
	switch value.Kind() {
	case slog.KindString:
		return quoteWhenNeeded(singleLine(value.String()))
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindTime:
		return value.Time().Format("2006-01-02T15:04:05.000Z07:00")
	case slog.KindAny:
		if err, ok := value.Any().(error); ok {
			return quoteWhenNeeded(singleLine(err.Error()))
		}
		return quoteWhenNeeded(singleLine(fmt.Sprint(value.Any())))
	default:
		return value.String()
	}
}

func quoteWhenNeeded(value string) string {
	if value == "" {
		return `""`
	}
	if strings.IndexFunc(value, func(character rune) bool {
		return unicode.IsSpace(character) || character == '"' || character == '='
	}) >= 0 {
		return strconv.Quote(value)
	}
	return value
}

func singleLine(value string) string {
	value = strings.ReplaceAll(value, "\r\n", `\n`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return strings.ReplaceAll(value, "\r", `\n`)
}
