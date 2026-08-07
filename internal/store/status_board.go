package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"meerkit/internal/core"
)

func (s *Store) CreateStatusBoardItem(ctx context.Context, item core.StatusBoardItem) error {
	_, err := s.orm.NewInsert().Model(statusBoardItemFromDomain(item)).Exec(ctx)
	return err
}

func (s *Store) UpdateStatusBoardItem(ctx context.Context, item core.StatusBoardItem) error {
	model := statusBoardItemFromDomain(item)
	_, err := s.orm.NewUpdate().Model(model).
		Column("name", "monitor_id", "enabled", "source_json", "invert", "thresholds_json", "history_limit", "trend_rules_json", "notification_channel_ids_json", "runtime_state_json", "updated_at").
		WherePK().Exec(ctx)
	return err
}

func (s *Store) GetStatusBoardItem(ctx context.Context, id string) (core.StatusBoardItem, error) {
	model := new(statusBoardItemModel)
	err := s.orm.NewSelect().Model(model).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return core.StatusBoardItem{}, err
	}
	return statusBoardItemToDomain(model)
}

func (s *Store) ListStatusBoardItems(ctx context.Context) ([]core.StatusBoardItem, error) {
	return s.listStatusBoardItems(ctx, "")
}

func (s *Store) ListStatusBoardItemsByMonitor(ctx context.Context, monitorID string) ([]core.StatusBoardItem, error) {
	return s.listStatusBoardItems(ctx, monitorID)
}

