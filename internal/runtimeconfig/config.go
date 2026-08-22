package runtimeconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"

	"meerkit/internal/app"
	"meerkit/internal/store"
)

type ApplyFunc func(context.Context, app.RuntimeConfig, app.RuntimeConfig) error
type ImportPersistFunc func(context.Context, map[string]json.RawMessage) (map[string]int, error)

type Manager struct {
	store    store.SystemConfigRepository
	defaults app.RuntimeConfig
	config   app.RuntimeConfig
	versions map[string]int
	subs     map[chan struct{}]struct{}
	apply    ApplyFunc
	mu       sync.RWMutex
}

func New(ctx context.Context, database store.SystemConfigRepository) (*Manager, error) {
	defaults := app.DefaultRuntimeConfig()
	manager := &Manager{store: database, defaults: defaults, versions: make(map[string]int), subs: make(map[chan struct{}]struct{})}
	for _, configType := range managedTypes() {
		if configType == app.SystemConfigAuth {
			if err := ensureAuthConfig(ctx, database, defaults.Auth); err != nil {
				return nil, err
			}
			continue
		}
		if _, err := database.EnsureSystemConfig(ctx, configType, manager.domain(defaults, configType)); err != nil {
			return nil, err
		}
	}
	if err := manager.load(ctx); err != nil {
		return nil, err
	}
	return manager, nil
}

func managedTypes() []string {
	return []string{app.SystemConfigStorage, app.SystemConfigScheduler, app.SystemConfigLogging, app.SystemConfigPlugins, app.SystemConfigAuth, app.SystemConfigMCP}
}

func ensureAuthConfig(ctx context.Context, database store.SystemConfigRepository, defaults app.RuntimeAuthConfig) error {
	row, err := database.GetSystemConfig(ctx, app.SystemConfigAuth)
	if store.IsNoRows(err) {
		_, err = database.EnsureSystemConfig(ctx, app.SystemConfigAuth, defaults)
		return err
	}
	if err != nil {
		return err
	}
	var current app.RuntimeAuthConfig
	if err := json.Unmarshal(row.Data, &current); err != nil {
		return fmt.Errorf("decode system config auth: %w", err)
	}
	if current.SessionTTL != "" {
		return nil
	}
	current.SessionTTL = defaults.SessionTTL
	data, err := json.Marshal(current)
	if err != nil {
		return err
	}
	_, err = database.UpdateSystemConfig(ctx, app.SystemConfigAuth, data, row.Version)
	if errors.Is(err, store.ErrSystemConfigVersionConflict) {
		return nil
	}
	return err
}

func (m *Manager) SetApply(apply ApplyFunc) {
	m.mu.Lock()
	m.apply = apply
	m.mu.Unlock()
}

// Subscribe returns a buffered notification channel for runtime changes. A
// notification means that a fresh Snapshot should be read; updates are
// coalesced while a component is busy.
func (m *Manager) Subscribe() <-chan struct{} {
	channel := make(chan struct{}, 1)
	m.mu.Lock()
	m.subs[channel] = struct{}{}
	m.mu.Unlock()
	return channel
}

func (m *Manager) Snapshot() app.RuntimeConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

func (m *Manager) Version(configType string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.versions[configType]
}

func (m *Manager) Metadata() []app.RuntimeConfigItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]app.RuntimeConfigItem, 0, len(app.RuntimeConfigDefinitions()))
	for _, definition := range app.RuntimeConfigDefinitions() {
		value := definition.Value(m.config)
		defaultValue := definition.Default(m.defaults)
		items = append(items, app.RuntimeConfigItem{
			Type: definition.Type, Path: definition.Path, Description: definition.Description,
			Value: value, Default: defaultValue, Version: m.versions[definition.Type],
			IsDefault: reflect.DeepEqual(value, defaultValue),
		})
	}
	return items
}

func (m *Manager) Update(ctx context.Context, configType string, data json.RawMessage, expectedVersion int) (app.RuntimeConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !isManagedType(configType) {
		return app.RuntimeConfig{}, fmt.Errorf("system config type %q is not editable", configType)
	}
	if expectedVersion != m.versions[configType] {
		return app.RuntimeConfig{}, store.ErrSystemConfigVersionConflict
	}
	candidate := m.config
	if err := decodeDomain(data, configType, &candidate); err != nil {
		return app.RuntimeConfig{}, err
	}
	if configType == app.SystemConfigAuth && candidate.Auth.AdminKeyHash != m.config.Auth.AdminKeyHash {
		return app.RuntimeConfig{}, errors.New("auth.admin_key_hash is managed by the authentication flow")
	}
	if err := candidate.Validate(); err != nil {
		return app.RuntimeConfig{}, err
	}
	canonical, err := json.Marshal(m.domain(candidate, configType))
	if err != nil {
		return app.RuntimeConfig{}, err
	}
	old := m.config
	if m.apply != nil {
		if err := m.apply(ctx, old, candidate); err != nil {
			return app.RuntimeConfig{}, err
		}
	}
	updated, err := m.store.UpdateSystemConfig(ctx, configType, canonical, expectedVersion)
	if err != nil {
		if m.apply != nil {
			_ = m.apply(ctx, candidate, old)
		}
		return app.RuntimeConfig{}, err
	}
	m.config = candidate
	m.versions[configType] = updated.Version
	for channel := range m.subs {
		select {
		case channel <- struct{}{}:
		default:
		}
	}
	return m.config, nil
}

