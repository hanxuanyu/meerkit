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
		result = append(result, module.Descriptor())
	}
	return result, nil
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
