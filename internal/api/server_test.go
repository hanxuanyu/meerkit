package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"meerkit/internal/app"
	"meerkit/internal/core"
	"meerkit/internal/store"
)

func TestPreviewSchedule(t *testing.T) {
	server := &APIServer{config: app.DefaultConfig()}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/schedules/preview", strings.NewReader(`{"expression":"*/5 * * * *"}`))
	response := httptest.NewRecorder()
	server.previewSchedule(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Description string   `json:"description"`
		NextRuns    []string `json:"next_runs"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Description != "每 5 分钟执行一次" || len(payload.NextRuns) != 3 {
		t.Fatalf("unexpected preview: %#v", payload)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/schedules/preview", strings.NewReader(`{"expression":"not-a-cron"}`))
	response = httptest.NewRecorder()
	server.previewSchedule(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid preview status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestMonitorListPageAddsNextRun(t *testing.T) {
	page := store.PageResult[core.Monitor]{
		Items: []core.Monitor{
			{ID: "monitor-1", Enabled: true, Schedules: []string{"*/5 * * * *"}},
			{ID: "monitor-2", Enabled: false, Schedules: []string{"*/5 * * * *"}},
		},
		Page:       2,
		PageSize:   20,
		Total:      21,
		TotalPages: 2,
	}
	result := monitorListPage(page, "Asia/Shanghai")
	if len(result.Items) != 2 || result.Items[0].NextRunAt == nil {
		t.Fatalf("next run was not added: %#v", result.Items)
	}
	if result.Items[1].NextRunAt != nil {
		t.Fatalf("disabled monitor should not have a next run: %#v", result.Items[1])
	}
	if result.Page != page.Page || result.PageSize != page.PageSize || result.Total != page.Total || result.TotalPages != page.TotalPages {
		t.Fatalf("pagination metadata changed: %#v", result)
	}
}
