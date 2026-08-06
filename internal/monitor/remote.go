package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/hanxuanyu/meerkit/sdk"
	"meerkit/internal/core"
)

type ExecutionGate struct {
	accepting atomic.Bool
	inflight  sync.WaitGroup
}

func NewExecutionGate() *ExecutionGate {
	gate := &ExecutionGate{}
	gate.accepting.Store(true)
	return gate
}
func (g *ExecutionGate) Stop()  { g.accepting.Store(false) }
func (g *ExecutionGate) Start() { g.accepting.Store(true) }
func (g *ExecutionGate) Wait()  { g.inflight.Wait() }

type remoteModule struct {
	descriptor       core.ModuleDescriptor
	remoteDescriptor sdk.ModuleDescriptor
	client           sdk.Provider
	gate             *ExecutionGate
}

func NewRemoteModule(client sdk.Provider, descriptor sdk.ModuleDescriptor, gate *ExecutionGate) (core.MonitorModule, error) {
	var coreDescriptor core.ModuleDescriptor
	if err := convert(descriptor, &coreDescriptor); err != nil {
		return nil, fmt.Errorf("decode descriptor %s: %w", descriptor.Type, err)
	}
	return &remoteModule{descriptor: coreDescriptor, remoteDescriptor: descriptor, client: client, gate: gate}, nil
}
func (m *remoteModule) Descriptor() core.ModuleDescriptor { return m.descriptor }
func (m *remoteModule) ValidateConfig(config json.RawMessage) error {
	return m.client.ValidateConfig(context.Background(), m.remoteDescriptor.Type, config)
}
func (m *remoteModule) Execute(ctx context.Context, config json.RawMessage) (core.Observation, error) {
	if !m.gate.accepting.Load() {
		return core.Observation{}, fmt.Errorf("plugin module %s is stopping", m.descriptor.Type)
	}
	m.gate.inflight.Add(1)
	defer m.gate.inflight.Done()
	observation, err := m.client.Execute(ctx, m.remoteDescriptor.Type, config)
	var result core.Observation
	if convertErr := convert(observation, &result); convertErr != nil {
		return core.Observation{}, convertErr
	}
	return result, err
}
func convert(source, target any) error {
	data, err := json.Marshal(source)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
