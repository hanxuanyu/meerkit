package browser

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/hanxuanyu/meerkit/sdk"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type capabilityServer struct {
	manager *Manager
	token   string
}

func (s *capabilityServer) Run(ctx context.Context, input *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error) {
	if !authorized(ctx, s.token) {
		return nil, status.Error(codes.Unauthenticated, "browser capability authorization failed")
	}
	var request sdk.BrowserRunRequest
	if input == nil || json.Unmarshal(input.Value, &request) != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid browser capability request")
	}
	result, err := s.manager.Execute(ctx, request)
	return sdk.MarshalBrowserCapabilityResponse(&result, err)
}

func authorized(ctx context.Context, expected string) bool {
	values := metadata.ValueFromIncomingContext(ctx, "authorization")
	if len(values) != 1 {
		return false
	}
	value := strings.TrimSpace(values[0])
	if !strings.HasPrefix(value, "Bearer ") {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))), []byte(expected)) == 1
}

type CapabilityServer struct {
	listener net.Listener
	server   *grpc.Server
	Endpoint string
	Token    string
}

func StartCapabilityServer(manager *Manager, token string) (*CapabilityServer, error) {
	if manager == nil || strings.TrimSpace(token) == "" {
		return nil, errors.New("browser capability manager and token are required")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen browser capability: %w", err)
	}
	server := grpc.NewServer()
	sdk.RegisterBrowserCapabilityServer(server, &capabilityServer{manager: manager, token: token})
	result := &CapabilityServer{listener: listener, server: server, Endpoint: listener.Addr().String(), Token: token}
	go func() { _ = server.Serve(listener) }()
	return result, nil
}

func (s *CapabilityServer) Close() {
	if s == nil {
		return
	}
	s.server.GracefulStop()
	_ = s.listener.Close()
}
