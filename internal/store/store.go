package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"meerkit/internal/core"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type PageResult[T any] struct {
	Items      []T `json:"items"`
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type MonitorListOptions struct {
	Page       int
	PageSize   int
	Search     string
	ModuleType string
	Status     string
}

type RecordListOptions struct {
	Page      int
	PageSize  int
	Search    string
	Status    string
	EventType string
}

type NotificationListOptions struct {
	Page       int
	PageSize   int
	Search     string
	UnreadOnly bool
}

func OpenStore(dataDir string) (*Store, error) {
	if err := ensureDir(dataDir); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s/meerkit.db?_pragma=busy_timeout(5000)", strings.TrimRight(dataDir, "/")))
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o750)
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS monitors (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  module_type TEXT NOT NULL,
  module_version TEXT NOT NULL,
  schedules_json TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  module_config_json TEXT NOT NULL,
  condition_config_json TEXT NOT NULL,
  notification_channel_ids_json TEXT NOT NULL DEFAULT '[]',
  runtime_state_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS monitor_records (
  id TEXT PRIMARY KEY,
  monitor_id TEXT NOT NULL,
  started_at TEXT NOT NULL,
  finished_at TEXT NOT NULL,
  success INTEGER NOT NULL,
  duration_ms INTEGER NOT NULL,
  result_schema_version TEXT NOT NULL,
  result_json TEXT NOT NULL,
  result_hash TEXT NOT NULL,
  condition_state TEXT NOT NULL,
  event_type TEXT NOT NULL,
  notification_result_json TEXT NOT NULL DEFAULT '{}',
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  FOREIGN KEY (monitor_id) REFERENCES monitors(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_monitor_records_monitor_time ON monitor_records(monitor_id, started_at DESC);
CREATE TABLE IF NOT EXISTS notification_channels (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  notifier_type TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  config_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS in_app_notifications (
  id TEXT PRIMARY KEY,
  channel_id TEXT NOT NULL,
  monitor_id TEXT NOT NULL DEFAULT '',
  record_id TEXT NOT NULL DEFAULT '',
  event_type TEXT NOT NULL,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  is_read INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  read_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_in_app_notifications_created ON in_app_notifications(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_in_app_notifications_unread ON in_app_notifications(is_read, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_channels_builtin_inapp ON notification_channels(notifier_type) WHERE notifier_type='inapp';
INSERT OR IGNORE INTO notification_channels(id,name,notifier_type,enabled,config_json,created_at,updated_at)
VALUES('builtin-inapp','站内通知','inapp',1,'{"title_template":"{{monitor.name}} · {{event.type}}","body_template":"{{event.summary}}"}',strftime('%Y-%m-%dT%H:%M:%fZ','now'),strftime('%Y-%m-%dT%H:%M:%fZ','now'));`)
	return err
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) CreateMonitor(ctx context.Context, monitor core.Monitor) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO monitors
(id,name,module_type,module_version,schedules_json,enabled,module_config_json,condition_config_json,notification_channel_ids_json,runtime_state_json,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, monitor.ID, monitor.Name, monitor.ModuleType, monitor.ModuleVersion, jsonString(monitor.Schedules), boolInt(monitor.Enabled), string(monitor.ModuleConfig), string(monitor.ConditionConfig), jsonString(monitor.NotificationChannelIDs), string(monitor.RuntimeState), monitor.CreatedAt.UTC().Format(time.RFC3339Nano), monitor.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) UpdateMonitor(ctx context.Context, monitor core.Monitor) error {
	_, err := s.db.ExecContext(ctx, `UPDATE monitors SET name=?,module_type=?,module_version=?,schedules_json=?,enabled=?,module_config_json=?,condition_config_json=?,notification_channel_ids_json=?,runtime_state_json=?,updated_at=? WHERE id=?`, monitor.Name, monitor.ModuleType, monitor.ModuleVersion, jsonString(monitor.Schedules), boolInt(monitor.Enabled), string(monitor.ModuleConfig), string(monitor.ConditionConfig), jsonString(monitor.NotificationChannelIDs), string(monitor.RuntimeState), monitor.UpdatedAt.UTC().Format(time.RFC3339Nano), monitor.ID)
	return err
}

func (s *Store) UpdateRuntimeState(ctx context.Context, id string, state core.RuntimeState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE monitors SET runtime_state_json=?,updated_at=? WHERE id=?`, string(data), time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

func (s *Store) DeleteMonitor(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM monitors WHERE id=?`, id)
	return err
}

func (s *Store) GetMonitor(ctx context.Context, id string) (core.Monitor, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,module_type,module_version,schedules_json,enabled,module_config_json,condition_config_json,notification_channel_ids_json,runtime_state_json,created_at,updated_at FROM monitors WHERE id=?`, id)
	return scanMonitor(row)
}

func (s *Store) ListMonitors(ctx context.Context) ([]core.Monitor, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,module_type,module_version,schedules_json,enabled,module_config_json,condition_config_json,notification_channel_ids_json,runtime_state_json,created_at,updated_at FROM monitors ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []core.Monitor
	for rows.Next() {
		monitor, err := scanMonitor(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, monitor)
	}
	return result, rows.Err()
}

func (s *Store) ListMonitorsPage(ctx context.Context, options MonitorListOptions) (PageResult[core.Monitor], error) {
	page, pageSize := normalizePage(options.Page, options.PageSize)
	where := []string{"1=1"}
	args := make([]any, 0, 8)
	if search := strings.TrimSpace(strings.ToLower(options.Search)); search != "" {
		pattern := "%" + search + "%"
		where = append(where, "(LOWER(name) LIKE ? OR LOWER(module_type) LIKE ? OR LOWER(module_config_json) LIKE ?)")
		args = append(args, pattern, pattern, pattern)
	}
	if moduleType := strings.TrimSpace(options.ModuleType); moduleType != "" && moduleType != "all" {
		where = append(where, "module_type=?")
		args = append(args, moduleType)
	}
	switch options.Status {
	case "enabled":
		where = append(where, "enabled=1")
	case "disabled":
		where = append(where, "enabled=0")
	case "triggered":
		where = append(where, "json_extract(runtime_state_json, '$.condition_active')=1")
	case "healthy":
		where = append(where, "json_extract(runtime_state_json, '$.last_success')=1 AND COALESCE(json_extract(runtime_state_json, '$.condition_active'), 0)=0")
	case "waiting":
		where = append(where, "enabled=1 AND COALESCE(json_extract(runtime_state_json, '$.last_success'), 0)=0")
	}

	clause := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM monitors WHERE "+clause, args...).Scan(&total); err != nil {
		return PageResult[core.Monitor]{}, err
	}
	result := PageResult[core.Monitor]{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages(total, pageSize), Items: []core.Monitor{}}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,module_type,module_version,schedules_json,enabled,module_config_json,condition_config_json,notification_channel_ids_json,runtime_state_json,created_at,updated_at FROM monitors WHERE `+clause+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return PageResult[core.Monitor]{}, err
	}
	defer rows.Close()
	for rows.Next() {
		monitor, err := scanMonitor(rows)
		if err != nil {
			return PageResult[core.Monitor]{}, err
		}
		result.Items = append(result.Items, monitor)
	}
	if err := rows.Err(); err != nil {
		return PageResult[core.Monitor]{}, err
	}
	return result, nil
}

func scanMonitor(scanner interface{ Scan(...any) error }) (core.Monitor, error) {
	var monitor core.Monitor
	var enabled int
	var schedules, moduleConfig, conditionConfig, channelIDs, runtimeState string
	var createdAt, updatedAt string
	if err := scanner.Scan(&monitor.ID, &monitor.Name, &monitor.ModuleType, &monitor.ModuleVersion, &schedules, &enabled, &moduleConfig, &conditionConfig, &channelIDs, &runtimeState, &createdAt, &updatedAt); err != nil {
		return monitor, err
	}
	monitor.Enabled = enabled == 1
	_ = json.Unmarshal([]byte(schedules), &monitor.Schedules)
	monitor.ModuleConfig = json.RawMessage(moduleConfig)
	monitor.ConditionConfig = json.RawMessage(conditionConfig)
	monitor.RuntimeState = json.RawMessage(runtimeState)
	_ = json.Unmarshal([]byte(channelIDs), &monitor.NotificationChannelIDs)
	monitor.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	monitor.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return monitor, nil
}

func (s *Store) AddRecord(ctx context.Context, record core.MonitorRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO monitor_records
(id,monitor_id,started_at,finished_at,success,duration_ms,result_schema_version,result_json,result_hash,condition_state,event_type,notification_result_json,error_code,error_message)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, record.ID, record.MonitorID, record.StartedAt.UTC().Format(time.RFC3339Nano), record.FinishedAt.UTC().Format(time.RFC3339Nano), boolInt(record.Success), record.DurationMS, record.ResultSchemaVersion, jsonString(record.Result), record.ResultHash, record.ConditionState, record.EventType, jsonString(record.NotificationResult), record.ErrorCode, record.ErrorMessage)
	return err
}

func (s *Store) UpdateRecordNotifications(ctx context.Context, id string, result map[string]any) error {
	_, err := s.db.ExecContext(ctx, `UPDATE monitor_records SET notification_result_json=? WHERE id=?`, jsonString(result), id)
	return err
}

func (s *Store) ListRecords(ctx context.Context, monitorID string, limit int) ([]core.MonitorRecord, error) {
	result, err := s.ListRecordsPage(ctx, monitorID, RecordListOptions{Page: 1, PageSize: limit})
	return result.Items, err
}

func (s *Store) GetRecord(ctx context.Context, monitorID, recordID string) (core.MonitorRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,monitor_id,started_at,finished_at,success,duration_ms,result_schema_version,result_json,result_hash,condition_state,event_type,notification_result_json,error_code,error_message FROM monitor_records WHERE monitor_id=? AND id=?`, monitorID, recordID)
	return scanRecord(row)
}

func (s *Store) ListRecordsPage(ctx context.Context, monitorID string, options RecordListOptions) (PageResult[core.MonitorRecord], error) {
	page, pageSize := normalizePage(options.Page, options.PageSize)
	where := []string{"monitor_id=?"}
	args := []any{monitorID}
	if search := strings.TrimSpace(strings.ToLower(options.Search)); search != "" {
		pattern := "%" + search + "%"
		where = append(where, "(LOWER(error_message) LIKE ? OR LOWER(event_type) LIKE ? OR LOWER(condition_state) LIKE ? OR LOWER(result_hash) LIKE ? OR LOWER(result_json) LIKE ?)")
		args = append(args, pattern, pattern, pattern, pattern, pattern)
	}
	switch options.Status {
	case "success":
		where = append(where, "success=1")
	case "failed":
		where = append(where, "success=0")
	}
	if eventType := strings.TrimSpace(options.EventType); eventType != "" && eventType != "all" {
		where = append(where, "event_type=?")
		args = append(args, eventType)
	}

	clause := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM monitor_records WHERE "+clause, args...).Scan(&total); err != nil {
		return PageResult[core.MonitorRecord]{}, err
	}
	result := PageResult[core.MonitorRecord]{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages(total, pageSize), Items: []core.MonitorRecord{}}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT id,monitor_id,started_at,finished_at,success,duration_ms,result_schema_version,result_json,result_hash,condition_state,event_type,notification_result_json,error_code,error_message FROM monitor_records WHERE `+clause+` ORDER BY started_at DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return PageResult[core.MonitorRecord]{}, err
	}
	defer rows.Close()
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return PageResult[core.MonitorRecord]{}, err
		}
		result.Items = append(result.Items, record)
	}
	return result, rows.Err()
}

func (s *Store) LatestSuccessfulRecord(ctx context.Context, monitorID string) (core.MonitorRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,monitor_id,started_at,finished_at,success,duration_ms,result_schema_version,result_json,result_hash,condition_state,event_type,notification_result_json,error_code,error_message FROM monitor_records WHERE monitor_id=? AND success=1 ORDER BY started_at DESC LIMIT 1`, monitorID)
	return scanRecord(row)
}

func scanRecord(scanner interface{ Scan(...any) error }) (core.MonitorRecord, error) {
	var record core.MonitorRecord
	var success int
	var resultJSON, notificationJSON string
	var startedAt, finishedAt string
	if err := scanner.Scan(&record.ID, &record.MonitorID, &startedAt, &finishedAt, &success, &record.DurationMS, &record.ResultSchemaVersion, &resultJSON, &record.ResultHash, &record.ConditionState, &record.EventType, &notificationJSON, &record.ErrorCode, &record.ErrorMessage); err != nil {
		return record, err
	}
	record.Success = success == 1
	_ = json.Unmarshal([]byte(resultJSON), &record.Result)
	_ = json.Unmarshal([]byte(notificationJSON), &record.NotificationResult)
	record.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
	record.FinishedAt, _ = time.Parse(time.RFC3339Nano, finishedAt)
	return record, nil
}

func (s *Store) DeleteMonitorRecords(ctx context.Context, monitorID string) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM monitor_records WHERE monitor_id=?`, monitorID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) PruneRecords(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM monitor_records WHERE finished_at < ?`, before.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) CreateChannel(ctx context.Context, channel core.NotificationChannel) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO notification_channels(id,name,notifier_type,enabled,config_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, channel.ID, channel.Name, channel.NotifierType, boolInt(channel.Enabled), string(channel.Config), channel.CreatedAt.UTC().Format(time.RFC3339Nano), channel.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) UpdateChannel(ctx context.Context, channel core.NotificationChannel) error {
	_, err := s.db.ExecContext(ctx, `UPDATE notification_channels SET name=?,notifier_type=?,enabled=?,config_json=?,updated_at=? WHERE id=?`, channel.Name, channel.NotifierType, boolInt(channel.Enabled), string(channel.Config), channel.UpdatedAt.UTC().Format(time.RFC3339Nano), channel.ID)
	return err
}

func (s *Store) DeleteChannel(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM notification_channels WHERE id=?`, id)
	return err
}

