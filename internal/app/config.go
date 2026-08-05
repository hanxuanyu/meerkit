package app

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Storage   StorageConfig   `yaml:"storage"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Logging   LoggingConfig   `yaml:"logging"`
	Security  SecurityConfig  `yaml:"security"`
}

type ServerConfig struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

type StorageConfig struct {
	DataDir   string `yaml:"data_dir"`
	Retention string `yaml:"retention"`
}

type SchedulerConfig struct {
	Timezone         string `yaml:"timezone"`
	MaxConcurrency   int    `yaml:"max_concurrency"`
	PollMilliseconds int    `yaml:"poll_milliseconds"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type SecurityConfig struct {
	SessionTTL    string `yaml:"session_ttl"`
	MasterKeyFile string `yaml:"master_key_file"`
}

func DefaultConfig() Config {
	return Config{
		Server:    ServerConfig{Address: "127.0.0.1", Port: 8080},
		Storage:   StorageConfig{DataDir: "./data", Retention: "30d"},
		Scheduler: SchedulerConfig{Timezone: "Local", MaxConcurrency: 16, PollMilliseconds: 500},
		Logging:   LoggingConfig{Level: "info", Format: "text"},
		Security:  SecurityConfig{SessionTTL: "24h", MasterKeyFile: "./data/master.key"},
	}
}

func LoadConfig(args []string) (Config, error) {
	cfg := DefaultConfig()
	flags := flag.NewFlagSet("meerkit", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", "", "path to config.yaml")
	listen := flags.String("listen", "", "listen address, for example 127.0.0.1:8080")
	dataDir := flags.String("data-dir", "", "SQLite data directory")
	retention := flags.String("retention", "", "record retention, for example 30d")
	timezone := flags.String("timezone", "", "scheduler timezone")
	maxConcurrency := flags.Int("max-concurrency", 0, "maximum concurrent checks")
	logLevel := flags.String("log-level", "", "log level")
	if err := flags.Parse(args); err != nil {
		return cfg, err
	}

	path := *configPath
	if path == "" {
		path = os.Getenv("MEERKIT_CONFIG_FILE")
	}
	if path == "" {
		path = "config.yaml"
	}
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parse config %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) || *configPath != "" || os.Getenv("MEERKIT_CONFIG_FILE") != "" {
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := applyEnvironment(&cfg); err != nil {
		return cfg, err
	}
	if *listen != "" {
		host, port, err := splitListen(*listen)
		if err != nil {
			return cfg, err
		}
		cfg.Server.Address, cfg.Server.Port = host, port
	}
	if *dataDir != "" {
		cfg.Storage.DataDir = *dataDir
	}
	if *retention != "" {
		cfg.Storage.Retention = *retention
	}
	if *timezone != "" {
		cfg.Scheduler.Timezone = *timezone
	}
	if *maxConcurrency > 0 {
		cfg.Scheduler.MaxConcurrency = *maxConcurrency
	}
	if *logLevel != "" {
		cfg.Logging.Level = *logLevel
	}
	return normalizeConfig(cfg)
}

func applyEnvironment(cfg *Config) error {
	if value := os.Getenv("MEERKIT_SERVER__ADDRESS"); value != "" {
		cfg.Server.Address = value
	}
	if value := os.Getenv("MEERKIT_SERVER__PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("MEERKIT_SERVER__PORT: %w", err)
		}
		cfg.Server.Port = port
	}
	if value := os.Getenv("MEERKIT_STORAGE__DATA_DIR"); value != "" {
		cfg.Storage.DataDir = value
	}
	if value := os.Getenv("MEERKIT_STORAGE__RETENTION"); value != "" {
		cfg.Storage.Retention = value
	}
	if value := os.Getenv("MEERKIT_SCHEDULER__TIMEZONE"); value != "" {
		cfg.Scheduler.Timezone = value
	}
	if value := os.Getenv("MEERKIT_SCHEDULER__MAX_CONCURRENCY"); value != "" {
		count, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("MEERKIT_SCHEDULER__MAX_CONCURRENCY: %w", err)
		}
		cfg.Scheduler.MaxConcurrency = count
	}
	if value := os.Getenv("MEERKIT_SCHEDULER__POLL_MILLISECONDS"); value != "" {
		milliseconds, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("MEERKIT_SCHEDULER__POLL_MILLISECONDS: %w", err)
		}
		cfg.Scheduler.PollMilliseconds = milliseconds
	}
	if value := os.Getenv("MEERKIT_LOGGING__LEVEL"); value != "" {
		cfg.Logging.Level = value
	}
	if value := os.Getenv("MEERKIT_LOGGING__FORMAT"); value != "" {
		cfg.Logging.Format = value
	}
	if value := os.Getenv("MEERKIT_SECURITY__SESSION_TTL"); value != "" {
		cfg.Security.SessionTTL = value
	}
	if value := os.Getenv("MEERKIT_SECURITY__MASTER_KEY_FILE"); value != "" {
		cfg.Security.MasterKeyFile = value
	}
	return nil
}

func normalizeConfig(cfg Config) (Config, error) {
	if cfg.Server.Address == "" {
		return cfg, errors.New("server.address cannot be empty")
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return cfg, errors.New("server.port must be between 1 and 65535")
	}
	if cfg.Storage.DataDir == "" {
		return cfg, errors.New("storage.data_dir cannot be empty")
	}
	if cfg.Scheduler.MaxConcurrency < 1 {
		return cfg, errors.New("scheduler.max_concurrency must be positive")
	}
	if cfg.Scheduler.PollMilliseconds < 100 {
		return cfg, errors.New("scheduler.poll_milliseconds must be at least 100")
	}
	if _, err := parseRetention(cfg.Storage.Retention); err != nil {
		return cfg, fmt.Errorf("storage.retention: %w", err)
	}
	if _, err := time.LoadLocation(cfg.Scheduler.Timezone); err != nil && cfg.Scheduler.Timezone != "Local" {
		return cfg, fmt.Errorf("scheduler.timezone: %w", err)
	}
	return cfg, nil
}

func parseRetention(value string) (time.Duration, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.HasSuffix(value, "d") {
		days, err := strconv.ParseFloat(strings.TrimSuffix(value, "d"), 64)
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("invalid day duration %q", value)
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid duration %q", value)
	}
	return duration, nil
}

func splitListen(value string) (string, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("listen must use host:port format")
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid listen port: %w", err)
	}
	return parts[0], port, nil
}

func (c Config) ListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Address, c.Server.Port)
}

func (c Config) RetentionDuration() time.Duration {
	duration, _ := parseRetention(c.Storage.Retention)
	return duration
}

func (c Config) SchedulerLocation() *time.Location {
	if c.Scheduler.Timezone == "Local" || c.Scheduler.Timezone == "" {
		return time.Local
	}
	location, err := time.LoadLocation(c.Scheduler.Timezone)
	if err != nil {
		return time.Local
	}
	return location
}
