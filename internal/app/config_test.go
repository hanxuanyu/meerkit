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
	if config.Logging.File.Directory != DefaultConfig().Logging.File.Directory {
		t.Fatalf("static logging defaults were not loaded: %#v", config.Logging.File)
	}
}

func TestDatabaseConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := "storage:\n  database:\n    type: mysql\n    dsn: user:secret@tcp(localhost:3306)/meerkit\n    auto_migrate: false\n    max_open_conns: 40\n    max_idle_conns: 12\n    conn_max_lifetime: 20m\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfigWithOptions(ConfigOptions{ConfigFile: path})
	if err != nil {
		t.Fatalf("load mysql config: %v", err)
	}
	if config.Storage.Database.Type != "mysql" || config.Storage.Database.AutoMigrate || config.Storage.Database.MaxOpenConns != 40 {
		t.Fatalf("unexpected database config: %#v", config.Storage.Database)
	}
	for _, item := range config.Metadata.Items {
		if item.Path == "storage.database.dsn" && item.Value != "configured" {
			t.Fatalf("database DSN leaked through metadata: %#v", item.Value)
		}
	}
}

func TestMCPConfigurationRequiresAndRedactsToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("mcp:\n  enabled: true\n  token: short\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfigWithOptions(ConfigOptions{ConfigFile: path}); err == nil || !strings.Contains(err.Error(), "mcp.token") {
		t.Fatalf("expected invalid MCP token error, got %v", err)
	}
	token := "test-mcp-token-with-at-least-32-characters"
	if err := os.WriteFile(path, []byte("mcp:\n  enabled: true\n  token: "+token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfigWithOptions(ConfigOptions{ConfigFile: path})
	if err != nil {
		t.Fatalf("load MCP config: %v", err)
	}
	if !config.MCP.Enabled || config.MCP.Token != token {
		t.Fatalf("unexpected MCP config: %#v", config.MCP)
	}
	for _, item := range config.Metadata.Items {
		if item.Path == "mcp.token" {
			if item.Value != "configured" || item.Default != "" {
				t.Fatalf("MCP token metadata leaked the token: %#v", item)
			}
			return
		}
	}
	t.Fatal("mcp.token is missing from configuration metadata")
}

func TestMySQLRequiresDSN(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("storage:\n  database:\n    type: mysql\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfigWithOptions(ConfigOptions{ConfigFile: path}); err == nil || !strings.Contains(err.Error(), "dsn") {
		t.Fatalf("expected missing mysql DSN error, got %v", err)
	}
}

func TestRuntimeConfigIsNotLoadedFromYamlEnvironmentOrFlags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("logging:\n  level: error\n  format: text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	logDirectory := filepath.Join(t.TempDir(), "logs")
	t.Setenv("MEERKIT_LOGGING__LEVEL", "debug")
	t.Setenv("MEERKIT_LOGGING__FILE__DIRECTORY", logDirectory)
	t.Setenv("MEERKIT_LOGGING__FILE__ACCESS__FILENAME", "access.log")
	config, err := LoadConfig([]string{"--config", path})
	if err != nil {
		t.Fatalf("load config failed: %v", err)
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
	for _, item := range config.Metadata.Items {
		if strings.HasPrefix(item.Path, "scheduler.") || strings.HasPrefix(item.Path, "storage.retention") {
			t.Fatalf("runtime config leaked into startup metadata: %s", item.Path)
		}
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
	for _, forbidden := range []string{"retention:", "timezone:", "session_ttl:", "log_level:", "log_format:"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("generated config contains runtime setting %q: %s", forbidden, data)
		}
	}
	if config.Server.Port != DefaultConfig().Server.Port {
		t.Fatalf("generated default port = %d, want %d", config.Server.Port, DefaultConfig().Server.Port)
	}
	if config.Server.Address != "0.0.0.0" {
		t.Fatalf("generated default address = %q, want 0.0.0.0", config.Server.Address)
	}
	if DefaultRuntimeConfig().Logging.Format != "simple" {
		t.Fatalf("generated runtime default log format = %q, want simple", DefaultRuntimeConfig().Logging.Format)
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
	if config.Plugins.SourceDir != "./custom-plugins" {
		t.Fatalf("plugin config = %#v", config.Plugins)
	}
	definitions := map[string]ConfigItem{}
	for _, item := range config.Metadata.Items {
		definitions[item.Path] = item
	}
	if definitions["plugins.source_dir"].Description == "" {
		t.Fatalf("plugin config metadata is incomplete: %#v", definitions)
	}
}

func TestPluginLogConfigurationIsNotLoadedFromEnvironment(t *testing.T) {
	t.Setenv("MEERKIT_PLUGINS__LOG_LEVEL", "warning")
	t.Setenv("MEERKIT_PLUGINS__LOG_FORMAT", "simple")
	config, err := LoadConfigWithOptions(ConfigOptions{})
	if err != nil {
		t.Fatalf("load plugin logging config: %v", err)
	}
	for _, item := range config.Metadata.Items {
		if item.Path == "plugins.log_level" || item.Path == "plugins.log_format" {
			t.Fatalf("plugin logging config leaked into startup metadata: %s", item.Path)
		}
	}
}

func TestRejectsUnknownLogFormats(t *testing.T) {
	runtime := DefaultRuntimeConfig()
	runtime.Logging.Format = "pretty"
	if err := runtime.Validate(); err == nil || !strings.Contains(err.Error(), "logging.format") {
		t.Fatalf("invalid host log format error = %v", err)
	}
	runtime = DefaultRuntimeConfig()
	runtime.Plugins.LogFormat = "pretty"
	if err := runtime.Validate(); err == nil || !strings.Contains(err.Error(), "plugins.log_format") {
		t.Fatalf("invalid plugin log format error = %v", err)
	}
}
