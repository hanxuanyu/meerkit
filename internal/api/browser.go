package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hanxuanyu/meerkit/sdk"
	"meerkit/internal/browser"
)

const (
	minimumBrowserActionTimeout      = time.Second
	maximumBrowserActionTimeout      = 5 * time.Minute
	browserSelectorCandidatesTimeout = 10 * time.Second
)

func (a *APIServer) handleBrowserExtension(c *gin.Context) {
	if a.browser == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "browser_unavailable", "browser manager is unavailable")
		return
	}
	a.browser.HandleExtension(c.Writer, c.Request)
}

func (a *APIServer) getBrowserStatus(c *gin.Context) {
	if a.browser == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "browser_unavailable", "browser manager is unavailable")
		return
	}
	c.Header("Cache-Control", "no-store")
	writeJSON(c.Writer, http.StatusOK, map[string]any{
		"protocol":         browser.ProtocolVersion,
		"websocket_path":   browser.ExtensionWebSocketPath,
		"pairing_token":    a.browser.PairingToken(),
		"connected_agents": a.browser.Agents(),
	})
}

func (a *APIServer) getBrowserActions(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	writeJSON(c.Writer, http.StatusOK, browser.BrowserActionCatalog())
}

func (a *APIServer) rotateBrowserPairingToken(c *gin.Context) {
	if a.browser == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "browser_unavailable", "browser manager is unavailable")
		return
	}
	c.Header("Cache-Control", "no-store")
	token, err := a.browser.RotatePairingToken()
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "browser_token_failed", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, map[string]any{"pairing_token": token, "connected_agents": []any{}})
}

func (a *APIServer) browserAction(c *gin.Context) {
	if a.browser == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "browser_unavailable", "browser manager is unavailable")
		return
	}
	var request sdk.BrowserActionRequest
	if err := decodeBody(c.Writer, c.Request, &request); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	timeout := time.Duration(request.TimeoutMS) * time.Millisecond
	if request.TimeoutMS <= 0 {
		timeout = time.Minute
	} else if timeout < minimumBrowserActionTimeout {
		timeout = minimumBrowserActionTimeout
	}
	if timeout > maximumBrowserActionTimeout {
		timeout = maximumBrowserActionTimeout
	}
	request.TimeoutMS = int(timeout.Milliseconds())
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout+5*time.Second)
	defer cancel()
	result, err := a.browser.ExecuteAction(ctx, request)
	if err != nil {
		status := http.StatusBadRequest
		code := "browser_action_failed"
		if errors.Is(err, context.DeadlineExceeded) {
			status, code = http.StatusGatewayTimeout, "browser_action_timeout"
		} else if strings.Contains(err.Error(), "not connected") || strings.Contains(err.Error(), "no browser extension") {
			status, code = http.StatusServiceUnavailable, "browser_agent_unavailable"
		}
		writeError(c.Writer, status, code, err.Error())
		return
	}
	c.Header("Cache-Control", "no-store")
	writeJSON(c.Writer, http.StatusOK, result)
}

func (a *APIServer) browserTargets(c *gin.Context) {
	if a.browser == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "browser_unavailable", "browser manager is unavailable")
		return
	}
	result, err := a.browser.Targets(c.Request.Context(), c.Query("agent_id"))
	if err != nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "browser_targets_failed", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, result)
}

func (a *APIServer) browserSelectorCandidates(c *gin.Context) {
	if a.browser == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "browser_unavailable", "browser manager is unavailable")
		return
	}
	var request sdk.BrowserSelectorCandidatesRequest
	if err := decodeBody(c.Writer, c.Request, &request); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), browserSelectorCandidatesTimeout)
	defer cancel()
	result, err := a.browser.SelectorCandidates(ctx, request)
	if err != nil {
		status := http.StatusBadRequest
		code := "browser_selector_candidates_failed"
		if errors.Is(err, context.DeadlineExceeded) {
			status, code = http.StatusGatewayTimeout, "browser_selector_candidates_timeout"
		} else if strings.Contains(err.Error(), "not connected") || strings.Contains(err.Error(), "does not support") {
			status, code = http.StatusServiceUnavailable, "browser_agent_unavailable"
		}
		writeError(c.Writer, status, code, err.Error())
		return
	}
	c.Header("Cache-Control", "no-store")
	writeJSON(c.Writer, http.StatusOK, result)
}

func (a *APIServer) startBrowserCapture(c *gin.Context) {
	if a.browser == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "browser_unavailable", "browser manager is unavailable")
		return
	}
	var request sdk.BrowserNetworkStartRequest
	if err := decodeBody(c.Writer, c.Request, &request); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	result, err := a.browser.StartNetworkCapture(c.Request.Context(), request)
	if err != nil {
		writeError(c.Writer, http.StatusBadRequest, "browser_capture_failed", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, result)
}

func (a *APIServer) stopBrowserCapture(c *gin.Context) {
	if a.browser == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "browser_unavailable", "browser manager is unavailable")
		return
	}
	result, err := a.browser.StopNetworkCapture(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c.Writer, http.StatusBadRequest, "browser_capture_stop_failed", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, result)
}

var browserDebugUpgrader = websocket.Upgrader{CheckOrigin: func(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Host == request.Host || parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1")
}}

func (a *APIServer) streamBrowserDebug(c *gin.Context) {
	if a.browser == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "browser_unavailable", "browser manager is unavailable")
		return
	}
	connection, err := browserDebugUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	events, unsubscribe := a.browser.Subscribe()
	defer unsubscribe()
	_ = connection.WriteJSON(map[string]any{"type": "connected", "captures": a.browser.Captures()})
	disconnected := make(chan struct{})
	go func() {
		defer close(disconnected)
		for {
			if _, _, err := connection.ReadMessage(); err != nil {
				return
			}
		}
	}()
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case event, ok := <-events:
			if !ok || connection.WriteJSON(event) != nil {
				return
			}
		case <-c.Request.Context().Done():
			return
		case <-disconnected:
			return
		case <-heartbeat.C:
			if connection.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)) != nil {
				return
			}
		}
	}
}
