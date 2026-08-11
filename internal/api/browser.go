package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"meerkit/internal/browser"
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
