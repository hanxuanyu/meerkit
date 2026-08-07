package core

import "time"

const (
	StatusSourceConditionOverall = "condition_overall"
	StatusSourceConditionRule    = "condition_rule"
	StatusSourceResultField      = "result_field"

	StatusValueBoolean = "boolean"
	StatusValueNumber  = "number"
	StatusValueText    = "text"

	StatusLevelSuccess = "success"
	StatusLevelWarning = "warning"
	StatusLevelFailure = "failure"
	StatusLevelUnknown = "unknown"
)

type StatusItemSource struct {
	Kind          string               `json:"kind"`
	RuleID        string               `json:"rule_id,omitempty"`
	ResultSet     string               `json:"result_set,omitempty"`
	Field         string               `json:"field,omitempty"`
	Path          string               `json:"path,omitempty"`
	ValueType     string               `json:"value_type"`
	ValueMappings []StatusValueMapping `json:"value_mappings,omitempty"`
	DefaultLevel  string               `json:"default_level,omitempty"`
	DefaultLabel  string               `json:"default_label,omitempty"`
	DefaultColor  string               `json:"default_color,omitempty"`
}

type StatusValueMapping struct {
	Value string `json:"value"`
	Level string `json:"level"`
	Label string `json:"label"`
	Color string `json:"color,omitempty"`
}

type StatusThreshold struct {
	Maximum *float64 `json:"maximum,omitempty"`
	Level   string   `json:"level"`
	Label   string   `json:"label"`
	Color   string   `json:"color,omitempty"`
}

const (
	TrendConsecutive = "consecutive"
	TrendCount       = "count"
	TrendAverage     = "average"
	TrendDelta       = "delta"
	TrendSlope       = "slope"
)

type TrendRule struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Window    int     `json:"window"`
	Minimum   int     `json:"minimum,omitempty"`
	Operator  string  `json:"operator,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
	DeltaMode string  `json:"delta_mode,omitempty"`
}

type TrendRuleState struct {
	Active       bool   `json:"active"`
	LastRecordID string `json:"last_record_id,omitempty"`
}

type StatusItemRuntimeState struct {
	EvaluationStartedAt time.Time                 `json:"evaluation_started_at"`
	Rules               map[string]TrendRuleState `json:"rules"`
}

type StatusBoardItem struct {
	ID                     string                 `json:"id"`
	Name                   string                 `json:"name"`
	MonitorID              string                 `json:"monitor_id"`
	Enabled                bool                   `json:"enabled"`
	Source                 StatusItemSource       `json:"source"`
	Invert                 bool                   `json:"invert"`
	Thresholds             []StatusThreshold      `json:"thresholds"`
	HistoryLimit           int                    `json:"history_limit"`
	TrendRules             []TrendRule            `json:"trend_rules"`
	NotificationChannelIDs []string               `json:"notification_channel_ids"`
	RuntimeState           StatusItemRuntimeState `json:"runtime_state"`
	CreatedAt              time.Time              `json:"created_at"`
	UpdatedAt              time.Time              `json:"updated_at"`
}

type StatusSample struct {
	RecordID  string    `json:"record_id"`
	StartedAt time.Time `json:"started_at"`
	RawValue  any       `json:"raw_value,omitempty"`
	Display   string    `json:"display"`
	State     string    `json:"state"`
	Level     string    `json:"level"`
	Color     string    `json:"color,omitempty"`
	Label     string    `json:"label,omitempty"`
	Height    float64   `json:"height"`
	Numeric   *float64  `json:"numeric,omitempty"`
}

type StatusBoardItemView struct {
	StatusBoardItem
	SourceLabel string         `json:"source_label"`
	Samples     []StatusSample `json:"samples"`
	Current     *StatusSample  `json:"current,omitempty"`
}

type StatusBoardGroup struct {
	Monitor Monitor               `json:"monitor"`
	Items   []StatusBoardItemView `json:"items"`
}

type StatusBoardSnapshot struct {
	Groups []StatusBoardGroup `json:"groups"`
}
