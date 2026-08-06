package app

import (
	"os"

	"github.com/spf13/viper"
)

type ConfigMetadata struct {
	ConfigFile string       `json:"config_file,omitempty"`
	Items      []ConfigItem `json:"items"`
}

type ConfigItem struct {
	Path        string `json:"path"`
	Description string `json:"description"`
	Value       any    `json:"value"`
	Default     any    `json:"default"`
	Source      string `json:"source"`
}

type configDefinition struct {
	path        string
	environment string
	flag        string
	description string
	value       func(Config) any
	defaultVal  func(Config) any
}

func buildConfigMetadata(v *viper.Viper, changedFlags map[string]bool, config Config) ConfigMetadata {
	defaults := DefaultConfig()
	items := make([]ConfigItem, 0, len(configDefinitions()))
	for _, definition := range configDefinitions() {
		item := ConfigItem{
			Path:        definition.path,
			Description: definition.description,
			Value:       definition.value(config),
			Default:     definition.defaultVal(defaults),
			Source:      "default",
		}
		if definition.flag != "" && changedFlags[definition.flag] {
			item.Source = "command_line"
		} else if definition.environment != "" {
			if _, ok := os.LookupEnv(definition.environment); ok {
				item.Source = "environment"
			}
		}
		if item.Source == "default" && v.InConfig(definition.path) {
			item.Source = "config_file"
		}
		items = append(items, item)
	}
	return ConfigMetadata{ConfigFile: v.ConfigFileUsed(), Items: items}
}

