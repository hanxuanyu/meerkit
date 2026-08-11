package store

import "github.com/uptrace/bun"

type schemaMigrationModel struct {
	bun.BaseModel `bun:"table:meerkit_schema_migrations"`
	Version       int64  `bun:"version,pk"`
	Name          string `bun:"name,type:varchar(160),notnull"`
	AppliedAt     string `bun:"applied_at,type:varchar(35),notnull"`
}

type monitorModel struct {
	bun.BaseModel              `bun:"table:monitors"`
	ID                         string `bun:"id,pk,type:varchar(64)"`
	Name                       string `bun:"name,type:varchar(255),notnull"`
	ModuleType                 string `bun:"module_type,type:varchar(160),notnull"`
	ModuleVersion              string `bun:"module_version,type:varchar(64),notnull"`
	ModuleConfigVersion        string `bun:"module_config_version,type:varchar(64),notnull"`
	SchedulesJSON              string `bun:"schedules_json,type:longtext,notnull"`
	Enabled                    bool   `bun:"enabled,notnull"`
	ModuleConfigJSON           string `bun:"module_config_json,type:longtext,notnull"`
	ConditionConfigJSON        string `bun:"condition_config_json,type:longtext,notnull"`
	NotificationChannelIDsJSON string `bun:"notification_channel_ids_json,type:longtext,notnull"`
	RuntimeStateJSON           string `bun:"runtime_state_json,type:longtext,notnull"`
	ConditionActive            bool   `bun:"condition_active,notnull"`
	LastSuccess                bool   `bun:"last_success,notnull"`
	CreatedAt                  string `bun:"created_at,type:varchar(35),notnull"`
	UpdatedAt                  string `bun:"updated_at,type:varchar(35),notnull"`
}

type monitorRecordModel struct {
	bun.BaseModel          `bun:"table:monitor_records"`
	ID                     string        `bun:"id,pk,type:varchar(64)"`
	MonitorID              string        `bun:"monitor_id,type:varchar(64),notnull"`
	ModuleType             string        `bun:"module_type,type:varchar(160),notnull"`
	ModuleVersion          string        `bun:"module_version,type:varchar(64),notnull"`
	StartedAt              int64         `bun:"started_at,type:bigint,notnull"`
	FinishedAt             int64         `bun:"finished_at,type:bigint,notnull"`
	Success                bool          `bun:"success,notnull"`
	DurationMS             int64         `bun:"duration_ms,notnull"`
	ResultSchemaVersion    string        `bun:"result_schema_version,type:varchar(64),notnull"`
	ResultJSON             string        `bun:"result_json,type:longtext,notnull"`
	ResultHash             string        `bun:"result_hash,type:varchar(128),notnull"`
	ConditionState         string        `bun:"condition_state,type:varchar(32),notnull"`
	EventType              string        `bun:"event_type,type:varchar(64),notnull"`
	NotificationEventsJSON string        `bun:"notification_events_json,type:longtext,notnull"`
	NotificationEventCount int           `bun:"notification_event_count,notnull"`
	TrendTriggered         bool          `bun:"trend_triggered,notnull"`
	TrendRecovered         bool          `bun:"trend_recovered,notnull"`
	ErrorCode              string        `bun:"error_code,type:varchar(160),notnull"`
	ErrorMessage           string        `bun:"error_message,type:text,notnull"`
	Monitor                *monitorModel `bun:"rel:belongs-to,join:monitor_id=id,on_delete:cascade"`
}

