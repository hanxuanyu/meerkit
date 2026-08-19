package plugin

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hanxuanyu/meerkit/sdk"
	"go.yaml.in/yaml/v3"
	"meerkit/internal/core"
)

func TestSourcePluginManifestsMatchRuntimeContract(t *testing.T) {
	schema := loadManifestSchema(t)
	for _, name := range []string{"network", "template"} {
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
		if manifest.Runtime == nil || manifest.Runtime.Mode != "direct" || manifest.Runtime.Command != "" || manifest.Runtime.Args == nil || len(manifest.Runtime.Args) != 0 {
			t.Fatalf("source manifest %s does not declare the complete direct runtime: %#v", name, manifest.Runtime)
		}
		validateManifestSchema(t, schema, data)
		if name == "network" {
			types := make(map[string]bool, len(manifest.Modules))
			for _, module := range manifest.Modules {
				types[module.Type] = true
			}
			for _, moduleType := range []string{"http", "tcp", "dns", "tls-certificate", "icmp"} {
				if !types[moduleType] {
					t.Fatalf("network source manifest does not declare %s", moduleType)
				}
			}
		}
	}
}

func loadManifestSchema(t *testing.T) *jsonschema.Resolved {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "plugins", "manifest.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode manifest schema: %v", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("resolve manifest schema: %v", err)
	}
	return resolved
}

func validateManifestSchema(t *testing.T, schema *jsonschema.Resolved, data []byte) {
	t.Helper()
	var document any
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode manifest YAML for schema validation: %v", err)
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("convert manifest YAML to JSON: %v", err)
	}
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(document); err != nil {
		t.Fatalf("manifest does not satisfy JSON Schema: %v", err)
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
	required, ok := schema["required"].([]any)
	if !ok || !containsSchemaField(required, "runtime") {
		t.Fatal("manifest schema does not require runtime")
	}
	artifacts, ok := properties["artifacts"].(map[string]any)
	if !ok || artifacts["items"] == nil {
		t.Fatal("manifest schema does not define artifact entries")
	}
	if minimum, exists := artifacts["minItems"]; exists && minimum.(float64) > 0 {
		t.Fatalf("source manifests cannot satisfy artifacts.minItems = %v", minimum)
	}
	definitions, ok := schema["$defs"].(map[string]any)
	if !ok || properties["runtime"] == nil || definitions["protocolRange"] == nil || definitions["module"] == nil || definitions["artifact"] == nil || definitions["runtime"] == nil {
		t.Fatal("manifest schema protocol, module, artifact, or runtime definitions are missing")
	}
}

func containsSchemaField(fields []any, target string) bool {
	for _, field := range fields {
		if field == target {
			return true
		}
	}
	return false
}

func TestManifestRejectsInvalidPackagedArtifact(t *testing.T) {
	runtimeConfig := ArtifactRuntime{Mode: "direct", Args: []string{}}
	manifest := Manifest{SchemaVersion: 1, ID: "example.monitor", Name: "Example", Version: "1.0.0", Vendor: "Example", Description: "Example plugin", URL: "https://example.com/plugin", Protocol: ProtocolRange{Min: 1, Max: 1}, Modules: []core.PluginModule{{Type: "example", Name: "Example", Version: "1", ConfigVersion: "1", ResultSchemaVersion: "1"}}, Runtime: &runtimeConfig, Artifacts: []Artifact{{GOOS: "linux", GOARCH: "amd64", Path: "../plugin", Size: 1, SHA256: "invalid"}}}
	if err := manifest.Validate(sdk.ProtocolVersion); err == nil {
		t.Fatal("invalid packaged artifact was accepted")
	}
}

func TestArtifactRuntimeValidation(t *testing.T) {
	valid := []ArtifactRuntime{
		{Mode: "direct", Args: []string{}},
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
		{Mode: "direct"},
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

func TestManifestRuntimeResolution(t *testing.T) {
	defaultRuntime := ArtifactRuntime{Mode: "direct", Args: []string{"serve"}}
	overrideRuntime := ArtifactRuntime{Mode: "direct", Args: []string{"run"}}
	manifest := Manifest{Runtime: &defaultRuntime}
	artifact := Artifact{Runtime: &overrideRuntime}

	if got, err := manifest.ResolveRuntime(nil); err != nil || len(got.Args) != 1 || got.Args[0] != "serve" {
		t.Fatalf("manifest runtime was not used: %#v", got)
	}
	if got, err := manifest.ResolveRuntime(&artifact); err != nil || len(got.Args) != 1 || got.Args[0] != "run" {
		t.Fatalf("artifact runtime did not override manifest runtime: %#v", got)
	}
	if _, err := (Manifest{}).ResolveRuntime(nil); err == nil {
		t.Fatal("manifest without runtime was accepted")
	}
}

func TestArtifactWithoutRuntimeOmitsPlatformOverride(t *testing.T) {
	data, err := yaml.Marshal(Artifact{GOOS: "linux", GOARCH: "amd64", Path: "bin/plugin", Size: 1, SHA256: strings.Repeat("0", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("runtime:")) {
		t.Fatalf("artifact unexpectedly serialized a platform runtime override:\n%s", data)
	}
}
