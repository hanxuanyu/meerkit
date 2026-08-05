package runtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
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

var ErrMonitorRunning = errors.New("monitor is already running")

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

func (r *Runner) runLocked(ctx context.Context, id string) (core.MonitorRecord, error) {
	monitor, err := r.store.GetMonitor(ctx, id)
	if err != nil {
		return core.MonitorRecord{}, err
	}
	module, ok := r.modules.Get(monitor.ModuleType)
	if !ok {
		return core.MonitorRecord{}, fmt.Errorf("unknown monitor module %q", monitor.ModuleType)
	}
	if r.logger != nil {
		r.logger.Debug("monitor execution started", "monitor_id", monitor.ID, "monitor_name", monitor.Name, "module_type", monitor.ModuleType, "module_version", module.Descriptor().Version)
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
	evaluation := core.EvaluateConditions(conditionConfig, current, previousResult)
	state := core.RuntimeState{}
	_ = json.Unmarshal(monitor.RuntimeState, &state)
	previousActive := state.ConditionActive
	eventType := "none"
	active := previousActive
	if evaluation.State == "true" {
		active = true
		if !previousActive {
			eventType = "triggered"
		}
	} else if evaluation.State == "false" {
		active = false
		if previousActive {
			eventType = "recovered"
		}
	}
	resultHash := core.HashString(core.JSONString(current))
	finished := time.Now().UTC()
	record := core.MonitorRecord{
		ID: core.NewID(), MonitorID: monitor.ID, StartedAt: started, FinishedAt: finished,
		Success: observation.Success && executeErr == nil, DurationMS: duration.Milliseconds(), ResultSchemaVersion: observation.SchemaVersion,
		Result: current, ResultHash: resultHash, ConditionState: evaluation.State, EventType: eventType,
		NotificationResult: map[string]any{}, ErrorCode: observation.ErrorCode, ErrorMessage: observation.ErrorMessage,
	}
	if record.ResultSchemaVersion == "" {
		record.ResultSchemaVersion = module.Descriptor().Version
	}
	if err := r.store.AddRecord(ctx, record); err != nil {
		return record, err
	}
	state.ConditionActive = active
	state.LastRecordID = record.ID
	state.LastRunAt = finished
	state.LastSuccess = record.Success
	state.LastSummary = observation.Summary
	if err := r.store.UpdateRuntimeState(ctx, monitor.ID, state); err != nil {
		return record, err
	}
	if r.logger != nil {
		r.logger.Info("monitor execution completed", "monitor_id", monitor.ID, "monitor_name", monitor.Name, "module_type", monitor.ModuleType, "success", record.Success, "duration_ms", record.DurationMS, "condition_state", record.ConditionState, "event_type", record.EventType, "summary", observation.Summary, "error_code", record.ErrorCode)
	}
	if eventType != "none" {
		event := core.NotificationEvent{EventType: eventType, MonitorID: monitor.ID, MonitorName: monitor.Name, ModuleType: monitor.ModuleType, TriggeredAt: finished, ConditionState: evaluation.State, Summary: observation.Summary, CurrentResult: current, ConditionDetail: evaluation.Details}
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
	defaultTimezone  string
	poll             time.Duration
	maxConcurrency   int
	retention        time.Duration
	semaphore        chan struct{}
	logger           *slog.Logger
	tasksMu          sync.Mutex
	tasks            map[string]*scheduleTask
	invalidSchedules map[string]string
}

func NewScheduler(runner *Runner, store *store.Store, config app.Config, logger *slog.Logger) *Scheduler {
	return &Scheduler{runner: runner, store: store, defaultTimezone: config.Scheduler.Timezone, poll: time.Duration(config.Scheduler.PollMilliseconds) * time.Millisecond, maxConcurrency: config.Scheduler.MaxConcurrency, retention: config.RetentionDuration(), semaphore: make(chan struct{}, config.Scheduler.MaxConcurrency), logger: logger, tasks: make(map[string]*scheduleTask), invalidSchedules: make(map[string]string)}
}

func (s *Scheduler) Start(ctx context.Context) {
	if s.logger != nil {
		s.logger.Info("scheduler started", "poll_ms", s.poll.Milliseconds(), "max_concurrency", s.maxConcurrency, "retention", s.retention.String())
	}
	ticker := time.NewTicker(s.poll)
	defer ticker.Stop()
	defer func() {
		if s.logger != nil {
			s.logger.Info("scheduler stopped")
		}
	}()
	lastPrune := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.syncAndRun(ctx, now)
			if time.Since(lastPrune) >= time.Hour {
				before := time.Now().Add(-s.retention)
				if deleted, err := s.store.Prune(ctx, before); err != nil && s.logger != nil {
					s.logger.Error("prune records failed", "error", err)
				} else if s.logger != nil {
					s.logger.Info("records pruned", "deleted", deleted, "before", before)
				}
				lastPrune = time.Now()
			}
		}
	}
}

func (s *Scheduler) syncAndRun(ctx context.Context, now time.Time) {
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
	for _, monitor := range monitors {
		if !monitor.Enabled {
			continue
		}
		present[monitor.ID] = true
		task := s.tasks[monitor.ID]
		signature := strings.Join(monitor.Schedules, "\x00")
		if task == nil || strings.Join(task.expressions, "\x00") != signature {
			next, scheduleErr := nextScheduleTime(monitor.Schedules, s.defaultTimezone, now)
			if scheduleErr != nil {
				if s.invalidSchedules[monitor.ID] != signature && s.logger != nil {
					s.logger.Warn("monitor schedule is invalid", "monitor_id", monitor.ID, "schedules", monitor.Schedules, "timezone", s.defaultTimezone, "error", scheduleErr)
				}
				s.invalidSchedules[monitor.ID] = signature
				continue
			}
			delete(s.invalidSchedules, monitor.ID)
			task = &scheduleTask{expressions: append([]string(nil), monitor.Schedules...), next: next}
			s.tasks[monitor.ID] = task
		}
		if !now.Before(task.next) {
			next, scheduleErr := nextScheduleTime(task.expressions, s.defaultTimezone, now)
			if scheduleErr == nil {
				task.next = next
				if s.logger != nil {
					s.logger.Debug("scheduled monitor queued", "monitor_id", monitor.ID, "next_run", task.next, "schedules", task.expressions)
				}
				s.launch(ctx, monitor.ID)
			} else if s.logger != nil {
				s.logger.Warn("monitor schedule became invalid", "monitor_id", monitor.ID, "schedules", task.expressions, "error", scheduleErr)
			}
		}
	}
	for id := range s.tasks {
		if !present[id] {
			delete(s.tasks, id)
			delete(s.invalidSchedules, id)
		}
	}
}

func (s *Scheduler) launch(ctx context.Context, monitorID string) {
	select {
	case s.semaphore <- struct{}{}:
		go func() {
			defer func() { <-s.semaphore }()
			if _, err := s.runner.TryRun(ctx, monitorID); err != nil && !errors.Is(err, ErrMonitorRunning) && s.logger != nil {
				s.logger.Error("scheduled monitor failed", "monitor_id", monitorID, "error", err)
			}
		}()
	default:
		if s.logger != nil {
			s.logger.Warn("scheduler concurrency limit reached", "monitor_id", monitorID, "limit", s.maxConcurrency)
		}
	}
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
