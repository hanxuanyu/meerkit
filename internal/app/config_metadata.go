package app

import (
	"os"

	"github.com/spf13/viper"
)

type ConfigMetadata struct {
	ConfigFile   string              `json:"config_file,omitempty"`
	Items        []ConfigItem        `json:"items"`
	RuntimeItems []RuntimeConfigItem `json:"runtime_items"`
}

type ConfigItem struct {
	Path        string `json:"path"`
	Description string `json:"description"`
	Value       any    `json:"value"`
	Default     any    `json:"default"`
	Source      string `json:"source"`
}

type RuntimeConfigItem struct {
	Type        string `json:"type"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Value       any    `json:"value"`
	Default     any    `json:"default"`
	Version     int    `json:"version"`
	IsDefault   bool   `json:"is_default"`
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
		value("server.address", "MEERKIT_SERVER__ADDRESS", "listen", "HTTP 服务监听地址，默认为 0.0.0.0。", func(c Config) any { return c.Server.Address }, func(c Config) any { return c.Server.Address }),
		value("server.port", "MEERKIT_SERVER__PORT", "listen", "HTTP 服务监听端口。", func(c Config) any { return c.Server.Port }, func(c Config) any { return c.Server.Port }),
		value("storage.data_dir", "MEERKIT_STORAGE__DATA_DIR", "data-dir", "运行数据目录；SQLite DSN 为空时数据库文件也保存在此目录。", func(c Config) any { return c.Storage.DataDir }, func(c Config) any { return c.Storage.DataDir }),
		value("storage.database.type", "MEERKIT_STORAGE__DATABASE__TYPE", "database-type", "数据库类型，可选 sqlite 或 mysql。", func(c Config) any { return c.Storage.Database.Type }, func(c Config) any { return c.Storage.Database.Type }),
		value("storage.database.dsn", "MEERKIT_STORAGE__DATABASE__DSN", "database-dsn", "数据库连接串；SQLite 留空时使用 data_dir 下的 meerkit.db。", func(c Config) any { return redactDSN(c.Storage.Database.DSN) }, func(c Config) any { return redactDSN(c.Storage.Database.DSN) }),
		value("storage.database.auto_migrate", "MEERKIT_STORAGE__DATABASE__AUTO_MIGRATE", "database-auto-migrate", "启动时是否自动执行数据库结构迁移。", func(c Config) any { return c.Storage.Database.AutoMigrate }, func(c Config) any { return c.Storage.Database.AutoMigrate }),
		value("logging.file.directory", "MEERKIT_LOGGING__FILE__DIRECTORY", "log-dir", "日志文件目录。", func(c Config) any { return c.Logging.File.Directory }, func(c Config) any { return c.Logging.File.Directory }),
		value("logging.file.filename", "MEERKIT_LOGGING__FILE__FILENAME", "log-filename", "业务日志文件名。", func(c Config) any { return c.Logging.File.Filename }, func(c Config) any { return c.Logging.File.Filename }),
		value("logging.file.max_size_mb", "MEERKIT_LOGGING__FILE__MAX_SIZE_MB", "", "单个日志文件达到该大小后轮转。", func(c Config) any { return c.Logging.File.MaxSizeMB }, func(c Config) any { return c.Logging.File.MaxSizeMB }),
		value("logging.file.max_backups", "MEERKIT_LOGGING__FILE__MAX_BACKUPS", "", "保留的旧日志文件数量。", func(c Config) any { return c.Logging.File.MaxBackups }, func(c Config) any { return c.Logging.File.MaxBackups }),
		value("logging.file.max_age_days", "MEERKIT_LOGGING__FILE__MAX_AGE_DAYS", "", "旧日志文件的最长保留天数。", func(c Config) any { return c.Logging.File.MaxAgeDays }, func(c Config) any { return c.Logging.File.MaxAgeDays }),
		value("logging.file.compress", "MEERKIT_LOGGING__FILE__COMPRESS", "", "轮转后的旧日志是否压缩。", func(c Config) any { return c.Logging.File.Compress }, func(c Config) any { return c.Logging.File.Compress }),
		value("logging.file.access.filename", "MEERKIT_LOGGING__FILE__ACCESS__FILENAME", "access-log-filename", "HTTP access 日志文件名。", func(c Config) any { return c.Logging.File.Access.Filename }, func(c Config) any { return c.Logging.File.Access.Filename }),
		value("security.master_key_file", "MEERKIT_SECURITY__MASTER_KEY_FILE", "", "主密钥文件路径。", func(c Config) any { return c.Security.MasterKeyFile }, func(c Config) any { return c.Security.MasterKeyFile }),
		value("plugins.source_dir", "MEERKIT_PLUGINS__SOURCE_DIR", "", "开发版宿主自动发现的源码插件根目录。", func(c Config) any { return c.Plugins.SourceDir }, func(c Config) any { return c.Plugins.SourceDir }),
		value("plugins.trusted_keys", "", "", "可信插件签名公钥。", func(c Config) any { return c.Plugins.TrustedKeys }, func(c Config) any { return c.Plugins.TrustedKeys }),
	}
}

func redactDSN(value string) string {
	if value == "" {
		return ""
	}
	return "configured"
}