func (s *Store) GetChannel(ctx context.Context, id string) (core.NotificationChannel, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,notifier_type,enabled,config_json,created_at,updated_at FROM notification_channels WHERE id=?`, id)
	return scanChannel(row)
}

func (s *Store) ListChannels(ctx context.Context) ([]core.NotificationChannel, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,notifier_type,enabled,config_json,created_at,updated_at FROM notification_channels ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var channels []core.NotificationChannel
	for rows.Next() {
		channel, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}

func (s *Store) CreateInAppNotification(ctx context.Context, notification core.InAppNotification) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO in_app_notifications(id,channel_id,monitor_id,record_id,event_type,title,content,is_read,created_at,read_at) VALUES(?,?,?,?,?,?,?,?,?,NULL)`, notification.ID, notification.ChannelID, notification.MonitorID, notification.RecordID, notification.EventType, notification.Title, notification.Content, boolInt(notification.Read), notification.CreatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) GetInAppNotification(ctx context.Context, id string) (core.InAppNotification, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,channel_id,monitor_id,record_id,event_type,title,content,is_read,created_at,read_at FROM in_app_notifications WHERE id=?`, id)
	return scanInAppNotification(row)
}

func (s *Store) ListInAppNotificationsPage(ctx context.Context, options NotificationListOptions) (PageResult[core.InAppNotification], error) {
	page, pageSize := normalizePage(options.Page, options.PageSize)
	where := []string{"1=1"}
	args := make([]any, 0, 4)
	if options.UnreadOnly {
		where = append(where, "is_read=0")
	}
	if search := strings.TrimSpace(strings.ToLower(options.Search)); search != "" {
		pattern := "%" + search + "%"
		where = append(where, "(LOWER(title) LIKE ? OR LOWER(content) LIKE ?)")
		args = append(args, pattern, pattern)
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM in_app_notifications WHERE "+clause, args...).Scan(&total); err != nil {
		return PageResult[core.InAppNotification]{}, err
	}
	result := PageResult[core.InAppNotification]{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages(total, pageSize), Items: []core.InAppNotification{}}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT id,channel_id,monitor_id,record_id,event_type,title,content,is_read,created_at,read_at FROM in_app_notifications WHERE `+clause+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return PageResult[core.InAppNotification]{}, err
	}
	defer rows.Close()
	for rows.Next() {
		notification, err := scanInAppNotification(rows)
		if err != nil {
			return PageResult[core.InAppNotification]{}, err
		}
		result.Items = append(result.Items, notification)
	}
	return result, rows.Err()
}

