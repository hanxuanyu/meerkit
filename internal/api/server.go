package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"meerkit/internal/app"
	"meerkit/internal/auth"
	"meerkit/internal/core"
	"meerkit/internal/monitor"
	"meerkit/internal/notification"
	"meerkit/internal/notification/inapp"
	pluginruntime "meerkit/internal/plugin"
	runtimeapp "meerkit/internal/runtime"
	"meerkit/internal/runtimeconfig"
	"meerkit/internal/statusboard"
	"meerkit/internal/store"
)

type APIServer struct {
	store        store.APIRepository
	modules      *monitor.Registry
	notifiers    *notification.Registry
	runner       *runtimeapp.Runner
	inAppHub     *inapp.Hub
	config       app.Config
	logger       *slog.Logger
	accessLogger *slog.Logger
	plugins      *pluginruntime.Manager
	auth         *auth.Service
	runtime      *runtimeconfig.Manager
	loginLimiter *loginLimiter
	statusBoard  *statusboard.Service
}

func (a *APIServer) SetStatusBoard(service *statusboard.Service) { a.statusBoard = service }

func NewAPIServer(store store.APIRepository, modules *monitor.Registry, notifiers *notification.Registry, runner *runtimeapp.Runner, inAppHub *inapp.Hub, plugins *pluginruntime.Manager, authService *auth.Service, config app.Config, logger, accessLogger *slog.Logger, runtimeManagers ...*runtimeconfig.Manager) *APIServer {
	var runtimeManager *runtimeconfig.Manager
	if len(runtimeManagers) > 0 {
		runtimeManager = runtimeManagers[0]
	}
	return &APIServer{store: store, modules: modules, notifiers: notifiers, runner: runner, inAppHub: inAppHub, plugins: plugins, auth: authService, runtime: runtimeManager, loginLimiter: newLoginLimiter(), config: config, logger: logger, accessLogger: accessLogger}
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
	api.GET("/auth/status", a.authStatus)
	api.POST("/auth/setup", a.authSetup)
	api.POST("/auth/login", a.authLogin)
	api.Use(a.requireAuth())
	api.POST("/auth/logout", a.authLogout)
	api.POST("/auth/change-key", a.authChangeKey)
	api.GET("/auth/session", a.authSession)
	api.GET("/plugins", a.listPlugins)
	api.POST("/plugins/import", a.importPlugin)
	api.POST("/plugins/scan", a.scanPlugins)
	api.GET("/plugins/:id/:version", a.pluginDetails)
	api.POST("/plugins/:id/:version/trust", a.trustPluginPublisher)
	api.POST("/plugins/:id/:version/enable", a.enablePlugin)
	api.POST("/plugins/:id/:version/disable", a.disablePlugin)
	api.DELETE("/plugins/:id/:version", a.uninstallPlugin)
	api.GET("/plugins/:id/:version/export", a.exportPlugin)
	api.GET("/plugins/:id/:version/logs", a.pluginLogs)
	api.GET("/plugins/:id/:version/logs/stream", a.streamPluginLogs)
	api.GET("/modules", legacyParts(a.handleModules))
	api.GET("/modules/:type", func(c *gin.Context) { a.handleModules(c.Writer, c.Request, []string{c.Param("type")}) })
	api.GET("/notifiers", legacy(a.handleNotifiers))
	api.GET("/system", func(c *gin.Context) {
		runtime := a.runtimeSnapshot()
		writeJSON(c.Writer, http.StatusOK, map[string]any{
			"server": a.config.ListenAddress(), "retention": runtime.Storage.Retention,
			"notification_retention": runtime.Storage.NotificationRetention, "cleanup_interval": runtime.Storage.CleanupInterval,
			"timezone": runtime.Scheduler.Timezone,
		})
	})
	api.GET("/system/logs", a.systemLogs)
	api.GET("/system/logs/stream", a.streamSystemLogs)
	api.GET("/system/config", func(c *gin.Context) {
		a.systemConfig(c)
	})
	api.PATCH("/system/config/runtime/:type", func(c *gin.Context) { a.updateSystemConfig(c, c.Param("type")) })
	api.POST("/system/config/runtime/:type/reset", func(c *gin.Context) { a.resetSystemConfig(c, c.Param("type")) })
	api.POST("/system/config/runtime/reset", a.resetAllSystemConfigs)
	api.POST("/system/config/transfer/export", a.exportConfiguration)
	api.POST("/system/config/transfer/import/preview", a.previewConfigurationImport)
	api.POST("/system/config/transfer/import", a.importConfiguration)

	api.GET("/notification-channels", legacyParts(a.handleChannels))
	api.POST("/notification-channels", legacyParts(a.handleChannels))
	api.POST("/notification-channels/test", legacy(a.testNotification))
	api.GET("/notification-channels/:id", func(c *gin.Context) { a.handleChannels(c.Writer, c.Request, []string{c.Param("id")}) })
	api.PATCH("/notification-channels/:id", func(c *gin.Context) { a.handleChannels(c.Writer, c.Request, []string{c.Param("id")}) })
	api.PUT("/notification-channels/:id", func(c *gin.Context) { a.handleChannels(c.Writer, c.Request, []string{c.Param("id")}) })
	api.DELETE("/notification-channels/:id", func(c *gin.Context) { a.handleChannels(c.Writer, c.Request, []string{c.Param("id")}) })
	api.POST("/notification-channels/:id/test", func(c *gin.Context) { a.handleChannels(c.Writer, c.Request, []string{c.Param("id"), "test"}) })
	api.GET("/in-app-notifications", legacy(a.handleInAppNotifications))
	api.GET("/in-app-notifications/unread-count", legacy(a.handleInAppNotificationCount))
	api.POST("/in-app-notifications/read-all", legacy(a.markAllInAppNotificationsRead))
	api.DELETE("/in-app-notifications/read", legacy(a.deleteReadInAppNotifications))
	api.GET("/in-app-notifications/ws", a.handleInAppNotificationStream)
	api.GET("/in-app-notifications/:id", func(c *gin.Context) { a.getInAppNotification(c.Writer, c.Request, c.Param("id")) })
	api.PATCH("/in-app-notifications/:id/read", func(c *gin.Context) { a.markInAppNotificationRead(c.Writer, c.Request, c.Param("id")) })

	api.GET("/monitors", legacyParts(a.handleMonitors))
	api.POST("/monitors", legacyParts(a.handleMonitors))
	api.GET("/monitors/:id", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id")}) })
	api.PATCH("/monitors/:id", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id")}) })
	api.PUT("/monitors/:id", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id")}) })
	api.DELETE("/monitors/:id", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id")}) })
	api.POST("/monitors/:id/run", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id"), "run"}) })
	api.POST("/monitors/test", legacy(a.testMonitor))
	api.POST("/schedules/preview", legacy(a.previewSchedule))
	api.GET("/monitors/:id/records", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id"), "records"}) })
	api.DELETE("/monitors/:id/records", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id"), "records"}) })
	api.GET("/monitors/:id/records/:record_id", func(c *gin.Context) {
		a.handleMonitors(c.Writer, c.Request, []string{c.Param("id"), "records", c.Param("record_id")})
	})
	api.GET("/monitors/:id/next-runs", func(c *gin.Context) { a.handleMonitors(c.Writer, c.Request, []string{c.Param("id"), "next-runs"}) })
	api.GET("/status-board", a.getStatusBoard)
	api.GET("/status-board/sources", a.getStatusBoardSources)
	api.POST("/status-board/items", a.createStatusBoardItem)
	api.GET("/status-board/items/:id", a.getStatusBoardItem)
	api.PATCH("/status-board/items/:id", a.updateStatusBoardItem)
	api.DELETE("/status-board/items/:id", a.deleteStatusBoardItem)
	api.GET("/status-board/ws", a.handleStatusBoardStream)

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
		descriptor, ok := a.modules.Descriptor(parts[0])
		if !ok {
			writeError(w, http.StatusNotFound, "module_not_found", "module not found")
			return
		}
		writeJSON(w, http.StatusOK, descriptor)
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
		if channel.BuiltIn && (payload.Name != "" || payload.NotifierType != "" || len(payload.Config) > 0) {
			writeError(w, http.StatusForbidden, "built_in_channel", "built-in notification channel configuration cannot be changed")
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
		if channel.BuiltIn {
			writeError(w, http.StatusForbidden, "built_in_channel", "built-in notification channel cannot be deleted")
			return
		}
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
	if payload.NotifierType == "inapp" {
		writeError(w, http.StatusForbidden, "built_in_channel", "in-app notification channel is built in")
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

func (a *APIServer) handleInAppNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	result, err := a.store.ListInAppNotificationsPage(r.Context(), store.NotificationListOptions{
		Page: queryInt(r, "page", 1), PageSize: queryInt(r, "page_size", 20), Search: r.URL.Query().Get("q"), UnreadOnly: r.URL.Query().Get("unread") == "true",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *APIServer) handleInAppNotificationCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	count, err := a.store.CountUnreadInAppNotifications(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}

func (a *APIServer) getInAppNotification(w http.ResponseWriter, r *http.Request, id string) {
	notification, err := a.store.GetInAppNotification(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "notification_not_found", "notification not found")
		return
	}
	writeJSON(w, http.StatusOK, notification)
}

func (a *APIServer) markInAppNotificationRead(w http.ResponseWriter, r *http.Request, id string) {
	notification, err := a.store.MarkInAppNotificationRead(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "notification_not_found", "notification not found")
		return
	}
	count, _ := a.store.CountUnreadInAppNotifications(r.Context())
	if a.inAppHub != nil {
		a.inAppHub.Publish(inapp.StreamEvent{Type: "read", Notification: &notification, UnreadCount: count})
	}
	writeJSON(w, http.StatusOK, notification)
}

func (a *APIServer) markAllInAppNotificationsRead(w http.ResponseWriter, r *http.Request) {
	updated, err := a.store.MarkAllInAppNotificationsRead(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	if a.inAppHub != nil {
		a.inAppHub.Publish(inapp.StreamEvent{Type: "read_all", UnreadCount: 0})
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": updated, "count": 0})
}

func (a *APIServer) deleteReadInAppNotifications(w http.ResponseWriter, r *http.Request) {
	deleted, err := a.store.DeleteReadInAppNotifications(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	count, err := a.store.CountUnreadInAppNotifications(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	if a.inAppHub != nil {
		a.inAppHub.Publish(inapp.StreamEvent{Type: "deleted_read", UnreadCount: count})
	}
	if a.logger != nil {
		a.logger.Info("read in-app notifications deleted", "deleted", deleted)
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted, "count": count})
}

var inAppUpgrader = websocket.Upgrader{CheckOrigin: func(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, request.Host)
}}

func (a *APIServer) handleInAppNotificationStream(c *gin.Context) {
	if a.inAppHub == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "notification_stream_unavailable", "notification stream is unavailable")
		return
	}
	connection, err := inAppUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	stream, unsubscribe := a.inAppHub.Subscribe()
	defer unsubscribe()
	count, _ := a.store.CountUnreadInAppNotifications(c.Request.Context())
	if err := connection.WriteJSON(inapp.StreamEvent{Type: "connected", UnreadCount: count}); err != nil {
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := connection.ReadMessage(); err != nil {
				return
			}
		}
	}()
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case event, ok := <-stream:
			if !ok || connection.WriteJSON(event) != nil {
				return
			}
		case <-ping.C:
			if connection.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)) != nil {
				return
			}
		case <-done:
			return
		case <-c.Request.Context().Done():
			return
		}
	}
}

func (a *APIServer) handleMonitors(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			runtime := a.runtimeSnapshot()
			monitors, err := a.store.ListMonitorsPage(r.Context(), store.MonitorListOptions{
				Page:                 queryInt(r, "page", 1),
				PageSize:             queryInt(r, "page_size", 20),
				Search:               r.URL.Query().Get("q"),
				ModuleType:           r.URL.Query().Get("module_type"),
				Status:               r.URL.Query().Get("status"),
				AvailableModuleTypes: a.modules.Types(),
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, monitorListPage(monitors, runtime.Scheduler.Timezone, a.modules))
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
			if errors.Is(err, runtimeapp.ErrMonitorModuleUnavailable) {
				writeError(w, http.StatusConflict, "module_unavailable", "对应插件当前不可用，监控调度已暂停")
				return
			}
			if err != nil {
				writeError(w, http.StatusBadRequest, "run_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, record)
		case "records":
			if r.Method == http.MethodDelete && len(parts) == 2 {
				if _, err := a.store.GetMonitor(r.Context(), id); err != nil {
					writeError(w, http.StatusNotFound, "monitor_not_found", "monitor not found")
					return
				}
				deleted, err := a.store.DeleteMonitorRecords(r.Context(), id)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
					return
				}
				if a.logger != nil {
					a.logger.Info("monitor records deleted", "monitor_id", id, "deleted", deleted)
				}
				if a.statusBoard != nil {
					_ = a.store.ResetStatusBoardRuntimeByMonitor(r.Context(), id, time.Now().UTC())
					a.statusBoard.Publish(statusboard.StreamEvent{Type: "records_cleared", MonitorID: id})
				}
				writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
				return
			}
			if r.Method != http.MethodGet {
				writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			if len(parts) == 3 {
				record, err := a.store.GetRecord(r.Context(), id, parts[2])
				if err != nil {
					writeError(w, http.StatusNotFound, "record_not_found", "monitor record not found")
					return
				}
				descriptor, descriptorErr := a.store.GetDescriptorSnapshot(r.Context(), record.ModuleType, record.ModuleVersion)
				if descriptorErr != nil {
					descriptor, _ = a.modules.Descriptor(record.ModuleType)
				}
				writeJSON(w, http.StatusOK, struct {
					core.MonitorRecord
					Descriptor core.ModuleDescriptor `json:"descriptor"`
				}{MonitorRecord: record, Descriptor: descriptor})
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
			next, err := runtimeapp.NextScheduleTimes(monitor.Schedules, a.runtimeSnapshot().Scheduler.Timezone, 5)
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
		writeJSON(w, http.StatusOK, monitorListItemFor(monitor, a.runtimeSnapshot().Scheduler.Timezone, a.modules))
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
		if a.statusBoard != nil {
			a.statusBoard.Publish(statusboard.StreamEvent{Type: "monitor_deleted", MonitorID: id})
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

type monitorListItem struct {
	core.Monitor
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
	ModuleAvailable bool       `json:"module_available"`
	PauseReason     string     `json:"pause_reason,omitempty"`
}

func monitorListPage(page store.PageResult[core.Monitor], timezone string, registry *monitor.Registry) store.PageResult[monitorListItem] {
	result := store.PageResult[monitorListItem]{
		Items:      make([]monitorListItem, 0, len(page.Items)),
		Page:       page.Page,
		PageSize:   page.PageSize,
		Total:      page.Total,
		TotalPages: page.TotalPages,
	}
	for _, value := range page.Items {
		result.Items = append(result.Items, monitorListItemFor(value, timezone, registry))
	}
	return result
}

func monitorListItemFor(value core.Monitor, timezone string, registry *monitor.Registry) monitorListItem {
	available := monitorModuleAvailable(value, registry)
	item := monitorListItem{Monitor: value, ModuleAvailable: available}
	if value.Enabled && !available {
		item.PauseReason = "module_unavailable"
	}
	if value.Enabled && available {
		if next, err := runtimeapp.NextScheduleTimes(value.Schedules, timezone, 1); err == nil && len(next) > 0 {
			item.NextRunAt = &next[0]
		}
	}
	return item
}

func monitorModuleAvailable(value core.Monitor, registry *monitor.Registry) bool {
	if registry == nil {
		return false
	}
	module, ok := registry.Get(value.ModuleType)
	return ok && (value.ModuleVersion == "" || module.Descriptor().Version == value.ModuleVersion)
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

func (a *APIServer) previewSchedule(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Expression string `json:"expression"`
	}
	if err := decodeBody(w, r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	payload.Expression = strings.TrimSpace(payload.Expression)
	timezone := a.runtimeSnapshot().Scheduler.Timezone
	if err := runtimeapp.ValidateSchedules([]string{payload.Expression}, timezone); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_schedule", err.Error())
		return
	}
	next, err := runtimeapp.NextScheduleTimes([]string{payload.Expression}, timezone, 3)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_schedule", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"expression":  payload.Expression,
		"description": runtimeapp.DescribeSchedule(payload.Expression),
		"next_runs":   next,
		"timezone":    timezone,
	})
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
	if err := runtimeapp.ValidateSchedules(payload.Schedules, a.runtimeSnapshot().Scheduler.Timezone); err != nil {
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
	if conditions.NotificationPolicy != "" && conditions.NotificationPolicy != core.NotificationPolicyOnce && conditions.NotificationPolicy != core.NotificationPolicyEvery {
		writeError(w, http.StatusBadRequest, "invalid_condition_config", "notification_policy must be once or every")
		return
	}
	conditionConfig = normalizedConditionConfig(conditions)
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	now := time.Now().UTC()
	descriptor := module.Descriptor()
	configVersion := descriptor.ConfigVersion
	if configVersion == "" {
		configVersion = "1"
	}
	monitor := core.Monitor{ID: core.NewID(), Name: payload.Name, ModuleType: payload.ModuleType, ModuleVersion: descriptor.Version, ModuleConfigVersion: configVersion, Schedules: payload.Schedules, Enabled: enabled, ModuleConfig: moduleConfig, ConditionConfig: conditionConfig, NotificationChannelIDs: payload.NotificationChannelIDs, RuntimeState: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
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
		current.ModuleConfigVersion = module.Descriptor().ConfigVersion
		if current.ModuleConfigVersion == "" {
			current.ModuleConfigVersion = "1"
		}
	}
	if payload.Enabled != nil {
		current.Enabled = *payload.Enabled
	}
	module, ok := a.modules.Get(current.ModuleType)
	if !ok && (current.Enabled || len(payload.ModuleConfig) > 0) {
		writeError(w, http.StatusBadRequest, "validation_error", "unknown module_type")
		return
	}
	if payload.Schedules != nil {
		current.Schedules = normalizeSchedules(payload.Schedules)
	}
	if len(payload.ModuleConfig) > 0 {
		current.ModuleConfig = payload.ModuleConfig
	}
	if len(payload.ConditionConfig) > 0 {
		var conditions core.ConditionConfig
		if err := json.Unmarshal(payload.ConditionConfig, &conditions); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_condition_config", err.Error())
			return
		}
		if conditions.NotificationPolicy != "" && conditions.NotificationPolicy != core.NotificationPolicyOnce && conditions.NotificationPolicy != core.NotificationPolicyEvery {
			writeError(w, http.StatusBadRequest, "invalid_condition_config", "notification_policy must be once or every")
			return
		}
		current.ConditionConfig = normalizedConditionConfig(conditions)
	}
	if payload.NotificationChannelIDs != nil {
		current.NotificationChannelIDs = payload.NotificationChannelIDs
	}
	if err := runtimeapp.ValidateSchedules(current.Schedules, a.runtimeSnapshot().Scheduler.Timezone); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_schedule", err.Error())
		return
	}
	if ok {
		if err := module.ValidateConfig(current.ModuleConfig); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_module_config", err.Error())
			return
		}
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

func normalizedConditionConfig(config core.ConditionConfig) json.RawMessage {
	for index := range config.Rules {
		if config.Rules[index].ID == "" {
			config.Rules[index].ID = core.NewID()
		}
	}
	data, _ := json.Marshal(config)
	return data
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
