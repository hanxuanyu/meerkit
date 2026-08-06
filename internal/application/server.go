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
	"meerkit/internal/logging"
	"meerkit/internal/monitor"
	"meerkit/internal/notification"
	"meerkit/internal/notification/inapp"
	pluginruntime "meerkit/internal/plugin"
	runtimeapp "meerkit/internal/runtime"
	"meerkit/internal/store"
)

func RunServer(ctx context.Context, config app.Config, frontend fs.FS) error {
	logger, accessLogger, closeLogger, err := logging.New(config.Logging)
	if err != nil {
		return err
	}
	defer closeLogger()
	database, err := store.OpenStore(config.Storage.DataDir)
	if err != nil {
		return err
	}
	defer database.Close()
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
	pluginManager, err := pluginruntime.NewManager(database, modules, pluginruntime.ManagerOptions{DataDir: config.Storage.DataDir, TrustedKeys: trustedKeys, Logger: logger})
	if err != nil {
		return err
	}
	defer pluginManager.Close()
	if executable, executableErr := os.Executable(); executableErr == nil {
		if err := pluginManager.SeedOfficial(ctx, filepath.Join(filepath.Dir(executable), "plugins")); err != nil {
			return err
		}
	}
	if err := pluginManager.Start(ctx); err != nil {
		return err
	}
	inAppHub := inapp.NewHub()
	notifiers := notification.NewRegistry(database, inAppHub)
	runner := runtimeapp.NewRunner(database, modules, notifiers, logger)
	scheduler := runtimeapp.NewScheduler(runner, database, config, logger)
	cleaner := runtimeapp.NewCleanupWorker(database, config, logger, func(unreadCount int) { inAppHub.Publish(inapp.StreamEvent{Type: "pruned", UnreadCount: unreadCount}) })
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
	return nil
}
