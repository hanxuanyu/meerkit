package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"meerkit/internal/store"
)

func TestMCPTokenLifecycleAndPendingSecret(t *testing.T) {
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service, err := NewServiceWithOptions(database, time.Hour, ServiceOptions{MasterKeyFile: filepath.Join(t.TempDir(), "master.key"), AllowTokenCopy: true})
	if err != nil {
		t.Fatal(err)
	}
	info, err := service.EnsureMCPToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.ListTokens(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	pending := service.ConsumePendingMCPToken()
	if len(items) != 1 || pending == nil || pending.Token == "" || info.ID != pending.ID {
		t.Fatalf("unexpected bootstrap: items=%#v pending=%#v", items, pending)
	}
	principal, err := service.AuthenticateToken(context.Background(), pending.Token)
	if err != nil || principal.TokenID != info.ID || !HasScope(principal, ScopeMCP) {
		t.Fatalf("MCP token authentication failed: %#v %v", principal, err)
	}
	if pending = service.ConsumePendingMCPToken(); pending != nil {
		t.Fatalf("pending token was not consumed: %#v", pending)
	}
	principal, err = service.AuthenticateToken(context.Background(), "bad")
	if err == nil || principal.TokenID != "" {
		t.Fatal("invalid token was accepted")
	}
	secret, err := service.RevealToken(context.Background(), info.ID)
	if err != nil || secret == "" {
		t.Fatalf("reveal failed: %q %v", secret, err)
	}
}

func TestTokenRevealCanBeDisabled(t *testing.T) {
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service, err := NewServiceWithOptions(database, time.Hour, ServiceOptions{MasterKeyFile: filepath.Join(t.TempDir(), "master.key")})
	if err != nil {
		t.Fatal(err)
	}
	info, _, err := service.CreateToken(context.Background(), "read-only", TokenTypeREST, []string{ScopeAPIRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RevealToken(context.Background(), info.ID); err == nil {
		t.Fatal("reveal should be disabled")
	}
}

func TestTokenRestoreAndPermanentDelete(t *testing.T) {
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := NewService(database, time.Hour)
	info, secret, err := service.CreateToken(context.Background(), "automation", TokenTypeREST, []string{ScopeAPIRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RevokeToken(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthenticateToken(context.Background(), secret); err == nil {
		t.Fatal("revoked token remained valid")
	}
	if err := service.RestoreToken(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthenticateToken(context.Background(), secret); err != nil {
		t.Fatalf("restored token was not valid: %v", err)
	}
	if err := service.DeleteToken(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthenticateToken(context.Background(), secret); err == nil {
		t.Fatal("deleted token remained valid")
	}
}

func TestTokenNamesMustBeUnique(t *testing.T) {
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := NewService(database, time.Hour)
	if _, _, err := service.CreateToken(context.Background(), "Deploy Bot", TokenTypeREST, []string{ScopeAPIRead}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.CreateToken(context.Background(), " deploy bot ", TokenTypeREST, []string{ScopeAPIRead}, nil); !errors.Is(err, ErrTokenNameExists) {
		t.Fatalf("duplicate token name error = %v", err)
	}
}