func (s *Store) listStatusBoardItems(ctx context.Context, monitorID string) ([]core.StatusBoardItem, error) {
	models := make([]statusBoardItemModel, 0)
	query := s.orm.NewSelect().Model(&models).OrderExpr("created_at ASC")
	if monitorID != "" {
		query = query.Where("monitor_id = ?", monitorID)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, err
	}
	items := make([]core.StatusBoardItem, 0, len(models))
	for index := range models {
		item, err := statusBoardItemToDomain(&models[index])
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) DeleteStatusBoardItem(ctx context.Context, id string) error {
	_, err := s.orm.NewDelete().Model((*statusBoardItemModel)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *Store) ResetStatusBoardRuntimeByMonitor(ctx context.Context, monitorID string, at time.Time) error {
	state := core.StatusItemRuntimeState{EvaluationStartedAt: at.UTC(), Rules: map[string]core.TrendRuleState{}}
	_, err := s.orm.NewUpdate().Model((*statusBoardItemModel)(nil)).Set("runtime_state_json = ?", jsonString(state)).Set("updated_at = ?", timestamp(at)).Where("monitor_id = ?", monitorID).Exec(ctx)
	return err
}

func statusBoardItemFromDomain(item core.StatusBoardItem) *statusBoardItemModel {
	return &statusBoardItemModel{
		ID: item.ID, Name: item.Name, MonitorID: item.MonitorID, Enabled: item.Enabled,
		SourceJSON: jsonString(item.Source), Invert: item.Invert, ThresholdsJSON: jsonString(item.Thresholds), HistoryLimit: item.HistoryLimit,
		TrendRulesJSON: jsonString(item.TrendRules), NotificationChannelIDsJSON: jsonString(item.NotificationChannelIDs), RuntimeStateJSON: jsonString(item.RuntimeState),
		CreatedAt: timestamp(item.CreatedAt), UpdatedAt: timestamp(item.UpdatedAt),
	}
}

func statusBoardItemToDomain(model *statusBoardItemModel) (core.StatusBoardItem, error) {
	item := core.StatusBoardItem{ID: model.ID, Name: model.Name, MonitorID: model.MonitorID, Enabled: model.Enabled, Invert: model.Invert, HistoryLimit: model.HistoryLimit}
	if err := json.Unmarshal([]byte(model.SourceJSON), &item.Source); err != nil {
		return item, err
	}
	if err := json.Unmarshal([]byte(model.ThresholdsJSON), &item.Thresholds); err != nil {
		return item, err
	}
	if err := json.Unmarshal([]byte(model.TrendRulesJSON), &item.TrendRules); err != nil {
		return item, err
	}
	if err := json.Unmarshal([]byte(model.NotificationChannelIDsJSON), &item.NotificationChannelIDs); err != nil {
		return item, err
	}
	if err := json.Unmarshal([]byte(model.RuntimeStateJSON), &item.RuntimeState); err != nil {
		return item, err
	}
	if item.RuntimeState.Rules == nil {
		item.RuntimeState.Rules = map[string]core.TrendRuleState{}
	}
	item.CreatedAt, _ = time.Parse(time.RFC3339Nano, model.CreatedAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339Nano, model.UpdatedAt)
	return item, nil
}

func (s *Store) CommitMonitorExecution(ctx context.Context, record core.MonitorRecord, monitorID string, monitorState core.RuntimeState, itemStates map[string]core.StatusItemRuntimeState) error {
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
	monitorResult, err := tx.ExecContext(ctx, `UPDATE monitors SET runtime_state_json=?,condition_active=?,last_success=?,updated_at=? WHERE id=?`, jsonString(monitorState), boolInt(monitorState.ConditionActive), boolInt(monitorState.LastSuccess), time.Now().UTC().Format(time.RFC3339Nano), monitorID)
	if err != nil {
		return err
	}
	if affected, _ := monitorResult.RowsAffected(); affected != 1 {
		return fmt.Errorf("monitor %s was not updated", monitorID)
	}
	for id, state := range itemStates {
		itemResult, updateErr := tx.ExecContext(ctx, `UPDATE status_board_items SET runtime_state_json=? WHERE id=?`, jsonString(state), id)
		if updateErr != nil {
			return updateErr
		}
		if affected, _ := itemResult.RowsAffected(); affected != 1 {
			return fmt.Errorf("status board item %s was not updated", id)
		}
	}
	return tx.Commit()
}

func addRecordTx(ctx context.Context, tx *sql.Tx, record core.MonitorRecord) error {
	eventCount, trendTriggered, trendRecovered := notificationEventFlags(record.NotificationEvents)
	_, err := tx.ExecContext(ctx, `INSERT INTO monitor_records
(id,monitor_id,module_type,module_version,started_at,finished_at,success,duration_ms,result_schema_version,result_json,result_hash,condition_state,event_type,notification_events_json,notification_event_count,trend_triggered,trend_recovered,error_code,error_message)
	VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, record.ID, record.MonitorID, record.ModuleType, record.ModuleVersion, unixMicros(record.StartedAt), unixMicros(record.FinishedAt), boolInt(record.Success), record.DurationMS, record.ResultSchemaVersion, jsonString(record.Result), record.ResultHash, record.ConditionState, record.EventType, jsonString(notificationEventMetadata(record.NotificationEvents)), eventCount, boolInt(trendTriggered), boolInt(trendRecovered), record.ErrorCode, record.ErrorMessage)
	return err
}

func addPendingNotificationDeliveriesTx(ctx context.Context, tx *sql.Tx, record core.MonitorRecord) error {
	now := unixMicros(time.Now())
	for _, event := range record.NotificationEvents {
		payload := jsonString(notificationEventMetadata([]core.RecordNotificationEvent{event})[0])
		for channelID := range event.Deliveries {
			if _, err := tx.ExecContext(ctx, `INSERT INTO notification_deliveries
(id,event_id,source,event_type,status_item_id,trend_rule_id,channel_id,notifier_type,monitor_id,record_id,title,content,payload_json,status,attempts,message,is_read,created_at,updated_at,delivered_at,read_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, core.NewID(), event.ID, event.Source, event.EventType, event.StatusItemID, event.TrendRuleID, channelID, "unknown", record.MonitorID, record.ID, "", event.Summary, payload, "pending", 0, "", 0, now, now, nil, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
