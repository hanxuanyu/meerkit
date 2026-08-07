package runtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"meerkit/internal/app"
	"meerkit/internal/core"
	"meerkit/internal/monitor"
	"meerkit/internal/notification"
	"meerkit/internal/store"
)

var (
	ErrMonitorRunning           = errors.New("monitor is already running")
	ErrMonitorModuleUnavailable = errors.New("monitor module is unavailable")
)

type Runner struct {
	store     *store.Store
	modules   *monitor.Registry
	notifiers *notification.Registry
	locksMu   sync.Mutex
	locks     map[string]*sync.Mutex
	logger    *slog.Logger
}

func NewRunner(store *store.Store, modules *monitor.Registry, notifiers *notification.Registry, logger *slog.Logger) *Runner {
	return &Runner{store: store, modules: modules, notifiers: notifiers, locks: make(map[string]*sync.Mutex), logger: logger}
}

func (r *Runner) lockFor(id string) *sync.Mutex {
	r.locksMu.Lock()
	defer r.locksMu.Unlock()
	lock := r.locks[id]
	if lock == nil {
		lock = &sync.Mutex{}
		r.locks[id] = lock
	}
	return lock
}

func (r *Runner) Run(ctx context.Context, id string) (core.MonitorRecord, error) {
	lock := r.lockFor(id)
	lock.Lock()
	defer lock.Unlock()
	return r.runLocked(ctx, id)
}

func (r *Runner) TryRun(ctx context.Context, id string) (core.MonitorRecord, error) {
	lock := r.lockFor(id)
	if !lock.TryLock() {
		return core.MonitorRecord{}, ErrMonitorRunning
	}
	defer lock.Unlock()
	return r.runLocked(ctx, id)
}

func (r *Runner) ModuleAvailable(moduleType, moduleVersion string) bool {
	module, ok := r.modules.Get(moduleType)
	if !ok {
		return false
	}
	return moduleVersion == "" || module.Descriptor().Version == moduleVersion
}

