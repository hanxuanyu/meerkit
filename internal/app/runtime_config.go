package app

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	SystemConfigStorage   = "storage"
	SystemConfigScheduler = "scheduler"
	SystemConfigLogging   = "logging"
	SystemConfigPlugins   = "plugins"
	SystemConfigAuth      = "auth"
	SystemConfigMCP       = "mcp"
)

type RuntimeConfig struct {
	Storage   RuntimeStorageConfig   `json:"storage"`
	Scheduler RuntimeSchedulerConfig `json:"scheduler"`
	Logging   RuntimeLoggingConfig   `json:"logging"`
	Plugins   RuntimePluginConfig    `json:"plugins"`
	Auth      RuntimeAuthConfig      `json:"auth"`
	MCP       RuntimeMCPConfig       `json:"mcp"`
}

type RuntimeStorageConfig struct {
	Retention             string `json:"retention"`
	NotificationRetention string `json:"notification_retention"`
	CleanupInterval       string `json:"cleanup_interval"`
}

type RuntimeSchedulerConfig struct {
	Timezone         string `json:"timezone"`
	MaxConcurrency   int    `json:"max_concurrency"`
	PollMilliseconds int    `json:"poll_milliseconds"`
}

type RuntimeLoggingConfig struct {
	Level     string            `json:"level"`
	Format    string            `json:"format"`
	AddSource bool              `json:"add_source"`
	Console   ConsoleLogConfig  `json:"console"`
	File      RuntimeFileConfig `json:"file"`
}

type ConsoleLogConfig struct {
	Enabled bool `json:"enabled"`
	Access  bool `json:"access"`
}

type RuntimeFileConfig struct {
	Enabled bool                `json:"enabled"`
	Access  RuntimeAccessConfig `json:"access"`
}

type RuntimeAccessConfig struct {
	Enabled bool `json:"enabled"`
}

type RuntimeAuthConfig struct {
	SessionTTL   string `json:"session_ttl"`
	AdminKeyHash string `json:"admin_key_hash,omitempty"`
}

type RuntimeMCPConfig struct {
	Enabled bool `json:"enabled"`
}

type RuntimePluginConfig struct {
	LogLevel  string `json:"log_level"`
	LogFormat string `json:"log_format"`
}

type RuntimeConfigDefinition struct {
	Type        string
	Path        string
	Description string
	Value       func(RuntimeConfig) any
	Default     func(RuntimeConfig) any
}

func RuntimeConfigDefinitions() []RuntimeConfigDefinition {
	value := func(configType, path, description string, getter func(RuntimeConfig) any) RuntimeConfigDefinition {
		return RuntimeConfigDefinition{Type: configType, Path: path, Description: description, Value: getter, Default: getter}
	}
	return []RuntimeConfigDefinition{
		value(SystemConfigStorage, "storage.retention", "监控执行记录的最长保留时间，支持 30d、12h 等持续时间格式。", func(c RuntimeConfig) any { return c.Storage.Retention }),
		value(SystemConfigStorage, "storage.notification_retention", "站内通知的最长保留时间。", func(c RuntimeConfig) any { return c.Storage.NotificationRetention }),
		value(SystemConfigStorage, "storage.cleanup_interval", "过期记录和通知清理任务的执行频率。", func(c RuntimeConfig) any { return c.Storage.CleanupInterval }),
		value(SystemConfigScheduler, "scheduler.timezone", "所有监控 cron 表达式使用的统一时区。", func(c RuntimeConfig) any { return c.Scheduler.Timezone }),
		value(SystemConfigScheduler, "scheduler.max_concurrency", "同时执行的监控任务上限。", func(c RuntimeConfig) any { return c.Scheduler.MaxConcurrency }),
		value(SystemConfigScheduler, "scheduler.poll_milliseconds", "调度器同步扫描间隔。", func(c RuntimeConfig) any { return c.Scheduler.PollMilliseconds }),
		value(SystemConfigLogging, "logging.level", "业务日志最低输出级别。", func(c RuntimeConfig) any { return c.Logging.Level }),
		value(SystemConfigLogging, "logging.format", "宿主日志格式，可选 text、simple 或 json。", func(c RuntimeConfig) any { return c.Logging.Format }),
		value(SystemConfigLogging, "logging.add_source", "业务日志是否包含源码位置。", func(c RuntimeConfig) any { return c.Logging.AddSource }),
		value(SystemConfigLogging, "logging.console.enabled", "是否将业务日志输出到标准输出。", func(c RuntimeConfig) any { return c.Logging.Console.Enabled }),
		value(SystemConfigLogging, "logging.console.access", "是否将 HTTP access 日志输出到标准输出。", func(c RuntimeConfig) any { return c.Logging.Console.Access }),
		value(SystemConfigLogging, "logging.file.enabled", "是否将业务日志写入轮转文件。", func(c RuntimeConfig) any { return c.Logging.File.Enabled }),
		value(SystemConfigLogging, "logging.file.access.enabled", "是否将 HTTP access 日志写入独立轮转文件。", func(c RuntimeConfig) any { return c.Logging.File.Access.Enabled }),
		value(SystemConfigPlugins, "plugins.log_level", "插件进程最低日志级别。", func(c RuntimeConfig) any { return c.Plugins.LogLevel }),
		value(SystemConfigPlugins, "plugins.log_format", "插件日志格式。", func(c RuntimeConfig) any { return c.Plugins.LogFormat }),
		value(SystemConfigAuth, "auth.session_ttl", "管理会话的有效期。", func(c RuntimeConfig) any { return c.Auth.SessionTTL }),
		value(SystemConfigMCP, "mcp.enabled", "是否启用 MCP 浏览器控制端点；首次启用时会自动创建 MCP Token。", func(c RuntimeConfig) any { return c.MCP.Enabled }),
	}
}

