package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"meerkit/internal/store"
)

const (
	TokenTypeREST = "rest"
	TokenTypeMCP  = "mcp"
	ScopeAPIRead  = "api:read"
	ScopeAPIWrite = "api:write"
	ScopeMCP      = "mcp:browser"
)

type TokenInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Scopes     []string   `json:"scopes"`
	TokenHint  string     `json:"token_hint"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type TokenPrincipal struct {
	TokenID string
	Type    string
	Scopes  []string
}

type TokenSecret struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Token string `json:"token"`
}

var ErrTokenNameExists = errors.New("token name already exists")

func (s *Service) EnsureMCPToken(ctx context.Context) (TokenInfo, error) {
	values, err := s.store.ListAPITokens(ctx)
	if err != nil {
		return TokenInfo{}, err
	}
	now := time.Now().UTC()
	for _, value := range values {
		if value.Type == TokenTypeMCP && value.RevokedAt == nil && (value.ExpiresAt == nil || value.ExpiresAt.After(now)) {
			return tokenInfo(value), nil
		}
	}
	usedNames := make(map[string]struct{}, len(values))
	for _, value := range values {
		usedNames[strings.ToLower(strings.TrimSpace(value.Name))] = struct{}{}
	}
	name := "MCP Browser"
	for suffix := 2; ; suffix++ {
		if _, exists := usedNames[strings.ToLower(name)]; !exists {
			break
		}
		name = fmt.Sprintf("MCP Browser %d", suffix)
	}
	info, secret, err := s.createToken(ctx, name, TokenTypeMCP, []string{ScopeMCP}, nil)
	if err != nil {
		return TokenInfo{}, err
	}
	s.pendingMu.Lock()
	s.pendingMCP = &secret
	s.pendingMu.Unlock()
	return info, nil
}

func (s *Service) ConsumePendingMCPToken() *TokenSecret {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	value := s.pendingMCP
	s.pendingMCP = nil
	return value
}

func (s *Service) CreateToken(ctx context.Context, name, tokenType string, scopes []string, expiresAt *time.Time) (TokenInfo, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return TokenInfo{}, "", errors.New("token name is required")
	}
	if values, err := s.store.ListAPITokens(ctx); err != nil {
		return TokenInfo{}, "", err
	} else {
		for _, value := range values {
			if strings.EqualFold(strings.TrimSpace(value.Name), name) {
				return TokenInfo{}, "", ErrTokenNameExists
			}
		}
	}
	if expiresAt != nil && !expiresAt.After(time.Now().UTC()) {
		return TokenInfo{}, "", errors.New("token expiration must be in the future")
	}
	scopes, err := normalizeScopes(tokenType, scopes)
	if err != nil {
		return TokenInfo{}, "", err
	}
	info, secret, err := s.createToken(ctx, name, tokenType, scopes, expiresAt)
	return info, secret.Token, err
}

func (s *Service) createToken(ctx context.Context, name, tokenType string, scopes []string, expiresAt *time.Time) (TokenInfo, TokenSecret, error) {
	if tokenType != TokenTypeREST && tokenType != TokenTypeMCP {
		return TokenInfo{}, TokenSecret{}, errors.New("token type must be rest or mcp")
	}
	plain, err := randomToken(32)
	if err != nil {
		return TokenInfo{}, TokenSecret{}, err
	}
	plain = "mk_" + plain
	ciphertext, nonce, err := encryptSecret(s.tokenKey, []byte(plain))
	if err != nil {
		return TokenInfo{}, TokenSecret{}, err
	}
	now := time.Now().UTC()
	value := store.APIToken{ID: uuid.NewString(), Name: name, Type: tokenType, Scopes: scopes, TokenHash: digest(plain), SecretCiphertext: ciphertext, SecretNonce: nonce, TokenHint: tokenHint(plain), ExpiresAt: expiresAt, CreatedAt: now}
	if err := s.store.CreateAPIToken(ctx, value); err != nil {
		return TokenInfo{}, TokenSecret{}, err
	}
	return tokenInfo(value), TokenSecret{ID: value.ID, Type: value.Type, Token: plain}, nil
}

func (s *Service) ListTokens(ctx context.Context) ([]TokenInfo, error) {
	values, err := s.store.ListAPITokens(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]TokenInfo, 0, len(values))
	for _, value := range values {
		result = append(result, tokenInfo(value))
	}
	return result, nil
}

func (s *Service) GetToken(ctx context.Context, id string) (TokenInfo, error) {
	value, err := s.store.GetAPIToken(ctx, id)
	if err != nil {
		return TokenInfo{}, err
	}
	return tokenInfo(value), nil
}

func (s *Service) RevokeToken(ctx context.Context, id string) error {
	return s.store.RevokeAPIToken(ctx, id, time.Now().UTC())
}

func (s *Service) RestoreToken(ctx context.Context, id string) error {
	return s.store.RestoreAPIToken(ctx, id)
}

func (s *Service) DeleteToken(ctx context.Context, id string) error {
	return s.store.DeleteAPIToken(ctx, id)
}

func (s *Service) RevealToken(ctx context.Context, id string) (string, error) {
	if !s.allowTokenCopy.Load() {
		return "", errors.New("token copying is disabled")
	}
	value, err := s.store.GetAPIToken(ctx, id)
	if err != nil {
		return "", err
	}
	if value.RevokedAt != nil {
		return "", errors.New("token is revoked")
	}
	return decryptSecret(s.tokenKey, value.SecretCiphertext, value.SecretNonce)
}

func (s *Service) AuthenticateToken(ctx context.Context, plain string) (TokenPrincipal, error) {
	if strings.TrimSpace(plain) == "" {
		return TokenPrincipal{}, errors.New("authentication required")
	}
	hash := digest(plain)
	value, err := s.store.GetAPITokenByHash(ctx, hash)
	if err != nil {
		return TokenPrincipal{}, errors.New("invalid token")
	}
	now := time.Now().UTC()
	if value.RevokedAt != nil || (value.ExpiresAt != nil && !value.ExpiresAt.After(now)) {
		return TokenPrincipal{}, errors.New("token is invalid or expired")
	}
	if err := s.store.TouchAPIToken(ctx, hash, now); err != nil {
		return TokenPrincipal{}, err
	}
	return TokenPrincipal{TokenID: value.ID, Type: value.Type, Scopes: value.Scopes}, nil
}

func HasScope(principal TokenPrincipal, scope string) bool {
	for _, value := range principal.Scopes {
		if value == scope {
			return true
		}
	}
	return false
}

func tokenInfo(value store.APIToken) TokenInfo {
	return TokenInfo{ID: value.ID, Name: value.Name, Type: value.Type, Scopes: value.Scopes, TokenHint: value.TokenHint, ExpiresAt: value.ExpiresAt, RevokedAt: value.RevokedAt, LastUsedAt: value.LastUsedAt, CreatedAt: value.CreatedAt}
}

func normalizeScopes(tokenType string, scopes []string) ([]string, error) {
	allowed := map[string]bool{ScopeAPIRead: true, ScopeAPIWrite: true, ScopeMCP: true}
	if tokenType == TokenTypeMCP {
		return []string{ScopeMCP}, nil
	}
	result := make([]string, 0, len(scopes))
	seen := map[string]bool{}
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" || seen[scope] {
			continue
		}
		if !allowed[scope] || scope == ScopeMCP {
			return nil, fmt.Errorf("invalid REST token scope %q", scope)
		}
		seen[scope] = true
		result = append(result, scope)
	}
	if len(result) == 0 {
		result = []string{ScopeAPIRead}
	}
	return result, nil
}

func tokenHint(value string) string {
	if len(value) <= 8 {
		return value
	}
	return value[:4] + "..." + value[len(value)-4:]
}

func encryptSecret(key []byte, plaintext []byte) (string, string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", "", err
	}
	return base64.RawStdEncoding.EncodeToString(gcm.Seal(nil, nonce, plaintext, nil)), base64.RawStdEncoding.EncodeToString(nonce), nil
}

func decryptSecret(key []byte, encoded, nonceEncoded string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	nonce, err := base64.RawStdEncoding.DecodeString(nonceEncoded)
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("unable to decrypt token")
	}
	return string(plaintext), nil
}

func loadOrCreateMasterKey(path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("security.master_key_file cannot be empty")
	}
	if data, err := os.ReadFile(path); err == nil {
		if len(data) != 32 {
			return nil, errors.New("master key must contain 32 bytes")
		}
		return data, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	key, err := randomBytes(32)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func randomBytes(size int) ([]byte, error) {
	value := make([]byte, size)
	_, err := rand.Read(value)
	return value, err
}