func (r *Runner) runLocked(ctx context.Context, id string) (core.MonitorRecord, error) {
	monitor, err := r.store.GetMonitor(ctx, id)
	if err != nil {
		return core.MonitorRecord{}, err
	}
	module, ok := r.modules.Get(monitor.ModuleType)
	if !ok {
		return core.MonitorRecord{}, fmt.Errorf("%w: %s", ErrMonitorModuleUnavailable, monitor.ModuleType)
	}
	descriptor := module.Descriptor()
	if monitor.ModuleVersion != "" && descriptor.Version != monitor.ModuleVersion {
		return core.MonitorRecord{}, fmt.Errorf("%w: %s version %s is required, version %s is active", ErrMonitorModuleUnavailable, monitor.ModuleType, monitor.ModuleVersion, descriptor.Version)
	}
	if r.logger != nil {
		r.logger.Debug("monitor execution started", "monitor_id", monitor.ID, "monitor_name", monitor.Name, "module_type", monitor.ModuleType, "module_version", descriptor.Version)
	}
	started := time.Now().UTC()
	previous, previousErr := r.store.LatestSuccessfulRecord(ctx, id)
	if previousErr != nil && !errors.Is(previousErr, sql.ErrNoRows) {
		return core.MonitorRecord{}, previousErr
	}
	observation, executeErr := module.Execute(ctx, monitor.ModuleConfig)
	if observation.Result == nil {
		observation.Result = map[string]any{}
	}
	// Result sets remain protocol-owned while the persisted result exposes each set
	// under its key, allowing the shared condition engine to address set.field.
	for key, values := range observation.ResultSets {
		setValues := make(map[string]any, len(values))
		for field, value := range values {
			setValues[field] = value
		}
		observation.Result[key] = setValues
	}
	duration := time.Since(started)
	observation.Result["success"] = observation.Success && executeErr == nil
	observation.Result["duration_ms"] = duration.Milliseconds()
	if executeErr != nil {
		observation.Success = false
		if observation.ErrorCode == "" {
			observation.ErrorCode = "execution_error"
		}
		if observation.ErrorMessage == "" {
			observation.ErrorMessage = executeErr.Error()
		}
		observation.Result["error"] = observation.ErrorMessage
	}
	current := observation.Result
	var conditionConfig core.ConditionConfig
	if err := json.Unmarshal(monitor.ConditionConfig, &conditionConfig); err != nil {
		observation.ErrorCode = "invalid_condition_config"
		observation.ErrorMessage = err.Error()
	}
	var previousResult map[string]any
	if previousErr == nil {
		previousResult = previous.Result
	}
	notificationPolicy := core.NormalizeNotificationPolicy(conditionConfig.NotificationPolicy)
	executionResult := map[string]any{
		"success":       observation.Success && executeErr == nil,
		"duration_ms":   duration.Milliseconds(),
		"error_code":    observation.ErrorCode,
		"error_message": observation.ErrorMessage,
		"summary":       observation.Summary,
	}
	current["summary"] = executionResult
	evaluation := core.EvaluateConditions(conditionConfig, current, previousResult)
	logic := strings.ToUpper(conditionConfig.Logic)
	if logic != "ANY" {
		logic = "ALL"
	}
	matchedCount := 0
	for _, detail := range evaluation.Details {
		if detail.State == "true" {
			matchedCount++
		}
	}
	state := core.RuntimeState{}
	_ = json.Unmarshal(monitor.RuntimeState, &state)
	previousActive := state.ConditionActive
	eventType := "none"
	active := previousActive
	if evaluation.State == "true" {
		active = true
		if notificationPolicy == core.NotificationPolicyEvery || !previousActive {
			eventType = "triggered"
		}
	} else if evaluation.State == "false" {
		active = false
		if previousActive {
			eventType = "recovered"
		}
	}
	executionSummary := composeExecutionSummary(descriptor, observation.Summary, observation.Success && executeErr == nil, duration.Milliseconds(), observation.ErrorCode, observation.ErrorMessage, eventType, evaluation, logic, matchedCount)
	observation.Summary = executionSummary
	executionResult["summary"] = executionSummary
	executionResult["triggered"] = evaluation.State == "true"
	executionResult["condition_state"] = evaluation.State
	executionResult["event_type"] = eventType
	executionResult["condition_logic"] = logic
	executionResult["matched_count"] = matchedCount
	executionResult["condition_details"] = evaluation.Details
	if observation.ResultSets == nil {
		observation.ResultSets = make(map[string]map[string]any)
	}
	observation.ResultSets["summary"] = executionResult
	current["summary"] = executionResult
	resultHash := core.HashString(core.JSONString(current))
	finished := time.Now().UTC()
	record := core.MonitorRecord{
		ID: core.NewID(), MonitorID: monitor.ID, StartedAt: started, FinishedAt: finished,
		ModuleType: monitor.ModuleType, ModuleVersion: descriptor.Version,
		Success: observation.Success && executeErr == nil, DurationMS: duration.Milliseconds(), ResultSchemaVersion: observation.SchemaVersion,
		Result: current, ResultHash: resultHash, ConditionState: evaluation.State, EventType: eventType,
		NotificationResult: map[string]any{}, ErrorCode: observation.ErrorCode, ErrorMessage: observation.ErrorMessage,
	}
	if record.ResultSchemaVersion == "" {
		record.ResultSchemaVersion = descriptor.Version
	}
	if err := r.store.AddRecord(ctx, record); err != nil {
		return record, err
	}
	state.ConditionActive = active
	state.LastRecordID = record.ID
	state.LastRunAt = finished
	state.LastSuccess = record.Success
	state.LastSummary = executionSummary
	if err := r.store.UpdateRuntimeState(ctx, monitor.ID, state); err != nil {
		return record, err
	}
	if r.logger != nil {
		r.logger.Info("monitor execution completed", "monitor_id", monitor.ID, "monitor_name", monitor.Name, "module_type", monitor.ModuleType, "success", record.Success, "duration_ms", record.DurationMS, "condition_state", record.ConditionState, "event_type", record.EventType, "summary", executionSummary, "error_code", record.ErrorCode)
	}
	if eventType != "none" {
		event := core.NotificationEvent{EventType: eventType, MonitorID: monitor.ID, RecordID: record.ID, MonitorName: monitor.Name, ModuleType: monitor.ModuleType, TriggeredAt: finished, ConditionState: evaluation.State, Summary: executionSummary, CurrentResult: current, ConditionDetail: evaluation.Details}
		if previousErr == nil {
			event.PreviousResult = previous.Result
		}
		channelIDs := append([]string(nil), monitor.NotificationChannelIDs...)
		if len(channelIDs) > 0 {
			if r.logger != nil {
				r.logger.Info("monitor condition event queued", "monitor_id", monitor.ID, "record_id", record.ID, "event_type", eventType, "channel_count", len(channelIDs))
			}
			go r.sendNotifications(context.Background(), record.ID, channelIDs, event)
		} else if r.logger != nil {
			r.logger.Info("monitor condition event has no notification channels", "monitor_id", monitor.ID, "record_id", record.ID, "event_type", eventType)
		}
	}
	return record, nil
}

