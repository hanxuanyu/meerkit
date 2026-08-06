package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  address: 0.0.0.0\n  port: 9000\nstorage:\n  data_dir: ./from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MEERKIT_SERVER__PORT", "9001")
	config, err := LoadConfig([]string{"--config", path, "--listen", "127.0.0.1:9002"})
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	if config.Server.Address != "127.0.0.1" || config.Server.Port != 9002 {
		t.Fatalf("CLI should override environment and file: %#v", config.Server)
	}
	if config.Storage.DataDir != "./from-file" {
		t.Fatalf("file value was not loaded: %s", config.Storage.DataDir)
	}
	if config.Storage.NotificationRetention != "30d" || config.Storage.CleanupInterval != "1h" {
		t.Fatalf("cleanup defaults were not loaded: %#v", config.Storage)
	}
}

func TestLoadLoggingConfigFromEnvironmentAndFlags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("logging:\n  level: error\n  format: text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	logDirectory := filepath.Join(t.TempDir(), "logs")
	t.Setenv("MEERKIT_LOGGING__LEVEL", "debug")
	t.Setenv("MEERKIT_LOGGING__FILE__DIRECTORY", logDirectory)
	t.Setenv("MEERKIT_LOGGING__FILE__ACCESS__FILENAME", "access.log")
	config, err := LoadConfig([]string{"--config", path, "--log-level", "warn"})
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	if config.Logging.Level != "warn" {
		t.Fatalf("CLI should override environment: %s", config.Logging.Level)
	}
	if config.Logging.File.Directory != logDirectory {
		t.Fatalf("environment file directory was not loaded: %s", config.Logging.File.Directory)
	}
	if config.Logging.File.Access.Filename != "access.log" {
		t.Fatalf("environment access file name was not loaded: %s", config.Logging.File.Access.Filename)
	}
	if config.Metadata.ConfigFile != path {
		t.Fatalf("metadata config file = %q, want %q", config.Metadata.ConfigFile, path)
	}
	var pollDescription string
	var notificationRetentionDescription string
	var cleanupIntervalDescription string
	for _, item := range config.Metadata.Items {
		if item.Path == "scheduler.poll_milliseconds" {
			pollDescription = item.Description
		}
		if item.Path == "storage.notification_retention" {
			notificationRetentionDescription = item.Description
		}
		if item.Path == "storage.cleanup_interval" {
			cleanupIntervalDescription = item.Description
		}
	}
	if !strings.Contains(pollDescription, "不决定监控执行频率") {
		t.Fatalf("poll metadata description does not explain cron behavior: %q", pollDescription)
	}
	if notificationRetentionDescription == "" || cleanupIntervalDescription == "" {
		t.Fatalf("cleanup metadata descriptions are missing: notification=%q interval=%q", notificationRetentionDescription, cleanupIntervalDescription)
	}
}

func TestLoadConfigCreatesDefaultFileWhenMissing(t *testing.T) {
	directory := t.TempDir()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(workingDirectory)

	config, err := LoadConfig(nil)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "config.yaml"))
	if err != nil {
		t.Fatalf("generated config.yaml was not created: %v", err)
	}
	if !strings.Contains(string(data), "server:") || !strings.Contains(string(data), "address: 0.0.0.0") || !strings.Contains(string(data), "port: 8080") {
		t.Fatalf("generated config does not contain defaults: %s", data)
	}
	if config.Server.Port != DefaultConfig().Server.Port {
		t.Fatalf("generated default port = %d, want %d", config.Server.Port, DefaultConfig().Server.Port)
	}
	if config.Server.Address != "0.0.0.0" {
		t.Fatalf("generated default address = %q, want 0.0.0.0", config.Server.Address)
	}
	if config.Logging.Format != "simple" {
		t.Fatalf("generated default log format = %q, want simple", config.Logging.Format)
	}
	if config.Metadata.ConfigFile == "" {
		t.Fatal("generated config file was not reported in metadata")
	}
}

func TestTrustedPluginKeysAreIncludedInConfigMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := "plugins:\n  trusted_keys:\n    release-2026: dGVzdA==\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfigWithOptions(ConfigOptions{ConfigFile: path})
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	for _, item := range config.Metadata.Items {
		if item.Path != "plugins.trusted_keys" {
			continue
		}
		keys, ok := item.Value.(map[string]string)
		if !ok || keys["release-2026"] != "dGVzdA==" {
			t.Fatalf("trusted key metadata value = %#v", item.Value)
		}
		if item.Source != "config_file" {
			t.Fatalf("trusted key metadata source = %q, want config_file", item.Source)
		}
		return
	}
	t.Fatal("plugins.trusted_keys is missing from config metadata")
}

func TestPluginConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("plugins:\n  source_dir: ./custom-plugins\n  log_level: debug\n  log_format: json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfigWithOptions(ConfigOptions{ConfigFile: path})
	if err != nil {
		t.Fatalf("load plugin config: %v", err)
	}
	if config.Plugins.SourceDir != "./custom-plugins" || config.Plugins.LogLevel != "debug" || config.Plugins.LogFormat != "json" {
		t.Fatalf("plugin config = %#v", config.Plugins)
	}
	definitions := map[string]ConfigItem{}
	for _, item := range config.Metadata.Items {
		definitions[item.Path] = item
	}
	if definitions["plugins.source_dir"].Description == "" || definitions["plugins.log_format"].Description == "" {
		t.Fatalf("plugin config metadata is incomplete: %#v", definitions)
	}
}

func TestPluginLogConfigurationFromEnvironment(t *testing.T) {
	t.Setenv("MEERKIT_PLUGINS__LOG_LEVEL", "warning")
	t.Setenv("MEERKIT_PLUGINS__LOG_FORMAT", "simple")
	config, err := LoadConfigWithOptions(ConfigOptions{})
	if err != nil {
		t.Fatalf("load plugin logging config: %v", err)
	}
	if config.Plugins.LogLevel != "warn" || config.Plugins.LogFormat != "simple" {
		t.Fatalf("plugin logging config = %#v", config.Plugins)
	}
}

func TestRejectsUnknownLogFormats(t *testing.T) {
	config := DefaultConfig()
	config.Logging.Format = "pretty"
	if _, err := normalizeConfig(config); err == nil || !strings.Contains(err.Error(), "logging.format") {
		t.Fatalf("invalid host log format error = %v", err)
	}
	config = DefaultConfig()
	config.Plugins.LogFormat = "pretty"
	if _, err := normalizeConfig(config); err == nil || !strings.Contains(err.Error(), "plugins.log_format") {
		t.Fatalf("invalid plugin log format error = %v", err)
	}
}
