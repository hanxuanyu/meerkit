package mcpserver

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hanxuanyu/meerkit/sdk"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"meerkit/internal/browser"
)

const (
	defaultActionTimeout = time.Minute
	minimumActionTimeout = time.Second
	maximumActionTimeout = 5 * time.Minute
	requestBodyLimit     = 2 << 20
)

type BrowserController interface {
	Agents() []browser.AgentInfo
	Targets(context.Context, string) (sdk.BrowserTargets, error)
	ExecuteAction(context.Context, sdk.BrowserActionRequest) (sdk.BrowserActionResult, error)
	SelectorCandidates(context.Context, sdk.BrowserSelectorCandidatesRequest) (sdk.BrowserSelectorCandidates, error)
	StartNetworkCapture(context.Context, sdk.BrowserNetworkStartRequest) (sdk.BrowserNetworkSession, error)
	StopNetworkCapture(context.Context, string) (sdk.BrowserNetworkStopResult, error)
	Captures() []sdk.BrowserNetworkSession
}

type Options struct {
	Token   string
	Version string
	Logger  *slog.Logger
}

type service struct {
	browser       BrowserController
	ownedCaptures sync.Map
}

func New(controller BrowserController, options Options) (http.Handler, error) {
	if controller == nil {
		return nil, errors.New("MCP browser controller is required")
	}
	token := strings.TrimSpace(options.Token)
	if len(token) < 32 {
		return nil, errors.New("MCP bearer token must contain at least 32 characters")
	}
	version := strings.TrimSpace(options.Version)
	if version == "" {
		version = "dev"
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "meerkit-browser", Version: version}, &mcp.ServerOptions{
		Instructions: "Use browser_list_agents and browser_list_targets before invoking browser actions. Query browser_list_actions when action parameters are unknown. Browser actions operate on the user's connected Chrome instance and may affect open pages.",
		Logger:       options.Logger,
	})
	svc := &service{browser: controller}
	mcp.AddTool(server, &mcp.Tool{Name: "browser_list_agents", Title: "List browser agents", Description: "List Chrome browser agents currently connected to Meerkit.", Annotations: readOnlyAnnotations("List browser agents")}, svc.listAgents)
	mcp.AddTool(server, &mcp.Tool{Name: "browser_list_targets", Title: "List browser targets", Description: "List browser windows and tabs for a connected agent. If agent_id is omitted, Meerkit selects the first connected agent.", Annotations: readOnlyAnnotations("List browser targets")}, svc.listTargets)
	mcp.AddTool(server, &mcp.Tool{Name: "browser_list_actions", Title: "List browser actions", Description: "Inspect Meerkit's browser Action Catalog, including target requirements, parameters, side effects, and result types. Filter by category or search query to keep the response focused.", Annotations: readOnlyAnnotations("List browser actions")}, svc.listActions)
	mcp.AddTool(server, executeActionTool(), svc.executeAction)
	mcp.AddTool(server, &mcp.Tool{Name: "browser_find_selectors", Title: "Find selector candidates", Description: "Find robust CSS selector candidates for one or more element queries in a browser tab.", Annotations: readOnlyAnnotations("Find selector candidates")}, svc.findSelectors)
	mcp.AddTool(server, &mcp.Tool{Name: "browser_start_network_capture", Title: "Start network capture", Description: "Start a bounded network capture for one browser tab. MCP tracks these captures separately and only permits stopping captures it created.", Annotations: modifyingAnnotations("Start network capture", false)}, svc.startNetworkCapture)
	mcp.AddTool(server, &mcp.Tool{Name: "browser_list_network_captures", Title: "List MCP network captures", Description: "List active network captures created through this MCP server.", Annotations: readOnlyAnnotations("List MCP network captures")}, svc.listNetworkCaptures)
	mcp.AddTool(server, &mcp.Tool{Name: "browser_stop_network_capture", Title: "Stop network capture", Description: "Stop an MCP-owned network capture and return the collected request and response events.", Annotations: modifyingAnnotations("Stop network capture", false)}, svc.stopNetworkCapture)

	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		JSONResponse:   true,
		Logger:         options.Logger,
		SessionTimeout: 30 * time.Minute,
	})
	withoutWriteDeadline := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = http.NewResponseController(writer).SetWriteDeadline(time.Time{})
		streamable.ServeHTTP(writer, request)
	})
	return bearerAuth(token, http.MaxBytesHandler(withoutWriteDeadline, requestBodyLimit)), nil
}

