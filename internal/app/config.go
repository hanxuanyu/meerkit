package app

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server" mapstructure:"server"`
	Storage   StorageConfig   `yaml:"storage" mapstructure:"storage"`
	Scheduler SchedulerConfig `yaml:"scheduler" mapstructure:"scheduler"`
	Logging   LoggingConfig   `yaml:"logging" mapstructure:"logging"`
	Security  SecurityConfig  `yaml:"security" mapstructure:"security"`
	Plugins   PluginConfig    `yaml:"plugins" mapstructure:"plugins"`
	Metadata  ConfigMetadata  `yaml:"-" mapstructure:"-"`
}

type ServerConfig struct {
	Address string `yaml:"address" mapstructure:"address"`
	Port    int    `yaml:"port" mapstructure:"port"`
}

type StorageConfig struct {
	DataDir               string `yaml:"data_dir" mapstructure:"data_dir"`
	Retention             string `yaml:"retention" mapstructure:"retention"`
	NotificationRetention string `yaml:"notification_retention" mapstructure:"notification_retention"`
	CleanupInterval       string `yaml:"cleanup_interval" mapstructure:"cleanup_interval"`
}

type SchedulerConfig struct {
	Timezone         string `yaml:"timezone" mapstructure:"timezone"`
	MaxConcurrency   int    `yaml:"max_concurrency" mapstructure:"max_concurrency"`
	PollMilliseconds int    `yaml:"poll_milliseconds" mapstructure:"poll_milliseconds"`
}

type LoggingConfig struct {
	Level     string           `yaml:"level" mapstructure:"level"`
	Format    string           `yaml:"format" mapstructure:"format"`
	AddSource bool             `yaml:"add_source" mapstructure:"add_source"`
	Console   ConsoleLogConfig `yaml:"console" mapstructure:"console"`
	File      LogFileConfig    `yaml:"file" mapstructure:"file"`
}

type ConsoleLogConfig struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	Access  bool `yaml:"access" mapstructure:"access"`
}

type LogFileConfig struct {
	Enabled    bool             `yaml:"enabled" mapstructure:"enabled"`
	Directory  string           `yaml:"directory" mapstructure:"directory"`
	Filename   string           `yaml:"filename" mapstructure:"filename"`
	MaxSizeMB  int              `yaml:"max_size_mb" mapstructure:"max_size_mb"`
	MaxBackups int              `yaml:"max_backups" mapstructure:"max_backups"`
	MaxAgeDays int              `yaml:"max_age_days" mapstructure:"max_age_days"`
	Compress   bool             `yaml:"compress" mapstructure:"compress"`
	Access     AccessFileConfig `yaml:"access" mapstructure:"access"`
}

type AccessFileConfig struct {
	Enabled  bool   `yaml:"enabled" mapstructure:"enabled"`
	Filename string `yaml:"filename" mapstructure:"filename"`
}

type SecurityConfig struct {
	SessionTTL    string `yaml:"session_ttl" mapstructure:"session_ttl"`
	MasterKeyFile string `yaml:"master_key_file" mapstructure:"master_key_file"`
}

type PluginConfig struct {
	SourceDir   string            `yaml:"source_dir" mapstructure:"source_dir"`
	LogLevel    string            `yaml:"log_level" mapstructure:"log_level"`
	LogFormat   string            `yaml:"log_format" mapstructure:"log_format"`
	TrustedKeys map[string]string `yaml:"trusted_keys" mapstructure:"trusted_keys"`
}

type ConfigOptions struct {
	ConfigFile    string
	CreateDefault bool
	Listen        string
	Overrides     map[string]any
	ChangedFlags  map[string]bool
}

