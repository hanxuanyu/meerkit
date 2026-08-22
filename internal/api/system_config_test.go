package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"meerkit/internal/app"
	"meerkit/internal/auth"
	"meerkit/internal/runtimeconfig"
	"meerkit/internal/store"
)

func TestRuntimeConfigAPIUpdatesResetsAndReportsConflicts(t *testing.T) {
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager, err := runtimeconfig.New(context.Background(), database)
	if err != nil {
		t.Fatal(err)
	}
	server := &APIServer{runtime: manager}

	version := manager.Version(app.SystemConfigScheduler)
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/system/config/runtime/scheduler", strings.NewReader(`{"version":1,"path":"scheduler.max_concurrency","value":4}`))
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = request
	server.updateSystemConfig(context, app.SystemConfigScheduler)
	if response.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", response.Code, response.Body.String())
	}
	if manager.Snapshot().Scheduler.MaxConcurrency != 4 {
		t.Fatalf("max concurrency = %d, want 4", manager.Snapshot().Scheduler.MaxConcurrency)
	}
	if manager.Version(app.SystemConfigScheduler) != version+1 {
		t.Fatalf("scheduler version = %d, want %d", manager.Version(app.SystemConfigScheduler), version+1)
	}

	request = httptest.NewRequest(http.MethodPatch, "/api/v1/system/config/runtime/scheduler", strings.NewReader(`{"version":1,"path":"scheduler.max_concurrency","value":8}`))
	response = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(response)
	context.Request = request
	server.updateSystemConfig(context, app.SystemConfigScheduler)
	if response.Code != http.StatusConflict {
		t.Fatalf("stale update status = %d, body = %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/system/config/runtime/scheduler/reset", nil)
	response = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(response)
	context.Request = request
	server.resetSystemConfig(context, app.SystemConfigScheduler)
	if response.Code != http.StatusOK {
		t.Fatalf("reset status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := manager.Snapshot().Scheduler.MaxConcurrency; got != app.DefaultRuntimeConfig().Scheduler.MaxConcurrency {
		t.Fatalf("reset max concurrency = %d, want %d", got, app.DefaultRuntimeConfig().Scheduler.MaxConcurrency)
	}
}

func TestEnablingMCPCreatesAndReturnsTokenOnce(t *testing.T) {
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager, err := runtimeconfig.New(context.Background(), database)
	if err != nil {
		t.Fatal(err)
	}
	server := &APIServer{runtime: manager, auth: auth.NewService(database, time.Hour)}

	request := httptest.NewRequest(http.MethodPatch, "/api/v1/system/config/runtime/mcp", strings.NewReader(`{"version":1,"path":"mcp.enabled","value":true}`))
	response := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(response)
	ginContext.Request = request
	server.updateSystemConfig(ginContext, app.SystemConfigMCP)
	if response.Code != http.StatusOK {
		t.Fatalf("enable MCP status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		BootstrapToken *auth.TokenSecret `json:"bootstrap_token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.BootstrapToken == nil || payload.BootstrapToken.Token == "" {
		t.Fatalf("enable MCP did not return generated token: %s", response.Body.String())
	}
	if !manager.Snapshot().MCP.Enabled {
		t.Fatal("MCP was not enabled")
	}

	items, err := server.auth.ListTokens(context.Background())
	if err != nil || len(items) != 1 || items[0].Type != auth.TokenTypeMCP {
		t.Fatalf("generated MCP token list = %+v, err = %v", items, err)
	}
	if pending := server.auth.ConsumePendingMCPToken(); pending != nil {
		t.Fatalf("generated token remained pending after response: %+v", pending)
	}
}

func TestDeletingMCPTokenDisablesMCP(t *testing.T) {
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager, err := runtimeconfig.New(context.Background(), database)
	if err != nil {
		t.Fatal(err)
	}
	authService := auth.NewService(database, time.Hour)
	server := &APIServer{runtime: manager, auth: authService}
	if _, err := manager.UpdatePath(context.Background(), app.SystemConfigMCP, "mcp.enabled", json.RawMessage("true"), manager.Version(app.SystemConfigMCP)); err != nil {
		t.Fatal(err)
	}
	item, err := authService.EnsureMCPToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/"+item.ID+"/permanent", nil)
	response := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(response)
	ginContext.Request = request
	ginContext.Params = gin.Params{{Key: "id", Value: item.ID}}
	server.deleteToken(ginContext)
	ginContext.Writer.WriteHeaderNow()
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete MCP token status = %d, body = %s", response.Code, response.Body.String())
	}
	if manager.Snapshot().MCP.Enabled {
		t.Fatal("MCP remained enabled after its token was permanently deleted")
	}
	if _, err := authService.GetToken(context.Background(), item.ID); err == nil {
		t.Fatal("MCP token was not deleted")
	}
}
