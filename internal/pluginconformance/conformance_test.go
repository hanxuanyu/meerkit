package pluginconformance

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/hanxuanyu/meerkit/sdk"
	"meerkit/internal/core"
	"meerkit/internal/plugin"
)

func TestLoadTemplateSuite(t *testing.T) {
	suite, err := LoadSuite(filepath.Join("..", "..", "plugins", "template", "conformance.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Cases) != 1 || suite.Cases[0].ModuleType != "example" || suite.Cases[0].Execute == nil {
		t.Fatalf("unexpected suite: %#v", suite)
	}
}

func TestObservationSchemaRejectsMissingRequiredFields(t *testing.T) {
	all, err := loadValidators()
	if err != nil {
		t.Fatal(err)
	}
	value, err := decodeJSON([]byte(`{"success":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := all.observation.Validate(value); err == nil {
		t.Fatal("incomplete observation was accepted")
	}
	valid := sdk.Observation{Success: true, SchemaVersion: "1", Result: map[string]any{"message": "ok"}, Summary: "ok"}
	data, _ := json.Marshal(valid)
	value, _ = decodeJSON(data)
	if err := all.observation.Validate(value); err != nil {
		t.Fatalf("valid observation: %v", err)
	}
}

func TestCompareModulesChecksManifestIdentityAndVersion(t *testing.T) {
	manifest := plugin.Manifest{Modules: []core.PluginModule{{Type: "http", Version: "1"}}}
	if err := compareModules(manifest, []sdk.ModuleDescriptor{{Type: "http", Version: "1"}}); err != nil {
		t.Fatal(err)
	}
	if err := compareModules(manifest, []sdk.ModuleDescriptor{{Type: "http", Version: "2"}}); err == nil {
		t.Fatal("version mismatch was accepted")
	}
}
