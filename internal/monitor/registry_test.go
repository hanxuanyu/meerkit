package monitor

import (
	"context"
	"encoding/json"
	"testing"

	"meerkit/internal/core"
)

type registryTestModule string

func (m registryTestModule) Descriptor() core.ModuleDescriptor {
	return core.ModuleDescriptor{Type: string(m), Version: "1"}
}
func (registryTestModule) ValidateConfig(json.RawMessage) error { return nil }
func (registryTestModule) Execute(context.Context, json.RawMessage) (core.Observation, error) {
	return core.Observation{}, nil
}

func TestRegistryRejectsOwnerCollisionAndReplacesAtomically(t *testing.T) {
	registry := NewRegistry()
	if err := registry.ReplaceOwner("first", []core.MonitorModule{registryTestModule("http")}); err != nil {
		t.Fatal(err)
	}
	if err := registry.ReplaceOwner("second", []core.MonitorModule{registryTestModule("http")}); err == nil {
		t.Fatal("expected owner collision")
	}
	if err := registry.ReplaceOwner("first", []core.MonitorModule{registryTestModule("tcp")}); err != nil {
		t.Fatal(err)
	}
	if _, exists := registry.Get("http"); exists {
		t.Fatal("old owner module was not removed")
	}
	if owner, exists := registry.Owner("tcp"); !exists || owner != "first" {
		t.Fatalf("owner = %q, %v", owner, exists)
	}
}