func (r *Runner) sendNotifications(ctx context.Context, recordID string, channelIDs []string, event core.NotificationEvent) {
	results := make(map[string]any, len(channelIDs))
	if r.logger != nil {
		r.logger.Debug("notification delivery started", "record_id", recordID, "event_type", event.EventType, "channel_count", len(channelIDs))
	}
	for _, channelID := range channelIDs {
		channel, err := r.store.GetChannel(ctx, channelID)
		if err != nil {
			results[channelID] = map[string]any{"status": "error", "message": err.Error()}
			if r.logger != nil {
				r.logger.Error("load notification channel failed", "record_id", recordID, "channel_id", channelID, "error", err)
			}
			continue
		}
		if !channel.Enabled {
			results[channelID] = map[string]any{"status": "skipped", "message": "channel disabled"}
			if r.logger != nil {
				r.logger.Info("notification channel skipped", "record_id", recordID, "channel_id", channelID, "reason", "disabled")
			}
			continue
		}
		notifier, ok := r.notifiers.Get(channel.NotifierType)
		if !ok {
			results[channelID] = map[string]any{"status": "error", "message": "unknown notifier type"}
			if r.logger != nil {
				r.logger.Error("notification notifier not found", "record_id", recordID, "channel_id", channelID, "notifier_type", channel.NotifierType)
			}
			continue
		}
		var sendErr error
		for attempt := 1; attempt <= 3; attempt++ {
			if r.logger != nil {
				r.logger.Debug("notification delivery attempt", "record_id", recordID, "channel_id", channelID, "notifier_type", channel.NotifierType, "attempt", attempt)
			}
			sendCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			sendErr = notifier.Send(sendCtx, channel.Config, event)
			cancel()
			if sendErr == nil {
				break
			}
			if attempt < 3 {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			}
		}
		if sendErr != nil {
			results[channelID] = map[string]any{"status": "error", "message": sendErr.Error(), "attempts": 3}
			if r.logger != nil {
				r.logger.Error("notification delivery failed", "record_id", recordID, "channel_id", channelID, "notifier_type", channel.NotifierType, "attempts", 3, "error", sendErr)
			}
		} else {
			results[channelID] = map[string]any{"status": "sent", "attempts": 1}
			if r.logger != nil {
				r.logger.Info("notification delivered", "record_id", recordID, "channel_id", channelID, "notifier_type", channel.NotifierType, "event_type", event.EventType)
			}
		}
	}
	if r.logger != nil {
		r.logger.Debug("notification delivery completed", "record_id", recordID, "event_type", event.EventType)
	}
	if err := r.store.UpdateRecordNotifications(ctx, recordID, results); err != nil && r.logger != nil {
		r.logger.Error("update notification result failed", "record_id", recordID, "error", err)
	}
}

type scheduleTask struct {
	expressions []string
	next        time.Time
}