func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		Storage:   RuntimeStorageConfig{Retention: "30d", NotificationRetention: "30d", CleanupInterval: "1h"},
		Scheduler: RuntimeSchedulerConfig{Timezone: "Local", MaxConcurrency: 16, PollMilliseconds: 500},
		Logging: RuntimeLoggingConfig{
			Level: "info", Format: "simple", AddSource: true,
			Console: ConsoleLogConfig{Enabled: true, Access: false},
			File:    RuntimeFileConfig{Enabled: true, Access: RuntimeAccessConfig{Enabled: true}},
		},
		Plugins: RuntimePluginConfig{LogLevel: "info", LogFormat: "simple"},
		Auth:    RuntimeAuthConfig{SessionTTL: "720h"},
		MCP:     RuntimeMCPConfig{Enabled: false},
	}
}

func (c *RuntimeConfig) Validate() error {
	if _, err := ParseRetention(c.Storage.Retention); err != nil {
		return fmt.Errorf("storage.retention: %w", err)
	}
	if _, err := ParseRetention(c.Storage.NotificationRetention); err != nil {
		return fmt.Errorf("storage.notification_retention: %w", err)
	}
	if _, err := ParseRetention(c.Storage.CleanupInterval); err != nil {
		return fmt.Errorf("storage.cleanup_interval: %w", err)
	}
	if c.Scheduler.MaxConcurrency < 1 {
		return errors.New("scheduler.max_concurrency must be positive")
	}
	if c.Scheduler.PollMilliseconds < 100 {
		return errors.New("scheduler.poll_milliseconds must be at least 100")
	}
	if _, err := time.LoadLocation(c.Scheduler.Timezone); err != nil && c.Scheduler.Timezone != "Local" {
		return fmt.Errorf("scheduler.timezone: %w", err)
	}
	if _, err := ParseDuration(c.Auth.SessionTTL); err != nil {
		return fmt.Errorf("auth.session_ttl: %w", err)
	}
	var err error
	if c.Logging.Level, err = NormalizeLogLevel("logging.level", c.Logging.Level); err != nil {
		return err
	}
	if c.Logging.Format, err = NormalizeLogFormat("logging.format", c.Logging.Format); err != nil {
		return err
	}
	if c.Logging.Console.Enabled == false && c.Logging.File.Enabled == false {
		return errors.New("at least one logging output must be enabled")
	}
	if c.Plugins.LogLevel, err = NormalizeLogLevel("plugins.log_level", c.Plugins.LogLevel); err != nil {
		return err
	}
	if c.Plugins.LogFormat, err = NormalizeLogFormat("plugins.log_format", c.Plugins.LogFormat); err != nil {
		return err
	}
	return nil
}

func ParseDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.HasSuffix(value, "d") {
		days, err := time.ParseDuration(strings.TrimSuffix(value, "d") + "h")
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("invalid day duration %q", value)
		}
		return days * 24, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid duration %q", value)
	}
	return duration, nil
}

func ParseRetention(value string) (time.Duration, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.HasSuffix(value, "d") {
		var days float64
		if _, err := fmt.Sscanf(strings.TrimSuffix(value, "d"), "%f", &days); err != nil || days <= 0 {
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

func NormalizeLogLevel(path, value string) (string, error) {
	level := strings.ToLower(strings.TrimSpace(value))
	if level == "warning" {
		level = "warn"
	}
	if level != "debug" && level != "info" && level != "warn" && level != "error" {
		return "", fmt.Errorf("%s must be debug, info, warn, or error", path)
	}
	return level, nil
}

func NormalizeLogFormat(path, value string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(value))
	if format != "text" && format != "simple" && format != "json" {
		return "", fmt.Errorf("%s must be text, simple, or json", path)
	}
	return format, nil
}

func (c RuntimeConfig) StorageDurations() (time.Duration, time.Duration, time.Duration) {
	retention, _ := ParseRetention(c.Storage.Retention)
	notificationRetention, _ := ParseRetention(c.Storage.NotificationRetention)
	cleanupInterval, _ := ParseRetention(c.Storage.CleanupInterval)
	return retention, notificationRetention, cleanupInterval
}

func (c RuntimeConfig) SessionTTLDuration() time.Duration {
	duration, _ := ParseDuration(c.Auth.SessionTTL)
	return duration
}

func (c RuntimeConfig) SchedulerLocation() *time.Location {
	if c.Scheduler.Timezone == "Local" || c.Scheduler.Timezone == "" {
		return time.Local
	}
	location, err := time.LoadLocation(c.Scheduler.Timezone)
	if err != nil {
		return time.Local
	}
	return location
}
