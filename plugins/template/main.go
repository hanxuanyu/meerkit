package main

import (
	"context"
	"encoding/json"
	"github.com/hanxuanyu/meerkit/sdk"
)

type module struct{}

func (module) Descriptor() sdk.ModuleDescriptor {
	return sdk.ModuleDescriptor{Type: "example", Version: "1", Name: "Example", Description: "Meerkit monitor plugin template.", ConfigSchema: map[string]any{"type": "object"}, ResultSchema: map[string]any{"type": "object"}, Parameters: []sdk.ParameterDescriptor{}, Fields: []sdk.FieldDescriptor{}}
}
func (module) ValidateConfig(json.RawMessage) error { return nil }
func (module) Execute(context.Context, json.RawMessage) (sdk.Observation, error) {
	return sdk.Observation{Success: true, SchemaVersion: "1", Result: map[string]any{"message": "ok"}, Summary: "Example check completed"}, nil
}
func main() { sdk.Serve(sdk.NewProvider(module{})) }
