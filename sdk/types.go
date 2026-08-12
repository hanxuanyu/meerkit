package sdk

import (
	"context"
	"encoding/json"
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
	Scope       string                  `json:"scope,omitempty"`
	Fields      []ResultFieldDescriptor `json:"fields"`
}

type ParameterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
type ParameterCondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}
type ParameterOptionSet struct {
	When    []ParameterCondition `json:"when,omitempty"`
	Options []ParameterOption    `json:"options"`
}
type ParameterType string

const (
	ParameterString        ParameterType = "string"
	ParameterText          ParameterType = "text"
	ParameterList          ParameterType = "list"
	ParameterMap           ParameterType = "map"
	ParameterBoolean       ParameterType = "boolean"
	ParameterInteger       ParameterType = "integer"
	ParameterNumber        ParameterType = "number"
	ParameterURL           ParameterType = "url"
	ParameterJSON          ParameterType = "json"
	ParameterDuration      ParameterType = "duration"
	ParameterBrowserWindow ParameterType = "browser_window"
	ParameterBrowserTab    ParameterType = "browser_tab"
)

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

type ModuleListSummaryDescriptor struct {
	Fields    []string `json:"fields"`
	Separator string   `json:"separator,omitempty"`
}
type ModuleDescriptor struct {
	Type                string                       `json:"type"`
	Version             string                       `json:"version"`
	ConfigVersion       string                       `json:"config_version,omitempty"`
	ResultSchemaVersion string                       `json:"result_schema_version,omitempty"`
	Name                string                       `json:"name"`
	Description         string                       `json:"description"`
	ListSummary         *ModuleListSummaryDescriptor `json:"list_summary,omitempty"`
	ConfigSchema        map[string]any               `json:"config_schema"`
	Parameters          []ParameterDescriptor        `json:"parameters"`
	ResultSchema        map[string]any               `json:"result_schema"`
	Fields              []FieldDescriptor            `json:"fields"`
	ResultSets          []ResultSetDescriptor        `json:"result_sets,omitempty"`
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

type Provider interface {
	ListModules() ([]ModuleDescriptor, error)
	ValidateConfig(ctx context.Context, moduleType string, config json.RawMessage) error
	Execute(ctx context.Context, moduleType string, config json.RawMessage) (Observation, error)
	MigrateConfig(ctx context.Context, moduleType, fromVersion, toVersion string, config json.RawMessage) (json.RawMessage, error)
	Health(ctx context.Context) error
}
