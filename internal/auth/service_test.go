package auth

import (
	"context"
	"testing"
	"time"

	"meerkit/internal/store"
)

func TestAdministratorSetupAndSessionLifecycle(t *testing.T) {
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := NewService(database, time.Hour)
	initialized, err := service.Initialized(context.Background())
	if err != nil || initialized {
		t.Fatalf("initialized = %v, err = %v", initialized, err)
	}
	session, err := service.Setup(context.Background(), "a-secure-test-key")
	if err != nil {
		t.Fatal(err)
	}
	if session.Token == "" || session.CSRFToken == "" {
		t.Fatal("session tokens were not created")
	}
	if _, err := service.Authenticate(context.Background(), session.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Login(context.Background(), "wrong-key-value"); err == nil {
		t.Fatal("expected invalid login to fail")
	}
	if err := service.Logout(context.Background(), session.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), session.Token); err == nil {
		t.Fatal("logged out session remained valid")
	}
}

func TestAdministratorChangeKey(t *testing.T) {
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := NewService(database, time.Hour)
	oldSession, err := service.Setup(context.Background(), "a-secure-test-key")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ChangeKey(context.Background(), "wrong-current-key", "another-secure-key"); err == nil {
		t.Fatal("expected an invalid current key to fail")
	}
	if err := service.ChangeKey(context.Background(), "a-secure-test-key", "another-secure-key"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), oldSession.Token); err == nil {
		t.Fatal("changing the administrator key should revoke existing sessions")
	}
	if _, err := service.Login(context.Background(), "a-secure-test-key"); err == nil {
		t.Fatal("old administrator key remained valid")
	}
	if _, err := service.Login(context.Background(), "another-secure-key"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateAccessKeyHash(t *testing.T) {
	encoded, err := hashAccessKey("a-secure-test-key")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateAccessKeyHash(encoded); err != nil {
		t.Fatalf("generated access key hash was rejected: %v", err)
	}

	invalid := []string{
		"$2a$04$W1X.JN8MO7VSG7n7d0bYE.5W3UIaXTxdy8ZDHdd60FmBHKEOUmeje",
		"$argon2id$v=19$m=0,t=3,p=4$c2FsdA$a2V5",
		"$argon2id$v=19$m=4294967295,t=3,p=4$MDEyMzQ1Njc4OWFiY2RlZg$MDEyMzQ1Njc4OWFiY2RlZg",
		"$argon2id$v=19$m=65536,t=3,p=4$not-base64!$a2V5",
	}
	for _, value := range invalid {
		if err := ValidateAccessKeyHash(value); err == nil {
			t.Fatalf("invalid access key hash was accepted: %q", value)
		}
	}
}
