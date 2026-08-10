package pluginconformance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hanxuanyu/meerkit/sdk"
	protocolschema "github.com/hanxuanyu/meerkit/sdk/schema"
	"github.com/hashicorp/go-hclog"
	hplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"meerkit/internal/plugin"
)

type Suite struct {
	Cases []Case `json:"cases"`
}

type Case struct {
	Name                  string                `json:"name"`
	ModuleType            string                `json:"module_type"`
	Config                json.RawMessage       `json:"config"`
	ExpectValidationError bool                  `json:"expect_validation_error,omitempty"`
	Execute               *ExecuteExpectation   `json:"execute,omitempty"`
	Migration             *MigrationExpectation `json:"migration,omitempty"`
}

type ExecuteExpectation struct {
	ExpectError bool  `json:"expect_error,omitempty"`
	Success     *bool `json:"success,omitempty"`
}

type MigrationExpectation struct {
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
	ExpectError bool   `json:"expect_error,omitempty"`
}

type Options struct {
	Manifest  plugin.Manifest
	Artifact  string
	Suite     Suite
	Timeout   time.Duration
	LogOutput io.Writer
}

type Report struct {
	Checks []string
}

type validators struct {
	request     *jsonschema.Resolved
	response    *jsonschema.Resolved
	observation *jsonschema.Resolved
	suite       *jsonschema.Resolved
}

type wireClient struct {
	client     sdk.MonitorServiceClient
	validators validators
}

func LoadSuite(path string) (Suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Suite{}, err
	}
	all, err := loadValidators()
	if err != nil {
		return Suite{}, err
	}
	value, err := decodeJSON(data)
	if err != nil {
		return Suite{}, fmt.Errorf("decode conformance suite: %w", err)
	}
	if err := all.suite.Validate(value); err != nil {
		return Suite{}, fmt.Errorf("validate conformance suite: %w", err)
	}
	var suite Suite
	if err := json.Unmarshal(data, &suite); err != nil {
		return Suite{}, err
	}
	return suite, nil
}

