package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meerkit/internal/app"
)

func TestParseLevel(t *testing.T) {
	tests := map[string]string{"debug": "DEBUG", "info": "INFO", "warning": "WARN", "error": "ERROR"}
	for input, expected := range tests {
		level, err := ParseLevel(input)
		if err != nil {
			t.Fatalf("ParseLevel(%q): %v", input, err)
		}
		if level.String() != expected {
			t.Fatalf("ParseLevel(%q) = %s, want %s", input, level, expected)
		}
	}
	if _, err := ParseLevel("verbose"); err == nil {
		t.Fatal("expected invalid level error")
	}
}

func TestNewWritesSingleLineTextLogAndRotatesFile(t *testing.T) {
	directory := t.TempDir()
	config := app.LoggingConfig{File: app.LogFileConfig{Directory: directory, Filename: "meerkit.log", MaxSizeMB: 1, MaxBackups: 2, MaxAgeDays: 1, Compress: true}}
	runtime := app.DefaultRuntimeConfig().Logging
	runtime.Format, runtime.Console.Enabled, runtime.File.Enabled, runtime.File.Access.Enabled, runtime.AddSource = "text", false, true, false, true
	logger, _, controller, err := NewDynamic(config, runtime)
	if err != nil {
		t.Fatalf("NewDynamic: %v", err)
	}
	closeLogger := controller.Close
	logger.Debug("hidden detail", "step", "debug")
	logger.Info("monitor execution completed", "monitor_id", "monitor-1", "success", true)
	if err := closeLogger(); err != nil {
		t.Fatalf("close logger: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(directory, "meerkit.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	output := string(data)
	if strings.Contains(output, "hidden detail") {
		t.Fatal("debug record should be filtered at info level")
	}
	if !strings.Contains(output, "level=INFO") || !strings.Contains(output, `msg="monitor execution completed"`) {
		t.Fatalf("unexpected text log: %s", output)
	}
	if strings.Count(strings.TrimSpace(output), "\n") != 0 {
		t.Fatalf("expected one log line: %q", output)
	}
}

func TestNewWritesJSONLog(t *testing.T) {
	directory := t.TempDir()
	config := app.LoggingConfig{File: app.LogFileConfig{Directory: directory, Filename: "meerkit.json.log", MaxSizeMB: 1}}
	runtime := app.DefaultRuntimeConfig().Logging
	runtime.Format, runtime.Console.Enabled, runtime.File.Enabled, runtime.File.Access.Enabled = "json", false, true, false
	logger, _, controller, err := NewDynamic(config, runtime)
	if err != nil {
		t.Fatalf("NewDynamic: %v", err)
	}
	closeLogger := controller.Close
	logger.Warn("scheduler concurrency limit reached", "limit", 4)
	if err := closeLogger(); err != nil {
		t.Fatalf("close logger: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "meerkit.json.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, `"level":"WARN"`) || !strings.Contains(output, `"limit":4`) {
		t.Fatalf("unexpected JSON log: %s", output)
	}
}

func TestNewWritesSimpleLogWithoutFixedContext(t *testing.T) {
	directory := t.TempDir()
	config := app.LoggingConfig{File: app.LogFileConfig{Directory: directory, Filename: "meerkit.simple.log", MaxSizeMB: 1}}
	runtime := app.DefaultRuntimeConfig().Logging
	runtime.Console.Enabled, runtime.File.Enabled, runtime.File.Access.Enabled = false, true, false
	logger, _, controller, err := NewDynamic(config, runtime)
	if err != nil {
		t.Fatalf("NewDynamic: %v", err)
	}
	closeLogger := controller.Close
	logger.Info("plugin activated", "plugin_id", "meerkit.http")
	if err := closeLogger(); err != nil {
		t.Fatalf("close logger: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "meerkit.simple.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, "[INFO] plugin activated plugin_id=meerkit.http") {
		t.Fatalf("unexpected simple log: %s", output)
	}
	if strings.Contains(output, "service=") || strings.Contains(output, "channel=") {
		t.Fatalf("simple log contains fixed context: %s", output)
	}
}

func TestNewSeparatesBusinessAndAccessLogs(t *testing.T) {
	directory := t.TempDir()
	config := app.LoggingConfig{File: app.LogFileConfig{
		Directory: directory, Filename: "meerkit.log", MaxSizeMB: 1,
		Access: app.AccessFileConfig{Filename: "meerkit-access.log"},
	}}
	runtime := app.DefaultRuntimeConfig().Logging
	runtime.Format, runtime.Console.Enabled, runtime.File.Enabled, runtime.File.Access.Enabled = "text", false, true, true
	businessLogger, accessLogger, controller, err := NewDynamic(config, runtime)
	if err != nil {
		t.Fatalf("NewDynamic: %v", err)
	}
	closeLogger := controller.Close
	businessLogger.Info("monitor execution completed")
	accessLogger.Info("http request", "method", "GET", "path", "/healthz")
	if err := closeLogger(); err != nil {
		t.Fatalf("close logger: %v", err)
	}

	businessData, err := os.ReadFile(filepath.Join(directory, "meerkit.log"))
	if err != nil {
		t.Fatalf("read business log: %v", err)
	}
	accessData, err := os.ReadFile(filepath.Join(directory, "meerkit-access.log"))
	if err != nil {
		t.Fatalf("read access log: %v", err)
	}
	if strings.Contains(string(businessData), "http request") || !strings.Contains(string(businessData), "monitor execution completed") {
		t.Fatalf("business log was not isolated: %s", businessData)
	}
	if strings.Contains(string(accessData), "monitor execution completed") || !strings.Contains(string(accessData), "http request") {
		t.Fatalf("access log was not isolated: %s", accessData)
	}
}
