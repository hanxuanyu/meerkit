package api

import (
	"database/sql"
	"errors"
	"net/http"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"

	"meerkit/internal/core"
	"meerkit/internal/statusboard"
)

type statusBoardItemPayload struct {
	Name                   *string                 `json:"name"`
	MonitorID              *string                 `json:"monitor_id"`
	Enabled                *bool                   `json:"enabled"`
	Source                 *core.StatusItemSource  `json:"source"`
	Invert                 *bool                   `json:"invert"`
	Thresholds             *[]core.StatusThreshold `json:"thresholds"`
	HistoryLimit           *int                    `json:"history_limit"`
	TrendRules             *[]core.TrendRule       `json:"trend_rules"`
	NotificationChannelIDs *[]string               `json:"notification_channel_ids"`
}

func (a *APIServer) requireStatusBoard(c *gin.Context) *statusboard.Service {
	if a.statusBoard == nil {
		writeError(c.Writer, http.StatusServiceUnavailable, "status_board_unavailable", "status board is unavailable")
		return nil
	}
	return a.statusBoard
}

func (a *APIServer) getStatusBoard(c *gin.Context) {
	service := a.requireStatusBoard(c)
	if service == nil {
		return
	}
	snapshot, err := service.Snapshot(c.Request.Context())
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "status_board_failed", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, snapshot)
}

func (a *APIServer) getStatusBoardSources(c *gin.Context) {
	service := a.requireStatusBoard(c)
	if service == nil {
		return
	}
	sources, err := service.Sources(c.Request.Context(), c.Query("monitor_id"))
	if err != nil {
		writeError(c.Writer, http.StatusNotFound, "monitor_not_found", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, sources)
}

func (a *APIServer) createStatusBoardItem(c *gin.Context) {
	service := a.requireStatusBoard(c)
	if service == nil {
		return
	}
	var payload statusBoardItemPayload
	if err := decodeBody(c.Writer, c.Request, &payload); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if payload.Name == nil || payload.MonitorID == nil || payload.Source == nil {
		writeError(c.Writer, http.StatusBadRequest, "validation_error", "name, monitor_id and source are required")
		return
	}
	now := time.Now().UTC()
	item := core.StatusBoardItem{ID: core.NewID(), Name: *payload.Name, MonitorID: *payload.MonitorID, Enabled: true, Source: *payload.Source, HistoryLimit: 60, CreatedAt: now, UpdatedAt: now}
	applyStatusBoardItemPayload(&item, payload)
	if err := service.NormalizeAndValidate(c.Request.Context(), &item, true); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if err := a.store.CreateStatusBoardItem(c.Request.Context(), item); err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	service.Publish(statusboard.StreamEvent{Type: "item_created", MonitorID: item.MonitorID, ItemID: item.ID})
	writeJSON(c.Writer, http.StatusCreated, item)
}

func (a *APIServer) getStatusBoardItem(c *gin.Context) {
	if a.requireStatusBoard(c) == nil {
		return
	}
	item, err := a.store.GetStatusBoardItem(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c.Writer, http.StatusNotFound, "status_item_not_found", "status board item not found")
		return
	}
	writeJSON(c.Writer, http.StatusOK, item)
}

func (a *APIServer) updateStatusBoardItem(c *gin.Context) {
	service := a.requireStatusBoard(c)
	if service == nil {
		return
	}
	item, err := a.store.GetStatusBoardItem(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c.Writer, http.StatusNotFound, "status_item_not_found", "status board item not found")
		return
	}
	previousEvaluation, previousRules := statusBoardEvaluationConfig(item), item.TrendRules
	var payload statusBoardItemPayload
	if err := decodeBody(c.Writer, c.Request, &payload); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	applyStatusBoardItemPayload(&item, payload)
	reset := !reflect.DeepEqual(previousEvaluation, statusBoardEvaluationConfig(item)) || !reflect.DeepEqual(previousRules, item.TrendRules)
	item.UpdatedAt = time.Now().UTC()
	if err := service.NormalizeAndValidate(c.Request.Context(), &item, reset); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if err := a.store.UpdateStatusBoardItem(c.Request.Context(), item); err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	service.Publish(statusboard.StreamEvent{Type: "item_updated", MonitorID: item.MonitorID, ItemID: item.ID})
	writeJSON(c.Writer, http.StatusOK, item)
}

func statusBoardEvaluationConfig(item core.StatusBoardItem) struct {
	Source     core.StatusItemSource
	Thresholds []core.StatusThreshold
} {
	source := item.Source
	source.DefaultColor, source.DefaultLabel = "", ""
	source.ValueMappings = append([]core.StatusValueMapping(nil), source.ValueMappings...)
	for index := range source.ValueMappings {
		source.ValueMappings[index].Color = ""
		source.ValueMappings[index].Label = ""
	}
	thresholds := append([]core.StatusThreshold(nil), item.Thresholds...)
	for index := range thresholds {
		thresholds[index].Color = ""
		thresholds[index].Label = ""
	}
	return struct {
		Source     core.StatusItemSource
		Thresholds []core.StatusThreshold
	}{Source: source, Thresholds: thresholds}
}

func (a *APIServer) deleteStatusBoardItem(c *gin.Context) {
	service := a.requireStatusBoard(c)
	if service == nil {
		return
	}
	item, err := a.store.GetStatusBoardItem(c.Request.Context(), c.Param("id"))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(c.Writer, http.StatusNotFound, "status_item_not_found", "status board item not found")
		return
	}
	if err := a.store.DeleteStatusBoardItem(c.Request.Context(), item.ID); err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	service.Publish(statusboard.StreamEvent{Type: "item_deleted", MonitorID: item.MonitorID, ItemID: item.ID})
	c.Status(http.StatusNoContent)
}

func applyStatusBoardItemPayload(item *core.StatusBoardItem, payload statusBoardItemPayload) {
	if payload.Name != nil {
		item.Name = *payload.Name
	}
	if payload.MonitorID != nil {
		item.MonitorID = *payload.MonitorID
	}
	if payload.Enabled != nil {
		item.Enabled = *payload.Enabled
	}
	if payload.Source != nil {
		item.Source = *payload.Source
	}
	if payload.Invert != nil {
		item.Invert = *payload.Invert
	}
	if payload.Thresholds != nil {
		item.Thresholds = *payload.Thresholds
	}
	if payload.HistoryLimit != nil {
		item.HistoryLimit = *payload.HistoryLimit
	}
	if payload.TrendRules != nil {
		item.TrendRules = *payload.TrendRules
	}
	if payload.NotificationChannelIDs != nil {
		item.NotificationChannelIDs = *payload.NotificationChannelIDs
	}
}

func (a *APIServer) handleStatusBoardStream(c *gin.Context) {
	service := a.requireStatusBoard(c)
	if service == nil {
		return
	}
	connection, err := inAppUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	stream, unsubscribe := service.Hub().Subscribe()
	defer unsubscribe()
	if err := connection.WriteJSON(statusboard.StreamEvent{Type: "connected"}); err != nil {
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, readErr := connection.ReadMessage(); readErr != nil {
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
			if connection.WriteControl(9, nil, time.Now().Add(5*time.Second)) != nil {
				return
			}
		case <-done:
			return
		case <-c.Request.Context().Done():
			return
		}
	}
}