func Check(ctx context.Context, options Options) (Report, error) {
	if options.Timeout <= 0 {
		options.Timeout = 10 * time.Second
	}
	if options.LogOutput == nil {
		options.LogOutput = io.Discard
	}
	if err := options.Manifest.Validate(sdk.ProtocolVersion); err != nil {
		return Report{}, fmt.Errorf("manifest: %w", err)
	}
	artifact, err := filepath.Abs(options.Artifact)
	if err != nil {
		return Report{}, err
	}
	info, err := os.Stat(artifact)
	if err != nil {
		return Report{}, fmt.Errorf("artifact: %w", err)
	}
	if !info.Mode().IsRegular() {
		return Report{}, fmt.Errorf("artifact %s is not a regular file", artifact)
	}
	var currentArtifact *plugin.Artifact
	if len(options.Manifest.Artifacts) != 0 {
		current, currentErr := options.Manifest.CurrentArtifact()
		if currentErr != nil {
			return Report{}, currentErr
		}
		currentArtifact = &current
	}
	runtimeConfig, err := options.Manifest.ResolveRuntime(currentArtifact)
	if err != nil {
		return Report{}, err
	}
	command, err := plugin.NewArtifactCommand(artifact, runtimeConfig)
	if err != nil {
		return Report{}, err
	}
	command.Env = append(os.Environ(),
		"MEERKIT_PLUGIN_ID="+options.Manifest.ID,
		"MEERKIT_PLUGIN_NAME="+options.Manifest.Name,
		"MEERKIT_PLUGIN_VERSION="+options.Manifest.Version,
		"MEERKIT_PLUGIN_LOG_LEVEL=debug",
		"MEERKIT_PLUGIN_LOG_FORMAT=simple",
	)
	client := hplugin.NewClient(&hplugin.ClientConfig{
		HandshakeConfig:  sdk.Handshake,
		Plugins:          map[string]hplugin.Plugin{"monitor": &sdk.MonitorPlugin{}},
		Cmd:              command,
		AllowedProtocols: []hplugin.Protocol{hplugin.ProtocolGRPC},
		Managed:          true,
		StartTimeout:     options.Timeout,
		Stderr:           options.LogOutput,
		SyncStdout:       options.LogOutput,
		SyncStderr:       options.LogOutput,
		Logger:           hclog.NewNullLogger(),
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	if err != nil {
		return Report{}, fmt.Errorf("go-plugin handshake: %w", err)
	}
	grpcClient, ok := rpcClient.(*hplugin.GRPCClient)
	if !ok {
		return Report{}, fmt.Errorf("plugin negotiated a non-gRPC protocol")
	}
	report := Report{Checks: []string{"go-plugin handshake"}}
	standardHealthCtx, standardHealthCancel := context.WithTimeout(ctx, options.Timeout)
	standardHealth, err := grpc_health_v1.NewHealthClient(grpcClient.Conn).Check(standardHealthCtx, &grpc_health_v1.HealthCheckRequest{Service: hplugin.GRPCServiceName})
	standardHealthCancel()
	if err != nil {
		return report, fmt.Errorf("standard gRPC health check: %w", err)
	}
	if standardHealth.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return report, fmt.Errorf("standard gRPC health status is %s, want SERVING", standardHealth.Status)
	}
	report.Checks = append(report.Checks, "standard gRPC health")
	all, err := loadValidators()
	if err != nil {
		return Report{}, err
	}
	wire := wireClient{client: sdk.NewMonitorServiceClient(grpcClient.Conn), validators: all}

	healthCtx, healthCancel := context.WithTimeout(ctx, options.Timeout)
	healthResponse, err := wire.call(healthCtx, "Health", map[string]any{})
	healthCancel()
	if err != nil {
		return report, err
	}
	if message := responseError(healthResponse); message != "" {
		return report, fmt.Errorf("Health returned an application error: %s", message)
	}
	report.Checks = append(report.Checks, "Meerkit application health")

	modulesCtx, modulesCancel := context.WithTimeout(ctx, options.Timeout)
	modulesResponse, err := wire.call(modulesCtx, "ListModules", nil)
	modulesCancel()
	if err != nil {
		return report, err
	}
	modules, err := decodeModules(modulesResponse)
	if err != nil {
		return report, err
	}
	if err := compareModules(options.Manifest, modules); err != nil {
		return report, err
	}
	report.Checks = append(report.Checks, "module response schema", "manifest module agreement")

	declared := make(map[string]string, len(options.Manifest.Modules))
	for _, module := range options.Manifest.Modules {
		declared[module.Type] = module.ResultSchemaVersion
	}
	for _, testCase := range options.Suite.Cases {
		caseCtx, caseCancel := context.WithTimeout(ctx, options.Timeout)
		err := wire.checkCase(caseCtx, testCase, declared)
		caseCancel()
		if err != nil {
			return report, fmt.Errorf("case %q: %w", testCase.Name, err)
		}
		report.Checks = append(report.Checks, "case "+testCase.Name)
	}
	return report, nil
}

func (c wireClient) checkCase(ctx context.Context, testCase Case, resultVersions map[string]string) error {
	if _, exists := resultVersions[testCase.ModuleType]; !exists {
		return fmt.Errorf("module %q is not declared by the manifest", testCase.ModuleType)
	}
	config, err := rawJSONValue(testCase.Config)
	if err != nil {
		return err
	}
	validation, err := c.call(ctx, "ValidateConfig", map[string]any{"module_type": testCase.ModuleType, "config": config})
	if err != nil {
		return err
	}
	if err := matchApplicationError(validation, testCase.ExpectValidationError, "validation"); err != nil {
		return err
	}
	if testCase.Execute != nil {
		execution, err := c.call(ctx, "Execute", map[string]any{"module_type": testCase.ModuleType, "config": config})
		if err != nil {
			return err
		}
		if err := matchApplicationError(execution, testCase.Execute.ExpectError, "execution"); err != nil {
			return err
		}
		if !testCase.Execute.ExpectError {
			raw, exists := execution["observation"]
			if !exists {
				return errors.New("successful execution response has no observation")
			}
			value, err := decodeJSON(raw)
			if err != nil {
				return fmt.Errorf("decode observation: %w", err)
			}
			if err := c.validators.observation.Validate(value); err != nil {
				return fmt.Errorf("observation schema: %w", err)
			}
			var observation sdk.Observation
			if err := json.Unmarshal(raw, &observation); err != nil {
				return err
			}
			if observation.SchemaVersion != resultVersions[testCase.ModuleType] {
				return fmt.Errorf("observation schema version %q does not match manifest version %q", observation.SchemaVersion, resultVersions[testCase.ModuleType])
			}
			if testCase.Execute.Success != nil && observation.Success != *testCase.Execute.Success {
				return fmt.Errorf("observation success is %t, want %t", observation.Success, *testCase.Execute.Success)
			}
		}
	}
	if testCase.Migration != nil {
		migration, err := c.call(ctx, "MigrateConfig", map[string]any{
			"module_type":  testCase.ModuleType,
			"from_version": testCase.Migration.FromVersion,
			"to_version":   testCase.Migration.ToVersion,
			"config":       config,
		})
		if err != nil {
			return err
		}
		if err := matchApplicationError(migration, testCase.Migration.ExpectError, "migration"); err != nil {
			return err
		}
		if !testCase.Migration.ExpectError {
			if _, exists := migration["config"]; !exists {
				return errors.New("successful migration response has no config field")
			}
		}
	}
	return nil
}

func (c wireClient) call(ctx context.Context, method string, request any) (map[string]json.RawMessage, error) {
	var input *wrapperspb.BytesValue
	if request == nil {
		input = wrapperspb.Bytes(nil)
	} else {
		data, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}
		value, err := decodeJSON(data)
		if err != nil {
			return nil, err
		}
		if err := c.validators.request.Validate(value); err != nil {
			return nil, fmt.Errorf("%s request schema: %w", method, err)
		}
		input = wrapperspb.Bytes(data)
	}
	var output *wrapperspb.BytesValue
	var err error
	switch method {
	case "ListModules":
		output, err = c.client.ListModules(ctx, input)
	case "ValidateConfig":
		output, err = c.client.ValidateConfig(ctx, input)
	case "Execute":
		output, err = c.client.Execute(ctx, input)
	case "MigrateConfig":
		output, err = c.client.MigrateConfig(ctx, input)
	case "Health":
		output, err = c.client.Health(ctx, input)
	default:
		return nil, fmt.Errorf("unknown method %s", method)
	}
	if err != nil {
		return nil, fmt.Errorf("%s RPC: %w", method, err)
	}
	if output == nil || len(output.Value) == 0 {
		return nil, fmt.Errorf("%s returned an empty response", method)
	}
	value, err := decodeJSON(output.Value)
	if err != nil {
		return nil, fmt.Errorf("decode %s response: %w", method, err)
	}
	if err := c.validators.response.Validate(value); err != nil {
		return nil, fmt.Errorf("%s response schema: %w", method, err)
	}
	var response map[string]json.RawMessage
	if err := json.Unmarshal(output.Value, &response); err != nil {
		return nil, err
	}
	return response, nil
}

