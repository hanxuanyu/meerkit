package logging

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"

	sdklogging "github.com/hanxuanyu/meerkit/sdk/logging"
	"meerkit/internal/app"
)

type Controller struct {
	mu            sync.Mutex
	business      *dynamicHandler
	access        *dynamicHandler
	closeBusiness func() error
	closeAccess   func() error
	static        app.LoggingConfig
}

func NewDynamic(config app.LoggingConfig, runtime app.RuntimeLoggingConfig) (*slog.Logger, *slog.Logger, *Controller, error) {
	businessState, closeBusiness, err := buildHandler("logging.business", config, runtime, false)
	if err != nil {
		return nil, nil, nil, err
	}
	accessState, closeAccess, err := buildHandler("logging.access", config, runtime, true)
	if err != nil {
		_ = closeBusiness()
		return nil, nil, nil, err
	}
	controller := &Controller{
		business: dynamicHandlerFor(businessState), access: dynamicHandlerFor(accessState),
		closeBusiness: closeBusiness, closeAccess: closeAccess, static: config,
	}
	businessLogger := loggerFor(controller.business, runtime.Format, "business")
	accessLogger := loggerFor(controller.access, runtime.Format, "access")
	return businessLogger, accessLogger, controller, nil
}

func (c *Controller) Apply(runtime app.RuntimeLoggingConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	businessState, closeBusiness, err := buildHandler("logging.business", c.static, runtime, false)
	if err != nil {
		return err
	}
	accessState, closeAccess, err := buildHandler("logging.access", c.static, runtime, true)
	if err != nil {
		_ = closeBusiness()
		return err
	}
	c.business.set(businessState)
	c.access.set(accessState)
	oldBusiness, oldAccess := c.closeBusiness, c.closeAccess
	c.closeBusiness, c.closeAccess = closeBusiness, closeAccess
	_ = oldBusiness()
	_ = oldAccess()
	return nil
}

func (c *Controller) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return errors.Join(c.closeAccess(), c.closeBusiness())
}

func buildHandler(prefix string, config app.LoggingConfig, runtime app.RuntimeLoggingConfig, access bool) (slog.Handler, func() error, error) {
	file := config.File
	console := runtime.Console.Enabled
	fileEnabled := runtime.File.Enabled
	addSource := runtime.AddSource
	if access {
		console = runtime.Console.Access
		fileEnabled = runtime.File.Enabled && runtime.File.Access.Enabled
		file.Filename = config.File.Access.Filename
		addSource = false
	}
	level, err := ParseLevel(runtime.Level)
	if err != nil {
		return nil, nil, err
	}
	output, closeOutput, err := createOutput(prefix, console, fileEnabled, file)
	if access && errors.Is(err, errNoLogOutput) {
		output, closeOutput, err = io.Discard, func() error { return nil }, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return newHandler(output, runtime.Format, level, addSource), closeOutput, nil
}

var errNoLogOutput = errors.New("no log output")

func newHandler(output io.Writer, format string, level slog.Level, addSource bool) slog.Handler {
	options := &slog.HandlerOptions{Level: level, AddSource: addSource}
	switch format {
	case "json":
		return slog.NewJSONHandler(output, options)
	case "simple":
		return sdklogging.NewSimpleHandler(output, options)
	default:
		return slog.NewTextHandler(output, options)
	}
}

func loggerFor(handler *dynamicHandler, format, channel string) *slog.Logger {
	logger := slog.New(handler)
	if format == "simple" {
		return logger
	}
	return logger.With("service", "meerkit", "channel", channel)
}

type dynamicState struct {
	mu      sync.RWMutex
	handler slog.Handler
}

type dynamicHandler struct {
	state  *dynamicState
	attrs  []slog.Attr
	groups []string
}

func dynamicHandlerFor(handler slog.Handler) *dynamicHandler {
	return &dynamicHandler{state: &dynamicState{handler: handler}}
}

func (h *dynamicHandler) set(handler slog.Handler) {
	h.state.mu.Lock()
	h.state.handler = handler
	h.state.mu.Unlock()
}

func (h *dynamicHandler) current() slog.Handler {
	h.state.mu.RLock()
	defer h.state.mu.RUnlock()
	return h.state.handler
}

func (h *dynamicHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.current().Enabled(ctx, level)
}

func (h *dynamicHandler) Handle(ctx context.Context, record slog.Record) error {
	h.state.mu.RLock()
	base := h.state.handler
	for _, group := range h.groups {
		base = base.WithGroup(group)
	}
	if len(h.attrs) > 0 {
		base = base.WithAttrs(h.attrs)
	}
	err := base.Handle(ctx, record)
	h.state.mu.RUnlock()
	return err
}

func (h *dynamicHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &clone
}

func (h *dynamicHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.groups = append(append([]string(nil), h.groups...), name)
	return &clone
}