type Scheduler struct {
	runner           *Runner
	store            *store.Store
	config           func() app.RuntimeConfig
	logger           *slog.Logger
	changes          <-chan struct{}
	tasksMu          sync.Mutex
	tasks            map[string]*scheduleTask
	invalidSchedules map[string]string
	pausedMonitors   map[string]string
	activeMu         sync.Mutex
	activeRuns       int
	lastTimezone     string
}

func NewScheduler(runner *Runner, store *store.Store, config func() app.RuntimeConfig, logger *slog.Logger, changes ...<-chan struct{}) *Scheduler {
	current := config()
	var changeChannel <-chan struct{}
	if len(changes) > 0 {
		changeChannel = changes[0]
	}
	return &Scheduler{runner: runner, store: store, config: config, logger: logger, changes: changeChannel, tasks: make(map[string]*scheduleTask), invalidSchedules: make(map[string]string), pausedMonitors: make(map[string]string), lastTimezone: current.Scheduler.Timezone}
}

func (s *Scheduler) Start(ctx context.Context) {
	current := s.config()
	if s.logger != nil {
		s.logger.Info("scheduler started", "poll_ms", current.Scheduler.PollMilliseconds, "max_concurrency", current.Scheduler.MaxConcurrency)
	}
	timer := time.NewTimer(time.Duration(current.Scheduler.PollMilliseconds) * time.Millisecond)
	defer timer.Stop()
	defer func() {
		if s.logger != nil {
			s.logger.Info("scheduler stopped")
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.changes:
			current = s.config()
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			s.syncAndRun(ctx, time.Now())
			interval := time.Duration(current.Scheduler.PollMilliseconds) * time.Millisecond
			if interval <= 0 {
				interval = 500 * time.Millisecond
			}
			timer.Reset(interval)
		case now := <-timer.C:
			s.syncAndRun(ctx, now)
			current = s.config()
			timer.Reset(time.Duration(current.Scheduler.PollMilliseconds) * time.Millisecond)
		}
	}
}

func (s *Scheduler) syncAndRun(ctx context.Context, now time.Time) {
	current := s.config()
	timezone := current.Scheduler.Timezone
	monitors, err := s.store.ListMonitors(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("load monitors failed", "error", err)
		}
		return
	}
	if s.logger != nil {
		s.logger.Debug("scheduler synchronization completed", "monitor_count", len(monitors))
	}
	present := make(map[string]bool, len(monitors))
	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()
	if s.lastTimezone != timezone {
		s.tasks = make(map[string]*scheduleTask)
		s.invalidSchedules = make(map[string]string)
		s.lastTimezone = timezone
	}
	for _, monitor := range monitors {
		if !monitor.Enabled {
			delete(s.pausedMonitors, monitor.ID)
			continue
		}
		if !s.runner.ModuleAvailable(monitor.ModuleType, monitor.ModuleVersion) {
			reason := monitor.ModuleType + "@" + monitor.ModuleVersion
			if s.pausedMonitors[monitor.ID] != reason && s.logger != nil {
				s.logger.Warn("scheduled monitor paused because module is unavailable", "monitor_id", monitor.ID, "monitor_name", monitor.Name, "module_type", monitor.ModuleType, "module_version", monitor.ModuleVersion)
			}
			s.pausedMonitors[monitor.ID] = reason
			continue
		}
		if _, paused := s.pausedMonitors[monitor.ID]; paused {
			delete(s.pausedMonitors, monitor.ID)
			if s.logger != nil {
				s.logger.Info("scheduled monitor resumed after module became available", "monitor_id", monitor.ID, "monitor_name", monitor.Name, "module_type", monitor.ModuleType, "module_version", monitor.ModuleVersion)
			}
		}
		present[monitor.ID] = true
		task := s.tasks[monitor.ID]
		signature := strings.Join(monitor.Schedules, "\x00")
		if task == nil || strings.Join(task.expressions, "\x00") != signature {
			next, scheduleErr := nextScheduleTime(monitor.Schedules, timezone, now)
			if scheduleErr != nil {
				if s.invalidSchedules[monitor.ID] != signature && s.logger != nil {
					s.logger.Warn("monitor schedule is invalid", "monitor_id", monitor.ID, "schedules", monitor.Schedules, "timezone", timezone, "error", scheduleErr)
				}
				s.invalidSchedules[monitor.ID] = signature
				continue
			}
			delete(s.invalidSchedules, monitor.ID)
			task = &scheduleTask{expressions: append([]string(nil), monitor.Schedules...), next: next}
			s.tasks[monitor.ID] = task
		}
		due, scheduleErr := advanceScheduleTask(task, timezone, now)
		if scheduleErr != nil {
			if s.logger != nil {
				s.logger.Warn("monitor schedule became invalid", "monitor_id", monitor.ID, "schedules", task.expressions, "error", scheduleErr)
			}
		} else if due {
			if s.logger != nil {
				s.logger.Debug("scheduled monitor queued", "monitor_id", monitor.ID, "next_run", task.next, "schedules", task.expressions)
			}
			s.launch(ctx, monitor.ID)
		}
	}
	for id := range s.tasks {
		if !present[id] {
			delete(s.tasks, id)
			delete(s.invalidSchedules, id)
		}
	}
	for id := range s.pausedMonitors {
		if _, exists := present[id]; !exists {
			if monitorExistsAndEnabled(monitors, id) {
				continue
			}
			delete(s.pausedMonitors, id)
		}
	}
}

