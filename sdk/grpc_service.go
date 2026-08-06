package sdk

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func NewMonitorServiceClient(conn grpc.ClientConnInterface) MonitorServiceClient {
	return &monitorServiceClient{conn: conn}
}

type monitorServiceClient struct{ conn grpc.ClientConnInterface }

func (c *monitorServiceClient) invoke(ctx context.Context, method string, in *wrapperspb.BytesValue, opts ...grpc.CallOption) (*wrapperspb.BytesValue, error) {
	out := new(wrapperspb.BytesValue)
	err := c.conn.Invoke(ctx, method, in, out, opts...)
	return out, err
}
func (c *monitorServiceClient) ListModules(ctx context.Context, in *wrapperspb.BytesValue, opts ...grpc.CallOption) (*wrapperspb.BytesValue, error) {
	return c.invoke(ctx, "/meerkit.sdk.Monitor/ListModules", in, opts...)
}
func (c *monitorServiceClient) ValidateConfig(ctx context.Context, in *wrapperspb.BytesValue, opts ...grpc.CallOption) (*wrapperspb.BytesValue, error) {
	return c.invoke(ctx, "/meerkit.sdk.Monitor/ValidateConfig", in, opts...)
}
func (c *monitorServiceClient) Execute(ctx context.Context, in *wrapperspb.BytesValue, opts ...grpc.CallOption) (*wrapperspb.BytesValue, error) {
	return c.invoke(ctx, "/meerkit.sdk.Monitor/Execute", in, opts...)
}
func (c *monitorServiceClient) MigrateConfig(ctx context.Context, in *wrapperspb.BytesValue, opts ...grpc.CallOption) (*wrapperspb.BytesValue, error) {
	return c.invoke(ctx, "/meerkit.sdk.Monitor/MigrateConfig", in, opts...)
}
func (c *monitorServiceClient) Health(ctx context.Context, in *wrapperspb.BytesValue, opts ...grpc.CallOption) (*wrapperspb.BytesValue, error) {
	return c.invoke(ctx, "/meerkit.sdk.Monitor/Health", in, opts...)
}

type MonitorServer interface {
	ListModules(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error)
	ValidateConfig(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error)
	Execute(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error)
	MigrateConfig(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error)
	Health(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error)
}

func RegisterMonitorServer(server grpc.ServiceRegistrar, implementation MonitorServer) {
	server.RegisterService(&grpc.ServiceDesc{ServiceName: "meerkit.sdk.Monitor", HandlerType: (*MonitorServer)(nil), Methods: []grpc.MethodDesc{{MethodName: "ListModules", Handler: handler("ListModules", implementation.ListModules)}, {MethodName: "ValidateConfig", Handler: handler("ValidateConfig", implementation.ValidateConfig)}, {MethodName: "Execute", Handler: handler("Execute", implementation.Execute)}, {MethodName: "MigrateConfig", Handler: handler("MigrateConfig", implementation.MigrateConfig)}, {MethodName: "Health", Handler: handler("Health", implementation.Health)}}, Streams: []grpc.StreamDesc{}, Metadata: "meerkit-plugin"}, implementation)
}
func handler(method string, fn func(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error)) func(any, context.Context, func(any) error, grpc.UnaryServerInterceptor) (any, error) {
	return func(server any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		in := new(wrapperspb.BytesValue)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return fn(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: server, FullMethod: "/meerkit.sdk.Monitor/" + method}
		return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) { return fn(ctx, req.(*wrapperspb.BytesValue)) })
	}
}
