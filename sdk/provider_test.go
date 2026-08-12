package sdk

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type descriptorTestModule struct{ descriptor ModuleDescriptor }

func (m descriptorTestModule) Descriptor() ModuleDescriptor       { return m.descriptor }
func (descriptorTestModule) ValidateConfig(json.RawMessage) error { return nil }
func (descriptorTestModule) Execute(context.Context, json.RawMessage) (Observation, error) {
	return Observation{}, nil
}

func TestProviderRequiresParametersForEveryConfigProperty(t *testing.T) {
	provider := NewProvider(descriptorTestModule{descriptor: ModuleDescriptor{
		Type: "test", ConfigSchema: map[string]any{"type": "object", "properties": map[string]any{"url": map[string]any{"type": "string"}}},
	}})
	_, err := provider.ListModules()
	if err == nil || !strings.Contains(err.Error(), "has no parameter descriptor") {
		t.Fatalf("ListModules error = %v", err)
	}
}

func TestProviderRequiresParameterAndSchemaRequiredFlagsToMatch(t *testing.T) {
	provider := NewProvider(descriptorTestModule{descriptor: ModuleDescriptor{
		Type: "test", ConfigSchema: map[string]any{"type": "object", "required": []string{"url"}, "properties": map[string]any{"url": map[string]any{"type": "string"}}},
		Parameters: []ParameterDescriptor{{Key: "url", Label: "URL", Type: ParameterURL}},
	}})
	_, err := provider.ListModules()
	if err == nil || !strings.Contains(err.Error(), "required flag does not match") {
		t.Fatalf("ListModules error = %v", err)
	}
}

func TestProviderAcceptsMatchingParameterDescriptors(t *testing.T) {
	provider := NewProvider(descriptorTestModule{descriptor: ModuleDescriptor{
		Type: "test", ConfigSchema: map[string]any{"type": "object", "required": []any{"url"}, "properties": map[string]any{"url": map[string]any{"type": "string"}}},
		Parameters: []ParameterDescriptor{{Key: "url", Label: "URL", Type: ParameterURL, Required: true}},
	}})
	if _, err := provider.ListModules(); err != nil {
		t.Fatalf("ListModules: %v", err)
	}
}
