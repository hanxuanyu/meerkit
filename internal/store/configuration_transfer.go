package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"meerkit/internal/core"
)

func (s *Store) ImportConfiguration(ctx context.Context, input ConfigurationImport) (ConfigurationImportResult, error) {
	s.systemConfigMu.Lock()
	defer s.systemConfigMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ConfigurationImportResult{}, err
	}
	defer tx.Rollback()

	versions, err := importRuntimeConfigTx(ctx, tx, input.Runtime, input.AdminKeyHash)
	if err != nil {
		return ConfigurationImportResult{}, err
	}
	if input.Replace {
		if err := clearTransferableConfigurationTx(ctx, tx, input.Monitors, input.NotificationChannels); err != nil {
			return ConfigurationImportResult{}, err
		}
	}
	for _, channel := range input.NotificationChannels {
		if channel.ID == core.BuiltInNotificationChannelID || channel.BuiltIn {
			continue
		}
		if err := upsertNotificationChannelTx(ctx, tx, channel); err != nil {
			return ConfigurationImportResult{}, fmt.Errorf("import notification channel %q: %w", channel.ID, err)
		}
	}
	for _, monitor := range input.Monitors {
		if err := upsertMonitorTx(ctx, tx, monitor); err != nil {
			return ConfigurationImportResult{}, fmt.Errorf("import monitor %q: %w", monitor.ID, err)
		}
	}
	for _, item := range input.StatusBoardItems {
		if err := upsertStatusBoardItemTx(ctx, tx, item); err != nil {
			return ConfigurationImportResult{}, fmt.Errorf("import status board item %q: %w", item.ID, err)
		}
	}
	for _, share := range input.StatusBoardShares {
		if err := upsertStatusBoardShareTx(ctx, tx, share); err != nil {
			return ConfigurationImportResult{}, fmt.Errorf("import status board share %q: %w", share.ID, err)
		}
	}
	if input.AdminKeyHash != "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM admin_sessions`); err != nil {
			return ConfigurationImportResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ConfigurationImportResult{}, err
	}
	return ConfigurationImportResult{Versions: versions}, nil
}

func importRuntimeConfigTx(ctx context.Context, tx *sql.Tx, values map[string]json.RawMessage, adminKeyHash string) (map[string]int, error) {
	versions := make(map[string]int, len(values))
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for configType, data := range values {
		if len(data) == 0 || !json.Valid(data) {
			return nil, fmt.Errorf("runtime config %q is not valid JSON", configType)
		}
		if configType == systemAuthConfigType {
			var auth systemAuthConfig
			if err := json.Unmarshal(data, &auth); err != nil {
				return nil, err
			}
			if adminKeyHash != "" {
				auth.AdminKeyHash = adminKeyHash
			} else {
				var existingJSON string
				if err := tx.QueryRowContext(ctx, `SELECT data_json FROM system_configs WHERE config_type=?`, configType).Scan(&existingJSON); err != nil {
					return nil, err
				}
				var existing systemAuthConfig
				if err := json.Unmarshal([]byte(existingJSON), &existing); err != nil {
					return nil, err
				}
				auth.AdminKeyHash = existing.AdminKeyHash
			}
			encoded, err := json.Marshal(auth)
			if err != nil {
				return nil, err
			}
			data = encoded
		}
		result, err := tx.ExecContext(ctx, `UPDATE system_configs SET data_json=?,version=version+1,updated_at=? WHERE config_type=?`, string(data), now, configType)
		if err != nil {
			return nil, err
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("runtime config %q was not updated", configType)
		}
		var version int
		if err := tx.QueryRowContext(ctx, `SELECT version FROM system_configs WHERE config_type=?`, configType).Scan(&version); err != nil {
			return nil, err
		}
		versions[configType] = version
	}
	return versions, nil
}

func clearTransferableConfigurationTx(ctx context.Context, tx *sql.Tx, monitors []core.Monitor, channels []core.NotificationChannel) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM status_board_shares`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM status_board_items`); err != nil {
		return err
	}
	monitorIDs := make(map[string]struct{}, len(monitors))
	for _, monitor := range monitors {
		monitorIDs[monitor.ID] = struct{}{}
	}
	if err := deleteRowsNotInImportTx(ctx, tx, "monitors", "", monitorIDs); err != nil {
		return err
	}
	channelIDs := make(map[string]struct{}, len(channels))
	for _, channel := range channels {
		channelIDs[channel.ID] = struct{}{}
	}
	return deleteRowsNotInImportTx(ctx, tx, "notification_channels", "builtin_key IS NULL", channelIDs)
}