type statusBoardItemModel struct {
	bun.BaseModel              `bun:"table:status_board_items"`
	ID                         string        `bun:"id,pk,type:varchar(64)"`
	Name                       string        `bun:"name,type:varchar(255),notnull"`
	MonitorID                  string        `bun:"monitor_id,type:varchar(64),notnull"`
	Enabled                    bool          `bun:"enabled,notnull"`
	SourceJSON                 string        `bun:"source_json,type:longtext,notnull"`
	Invert                     bool          `bun:"invert,notnull"`
	ThresholdsJSON             string        `bun:"thresholds_json,type:longtext,notnull"`
	HistoryLimit               int           `bun:"history_limit,notnull"`
	TrendRulesJSON             string        `bun:"trend_rules_json,type:longtext,notnull"`
	NotificationChannelIDsJSON string        `bun:"notification_channel_ids_json,type:longtext,notnull"`
	RuntimeStateJSON           string        `bun:"runtime_state_json,type:longtext,notnull"`
	CreatedAt                  string        `bun:"created_at,type:varchar(35),notnull"`
	UpdatedAt                  string        `bun:"updated_at,type:varchar(35),notnull"`
	Monitor                    *monitorModel `bun:"rel:belongs-to,join:monitor_id=id,on_delete:cascade"`
}

type statusBoardShareModel struct {
	bun.BaseModel  `bun:"table:status_board_shares"`
	ID             string `bun:"id,pk,type:varchar(64)"`
	Name           string `bun:"name,type:varchar(255),notnull"`
	Token          string `bun:"token,type:varchar(128),notnull"`
	MonitorIDsJSON string `bun:"monitor_ids_json,type:longtext,notnull"`
	ItemIDsJSON    string `bun:"item_ids_json,type:longtext,notnull"`
	Active         bool   `bun:"active,notnull"`
	CreatedAt      string `bun:"created_at,type:varchar(35),notnull"`
}

type notificationChannelModel struct {
	bun.BaseModel `bun:"table:notification_channels"`
	ID            string  `bun:"id,pk,type:varchar(64)"`
	BuiltinKey    *string `bun:"builtin_key,type:varchar(64)"`
	Name          string  `bun:"name,type:varchar(255),notnull"`
	NotifierType  string  `bun:"notifier_type,type:varchar(128),notnull"`
	Enabled       bool    `bun:"enabled,notnull"`
	ConfigJSON    string  `bun:"config_json,type:longtext,notnull"`
	CreatedAt     string  `bun:"created_at,type:varchar(35),notnull"`
	UpdatedAt     string  `bun:"updated_at,type:varchar(35),notnull"`
}

type notificationDeliveryModel struct {
	bun.BaseModel `bun:"table:notification_deliveries"`
	ID            string `bun:"id,pk,type:varchar(64)"`
	EventID       string `bun:"event_id,type:varchar(64),notnull"`
	Source        string `bun:"source,type:varchar(64),notnull"`
	EventType     string `bun:"event_type,type:varchar(64),notnull"`
	StatusItemID  string `bun:"status_item_id,type:varchar(64),notnull"`
	TrendRuleID   string `bun:"trend_rule_id,type:varchar(64),notnull"`
	ChannelID     string `bun:"channel_id,type:varchar(64),notnull"`
	NotifierType  string `bun:"notifier_type,type:varchar(128),notnull"`
	MonitorID     string `bun:"monitor_id,type:varchar(64),notnull"`
	RecordID      string `bun:"record_id,type:varchar(64),notnull"`
	Title         string `bun:"title,type:varchar(500),notnull"`
	Content       string `bun:"content,type:longtext,notnull"`
	PayloadJSON   string `bun:"payload_json,type:longtext,notnull"`
	Status        string `bun:"status,type:varchar(32),notnull"`
	Attempts      int    `bun:"attempts,notnull"`
	Message       string `bun:"message,type:text,notnull"`
	IsRead        bool   `bun:"is_read,notnull"`
	CreatedAt     int64  `bun:"created_at,type:bigint,notnull"`
	UpdatedAt     int64  `bun:"updated_at,type:bigint,notnull"`
	DeliveredAt   *int64 `bun:"delivered_at,type:bigint"`
	ReadAt        *int64 `bun:"read_at,type:bigint"`
}

