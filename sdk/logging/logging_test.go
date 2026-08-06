package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestSimpleHandlerWritesCompactSingleLine(t *testing.T) {
	var output bytes.Buffer
	handler := NewSimpleHandler(&output, &slog.HandlerOptions{Level: slog.LevelInfo})
	record := slog.NewRecord(time.Date(2026, 8, 6, 9, 8, 7, 0, time.Local), slog.LevelInfo, "plugin activated", 0)
	record.Add("plugin_id", "meerkit.http", "modules", 1, "detail", "first\nsecond")
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	line := output.String()
	expected := "[09:08:07] [INFO] plugin activated plugin_id=meerkit.http modules=1 detail=first\\nsecond\n"
	if line != expected {
		t.Fatalf("simple log = %q", line)
	}
	if strings.Count(strings.TrimSpace(line), "\n") != 0 {
		t.Fatalf("simple log must remain on one line: %q", line)
	}
}

func TestSimpleHandlerFiltersByLevelAndKeepsContext(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(NewSimpleHandler(&output, &slog.HandlerOptions{Level: slog.LevelWarn})).With("plugin_id", "meerkit.tcp")
	logger.Info("hidden")
	logger.Error("plugin failed", "error", "connection closed")
	line := output.String()
	if strings.Contains(line, "hidden") || !strings.Contains(line, "[ERROR] plugin failed plugin_id=meerkit.tcp error=\"connection closed\"") {
		t.Fatalf("unexpected filtered log: %q", line)
	}
}
