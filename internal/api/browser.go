package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanxuanyu/meerkit/sdk"
	"meerkit/internal/browser"
)

const (
	minimumBrowserRunTimeout = time.Second
	maximumBrowserRunTimeout = 5 * time.Minute
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

func (a *APIServer) runBrowser(c *gin.Context) {
	if a.browser == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "browser_unavailable", "browser manager is unavailable")
		return
	}
	var request sdk.BrowserRunRequest
	if err := decodeBody(c.Writer, c.Request, &request); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	timeout := time.Duration(request.TimeoutMS) * time.Millisecond
	if request.TimeoutMS <= 0 {
		timeout = time.Minute
	} else if timeout < minimumBrowserRunTimeout {
		timeout = minimumBrowserRunTimeout
	}
	if timeout > maximumBrowserRunTimeout {
		timeout = maximumBrowserRunTimeout
	}
	request.TimeoutMS = int(timeout.Milliseconds())
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout+5*time.Second)
	defer cancel()
	result, err := a.browser.Execute(ctx, request)
	if err != nil {
		status := http.StatusBadRequest
		code := "browser_run_failed"
		if errors.Is(err, context.DeadlineExceeded) {
			status, code = http.StatusGatewayTimeout, "browser_run_timeout"
		} else if strings.Contains(err.Error(), "not connected") || strings.Contains(err.Error(), "no browser extension") {
			status, code = http.StatusServiceUnavailable, "browser_agent_unavailable"
		}
		writeError(c.Writer, status, code, err.Error())
		return
	}
	c.Header("Cache-Control", "no-store")
	writeJSON(c.Writer, http.StatusOK, result)
}
