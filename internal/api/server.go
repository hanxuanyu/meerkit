package api

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"meerkit/internal/app"
	"meerkit/internal/core"
	"meerkit/internal/monitor"
	"meerkit/internal/notification"
	runtimeapp "meerkit/internal/runtime"
	"meerkit/internal/store"
)

type APIServer struct {
	store     *store.Store
	modules   *monitor.Registry
	notifiers *notification.Registry
	runner    *runtimeapp.Runner
	config    app.Config
	logger    *slog.Logger
}

func NewAPIServer(store *store.Store, modules *monitor.Registry, notifiers *notification.Registry, runner *runtimeapp.Runner, config app.Config, logger *slog.Logger) *APIServer {
	return &APIServer{store: store, modules: modules, notifiers: notifiers, runner: runner, config: config, logger: logger}
}

func (a *APIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		return
	}
	if r.URL.Path == "/readyz" {
		if err := a.store.Ping(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "storage_unavailable", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		a.handleAPI(w, r)
		return
	}
	serveFrontend(w, r)
}

func (a *APIServer) handleAPI(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path)
	if len(parts) < 3 || parts[0] != "api" || parts[1] != "v1" {
		writeError(w, http.StatusNotFound, "not_found", "API route not found")
		return
	}
	switch parts[2] {
	case "modules":
		a.handleModules(w, r, parts[3:])
	case "notifiers":
		a.handleNotifiers(w, r)
	case "notification-channels":
		a.handleChannels(w, r, parts[3:])
	case "system":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"server": a.config.ListenAddress(), "retention": a.config.Storage.Retention, "timezone": a.config.Scheduler.Timezone})
	case "monitors":
		a.handleMonitors(w, r, parts[3:])
	default:
		writeError(w, http.StatusNotFound, "not_found", "API route not found")
	}
}

func (a *APIServer) handleModules(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if len(parts) == 1 {
		module, ok := a.modules.Get(parts[0])
		if !ok {
			writeError(w, http.StatusNotFound, "module_not_found", "module not found")
			return
		}
		writeJSON(w, http.StatusOK, module.Descriptor())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": a.modules.Descriptors()})
}

func (a *APIServer) handleNotifiers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": a.notifiers.Descriptors()})
}