func loadValidators() (validators, error) {
	load := func(name string) (*jsonschema.Resolved, error) {
		data, err := protocolschema.Files.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var value jsonschema.Schema
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, fmt.Errorf("decode %s: %w", name, err)
		}
		return value.Resolve(&jsonschema.ResolveOptions{Loader: func(uri *url.URL) (*jsonschema.Schema, error) {
			data, err := protocolschema.Files.ReadFile(filepath.Base(uri.Path))
			if err != nil {
				return nil, err
			}
			var referenced jsonschema.Schema
			if err := json.Unmarshal(data, &referenced); err != nil {
				return nil, err
			}
			return &referenced, nil
		}})
	}
	request, err := load("request.schema.json")
	if err != nil {
		return validators{}, err
	}
	response, err := load("response.schema.json")
	if err != nil {
		return validators{}, err
	}
	observation, err := load("observation.schema.json")
	if err != nil {
		return validators{}, err
	}
	suite, err := load("conformance-suite.schema.json")
	if err != nil {
		return validators{}, err
	}
	return validators{request: request, response: response, observation: observation, suite: suite}, nil
}

func decodeJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, errors.New("JSON contains trailing data")
	}
	return value, nil
}

func rawJSONValue(data json.RawMessage) (any, error) {
	if len(data) == 0 {
		return nil, errors.New("config is missing")
	}
	value, err := decodeJSON(data)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return value, nil
}

func decodeModules(response map[string]json.RawMessage) ([]sdk.ModuleDescriptor, error) {
	if message := responseError(response); message != "" {
		return nil, fmt.Errorf("ListModules: %s", message)
	}
	raw, exists := response["modules"]
	if !exists {
		return nil, errors.New("successful ListModules response has no modules field")
	}
	var modules []sdk.ModuleDescriptor
	if err := json.Unmarshal(raw, &modules); err != nil {
		return nil, err
	}
	return modules, nil
}

func compareModules(manifest plugin.Manifest, actual []sdk.ModuleDescriptor) error {
	expected := make(map[string]string, len(manifest.Modules))
	for _, module := range manifest.Modules {
		expected[module.Type] = module.Version
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("plugin returned %d modules, manifest declares %d", len(actual), len(expected))
	}
	seen := make(map[string]struct{}, len(actual))
	for _, module := range actual {
		version, exists := expected[module.Type]
		if !exists || version != module.Version {
			return fmt.Errorf("module %s version %s does not match manifest", module.Type, module.Version)
		}
		if _, duplicate := seen[module.Type]; duplicate {
			return fmt.Errorf("plugin returned duplicate module %s", module.Type)
		}
		seen[module.Type] = struct{}{}
	}
	return nil
}

func matchApplicationError(response map[string]json.RawMessage, expected bool, operation string) error {
	message := responseError(response)
	if expected && message == "" {
		return fmt.Errorf("%s succeeded, want an application error", operation)
	}
	if !expected && message != "" {
		return fmt.Errorf("%s returned an application error: %s", operation, message)
	}
	return nil
}

func responseError(response map[string]json.RawMessage) string {
	raw, exists := response["error"]
	if !exists {
		return ""
	}
	var message string
	_ = json.Unmarshal(raw, &message)
	return message
}
