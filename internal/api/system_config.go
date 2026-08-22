package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"meerkit/internal/app"
	"meerkit/internal/store"
)

func (a *APIServer) runtimeSnapshot() app.RuntimeConfig {
	if a.runtime == nil {
		return app.DefaultRuntimeConfig()
	}
	return a.runtime.Snapshot()
}

func (a *APIServer) systemConfig(c *gin.Context) {
	metadata := a.config.Metadata
	if a.runtime != nil {
		metadata.RuntimeItems = a.runtime.Metadata()
	}
	writeJSON(c.Writer, http.StatusOK, metadata)
}

func (a *APIServer) updateSystemConfig(c *gin.Context, configType string) {
	if a.runtime == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "runtime_config_unavailable", "runtime config is unavailable")
		return
	}
	var payload struct {
		Version int             `json:"version"`
		Path    string          `json:"path"`
		Value   json.RawMessage `json:"value"`
		Data    json.RawMessage `json:"data"`
	}
	if err := decodeBody(c.Writer, c.Request, &payload); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	var err error
	wasMCPEnabled := a.runtime.Snapshot().MCP.Enabled
	if payload.Path != "" {
		_, err = a.runtime.UpdatePath(c.Request.Context(), configType, payload.Path, payload.Value, payload.Version)
	} else {
		_, err = a.runtime.Update(c.Request.Context(), configType, payload.Data, payload.Version)
	}
	if err != nil {
		writeRuntimeConfigError(c, err)
		return
	}
	response := map[string]any{"items": a.runtime.Metadata()}
	if configType == app.SystemConfigMCP && !wasMCPEnabled && a.runtime.Snapshot().MCP.Enabled && a.auth != nil {
		if _, err := a.auth.EnsureMCPToken(c.Request.Context()); err != nil {
			_, _ = a.runtime.UpdatePath(c.Request.Context(), app.SystemConfigMCP, "mcp.enabled", json.RawMessage("false"), a.runtime.Version(app.SystemConfigMCP))
			writeError(c.Writer, http.StatusInternalServerError, "mcp_token_create_failed", err.Error())
			return
		}
		if pending := a.auth.ConsumePendingMCPToken(); pending != nil {
			response["bootstrap_token"] = pending
		}
	}
	writeJSON(c.Writer, http.StatusOK, response)
}

func (a *APIServer) resetSystemConfig(c *gin.Context, configType string) {
	if a.runtime == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "runtime_config_unavailable", "runtime config is unavailable")
		return
	}
	if _, err := a.runtime.Reset(c.Request.Context(), configType); err != nil {
		writeRuntimeConfigError(c, err)
		return
	}
	writeJSON(c.Writer, http.StatusOK, map[string]any{"items": a.runtime.Metadata()})
}

func (a *APIServer) resetAllSystemConfigs(c *gin.Context) {
	if a.runtime == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "runtime_config_unavailable", "runtime config is unavailable")
		return
	}
	if _, err := a.runtime.ResetAll(c.Request.Context()); err != nil {
		writeRuntimeConfigError(c, err)
		return
	}
	writeJSON(c.Writer, http.StatusOK, map[string]any{"items": a.runtime.Metadata()})
}

func writeRuntimeConfigError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrSystemConfigVersionConflict):
		writeError(c.Writer, http.StatusConflict, "config_version_conflict", "runtime config changed; reload and try again")
	default:
		writeError(c.Writer, http.StatusBadRequest, "invalid_runtime_config", err.Error())
	}
}
