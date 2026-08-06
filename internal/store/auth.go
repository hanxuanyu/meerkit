package store

import (
	"context"
	"database/sql"
	"time"
)

type AdminSession struct {
	TokenHash  string
	CSRFToken  string
	ExpiresAt  time.Time
	LastSeenAt time.Time
	CreatedAt  time.Time
}

func (s *Store) AdminKeyHash(ctx context.Context) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT key_hash FROM admin_credentials WHERE id=1`).Scan(&value)
	return value, err
}
func (s *Store) SetAdminKeyHash(ctx context.Context, value string, resetSessions bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_credentials(id,key_hash,created_at,updated_at) VALUES(1,?,?,?) ON CONFLICT(id) DO UPDATE SET key_hash=excluded.key_hash,updated_at=excluded.updated_at`, value, now, now); err != nil {
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