type listAgentsInput struct{}
type listAgentsOutput struct {
	Agents []browser.AgentInfo `json:"agents"`
}

func (s *service) listAgents(context.Context, *mcp.CallToolRequest, listAgentsInput) (*mcp.CallToolResult, listAgentsOutput, error) {
	agents := s.browser.Agents()
	if agents == nil {
		agents = []browser.AgentInfo{}
	}
	return nil, listAgentsOutput{Agents: agents}, nil
}

type listTargetsInput struct {
	AgentID string `json:"agent_id,omitempty" jsonschema:"ID of the connected browser agent; omit to use the first connected agent"`
}
type listTargetsOutput struct {
	Targets sdk.BrowserTargets `json:"targets"`
}

func (s *service) listTargets(ctx context.Context, _ *mcp.CallToolRequest, input listTargetsInput) (*mcp.CallToolResult, listTargetsOutput, error) {
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	targets, err := s.browser.Targets(callCtx, input.AgentID)
	return nil, listTargetsOutput{Targets: targets}, err
}

type listActionsInput struct {
	Category string `json:"category,omitempty" jsonschema:"Exact action category, such as tab, window, page, dom, input, cookie, storage, or runtime"`
	Query    string `json:"query,omitempty" jsonschema:"Case-insensitive text matched against action type, label, description, and category"`
}
type listActionsOutput struct {
	Actions []browser.ActionDefinition `json:"actions"`
}

func (s *service) listActions(_ context.Context, _ *mcp.CallToolRequest, input listActionsInput) (*mcp.CallToolResult, listActionsOutput, error) {
	catalog := browser.BrowserActionCatalog()
	category := strings.TrimSpace(input.Category)
	query := strings.ToLower(strings.TrimSpace(input.Query))
	actions := make([]browser.ActionDefinition, 0, len(catalog.Actions))
	for _, action := range catalog.Actions {
		if category != "" && action.Category != category {
			continue
		}
		haystack := strings.ToLower(strings.Join([]string{action.Type, action.Label, action.Description, action.Category, action.CategoryLabel}, " "))
		if query != "" && !strings.Contains(haystack, query) {
			continue
		}
		actions = append(actions, action)
	}
	return nil, listActionsOutput{Actions: actions}, nil
}

type executeActionInput struct {
	AgentID  string         `json:"agent_id,omitempty"`
	WindowID int            `json:"window_id,omitempty"`
	TabID    int            `json:"tab_id,omitempty"`
	Action   string         `json:"action"`
	Params   map[string]any `json:"params,omitempty"`
	Timeout  int            `json:"timeout_ms,omitempty"`
}

func executeActionTool() *mcp.Tool {
	actions := browser.BrowserActionCatalog().Actions
	actionTypes := make([]string, 0, len(actions))
	for _, action := range actions {
		actionTypes = append(actionTypes, action.Type)
	}
	sort.Strings(actionTypes)
	return &mcp.Tool{
		Name:        "browser_execute_action",
		Title:       "Execute browser action",
		Description: "Execute one atomic action from Meerkit's Browser Action Catalog. Use browser_list_actions for parameter definitions and browser_list_targets for target IDs. The existing Meerkit validator enforces target and parameter requirements.",
		Annotations: modifyingAnnotations("Execute browser action", true),
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"action"},
			"properties": map[string]any{
				"agent_id":  map[string]any{"type": "string", "description": "Connected browser agent ID; omit to use the first connected agent"},
				"window_id": map[string]any{"type": "integer", "minimum": 0, "description": "Browser window ID when required by the action"},
				"tab_id":    map[string]any{"type": "integer", "minimum": 0, "description": "Browser tab ID when required by the action"},
				"action":    map[string]any{"type": "string", "enum": actionTypes, "description": "Atomic browser action type"},
				"params":    map[string]any{"type": "object", "additionalProperties": true, "description": "Action parameters from browser_list_actions"},
				"timeout_ms": map[string]any{"type": "integer", "minimum": 1000, "maximum": 300000, "default": 60000,
					"description": "Total action timeout in milliseconds"},
			},
		},
	}
}