func DefaultConfig() Config {
	return Config{
		Server:    ServerConfig{Address: "0.0.0.0", Port: 8080},
		Storage:   StorageConfig{DataDir: "./data", Retention: "30d", NotificationRetention: "30d", CleanupInterval: "1h"},
		Scheduler: SchedulerConfig{Timezone: "Local", MaxConcurrency: 16, PollMilliseconds: 500},
		Logging: LoggingConfig{
			Level: "info", Format: "simple", AddSource: true,
			Console: ConsoleLogConfig{Enabled: true, Access: false},
			File: LogFileConfig{
				Enabled: true, Directory: "./logs", Filename: "meerkit.log",
				MaxSizeMB: 100, MaxBackups: 7, MaxAgeDays: 30, Compress: true,
				Access: AccessFileConfig{Enabled: true, Filename: "meerkit-access.log"},
			},
		},
		Security: SecurityConfig{SessionTTL: "720h", MasterKeyFile: "./data/master.key"},
		Plugins:  PluginConfig{SourceDir: "./plugins", LogLevel: "info", LogFormat: "simple", TrustedKeys: map[string]string{}},
	}
}

// LoadConfig is kept as a compatibility wrapper for package callers. New commands
// parse flags with Cobra and call LoadConfigWithOptions directly.
func LoadConfig(args []string) (Config, error) {
	flags := pflag.NewFlagSet("meerkit", pflag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", "", "path to config.yaml")
	listen := flags.String("listen", "", "listen address")
	flags.String("data-dir", "", "SQLite data directory")
	flags.String("retention", "", "record retention")
	flags.String("timezone", "", "scheduler timezone")
	flags.Int("max-concurrency", 0, "maximum concurrent checks")
	flags.String("log-level", "", "log level")
	flags.String("log-format", "", "log format")
	flags.String("log-dir", "", "log file directory")
	flags.String("log-filename", "", "log file name")
	flags.Bool("log-console", false, "enable business logging on console")
	flags.Bool("log-file-enabled", false, "enable file logging")
	flags.Bool("log-console-access", false, "enable HTTP access logging on console")
	flags.String("access-log-filename", "", "HTTP access log file name")
	flags.Bool("access-log-file-enabled", false, "enable HTTP access log file")
	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}
	options := ConfigOptions{ConfigFile: *configPath, CreateDefault: true, Listen: *listen, Overrides: map[string]any{}, ChangedFlags: map[string]bool{}}
	bindings := map[string]string{"storage.data_dir": "data-dir", "storage.retention": "retention", "scheduler.timezone": "timezone", "scheduler.max_concurrency": "max-concurrency", "logging.level": "log-level", "logging.format": "log-format", "logging.file.directory": "log-dir", "logging.file.filename": "log-filename", "logging.console.enabled": "log-console", "logging.console.access": "log-console-access", "logging.file.enabled": "log-file-enabled", "logging.file.access.filename": "access-log-filename", "logging.file.access.enabled": "access-log-file-enabled"}
	for key, name := range bindings {
		if flags.Changed(name) {
			options.ChangedFlags[name] = true
			value, _ := flags.GetString(name)
			if flag := flags.Lookup(name); flag != nil && flag.Value.Type() == "bool" {
				boolValue, _ := flags.GetBool(name)
				options.Overrides[key] = boolValue
			} else if flag != nil && flag.Value.Type() == "int" {
				intValue, _ := flags.GetInt(name)
				options.Overrides[key] = intValue
			} else {
				options.Overrides[key] = value
			}
		}
	}
	if flags.Changed("listen") {
		options.ChangedFlags["listen"] = true
	}
	return LoadConfigWithOptions(options)
}

