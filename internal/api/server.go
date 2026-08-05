package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"meerkit/internal/app"
	"meerkit/internal/core"
	"meerkit/internal/monitor"
	"meerkit/internal/notification"
	runtimeapp "meerkit/internal/runtime"
	"meerkit/internal/store"
)

type APIServer struct {
	store        *store.Store
	modules      *monitor.Registry
	notifiers    *notification.Registry
	runner       *runtimeapp.Runner
	config       app.Config
	logger       *slog.Logger
	accessLogger *slog.Logger
}

func NewAPIServer(store *store.Store, modules *monitor.Registry, notifiers *notification.Registry, runner *runtimeapp.Runner, config app.Config, logger, accessLogger *slog.Logger) *APIServer {
	return &APIServer{store: store, modules: modules, notifiers: notifiers, runner: runner, config: config, logger: logger, accessLogger: accessLogger}
}

func (a *APIServer) Router() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.RedirectTrailingSlash = false
	router.Use(gin.Recovery(), a.requestLogger())

	router.GET("/healthz", func(c *gin.Context) {
		writeJSON(c.Writer, http.StatusOK, map[string]any{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		if err := a.store.Ping(c.Request.Context()); err != nil {
			writeError(c.Writer, http.StatusServiceUnavailable, "storage_unavailable", err.Error())
			return
		}
		writeJSON(c.Writer, http.StatusOK, map[string]any{"status": "ready"})
	})

	api := router.Group("/api/v1")
	api.GET("/modules", legacyParts(a.handleModules))
	api.GET("/modules/:type", func(c *gin.Context) { a.handleModules(c.Writer, c.Request, []string{c.Param("type")}) })
	api.GET("/notifiers", legacy(a.handleNotifiers))
	api.GET("/system", func(c *gin.Context) {
		writeJSON(c.Writer, http.StatusOK, map[string]any{"server": a.config.ListenAddress(), "retention": a.config.Storage.Retention, "timezone": a.config.Scheduler.Timezone})
	})
	api.GET("/system/config", func(c *gin.Context) {
		writeJSON(c.Writer, http.StatusOK, a.config.Metadata)
	})

	api.GET("/notification-channels", legacyParts(a.handleChannels))
	api.POST("/notification-channels", legacyParts(a.handleChannels))
	api.POST("/notification-channels/test", legacy(a.testNotification))
	api.GET("/notification-channels/:id", func(c *gin.Context) { a.handleChannels(c.Writer, c.Request, []string{c.Param("id")}) })
	api.PATCH("/notification-channels/:id", func(c *gin.Context) { a.handleChannels(c.Writer, c.Request, []string{c.Param("id")}) })
	api.PUT("/notification-channels/:id", func(c *gin.Context) { a.handleChannels(c.Writer, c.Request, []string{c.Param("id")}) })
	api.DELETE("/notification-channels/:id", func(c *gin.Context) { a.handleChannels(c.Writer, c.Request, []string{c.Param("id")}) })
	api.POST("/notification-channels/:id/test", func(c *gin.Context) { a.handleChannels(c.Writer, c.Request, []string{c.Param("id"), "test"}) })

	api.GET("/monitors", legacyParts(a.handleMonitors))
	api.POST("/monitors", legacyParts(a.handleMonitors))
	api.GET("/monitors/:id", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id")}) })
	api.PATCH("/monitors/:id", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id")}) })
	api.PUT("/monitors/:id", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id")}) })
	api.DELETE("/monitors/:id", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id")}) })
	api.POST("/monitors/:id/run", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id"), "run"}) })
	api.POST("/monitors/test", legacy(a.testMonitor))
	api.GET("/monitors/:id/records", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id"), "records"}) })
	api.GET("/monitors/:id/next-runs", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id"), "next-runs"}) })

	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			writeError(c.Writer, http.StatusNotFound, "not_found", "API route not found")
			return
		}
		serveFrontend(c.Writer, c.Request)
	})
	return router
}

func legacy(handler func(http.ResponseWriter, *http.Request)) gin.HandlerFunc {
	return func(c *gin.Context) { handler(c.Writer, c.Request) }
}

func legacyParts(handler func(http.ResponseWriter, *http.Request, []string)) gin.HandlerFunc {
	return func(c *gin.Context) { handler(c.Writer, c.Request, nil) }
}

func (a *APIServer) requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		if a.accessLogger != nil {
			a.accessLogger.Info("http request", "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status(), "duration_ms", time.Since(started).Milliseconds(), "client_ip", c.ClientIP(), "user_agent", c.Request.UserAgent())
		}
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
		if a.logger != nil {
			a.logger.Info("notification test sent", "channel_id", channel.ID, "notifier_type", channel.NotifierType)
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
		if a.logger != nil {
			a.logger.Info("notification channel updated", "channel_id", channel.ID, "channel_name", channel.Name, "notifier_type", channel.NotifierType, "enabled", channel.Enabled)
		}
		writeJSON(w, http.StatusOK, channel)
	case http.MethodDelete:
		if err := a.store.DeleteChannel(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
			return
		}
		if a.logger != nil {
			a.logger.Info("notification channel deleted", "channel_id", id, "channel_name", channel.Name, "notifier_type", channel.NotifierType)
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (a *APIServer) testNotification(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		NotifierType string          `json:"notifier_type"`
		Config       json.RawMessage `json:"config"`
	}
	if err := decodeBody(w, r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
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
	event := core.NotificationEvent{EventType: "test", MonitorName: "Meerkit test notification", ModuleType: "system", TriggeredAt: time.Now().UTC(), ConditionState: "true", Summary: "This is a test notification."}
	if err := notifier.Send(r.Context(), payload.Config, event); err != nil {
		writeError(w, http.StatusBadRequest, "notification_failed", err.Error())
		return
	}
	if a.logger != nil {
		a.logger.Info("notification configuration test sent", "notifier_type", payload.NotifierType)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "sent"})
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
	if a.logger != nil {
		a.logger.Info("notification channel created", "channel_id", channel.ID, "channel_name", channel.Name, "notifier_type", channel.NotifierType, "enabled", channel.Enabled)
	}
	writeJSON(w, http.StatusCreated, channel)
}

func (a *APIServer) handleMonitors(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			monitors, err := a.store.ListMonitorsPage(r.Context(), store.MonitorListOptions{
				Page:       queryInt(r, "page", 1),
				PageSize:   queryInt(r, "page_size", 20),
				Search:     r.URL.Query().Get("q"),
				ModuleType: r.URL.Query().Get("module_type"),
				Status:     r.URL.Query().Get("status"),
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, monitors)
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
			records, err := a.store.ListRecordsPage(r.Context(), id, store.RecordListOptions{
				Page:      queryInt(r, "page", 1),
				PageSize:  queryInt(r, "page_size", 20),
				Search:    r.URL.Query().Get("q"),
				Status:    r.URL.Query().Get("status"),
				EventType: r.URL.Query().Get("event_type"),
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, records)
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
			next, err := runtimeapp.NextScheduleTimes(monitor.Schedules, a.config.Scheduler.Timezone, 5)
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
		if a.logger != nil {
			a.logger.Info("monitor deleted", "monitor_id", id, "monitor_name", monitor.Name, "module_type", monitor.ModuleType)
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (a *APIServer) testMonitor(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ModuleType   string          `json:"module_type"`
		ModuleConfig json.RawMessage `json:"module_config"`
	}
	if err := decodeBody(w, r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	module, ok := a.modules.Get(payload.ModuleType)
	if !ok {
		writeError(w, http.StatusBadRequest, "module_not_found", "monitor module not found")
		return
	}
	payload.ModuleConfig = ensureRawJSON(payload.ModuleConfig, "{}")
	if err := module.ValidateConfig(payload.ModuleConfig); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_module_config", err.Error())
		return
	}
	observation, executeErr := module.Execute(r.Context(), payload.ModuleConfig)
	if observation.Result == nil {
		observation.Result = map[string]any{}
	}
	if executeErr != nil {
		observation.Success = false
		if observation.ErrorCode == "" {
			observation.ErrorCode = "execution_error"
		}
		if observation.ErrorMessage == "" {
			observation.ErrorMessage = executeErr.Error()
		}
	}
	if a.logger != nil {
		a.logger.Info("monitor test completed", "module_type", payload.ModuleType, "success", observation.Success && executeErr == nil, "error_code", observation.ErrorCode)
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": observation.Success && executeErr == nil, "observation": observation})
}

type monitorPayload struct {
	Name                   string          `json:"name"`
	ModuleType             string          `json:"module_type"`
	Schedules              []string        `json:"schedules"`
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
	payload.Schedules = normalizeSchedules(payload.Schedules)
	if len(payload.Schedules) == 0 {
		payload.Schedules = []string{"*/5 * * * *"}
	}
	if err := runtimeapp.ValidateSchedules(payload.Schedules, a.config.Scheduler.Timezone); err != nil {
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
	monitor := core.Monitor{ID: core.NewID(), Name: payload.Name, ModuleType: payload.ModuleType, ModuleVersion: module.Descriptor().Version, Schedules: payload.Schedules, Enabled: enabled, ModuleConfig: moduleConfig, ConditionConfig: conditionConfig, NotificationChannelIDs: payload.NotificationChannelIDs, RuntimeState: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := a.store.CreateMonitor(r.Context(), monitor); err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	if a.logger != nil {
		a.logger.Info("monitor created", "monitor_id", monitor.ID, "monitor_name", monitor.Name, "module_type", monitor.ModuleType, "schedules", monitor.Schedules, "enabled", monitor.Enabled)
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
	if payload.Schedules != nil {
		current.Schedules = normalizeSchedules(payload.Schedules)
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
	if err := runtimeapp.ValidateSchedules(current.Schedules, a.config.Scheduler.Timezone); err != nil {
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
	if a.logger != nil {
		a.logger.Info("monitor updated", "monitor_id", current.ID, "monitor_name", current.Name, "module_type", current.ModuleType, "schedules", current.Schedules, "enabled", current.Enabled)
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

func normalizeSchedules(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func queryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
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
	assetPath := strings.TrimPrefix(r.URL.Path, "/")
	if assetPath == "" || assetPath == "index.html" || strings.HasSuffix(assetPath, "/") {
		assetPath = ""
	} else if _, err := frontendFS.Open(assetPath); err != nil {
		assetPath = ""
	}
	request := r.Clone(r.Context())
	request.URL.Path = "/" + assetPath
	request.URL.RawPath = ""
	http.FileServer(http.FS(frontendFS)).ServeHTTP(w, request)
}

// frontendFS is replaced by the embedded build output at compile time.
// Keeping it in one variable also makes the static server easy to test.
var frontendFS fs.FS

func SetFrontendFS(files fs.FS) {
	frontendFS = files
}

func init() {
	if frontendFS == nil {
		frontendFS = emptyFS{}
	}
}

type emptyFS struct{}

func (emptyFS) Open(string) (fs.File, error) { return nil, fmt.Errorf("frontend is not built") }
