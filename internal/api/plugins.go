package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	pluginruntime "meerkit/internal/plugin"
)

func (a *APIServer) listPlugins(c *gin.Context) {
	values, err := a.plugins.List(c.Request.Context())
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, map[string]any{"items": values})
}
func (a *APIServer) pluginDetails(c *gin.Context) {
	value, err := a.plugins.Details(c.Request.Context(), c.Param("id"), c.Param("version"))
	if err != nil {
		writeError(c.Writer, http.StatusNotFound, "plugin_not_found", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, value)
}
func (a *APIServer) importPlugin(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 260<<20)
	header, err := c.FormFile("package")
	if err != nil {
		writeError(c.Writer, http.StatusBadRequest, "package_required", "a zip or tar.gz package is required")
		return
	}
	suffix := ".zip"
	if strings.HasSuffix(strings.ToLower(header.Filename), ".tar.gz") {
		suffix = ".tar.gz"
	} else if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		writeError(c.Writer, http.StatusBadRequest, "invalid_package", "plugin package must be .zip or .tar.gz")
		return
	}
	input, err := header.Open()
	if err != nil {
		writeError(c.Writer, http.StatusBadRequest, "upload_failed", err.Error())
		return
	}
	defer input.Close()
	temporary, err := os.CreateTemp(filepath.Join(a.plugins.Root(), "staging"), "upload-*"+suffix)
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	path := temporary.Name()
	defer os.Remove(path)
	if _, err := io.Copy(temporary, input); err != nil {
		temporary.Close()
		writeError(c.Writer, http.StatusBadRequest, "upload_failed", err.Error())
		return
	}
	if err := temporary.Close(); err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	value, err := a.plugins.Import(c.Request.Context(), path, pluginruntime.ImportOptions{})
	if err != nil {
		writeError(c.Writer, http.StatusBadRequest, "plugin_import_failed", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusCreated, value)
}
func (a *APIServer) scanPlugins(c *gin.Context) {
	values, err := a.plugins.Scan(c.Request.Context())
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "plugin_scan_failed", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, map[string]any{"items": values, "imported": len(values)})
}
func (a *APIServer) enablePlugin(c *gin.Context) {
	var payload struct {
		ConfirmUnverified bool `json:"confirm_unverified"`
	}
	_ = c.ShouldBindJSON(&payload)
	if err := a.plugins.Enable(c.Request.Context(), c.Param("id"), c.Param("version"), payload.ConfirmUnverified); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "plugin_enable_failed", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, map[string]string{"status": "healthy"})
}
func (a *APIServer) trustPluginPublisher(c *gin.Context) {
	var payload struct {
		Fingerprint string `json:"fingerprint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "fingerprint_required", "the signer fingerprint is required")
		return
	}
	value, err := a.plugins.TrustPublisher(c.Request.Context(), c.Param("id"), c.Param("version"), payload.Fingerprint)
	if err != nil {
		writeError(c.Writer, http.StatusBadRequest, "plugin_trust_failed", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, value)
}
func (a *APIServer) disablePlugin(c *gin.Context) {
	if err := a.plugins.Disable(c.Request.Context(), c.Param("id"), c.Param("version")); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "plugin_disable_failed", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, map[string]string{"status": "disabled"})
}
func (a *APIServer) uninstallPlugin(c *gin.Context) {
	if err := a.plugins.Uninstall(c.Request.Context(), c.Param("id"), c.Param("version")); err != nil {
		writeError(c.Writer, http.StatusConflict, "plugin_uninstall_failed", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
func (a *APIServer) exportPlugin(c *gin.Context) {
	path, err := a.plugins.Export(c.Request.Context(), c.Param("id"), c.Param("version"))
	if err != nil {
		writeError(c.Writer, http.StatusNotFound, "plugin_not_found", err.Error())
		return
	}
	if _, err := os.Stat(path); err != nil {
		writeError(c.Writer, http.StatusNotFound, "package_not_found", err.Error())
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(path)))
	c.File(path)
}
func (a *APIServer) pluginLogs(c *gin.Context) {
	data, err := a.plugins.Logs(c.Param("id"), c.Param("version"), 128<<10)
	if err != nil {
		writeError(c.Writer, http.StatusNotFound, "plugin_logs_not_found", err.Error())
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", data)
}