func LoadConfigWithOptions(options ConfigOptions) (Config, error) {
	v := viper.New()
	setDefaults(v)
	if err := bindEnvironment(v); err != nil {
		return Config{}, err
	}
	path := options.ConfigFile
	if path == "" {
		path = os.Getenv("MEERKIT_CONFIG_FILE")
	}
	explicitConfig := path != ""
	if path == "" {
		path = "config.yaml"
	}
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		missing := errors.As(err, &notFound) || errors.Is(err, os.ErrNotExist)
		if !missing || explicitConfig {
			return Config{}, fmt.Errorf("read config %s: %w", pathOrDefault(path), err)
		}
		if options.CreateDefault {
			if err := writeDefaultConfig(path); err != nil {
				return Config{}, fmt.Errorf("create default config %s: %w", path, err)
			}
			if err := v.ReadInConfig(); err != nil {
				return Config{}, fmt.Errorf("read generated config %s: %w", path, err)
			}
		}
	}
	for key, value := range options.Overrides {
		v.Set(key, value)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if options.Listen != "" {
		host, port, err := splitListen(options.Listen)
		if err != nil {
			return Config{}, err
		}
		cfg.Server.Address, cfg.Server.Port = host, port
	}
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return Config{}, err
	}
	normalized.Metadata = buildConfigMetadata(v, options.ChangedFlags, normalized)
	return normalized, nil
}

func setDefaults(v *viper.Viper) {
	defaults := DefaultConfig()
	v.SetDefault("server.address", defaults.Server.Address)
	v.SetDefault("server.port", defaults.Server.Port)
	v.SetDefault("storage.data_dir", defaults.Storage.DataDir)
	v.SetDefault("storage.retention", defaults.Storage.Retention)
	v.SetDefault("storage.notification_retention", defaults.Storage.NotificationRetention)
	v.SetDefault("storage.cleanup_interval", defaults.Storage.CleanupInterval)
	v.SetDefault("scheduler.timezone", defaults.Scheduler.Timezone)
	v.SetDefault("scheduler.max_concurrency", defaults.Scheduler.MaxConcurrency)
	v.SetDefault("scheduler.poll_milliseconds", defaults.Scheduler.PollMilliseconds)
	v.SetDefault("logging.level", defaults.Logging.Level)
	v.SetDefault("logging.format", defaults.Logging.Format)
	v.SetDefault("logging.console.enabled", defaults.Logging.Console.Enabled)
	v.SetDefault("logging.console.access", defaults.Logging.Console.Access)
	v.SetDefault("logging.add_source", defaults.Logging.AddSource)
	v.SetDefault("logging.file.enabled", defaults.Logging.File.Enabled)
	v.SetDefault("logging.file.directory", defaults.Logging.File.Directory)
	v.SetDefault("logging.file.filename", defaults.Logging.File.Filename)
	v.SetDefault("logging.file.max_size_mb", defaults.Logging.File.MaxSizeMB)
	v.SetDefault("logging.file.max_backups", defaults.Logging.File.MaxBackups)
	v.SetDefault("logging.file.max_age_days", defaults.Logging.File.MaxAgeDays)
	v.SetDefault("logging.file.compress", defaults.Logging.File.Compress)
	v.SetDefault("logging.file.access.enabled", defaults.Logging.File.Access.Enabled)
	v.SetDefault("logging.file.access.filename", defaults.Logging.File.Access.Filename)
	v.SetDefault("security.session_ttl", defaults.Security.SessionTTL)
	v.SetDefault("security.master_key_file", defaults.Security.MasterKeyFile)
	v.SetDefault("plugins.source_dir", defaults.Plugins.SourceDir)
	v.SetDefault("plugins.log_level", defaults.Plugins.LogLevel)
	v.SetDefault("plugins.log_format", defaults.Plugins.LogFormat)
	v.SetDefault("plugins.trusted_keys", defaults.Plugins.TrustedKeys)
}

