package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
	"meerkit/internal/store"
)

const CookieName = "meerkit_session"
const minimumAccessKeyLength = 12

type Session struct {
	Token     string    `json:"-"`
	CSRFToken string    `json:"csrf_token"`
	ExpiresAt time.Time `json:"expires_at"`
}
type Service struct {
	store *store.Store
	ttl   time.Duration
}

func NewService(database *store.Store, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	return &Service{store: database, ttl: ttl}
}
func (s *Service) Initialized(ctx context.Context) (bool, error) {
	_, err := s.store.AdminKeyHash(ctx)
	if store.IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}
func (s *Service) Setup(ctx context.Context, accessKey string) (Session, error) {
	initialized, err := s.Initialized(ctx)
	if err != nil {
		return Session{}, err
	}
	if initialized {
		return Session{}, fmt.Errorf("administrator is already initialized")
	}
	return s.setKeyAndSession(ctx, accessKey, false)
}
func (s *Service) ResetKey(ctx context.Context, accessKey string) error {
	if err := validateAccessKey(accessKey); err != nil {
		return err
	}
	encoded, err := hashAccessKey(accessKey)
	if err != nil {
		return err
	}
	return s.store.SetAdminKeyHash(ctx, encoded, true)
}
func (s *Service) Login(ctx context.Context, accessKey string) (Session, error) {
	encoded, err := s.store.AdminKeyHash(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("administrator is not initialized")
	}
	valid, err := verifyAccessKey(accessKey, encoded)
	if err != nil || !valid {
		return Session{}, fmt.Errorf("invalid administrator access key")
	}
	return s.createSession(ctx)
}
func (s *Service) Authenticate(ctx context.Context, token string) (store.AdminSession, error) {
	if token == "" {
		return store.AdminSession{}, fmt.Errorf("authentication required")
	}
	now := time.Now().UTC()
	tokenHash := digest(token)
	value, err := s.store.GetAdminSession(ctx, tokenHash)
	if err != nil || !value.ExpiresAt.After(now) {
		if err == nil {
			_ = s.store.DeleteAdminSession(ctx, tokenHash)
		}
		return store.AdminSession{}, fmt.Errorf("session is invalid or expired")
	}
	if value.ExpiresAt.Sub(now) < s.ttl/2 {
		value.ExpiresAt = now.Add(s.ttl)
		value.LastSeenAt = now
		if err := s.store.RefreshAdminSession(ctx, tokenHash, value.ExpiresAt, now); err != nil {
			return store.AdminSession{}, err
		}
	}
	return value, nil
}
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteAdminSession(ctx, digest(token))
}
func (s *Service) setKeyAndSession(ctx context.Context, accessKey string, reset bool) (Session, error) {
	if err := validateAccessKey(accessKey); err != nil {
		return Session{}, err
	}
	encoded, err := hashAccessKey(accessKey)
	if err != nil {
		return Session{}, err
	}
	if err := s.store.SetAdminKeyHash(ctx, encoded, reset); err != nil {
		return Session{}, err
	}
	return s.createSession(ctx)
}
func (s *Service) createSession(ctx context.Context) (Session, error) {
	token, err := randomToken(32)
	if err != nil {
		return Session{}, err
	}
	csrf, err := randomToken(24)
	if err != nil {
		return Session{}, err
	}
	now := time.Now().UTC()
	value := Session{Token: token, CSRFToken: csrf, ExpiresAt: now.Add(s.ttl)}
	if err := s.store.CreateAdminSession(ctx, store.AdminSession{TokenHash: digest(token), CSRFToken: csrf, ExpiresAt: value.ExpiresAt, LastSeenAt: now, CreatedAt: now}); err != nil {
		return Session{}, err
	}
	return value, nil
}
func validateAccessKey(value string) error {
	if len(strings.TrimSpace(value)) < minimumAccessKeyLength {
		return fmt.Errorf("administrator access key must contain at least %d characters", minimumAccessKeyLength)
	}
	return nil
}
func hashAccessKey(value string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(value), salt, 3, 64*1024, 4, 32)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, 64*1024, 3, 4, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}
func verifyAccessKey(value, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("invalid access key hash")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, fmt.Errorf("unsupported Argon2 version")
	}
	parameters := strings.Split(parts[3], ",")
	if len(parameters) != 3 {
		return false, fmt.Errorf("invalid Argon2 parameters")
	}
	memoryValue, _ := strconv.ParseUint(strings.TrimPrefix(parameters[0], "m="), 10, 32)
	timeValue, _ := strconv.ParseUint(strings.TrimPrefix(parameters[1], "t="), 10, 32)
	threadsValue, _ := strconv.ParseUint(strings.TrimPrefix(parameters[2], "p="), 10, 8)
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(value), salt, uint32(timeValue), uint32(memoryValue), uint8(threadsValue), uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}
func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
func digest(value string) string {
	result := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", result[:])
}