func (s *service) executeAction(ctx context.Context, _ *mcp.CallToolRequest, input executeActionInput) (*mcp.CallToolResult, sdk.BrowserActionResult, error) {
	timeout := normalizedActionTimeout(input.Timeout)
	callCtx, cancel := context.WithTimeout(ctx, timeout+5*time.Second)
	defer cancel()
	result, err := s.browser.ExecuteAction(callCtx, sdk.BrowserActionRequest{
		Target:    sdk.BrowserTarget{AgentID: input.AgentID, WindowID: input.WindowID, TabID: input.TabID},
		TimeoutMS: int(timeout.Milliseconds()),
		Action:    sdk.BrowserAction{Type: input.Action, Params: input.Params},
	})
	if err != nil {
		return nil, sdk.BrowserActionResult{}, err
	}
	callResult, sanitized := actionContent(result)
	return callResult, sanitized, nil
}

type findSelectorsInput struct {
	AgentID  string   `json:"agent_id,omitempty" jsonschema:"Connected browser agent ID; omit to use the first connected agent"`
	WindowID int      `json:"window_id,omitempty" jsonschema:"Window containing the target tab"`
	TabID    int      `json:"tab_id" jsonschema:"Target browser tab ID"`
	Queries  []string `json:"queries" jsonschema:"Natural-language or CSS-oriented descriptions of the elements to locate"`
	Limit    int      `json:"limit,omitempty" jsonschema:"Maximum candidates to return across all queries; maximum 200"`
}

func (s *service) findSelectors(ctx context.Context, _ *mcp.CallToolRequest, input findSelectorsInput) (*mcp.CallToolResult, sdk.BrowserSelectorCandidates, error) {
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result, err := s.browser.SelectorCandidates(callCtx, sdk.BrowserSelectorCandidatesRequest{
		Target:  sdk.BrowserTarget{AgentID: input.AgentID, WindowID: input.WindowID, TabID: input.TabID},
		Queries: input.Queries,
		Limit:   input.Limit,
	})
	return nil, result, err
}

type startNetworkCaptureInput struct {
	AgentID      string                          `json:"agent_id,omitempty" jsonschema:"Connected browser agent ID; omit to use the first connected agent"`
	WindowID     int                             `json:"window_id,omitempty" jsonschema:"Window containing the target tab"`
	TabID        int                             `json:"tab_id" jsonschema:"Target browser tab ID"`
	Rules        []sdk.BrowserNetworkCaptureRule `json:"rules,omitempty" jsonschema:"Capture filters; omit to capture all requests with a bounded response body"`
	DisableCache bool                            `json:"disable_cache,omitempty" jsonschema:"Temporarily disable the browser cache during capture"`
}

