package monitor

import (
	"fmt"
	"sort"
	"sync"

	"meerkit/internal/core"
)

type registryEntry struct {
	owner     string
	ownerName string
	module    core.MonitorModule
}

type Registry struct {
	mu      sync.RWMutex
	modules map[string]registryEntry
}

func NewRegistry() *Registry { return &Registry{modules: make(map[string]registryEntry)} }

// Register is reserved for host-owned modules used by tests and internal extensions.
func (r *Registry) Register(module core.MonitorModule) { _ = r.RegisterOwned("system", module) }

func (r *Registry) RegisterOwned(owner string, module core.MonitorModule) error {
	return r.RegisterOwnedAs(owner, owner, module)
}

func (r *Registry) RegisterOwnedAs(owner, ownerName string, module core.MonitorModule) error {
	if owner == "" || module == nil {
		return fmt.Errorf("module owner and implementation are required")
	}
	descriptor := module.Descriptor()
	if descriptor.Type == "" {
		return fmt.Errorf("module type is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if current, exists := r.modules[descriptor.Type]; exists && current.owner != owner {
		return fmt.Errorf("module type %q is owned by %s", descriptor.Type, current.owner)
	}
	r.modules[descriptor.Type] = registryEntry{owner: owner, ownerName: ownerName, module: module}
	return nil
}

func (r *Registry) ReplaceOwner(owner string, modules []core.MonitorModule) error {
	return r.ReplaceOwnerAs(owner, owner, modules)
}

func (r *Registry) ReplaceOwnerAs(owner, ownerName string, modules []core.MonitorModule) error {
	next := make(map[string]registryEntry, len(modules))
	for _, module := range modules {
		if module == nil || module.Descriptor().Type == "" {
			return fmt.Errorf("plugin %s returned an invalid module", owner)
		}
		moduleType := module.Descriptor().Type
		if _, duplicate := next[moduleType]; duplicate {
			return fmt.Errorf("plugin %s returned duplicate module type %q", owner, moduleType)
		}
		next[moduleType] = registryEntry{owner: owner, ownerName: ownerName, module: module}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for moduleType := range next {
		if current, exists := r.modules[moduleType]; exists && current.owner != owner {
			return fmt.Errorf("module type %q is owned by %s", moduleType, current.owner)
		}
	}
	for moduleType, current := range r.modules {
		if current.owner == owner {
			delete(r.modules, moduleType)
		}
	}
	for moduleType, entry := range next {
		r.modules[moduleType] = entry
	}
	return nil
}

func (r *Registry) ValidateReplaceOwner(owner string, moduleTypes []string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[string]struct{}{}
	for _, moduleType := range moduleTypes {
		if moduleType == "" {
			return fmt.Errorf("module type is required")
		}
		if _, exists := seen[moduleType]; exists {
			return fmt.Errorf("duplicate module type %q", moduleType)
		}
		seen[moduleType] = struct{}{}
		if current, exists := r.modules[moduleType]; exists && current.owner != owner {
			return fmt.Errorf("module type %q is owned by %s", moduleType, current.owner)
		}
	}
	return nil
}

func (r *Registry) RemoveOwner(owner string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for moduleType, current := range r.modules {
		if current.owner == owner {
			delete(r.modules, moduleType)
		}
	}
}
func (r *Registry) Get(moduleType string) (core.MonitorModule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.modules[moduleType]
	return entry.module, ok
}
func (r *Registry) Owner(moduleType string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.modules[moduleType]
	return entry.owner, ok
}
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, 0, len(r.modules))
	for moduleType := range r.modules {
		result = append(result, moduleType)
	}
	sort.Strings(result)
	return result
}
func (r *Registry) Descriptors() []core.ModuleDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]core.ModuleDescriptor, 0, len(r.modules))
	for _, entry := range r.modules {
		result = append(result, descriptorWithOwner(entry))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Type < result[j].Type })
	return result
}
func (r *Registry) Descriptor(moduleType string) (core.ModuleDescriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.modules[moduleType]
	if !ok {
		return core.ModuleDescriptor{}, false
	}
	return descriptorWithOwner(entry), true
}

func descriptorWithOwner(entry registryEntry) core.ModuleDescriptor {
	descriptor := core.WithCommonResultSets(entry.module.Descriptor())
	descriptor.PluginID = entry.owner
	descriptor.PluginName = entry.ownerName
	return descriptor
}
