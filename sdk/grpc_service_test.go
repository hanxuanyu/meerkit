package sdk

import (
	"context"
	"os"
	"regexp"
	"sort"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type serviceCapture struct {
	description *grpc.ServiceDesc
}

func (c *serviceCapture) RegisterService(description *grpc.ServiceDesc, _ any) {
	c.description = description
}

type protocolTestServer struct{}

func (protocolTestServer) ListModules(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error) {
	return wrapperspb.Bytes(nil), nil
}
func (protocolTestServer) ValidateConfig(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error) {
	return wrapperspb.Bytes(nil), nil
}
func (protocolTestServer) Execute(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error) {
	return wrapperspb.Bytes(nil), nil
}
func (protocolTestServer) MigrateConfig(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error) {
	return wrapperspb.Bytes(nil), nil
}
func (protocolTestServer) Health(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error) {
	return wrapperspb.Bytes(nil), nil
}

func TestRegisteredGRPCServiceMatchesPublishedProto(t *testing.T) {
	data, err := os.ReadFile("proto/monitor.proto")
	if err != nil {
		t.Fatal(err)
	}
	matches := regexp.MustCompile(`(?m)^\s*rpc\s+([A-Za-z0-9_]+)\(`).FindAllSubmatch(data, -1)
	protoMethods := make([]string, 0, len(matches))
	for _, match := range matches {
		protoMethods = append(protoMethods, string(match[1]))
	}
	capture := &serviceCapture{}
	RegisterMonitorServer(capture, protocolTestServer{})
	if capture.description == nil || capture.description.ServiceName != "meerkit.sdk.Monitor" {
		t.Fatalf("unexpected service description: %#v", capture.description)
	}
	registeredMethods := make([]string, 0, len(capture.description.Methods))
	for _, method := range capture.description.Methods {
		registeredMethods = append(registeredMethods, method.MethodName)
	}
	sort.Strings(protoMethods)
	sort.Strings(registeredMethods)
	if len(protoMethods) != len(registeredMethods) {
		t.Fatalf("proto methods %v do not match registered methods %v", protoMethods, registeredMethods)
	}
	for index := range protoMethods {
		if protoMethods[index] != registeredMethods[index] {
			t.Fatalf("proto methods %v do not match registered methods %v", protoMethods, registeredMethods)
		}
	}
}
