package statusboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"meerkit/internal/core"
	"meerkit/internal/monitor"
	"meerkit/internal/store"
)

type PendingNotification struct {
	Event      core.NotificationEvent
	ChannelIDs []string
}

type ExecutionEvaluation struct {
	ItemStates map[string]core.StatusItemRuntimeState
	Events     []PendingNotification
	Stream     StreamEvent
}

type ConditionSource struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type ResultSource struct {
	ResultSet   string `json:"result_set"`
	ResultLabel string `json:"result_label"`
	Field       string `json:"field"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Unit        string `json:"unit,omitempty"`
	Path        bool   `json:"path,omitempty"`
}

type Sources struct {
	Conditions []ConditionSource `json:"conditions"`
	Results    []ResultSource    `json:"results"`
}

type Service struct {
	store   *store.Store
	modules *monitor.Registry
	hub     *Hub
}

func NewService(database *store.Store, modules *monitor.Registry, hub *Hub) *Service {
	return &Service{store: database, modules: modules, hub: hub}
}

func (s *Service) Hub() *Hub { return s.hub }

func (s *Service) Publish(event StreamEvent) {
	if s.hub != nil {
		s.hub.Publish(event)
	}
}

func (s *Service) Sources(ctx context.Context, monitorID string) (Sources, error) {
	value, err := s.store.GetMonitor(ctx, monitorID)
	if err != nil {
		return Sources{}, err
	}
	descriptor, err := s.descriptor(ctx, value)
	if err != nil {
		return Sources{}, err
	}
	result := Sources{Conditions: []ConditionSource{{ID: "", Label: "整体条件"}}, Results: []ResultSource{}}
	var config core.ConditionConfig
	_ = json.Unmarshal(value.ConditionConfig, &config)
	for index, rule := range config.Rules {
		result.Conditions = append(result.Conditions, ConditionSource{ID: rule.ID, Label: fmt.Sprintf("条件 %d · %s %s", index+1, rule.Field, rule.Operator)})
	}
	for _, set := range descriptor.ResultSets {
		for _, field := range set.Fields {
			result.Results = append(result.Results, ResultSource{ResultSet: set.Key, ResultLabel: set.Label, Field: field.Name, Label: field.Label, Type: field.Type, Unit: field.Unit, Path: field.Path})
		}
	}
	return result, nil
}

func (s *Service) NormalizeAndValidate(ctx context.Context, item *core.StatusBoardItem, resetRuntime bool) error {
	item.Name = strings.TrimSpace(item.Name)
	if item.Name == "" {
		return errors.New("看板项名称不能为空")
	}
	if item.HistoryLimit == 0 {
		item.HistoryLimit = 60
	}
	if item.HistoryLimit < 20 || item.HistoryLimit > 200 {
		return errors.New("展示执行次数必须在 20 到 200 之间")
	}
	monitorValue, err := s.store.GetMonitor(ctx, item.MonitorID)
	if err != nil {
		return errors.New("监控项不存在")
	}
	sources, err := s.Sources(ctx, item.MonitorID)
	if err != nil {
		return err
	}
	switch item.Source.Kind {
	case core.StatusSourceConditionOverall:
		item.Source.RuleID, item.Source.ResultSet, item.Source.Field, item.Source.Path, item.Source.ValueType = "", "", "", "", core.StatusValueBoolean
	case core.StatusSourceConditionRule:
		found := false
		for _, source := range sources.Conditions {
			found = found || (source.ID != "" && source.ID == item.Source.RuleID)
		}
		if !found {
			return errors.New("所选条件规则不存在")
		}
		item.Source.ResultSet, item.Source.Field, item.Source.Path, item.Source.ValueType = "", "", "", core.StatusValueBoolean
	case core.StatusSourceResultField:
		var selected *ResultSource
		for index := range sources.Results {
			candidate := &sources.Results[index]
			if candidate.ResultSet == item.Source.ResultSet && candidate.Field == item.Source.Field {
				selected = candidate
				break
			}
		}
		if selected == nil {
			return errors.New("所选结果字段不存在")
		}
		if selected.Path && strings.TrimSpace(item.Source.Path) == "" && !isPrimitiveType(selected.Type) {
			return errors.New("JSON 结果字段必须填写路径")
		}
		if !selected.Path && item.Source.Path != "" {
			return errors.New("所选字段不支持 JSON 路径")
		}
		if primitive := normalizeValueType(selected.Type); primitive != "" {
			item.Source.ValueType = primitive
		}
		if item.Source.ValueType == "" {
			item.Source.ValueType = s.inferValueType(ctx, *item)
		}
		if !validValueType(item.Source.ValueType) {
			return errors.New("结果字段显示类型必须为 boolean、number 或 text")
		}
	default:
		return errors.New("不支持的数据来源")
	}
	if item.Source.ValueType == core.StatusValueNumber {
		if len(item.Thresholds) == 0 {
			item.Thresholds = []core.StatusThreshold{{Level: core.StatusLevelSuccess, Label: "正常"}}
		}
		if err := validateThresholds(item.Thresholds); err != nil {
			return err
		}
	} else {
		item.Thresholds = []core.StatusThreshold{}
	}
	seenRules := map[string]bool{}
	for index := range item.TrendRules {
		rule := &item.TrendRules[index]
		if rule.ID == "" {
			rule.ID = core.NewID()
		}
		if seenRules[rule.ID] {
			return errors.New("趋势规则 ID 重复")
		}
		seenRules[rule.ID] = true
		if rule.Name == "" {
			rule.Name = fmt.Sprintf("趋势规则 %d", index+1)
		}
		if err := validateTrendRule(*rule, item.Source.ValueType); err != nil {
			return fmt.Errorf("%s: %w", rule.Name, err)
		}
	}
	for _, channelID := range item.NotificationChannelIDs {
		if _, err := s.store.GetChannel(ctx, channelID); err != nil {
			return fmt.Errorf("通知渠道 %s 不存在", channelID)
		}
	}
	_ = monitorValue
	if resetRuntime || item.RuntimeState.Rules == nil {
		item.RuntimeState = core.StatusItemRuntimeState{EvaluationStartedAt: time.Now().UTC(), Rules: map[string]core.TrendRuleState{}}
	}
	return nil
}

func (s *Service) inferValueType(ctx context.Context, item core.StatusBoardItem) string {
	records, err := s.store.ListRecords(ctx, item.MonitorID, 20)
	if err != nil {
		return ""
	}
	for _, record := range records {
		value, ok := extractValue(item.Source, record)
		if !ok {
			continue
		}
		switch value.(type) {
		case bool:
			return core.StatusValueBoolean
		case string:
			return core.StatusValueText
		default:
			if _, numberErr := core.NumberValue(value); numberErr == nil {
				return core.StatusValueNumber
			}
		}
	}
	return ""
}

func (s *Service) Snapshot(ctx context.Context) (core.StatusBoardSnapshot, error) {
	items, err := s.store.ListStatusBoardItems(ctx)
	if err != nil {
		return core.StatusBoardSnapshot{}, err
	}
	monitors, err := s.store.ListMonitors(ctx)
	if err != nil {
		return core.StatusBoardSnapshot{}, err
	}
	monitorMap := map[string]core.Monitor{}
	for _, value := range monitors {
		monitorMap[value.ID] = value
	}
	groups := map[string]*core.StatusBoardGroup{}
	for _, item := range items {
		monitorValue, ok := monitorMap[item.MonitorID]
		if !ok {
			continue
		}
		group := groups[item.MonitorID]
		if group == nil {
			group = &core.StatusBoardGroup{Monitor: monitorValue, Items: []core.StatusBoardItemView{}}
			groups[item.MonitorID] = group
		}
		records, loadErr := s.store.ListRecords(ctx, item.MonitorID, item.HistoryLimit)
		if loadErr != nil {
			return core.StatusBoardSnapshot{}, loadErr
		}
		reverseRecords(records)
		samples := BuildSamples(item, records)
		view := core.StatusBoardItemView{StatusBoardItem: item, SourceLabel: s.sourceLabel(ctx, item), Samples: samples}
		if len(samples) > 0 {
			view.Current = &samples[len(samples)-1]
		}
		group.Items = append(group.Items, view)
	}
	result := core.StatusBoardSnapshot{Groups: make([]core.StatusBoardGroup, 0, len(groups))}
	for _, group := range groups {
		result.Groups = append(result.Groups, *group)
	}
	sort.Slice(result.Groups, func(i, j int) bool { return result.Groups[i].Monitor.Name < result.Groups[j].Monitor.Name })
	return result, nil
}

func (s *Service) EvaluateExecution(ctx context.Context, monitorValue core.Monitor, record core.MonitorRecord) (ExecutionEvaluation, error) {
	items, err := s.store.ListStatusBoardItemsByMonitor(ctx, monitorValue.ID)
	if err != nil {
		return ExecutionEvaluation{}, err
	}
	result := ExecutionEvaluation{ItemStates: map[string]core.StatusItemRuntimeState{}, Events: []PendingNotification{}, Stream: StreamEvent{Type: "record_created", MonitorID: monitorValue.ID, RecordID: record.ID, Items: []StreamItem{}}}
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		maxWindow := 1
		for _, rule := range item.TrendRules {
			if rule.Window > maxWindow {
				maxWindow = rule.Window
			}
		}
		previous, loadErr := s.store.ListRecords(ctx, monitorValue.ID, maxWindow-1)
		if loadErr != nil {
			return ExecutionEvaluation{}, loadErr
		}
		reverseRecords(previous)
		previous = append(previous, record)
		samples := BuildSamples(item, previous)
		filtered := samples[:0]
		for _, sample := range samples {
			if !sample.StartedAt.Before(item.RuntimeState.EvaluationStartedAt) {
				filtered = append(filtered, sample)
			}
		}
		state := item.RuntimeState
		if state.Rules == nil {
			state.Rules = map[string]core.TrendRuleState{}
		}
		for _, rule := range item.TrendRules {
			evaluation, detail := EvaluateTrend(rule, filtered)
			previousState := state.Rules[rule.ID]
			eventType := ""
			if evaluation == "true" && !previousState.Active {
				previousState.Active = true
				eventType = "trend_triggered"
			} else if evaluation == "false" && previousState.Active {
				previousState.Active = false
				eventType = "trend_recovered"
			}
			previousState.LastRecordID = record.ID
			state.Rules[rule.ID] = previousState
			if eventType != "" {
				action := "已触发"
				if eventType == "trend_recovered" {
					action = "已恢复"
				}
				summary := fmt.Sprintf("状态看板“%s”的趋势规则“%s”%s", item.Name, rule.Name, action)
				result.Events = append(result.Events, PendingNotification{ChannelIDs: append([]string(nil), item.NotificationChannelIDs...), Event: core.NotificationEvent{
					ID: core.NewID(), Source: "status_trend", EventType: eventType, MonitorID: monitorValue.ID, MonitorName: monitorValue.Name, ModuleType: monitorValue.ModuleType,
					RecordID: record.ID, TriggeredAt: record.FinishedAt, Summary: summary, CurrentResult: record.Result, StatusItemID: item.ID, StatusItemName: item.Name,
					TrendRuleID: rule.ID, TrendRuleName: rule.Name, TrendDetail: detail,
				}})
			}
		}
		result.ItemStates[item.ID] = state
		currentSamples := BuildSamples(item, []core.MonitorRecord{record})
		current := currentSamples[0]
		result.Stream.Items = append(result.Stream.Items, StreamItem{ItemID: item.ID, Sample: current})
	}
	return result, nil
}

func (s *Service) descriptor(ctx context.Context, value core.Monitor) (core.ModuleDescriptor, error) {
	if descriptor, ok := s.modules.Descriptor(value.ModuleType); ok {
		return descriptor, nil
	}
	return s.store.GetDescriptorSnapshot(ctx, value.ModuleType, value.ModuleVersion)
}

func (s *Service) sourceLabel(ctx context.Context, item core.StatusBoardItem) string {
	sources, err := s.Sources(ctx, item.MonitorID)
	if err != nil {
		return item.Source.Field
	}
	if item.Source.Kind == core.StatusSourceConditionOverall {
		return "整体条件"
	}
	if item.Source.Kind == core.StatusSourceConditionRule {
		for _, source := range sources.Conditions {
			if source.ID == item.Source.RuleID {
				return source.Label
			}
		}
	}
	for _, source := range sources.Results {
		if source.ResultSet == item.Source.ResultSet && source.Field == item.Source.Field {
			label := source.ResultLabel + " · " + source.Label
			if item.Source.Path != "" {
				label += " · " + item.Source.Path
			}
			return label
		}
	}
	return item.Source.Field
}

func reverseRecords(records []core.MonitorRecord) {
	for left, right := 0, len(records)-1; left < right; left, right = left+1, right-1 {
		records[left], records[right] = records[right], records[left]
	}
}

func validateThresholds(values []core.StatusThreshold) error {
	var previous *float64
	for index, value := range values {
		if value.Level != core.StatusLevelSuccess && value.Level != core.StatusLevelWarning && value.Level != core.StatusLevelFailure {
			return errors.New("数值区间颜色必须为 success、warning 或 failure")
		}
		if strings.TrimSpace(value.Label) == "" {
			return errors.New("数值区间标签不能为空")
		}
		if value.Maximum == nil {
			if index != len(values)-1 {
				return errors.New("只有最后一个数值区间可以无上界")
			}
			continue
		}
		if index == len(values)-1 {
			return errors.New("最后一个数值区间必须无上界")
		}
		if previous != nil && *value.Maximum <= *previous {
			return errors.New("数值区间上界必须严格递增")
		}
		maximum := *value.Maximum
		previous = &maximum
	}
	return nil
}

func validateTrendRule(rule core.TrendRule, valueType string) error {
	if rule.Window < 1 || rule.Window > 200 {
		return errors.New("窗口必须在 1 到 200 之间")
	}
	switch rule.Type {
	case core.TrendConsecutive:
		return nil
	case core.TrendCount:
		if rule.Minimum < 1 || rule.Minimum > rule.Window {
			return errors.New("异常次数必须在 1 到窗口大小之间")
		}
		return nil
	case core.TrendAverage, core.TrendDelta, core.TrendSlope:
		if valueType != core.StatusValueNumber {
			return errors.New("该规则仅适用于数值来源")
		}
		if rule.Window < 2 {
			return errors.New("数值统计窗口至少为 2")
		}
		if rule.Operator != "gt" && rule.Operator != "gte" && rule.Operator != "lt" && rule.Operator != "lte" {
			return errors.New("比较操作符无效")
		}
		if rule.Type == core.TrendDelta && rule.DeltaMode != "absolute" && rule.DeltaMode != "percent" {
			return errors.New("变化模式必须为 absolute 或 percent")
		}
		return nil
	default:
		return errors.New("趋势规则类型无效")
	}
}

func normalizeValueType(value string) string {
	switch value {
	case "boolean":
		return core.StatusValueBoolean
	case "number", "integer":
		return core.StatusValueNumber
	case "string", "text":
		return core.StatusValueText
	default:
		return ""
	}
}

func validValueType(value string) bool {
	return value == core.StatusValueBoolean || value == core.StatusValueNumber || value == core.StatusValueText
}

func isPrimitiveType(value string) bool { return normalizeValueType(value) != "" }
