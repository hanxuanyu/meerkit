package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"meerkit/internal/core"
)

func TestTrustPluginSignerUpdatesMatchingPlugins(t *testing.T) {
	database, err := OpenStore(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	now := time.Now().UTC()
	plugin := core.PluginInstallation{
		ID: "example.signed", Version: "1.0.0", Name: "Signed", Vendor: "Example", Description: "test", URL: "https://example.com/plugin",
		TrustState: "untrusted", SignerKeyID: "release", SignerFingerprint: "SHA256:ABC", SignerPublicKey: "public", Status: "installed",
		PackagePath: "package", BinaryPath: "binary", PackageName: "plugin.zip", PackageSHA256: "hash", Manifest: json.RawMessage(`{}`), Modules: []core.PluginModule{}, CreatedAt: now, UpdatedAt: now,
	}
	if err := database.UpsertPlugin(context.Background(), plugin); err != nil {
		t.Fatal(err)
	}
	if err := database.TrustPluginSigner(context.Background(), core.TrustedPluginSigner{Fingerprint: "SHA256:ABC", KeyID: "release", PublicKey: "public", Vendor: "Example", Source: "user"}); err != nil {
		t.Fatal(err)
	}
	updated, err := database.GetPlugin(context.Background(), plugin.ID, plugin.Version)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Verified || updated.TrustState != "trusted" {
		t.Fatalf("plugin trust was not updated: %#v", updated)
	}
}
