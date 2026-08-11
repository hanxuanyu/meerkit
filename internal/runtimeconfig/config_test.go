package runtimeconfig

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"meerkit/internal/app"
	"meerkit/internal/auth"
	"meerkit/internal/store"
)

func TestManagerInitializesDefaultsAndPersistsRuntimeValues(t *testing.T) {
	ctx := context.Background()
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	manager, err := New(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := database.ListSystemConfigs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("system config rows = %d, want 5", len(rows))
	}
	for _, configType := range []string{app.SystemConfigStorage, app.SystemConfigScheduler, app.SystemConfigLogging, app.SystemConfigPlugins, app.SystemConfigAuth} {
		if manager.Version(configType) != 1 {
			t.Fatalf("initial %s version = %d, want 1", configType, manager.Version(configType))
		}
	}

	t.Setenv("MEERKIT_STORAGE__RETENTION", "1h")
	changes := manager.Subscribe()
	if _, err := manager.UpdatePath(ctx, app.SystemConfigStorage, "storage.retention", json.RawMessage(`"7d"`), manager.Version(app.SystemConfigStorage)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changes:
	default:
		t.Fatal("runtime update did not notify subscribers")
	}
	if got := manager.Snapshot().Storage.Retention; got != "7d" {
		t.Fatalf("updated retention = %q, want 7d", got)
	}
	row, err := database.GetSystemConfig(ctx, app.SystemConfigStorage)
	if err != nil {
		t.Fatal(err)
	}
	if row.Version != 2 {
		t.Fatalf("updated storage version = %d, want 2", row.Version)
	}

	reloaded, err := New(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Snapshot().Storage.Retention; got != "7d" {
		t.Fatalf("reloaded retention = %q, want database value 7d", got)
	}
	if _, err := reloaded.Reset(ctx, app.SystemConfigStorage); err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Snapshot().Storage.Retention; got != app.DefaultRuntimeConfig().Storage.Retention {
		t.Fatalf("reset retention = %q, want %q", got, app.DefaultRuntimeConfig().Storage.Retention)
	}
}

func TestManagerRejectsStaleVersionAndUpdatesNestedPath(t *testing.T) {
	ctx := context.Background()
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager, err := New(ctx, database)
	if err != nil {
		t.Fatal(err)
	}

	version := manager.Version(app.SystemConfigLogging)
	if _, err := manager.UpdatePath(ctx, app.SystemConfigLogging, "logging.level", json.RawMessage(`"debug"`), version); err != nil {
		t.Fatal(err)
	}
	if got := manager.Snapshot().Logging.Level; got != "debug" {
		t.Fatalf("updated log level = %q, want debug", got)
	}
	if _, err := manager.UpdatePath(ctx, app.SystemConfigLogging, "logging.add_source", json.RawMessage(`false`), version); !errors.Is(err, store.ErrSystemConfigVersionConflict) {
		t.Fatalf("stale version error = %v, want version conflict", err)
	}
}

func TestAuthRowPreservesKeyAcrossRuntimeUpdatesAndReset(t *testing.T) {
	ctx := context.Background()
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager, err := New(ctx, database)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := manager.UpdatePath(ctx, app.SystemConfigAuth, "auth.session_ttl", json.RawMessage(`"168h"`), manager.Version(app.SystemConfigAuth)); err != nil {
		t.Fatal(err)
	}
	service := auth.NewService(database, manager.Snapshot().SessionTTLDuration())
	if _, err := service.Setup(ctx, "a-secure-test-key"); err != nil {
		t.Fatal(err)
	}
	if manager.Snapshot().Auth.AdminKeyHash != "" {
		t.Fatal("runtime manager exposed the administrator key hash")
	}
	if _, err := manager.Reset(ctx, app.SystemConfigAuth); err != nil {
		t.Fatal(err)
	}
	if got := manager.Snapshot().Auth.SessionTTL; got != app.DefaultRuntimeConfig().Auth.SessionTTL {
		t.Fatalf("auth reset ttl = %q, want %q", got, app.DefaultRuntimeConfig().Auth.SessionTTL)
	}
	if _, err := service.Login(ctx, "a-secure-test-key"); err != nil {
		t.Fatalf("administrator key was lost during auth reset: %v", err)
	}
	row, err := database.GetSystemConfig(ctx, app.SystemConfigAuth)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(row.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data["admin_key_hash"] == nil || data["admin_key_hash"] == "" {
		t.Fatal("auth row does not contain administrator key hash")
	}
}

func TestManagerImportAppliesAndPublishesCompleteRuntimeConfig(t *testing.T) {
	ctx := context.Background()
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager, err := New(ctx, database)
	if err != nil {
		t.Fatal(err)
	}

	changes := manager.Subscribe()
	applyCalls := 0
	manager.SetApply(func(_ context.Context, oldConfig, newConfig app.RuntimeConfig) error {
		applyCalls++
		if oldConfig.Scheduler.MaxConcurrency == newConfig.Scheduler.MaxConcurrency {
			t.Fatal("apply did not receive changed config")
		}
		return nil
	})
	candidate := manager.Snapshot()
	candidate.Scheduler.MaxConcurrency = 7
	candidate.Storage.Retention = "14d"
	_, err = manager.Import(ctx, candidate, func(ctx context.Context, domains map[string]json.RawMessage) (map[string]int, error) {
		result, err := database.ImportConfiguration(ctx, store.ConfigurationImport{Runtime: domains})
		return result.Versions, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if applyCalls != 1 {
		t.Fatalf("apply calls = %d, want 1", applyCalls)
	}
	if got := manager.Snapshot(); got.Scheduler.MaxConcurrency != 7 || got.Storage.Retention != "14d" {
		t.Fatalf("unexpected imported snapshot: %+v", got)
	}
	select {
	case <-changes:
	default:
		t.Fatal("runtime import did not notify subscribers")
	}
	reloaded, err := New(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Snapshot(); got.Scheduler.MaxConcurrency != 7 || got.Storage.Retention != "14d" {
		t.Fatalf("runtime import was not persisted: %+v", got)
	}
}
