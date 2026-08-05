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
	for _, item := range config.Metadata.Items {
		if item.Path == "scheduler.poll_milliseconds" {
			pollDescription = item.Description
			break
		}
	}
	if !strings.Contains(pollDescription, "不决定监控执行频率") {
		t.Fatalf("poll metadata description does not explain cron behavior: %q", pollDescription)
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
	if !strings.Contains(string(data), "server:") || !strings.Contains(string(data), "port: 8080") {
		t.Fatalf("generated config does not contain defaults: %s", data)
	}
	if config.Server.Port != DefaultConfig().Server.Port {
		t.Fatalf("generated default port = %d, want %d", config.Server.Port, DefaultConfig().Server.Port)
	}
	if config.Metadata.ConfigFile == "" {
		t.Fatal("generated config file was not reported in metadata")
	}
}