func monitorExistsAndEnabled(monitors []core.Monitor, id string) bool {
	for _, monitor := range monitors {
		if monitor.ID == id {
			return monitor.Enabled
		}
	}
	return false
}

func advanceScheduleTask(task *scheduleTask, timezone string, now time.Time) (bool, error) {
	if now.Before(task.next) {
		return false, nil
	}
	next, err := nextScheduleTime(task.expressions, timezone, now)
	if err != nil {
		return false, err
	}
	task.next = next
	return true, nil
}

func (s *Scheduler) launch(ctx context.Context, monitorID string) {
	limit := s.config().Scheduler.MaxConcurrency
	s.activeMu.Lock()
	if s.activeRuns >= limit {
		s.activeMu.Unlock()
		if s.logger != nil {
			s.logger.Warn("scheduler concurrency limit reached", "monitor_id", monitorID, "limit", limit)
		}
		return
	}
	s.activeRuns++
	s.activeMu.Unlock()
	go func() {
		defer func() {
			s.activeMu.Lock()
			s.activeRuns--
			s.activeMu.Unlock()
		}()
		if _, err := s.runner.TryRun(ctx, monitorID); err != nil && !errors.Is(err, ErrMonitorRunning) && s.logger != nil {
			if errors.Is(err, ErrMonitorModuleUnavailable) {
				s.logger.Warn("scheduled monitor skipped because module became unavailable", "monitor_id", monitorID)
			} else {
				s.logger.Error("scheduled monitor failed", "monitor_id", monitorID, "error", err)
			}
		}
	}()
}

func parseSchedule(expression string) (cron.Schedule, error) {
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(expression)
	if err != nil {
		return nil, err
	}
	return schedule, nil
}

func schedulerLocation(name string) (*time.Location, error) {
	if name == "" || name == "Local" {
		return time.Local, nil
	}
	return time.LoadLocation(name)
}

func nextScheduleTime(expressions []string, timezone string, after time.Time) (time.Time, error) {
	location, err := schedulerLocation(timezone)
	if err != nil {
		return time.Time{}, err
	}
	if len(expressions) == 0 {
		return time.Time{}, errors.New("at least one cron expression is required")
	}
	next := time.Time{}
	for _, expression := range expressions {
		schedule, parseErr := parseSchedule(expression)
		if parseErr != nil {
			return time.Time{}, fmt.Errorf("%q: %w", expression, parseErr)
		}
		candidate := schedule.Next(after.In(location))
		if next.IsZero() || candidate.Before(next) {
			next = candidate
		}
	}
	return next, nil
}

