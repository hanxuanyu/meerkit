package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"meerkit/internal/monitor"
	"meerkit/internal/store"
)

func TestSyncDevelopmentBuildsSourcePluginsAndSkipsTemplate(t *testing.T) {
	dataDir := t.TempDir()
	database, err := store.OpenStore(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager, err := NewManager(database, monitor.NewRegistry(), ManagerOptions{DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}

	sourceRoot := filepath.Join(t.TempDir(), "plugins")
	writeDevelopmentPlugin(t, filepath.Join(sourceRoot, "http"), "meerkit.http", "http")
	writeDevelopmentPlugin(t, filepath.Join(sourceRoot, "template"), "example.monitor", "example")
	builds := 0
	manager.developmentBuilder = func(_ context.Context, sourceDir, outputPath string) error {
		builds++
		return os.WriteFile(outputPath, []byte(filepath.Base(sourceDir)), 0o700)
	}

	values, err := manager.SyncDevelopment(context.Background(), sourceRoot)
	if err != nil {
		t.Fatalf("sync development plugins: %v", err)
	}
	if builds != 1 || len(values) != 1 {
		t.Fatalf("builds = %d, plugins = %d; want one publishable source plugin", builds, len(values))
	}
	value := values[0]
	if value.ID != "meerkit.http" || value.TrustState != trustStateDevelopment || !value.Enabled || !value.Official || !value.Verified {
		t.Fatalf("unexpected development installation: %#v", value)
	}
	if value.PackagePath != "" || value.PackageName != "本地源码" {
		t.Fatalf("development package metadata = path %q name %q", value.PackagePath, value.PackageName)
	}
	if _, err := os.Stat(value.BinaryPath); err != nil {
		t.Fatalf("development binary was not installed: %v", err)
	}

	if err := manager.ClearDevelopment(context.Background()); err != nil {
		t.Fatalf("clear development plugins: %v", err)
	}
	installed, err := database.ListPlugins(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(installed) != 0 {
		t.Fatalf("development records were not cleared: %#v", installed)
	}
}

func TestSyncDevelopmentReportsMissingSources(t *testing.T) {
	dataDir := t.TempDir()
	database, err := store.OpenStore(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager, err := NewManager(database, monitor.NewRegistry(), ManagerOptions{DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.SyncDevelopment(context.Background(), filepath.Join(t.TempDir(), "missing")); err != ErrNoDevelopmentPlugins {
		t.Fatalf("missing source error = %v, want %v", err, ErrNoDevelopmentPlugins)
	}
}

func TestManagerResolvesRelativeDevelopmentPathsBeforeChangingBuildDirectory(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	temporaryDirectory := t.TempDir()
	if err := os.Chdir(temporaryDirectory); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(workingDirectory)

	database, err := store.OpenStore("./data")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager, err := NewManager(database, monitor.NewRegistry(), ManagerOptions{DataDir: "./data"})
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(manager.Root()) {
		t.Fatalf("plugin root = %q, want an absolute path", manager.Root())
	}

	sourceRoot := filepath.Join(temporaryDirectory, "plugins")
	writeDevelopmentPlugin(t, filepath.Join(sourceRoot, "http"), "meerkit.http", "http")
	manager.developmentBuilder = func(_ context.Context, _ string, outputPath string) error {
		if !filepath.IsAbs(outputPath) {
			t.Fatalf("development output path = %q, want an absolute path", outputPath)
		}
		return os.WriteFile(outputPath, []byte("development"), 0o700)
	}
	if _, err := manager.SyncDevelopment(context.Background(), sourceRoot); err != nil {
		t.Fatalf("sync with relative data directory: %v", err)
	}
}

func writeDevelopmentPlugin(t *testing.T, directory, id, moduleType string) {
	t.Helper()
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	manifest := "schema_version: 1\nid: " + id + "\nname: Development plugin\nversion: 1.0.0\nvendor: Meerkit\ndesp: Development plugin used by tests.\nurl: https://example.com/plugin\nprotocol: {min: 1, max: 1}\nmodules:\n  - {type: " + moduleType + ", name: Test, version: \"1\", config_version: \"1\", result_schema_version: \"1\"}\nartifacts: []\n"
	for name, contents := range map[string]string{"meerkit-plugin.yaml": manifest, "go.mod": "module example.com/test-plugin\n\ngo 1.26\n", "README.md": "# Development plugin\n"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
