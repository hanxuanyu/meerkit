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
	"meerkit/internal/browser"
	"meerkit/internal/core"
	"meerkit/internal/logging"
	"meerkit/internal/mcpserver"
	"meerkit/internal/monitor"
	"meerkit/internal/notification"
	"meerkit/internal/notification/inapp"
	pluginruntime "meerkit/internal/plugin"
	runtimeapp "meerkit/internal/runtime"
	"meerkit/internal/runtimeconfig"
	"meerkit/internal/statusboard"
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
	connectionLifetime, connectionIdleTime := config.Storage.Database.ConnectionDurations()
	database, err := store.Open(ctx, store.Options{
		Type:            store.DatabaseType(config.Storage.Database.Type),
		DSN:             config.Storage.Database.DSN,
		DataDir:         config.Storage.DataDir,
		AutoMigrate:     config.Storage.Database.AutoMigrate,
		MaxOpenConns:    config.Storage.Database.MaxOpenConns,
		MaxIdleConns:    config.Storage.Database.MaxIdleConns,
		ConnMaxLifetime: connectionLifetime,
		ConnMaxIdleTime: connectionIdleTime,
	})
	if err != nil {
		return err
	}
	defer database.Close()
	runtimeManager, err := runtimeconfig.New(ctx, database)
	if err != nil {
		return err
	}
	runtimeSnapshot := runtimeManager.Snapshot()
	logger, accessLogger, loggingController, err := logging.NewDynamic(config.Logging, runtimeSnapshot.Logging)
	if err != nil {
		return err
	}
	defer loggingController.Close()
	runtimeMode := "release"
	if options.Version == "dev" {
		runtimeMode = "development"
	}
	logger.Info("Meerkit runtime selected", "version", options.Version, "runtime_mode", runtimeMode, "address", config.ListenAddress(), "data_dir", config.Storage.DataDir, "config_file", config.Metadata.ConfigFile)
	logger.Info("Meerkit scheduler configuration", "timezone", runtimeSnapshot.Scheduler.Timezone, "max_concurrency", runtimeSnapshot.Scheduler.MaxConcurrency, "poll_ms", runtimeSnapshot.Scheduler.PollMilliseconds)
	logger.Info("Meerkit retention configuration", "records", runtimeSnapshot.Storage.Retention, "notifications", runtimeSnapshot.Storage.NotificationRetention, "cleanup_interval", runtimeSnapshot.Storage.CleanupInterval)
	logger.Info("Meerkit logging configuration", "host_level", runtimeSnapshot.Logging.Level, "host_format", runtimeSnapshot.Logging.Format, "plugin_level", runtimeSnapshot.Plugins.LogLevel, "plugin_format", runtimeSnapshot.Plugins.LogFormat)
	logger.Info("Meerkit storage initialized", "database_type", config.Storage.Database.Type, "data_dir", config.Storage.DataDir, "auto_migrate", config.Storage.Database.AutoMigrate)
	api.SetFrontendFS(frontend)
	modules := monitor.NewRegistry()
	browserManager, err := browser.OpenManager(config.Storage.DataDir)
	if err != nil {
		return err
	}
	trustedKeys := make(map[string]ed25519.PublicKey, len(config.Plugins.TrustedKeys))
	for id, encoded := range config.Plugins.TrustedKeys {
		value, decodeErr := base64.StdEncoding.DecodeString(encoded)
		if decodeErr != nil || len(value) != ed25519.PublicKeySize {
			return errors.New("plugins.trusted_keys contains an invalid base64 Ed25519 public key")
		}
		trustedKeys[id] = ed25519.PublicKey(value)
	}
	pluginManager, err := pluginruntime.NewManager(database, modules, pluginruntime.ManagerOptions{DataDir: config.Storage.DataDir, TrustedKeys: trustedKeys, Logger: logger, LogLevel: runtimeSnapshot.Plugins.LogLevel, LogFormat: runtimeSnapshot.Plugins.LogFormat, BrowserManager: browserManager})
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
	statusBoardHub := statusboard.NewHub()
	statusBoardService := statusboard.NewService(database, modules, statusBoardHub)
	runner := runtimeapp.NewRunner(database, modules, notifiers, logger, statusBoardService)
	scheduler := runtimeapp.NewScheduler(runner, database, runtimeManager.Snapshot, logger, runtimeManager.Subscribe())
	cleaner := runtimeapp.NewCleanupWorker(database, runtimeManager.Snapshot, logger, func(unreadCount int) { inAppHub.Publish(inapp.StreamEvent{Type: "pruned", UnreadCount: unreadCount}) }, runtimeManager.Subscribe())
	cleaner.SetRecordsPruned(func(deleted int) {
		if deleted > 0 {
			statusBoardService.Publish(statusboard.StreamEvent{Type: "records_pruned"})
		}
	})
	logger.Info("Meerkit background workers configured", "scheduler", true, "cleanup", true, "max_concurrency", runtimeSnapshot.Scheduler.MaxConcurrency, "cleanup_interval", runtimeSnapshot.Storage.CleanupInterval)
	authService, authErr := auth.NewServiceWithOptions(database, runtimeSnapshot.SessionTTLDuration(), auth.ServiceOptions{MasterKeyFile: config.Security.MasterKeyFile, AllowTokenCopy: config.Security.AllowTokenCopy})
	if authErr != nil {
		return authErr
	}
	apiServer := api.NewAPIServer(database, modules, notifiers, runner, inAppHub, pluginManager, authService, config, logger, accessLogger, runtimeManager)
	apiServer.SetStatusBoard(statusBoardService)
	apiServer.SetBrowser(browserManager)
	mcpHandler, mcpErr := mcpserver.New(browserManager, mcpserver.Options{ValidateToken: func(validateCtx context.Context, token string) error {
		principal, err := authService.AuthenticateToken(validateCtx, token)
		if err != nil || principal.Type != auth.TokenTypeMCP || !auth.HasScope(principal, auth.ScopeMCP) {
			return errors.New("invalid MCP token")
		}
		return nil
	}, Version: options.Version, Logger: logger})
	if mcpErr != nil {
		return mcpErr
	}
	apiServer.SetMCP(mcpHandler)
	runtimeManager.SetApply(func(applyCtx context.Context, oldConfig, newConfig app.RuntimeConfig) error {
		if oldConfig.Logging != newConfig.Logging {
			if err := loggingController.Apply(newConfig.Logging); err != nil {
				return err
			}
		}
		if oldConfig.Plugins != newConfig.Plugins {
			if err := pluginManager.UpdateLogConfig(applyCtx, newConfig.Plugins.LogLevel, newConfig.Plugins.LogFormat); err != nil {
				return err
			}
		}
		authService.SetSessionTTL(newConfig.SessionTTLDuration())
		return nil
	})
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