func bindEnvironment(v *viper.Viper) error {
	v.SetEnvPrefix("MEERKIT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "__"))
	v.AutomaticEnv()
	envKeys := map[string]string{
		"server.address":                 "MEERKIT_SERVER__ADDRESS",
		"server.port":                    "MEERKIT_SERVER__PORT",
		"storage.data_dir":               "MEERKIT_STORAGE__DATA_DIR",
		"storage.retention":              "MEERKIT_STORAGE__RETENTION",
		"storage.notification_retention": "MEERKIT_STORAGE__NOTIFICATION_RETENTION",
		"storage.cleanup_interval":       "MEERKIT_STORAGE__CLEANUP_INTERVAL",
		"scheduler.timezone":             "MEERKIT_SCHEDULER__TIMEZONE",
		"scheduler.max_concurrency":      "MEERKIT_SCHEDULER__MAX_CONCURRENCY",
		"scheduler.poll_milliseconds":    "MEERKIT_SCHEDULER__POLL_MILLISECONDS",
		"logging.level":                  "MEERKIT_LOGGING__LEVEL",
		"logging.format":                 "MEERKIT_LOGGING__FORMAT",
		"logging.console.enabled":        "MEERKIT_LOGGING__CONSOLE__ENABLED",
		"logging.console.access":         "MEERKIT_LOGGING__CONSOLE__ACCESS",
		"logging.add_source":             "MEERKIT_LOGGING__ADD_SOURCE",
		"logging.file.enabled":           "MEERKIT_LOGGING__FILE__ENABLED",
		"logging.file.directory":         "MEERKIT_LOGGING__FILE__DIRECTORY",
		"logging.file.filename":          "MEERKIT_LOGGING__FILE__FILENAME",
		"logging.file.max_size_mb":       "MEERKIT_LOGGING__FILE__MAX_SIZE_MB",
		"logging.file.max_backups":       "MEERKIT_LOGGING__FILE__MAX_BACKUPS",
		"logging.file.max_age_days":      "MEERKIT_LOGGING__FILE__MAX_AGE_DAYS",
		"logging.file.compress":          "MEERKIT_LOGGING__FILE__COMPRESS",
		"logging.file.access.enabled":    "MEERKIT_LOGGING__FILE__ACCESS__ENABLED",
		"logging.file.access.filename":   "MEERKIT_LOGGING__FILE__ACCESS__FILENAME",
		"security.session_ttl":           "MEERKIT_SECURITY__SESSION_TTL",
		"security.master_key_file":       "MEERKIT_SECURITY__MASTER_KEY_FILE",
		"plugins.source_dir":             "MEERKIT_PLUGINS__SOURCE_DIR",
		"plugins.log_level":              "MEERKIT_PLUGINS__LOG_LEVEL",
		"plugins.log_format":             "MEERKIT_PLUGINS__LOG_FORMAT",
	}
	for key, env := range envKeys {
		if err := v.BindEnv(key, env); err != nil {
			return fmt.Errorf("bind %s: %w", env, err)
		}
	}
	return nil
}

func bindFlags(v *viper.Viper, flags *pflag.FlagSet) {
	bindings := map[string]string{
		"storage.data_dir":             "data-dir",
		"storage.retention":            "retention",
		"scheduler.timezone":           "timezone",
		"scheduler.max_concurrency":    "max-concurrency",
		"logging.level":                "log-level",
		"logging.format":               "log-format",
		"logging.file.directory":       "log-dir",
		"logging.file.filename":        "log-filename",
		"logging.console.enabled":      "log-console",
		"logging.console.access":       "log-console-access",
		"logging.file.enabled":         "log-file-enabled",
		"logging.file.access.filename": "access-log-filename",
		"logging.file.access.enabled":  "access-log-file-enabled",
	}
	for key, flagName := range bindings {
		if flags.Changed(flagName) {
			_ = v.BindPFlag(key, flags.Lookup(flagName))
		}
	}
}

func pathOrDefault(path string) string {
	if path == "" {
		return "config.yaml"
	}
	return path
}

func writeDefaultConfig(path string) error {
	data, err := yaml.Marshal(DefaultConfig())
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
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
	level, err := normalizeLogLevel("logging.level", cfg.Logging.Level)
	if err != nil {
		return cfg, err
	}
	cfg.Logging.Level = level
	cfg.Logging.Format, err = normalizeLogFormat("logging.format", cfg.Logging.Format)
	if err != nil {
		return cfg, err
	}
	if !cfg.Logging.Console.Enabled && !cfg.Logging.File.Enabled {
		return cfg, errors.New("at least one logging output must be enabled")
	}
	if err := validateLogFile("logging.file", cfg.Logging.File, cfg.Logging.File.Enabled); err != nil {
		return cfg, err
	}
	if cfg.Logging.File.Enabled && cfg.Logging.File.Access.Enabled {
		if strings.TrimSpace(cfg.Logging.File.Access.Filename) == "" {
			return cfg, errors.New("logging.file.access.filename cannot be empty")
		}
		if err := validateLogFilename("logging.file.access", cfg.Logging.File.Access.Filename); err != nil {
			return cfg, err
		}
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
	if _, err := parseRetention(cfg.Storage.NotificationRetention); err != nil {
		return cfg, fmt.Errorf("storage.notification_retention: %w", err)
	}
	if _, err := parseRetention(cfg.Storage.CleanupInterval); err != nil {
		return cfg, fmt.Errorf("storage.cleanup_interval: %w", err)
	}
	if _, err := time.LoadLocation(cfg.Scheduler.Timezone); err != nil && cfg.Scheduler.Timezone != "Local" {
		return cfg, fmt.Errorf("scheduler.timezone: %w", err)
	}
	if strings.TrimSpace(cfg.Plugins.SourceDir) == "" {
		return cfg, errors.New("plugins.source_dir cannot be empty")
	}
	cfg.Plugins.LogLevel, err = normalizeLogLevel("plugins.log_level", cfg.Plugins.LogLevel)
	if err != nil {
		return cfg, err
	}
	cfg.Plugins.LogFormat, err = normalizeLogFormat("plugins.log_format", cfg.Plugins.LogFormat)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

func normalizeLogLevel(path, value string) (string, error) {
	level := strings.ToLower(strings.TrimSpace(value))
	if level == "warning" {
		level = "warn"
	}
	if level != "debug" && level != "info" && level != "warn" && level != "error" {
		return "", fmt.Errorf("%s must be debug, info, warn, or error", path)
	}
	return level, nil
}

func normalizeLogFormat(path, value string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(value))
	if format != "text" && format != "simple" && format != "json" {
		return "", fmt.Errorf("%s must be text, simple, or json", path)
	}
	return format, nil
}

func validateLogFile(prefix string, file LogFileConfig, enabled bool) error {
	if !enabled {
		return nil
	}
	if strings.TrimSpace(file.Directory) == "" {
		return fmt.Errorf("%s.directory cannot be empty", prefix)
	}
	if err := validateLogFilename(prefix, file.Filename); err != nil {
		return err
	}
	if file.MaxSizeMB < 1 {
		return fmt.Errorf("%s.max_size_mb must be positive", prefix)
	}
	if file.MaxBackups < 0 || file.MaxAgeDays < 0 {
		return fmt.Errorf("%s.max_backups and max_age_days cannot be negative", prefix)
	}
	return nil
}

func validateLogFilename(prefix, filename string) error {
	if strings.TrimSpace(filename) == "" {
		return fmt.Errorf("%s.filename cannot be empty", prefix)
	}
	if filepath.Base(filename) != filename || filename == "." || filename == ".." {
		return fmt.Errorf("%s.filename must be a file name, got %q", prefix, filename)
	}
	return nil
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
	host, portValue, err := net.SplitHostPort(value)
	if err != nil {
		return "", 0, fmt.Errorf("listen must use host:port format: %w", err)
	}
	port, err := strconv.Atoi(portValue)
	if err != nil {
		return "", 0, fmt.Errorf("invalid listen port: %w", err)
	}
	return host, port, nil
}

func (c Config) ListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Address, c.Server.Port)
}

func (c Config) RetentionDuration() time.Duration {
	duration, _ := parseRetention(c.Storage.Retention)
	return duration
}

func (c Config) NotificationRetentionDuration() time.Duration {
	duration, _ := parseRetention(c.Storage.NotificationRetention)
	return duration
}

func (c Config) CleanupIntervalDuration() time.Duration {
	duration, _ := parseRetention(c.Storage.CleanupInterval)
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
