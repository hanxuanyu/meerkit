package schema

import (
	"encoding/json"
	"testing"
)

func TestPublishedSchemasAreJSONObjects(t *testing.T) {
	for _, name := range []string{"request.schema.json", "response.schema.json", "module-descriptor.schema.json", "observation.schema.json", "conformance-suite.schema.json"} {
		data, err := Files.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		if value["$schema"] != "https://json-schema.org/draft/2020-12/schema" || value["$id"] == nil {
			t.Fatalf("%s does not declare draft 2020-12 and an id", name)
		}
	}
}
