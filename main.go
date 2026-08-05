package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"meerkit/internal/api"
	"meerkit/internal/app"
	"meerkit/internal/logging"
	"meerkit/internal/monitor"
	"meerkit/internal/notification"
	"meerkit/internal/notification/inapp"
	runtimeapp "meerkit/internal/runtime"
	"meerkit/internal/store"
)

//go:embed web/dist/*
var embeddedFrontend embed.FS

func main() {
	config, err := app.LoadConfig(os.Args[1:])
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}
	logger, accessLogger, closeLogger, err := logging.New(config.Logging)
	if err != nil {
		slog.Error("initialize logger failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := closeLogger(); err != nil {
			logger.Error("close logger failed", "error", err)
		}
	}()
	store, err := store.OpenStore(config.Storage.DataDir)
	if err != nil {
		logger.Error("open store failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	static, err := fsSub(embeddedFrontend, "web/dist")
	if err != nil {
		logger.Error("load embedded frontend failed", "error", err)
		os.Exit(1)
	}
	api.SetFrontendFS(static)
	modules := monitor.NewRegistry()
	inAppHub := inapp.NewHub()
	notifiers := notification.NewRegistry(store, inAppHub)
	runner := runtimeapp.NewRunner(store, modules, notifiers, logger)
	scheduler := runtimeapp.NewScheduler(runner, store, config, logger)
	cleaner := runtimeapp.NewCleanupWorker(store, config, logger, func(unreadCount int) {
		inAppHub.Publish(inapp.StreamEvent{Type: "pruned", UnreadCount: unreadCount})
	})
	apiServer := api.NewAPIServer(store, modules, notifiers, runner, inAppHub, config, logger, accessLogger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go scheduler.Start(ctx)
	go cleaner.Start(ctx)
	server := &http.Server{Addr: config.ListenAddress(), Handler: apiServer.Router(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownContext)
	}()
	logger.Info("Meerkit started", "address", config.ListenAddress(), "data_dir", config.Storage.DataDir)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func fsSub(files embed.FS, path string) (fs.FS, error) {
	return fs.Sub(files, path)
}
