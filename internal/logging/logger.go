package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	sdklogging "github.com/hanxuanyu/meerkit/sdk/logging"
	"gopkg.in/natefinch/lumberjack.v2"

	"meerkit/internal/app"
)

// New creates separate business and HTTP access loggers. The access logger is
// nil when access logging is disabled.
func New(config app.LoggingConfig) (*slog.Logger, *slog.Logger, func() error, error) {
	businessLogger, accessLogger, controller, err := NewDynamic(config, app.DefaultRuntimeConfig().Logging)
	if err != nil {
		return nil, nil, nil, err
	}
	return businessLogger, accessLogger, controller.Close, nil
}

func newLogger(output io.Writer, format string, level slog.Level, addSource bool, channel string) *slog.Logger {
	logger := sdklogging.NewLogger(output, format, level, addSource)
	if format == "simple" {
		return logger
	}
	return logger.With("service", "meerkit", "channel", channel)
}

func createOutput(prefix string, console, fileEnabled bool, file app.LogFileConfig) (io.Writer, func() error, error) {
	writers := make([]io.Writer, 0, 2)
	if console {
		writers = append(writers, os.Stdout)
	}
	var rotatingFile *lumberjack.Logger
	if fileEnabled {
		if filepath.Base(file.Filename) != file.Filename || file.Filename == "." || file.Filename == ".." {
			return nil, nil, fmt.Errorf("%s.filename must be a file name, got %q", prefix, file.Filename)
		}
		if err := os.MkdirAll(file.Directory, 0o755); err != nil {
			return nil, nil, fmt.Errorf("create %s directory: %w", prefix, err)
		}
		rotatingFile = &lumberjack.Logger{
			Filename:   filepath.Join(file.Directory, file.Filename),
			MaxSize:    file.MaxSizeMB,
			MaxBackups: file.MaxBackups,
			MaxAge:     file.MaxAgeDays,
			Compress:   file.Compress,
		}
		writers = append(writers, rotatingFile)
	}
	if len(writers) == 0 {
		return nil, nil, errNoLogOutput
	}

	var output io.Writer = writers[0]
	if len(writers) > 1 {
		output = io.MultiWriter(writers...)
	}
	closeOutput := func() error {
		if rotatingFile != nil {
			return rotatingFile.Close()
		}
		return nil
	}
	return output, closeOutput, nil
}

func ParseLevel(value string) (slog.Level, error) {
	return sdklogging.ParseLevel(value)
}
