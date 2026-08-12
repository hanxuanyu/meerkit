package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	BrowserCapabilityEndpointEnv = "MEERKIT_BROWSER_CAPABILITY_ENDPOINT"
	BrowserCapabilityTokenEnv    = "MEERKIT_BROWSER_CAPABILITY_TOKEN"
	BrowserProtocolVersion       = 1
	browserRunMethod             = "/meerkit.sdk.BrowserCapability/Run"
)

type BrowserAction struct {
	ID              string         `json:"id"`
	Type            string         `json:"type"`
	Params          map[string]any `json:"params,omitempty"`
	ContinueOnError bool           `json:"continue_on_error,omitempty"`
}

type BrowserNetworkCapture struct {
	ID           string `json:"id"`
	URLContains  string `json:"url_contains"`
	ResourceType string `json:"resource_type,omitempty"`
	MaxBodyBytes int    `json:"max_body_bytes,omitempty"`
}

type BrowserRunRequest struct {
	AgentID         string                  `json:"agent_id,omitempty"`
	TabID           int                     `json:"tab_id,omitempty"`
	WindowID        int                     `json:"window_id,omitempty"`
	TimeoutMS       int                     `json:"timeout_ms,omitempty"`
	KeepTab         bool                    `json:"keep_tab,omitempty"`
	Actions         []BrowserAction         `json:"actions"`
	NetworkCaptures []BrowserNetworkCapture `json:"network_captures,omitempty"`
}

type BrowserActionResult struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Success  bool           `json:"success"`
	Duration int64          `json:"duration_ms,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
	Error    string         `json:"error,omitempty"`
}

type BrowserNetworkResult struct {
	CaptureID            string             `json:"capture_id"`
	URL                  string             `json:"url"`
	Method               string             `json:"method,omitempty"`
	Status               int                `json:"status,omitempty"`
	StatusText           string             `json:"status_text,omitempty"`
	ResourceType         string             `json:"resource_type,omitempty"`
	MimeType             string             `json:"mime_type,omitempty"`
	Protocol             string             `json:"protocol,omitempty"`
	RemoteIPAddress      string             `json:"remote_ip_address,omitempty"`
	RemotePort           int                `json:"remote_port,omitempty"`
	InitiatorType        string             `json:"initiator_type,omitempty"`
	Headers              map[string]string  `json:"headers,omitempty"`
	RequestHeaders       map[string]string  `json:"request_headers,omitempty"`
	RequestBody          string             `json:"request_body,omitempty"`
	RequestBodyTruncated bool               `json:"request_body_truncated,omitempty"`
	Body                 string             `json:"body,omitempty"`
	BodyBase64           bool               `json:"body_base64,omitempty"`
	Truncated            bool               `json:"truncated,omitempty"`
	EncodedDataLength    int64              `json:"encoded_data_length,omitempty"`
	Duration             int64              `json:"duration_ms,omitempty"`
	FromDiskCache        bool               `json:"from_disk_cache,omitempty"`
	FromServiceWorker    bool               `json:"from_service_worker,omitempty"`
	Timing               map[string]float64 `json:"timing,omitempty"`
	Error                string             `json:"error,omitempty"`
}

type BrowserRunResult struct {
	AgentID  string                 `json:"agent_id"`
	TabID    int                    `json:"tab_id,omitempty"`
	WindowID int                    `json:"window_id,omitempty"`
	Duration int64                  `json:"duration_ms"`
	Actions  []BrowserActionResult  `json:"actions"`
	Network  []BrowserNetworkResult `json:"network,omitempty"`
}

type browserCapabilityResponse struct {
	Result *BrowserRunResult `json:"result,omitempty"`
	Error  string            `json:"error,omitempty"`
}

type BrowserClient interface {
	Run(context.Context, BrowserRunRequest) (BrowserRunResult, error)
	Close() error
}

type browserClient struct {
	connection *grpc.ClientConn
	token      string
}

func NewBrowserClient(endpoint, token string) (BrowserClient, error) {
	endpoint, token = strings.TrimSpace(endpoint), strings.TrimSpace(token)
	if endpoint == "" || token == "" {
		return nil, errors.New("browser capability endpoint and token are required")
	}
	connection, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connect browser capability: %w", err)
	}
	return &browserClient{connection: connection, token: token}, nil
}

func NewBrowserClientFromEnvironment() (BrowserClient, error) {
	return NewBrowserClient(os.Getenv(BrowserCapabilityEndpointEnv), os.Getenv(BrowserCapabilityTokenEnv))
}

func (c *browserClient) Run(ctx context.Context, request BrowserRunRequest) (BrowserRunResult, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return BrowserRunResult{}, err
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.token)
	response := new(wrapperspb.BytesValue)
	if err := c.connection.Invoke(ctx, browserRunMethod, wrapperspb.Bytes(data), response); err != nil {
		return BrowserRunResult{}, err
	}
	var envelope browserCapabilityResponse
	if err := json.Unmarshal(response.Value, &envelope); err != nil {
		return BrowserRunResult{}, fmt.Errorf("decode browser capability response: %w", err)
	}
	if envelope.Error != "" {
		return BrowserRunResult{}, errors.New(envelope.Error)
	}
	if envelope.Result == nil {
		return BrowserRunResult{}, errors.New("browser capability returned no result")
	}
	return *envelope.Result, nil
}

func (c *browserClient) Close() error { return c.connection.Close() }

type BrowserCapabilityServer interface {
	Run(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error)
}

func RegisterBrowserCapabilityServer(server grpc.ServiceRegistrar, implementation BrowserCapabilityServer) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: "meerkit.sdk.BrowserCapability",
		HandlerType: (*BrowserCapabilityServer)(nil),
		Methods: []grpc.MethodDesc{{MethodName: "Run", Handler: func(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
			input := new(wrapperspb.BytesValue)
			if err := decode(input); err != nil {
				return nil, err
			}
			if interceptor == nil {
				return implementation.Run(ctx, input)
			}
			info := &grpc.UnaryServerInfo{Server: server, FullMethod: browserRunMethod}
			return interceptor(ctx, input, info, func(ctx context.Context, request any) (any, error) {
				return implementation.Run(ctx, request.(*wrapperspb.BytesValue))
			})
		}}},
		Streams:  []grpc.StreamDesc{},
		Metadata: "meerkit-browser-capability",
	}, implementation)
}

func MarshalBrowserCapabilityResponse(result *BrowserRunResult, err error) (*wrapperspb.BytesValue, error) {
	response := browserCapabilityResponse{Result: result}
	if err != nil {
		response.Error = err.Error()
	}
	data, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		return nil, marshalErr
	}
	return wrapperspb.Bytes(data), nil
}
