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
	Server   ServerConfig   `yaml:"server" mapstructure:"server"`
	Storage  StorageConfig  `yaml:"storage" mapstructure:"storage"`
	Logging  LoggingConfig  `yaml:"logging" mapstructure:"logging"`
	Security SecurityConfig `yaml:"security" mapstructure:"security"`
	Plugins  PluginConfig   `yaml:"plugins" mapstructure:"plugins"`
	Metadata ConfigMetadata `yaml:"-" mapstructure:"-"`
}

type ServerConfig struct {
	Address string `yaml:"address" mapstructure:"address"`
	Port    int    `yaml:"port" mapstructure:"port"`
}

type StorageConfig struct {
	DataDir  string         `yaml:"data_dir" mapstructure:"data_dir"`
	Database DatabaseConfig `yaml:"database" mapstructure:"database"`
}

type DatabaseConfig struct {
	Type            string `yaml:"type" mapstructure:"type"`
	DSN             string `yaml:"dsn,omitempty" mapstructure:"dsn"`
	AutoMigrate     bool   `yaml:"auto_migrate" mapstructure:"auto_migrate"`
	MaxOpenConns    int    `yaml:"max_open_conns" mapstructure:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime" mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime string `yaml:"conn_max_idle_time" mapstructure:"conn_max_idle_time"`
}

func (c DatabaseConfig) ConnectionDurations() (time.Duration, time.Duration) {
	var lifetime, idle time.Duration
	if c.ConnMaxLifetime != "" {
		lifetime, _ = time.ParseDuration(c.ConnMaxLifetime)
	}
	if c.ConnMaxIdleTime != "" {
		idle, _ = time.ParseDuration(c.ConnMaxIdleTime)
	}
	return lifetime, idle
}

type LoggingConfig struct {
	File LogFileConfig `yaml:"file" mapstructure:"file"`
}

type LogFileConfig struct {
	Directory  string           `yaml:"directory" mapstructure:"directory"`
	Filename   string           `yaml:"filename" mapstructure:"filename"`
	MaxSizeMB  int              `yaml:"max_size_mb" mapstructure:"max_size_mb"`
	MaxBackups int              `yaml:"max_backups" mapstructure:"max_backups"`
	MaxAgeDays int              `yaml:"max_age_days" mapstructure:"max_age_days"`
	Compress   bool             `yaml:"compress" mapstructure:"compress"`
	Access     AccessFileConfig `yaml:"access" mapstructure:"access"`
}

type AccessFileConfig struct {
	Filename string `yaml:"filename" mapstructure:"filename"`
}

type SecurityConfig struct {
	MasterKeyFile  string `yaml:"master_key_file" mapstructure:"master_key_file"`
	AllowTokenCopy bool   `yaml:"allow_token_copy" mapstructure:"allow_token_copy"`
}

type PluginConfig struct {
	SourceDir   string            `yaml:"source_dir" mapstructure:"source_dir"`
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
		Server: ServerConfig{Address: "0.0.0.0", Port: 8080},
		Storage: StorageConfig{DataDir: "./data", Database: DatabaseConfig{
			Type: "sqlite", AutoMigrate: true,
		}},
		Logging: LoggingConfig{
			File: LogFileConfig{
				Directory: "./logs", Filename: "meerkit.log",
				MaxSizeMB: 100, MaxBackups: 7, MaxAgeDays: 30, Compress: true,
				Access: AccessFileConfig{Filename: "meerkit-access.log"},
			},
		},
		Security: SecurityConfig{MasterKeyFile: "./data/master.key", AllowTokenCopy: false},
		Plugins:  PluginConfig{SourceDir: "./plugins", TrustedKeys: map[string]string{}},
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
	flags.String("database-type", "", "database type")
	flags.String("database-dsn", "", "database DSN")
	flags.Bool("database-auto-migrate", true, "automatically migrate database schema")
	flags.String("log-dir", "", "log file directory")
	flags.String("log-filename", "", "log file name")
	flags.String("access-log-filename", "", "HTTP access log file name")
	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}
	options := ConfigOptions{ConfigFile: *configPath, CreateDefault: true, Listen: *listen, Overrides: map[string]any{}, ChangedFlags: map[string]bool{}}
	bindings := map[string]string{
		"storage.data_dir": "data-dir", "storage.database.type": "database-type", "storage.database.dsn": "database-dsn", "storage.database.auto_migrate": "database-auto-migrate",
		"logging.file.directory": "log-dir", "logging.file.filename": "log-filename", "logging.file.access.filename": "access-log-filename",
	}
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
	cfg := DefaultConfig()
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
	v.SetDefault("storage.database.type", defaults.Storage.Database.Type)
	v.SetDefault("storage.database.dsn", defaults.Storage.Database.DSN)
	v.SetDefault("storage.database.auto_migrate", defaults.Storage.Database.AutoMigrate)
	v.SetDefault("storage.database.max_open_conns", defaults.Storage.Database.MaxOpenConns)
	v.SetDefault("storage.database.max_idle_conns", defaults.Storage.Database.MaxIdleConns)
	v.SetDefault("storage.database.conn_max_lifetime", defaults.Storage.Database.ConnMaxLifetime)
	v.SetDefault("storage.database.conn_max_idle_time", defaults.Storage.Database.ConnMaxIdleTime)
	v.SetDefault("logging.file.directory", defaults.Logging.File.Directory)
	v.SetDefault("logging.file.filename", defaults.Logging.File.Filename)
	v.SetDefault("logging.file.max_size_mb", defaults.Logging.File.MaxSizeMB)
	v.SetDefault("logging.file.max_backups", defaults.Logging.File.MaxBackups)
	v.SetDefault("logging.file.max_age_days", defaults.Logging.File.MaxAgeDays)
	v.SetDefault("logging.file.compress", defaults.Logging.File.Compress)
	v.SetDefault("logging.file.access.filename", defaults.Logging.File.Access.Filename)
	v.SetDefault("security.master_key_file", defaults.Security.MasterKeyFile)
	v.SetDefault("security.allow_token_copy", defaults.Security.AllowTokenCopy)
	v.SetDefault("plugins.source_dir", defaults.Plugins.SourceDir)
	v.SetDefault("plugins.trusted_keys", defaults.Plugins.TrustedKeys)
}

func bindEnvironment(v *viper.Viper) error {
	v.SetEnvPrefix("MEERKIT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "__"))
	v.AutomaticEnv()
	envKeys := map[string]string{
		"server.address":                      "MEERKIT_SERVER__ADDRESS",
		"server.port":                         "MEERKIT_SERVER__PORT",
		"storage.data_dir":                    "MEERKIT_STORAGE__DATA_DIR",
		"storage.database.type":               "MEERKIT_STORAGE__DATABASE__TYPE",
		"storage.database.dsn":                "MEERKIT_STORAGE__DATABASE__DSN",
		"storage.database.auto_migrate":       "MEERKIT_STORAGE__DATABASE__AUTO_MIGRATE",
		"storage.database.max_open_conns":     "MEERKIT_STORAGE__DATABASE__MAX_OPEN_CONNS",
		"storage.database.max_idle_conns":     "MEERKIT_STORAGE__DATABASE__MAX_IDLE_CONNS",
		"storage.database.conn_max_lifetime":  "MEERKIT_STORAGE__DATABASE__CONN_MAX_LIFETIME",
		"storage.database.conn_max_idle_time": "MEERKIT_STORAGE__DATABASE__CONN_MAX_IDLE_TIME",
		"logging.file.directory":              "MEERKIT_LOGGING__FILE__DIRECTORY",
		"logging.file.filename":               "MEERKIT_LOGGING__FILE__FILENAME",
		"logging.file.max_size_mb":            "MEERKIT_LOGGING__FILE__MAX_SIZE_MB",
		"logging.file.max_backups":            "MEERKIT_LOGGING__FILE__MAX_BACKUPS",
		"logging.file.max_age_days":           "MEERKIT_LOGGING__FILE__MAX_AGE_DAYS",
		"logging.file.compress":               "MEERKIT_LOGGING__FILE__COMPRESS",
		"logging.file.access.filename":        "MEERKIT_LOGGING__FILE__ACCESS__FILENAME",
		"security.master_key_file":            "MEERKIT_SECURITY__MASTER_KEY_FILE",
		"security.allow_token_copy":           "MEERKIT_SECURITY__ALLOW_TOKEN_COPY",
		"plugins.source_dir":                  "MEERKIT_PLUGINS__SOURCE_DIR",
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
		"storage.data_dir":              "data-dir",
		"storage.database.type":         "database-type",
		"storage.database.dsn":          "database-dsn",
		"storage.database.auto_migrate": "database-auto-migrate",
		"logging.file.directory":        "log-dir",
		"logging.file.filename":         "log-filename",
		"logging.file.access.filename":  "access-log-filename",
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
	cfg.Storage.Database.Type = strings.ToLower(strings.TrimSpace(cfg.Storage.Database.Type))
	switch cfg.Storage.Database.Type {
	case "sqlite":
	case "mysql":
		if strings.TrimSpace(cfg.Storage.Database.DSN) == "" {
			return cfg, errors.New("storage.database.dsn cannot be empty for mysql")
		}
	default:
		return cfg, fmt.Errorf("storage.database.type must be sqlite or mysql, got %q", cfg.Storage.Database.Type)
	}
	if cfg.Storage.Database.MaxOpenConns < 0 || cfg.Storage.Database.MaxIdleConns < 0 {
		return cfg, errors.New("storage.database connection limits cannot be negative")
	}
	if cfg.Storage.Database.MaxOpenConns > 0 && cfg.Storage.Database.MaxIdleConns > cfg.Storage.Database.MaxOpenConns {
		return cfg, errors.New("storage.database.max_idle_conns cannot exceed max_open_conns")
	}
	if cfg.Storage.Database.Type == "mysql" && cfg.Storage.Database.MaxOpenConns == 1 {
		return cfg, errors.New("storage.database.max_open_conns must be at least 2 for mysql")
	}
	for name, value := range map[string]string{
		"storage.database.conn_max_lifetime":  cfg.Storage.Database.ConnMaxLifetime,
		"storage.database.conn_max_idle_time": cfg.Storage.Database.ConnMaxIdleTime,
	} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if duration, err := time.ParseDuration(value); err != nil || duration < 0 {
			return cfg, fmt.Errorf("%s must be a non-negative duration", name)
		}
	}
	if err := validateLogFile("logging.file", cfg.Logging.File); err != nil {
		return cfg, err
	}
	if strings.TrimSpace(cfg.Plugins.SourceDir) == "" {
		return cfg, errors.New("plugins.source_dir cannot be empty")
	}
	return cfg, nil
}

func validateLogFile(prefix string, file LogFileConfig) error {
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
	if err := validateLogFilename(prefix+".access", file.Access.Filename); err != nil {
		return err
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
