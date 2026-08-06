package plugin

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectSignature(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := []byte("schema_version: 1\nid: example.signed\n")
	documents := map[string][]byte{"README.md": []byte("# Signed plugin")}
	payload := SignaturePayload(manifest, documents)
	signature, err := json.Marshal(map[string]any{
		"version":    1,
		"key_id":     "release-2026",
		"public_key": base64.StdEncoding.EncodeToString(publicKey),
		"signature":  base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload)),
	})
	if err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	if err := os.WriteFile(filepath.Join(stage, "README.md"), documents["README.md"], 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "meerkit-plugin.sig"), signature, 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := inspectSignature(stage, manifest)
	if err != nil || !info.Signed || info.Fingerprint != publicKeyFingerprint(publicKey) {
		t.Fatalf("valid signature was not verified: info=%#v err=%v", info, err)
	}
	if _, err := inspectSignature(stage, append(manifest, 'x')); err == nil {
		t.Fatal("signature verified after manifest modification")
	}
	if err := os.WriteFile(filepath.Join(stage, "README.md"), []byte("modified"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectSignature(stage, manifest); err == nil {
		t.Fatal("signature verified after README modification")
	}
}
