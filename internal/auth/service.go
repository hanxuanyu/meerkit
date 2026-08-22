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
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/argon2"
	"meerkit/internal/store"
)

const CookieName = "meerkit_session"
const minimumAccessKeyLength = 12

const (
	minimumHashMemory  = 8 * 1024
	maximumHashMemory  = 256 * 1024
	maximumHashTime    = 10
	maximumHashThreads = 16
	minimumHashBytes   = 16
	maximumHashBytes   = 64
)

type Session struct {
	Token     string    `json:"-"`
	CSRFToken string    `json:"csrf_token"`
	ExpiresAt time.Time `json:"expires_at"`
}
type Service struct {
	store          store.AuthRepository
	ttlNanos       atomic.Int64
	tokenKey       []byte
	allowTokenCopy atomic.Bool
	pendingMu      sync.Mutex
	pendingMCP     *TokenSecret
}

func NewService(database store.AuthRepository, ttl time.Duration) *Service {
	key, _ := randomBytes(32)
	return newService(database, ttl, key, false)
}

type ServiceOptions struct {
	MasterKeyFile  string
	AllowTokenCopy bool
}

func NewServiceWithOptions(database store.AuthRepository, ttl time.Duration, options ServiceOptions) (*Service, error) {
	key, err := loadOrCreateMasterKey(options.MasterKeyFile)
	if err != nil {
		return nil, err
	}
	return newService(database, ttl, key, options.AllowTokenCopy), nil
}

func newService(database store.AuthRepository, ttl time.Duration, key []byte, allowCopy bool) *Service {
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	service := &Service{store: database, tokenKey: key}
	service.allowTokenCopy.Store(allowCopy)
	service.ttlNanos.Store(int64(ttl))
	return service
}

func (s *Service) SetSessionTTL(ttl time.Duration) {
	if ttl > 0 {
		s.ttlNanos.Store(int64(ttl))
	}
}

func (s *Service) SessionTTL() time.Duration {
	value := time.Duration(s.ttlNanos.Load())
	if value <= 0 {
		return 30 * 24 * time.Hour
	}
	return value
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
	return s.replaceKey(ctx, accessKey)
}

func (s *Service) ChangeKey(ctx context.Context, currentAccessKey, accessKey string) error {
	encoded, err := s.store.AdminKeyHash(ctx)
	if err != nil {
		return fmt.Errorf("administrator is not initialized")
	}
	valid, err := verifyAccessKey(currentAccessKey, encoded)
	if err != nil || !valid {
		return fmt.Errorf("invalid current administrator access key")
	}
	return s.replaceKey(ctx, accessKey)
}

func (s *Service) replaceKey(ctx context.Context, accessKey string) error {
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
	ttl := s.SessionTTL()
	if value.ExpiresAt.Sub(now) < ttl/2 {
		value.ExpiresAt = now.Add(ttl)
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
	value := Session{Token: token, CSRFToken: csrf, ExpiresAt: now.Add(s.SessionTTL())}
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
	memoryValue, timeValue, threadsValue, salt, expected, err := parseAccessKeyHash(encoded)
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(value), salt, timeValue, memoryValue, threadsValue, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

// ValidateAccessKeyHash checks that an encoded administrator key uses the
// format and parameters understood by this authentication implementation.
// It does not verify a plaintext key and is intended for configuration import
// validation.
func ValidateAccessKeyHash(encoded string) error {
	_, _, _, _, _, err := parseAccessKeyHash(encoded)
	return err
}

func parseAccessKeyHash(encoded string) (uint32, uint32, uint8, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid access key hash")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return 0, 0, 0, nil, nil, fmt.Errorf("unsupported Argon2 version")
	}
	parameters := strings.Split(parts[3], ",")
	if len(parameters) != 3 {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid Argon2 parameters")
	}
	memoryValue, err := parseHashParameter(parameters[0], "m=", 32)
	if err != nil || memoryValue < minimumHashMemory || memoryValue > maximumHashMemory {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid Argon2 memory parameter")
	}
	timeValue, err := parseHashParameter(parameters[1], "t=", 32)
	if err != nil || timeValue == 0 || timeValue > maximumHashTime {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid Argon2 time parameter")
	}
	threadsValue, err := parseHashParameter(parameters[2], "p=", 8)
	if err != nil || threadsValue == 0 || threadsValue > maximumHashThreads {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid Argon2 threads parameter")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < minimumHashBytes || len(salt) > maximumHashBytes {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid Argon2 salt")
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) < minimumHashBytes || len(expected) > maximumHashBytes {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid Argon2 key")
	}
	return uint32(memoryValue), uint32(timeValue), uint8(threadsValue), salt, expected, nil
}

func parseHashParameter(value, prefix string, bits int) (uint64, error) {
	if !strings.HasPrefix(value, prefix) || strings.TrimPrefix(value, prefix) == "" {
		return 0, fmt.Errorf("invalid parameter")
	}
	return strconv.ParseUint(strings.TrimPrefix(value, prefix), 10, bits)
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
