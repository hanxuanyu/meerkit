package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/uptrace/bun"
	"meerkit/internal/core"
)

type Store struct {
	db             *sql.DB
	orm            *bun.DB
	databaseType   DatabaseType
	systemConfigMu sync.Mutex
}

type PageResult[T any] struct {
	Items      []T `json:"items"`
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type MonitorListOptions struct {
	Page                 int
	PageSize             int
	Search               string
	ModuleType           string
	Status               string
	AvailableModuleTypes []string
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

func (s *Store) Close() error { return s.orm.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) CreateMonitor(ctx context.Context, monitor core.Monitor) error {
	if monitor.ModuleConfigVersion == "" {
		monitor.ModuleConfigVersion = "1"
	}
	_, err := s.orm.NewInsert().Model(monitorFromDomain(monitor)).Exec(ctx)
	return err
}

func (s *Store) UpdateMonitor(ctx context.Context, monitor core.Monitor) error {
	if monitor.ModuleConfigVersion == "" {
		monitor.ModuleConfigVersion = "1"
	}
	model := monitorFromDomain(monitor)
	_, err := s.orm.NewUpdate().Model(model).Column("name", "module_type", "module_version", "module_config_version", "schedules_json", "enabled", "module_config_json", "condition_config_json", "notification_channel_ids_json", "runtime_state_json", "condition_active", "last_success", "updated_at").WherePK().Exec(ctx)
	return err
}

func (s *Store) UpdateRuntimeState(ctx context.Context, id string, state core.RuntimeState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = s.orm.NewUpdate().Model((*monitorModel)(nil)).Set("runtime_state_json = ?", string(data)).Set("condition_active = ?", state.ConditionActive).Set("last_success = ?", state.LastSuccess).Set("updated_at = ?", timestamp(time.Now())).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *Store) DeleteMonitor(ctx context.Context, id string) error {
	_, err := s.orm.NewDelete().Model((*monitorModel)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *Store) GetMonitor(ctx context.Context, id string) (core.Monitor, error) {
	model := new(monitorModel)
	if err := s.orm.NewSelect().Model(model).Where("id = ?", id).Scan(ctx); err != nil {
		return core.Monitor{}, err
	}
	return monitorToDomain(model)
}

func (s *Store) ListMonitors(ctx context.Context) ([]core.Monitor, error) {
	models := make([]monitorModel, 0)
	if err := s.orm.NewSelect().Model(&models).OrderExpr("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]core.Monitor, 0, len(models))
	for index := range models {
		monitor, err := monitorToDomain(&models[index])
		if err != nil {
			return nil, err
		}
		result = append(result, monitor)
	}
	return result, nil
}

func (s *Store) ListMonitorsPage(ctx context.Context, options MonitorListOptions) (PageResult[core.Monitor], error) {
	page, pageSize := normalizePage(options.Page, options.PageSize)
	countQuery := s.orm.NewSelect().Model((*monitorModel)(nil))
	applyMonitorListOptions(countQuery, options)
	total, err := countQuery.Count(ctx)
	if err != nil {
		return PageResult[core.Monitor]{}, err
	}
	models := make([]monitorModel, 0, pageSize)
	query := s.orm.NewSelect().Model(&models)
	applyMonitorListOptions(query, options)
	if err := query.OrderExpr("created_at DESC").Limit(pageSize).Offset((page - 1) * pageSize).Scan(ctx); err != nil {
		return PageResult[core.Monitor]{}, err
	}
	result := PageResult[core.Monitor]{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages(total, pageSize), Items: make([]core.Monitor, 0, len(models))}
	for index := range models {
		monitor, err := monitorToDomain(&models[index])
		if err != nil {
			return PageResult[core.Monitor]{}, err
		}
		result.Items = append(result.Items, monitor)
	}
	return result, nil
}

func applyMonitorListOptions(query *bun.SelectQuery, options MonitorListOptions) {
	if search := strings.TrimSpace(strings.ToLower(options.Search)); search != "" {
		pattern := "%" + search + "%"
		query.Where("(LOWER(name) LIKE ? OR LOWER(module_type) LIKE ? OR LOWER(module_config_json) LIKE ?)", pattern, pattern, pattern)
	}
	if moduleType := strings.TrimSpace(options.ModuleType); moduleType != "" && moduleType != "all" {
		query.Where("module_type = ?", moduleType)
	}
	switch options.Status {
	case "enabled":
		query.Where("enabled = ?", true)
	case "disabled":
		query.Where("enabled = ?", false)
	case "triggered":
		query.Where("enabled = ?", true).Where("condition_active = ?", true)
		applyModuleAvailabilityFilter(query, options.AvailableModuleTypes, true)
	case "healthy":
		query.Where("enabled = ?", true).Where("last_success = ?", true).Where("condition_active = ?", false)
		applyModuleAvailabilityFilter(query, options.AvailableModuleTypes, true)
	case "waiting":
		query.Where("enabled = ?", true).Where("last_success = ?", false)
		applyModuleAvailabilityFilter(query, options.AvailableModuleTypes, true)
	case "unavailable":
		query.Where("enabled = ?", true)
		applyModuleAvailabilityFilter(query, options.AvailableModuleTypes, false)
	}
}

func applyModuleAvailabilityFilter(query *bun.SelectQuery, available []string, wantAvailable bool) {
	if len(available) == 0 {
		if wantAvailable {
			query.Where("1 = 0")
		}
		return
	}
	if !wantAvailable {
		query.Where("module_type NOT IN (?)", bun.In(available))
		return
	}
	query.Where("module_type IN (?)", bun.In(available))
}

func monitorFromDomain(monitor core.Monitor) *monitorModel {
	conditionActive, lastSuccess := monitorRuntimeFlags(monitor.RuntimeState)
	return &monitorModel{
		ID: monitor.ID, Name: monitor.Name, ModuleType: monitor.ModuleType, ModuleVersion: monitor.ModuleVersion, ModuleConfigVersion: monitor.ModuleConfigVersion,
		SchedulesJSON: jsonString(monitor.Schedules), Enabled: monitor.Enabled, ModuleConfigJSON: string(monitor.ModuleConfig), ConditionConfigJSON: string(monitor.ConditionConfig),
		NotificationChannelIDsJSON: jsonString(monitor.NotificationChannelIDs), RuntimeStateJSON: string(monitor.RuntimeState), ConditionActive: conditionActive, LastSuccess: lastSuccess,
		CreatedAt: timestamp(monitor.CreatedAt), UpdatedAt: timestamp(monitor.UpdatedAt),
	}
}

func monitorToDomain(model *monitorModel) (core.Monitor, error) {
	monitor := core.Monitor{
		ID: model.ID, Name: model.Name, ModuleType: model.ModuleType, ModuleVersion: model.ModuleVersion, ModuleConfigVersion: model.ModuleConfigVersion,
		Enabled: model.Enabled, ModuleConfig: json.RawMessage(model.ModuleConfigJSON), ConditionConfig: json.RawMessage(model.ConditionConfigJSON), RuntimeState: json.RawMessage(model.RuntimeStateJSON),
	}
	if err := json.Unmarshal([]byte(model.SchedulesJSON), &monitor.Schedules); err != nil {
		return monitor, err
	}
	if err := json.Unmarshal([]byte(model.NotificationChannelIDsJSON), &monitor.NotificationChannelIDs); err != nil {
		return monitor, err
	}
	monitor.CreatedAt, _ = time.Parse(time.RFC3339Nano, model.CreatedAt)
	monitor.UpdatedAt, _ = time.Parse(time.RFC3339Nano, model.UpdatedAt)
	return monitor, nil
}

func (s *Store) AddRecord(ctx context.Context, record core.MonitorRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := addRecordTx(ctx, tx, record); err != nil {
		return err
	}
	if err := addPendingNotificationDeliveriesTx(ctx, tx, record); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UpdateRecordNotificationEvents(ctx context.Context, id string, events []core.RecordNotificationEvent) error {
	eventCount, trendTriggered, trendRecovered := notificationEventFlags(events)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE monitor_records SET notification_events_json=?,notification_event_count=?,trend_triggered=?,trend_recovered=? WHERE id=?`, jsonString(notificationEventMetadata(events)), eventCount, boolInt(trendTriggered), boolInt(trendRecovered), id); err != nil {
		return err
	}
	now := unixMicros(time.Now())
	for _, event := range events {
		for channelID, delivery := range event.Deliveries {
			var deliveredAt any
			if delivery.Status == "sent" {
				deliveredAt = now
			}
			if _, err := tx.ExecContext(ctx, `UPDATE notification_deliveries SET status=?,attempts=?,message=?,updated_at=?,delivered_at=COALESCE(?,delivered_at) WHERE event_id=? AND channel_id=?`, delivery.Status, delivery.Attempts, delivery.Message, now, deliveredAt, event.ID, channelID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) ListRecords(ctx context.Context, monitorID string, limit int) ([]core.MonitorRecord, error) {
	result, err := s.ListRecordsPage(ctx, monitorID, RecordListOptions{Page: 1, PageSize: limit})
	return result.Items, err
}

func (s *Store) GetRecord(ctx context.Context, monitorID, recordID string) (core.MonitorRecord, error) {
	model := new(monitorRecordModel)
	if err := s.orm.NewSelect().Model(model).Where("monitor_id = ?", monitorID).Where("id = ?", recordID).Scan(ctx); err != nil {
		return core.MonitorRecord{}, err
	}
	record, err := recordToDomain(model)
	if err != nil {
		return core.MonitorRecord{}, err
	}
	records := []core.MonitorRecord{record}
	if err := s.hydrateNotificationDeliveries(ctx, records); err != nil {
		return core.MonitorRecord{}, err
	}
	return records[0], nil
}

func (s *Store) ListRecordsPage(ctx context.Context, monitorID string, options RecordListOptions) (PageResult[core.MonitorRecord], error) {
	page, pageSize := normalizePage(options.Page, options.PageSize)
	countQuery := s.orm.NewSelect().Model((*monitorRecordModel)(nil))
	applyRecordListOptions(countQuery, monitorID, options)
	total, err := countQuery.Count(ctx)
	if err != nil {
		return PageResult[core.MonitorRecord]{}, err
	}
	models := make([]monitorRecordModel, 0, pageSize)
	query := s.orm.NewSelect().Model(&models)
	applyRecordListOptions(query, monitorID, options)
	if err := query.OrderExpr("started_at DESC").Limit(pageSize).Offset((page - 1) * pageSize).Scan(ctx); err != nil {
		return PageResult[core.MonitorRecord]{}, err
	}
	result := PageResult[core.MonitorRecord]{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages(total, pageSize), Items: make([]core.MonitorRecord, 0, len(models))}
	for index := range models {
		record, err := recordToDomain(&models[index])
		if err != nil {
			return PageResult[core.MonitorRecord]{}, err
		}
		result.Items = append(result.Items, record)
	}
	if err := s.hydrateNotificationDeliveries(ctx, result.Items); err != nil {
		return PageResult[core.MonitorRecord]{}, err
	}
	return result, nil
}

func applyRecordListOptions(query *bun.SelectQuery, monitorID string, options RecordListOptions) {
	query.Where("monitor_id = ?", monitorID)
	if search := strings.TrimSpace(strings.ToLower(options.Search)); search != "" {
		pattern := "%" + search + "%"
		query.Where("(LOWER(error_message) LIKE ? OR LOWER(event_type) LIKE ? OR LOWER(condition_state) LIKE ? OR LOWER(result_hash) LIKE ? OR LOWER(result_json) LIKE ?)", pattern, pattern, pattern, pattern, pattern)
	}
	switch options.Status {
	case "success":
		query.Where("success = ?", true)
	case "failed":
		query.Where("success = ?", false)
	}
	if eventType := strings.TrimSpace(options.EventType); eventType != "" && eventType != "all" {
		if eventType == "trend_triggered" || eventType == "trend_recovered" {
			query.Where(map[string]string{"trend_triggered": "trend_triggered = ?", "trend_recovered": "trend_recovered = ?"}[eventType], true)
		} else if eventType == "none" {
			query.Where("event_type = ?", "none").Where("notification_event_count = 0")
		} else {
			query.Where("event_type = ?", eventType)
		}
	}
}

func (s *Store) LatestSuccessfulRecord(ctx context.Context, monitorID string) (core.MonitorRecord, error) {
	model := new(monitorRecordModel)
	if err := s.orm.NewSelect().Model(model).Where("monitor_id = ?", monitorID).Where("success = ?", true).OrderExpr("started_at DESC").Limit(1).Scan(ctx); err != nil {
		return core.MonitorRecord{}, err
	}
	return recordToDomain(model)
}

func recordFromDomain(record core.MonitorRecord) *monitorRecordModel {
	eventCount, trendTriggered, trendRecovered := notificationEventFlags(record.NotificationEvents)
	return &monitorRecordModel{
		ID: record.ID, MonitorID: record.MonitorID, ModuleType: record.ModuleType, ModuleVersion: record.ModuleVersion,
		StartedAt: unixMicros(record.StartedAt), FinishedAt: unixMicros(record.FinishedAt), Success: record.Success, DurationMS: record.DurationMS,
		ResultSchemaVersion: record.ResultSchemaVersion, ResultJSON: jsonString(record.Result), ResultHash: record.ResultHash,
		ConditionState: record.ConditionState, EventType: record.EventType, NotificationEventsJSON: jsonString(notificationEventMetadata(record.NotificationEvents)),
		NotificationEventCount: eventCount, TrendTriggered: trendTriggered, TrendRecovered: trendRecovered, ErrorCode: record.ErrorCode, ErrorMessage: record.ErrorMessage,
	}
}

func notificationEventMetadata(events []core.RecordNotificationEvent) []core.RecordNotificationEvent {
	result := make([]core.RecordNotificationEvent, len(events))
	for index := range events {
		result[index] = events[index]
		result[index].Deliveries = nil
	}
	return result
}

func (s *Store) hydrateNotificationDeliveries(ctx context.Context, records []core.MonitorRecord) error {
	if len(records) == 0 {
		return nil
	}
	recordIDs := make([]string, 0, len(records))
	recordIndex := make(map[string]int, len(records))
	for index := range records {
		recordIDs = append(recordIDs, records[index].ID)
		recordIndex[records[index].ID] = index
		for eventIndex := range records[index].NotificationEvents {
			records[index].NotificationEvents[eventIndex].Deliveries = map[string]core.NotificationDelivery{}
		}
	}
	models := make([]notificationDeliveryModel, 0)
	if err := s.orm.NewSelect().Model(&models).Where("record_id IN (?)", bun.In(recordIDs)).Scan(ctx); err != nil {
		return err
	}
	for index := range models {
		model := &models[index]
		recordPosition, ok := recordIndex[model.RecordID]
		if !ok {
			continue
		}
		for eventIndex := range records[recordPosition].NotificationEvents {
			event := &records[recordPosition].NotificationEvents[eventIndex]
			if event.ID == model.EventID {
				event.Deliveries[model.ChannelID] = core.NotificationDelivery{Status: model.Status, Attempts: model.Attempts, Message: model.Message}
				break
			}
		}
	}
	return nil
}

func recordToDomain(model *monitorRecordModel) (core.MonitorRecord, error) {
	record := core.MonitorRecord{
		ID: model.ID, MonitorID: model.MonitorID, ModuleType: model.ModuleType, ModuleVersion: model.ModuleVersion,
		Success: model.Success, DurationMS: model.DurationMS, ResultSchemaVersion: model.ResultSchemaVersion, ResultHash: model.ResultHash,
		ConditionState: model.ConditionState, EventType: model.EventType, ErrorCode: model.ErrorCode, ErrorMessage: model.ErrorMessage,
	}
	if err := json.Unmarshal([]byte(model.ResultJSON), &record.Result); err != nil {
		return record, err
	}
	if err := json.Unmarshal([]byte(model.NotificationEventsJSON), &record.NotificationEvents); err != nil {
		return record, err
	}
	record.StartedAt = time.UnixMicro(model.StartedAt).UTC()
	record.FinishedAt = time.UnixMicro(model.FinishedAt).UTC()
	return record, nil
}

func (s *Store) DeleteMonitorRecords(ctx context.Context, monitorID string) (int64, error) {
	result, err := s.orm.NewDelete().Model((*monitorRecordModel)(nil)).Where("monitor_id = ?", monitorID).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) PruneRecords(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.orm.NewDelete().Model((*monitorRecordModel)(nil)).Where("finished_at < ?", unixMicros(before)).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) CreateChannel(ctx context.Context, channel core.NotificationChannel) error {
	_, err := s.orm.NewInsert().Model(notificationChannelFromDomain(channel)).Exec(ctx)
	return err
}

func (s *Store) UpdateChannel(ctx context.Context, channel core.NotificationChannel) error {
	model := notificationChannelFromDomain(channel)
	_, err := s.orm.NewUpdate().Model(model).Column("name", "notifier_type", "enabled", "config_json", "updated_at").WherePK().Exec(ctx)
	return err
}

func (s *Store) DeleteChannel(ctx context.Context, id string) error {
	_, err := s.orm.NewDelete().Model((*notificationChannelModel)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *Store) GetChannel(ctx context.Context, id string) (core.NotificationChannel, error) {
	model := new(notificationChannelModel)
	if err := s.orm.NewSelect().Model(model).Where("id = ?", id).Scan(ctx); err != nil {
		return core.NotificationChannel{}, err
	}
	return notificationChannelToDomain(model), nil
}

func (s *Store) ListChannels(ctx context.Context) ([]core.NotificationChannel, error) {
	models := make([]notificationChannelModel, 0)
	if err := s.orm.NewSelect().Model(&models).OrderExpr("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	channels := make([]core.NotificationChannel, 0, len(models))
	for index := range models {
		channels = append(channels, notificationChannelToDomain(&models[index]))
	}
	return channels, nil
}

func (s *Store) CreateInAppNotification(ctx context.Context, notification core.InAppNotification) error {
	now := notification.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return s.CreateNotificationDelivery(ctx, core.NotificationDeliveryRecord{
		ID: notification.ID, EventID: notification.ID, Source: "inapp", EventType: notification.EventType,
		ChannelID: notification.ChannelID, NotifierType: "inapp", MonitorID: notification.MonitorID, RecordID: notification.RecordID,
		Title: notification.Title, Content: notification.Content, Status: "sent", Attempts: 1, Read: notification.Read,
		CreatedAt: now, UpdatedAt: now, DeliveredAt: &now, ReadAt: notification.ReadAt,
	})
}

func (s *Store) CreateNotificationDelivery(ctx context.Context, delivery core.NotificationDeliveryRecord) error {
	if delivery.ID == "" {
		delivery.ID = core.NewID()
	}
	if delivery.CreatedAt.IsZero() {
		delivery.CreatedAt = time.Now().UTC()
	}
	if delivery.UpdatedAt.IsZero() {
		delivery.UpdatedAt = delivery.CreatedAt
	}
	model := notificationDeliveryFromDomain(delivery)
	_, err := s.orm.NewInsert().Model(model).Ignore().Exec(ctx)
	return err
}

func (s *Store) UpdateNotificationDeliveryContent(ctx context.Context, eventID, channelID, title, content string, payload json.RawMessage) error {
	query := s.orm.NewUpdate().Model((*notificationDeliveryModel)(nil)).
		Set("title = ?", title).
		Set("content = ?", content).
		Set("updated_at = ?", unixMicros(time.Now())).
		Where("event_id = ? AND channel_id = ?", eventID, channelID)
	if len(payload) > 0 {
		query = query.Set("payload_json = ?", string(payload))
	}
	_, err := query.Exec(ctx)
	return err
}

func (s *Store) UpdateNotificationDeliveryResult(ctx context.Context, eventID, channelID string, result core.NotificationDelivery) error {
	now := time.Now().UTC()
	query := s.orm.NewUpdate().Model((*notificationDeliveryModel)(nil)).
		Set("status = ?", result.Status).
		Set("attempts = ?", result.Attempts).
		Set("message = ?", result.Message).
		Set("updated_at = ?", unixMicros(now)).
		Where("event_id = ? AND channel_id = ?", eventID, channelID)
	if result.Status == "sent" {
		query = query.Set("delivered_at = ?", unixMicros(now))
	}
	_, err := query.Exec(ctx)
	return err
}

func (s *Store) GetInAppNotification(ctx context.Context, id string) (core.InAppNotification, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,channel_id,monitor_id,record_id,event_type,title,content,is_read,created_at,read_at FROM notification_deliveries WHERE notifier_type='inapp' AND id=?`, id)
	return scanInAppNotification(row)
}

func (s *Store) GetInAppNotificationByEvent(ctx context.Context, eventID, channelID string) (core.InAppNotification, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,channel_id,monitor_id,record_id,event_type,title,content,is_read,created_at,read_at FROM notification_deliveries WHERE notifier_type='inapp' AND event_id=? AND channel_id=?`, eventID, channelID)
	return scanInAppNotification(row)
}

func (s *Store) UpdateNotificationDeliveryNotifier(ctx context.Context, eventID, channelID, notifierType string) error {
	_, err := s.orm.NewUpdate().Model((*notificationDeliveryModel)(nil)).
		Set("notifier_type = ?", notifierType).
		Set("updated_at = ?", unixMicros(time.Now())).
		Where("event_id = ? AND channel_id = ?", eventID, channelID).
		Exec(ctx)
	return err
}

func (s *Store) ListInAppNotificationsPage(ctx context.Context, options NotificationListOptions) (PageResult[core.InAppNotification], error) {
	page, pageSize := normalizePage(options.Page, options.PageSize)
	where := []string{"notifier_type='inapp'"}
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
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM notification_deliveries WHERE "+clause, args...).Scan(&total); err != nil {
		return PageResult[core.InAppNotification]{}, err
	}
	result := PageResult[core.InAppNotification]{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages(total, pageSize), Items: []core.InAppNotification{}}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT id,channel_id,monitor_id,record_id,event_type,title,content,is_read,created_at,read_at FROM notification_deliveries WHERE `+clause+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, queryArgs...)
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
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_deliveries WHERE notifier_type='inapp' AND is_read=0`).Scan(&count)
	return count, err
}

func (s *Store) MarkInAppNotificationRead(ctx context.Context, id string) (core.InAppNotification, error) {
	now := unixMicros(time.Now())
	if _, err := s.db.ExecContext(ctx, `UPDATE notification_deliveries SET is_read=1,read_at=COALESCE(read_at,?),updated_at=? WHERE notifier_type='inapp' AND id=?`, now, now, id); err != nil {
		return core.InAppNotification{}, err
	}
	return s.GetInAppNotification(ctx, id)
}

func (s *Store) MarkAllInAppNotificationsRead(ctx context.Context) (int64, error) {
	now := unixMicros(time.Now())
	result, err := s.db.ExecContext(ctx, `UPDATE notification_deliveries SET is_read=1,read_at=?,updated_at=? WHERE notifier_type='inapp' AND is_read=0`, now, now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) DeleteReadInAppNotifications(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM notification_deliveries WHERE notifier_type='inapp' AND is_read=1`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) PruneInAppNotifications(ctx context.Context, before time.Time) (int64, error) {
	return s.PruneNotificationDeliveries(ctx, before)
}

func (s *Store) PruneNotificationDeliveries(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM notification_deliveries WHERE created_at < ?`, unixMicros(before))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func scanInAppNotification(scanner interface{ Scan(...any) error }) (core.InAppNotification, error) {
	var notification core.InAppNotification
	var read int
	var createdAt int64
	var readAt sql.NullInt64
	if err := scanner.Scan(&notification.ID, &notification.ChannelID, &notification.MonitorID, &notification.RecordID, &notification.EventType, &notification.Title, &notification.Content, &read, &createdAt, &readAt); err != nil {
		return notification, err
	}
	notification.Read = read == 1
	notification.CreatedAt = time.UnixMicro(createdAt).UTC()
	if readAt.Valid {
		value := time.UnixMicro(readAt.Int64).UTC()
		notification.ReadAt = &value
	}
	return notification, nil
}

func notificationDeliveryFromDomain(value core.NotificationDeliveryRecord) *notificationDeliveryModel {
	payload := "{}"
	if len(value.Payload) > 0 {
		payload = string(value.Payload)
	}
	return &notificationDeliveryModel{
		ID: value.ID, EventID: value.EventID, Source: value.Source, EventType: value.EventType,
		StatusItemID: value.StatusItemID, TrendRuleID: value.TrendRuleID, ChannelID: value.ChannelID,
		NotifierType: value.NotifierType, MonitorID: value.MonitorID, RecordID: value.RecordID,
		Title: value.Title, Content: value.Content, PayloadJSON: payload, Status: value.Status,
		Attempts: value.Attempts, Message: value.Message, IsRead: value.Read,
		CreatedAt: unixMicros(value.CreatedAt), UpdatedAt: unixMicros(value.UpdatedAt),
		DeliveredAt: optionalUnixMicros(value.DeliveredAt), ReadAt: optionalUnixMicros(value.ReadAt),
	}
}

func optionalUnixMicros(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	formatted := unixMicros(*value)
	return &formatted
}

func notificationChannelFromDomain(channel core.NotificationChannel) *notificationChannelModel {
	var builtinKey *string
	if channel.ID == core.BuiltInNotificationChannelID {
		key := "inapp"
		builtinKey = &key
	}
	return &notificationChannelModel{
		ID: channel.ID, BuiltinKey: builtinKey, Name: channel.Name, NotifierType: channel.NotifierType,
		Enabled: channel.Enabled, ConfigJSON: string(channel.Config), CreatedAt: timestamp(channel.CreatedAt), UpdatedAt: timestamp(channel.UpdatedAt),
	}
}

func notificationChannelToDomain(model *notificationChannelModel) core.NotificationChannel {
	createdAt, _ := time.Parse(time.RFC3339Nano, model.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339Nano, model.UpdatedAt)
	return core.NotificationChannel{
		ID: model.ID, Name: model.Name, NotifierType: model.NotifierType, Enabled: model.Enabled,
		Config: json.RawMessage(model.ConfigJSON), CreatedAt: createdAt, UpdatedAt: updatedAt, BuiltIn: model.BuiltinKey != nil,
	}
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

func monitorRuntimeFlags(raw json.RawMessage) (bool, bool) {
	var state core.RuntimeState
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &state)
	}
	return state.ConditionActive, state.LastSuccess
}

func notificationEventFlags(events []core.RecordNotificationEvent) (int, bool, bool) {
	var triggered, recovered bool
	for _, event := range events {
		switch event.EventType {
		case "trend_triggered":
			triggered = true
		case "trend_recovered":
			recovered = true
		}
	}
	return len(events), triggered, recovered
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
