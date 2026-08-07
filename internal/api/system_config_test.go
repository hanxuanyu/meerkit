package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"meerkit/internal/app"
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
