package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/hanxuanyu/meerkit/sdk/logging"
)

type lifecycleTestProvider struct {
	healthErr error
}

func (p lifecycleTestProvider) ListModules() ([]ModuleDescriptor, error) {
	return []ModuleDescriptor{{Type: "http"}}, nil
}

func (p lifecycleTestProvider) ValidateConfig(context.Context, string, json.RawMessage) error {
	return nil
}

func (p lifecycleTestProvider) Execute(context.Context, string, json.RawMessage) (Observation, error) {
	return Observation{Success: true}, nil
}

func (p lifecycleTestProvider) MigrateConfig(_ context.Context, _, _, _ string, config json.RawMessage) (json.RawMessage, error) {
	return config, nil
}

func (p lifecycleTestProvider) Health(context.Context) error { return p.healthErr }

func TestLoggingProviderRecordsLifecycleWithoutConfigContents(t *testing.T) {
	var output bytes.Buffer
	logger := logging.NewLogger(&output, "simple", slog.LevelDebug, false).With("plugin_id", "meerkit.http")
	provider := newLoggingProvider(lifecycleTestProvider{}, logger)
	if _, err := provider.ListModules(); err != nil {
		t.Fatal(err)
	}
	secretConfig := json.RawMessage("{\"token\":\"must-not-be-logged\"}")
	if err := provider.ValidateConfig(context.Background(), "http", secretConfig); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Execute(context.Background(), "http", secretConfig); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.MigrateConfig(context.Background(), "http", "1", "2", secretConfig); err != nil {
		t.Fatal(err)
	}
	logs := output.String()
	for _, message := range []string{"plugin modules discovered", "plugin config validated", "plugin execution started", "plugin execution completed", "plugin config migration completed"} {
		if !strings.Contains(logs, message) {
			t.Fatalf("missing %q in lifecycle logs:\n%s", message, logs)
		}
	}
	if strings.Contains(logs, "must-not-be-logged") {
		t.Fatalf("raw config leaked into lifecycle logs: %s", logs)
	}
}

func TestLoggingProviderRecordsHealthFailure(t *testing.T) {
	var output bytes.Buffer
	provider := newLoggingProvider(lifecycleTestProvider{healthErr: errors.New("unavailable")}, logging.NewLogger(&output, "simple", slog.LevelInfo, false))
	if err := provider.Health(context.Background()); err == nil {
		t.Fatal("expected health failure")
	}
	if !strings.Contains(output.String(), "[ERROR] plugin health check failed") {
		t.Fatalf("missing health failure log: %s", output.String())
	}
}