func NextScheduleTimes(expressions []string, timezone string, count int) ([]time.Time, error) {
	if count <= 0 {
		return []time.Time{}, nil
	}
	location, err := schedulerLocation(timezone)
	if err != nil {
		return nil, err
	}
	if len(expressions) == 0 {
		return nil, errors.New("at least one cron expression is required")
	}
	now := time.Now().In(location)
	result := make([]time.Time, 0, len(expressions)*count)
	for _, expression := range expressions {
		schedule, parseErr := parseSchedule(expression)
		if parseErr != nil {
			return nil, fmt.Errorf("%q: %w", expression, parseErr)
		}
		next := now
		for i := 0; i < count; i++ {
			next = schedule.Next(next)
			result = append(result, next)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Before(result[j]) })
	unique := result[:0]
	for _, value := range result {
		if len(unique) == 0 || !value.Equal(unique[len(unique)-1]) {
			unique = append(unique, value)
		}
		if len(unique) == count {
			break
		}
	}
	return unique, nil
}

func ValidateSchedules(expressions []string, timezone string) error {
	if len(expressions) == 0 {
		return errors.New("at least one cron expression is required")
	}
	if _, err := schedulerLocation(timezone); err != nil {
		return err
	}
	for _, expression := range expressions {
		if strings.TrimSpace(expression) == "" {
			return errors.New("cron expression cannot be empty")
		}
		if _, err := parseSchedule(expression); err != nil {
			return fmt.Errorf("%q: %w", expression, err)
		}
	}
	return nil
}

func DescribeSchedule(expression string) string {
	expression = strings.TrimSpace(expression)
	descriptors := map[string]string{
		"@yearly": "每年执行一次", "@annually": "每年执行一次", "@monthly": "每月执行一次",
		"@weekly": "每周执行一次", "@daily": "每天执行一次", "@midnight": "每天零点执行",
		"@hourly": "每小时执行一次",
	}
	if description, ok := descriptors[strings.ToLower(expression)]; ok {
		return description
	}
	if strings.HasPrefix(strings.ToLower(expression), "@every ") {
		return "每 " + strings.TrimSpace(expression[len("@every "):]) + " 执行一次"
	}
	fields := strings.Fields(expression)
	if len(fields) == 6 {
		if interval, ok := stepValue(fields[0]); ok && allWildcard(fields[1:]) {
			return fmt.Sprintf("每 %d 秒执行一次", interval)
		}
		if fields[0] == "0" {
			fields = fields[1:]
		} else {
			return "按自定义 Cron 计划执行"
		}
	}
	if len(fields) != 5 {
		return "按自定义 Cron 计划执行"
	}
	minute, hour, dayOfMonth, month, dayOfWeek := fields[0], fields[1], fields[2], fields[3], fields[4]
	if allWildcard(fields) {
		return "每分钟执行一次"
	}
	if interval, ok := stepValue(minute); ok && allWildcard(fields[1:]) {
		return fmt.Sprintf("每 %d 分钟执行一次", interval)
	}
	if fixedMinute, ok := fixedNumber(minute); ok && hour == "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
		return fmt.Sprintf("每小时第 %02d 分钟执行", fixedMinute)
	}
	fixedMinute, minuteOK := fixedNumber(minute)
	fixedHour, hourOK := fixedNumber(hour)
	if minuteOK && hourOK && month == "*" {
		clock := fmt.Sprintf("%02d:%02d", fixedHour, fixedMinute)
		switch {
		case dayOfMonth == "*" && dayOfWeek == "*":
			return "每天 " + clock + " 执行"
		case dayOfMonth == "*" && dayOfWeek == "1-5":
			return "每个工作日 " + clock + " 执行"
		case dayOfWeek == "*":
			if day, ok := fixedNumber(dayOfMonth); ok {
				return fmt.Sprintf("每月 %d 日 %s 执行", day, clock)
			}
		}
	}
	return "按自定义 Cron 计划执行"
}

func stepValue(value string) (int, bool) {
	if !strings.HasPrefix(value, "*/") {
		return 0, false
	}
	parsed, err := strconv.Atoi(strings.TrimPrefix(value, "*/"))
	return parsed, err == nil && parsed > 0
}

func fixedNumber(value string) (int, bool) {
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil
}

func allWildcard(values []string) bool {
	for _, value := range values {
		if value != "*" {
			return false
		}
	}
	return true
}