func deleteRowsNotInImportTx(ctx context.Context, tx *sql.Tx, table, where string, retained map[string]struct{}) error {
	query := `SELECT id FROM ` + table
	if where != "" {
		query += ` WHERE ` + where
	}
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	var stale []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		if _, keep := retained[id]; !keep {
			stale = append(stale, id)
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range stale {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE id=?`, id); err != nil {
			return err
		}
	}
	return nil
}

func rowExistsTx(ctx context.Context, tx *sql.Tx, table, id string) (bool, error) {
	var value int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM `+table+` WHERE id=?`, id).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func upsertNotificationChannelTx(ctx context.Context, tx *sql.Tx, channel core.NotificationChannel) error {
	model := notificationChannelFromDomain(channel)
	exists, err := rowExistsTx(ctx, tx, "notification_channels", channel.ID)
	if err != nil {
		return err
	}
	if exists {
		_, err = tx.ExecContext(ctx, `UPDATE notification_channels SET builtin_key=NULL,name=?,notifier_type=?,enabled=?,config_json=?,created_at=?,updated_at=? WHERE id=?`, model.Name, model.NotifierType, model.Enabled, model.ConfigJSON, model.CreatedAt, model.UpdatedAt, model.ID)
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO notification_channels(id,builtin_key,name,notifier_type,enabled,config_json,created_at,updated_at) VALUES(?,NULL,?,?,?,?,?,?)`, model.ID, model.Name, model.NotifierType, model.Enabled, model.ConfigJSON, model.CreatedAt, model.UpdatedAt)
	return err
}

func upsertMonitorTx(ctx context.Context, tx *sql.Tx, monitor core.Monitor) error {
	model := monitorFromDomain(monitor)
	exists, err := rowExistsTx(ctx, tx, "monitors", monitor.ID)
	if err != nil {
		return err
	}
	values := []any{model.Name, model.ModuleType, model.ModuleVersion, model.ModuleConfigVersion, model.SchedulesJSON, model.Enabled, model.ModuleConfigJSON, model.ConditionConfigJSON, model.NotificationChannelIDsJSON, model.RuntimeStateJSON, model.ConditionActive, model.LastSuccess, model.CreatedAt, model.UpdatedAt, model.ID}
	if exists {
		_, err = tx.ExecContext(ctx, `UPDATE monitors SET name=?,module_type=?,module_version=?,module_config_version=?,schedules_json=?,enabled=?,module_config_json=?,condition_config_json=?,notification_channel_ids_json=?,runtime_state_json=?,condition_active=?,last_success=?,created_at=?,updated_at=? WHERE id=?`, values...)
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO monitors(id,name,module_type,module_version,module_config_version,schedules_json,enabled,module_config_json,condition_config_json,notification_channel_ids_json,runtime_state_json,condition_active,last_success,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, model.ID, model.Name, model.ModuleType, model.ModuleVersion, model.ModuleConfigVersion, model.SchedulesJSON, model.Enabled, model.ModuleConfigJSON, model.ConditionConfigJSON, model.NotificationChannelIDsJSON, model.RuntimeStateJSON, model.ConditionActive, model.LastSuccess, model.CreatedAt, model.UpdatedAt)
	return err
}

func upsertStatusBoardItemTx(ctx context.Context, tx *sql.Tx, item core.StatusBoardItem) error {
	model := statusBoardItemFromDomain(item)
	exists, err := rowExistsTx(ctx, tx, "status_board_items", item.ID)
	if err != nil {
		return err
	}
	values := []any{model.Name, model.MonitorID, model.Enabled, model.SourceJSON, model.Invert, model.ThresholdsJSON, model.HistoryLimit, model.TrendRulesJSON, model.NotificationChannelIDsJSON, model.RuntimeStateJSON, model.CreatedAt, model.UpdatedAt, model.ID}
	if exists {
		_, err = tx.ExecContext(ctx, `UPDATE status_board_items SET name=?,monitor_id=?,enabled=?,source_json=?,invert=?,thresholds_json=?,history_limit=?,trend_rules_json=?,notification_channel_ids_json=?,runtime_state_json=?,created_at=?,updated_at=? WHERE id=?`, values...)
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO status_board_items(id,name,monitor_id,enabled,source_json,invert,thresholds_json,history_limit,trend_rules_json,notification_channel_ids_json,runtime_state_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, model.ID, model.Name, model.MonitorID, model.Enabled, model.SourceJSON, model.Invert, model.ThresholdsJSON, model.HistoryLimit, model.TrendRulesJSON, model.NotificationChannelIDsJSON, model.RuntimeStateJSON, model.CreatedAt, model.UpdatedAt)
	return err
}

func upsertStatusBoardShareTx(ctx context.Context, tx *sql.Tx, share core.StatusBoardShare) error {
	model := statusBoardShareModel{
		ID:             share.ID,
		Name:           share.Name,
		Token:          share.Token,
		MonitorIDsJSON: jsonString(share.MonitorIDs),
		ItemIDsJSON:    jsonString(share.ItemIDs),
		Active:         share.Active,
		CreatedAt:      timestamp(share.CreatedAt),
	}
	exists, err := rowExistsTx(ctx, tx, "status_board_shares", share.ID)
	if err != nil {
		return err
	}
	if exists {
		_, err = tx.ExecContext(ctx, `UPDATE status_board_shares SET name=?,token=?,monitor_ids_json=?,item_ids_json=?,active=?,created_at=? WHERE id=?`, model.Name, model.Token, model.MonitorIDsJSON, model.ItemIDsJSON, model.Active, model.CreatedAt, model.ID)
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO status_board_shares(id,name,token,monitor_ids_json,item_ids_json,active,created_at) VALUES(?,?,?,?,?,?,?)`, model.ID, model.Name, model.Token, model.MonitorIDsJSON, model.ItemIDsJSON, model.Active, model.CreatedAt)
	return err
}
