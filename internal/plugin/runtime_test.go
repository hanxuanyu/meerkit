package plugin

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"meerkit/internal/core"
)

func TestNewArtifactCommandUsesDirectArtifactByDefault(t *testing.T) {
	artifact := filepath.Join(t.TempDir(), "plugin")
	command, err := NewArtifactCommand(artifact, ArtifactRuntime{Mode: "direct", Args: []string{"serve"}})
	if err != nil {
		t.Fatal(err)
	}
	if command.Path != artifact || len(command.Args) != 2 || command.Args[0] != artifact || command.Args[1] != "serve" {
		t.Fatalf("unexpected direct command: path=%q args=%q", command.Path, command.Args)
	}
}

func TestNewArtifactCommandSubstitutesInterpreterArtifact(t *testing.T) {
	goCommand, err := execLookPathForTest("go")
	if err != nil {
		t.Skip(err)
	}
	artifact := filepath.Join(t.TempDir(), "plugin payload")
	command, err := NewArtifactCommand(artifact, ArtifactRuntime{Mode: "interpreter", Command: "go", Args: []string{"run", "{artifact}"}})
	if err != nil {
		t.Fatal(err)
	}
	if command.Path != goCommand || len(command.Args) != 3 || command.Args[2] != artifact {
		t.Fatalf("unexpected interpreter command: path=%q args=%q", command.Path, command.Args)
	}
}

func TestCommandForInstallationUsesCurrentArtifactRuntime(t *testing.T) {
	defaultRuntime := ArtifactRuntime{Mode: "direct", Args: []string{}}
	manifest := Manifest{Runtime: &defaultRuntime, Artifacts: []Artifact{{
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Path: "plugin", Size: 1, SHA256: strings.Repeat("0", 64),
		Runtime: &ArtifactRuntime{Mode: "interpreter", Command: "go", Args: []string{"run", "{artifact}"}},
	}}}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(t.TempDir(), "plugin")
	command, err := commandForInstallation(core.PluginInstallation{BinaryPath: artifact, Manifest: data})
	if err != nil {
		t.Fatal(err)
	}
	if command.Args[len(command.Args)-1] != artifact {
		t.Fatalf("artifact was not substituted: %q", command.Args)
	}
}

func TestCommandForInstallationUsesManifestRuntimeDefault(t *testing.T) {
	manifest := Manifest{
		Runtime: &ArtifactRuntime{Mode: "direct", Args: []string{"serve"}},
		Artifacts: []Artifact{{
			GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Path: "plugin", Size: 1, SHA256: strings.Repeat("0", 64),
		}},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(t.TempDir(), "plugin")
	command, err := commandForInstallation(core.PluginInstallation{BinaryPath: artifact, Manifest: data})
	if err != nil {
		t.Fatal(err)
	}
	if len(command.Args) != 2 || command.Args[0] != artifact || command.Args[1] != "serve" {
		t.Fatalf("manifest runtime default was not applied: %q", command.Args)
	}
}

func execLookPathForTest(command string) (string, error) {
	path, err := exec.LookPath(command)
	return path, err
}