func (m *Manager) UpdatePath(ctx context.Context, configType, path string, value json.RawMessage, expectedVersion int) (app.RuntimeConfig, error) {
	if !isManagedType(configType) {
		return app.RuntimeConfig{}, fmt.Errorf("system config type %q is not editable", configType)
	}
	m.mu.RLock()
	data, err := json.Marshal(m.domain(m.config, configType))
	m.mu.RUnlock()
	if err != nil {
		return app.RuntimeConfig{}, err
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return app.RuntimeConfig{}, err
	}
	path = trimDomainPath(configType, path)
	if path == "" {
		return app.RuntimeConfig{}, errors.New("system config path is required")
	}
	parts := strings.Split(path, ".")
	var replacement any
	if !json.Valid(value) {
		return app.RuntimeConfig{}, errors.New("system config value must be valid JSON")
	}
	if err := json.Unmarshal(value, &replacement); err != nil {
		return app.RuntimeConfig{}, err
	}
	current := object
	for _, part := range parts[:len(parts)-1] {
		child, ok := current[part].(map[string]any)
		if !ok {
			child = make(map[string]any)
			current[part] = child
		}
		current = child
	}
	current[parts[len(parts)-1]] = replacement
	updated, err := json.Marshal(object)
	if err != nil {
		return app.RuntimeConfig{}, err
	}
	return m.Update(ctx, configType, updated, expectedVersion)
}

func trimDomainPath(configType, path string) string {
	prefix := configType + "."
	return strings.TrimPrefix(path, prefix)
}

func (m *Manager) Reset(ctx context.Context, configType string) (app.RuntimeConfig, error) {
	m.mu.RLock()
	reset := m.defaults
	data, err := json.Marshal(m.domain(reset, configType))
	version := m.versions[configType]
	m.mu.RUnlock()
	if err != nil {
		return app.RuntimeConfig{}, err
	}
	return m.Update(ctx, configType, data, version)
}

func (m *Manager) ResetAll(ctx context.Context) (app.RuntimeConfig, error) {
	for _, configType := range managedTypes() {
		if _, err := m.Reset(ctx, configType); err != nil {
			return app.RuntimeConfig{}, err
		}
	}
	return m.Snapshot(), nil
}

// Import replaces every managed runtime domain as one logical change. The
// persistence callback can include the runtime rows in a wider transaction.
func (m *Manager) Import(ctx context.Context, candidate app.RuntimeConfig, persist ImportPersistFunc) (app.RuntimeConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if persist == nil {
		return app.RuntimeConfig{}, errors.New("runtime config import persistence is required")
	}
	candidate.Auth.AdminKeyHash = ""
	if err := candidate.Validate(); err != nil {
		return app.RuntimeConfig{}, err
	}
	domains := make(map[string]json.RawMessage, len(managedTypes()))
	for _, configType := range managedTypes() {
		encoded, err := json.Marshal(m.domain(candidate, configType))
		if err != nil {
			return app.RuntimeConfig{}, err
		}
		domains[configType] = encoded
	}
	old := m.config
	if m.apply != nil {
		if err := m.apply(ctx, old, candidate); err != nil {
			return app.RuntimeConfig{}, err
		}
	}
	versions, err := persist(ctx, domains)
	if err != nil {
		if m.apply != nil {
			_ = m.apply(ctx, candidate, old)
		}
		return app.RuntimeConfig{}, err
	}
	for _, configType := range managedTypes() {
		version, ok := versions[configType]
		if !ok || version < 1 {
			return app.RuntimeConfig{}, fmt.Errorf("runtime config import did not return version for %q", configType)
		}
		m.versions[configType] = version
	}
	m.config = candidate
	m.notifyLocked()
	return m.config, nil
}

func (m *Manager) notifyLocked() {
	for channel := range m.subs {
		select {
		case channel <- struct{}{}:
		default:
		}
	}
}

func (m *Manager) load(ctx context.Context) error {
	var config app.RuntimeConfig
	for _, configType := range managedTypes() {
		row, err := m.store.GetSystemConfig(ctx, configType)
		if err != nil {
			return err
		}
		if err := decodeDomain(row.Data, configType, &config); err != nil {
			return fmt.Errorf("decode system config %s: %w", configType, err)
		}
		m.versions[configType] = row.Version
	}
	if err := config.Validate(); err != nil {
		return err
	}
	// The key hash is intentionally never retained in the runtime manager
	// snapshot. Store updates preserve it inside the auth row.
	config.Auth.AdminKeyHash = ""
	m.config = config
	return nil
}

func (m *Manager) domain(config app.RuntimeConfig, configType string) any {
	switch configType {
	case app.SystemConfigStorage:
		return config.Storage
	case app.SystemConfigScheduler:
		return config.Scheduler
	case app.SystemConfigLogging:
		return config.Logging
	case app.SystemConfigPlugins:
		return config.Plugins
	case app.SystemConfigAuth:
		return config.Auth
	case app.SystemConfigMCP:
		return config.MCP
	default:
		return nil
	}
}

func isManagedType(configType string) bool {
	for _, value := range managedTypes() {
		if value == configType {
			return true
		}
	}
	return false
}

func decodeDomain(data []byte, configType string, config *app.RuntimeConfig) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var target any
	switch configType {
	case app.SystemConfigStorage:
		target = &config.Storage
	case app.SystemConfigScheduler:
		target = &config.Scheduler
	case app.SystemConfigLogging:
		target = &config.Logging
	case app.SystemConfigPlugins:
		target = &config.Plugins
	case app.SystemConfigAuth:
		target = &config.Auth
	case app.SystemConfigMCP:
		target = &config.MCP
	default:
		return fmt.Errorf("unknown system config type %q", configType)
	}
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("system config contains multiple JSON values")
		}
		return err
	}
	return nil
}
