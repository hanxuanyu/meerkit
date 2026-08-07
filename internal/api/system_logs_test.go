package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"meerkit/internal/app"
)

func TestSystemLogsReadsConfiguredLogSources(t *testing.T) {
	directory := t.TempDir()
	config := app.DefaultConfig()
	config.Logging.File.Directory = directory
	if err := os.WriteFile(filepath.Join(directory, config.Logging.File.Filename), []byte("business log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, config.Logging.File.Access.Filename), []byte("access log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := &APIServer{config: config}

	for source, expected := range map[string]string{"business": "business log", "access": "access log"} {
		response := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(response)
		context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/system/logs?source="+source, nil)
		server.systemLogs(context)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("source %s: status=%d body=%q", source, response.Code, response.Body.String())
		}
	}
}

func TestSystemLogsRejectsUnknownSource(t *testing.T) {
	server := &APIServer{config: app.DefaultConfig()}
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/system/logs?source=../../config.yaml", nil)
	server.systemLogs(context)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}
