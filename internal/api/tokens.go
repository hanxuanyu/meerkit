package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"meerkit/internal/app"
	"meerkit/internal/auth"
)

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func (a *APIServer) listTokens(c *gin.Context) {
	if isBearerPrincipal(c) {
		writeError(c.Writer, http.StatusForbidden, "forbidden", "administrator session required")
		return
	}
	items, err := a.auth.ListTokens(c.Request.Context())
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	query := strings.ToLower(strings.TrimSpace(c.Query("q")))
	filtered := make([]auth.TokenInfo, 0, len(items))
	for _, item := range items {
		if query != "" && !strings.Contains(strings.ToLower(strings.Join([]string{item.Name, item.Type, item.TokenHint, strings.Join(item.Scopes, " ")}, " ")), query) {
			continue
		}
		filtered = append(filtered, item)
	}
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("page_size"), 20)
	if pageSize != 10 && pageSize != 20 && pageSize != 50 && pageSize != 100 {
		pageSize = 20
	}
	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pageItems := filtered[start:end]
	response := map[string]any{"items": pageItems, "page": page, "page_size": pageSize, "total": total, "total_pages": (total + pageSize - 1) / pageSize, "allow_token_copy": a.config.Security.AllowTokenCopy}
	writeJSON(c.Writer, http.StatusOK, response)
}

func (a *APIServer) createToken(c *gin.Context) {
	if isBearerPrincipal(c) {
		writeError(c.Writer, http.StatusForbidden, "forbidden", "administrator session required")
		return
	}
	var payload struct {
		Name      string     `json:"name"`
		Type      string     `json:"type"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	info, secret, err := a.auth.CreateToken(c.Request.Context(), payload.Name, strings.ToLower(strings.TrimSpace(payload.Type)), payload.Scopes, payload.ExpiresAt)
	if err != nil {
		if errors.Is(err, auth.ErrTokenNameExists) {
			writeError(c.Writer, http.StatusConflict, "token_name_exists", err.Error())
			return
		}
		writeError(c.Writer, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusCreated, map[string]any{"token": secret, "item": info})
}

func (a *APIServer) revealToken(c *gin.Context) {
	if isBearerPrincipal(c) {
		writeError(c.Writer, http.StatusForbidden, "forbidden", "administrator session required")
		return
	}
	if !a.config.Security.AllowTokenCopy {
		writeError(c.Writer, http.StatusForbidden, "token_copy_disabled", "token copying is disabled")
		return
	}
	value, err := a.auth.RevealToken(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c.Writer, http.StatusNotFound, "token_not_found", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, map[string]string{"token": value})
}

func (a *APIServer) revokeToken(c *gin.Context) {
	if isBearerPrincipal(c) {
		writeError(c.Writer, http.StatusForbidden, "forbidden", "administrator session required")
		return
	}
	if err := a.auth.RevokeToken(c.Request.Context(), c.Param("id")); err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *APIServer) restoreToken(c *gin.Context) {
	if isBearerPrincipal(c) {
		writeError(c.Writer, http.StatusForbidden, "forbidden", "administrator session required")
		return
	}
	if err := a.auth.RestoreToken(c.Request.Context(), c.Param("id")); err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *APIServer) deleteToken(c *gin.Context) {
	if isBearerPrincipal(c) {
		writeError(c.Writer, http.StatusForbidden, "forbidden", "administrator session required")
		return
	}
	item, err := a.auth.GetToken(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c.Writer, http.StatusNotFound, "token_not_found", err.Error())
		return
	}
	if item.Type == auth.TokenTypeMCP && a.runtime != nil && a.runtime.Snapshot().MCP.Enabled {
		if _, err := a.runtime.UpdatePath(c.Request.Context(), app.SystemConfigMCP, "mcp.enabled", json.RawMessage("false"), a.runtime.Version(app.SystemConfigMCP)); err != nil {
			writeRuntimeConfigError(c, err)
			return
		}
	}
	if err := a.auth.DeleteToken(c.Request.Context(), c.Param("id")); err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
