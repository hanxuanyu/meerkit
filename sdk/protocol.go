package sdk

import (
	"context"
	"encoding/json"
	"fmt"

	plugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const ProtocolVersion = 1

var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  ProtocolVersion,
	MagicCookieKey:   "MEERKIT_MONITOR_PLUGIN",
	MagicCookieValue: "meerkit-monitor-v1",
}

type request struct {
	ModuleType  string          `json:"module_type,omitempty"`
	FromVersion string          `json:"from_version,omitempty"`
	ToVersion   string          `json:"to_version,omitempty"`
	Config      json.RawMessage `json:"config,omitempty"`
}

type response struct {
	Modules     []ModuleDescriptor `json:"modules,omitempty"`
	Observation *Observation       `json:"observation,omitempty"`
	Config      json.RawMessage    `json:"config,omitempty"`
	Error       string             `json:"error,omitempty"`
}

type MonitorPlugin struct {
	plugin.Plugin
	Impl    Provider
	Runtime *PluginRuntime
}

func (p *MonitorPlugin) GRPCServer(_ *plugin.GRPCBroker, server *grpc.Server) error {
	RegisterMonitorServer(server, &monitorServer{provider: p.Impl})
	if p.Runtime != nil {
		RegisterBrowserBridgeServer(server, p.Runtime.browserServer())
	}
	return nil
}

func (p *MonitorPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, conn *grpc.ClientConn) (interface{}, error) {
	return &monitorClient{client: NewMonitorServiceClient(conn)}, nil
}

type monitorServer struct{ provider Provider }

func (s *monitorServer) ListModules(ctx context.Context, in *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error) {
	modules, err := s.provider.ListModules()
	if err == nil {
		err = validateModuleDescriptors(modules)
	}
	return marshalResponse(response{Modules: modules}, err)
}
func (s *monitorServer) ValidateConfig(ctx context.Context, in *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error) {
	var req request
	if err := unmarshalRequest(in, &req); err != nil {
		return nil, err
	}
	err := s.provider.ValidateConfig(ctx, req.ModuleType, req.Config)
	return marshalResponse(response{}, err)
}
func (s *monitorServer) Execute(ctx context.Context, in *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error) {
	var req request
	if err := unmarshalRequest(in, &req); err != nil {
		return nil, err
	}
	observation, err := s.provider.Execute(ctx, req.ModuleType, req.Config)
	return marshalResponse(response{Observation: &observation}, err)
}
func (s *monitorServer) MigrateConfig(ctx context.Context, in *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error) {
	var req request
	if err := unmarshalRequest(in, &req); err != nil {
		return nil, err
	}
	config, err := s.provider.MigrateConfig(ctx, req.ModuleType, req.FromVersion, req.ToVersion, req.Config)
	return marshalResponse(response{Config: config}, err)
}
func (s *monitorServer) Health(ctx context.Context, in *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error) {
	return marshalResponse(response{}, s.provider.Health(ctx))
}

func marshalResponse(value response, err error) (*wrapperspb.BytesValue, error) {
	if err != nil {
		value.Error = err.Error()
	}
	data, marshalErr := json.Marshal(value)
	if marshalErr != nil {
		return nil, marshalErr
	}
	return wrapperspb.Bytes(data), nil
}
func unmarshalRequest(value *wrapperspb.BytesValue, target *request) error {
	if value == nil {
		return fmt.Errorf("missing plugin request")
	}
	return json.Unmarshal(value.Value, target)
}

type MonitorServiceClient interface {
	ListModules(context.Context, *wrapperspb.BytesValue, ...grpc.CallOption) (*wrapperspb.BytesValue, error)
	ValidateConfig(context.Context, *wrapperspb.BytesValue, ...grpc.CallOption) (*wrapperspb.BytesValue, error)
	Execute(context.Context, *wrapperspb.BytesValue, ...grpc.CallOption) (*wrapperspb.BytesValue, error)
	MigrateConfig(context.Context, *wrapperspb.BytesValue, ...grpc.CallOption) (*wrapperspb.BytesValue, error)
	Health(context.Context, *wrapperspb.BytesValue, ...grpc.CallOption) (*wrapperspb.BytesValue, error)
}

type monitorClient struct{ client MonitorServiceClient }

func (c *monitorClient) ListModules() ([]ModuleDescriptor, error) {
	result, err := c.client.ListModules(context.Background(), wrapperspb.Bytes(nil))
	if err != nil {
		return nil, err
	}
	var value response
	if err := decodeResponse(result, &value); err != nil {
		return nil, err
	}
	return value.Modules, responseError(value)
}
func (c *monitorClient) ValidateConfig(ctx context.Context, moduleType string, config json.RawMessage) error {
	result, err := c.call(ctx, "/validate", request{ModuleType: moduleType, Config: config})
	if err != nil {
		return err
	}
	var value response
	if err := json.Unmarshal(result, &value); err != nil {
		return err
	}
	return responseError(value)
}
func (c *monitorClient) Execute(ctx context.Context, moduleType string, config json.RawMessage) (Observation, error) {
	result, err := c.call(ctx, "/execute", request{ModuleType: moduleType, Config: config})
	if err != nil {
		return Observation{}, err
	}
	var value response
	if err := json.Unmarshal(result, &value); err != nil {
		return Observation{}, err
	}
	if err := responseError(value); err != nil {
		return Observation{}, err
	}
	if value.Observation == nil {
		return Observation{}, fmt.Errorf("plugin returned no observation")
	}
	return *value.Observation, nil
}
func (c *monitorClient) MigrateConfig(ctx context.Context, moduleType, fromVersion, toVersion string, config json.RawMessage) (json.RawMessage, error) {
	result, err := c.call(ctx, "/migrate", request{ModuleType: moduleType, FromVersion: fromVersion, ToVersion: toVersion, Config: config})
	if err != nil {
		return nil, err
	}
	var value response
	if err := json.Unmarshal(result, &value); err != nil {
		return nil, err
	}
	if err := responseError(value); err != nil {
		return nil, err
	}
	return value.Config, nil
}
func (c *monitorClient) Health(ctx context.Context) error {
	result, err := c.call(ctx, "/health", request{})
	if err != nil {
		return err
	}
	var value response
	if err := json.Unmarshal(result, &value); err != nil {
		return err
	}
	return responseError(value)
}
func (c *monitorClient) call(ctx context.Context, method string, value request) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	in := wrapperspb.Bytes(data)
	var out *wrapperspb.BytesValue
	switch method {
	case "/validate":
		out, err = c.client.ValidateConfig(ctx, in)
	case "/execute":
		out, err = c.client.Execute(ctx, in)
	case "/migrate":
		out, err = c.client.MigrateConfig(ctx, in)
	case "/health":
		out, err = c.client.Health(ctx, in)
	default:
		return nil, fmt.Errorf("unknown plugin method %s", method)
	}
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, fmt.Errorf("plugin returned an empty response")
	}
	return out.Value, nil
}
func decodeResponse(value *wrapperspb.BytesValue, target *response) error {
	if value == nil {
		return fmt.Errorf("plugin returned an empty response")
	}
	if err := json.Unmarshal(value.Value, target); err != nil {
		return err
	}
	return nil
}
func responseError(value response) error {
	if value.Error != "" {
		return fmt.Errorf("plugin: %s", value.Error)
	}
	return nil
}
