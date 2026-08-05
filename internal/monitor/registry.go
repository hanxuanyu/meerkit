package monitor

import (
	"meerkit/internal/core"
	httpmonitor "meerkit/internal/monitor/http"
	tcpmonitor "meerkit/internal/monitor/tcp"
)

type Registry struct {
	modules map[string]core.MonitorModule
}

func NewRegistry() *Registry {
	registry := &Registry{modules: make(map[string]core.MonitorModule)}
	registry.Register(httpmonitor.New())
	registry.Register(tcpmonitor.New())
	return registry
}

func (r *Registry) Register(module core.MonitorModule) {
	r.modules[module.Descriptor().Type] = module
}

func (r *Registry) Get(moduleType string) (core.MonitorModule, bool) {
	module, ok := r.modules[moduleType]
	return module, ok
}

func (r *Registry) Descriptors() []core.ModuleDescriptor {
	result := make([]core.ModuleDescriptor, 0, len(r.modules))
	for _, module := range r.modules {
		result = append(result, module.Descriptor())
	}
	return result
}