type pluginModel struct {
	bun.BaseModel     `bun:"table:plugins"`
	ID                string `bun:"id,pk,type:varchar(160)"`
	Version           string `bun:"version,pk,type:varchar(64)"`
	Name              string `bun:"name,type:varchar(255),notnull"`
	Vendor            string `bun:"vendor,type:varchar(255),notnull"`
	Description       string `bun:"desp,type:text,notnull"`
	URL               string `bun:"url,type:text,notnull"`
	Enabled           bool   `bun:"enabled,notnull"`
	Verified          bool   `bun:"verified,notnull"`
	Official          bool   `bun:"official,notnull"`
	TrustState        string `bun:"trust_state,type:varchar(32),notnull"`
	SignerKeyID       string `bun:"signer_key_id,type:varchar(160),notnull"`
	SignerFingerprint string `bun:"signer_fingerprint,type:varchar(255),notnull"`
	SignerPublicKey   string `bun:"signer_public_key,type:text,notnull"`
	Status            string `bun:"status,type:varchar(32),notnull"`
	Error             string `bun:"error,type:text,notnull"`
	PackagePath       string `bun:"package_path,type:text,notnull"`
	BinaryPath        string `bun:"binary_path,type:text,notnull"`
	PackageName       string `bun:"package_name,type:varchar(255),notnull"`
	PackageSHA256     string `bun:"package_sha256,type:varchar(128),notnull"`
	Readme            string `bun:"readme,type:longtext,notnull"`
	ManifestJSON      string `bun:"manifest_json,type:longtext,notnull"`
	ModulesJSON       string `bun:"modules_json,type:longtext,notnull"`
	CreatedAt         string `bun:"created_at,type:varchar(35),notnull"`
	UpdatedAt         string `bun:"updated_at,type:varchar(35),notnull"`
}

type trustedPluginSignerModel struct {
	bun.BaseModel `bun:"table:plugin_trusted_signers"`
	Fingerprint   string `bun:"fingerprint,pk,type:varchar(255)"`
	KeyID         string `bun:"key_id,type:varchar(160),notnull"`
	PublicKey     string `bun:"public_key,type:text,notnull"`
	Vendor        string `bun:"vendor,type:varchar(255),notnull"`
	Source        string `bun:"source,type:varchar(64),notnull"`
	CreatedAt     string `bun:"created_at,type:varchar(35),notnull"`
	UpdatedAt     string `bun:"updated_at,type:varchar(35),notnull"`
}

type moduleDescriptorSnapshotModel struct {
	bun.BaseModel  `bun:"table:module_descriptor_snapshots"`
	ModuleType     string `bun:"module_type,pk,type:varchar(160)"`
	ModuleVersion  string `bun:"module_version,pk,type:varchar(64)"`
	DescriptorJSON string `bun:"descriptor_json,type:longtext,notnull"`
	CreatedAt      string `bun:"created_at,type:varchar(35),notnull"`
}

type systemConfigModel struct {
	bun.BaseModel `bun:"table:system_configs"`
	ConfigType    string `bun:"config_type,pk,type:varchar(128)"`
	DataJSON      string `bun:"data_json,type:longtext,notnull"`
	Version       int    `bun:"version,notnull"`
	CreatedAt     string `bun:"created_at,type:varchar(35),notnull"`
	UpdatedAt     string `bun:"updated_at,type:varchar(35),notnull"`
}

type adminSessionModel struct {
	bun.BaseModel `bun:"table:admin_sessions"`
	TokenHash     string `bun:"token_hash,pk,type:varchar(128)"`
	CSRFToken     string `bun:"csrf_token,type:varchar(128),notnull"`
	ExpiresAt     string `bun:"expires_at,type:varchar(35),notnull"`
	LastSeenAt    string `bun:"last_seen_at,type:varchar(35),notnull"`
	CreatedAt     string `bun:"created_at,type:varchar(35),notnull"`
}
