package plugin

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go.yaml.in/yaml/v3"
	"meerkit/internal/core"
	"meerkit/internal/monitor"
	"meerkit/internal/store"
)

func TestTrustPublisherAppliesToLaterPlugins(t *testing.T) {
	manager, closeManager := newTrustTestManager(t)
	defer closeManager()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	firstArchive := writeSignedTestPackage(t, "example.first", "first", privateKey)
	first, err := manager.Import(context.Background(), firstArchive, ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if first.Verified || first.TrustState != trustStateUntrusted || first.SignerFingerprint != publicKeyFingerprint(publicKey) {
		t.Fatalf("first plugin trust state = %#v", first)
	}
	trusted, err := manager.TrustPublisher(context.Background(), first.ID, first.Version, first.SignerFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if !trusted.Verified || trusted.TrustState != trustStateTrusted {
		t.Fatalf("trusted plugin state = %#v", trusted)
	}
	secondArchive := writeSignedTestPackage(t, "example.second", "second", privateKey)
	second, err := manager.Import(context.Background(), secondArchive, ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Verified || second.TrustState != trustStateTrusted {
		t.Fatalf("later plugin was not automatically trusted: %#v", second)
	}
}

func TestOfficialPackageBootstrapsPublisherTrust(t *testing.T) {
	manager, closeManager := newTrustTestManager(t)
	defer closeManager()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	officialArchive := writeSignedTestPackage(t, "meerkit.official", "official", privateKey)
	official, err := manager.Import(context.Background(), officialArchive, ImportOptions{Official: true})
	if err != nil {
		t.Fatal(err)
	}
	if !official.Verified || official.TrustState != trustStateOfficial {
		t.Fatalf("official plugin state = %#v", official)
	}
	laterArchive := writeSignedTestPackage(t, "meerkit.later", "later", privateKey)
	later, err := manager.Import(context.Background(), laterArchive, ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !later.Verified || later.TrustState != trustStateTrusted {
		t.Fatalf("official publisher was not bootstrapped: %#v", later)
	}
}

func newTrustTestManager(t *testing.T) (*Manager, func()) {
	t.Helper()
	dataDir := t.TempDir()
	database, err := store.OpenStore(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(database, monitor.NewRegistry(), ManagerOptions{DataDir: dataDir})
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	return manager, func() { manager.Close(); database.Close() }
}

func writeSignedTestPackage(t *testing.T, id, moduleType string, privateKey ed25519.PrivateKey) string {
	t.Helper()
	binary := []byte("test plugin binary")
	digest := sha256.Sum256(binary)
	runtimeConfig := ArtifactRuntime{Mode: "direct", Args: []string{}}
	manifest := Manifest{
		SchemaVersion: 1, ID: id, Name: id, Version: "1.0.0", Vendor: "Example", Description: "Signed test plugin", URL: "https://example.com/" + id,
		Protocol:  ProtocolRange{Min: 1, Max: 1},
		Modules:   []core.PluginModule{{Type: moduleType, Name: moduleType, Version: "1", ConfigVersion: "1", ResultSchemaVersion: "1"}},
		Runtime:   &runtimeConfig,
		Artifacts: []Artifact{{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Path: "bin/" + runtime.GOOS + "-" + runtime.GOARCH + "/plugin", Size: int64(len(binary)), SHA256: hex.EncodeToString(digest[:])}},
	}
	manifestBytes, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	documents := map[string][]byte{"README.md": []byte("# " + id)}
	signature, err := json.Marshal(map[string]any{
		"version": 1, "key_id": "release-test", "public_key": base64.StdEncoding.EncodeToString(publicKey), "signature": base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, SignaturePayload(manifestBytes, documents))),
	})
	if err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), id+".zip")
	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	files := map[string][]byte{"meerkit-plugin.yaml": manifestBytes, "meerkit-plugin.sig": signature, manifest.Artifacts[0].Path: binary, "README.md": documents["README.md"]}
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return archivePath
}
