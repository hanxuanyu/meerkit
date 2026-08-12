package sdk

import (
	"context"
	"encoding/json"
	"fmt"
)

type Module interface {
	Descriptor() ModuleDescriptor
	ValidateConfig(json.RawMessage) error
	Execute(context.Context, json.RawMessage) (Observation, error)
}

type moduleProvider struct{ modules map[string]Module }

func NewProvider(modules ...Module) Provider {
	provider := &moduleProvider{modules: make(map[string]Module, len(modules))}
	for _, module := range modules {
		provider.modules[module.Descriptor().Type] = module
	}
	return provider
}
func (p *moduleProvider) module(moduleType string) (Module, error) {
	module, ok := p.modules[moduleType]
	if !ok {
		return nil, fmt.Errorf("unknown module %q", moduleType)
	}
	return module, nil
}
func (p *moduleProvider) ListModules() ([]ModuleDescriptor, error) {
	result := make([]ModuleDescriptor, 0, len(p.modules))
	for _, module := range p.modules {
		descriptor := module.Descriptor()
		if err := validateParameterDescriptors(descriptor); err != nil {
			return nil, fmt.Errorf("module %s descriptor: %w", descriptor.Type, err)
		}
		result = append(result, descriptor)
	}
	return result, nil
}

func validateParameterDescriptors(descriptor ModuleDescriptor) error {
	properties, ok := descriptor.ConfigSchema["properties"].(map[string]any)
	if !ok {
		if len(descriptor.Parameters) == 0 {
			return nil
		}
		return fmt.Errorf("config_schema.properties is required when parameters are declared")
	}
	required := schemaRequiredKeys(descriptor.ConfigSchema["required"])
	parameters := make(map[string]ParameterDescriptor, len(descriptor.Parameters))
	for _, parameter := range descriptor.Parameters {
		if _, exists := parameters[parameter.Key]; exists {
			return fmt.Errorf("duplicate parameter %q", parameter.Key)
		}
		if _, exists := properties[parameter.Key]; !exists {
			return fmt.Errorf("parameter %q has no matching config schema property", parameter.Key)
		}
		if parameter.Required != required[parameter.Key] {
			return fmt.Errorf("parameter %q required flag does not match config schema", parameter.Key)
		}
		parameters[parameter.Key] = parameter
	}
	for key := range properties {
		if _, exists := parameters[key]; !exists {
			return fmt.Errorf("config schema property %q has no parameter descriptor", key)
		}
	}
	return nil
}

func validateModuleDescriptors(descriptors []ModuleDescriptor) error {
	for _, descriptor := range descriptors {
		if err := validateParameterDescriptors(descriptor); err != nil {
			return fmt.Errorf("module %s descriptor: %w", descriptor.Type, err)
		}
	}
	return nil
}

func schemaRequiredKeys(value any) map[string]bool {
	result := map[string]bool{}
	switch keys := value.(type) {
	case []string:
		for _, key := range keys {
			result[key] = true
		}
	case []any:
		for _, value := range keys {
			if key, ok := value.(string); ok {
				result[key] = true
			}
		}
	}
	return result
}
func (p *moduleProvider) ValidateConfig(_ context.Context, moduleType string, config json.RawMessage) error {
	module, err := p.module(moduleType)
	if err != nil {
		return err
	}
	return module.ValidateConfig(config)
}
func (p *moduleProvider) Execute(ctx context.Context, moduleType string, config json.RawMessage) (Observation, error) {
	module, err := p.module(moduleType)
	if err != nil {
		return Observation{}, err
	}
	return module.Execute(ctx, config)
}
func (p *moduleProvider) MigrateConfig(_ context.Context, moduleType, fromVersion, toVersion string, config json.RawMessage) (json.RawMessage, error) {
	if _, err := p.module(moduleType); err != nil {
		return nil, err
	}
	if fromVersion != toVersion {
		return nil, fmt.Errorf("module %s does not support config migration from %s to %s", moduleType, fromVersion, toVersion)
	}
	return config, nil
}
func (p *moduleProvider) Health(context.Context) error { return nil }

func Float64(value float64) *float64 { return &value }
