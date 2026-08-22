package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

const systemAuthConfigType = "auth"

type systemAuthConfig struct {
	SessionTTL   string `json:"session_ttl,omitempty"`
	AdminKeyHash string `json:"admin_key_hash,omitempty"`
}

type AdminSession struct {
	TokenHash  string
	CSRFToken  string
	ExpiresAt  time.Time
	LastSeenAt time.Time
	CreatedAt  time.Time
}

type APIToken struct {
	ID               string
	Name             string
	Type             string
	Scopes           []string
	TokenHash        string
	SecretCiphertext string
	SecretNonce      string
	TokenHint        string
	ExpiresAt        *time.Time
	RevokedAt        *time.Time
	LastUsedAt       *time.Time
	CreatedAt        time.Time
}

func (s *Store) CreateAPIToken(ctx context.Context, value APIToken) error {
	scopes, err := json.Marshal(value.Scopes)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO api_tokens(id,name,type,scopes_json,token_hash,secret_ciphertext,secret_nonce,token_hint,expires_at,revoked_at,last_used_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, value.ID, value.Name, value.Type, string(scopes), value.TokenHash, value.SecretCiphertext, value.SecretNonce, value.TokenHint, optionalTimestamp(value.ExpiresAt), optionalTimestamp(value.RevokedAt), optionalTimestamp(value.LastUsedAt), value.CreatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListAPITokens(ctx context.Context) ([]APIToken, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,type,scopes_json,token_hash,secret_ciphertext,secret_nonce,token_hint,expires_at,revoked_at,last_used_at,created_at FROM api_tokens ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]APIToken, 0)
	for rows.Next() {
		value, err := scanAPIToken(rows.Scan)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *Store) GetAPIToken(ctx context.Context, id string) (APIToken, error) {
	return s.getAPIToken(ctx, `WHERE id=?`, id)
}

func (s *Store) GetAPITokenByHash(ctx context.Context, hash string) (APIToken, error) {
	return s.getAPIToken(ctx, `WHERE token_hash=?`, hash)
}

func (s *Store) getAPIToken(ctx context.Context, where string, arg any) (APIToken, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,type,scopes_json,token_hash,secret_ciphertext,secret_nonce,token_hint,expires_at,revoked_at,last_used_at,created_at FROM api_tokens `+where, arg)
	return scanAPIToken(row.Scan)
}

func (s *Store) RevokeAPIToken(ctx context.Context, id string, revoked time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE api_tokens SET revoked_at=? WHERE id=? AND revoked_at IS NULL`, revoked.UTC().Format(time.RFC3339Nano), id)
	return err
}

func (s *Store) RestoreAPIToken(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE api_tokens SET revoked_at=NULL WHERE id=? AND revoked_at IS NOT NULL`, id)
	return err
}

func (s *Store) DeleteAPIToken(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id=?`, id)
	return err
}

func (s *Store) TouchAPIToken(ctx context.Context, hash string, seen time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at=? WHERE token_hash=?`, seen.UTC().Format(time.RFC3339Nano), hash)
	return err
}

type scanFunc func(...any) error

func scanAPIToken(scan scanFunc) (APIToken, error) {
	var value APIToken
	var scopes, created string
	var expires, revoked, lastUsed sql.NullString
	if err := scan(&value.ID, &value.Name, &value.Type, &scopes, &value.TokenHash, &value.SecretCiphertext, &value.SecretNonce, &value.TokenHint, &expires, &revoked, &lastUsed, &created); err != nil {
		return value, err
	}
	if err := json.Unmarshal([]byte(scopes), &value.Scopes); err != nil {
		return value, err
	}
	value.ExpiresAt = parseOptionalTimestamp(expires.String)
	value.RevokedAt = parseOptionalTimestamp(revoked.String)
	value.LastUsedAt = parseOptionalTimestamp(lastUsed.String)
	value.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return value, nil
}

func optionalTimestamp(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}
func parseOptionalTimestamp(value string) *time.Time {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	return &parsed
}

func (s *Store) AdminKeyHash(ctx context.Context) (string, error) {
	value, err := s.GetSystemConfig(ctx, systemAuthConfigType)
	if err != nil {
		return "", err
	}
	var config systemAuthConfig
	if err := json.Unmarshal(value.Data, &config); err != nil {
		return "", err
	}
	if config.AdminKeyHash == "" {
		return "", sql.ErrNoRows
	}
	return config.AdminKeyHash, nil
}
func (s *Store) SetAdminKeyHash(ctx context.Context, value string, resetSessions bool) error {
	s.systemConfigMu.Lock()
	defer s.systemConfigMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var existingJSON string
	config := systemAuthConfig{}
	err = tx.QueryRowContext(ctx, `SELECT data_json FROM system_configs WHERE config_type=?`, systemAuthConfigType).Scan(&existingJSON)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	exists := err == nil
	if exists && existingJSON != "" {
		if err := json.Unmarshal([]byte(existingJSON), &config); err != nil {
			return err
		}
	}
	config.AdminKeyHash = value
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if exists {
		// The key is managed outside the runtime-config editor, so rotating it
		// must not invalidate an in-flight auth.session_ttl version.
		if _, err := tx.ExecContext(ctx, `UPDATE system_configs SET data_json=?,updated_at=? WHERE config_type=?`, string(data), now, systemAuthConfigType); err != nil {
			return err
		}
	} else if _, err := tx.ExecContext(ctx, `INSERT INTO system_configs(config_type,data_json,version,created_at,updated_at) VALUES(?,?,1,?,?)`, systemAuthConfigType, string(data), now, now); err != nil {
		return err
	}
	if resetSessions {
		if _, err := tx.ExecContext(ctx, `DELETE FROM admin_sessions`); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) CreateAdminSession(ctx context.Context, value AdminSession) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO admin_sessions(token_hash,csrf_token,expires_at,last_seen_at,created_at) VALUES(?,?,?,?,?)`, value.TokenHash, value.CSRFToken, value.ExpiresAt.UTC().Format(time.RFC3339Nano), value.LastSeenAt.UTC().Format(time.RFC3339Nano), value.CreatedAt.UTC().Format(time.RFC3339Nano))
	return err
}
func (s *Store) GetAdminSession(ctx context.Context, tokenHash string) (AdminSession, error) {
	var value AdminSession
	var expires, seen, created string
	err := s.db.QueryRowContext(ctx, `SELECT token_hash,csrf_token,expires_at,last_seen_at,created_at FROM admin_sessions WHERE token_hash=?`, tokenHash).Scan(&value.TokenHash, &value.CSRFToken, &expires, &seen, &created)
	if err != nil {
		return value, err
	}
	value.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expires)
	value.LastSeenAt, _ = time.Parse(time.RFC3339Nano, seen)
	value.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return value, nil
}
func (s *Store) RefreshAdminSession(ctx context.Context, tokenHash string, expires, seen time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE admin_sessions SET expires_at=?,last_seen_at=? WHERE token_hash=?`, expires.UTC().Format(time.RFC3339Nano), seen.UTC().Format(time.RFC3339Nano), tokenHash)
	return err
}
func (s *Store) DeleteAdminSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE token_hash=?`, tokenHash)
	return err
}
func (s *Store) DeleteExpiredAdminSessions(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE expires_at<=?`, now.UTC().Format(time.RFC3339Nano))
	return err
}
func IsNoRows(err error) bool { return err == sql.ErrNoRows }
