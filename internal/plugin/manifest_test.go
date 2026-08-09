package plugin

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hanxuanyu/meerkit/sdk"
	"go.yaml.in/yaml/v3"
	"meerkit/internal/core"
)

func TestSourcePluginManifestsMatchRuntimeContract(t *testing.T) {
	for _, name := range []string{"http", "tcp", "template"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "plugins", name, "meerkit-plugin.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var manifest Manifest
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			t.Fatalf("decode %s manifest: %v", name, err)
		}
		if err := manifest.Validate(sdk.ProtocolVersion); err != nil {
			t.Fatalf("validate %s source manifest: %v", name, err)
		}
		if len(manifest.Artifacts) != 0 {
			t.Fatalf("source manifest %s contains packaged artifacts", name)
		}
	}
}

func TestManifestSchemaAllowsEmptySourceArtifactsAndDefinesEntries(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "plugins", "manifest.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode manifest schema: %v", err)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("manifest schema properties are missing")
	}
	artifacts, ok := properties["artifacts"].(map[string]any)
	if !ok || artifacts["items"] == nil {
		t.Fatal("manifest schema does not define artifact entries")
	}
	if minimum, exists := artifacts["minItems"]; exists && minimum.(float64) > 0 {
		t.Fatalf("source manifests cannot satisfy artifacts.minItems = %v", minimum)
	}
	definitions, ok := schema["$defs"].(map[string]any)
	if !ok || definitions["protocolRange"] == nil || definitions["module"] == nil || definitions["artifact"] == nil || definitions["runtime"] == nil {
		t.Fatal("manifest schema protocol, module, artifact, or runtime definitions are missing")
	}
}

func TestManifestRejectsInvalidPackagedArtifact(t *testing.T) {
	manifest := Manifest{SchemaVersion: 1, ID: "example.monitor", Name: "Example", Version: "1.0.0", Vendor: "Example", Description: "Example plugin", URL: "https://example.com/plugin", Protocol: ProtocolRange{Min: 1, Max: 1}, Modules: []core.PluginModule{{Type: "example", Name: "Example", Version: "1", ConfigVersion: "1", ResultSchemaVersion: "1"}}, Artifacts: []Artifact{{GOOS: "linux", GOARCH: "amd64", Path: "../plugin", Size: 1, SHA256: "invalid"}}}
	if err := manifest.Validate(sdk.ProtocolVersion); err == nil {
		t.Fatal("invalid packaged artifact was accepted")
	}
}

func TestArtifactRuntimeValidation(t *testing.T) {
	valid := []ArtifactRuntime{
		{Mode: "direct"},
		{Mode: "direct", Args: []string{"serve", "--plugin"}},
		{Mode: "interpreter", Command: "python3", Args: []string{"-I", "{artifact}"}},
		{Mode: "interpreter", Command: "java", Args: []string{"-jar", "{artifact}"}},
	}
	for _, value := range valid {
		if err := value.Validate(); err != nil {
			t.Fatalf("valid runtime %#v: %v", value, err)
		}
	}
	invalid := []ArtifactRuntime{
		{},
		{Mode: "direct", Command: "plugin"},
		{Mode: "direct", Args: []string{"{artifact}"}},
		{Mode: "interpreter", Command: "/usr/bin/python3", Args: []string{"{artifact}"}},
		{Mode: "interpreter", Command: "python3", Args: []string{"script.py"}},
		{Mode: "interpreter", Command: "python3", Args: []string{"{artifact}", "{artifact}"}},
	}
	for _, value := range invalid {
		if err := value.Validate(); err == nil {
			t.Fatalf("invalid runtime was accepted: %#v", value)
		}
	}
}

func TestArtifactWithoutRuntimeKeepsLegacyManifestShape(t *testing.T) {
	data, err := yaml.Marshal(Artifact{GOOS: "linux", GOARCH: "amd64", Path: "bin/plugin", Size: 1, SHA256: strings.Repeat("0", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("runtime:")) {
		t.Fatalf("default artifact unexpectedly serialized runtime metadata:\n%s", data)
	}
}
