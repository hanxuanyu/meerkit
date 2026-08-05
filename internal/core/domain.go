package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type FieldDescriptor struct {
	Name      string   `json:"name"`
	Label     string   `json:"label"`
	Type      string   `json:"type"`
	Operators []string `json:"operators"`
	Path      bool     `json:"path,omitempty"`
}

type ResultFieldDescriptor struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
	Format      string   `json:"format,omitempty"`
	Unit        string   `json:"unit,omitempty"`
	Operators   []string `json:"operators"`
	Path        bool     `json:"path,omitempty"`
}

type ResultSetDescriptor struct {
	Key         string                  `json:"key"`
	Label       string                  `json:"label"`
	Description string                  `json:"description,omitempty"`
	Fields      []ResultFieldDescriptor `json:"fields"`
}

type ParameterType string

const (
	ParameterString   ParameterType = "string"
	ParameterText     ParameterType = "text"
	ParameterList     ParameterType = "list"
	ParameterMap      ParameterType = "map"
	ParameterBoolean  ParameterType = "boolean"
	ParameterInteger  ParameterType = "integer"
	ParameterNumber   ParameterType = "number"
	ParameterURL      ParameterType = "url"
	ParameterEmail    ParameterType = "email"
	ParameterJSON     ParameterType = "json"
	ParameterDate     ParameterType = "date"
	ParameterTime     ParameterType = "time"
	ParameterDateTime ParameterType = "datetime"
	ParameterDuration ParameterType = "duration"
)

type ParameterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type ParameterOptionSet struct {
	When    []ParameterCondition `json:"when,omitempty"`
	Options []ParameterOption    `json:"options"`
}

type ParameterCondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}

type ParameterDescriptor struct {
	Key         string               `json:"key"`
	Label       string               `json:"label"`
	Description string               `json:"description,omitempty"`
	Type        ParameterType        `json:"type"`
	Order       int                  `json:"order,omitempty"`
	FullWidth   bool                 `json:"full_width,omitempty"`
	Required    bool                 `json:"required,omitempty"`
	Default     any                  `json:"default,omitempty"`
	Placeholder string               `json:"placeholder,omitempty"`
	Secret      bool                 `json:"secret,omitempty"`
	Options     []ParameterOption    `json:"options,omitempty"`
	OptionsWhen []ParameterOptionSet `json:"options_when,omitempty"`
	VisibleWhen []ParameterCondition `json:"visible_when,omitempty"`
	EnabledWhen []ParameterCondition `json:"enabled_when,omitempty"`
	Minimum     *float64             `json:"minimum,omitempty"`
	Maximum     *float64             `json:"maximum,omitempty"`
	Step        *float64             `json:"step,omitempty"`
	Rows        int                  `json:"rows,omitempty"`
	Format      string               `json:"format,omitempty"`
	Unit        string               `json:"unit,omitempty"`
}

type ModuleDescriptor struct {
	Type         string                `json:"type"`
	Version      string                `json:"version"`
	Name         string                `json:"name"`
	Description  string                `json:"description"`
	ConfigSchema map[string]any        `json:"config_schema"`
	Parameters   []ParameterDescriptor `json:"parameters"`
	ResultSchema map[string]any        `json:"result_schema"`
	Fields       []FieldDescriptor     `json:"fields"`
	ResultSets   []ResultSetDescriptor `json:"result_sets,omitempty"`
}

type Observation struct {
	Success       bool                      `json:"success"`
	SchemaVersion string                    `json:"schema_version"`
	Result        map[string]any            `json:"result"`
	ResultSets    map[string]map[string]any `json:"result_sets,omitempty"`
	Summary       string                    `json:"summary"`
	ErrorCode     string                    `json:"error_code,omitempty"`
	ErrorMessage  string                    `json:"error_message,omitempty"`
}

type MonitorModule interface {
	Descriptor() ModuleDescriptor
	ValidateConfig(config json.RawMessage) error
	Execute(ctx context.Context, config json.RawMessage) (Observation, error)
}

type NotifierDescriptor struct {
	Type         string                `json:"type"`
	Name         string                `json:"name"`
	Description  string                `json:"description"`
	ConfigSchema map[string]any        `json:"config_schema"`
	Parameters   []ParameterDescriptor `json:"parameters"`
}

type NotificationEvent struct {
	EventType       string         `json:"event_type"`
	MonitorID       string         `json:"monitor_id"`
	MonitorName     string         `json:"monitor_name"`
	ModuleType      string         `json:"module_type"`
	TriggeredAt     time.Time      `json:"triggered_at"`
	ConditionState  string         `json:"condition_state"`
	Summary         string         `json:"summary"`
	PreviousResult  map[string]any `json:"previous_result,omitempty"`
	CurrentResult   map[string]any `json:"current_result,omitempty"`
	ConditionDetail []RuleResult   `json:"condition_detail,omitempty"`
}

type NotifierModule interface {
	Descriptor() NotifierDescriptor
	ValidateConfig(config json.RawMessage) error
	Send(ctx context.Context, config json.RawMessage, event NotificationEvent) error
}

type Monitor struct {
	ID                     string          `json:"id"`
	Name                   string          `json:"name"`
	ModuleType             string          `json:"module_type"`
	ModuleVersion          string          `json:"module_version"`
	Schedules              []string        `json:"schedules"`
	Enabled                bool            `json:"enabled"`
	ModuleConfig           json.RawMessage `json:"module_config"`
	ConditionConfig        json.RawMessage `json:"condition_config"`
	NotificationChannelIDs []string        `json:"notification_channel_ids"`
	RuntimeState           json.RawMessage `json:"runtime_state"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

type MonitorRecord struct {
	ID                  string         `json:"id"`
	MonitorID           string         `json:"monitor_id"`
	StartedAt           time.Time      `json:"started_at"`
	FinishedAt          time.Time      `json:"finished_at"`
	Success             bool           `json:"success"`
	DurationMS          int64          `json:"duration_ms"`
	ResultSchemaVersion string         `json:"result_schema_version"`
	Result              map[string]any `json:"result"`
	ResultHash          string         `json:"result_hash"`
	ConditionState      string         `json:"condition_state"`
	EventType           string         `json:"event_type"`
	NotificationResult  map[string]any `json:"notification_result,omitempty"`
	ErrorCode           string         `json:"error_code,omitempty"`
	ErrorMessage        string         `json:"error_message,omitempty"`
}

type NotificationChannel struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	NotifierType string          `json:"notifier_type"`
	Enabled      bool            `json:"enabled"`
	Config       json.RawMessage `json:"config"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type RuntimeState struct {
	ConditionActive bool      `json:"condition_active"`
	LastRecordID    string    `json:"last_record_id"`
	LastRunAt       time.Time `json:"last_run_at"`
	LastSuccess     bool      `json:"last_success"`
	LastSummary     string    `json:"last_summary"`
}

func NewID() string {
	return uuid.NewString()
}

func Float64(value float64) *float64 { return &value }

func JSONString(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func HashString(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func decodeJSON[T any](raw json.RawMessage, target *T) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, target)
}
