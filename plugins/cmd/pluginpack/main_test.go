package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateKeyCreatesMatchingBase64KeyPair(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "release")
	privatePath, publicPath, err := generateKey(prefix)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	privateKey := decodeKeyFile(t, privatePath)
	publicKey := decodeKeyFile(t, publicPath)
	if len(privateKey) != ed25519.PrivateKeySize {
		t.Fatalf("private key size = %d", len(privateKey))
	}
	if len(publicKey) != ed25519.PublicKeySize {
		t.Fatalf("public key size = %d", len(publicKey))
	}
	derived := ed25519.PrivateKey(privateKey).Public().(ed25519.PublicKey)
	if string(derived) != string(publicKey) {
		t.Fatal("generated public key does not match private key")
	}
	if _, _, err := generateKey(prefix); err == nil {
		t.Fatal("existing key files should not be overwritten")
	}
}

func TestSignIncludesPublicKey(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "release")
	privatePath, _, err := generateKey(prefix)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("manifest")
	encoded, err := sign(data, privatePath, "release-2026")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Version   int    `json:"version"`
		KeyID     string `json:"key_id"`
		PublicKey string `json:"public_key"`
		Signature string `json:"signature"`
	}
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatal(err)
	}
	publicKey, err := base64.StdEncoding.DecodeString(envelope.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := base64.StdEncoding.DecodeString(envelope.Signature)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Version != 1 || envelope.KeyID != "release-2026" || !ed25519.Verify(publicKey, data, signature) {
		t.Fatalf("invalid signature envelope: %#v", envelope)
	}
}

func decodeKeyFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(contents)))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}
