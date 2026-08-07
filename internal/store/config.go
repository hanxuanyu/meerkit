package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrSystemConfigVersionConflict = errors.New("system config version conflict")

type SystemConfig struct {
	Type      string
	Data      json.RawMessage
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *Store) GetSystemConfig(ctx context.Context, configType string) (SystemConfig, error) {
	var value SystemConfig
	var data string
	var created, updated string
	err := s.db.QueryRowContext(ctx, `SELECT config_type,data_json,version,created_at,updated_at FROM system_configs WHERE config_type=?`, configType).Scan(&value.Type, &data, &value.Version, &created, &updated)
	if err != nil {
		return value, err
	}
	value.Data = json.RawMessage(data)
	value.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	value.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return value, nil
}

func (s *Store) ListSystemConfigs(ctx context.Context) ([]SystemConfig, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT config_type,data_json,version,created_at,updated_at FROM system_configs ORDER BY config_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]SystemConfig, 0)
	for rows.Next() {
		var value SystemConfig
		var data string
		var created, updated string
		if err := rows.Scan(&value.Type, &data, &value.Version, &created, &updated); err != nil {
			return nil, err
		}
		value.Data = json.RawMessage(data)
		value.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		value.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *Store) EnsureSystemConfig(ctx context.Context, configType string, data any) (SystemConfig, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return SystemConfig{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO system_configs(config_type,data_json,version,created_at,updated_at) VALUES(?,?,1,?,?)`, configType, string(encoded), now, now)
	if err != nil {
		return SystemConfig{}, err
	}
	return s.GetSystemConfig(ctx, configType)
}

func (s *Store) UpdateSystemConfig(ctx context.Context, configType string, data json.RawMessage, expectedVersion int) (SystemConfig, error) {
	s.systemConfigMu.Lock()
	defer s.systemConfigMu.Unlock()
	if len(data) == 0 || !json.Valid(data) {
		return SystemConfig{}, errors.New("system config data must be valid JSON")
	}
	if expectedVersion < 1 {
		return SystemConfig{}, errors.New("system config version must be positive")
	}
	var err error
	if configType == systemAuthConfigType {
		data, err = s.preserveAuthKey(ctx, data)
		if err != nil {
			return SystemConfig{}, err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `UPDATE system_configs SET data_json=?,version=version+1,updated_at=? WHERE config_type=? AND version=?`, string(data), now, configType, expectedVersion)
	if err != nil {
		return SystemConfig{}, err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return SystemConfig{}, err
	}
	if updated == 0 {
		if _, getErr := s.GetSystemConfig(ctx, configType); errors.Is(getErr, sql.ErrNoRows) {
			return SystemConfig{}, fmt.Errorf("system config %q not found", configType)
		}
		return SystemConfig{}, ErrSystemConfigVersionConflict
	}
	return s.GetSystemConfig(ctx, configType)
}

func (s *Store) preserveAuthKey(ctx context.Context, data json.RawMessage) (json.RawMessage, error) {
	var incoming systemAuthConfig
	if err := json.Unmarshal(data, &incoming); err != nil {
		return nil, err
	}
	if incoming.AdminKeyHash != "" {
		return data, nil
	}
	var existingJSON string
	if err := s.db.QueryRowContext(ctx, `SELECT data_json FROM system_configs WHERE config_type=?`, systemAuthConfigType).Scan(&existingJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return data, nil
		}
		return nil, err
	}
	var existing systemAuthConfig
	if err := json.Unmarshal([]byte(existingJSON), &existing); err != nil {
		return nil, err
	}
	incoming.AdminKeyHash = existing.AdminKeyHash
	return json.Marshal(incoming)
}
