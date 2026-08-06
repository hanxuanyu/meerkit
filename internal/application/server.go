package application

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"meerkit/internal/api"
	"meerkit/internal/app"
	"meerkit/internal/auth"
	"meerkit/internal/core"
	"meerkit/internal/logging"
	"meerkit/internal/monitor"
	"meerkit/internal/notification"
	"meerkit/internal/notification/inapp"
	pluginruntime "meerkit/internal/plugin"
	runtimeapp "meerkit/internal/runtime"
	"meerkit/internal/store"
)

type ServerOptions struct {
	Version string
}

func RunServer(ctx context.Context, config app.Config, frontend fs.FS, serverOptions ...ServerOptions) error {
	options := ServerOptions{}
	if len(serverOptions) > 0 {
		options = serverOptions[0]
	}
	logger, accessLogger, closeLogger, err := logging.New(config.Logging)
	if err != nil {
		return err
	}
	defer closeLogger()
	runtimeMode := "release"
	if options.Version == "dev" {
		runtimeMode = "development"
	}
	logger.Info("Meerkit runtime selected", "version", options.Version, "runtime_mode", runtimeMode, "address", config.ListenAddress(), "data_dir", config.Storage.DataDir, "config_file", config.Metadata.ConfigFile)
	logger.Info("Meerkit scheduler configuration", "timezone", config.Scheduler.Timezone, "max_concurrency", config.Scheduler.MaxConcurrency, "poll_ms", config.Scheduler.PollMilliseconds)
	logger.Info("Meerkit retention configuration", "records", config.Storage.Retention, "notifications", config.Storage.NotificationRetention, "cleanup_interval", config.Storage.CleanupInterval)
	logger.Info("Meerkit logging configuration", "host_level", config.Logging.Level, "host_format", config.Logging.Format, "plugin_level", config.Plugins.LogLevel, "plugin_format", config.Plugins.LogFormat)
	database, err := store.OpenStore(config.Storage.DataDir)
	if err != nil {
		return err
	}
	defer database.Close()
	logger.Info("Meerkit storage initialized", "data_dir", config.Storage.DataDir)
	api.SetFrontendFS(frontend)
	modules := monitor.NewRegistry()
	trustedKeys := make(map[string]ed25519.PublicKey, len(config.Plugins.TrustedKeys))
	for id, encoded := range config.Plugins.TrustedKeys {
		value, decodeErr := base64.StdEncoding.DecodeString(encoded)
		if decodeErr != nil || len(value) != ed25519.PublicKeySize {
			return errors.New("plugins.trusted_keys contains an invalid base64 Ed25519 public key")
		}
		trustedKeys[id] = ed25519.PublicKey(value)
	}
	pluginManager, err := pluginruntime.NewManager(database, modules, pluginruntime.ManagerOptions{DataDir: config.Storage.DataDir, TrustedKeys: trustedKeys, Logger: logger, LogLevel: config.Plugins.LogLevel, LogFormat: config.Plugins.LogFormat})
	if err != nil {
		return err
	}
	defer pluginManager.Close()
	useSource := options.Version == "dev"
	if useSource {
		var developmentPlugins []core.PluginInstallation
		developmentPlugins, err = pluginManager.SyncDevelopment(ctx, config.Plugins.SourceDir)
		if err != nil && !errors.Is(err, pluginruntime.ErrNoDevelopmentPlugins) {
			return err
		}
		useSource = err == nil
		if useSource {
			logger.Info("plugin runtime mode selected", "mode", "source", "reason", "development host with source plugins", "source_dir", config.Plugins.SourceDir, "plugins", len(developmentPlugins))
		} else {
			logger.Info("development plugin sources not found; falling back to packages", "source_dir", config.Plugins.SourceDir)
		}
	}
	if !useSource {
		reason := "release host"
		if options.Version == "dev" {
			reason = "development sources unavailable"
		}
		logger.Info("plugin runtime mode selected", "mode", "package", "reason", reason)
		if err := pluginManager.ClearDevelopment(ctx); err != nil {
			return err
		}
		if executable, executableErr := os.Executable(); executableErr == nil {
			if err := pluginManager.SeedOfficial(ctx, filepath.Join(filepath.Dir(executable), "plugins")); err != nil {
				return err
			}
		}
	}
	if err := pluginManager.Start(ctx); err != nil {
		return err
	}
	installations, err := pluginManager.List(ctx)
	if err != nil {
		return err
	}
	activePlugins := 0
	for _, installation := range installations {
		if !installation.Enabled {
			continue
		}
		activePlugins++
		logger.Info("active plugin", "plugin_id", installation.ID, "name", installation.Name, "version", installation.Version, "status", installation.Status, "modules", len(installation.Modules))
	}
	logger.Info("plugin activation summary", "installed", len(installations), "active", activePlugins)
	inAppHub := inapp.NewHub()
	notifiers := notification.NewRegistry(database, inAppHub)
	runner := runtimeapp.NewRunner(database, modules, notifiers, logger)
	scheduler := runtimeapp.NewScheduler(runner, database, config, logger)
	cleaner := runtimeapp.NewCleanupWorker(database, config, logger, func(unreadCount int) { inAppHub.Publish(inapp.StreamEvent{Type: "pruned", UnreadCount: unreadCount}) })
	logger.Info("Meerkit background workers configured", "scheduler", true, "cleanup", true, "max_concurrency", config.Scheduler.MaxConcurrency, "cleanup_interval", config.Storage.CleanupInterval)
	sessionTTL, _ := time.ParseDuration(config.Security.SessionTTL)
	authService := auth.NewService(database, sessionTTL)
	apiServer := api.NewAPIServer(database, modules, notifiers, runner, inAppHub, pluginManager, authService, config, logger, accessLogger)
	go scheduler.Start(ctx)
	go cleaner.Start(ctx)
	server := &http.Server{Addr: config.ListenAddress(), Handler: apiServer.Router(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		logger.Info("Meerkit shutdown requested", "reason", ctx.Err())
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()
	logger.Info("Meerkit started", "address", config.ListenAddress(), "data_dir", config.Storage.DataDir)
	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	if ctx.Err() != nil {
		<-shutdownDone
	}
	logger.Info("Meerkit stopped")
	return nil
}
