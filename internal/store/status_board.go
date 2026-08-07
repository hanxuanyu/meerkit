package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"meerkit/internal/core"
)

const statusItemColumns = `id,name,monitor_id,enabled,source_json,invert,thresholds_json,history_limit,trend_rules_json,notification_channel_ids_json,runtime_state_json,created_at,updated_at`

func (s *Store) CreateStatusBoardItem(ctx context.Context, item core.StatusBoardItem) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO status_board_items (`+statusItemColumns+`) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		item.ID, item.Name, item.MonitorID, boolInt(item.Enabled), jsonString(item.Source), boolInt(item.Invert), jsonString(item.Thresholds), item.HistoryLimit,
		jsonString(item.TrendRules), jsonString(item.NotificationChannelIDs), jsonString(item.RuntimeState), item.CreatedAt.UTC().Format(time.RFC3339Nano), item.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) UpdateStatusBoardItem(ctx context.Context, item core.StatusBoardItem) error {
	_, err := s.db.ExecContext(ctx, `UPDATE status_board_items SET name=?,monitor_id=?,enabled=?,source_json=?,invert=?,thresholds_json=?,history_limit=?,trend_rules_json=?,notification_channel_ids_json=?,runtime_state_json=?,updated_at=? WHERE id=?`,
		item.Name, item.MonitorID, boolInt(item.Enabled), jsonString(item.Source), boolInt(item.Invert), jsonString(item.Thresholds), item.HistoryLimit,
		jsonString(item.TrendRules), jsonString(item.NotificationChannelIDs), jsonString(item.RuntimeState), item.UpdatedAt.UTC().Format(time.RFC3339Nano), item.ID)
	return err
}

func (s *Store) GetStatusBoardItem(ctx context.Context, id string) (core.StatusBoardItem, error) {
	return scanStatusBoardItem(s.db.QueryRowContext(ctx, `SELECT `+statusItemColumns+` FROM status_board_items WHERE id=?`, id))
}

func (s *Store) ListStatusBoardItems(ctx context.Context) ([]core.StatusBoardItem, error) {
	return s.listStatusBoardItems(ctx, `SELECT `+statusItemColumns+` FROM status_board_items ORDER BY created_at`, nil)
}

func (s *Store) ListStatusBoardItemsByMonitor(ctx context.Context, monitorID string) ([]core.StatusBoardItem, error) {
	return s.listStatusBoardItems(ctx, `SELECT `+statusItemColumns+` FROM status_board_items WHERE monitor_id=? ORDER BY created_at`, []any{monitorID})
}

func (s *Store) listStatusBoardItems(ctx context.Context, query string, args []any) ([]core.StatusBoardItem, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []core.StatusBoardItem{}
	for rows.Next() {
		item, scanErr := scanStatusBoardItem(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) DeleteStatusBoardItem(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM status_board_items WHERE id=?`, id)
	return err
}

func (s *Store) ResetStatusBoardRuntimeByMonitor(ctx context.Context, monitorID string, at time.Time) error {
	state := core.StatusItemRuntimeState{EvaluationStartedAt: at.UTC(), Rules: map[string]core.TrendRuleState{}}
	_, err := s.db.ExecContext(ctx, `UPDATE status_board_items SET runtime_state_json=?,updated_at=? WHERE monitor_id=?`, jsonString(state), at.UTC().Format(time.RFC3339Nano), monitorID)
	return err
}

func scanStatusBoardItem(scanner interface{ Scan(...any) error }) (core.StatusBoardItem, error) {
	var item core.StatusBoardItem
	var enabled, invert int
	var sourceJSON, thresholdsJSON, trendsJSON, channelsJSON, runtimeJSON string
	var createdAt, updatedAt string
	if err := scanner.Scan(&item.ID, &item.Name, &item.MonitorID, &enabled, &sourceJSON, &invert, &thresholdsJSON, &item.HistoryLimit, &trendsJSON, &channelsJSON, &runtimeJSON, &createdAt, &updatedAt); err != nil {
		return item, err
	}
	item.Enabled = enabled == 1
	item.Invert = invert == 1
	if err := json.Unmarshal([]byte(sourceJSON), &item.Source); err != nil {
		return item, err
	}
	_ = json.Unmarshal([]byte(thresholdsJSON), &item.Thresholds)
	_ = json.Unmarshal([]byte(trendsJSON), &item.TrendRules)
	_ = json.Unmarshal([]byte(channelsJSON), &item.NotificationChannelIDs)
	_ = json.Unmarshal([]byte(runtimeJSON), &item.RuntimeState)
	if item.RuntimeState.Rules == nil {
		item.RuntimeState.Rules = map[string]core.TrendRuleState{}
	}
	item.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
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
	monitorResult, err := tx.ExecContext(ctx, `UPDATE monitors SET runtime_state_json=?,updated_at=? WHERE id=?`, jsonString(monitorState), time.Now().UTC().Format(time.RFC3339Nano), monitorID)
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
	_, err := tx.ExecContext(ctx, `INSERT INTO monitor_records
(id,monitor_id,module_type,module_version,started_at,finished_at,success,duration_ms,result_schema_version,result_json,result_hash,condition_state,event_type,notification_events_json,error_code,error_message)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, record.ID, record.MonitorID, record.ModuleType, record.ModuleVersion, record.StartedAt.UTC().Format(time.RFC3339Nano), record.FinishedAt.UTC().Format(time.RFC3339Nano), boolInt(record.Success), record.DurationMS, record.ResultSchemaVersion, jsonString(record.Result), record.ResultHash, record.ConditionState, record.EventType, jsonString(record.NotificationEvents), record.ErrorCode, record.ErrorMessage)
	return err
}