func configDefinitions() []configDefinition {
	value := func(path, env, flag, description string, getter func(Config) any, defaultGetter func(Config) any) configDefinition {
		return configDefinition{path: path, environment: env, flag: flag, description: description, value: getter, defaultVal: defaultGetter}
	}
	return []configDefinition{
		value("server.address", "MEERKIT_SERVER__ADDRESS", "listen", "HTTP 服务监听地址，默认为 0.0.0.0，即监听所有网络接口。使用 --listen 时会同时覆盖地址和端口。", func(c Config) any { return c.Server.Address }, func(c Config) any { return c.Server.Address }),
		value("server.port", "MEERKIT_SERVER__PORT", "listen", "HTTP 服务监听端口。使用 --listen 时会同时覆盖地址和端口。", func(c Config) any { return c.Server.Port }, func(c Config) any { return c.Server.Port }),
		value("storage.data_dir", "MEERKIT_STORAGE__DATA_DIR", "data-dir", "SQLite 数据库和运行数据所在目录。", func(c Config) any { return c.Storage.DataDir }, func(c Config) any { return c.Storage.DataDir }),
		value("storage.retention", "MEERKIT_STORAGE__RETENTION", "retention", "监控执行记录的最长保留时间，支持 30d、12h 等持续时间格式。", func(c Config) any { return c.Storage.Retention }, func(c Config) any { return c.Storage.Retention }),
		value("storage.notification_retention", "MEERKIT_STORAGE__NOTIFICATION_RETENTION", "", "站内通知的最长保留时间；到期后无论已读状态都会由清理任务删除。", func(c Config) any { return c.Storage.NotificationRetention }, func(c Config) any { return c.Storage.NotificationRetention }),
		value("storage.cleanup_interval", "MEERKIT_STORAGE__CLEANUP_INTERVAL", "", "过期通知和执行记录清理任务的执行频率，服务启动时也会立即清理一次。", func(c Config) any { return c.Storage.CleanupInterval }, func(c Config) any { return c.Storage.CleanupInterval }),
		value("scheduler.timezone", "MEERKIT_SCHEDULER__TIMEZONE", "timezone", "所有监控 cron 表达式使用的统一时区。", func(c Config) any { return c.Scheduler.Timezone }, func(c Config) any { return c.Scheduler.Timezone }),
		value("scheduler.max_concurrency", "MEERKIT_SCHEDULER__MAX_CONCURRENCY", "max-concurrency", "同时执行的监控任务上限。", func(c Config) any { return c.Scheduler.MaxConcurrency }, func(c Config) any { return c.Scheduler.MaxConcurrency }),
		value("scheduler.poll_milliseconds", "MEERKIT_SCHEDULER__POLL_MILLISECONDS", "", "调度器同步扫描间隔，用于发现配置变化并检查 cron 是否到期，不决定监控执行频率。", func(c Config) any { return c.Scheduler.PollMilliseconds }, func(c Config) any { return c.Scheduler.PollMilliseconds }),
		value("logging.level", "MEERKIT_LOGGING__LEVEL", "log-level", "业务日志最低输出级别。", func(c Config) any { return c.Logging.Level }, func(c Config) any { return c.Logging.Level }),
		value("logging.format", "MEERKIT_LOGGING__FORMAT", "log-format", "日志格式，可选 text 或 json。", func(c Config) any { return c.Logging.Format }, func(c Config) any { return c.Logging.Format }),
		value("logging.add_source", "MEERKIT_LOGGING__ADD_SOURCE", "", "业务日志是否包含源码位置。", func(c Config) any { return c.Logging.AddSource }, func(c Config) any { return c.Logging.AddSource }),
		value("logging.console.enabled", "MEERKIT_LOGGING__CONSOLE__ENABLED", "log-console", "是否将业务日志输出到标准输出。", func(c Config) any { return c.Logging.Console.Enabled }, func(c Config) any { return c.Logging.Console.Enabled }),
		value("logging.console.access", "MEERKIT_LOGGING__CONSOLE__ACCESS", "log-console-access", "是否将 HTTP access 日志输出到标准输出。", func(c Config) any { return c.Logging.Console.Access }, func(c Config) any { return c.Logging.Console.Access }),
		value("logging.file.enabled", "MEERKIT_LOGGING__FILE__ENABLED", "log-file-enabled", "是否将业务日志写入轮转文件。", func(c Config) any { return c.Logging.File.Enabled }, func(c Config) any { return c.Logging.File.Enabled }),
		value("logging.file.directory", "MEERKIT_LOGGING__FILE__DIRECTORY", "log-dir", "日志文件目录，业务日志和 access 日志共用。", func(c Config) any { return c.Logging.File.Directory }, func(c Config) any { return c.Logging.File.Directory }),
		value("logging.file.filename", "MEERKIT_LOGGING__FILE__FILENAME", "log-filename", "业务日志文件名。", func(c Config) any { return c.Logging.File.Filename }, func(c Config) any { return c.Logging.File.Filename }),
		value("logging.file.max_size_mb", "MEERKIT_LOGGING__FILE__MAX_SIZE_MB", "", "单个日志文件达到该大小后轮转。", func(c Config) any { return c.Logging.File.MaxSizeMB }, func(c Config) any { return c.Logging.File.MaxSizeMB }),
		value("logging.file.max_backups", "MEERKIT_LOGGING__FILE__MAX_BACKUPS", "", "保留的旧日志文件数量。", func(c Config) any { return c.Logging.File.MaxBackups }, func(c Config) any { return c.Logging.File.MaxBackups }),
		value("logging.file.max_age_days", "MEERKIT_LOGGING__FILE__MAX_AGE_DAYS", "", "旧日志文件的最长保留天数。", func(c Config) any { return c.Logging.File.MaxAgeDays }, func(c Config) any { return c.Logging.File.MaxAgeDays }),
		value("logging.file.compress", "MEERKIT_LOGGING__FILE__COMPRESS", "", "轮转后的旧日志是否压缩。", func(c Config) any { return c.Logging.File.Compress }, func(c Config) any { return c.Logging.File.Compress }),
		value("logging.file.access.enabled", "MEERKIT_LOGGING__FILE__ACCESS__ENABLED", "access-log-file-enabled", "是否将 HTTP access 日志写入独立轮转文件。", func(c Config) any { return c.Logging.File.Access.Enabled }, func(c Config) any { return c.Logging.File.Access.Enabled }),
		value("logging.file.access.filename", "MEERKIT_LOGGING__FILE__ACCESS__FILENAME", "access-log-filename", "HTTP access 日志文件名。", func(c Config) any { return c.Logging.File.Access.Filename }, func(c Config) any { return c.Logging.File.Access.Filename }),
		value("security.session_ttl", "MEERKIT_SECURITY__SESSION_TTL", "", "管理会话的有效期。", func(c Config) any { return c.Security.SessionTTL }, func(c Config) any { return c.Security.SessionTTL }),
		value("security.master_key_file", "MEERKIT_SECURITY__MASTER_KEY_FILE", "", "主密钥文件路径。", func(c Config) any { return c.Security.MasterKeyFile }, func(c Config) any { return c.Security.MasterKeyFile }),
		value("plugins.trusted_keys", "", "", "可信插件签名公钥。键为签名 key ID，值为 Base64 编码的 Ed25519 公钥；修改后需重启服务。", func(c Config) any { return c.Plugins.TrustedKeys }, func(c Config) any { return c.Plugins.TrustedKeys }),
	}
}