func (s *Store) CountUnreadInAppNotifications(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM in_app_notifications WHERE is_read=0`).Scan(&count)
	return count, err
}

func (s *Store) MarkInAppNotificationRead(ctx context.Context, id string) (core.InAppNotification, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, `UPDATE in_app_notifications SET is_read=1,read_at=COALESCE(read_at,?) WHERE id=?`, now, id); err != nil {
		return core.InAppNotification{}, err
	}
	return s.GetInAppNotification(ctx, id)
}

func (s *Store) MarkAllInAppNotificationsRead(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE in_app_notifications SET is_read=1,read_at=? WHERE is_read=0`, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) DeleteReadInAppNotifications(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM in_app_notifications WHERE is_read=1`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) PruneInAppNotifications(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM in_app_notifications WHERE created_at < ?`, before.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func scanInAppNotification(scanner interface{ Scan(...any) error }) (core.InAppNotification, error) {
	var notification core.InAppNotification
	var read int
	var createdAt string
	var readAt sql.NullString
	if err := scanner.Scan(&notification.ID, &notification.ChannelID, &notification.MonitorID, &notification.RecordID, &notification.EventType, &notification.Title, &notification.Content, &read, &createdAt, &readAt); err != nil {
		return notification, err
	}
	notification.Read = read == 1
	notification.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	if readAt.Valid {
		value, _ := time.Parse(time.RFC3339Nano, readAt.String)
		notification.ReadAt = &value
	}
	return notification, nil
}

func scanChannel(scanner interface{ Scan(...any) error }) (core.NotificationChannel, error) {
	var channel core.NotificationChannel
	var enabled int
	var config, createdAt, updatedAt string
	if err := scanner.Scan(&channel.ID, &channel.Name, &channel.NotifierType, &enabled, &config, &createdAt, &updatedAt); err != nil {
		return channel, err
	}
	channel.Enabled = enabled == 1
	channel.Config = json.RawMessage(config)
	channel.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	channel.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	channel.BuiltIn = channel.ID == core.BuiltInNotificationChannelID || channel.NotifierType == "inapp"
	return channel, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func jsonString(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

func totalPages(total, pageSize int) int {
	if total == 0 {
		return 0
	}
	return (total + pageSize - 1) / pageSize
}

func sortedRecordIDs(records []core.MonitorRecord) []string {
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	sort.Strings(ids)
	return ids
}
