package sdk

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"time"
)

type loggingProvider struct {
	provider Provider
	logger   *slog.Logger
}

func newLoggingProvider(provider Provider, logger *slog.Logger) Provider {
	return &loggingProvider{provider: provider, logger: logger}
}

func (p *loggingProvider) ListModules() ([]ModuleDescriptor, error) {
	started := time.Now()
	modules, err := p.provider.ListModules()
	if err != nil {
		p.logger.Error("plugin module discovery failed", "duration_ms", time.Since(started).Milliseconds(), "error", err)
		return nil, err
	}
	types := make([]string, 0, len(modules))
	for _, module := range modules {
		types = append(types, module.Type)
	}
	sort.Strings(types)
	p.logger.Info("plugin modules discovered", "module_count", len(modules), "modules", types, "duration_ms", time.Since(started).Milliseconds())
	return modules, nil
}

func (p *loggingProvider) ValidateConfig(ctx context.Context, moduleType string, config json.RawMessage) error {
	started := time.Now()
	err := p.provider.ValidateConfig(ctx, moduleType, config)
	if err != nil {
		p.logger.Warn("plugin config validation failed", "module_type", moduleType, "duration_ms", time.Since(started).Milliseconds(), "error", err)
		return err
	}
	p.logger.Debug("plugin config validated", "module_type", moduleType, "duration_ms", time.Since(started).Milliseconds())
	return nil
}

func (p *loggingProvider) Execute(ctx context.Context, moduleType string, config json.RawMessage) (Observation, error) {
	started := time.Now()
	p.logger.Info("plugin execution started", "module_type", moduleType)
	observation, err := p.provider.Execute(ctx, moduleType, config)
	if err != nil {
		p.logger.Error("plugin execution failed", "module_type", moduleType, "duration_ms", time.Since(started).Milliseconds(), "error", err)
		return observation, err
	}
	p.logger.Info("plugin execution completed", "module_type", moduleType, "duration_ms", time.Since(started).Milliseconds(), "success", observation.Success, "error_code", observation.ErrorCode)
	return observation, nil
}

func (p *loggingProvider) MigrateConfig(ctx context.Context, moduleType, fromVersion, toVersion string, config json.RawMessage) (json.RawMessage, error) {
	started := time.Now()
	if fromVersion != toVersion {
		p.logger.Info("plugin config migration started", "module_type", moduleType, "from_version", fromVersion, "to_version", toVersion)
	}
	migrated, err := p.provider.MigrateConfig(ctx, moduleType, fromVersion, toVersion, config)
	if err != nil {
		p.logger.Error("plugin config migration failed", "module_type", moduleType, "from_version", fromVersion, "to_version", toVersion, "duration_ms", time.Since(started).Milliseconds(), "error", err)
		return nil, err
	}
	if fromVersion != toVersion {
		p.logger.Info("plugin config migration completed", "module_type", moduleType, "from_version", fromVersion, "to_version", toVersion, "duration_ms", time.Since(started).Milliseconds())
	} else {
		p.logger.Debug("plugin config migration not required", "module_type", moduleType, "version", toVersion)
	}
	return migrated, nil
}

func (p *loggingProvider) Health(ctx context.Context) error {
	started := time.Now()
	err := p.provider.Health(ctx)
	if err != nil {
		p.logger.Error("plugin health check failed", "duration_ms", time.Since(started).Milliseconds(), "error", err)
		return err
	}
	p.logger.Info("plugin health check passed", "duration_ms", time.Since(started).Milliseconds())
	return nil
}