func (s *service) startNetworkCapture(ctx context.Context, _ *mcp.CallToolRequest, input startNetworkCaptureInput) (*mcp.CallToolResult, sdk.BrowserNetworkSession, error) {
	rules := input.Rules
	if len(rules) == 0 {
		rules = []sdk.BrowserNetworkCaptureRule{{ID: "mcp", MaxBodyBytes: 256 << 10}}
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	session, err := s.browser.StartNetworkCapture(callCtx, sdk.BrowserNetworkStartRequest{
		Target:       sdk.BrowserTarget{AgentID: input.AgentID, WindowID: input.WindowID, TabID: input.TabID},
		Rules:        rules,
		DisableCache: input.DisableCache,
	})
	if err == nil {
		s.ownedCaptures.Store(session.ID, struct{}{})
	}
	return nil, session, err
}

type listNetworkCapturesInput struct{}
type listNetworkCapturesOutput struct {
	Captures []sdk.BrowserNetworkSession `json:"captures"`
}

func (s *service) listNetworkCaptures(context.Context, *mcp.CallToolRequest, listNetworkCapturesInput) (*mcp.CallToolResult, listNetworkCapturesOutput, error) {
	owned := make([]sdk.BrowserNetworkSession, 0)
	active := make(map[string]struct{})
	for _, capture := range s.browser.Captures() {
		active[capture.ID] = struct{}{}
		if _, ok := s.ownedCaptures.Load(capture.ID); ok {
			owned = append(owned, capture)
		}
	}
	s.ownedCaptures.Range(func(key, _ any) bool {
		if _, ok := active[key.(string)]; !ok {
			s.ownedCaptures.Delete(key)
		}
		return true
	})
	sort.Slice(owned, func(i, j int) bool { return owned[i].StartedAt < owned[j].StartedAt })
	return nil, listNetworkCapturesOutput{Captures: owned}, nil
}

type stopNetworkCaptureInput struct {
	CaptureID string `json:"capture_id" jsonschema:"ID returned by browser_start_network_capture"`
}

func (s *service) stopNetworkCapture(ctx context.Context, _ *mcp.CallToolRequest, input stopNetworkCaptureInput) (*mcp.CallToolResult, sdk.BrowserNetworkStopResult, error) {
	if _, ok := s.ownedCaptures.Load(input.CaptureID); !ok {
		return nil, sdk.BrowserNetworkStopResult{}, errors.New("network capture is not owned by this MCP server")
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result, err := s.browser.StopNetworkCapture(callCtx, input.CaptureID)
	if err == nil {
		s.ownedCaptures.Delete(input.CaptureID)
	}
	return nil, result, err
}

func normalizedActionTimeout(milliseconds int) time.Duration {
	if milliseconds <= 0 {
		return defaultActionTimeout
	}
	timeout := time.Duration(milliseconds) * time.Millisecond
	if timeout < minimumActionTimeout {
		return minimumActionTimeout
	}
	if timeout > maximumActionTimeout {
		return maximumActionTimeout
	}
	return timeout
}

func actionContent(result sdk.BrowserActionResult) (*mcp.CallToolResult, sdk.BrowserActionResult) {
	sanitized := result
	contents := []mcp.Content{}
	if result.Type == "page.screenshot" {
		if dataURL, ok := result.Data["data_url"].(string); ok {
			mimeType, imageData, err := decodeDataURL(dataURL)
			if err == nil {
				sanitized.Data = make(map[string]any, len(result.Data))
				for key, value := range result.Data {
					if key != "data_url" {
						sanitized.Data[key] = value
					}
				}
				sanitized.Data["image_content"] = true
				contents = append(contents, &mcp.ImageContent{Data: imageData, MIMEType: mimeType})
			}
		}
	}
	serialized, _ := json.Marshal(sanitized)
	contents = append([]mcp.Content{&mcp.TextContent{Text: string(serialized)}}, contents...)
	return &mcp.CallToolResult{Content: contents, IsError: !result.Success}, sanitized
}

func decodeDataURL(value string) (string, []byte, error) {
	prefix, encoded, ok := strings.Cut(value, ",")
	if !ok || !strings.HasPrefix(prefix, "data:image/") || !strings.HasSuffix(prefix, ";base64") {
		return "", nil, errors.New("invalid screenshot data URL")
	}
	mimeType := strings.TrimSuffix(strings.TrimPrefix(prefix, "data:"), ";base64")
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", nil, fmt.Errorf("decode screenshot: %w", err)
	}
	return mimeType, data, nil
}

func bearerAuth(token string, next http.Handler) http.Handler {
	expected := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		parts := strings.Fields(request.Header.Get("Authorization"))
		provided := sha256.Sum256(nil)
		if len(parts) == 2 {
			provided = sha256.Sum256([]byte(parts[1]))
		}
		valid := len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && subtle.ConstantTimeCompare(provided[:], expected[:]) == 1
		if !valid {
			writer.Header().Set("WWW-Authenticate", `Bearer realm="meerkit-mcp"`)
			http.Error(writer, "authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func readOnlyAnnotations(title string) *mcp.ToolAnnotations {
	openWorld := true
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, OpenWorldHint: &openWorld}
}

func modifyingAnnotations(title string, destructive bool) *mcp.ToolAnnotations {
	openWorld := true
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: &destructive, OpenWorldHint: &openWorld}
}