func (a *APIServer) handleChannels(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			channels, err := a.store.ListChannels(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": channels})
		case http.MethodPost:
			a.createChannel(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
		return
	}
	id := parts[0]
	channel, err := a.store.GetChannel(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "channel_not_found", "notification channel not found")
		return
	}
	if len(parts) > 1 && parts[1] == "test" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		notifier, ok := a.notifiers.Get(channel.NotifierType)
		if !ok {
			writeError(w, http.StatusBadRequest, "notifier_not_found", "notifier not found")
			return
		}
		event := core.NotificationEvent{EventType: "test", MonitorName: "Meerkit test notification", ModuleType: "system", TriggeredAt: time.Now().UTC(), ConditionState: "true", Summary: "This is a test notification."}
		if err := notifier.Send(r.Context(), channel.Config, event); err != nil {
			writeError(w, http.StatusBadRequest, "notification_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "sent"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, channel)
	case http.MethodPatch, http.MethodPut:
		var payload struct {
			Name         string          `json:"name"`
			NotifierType string          `json:"notifier_type"`
			Enabled      *bool           `json:"enabled"`
			Config       json.RawMessage `json:"config"`
		}
		if err := decodeBody(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if payload.Name != "" {
			channel.Name = payload.Name
		}
		if payload.NotifierType != "" {
			channel.NotifierType = payload.NotifierType
		}
		if payload.Enabled != nil {
			channel.Enabled = *payload.Enabled
		}
		if len(payload.Config) > 0 {
			channel.Config = payload.Config
		}
		notifier, ok := a.notifiers.Get(channel.NotifierType)
		if !ok {
			writeError(w, http.StatusBadRequest, "notifier_not_found", "notifier not found")
			return
		}
		if err := notifier.ValidateConfig(channel.Config); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_notifier_config", err.Error())
			return
		}
		channel.UpdatedAt = time.Now().UTC()
		if err := a.store.UpdateChannel(r.Context(), channel); err != nil {
			writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, channel)
	case http.MethodDelete:
		if err := a.store.DeleteChannel(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (a *APIServer) createChannel(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name         string          `json:"name"`
		NotifierType string          `json:"notifier_type"`
		Enabled      *bool           `json:"enabled"`
		Config       json.RawMessage `json:"config"`
	}
	if err := decodeBody(w, r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if payload.Name == "" || payload.NotifierType == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name and notifier_type are required")
		return
	}
	notifier, ok := a.notifiers.Get(payload.NotifierType)
	if !ok {
		writeError(w, http.StatusBadRequest, "notifier_not_found", "notifier not found")
		return
	}
	payload.Config = ensureRawJSON(payload.Config, "{}")
	if err := notifier.ValidateConfig(payload.Config); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_notifier_config", err.Error())
		return
	}
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	now := time.Now().UTC()
	channel := core.NotificationChannel{ID: core.NewID(), Name: payload.Name, NotifierType: payload.NotifierType, Enabled: enabled, Config: payload.Config, CreatedAt: now, UpdatedAt: now}
	if err := a.store.CreateChannel(r.Context(), channel); err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, channel)
}

func (a *APIServer) handleMonitors(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			monitors, err := a.store.ListMonitors(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": monitors})
		case http.MethodPost:
			a.createMonitor(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
		return
	}
	id := parts[0]
	if len(parts) > 1 {
		switch parts[1] {
		case "run":
			if r.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			record, err := a.runner.Run(r.Context(), id)
			if errors.Is(err, runtimeapp.ErrMonitorRunning) {
				writeError(w, http.StatusConflict, "monitor_running", err.Error())
				return
			}
			if err != nil {
				writeError(w, http.StatusBadRequest, "run_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, record)
		case "records":
			if r.Method != http.MethodGet {
				writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			records, err := a.store.ListRecords(r.Context(), id, limit)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": records})
		case "next-runs":
			if r.Method != http.MethodGet {
				writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			monitor, err := a.store.GetMonitor(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusNotFound, "monitor_not_found", err.Error())
				return
			}
			next, err := runtimeapp.NextScheduleTimes(monitor.Schedule, monitor.Timezone, a.config.Scheduler.Timezone, 5)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_schedule", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": next})
		default:
			writeError(w, http.StatusNotFound, "not_found", "monitor route not found")
		}
		return
	}
	monitor, err := a.store.GetMonitor(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "monitor_not_found", "monitor not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, monitor)
	case http.MethodPatch, http.MethodPut:
		a.updateMonitor(w, r, monitor)
	case http.MethodDelete:
		if err := a.store.DeleteMonitor(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

type monitorPayload struct {
	Name                   string          `json:"name"`
	ModuleType             string          `json:"module_type"`
	Schedule               string          `json:"schedule"`
	Timezone               string          `json:"timezone"`
	Enabled                *bool           `json:"enabled"`
	ModuleConfig           json.RawMessage `json:"module_config"`
	ConditionConfig        json.RawMessage `json:"condition_config"`
	NotificationChannelIDs []string        `json:"notification_channel_ids"`
}

func (a *APIServer) createMonitor(w http.ResponseWriter, r *http.Request) {
	var payload monitorPayload
	if err := decodeBody(w, r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if payload.Name == "" || payload.ModuleType == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name and module_type are required")
		return
	}
	module, ok := a.modules.Get(payload.ModuleType)
	if !ok {
		writeError(w, http.StatusBadRequest, "validation_error", "unknown module_type")
		return
	}
	if payload.Schedule == "" {
		payload.Schedule = "*/5 * * * *"
	}
	timezone := payload.Timezone
	if timezone == "" {
		timezone = a.config.Scheduler.Timezone
	}
	if err := runtimeapp.ValidateSchedule(payload.Schedule, timezone, a.config.Scheduler.Timezone); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_schedule", err.Error())
		return
	}
	moduleConfig := ensureRawJSON(payload.ModuleConfig, "{}")
	if err := module.ValidateConfig(moduleConfig); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_module_config", err.Error())
		return
	}
	conditionConfig := ensureRawJSON(payload.ConditionConfig, `{"logic":"ALL","rules":[]}`)
	var conditions core.ConditionConfig
	if err := json.Unmarshal(conditionConfig, &conditions); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_condition_config", err.Error())
		return
	}
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	now := time.Now().UTC()
	monitor := core.Monitor{ID: core.NewID(), Name: payload.Name, ModuleType: payload.ModuleType, ModuleVersion: module.Descriptor().Version, Schedule: payload.Schedule, Timezone: timezone, Enabled: enabled, ModuleConfig: moduleConfig, ConditionConfig: conditionConfig, NotificationChannelIDs: payload.NotificationChannelIDs, RuntimeState: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := a.store.CreateMonitor(r.Context(), monitor); err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, monitor)
}

func (a *APIServer) updateMonitor(w http.ResponseWriter, r *http.Request, current core.Monitor) {
	var payload monitorPayload
	if err := decodeBody(w, r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if payload.Name != "" {
		current.Name = payload.Name
	}
	if payload.ModuleType != "" && payload.ModuleType != current.ModuleType {
		current.ModuleType = payload.ModuleType
		module, ok := a.modules.Get(payload.ModuleType)
		if !ok {
			writeError(w, http.StatusBadRequest, "validation_error", "unknown module_type")
			return
		}
		current.ModuleVersion = module.Descriptor().Version
	}
	module, ok := a.modules.Get(current.ModuleType)
	if !ok {
		writeError(w, http.StatusBadRequest, "validation_error", "unknown module_type")
		return
	}
	if payload.Schedule != "" {
		current.Schedule = payload.Schedule
	}
	if payload.Timezone != "" {
		current.Timezone = payload.Timezone
	}
	if payload.Enabled != nil {
		current.Enabled = *payload.Enabled
	}
	if len(payload.ModuleConfig) > 0 {
		current.ModuleConfig = payload.ModuleConfig
	}
	if len(payload.ConditionConfig) > 0 {
		current.ConditionConfig = payload.ConditionConfig
	}
	if payload.NotificationChannelIDs != nil {
		current.NotificationChannelIDs = payload.NotificationChannelIDs
	}
	if err := runtimeapp.ValidateSchedule(current.Schedule, current.Timezone, a.config.Scheduler.Timezone); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_schedule", err.Error())
		return
	}
	if err := module.ValidateConfig(current.ModuleConfig); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_module_config", err.Error())
		return
	}
	if current.ConditionConfig == nil || len(current.ConditionConfig) == 0 {
		current.ConditionConfig = json.RawMessage(`{"logic":"ALL","rules":[]}`)
	}
	current.RuntimeState = json.RawMessage(`{}`)
	current.UpdatedAt = time.Now().UTC()
	if err := a.store.UpdateMonitor(r.Context(), current); err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, current)
}

func decodeBody(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	return decoder.Decode(target)
}

func ensureRawJSON(value json.RawMessage, fallback string) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(fallback)
	}
	return value
}

func pathParts(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"code": code, "message": message})
}

func serveFrontend(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := frontendFS.Open(path); err != nil || strings.HasSuffix(path, "/") {
		path = "index.html"
	}
	data, err := fs.ReadFile(frontendFS, path)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "page not found")
		return
	}
	contentType := "text/html; charset=utf-8"
	if strings.HasSuffix(path, ".js") {
		contentType = "application/javascript; charset=utf-8"
	} else if strings.HasSuffix(path, ".css") {
		contentType = "text/css; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(data)
}

// frontendFS is replaced by the embedded build output at compile time.
// Keeping it in one variable also makes the static server easy to test.
var frontendFS fs.FS

func SetFrontendFS(files fs.FS) {
	frontendFS = files
}

var _ = embed.FS{}

func init() {
	if frontendFS == nil {
		frontendFS = emptyFS{}
	}
}

type emptyFS struct{}

func (emptyFS) Open(string) (fs.File, error) { return nil, fmt.Errorf("frontend is not built") }
