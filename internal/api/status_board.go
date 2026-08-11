package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"reflect"
	"sort"
	"strings"
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

type statusBoardSharePayload struct {
	Name       string   `json:"name"`
	MonitorIDs []string `json:"monitor_ids"`
	ItemIDs    []string `json:"item_ids"`
}

type statusBoardShareCreated struct {
	core.StatusBoardShare
	URL string `json:"url"`
}

type publicStatusBoardMonitor struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ModuleType string `json:"module_type"`
	Enabled    bool   `json:"enabled"`
}

type publicStatusBoardItem struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Enabled          bool                `json:"enabled"`
	SourceLabel      string              `json:"source_label"`
	Samples          []core.StatusSample `json:"samples"`
	Current          *core.StatusSample  `json:"current,omitempty"`
	ActiveTrendRules int                 `json:"active_trend_rules"`
}

type publicStatusBoardGroup struct {
	Monitor publicStatusBoardMonitor `json:"monitor"`
	Items   []publicStatusBoardItem  `json:"items"`
}

type publicStatusBoardResponse struct {
	Name        string                   `json:"name"`
	GeneratedAt time.Time                `json:"generated_at"`
	Groups      []publicStatusBoardGroup `json:"groups"`
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

func (a *APIServer) listStatusBoardShares(c *gin.Context) {
	shares, err := a.store.ListStatusBoardShares(c.Request.Context())
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	responses := make([]statusBoardShareCreated, 0, len(shares))
	for _, share := range shares {
		responses = append(responses, statusBoardShareCreated{StatusBoardShare: share, URL: "/shared/status-board/" + share.Token})
	}
	writeJSON(c.Writer, http.StatusOK, responses)
}

func (a *APIServer) createStatusBoardShare(c *gin.Context) {
	service := a.requireStatusBoard(c)
	if service == nil {
		return
	}
	var payload statusBoardSharePayload
	if err := decodeBody(c.Writer, c.Request, &payload); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" || len([]rune(payload.Name)) > 80 {
		writeError(c.Writer, http.StatusBadRequest, "validation_error", "分享名称不能为空且不能超过 80 个字符")
		return
	}
	snapshot, err := service.Snapshot(c.Request.Context())
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "status_board_failed", err.Error())
		return
	}
	monitorIDs, itemIDs, err := normalizeStatusBoardShareSelection(snapshot, payload.MonitorIDs, payload.ItemIDs)
	if err != nil {
		writeError(c.Writer, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	token, err := randomStatusBoardShareToken()
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "token_generation_failed", err.Error())
		return
	}
	share := core.StatusBoardShare{ID: core.NewID(), Token: token, Name: payload.Name, MonitorIDs: monitorIDs, ItemIDs: itemIDs, Active: true, CreatedAt: time.Now().UTC()}
	if err := a.store.CreateStatusBoardShare(c.Request.Context(), share); err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusCreated, statusBoardShareCreated{StatusBoardShare: share, URL: "/shared/status-board/" + token})
}

func (a *APIServer) deleteStatusBoardShare(c *gin.Context) {
	if err := a.store.SetStatusBoardShareActive(c.Request.Context(), c.Param("id"), false); err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *APIServer) restoreStatusBoardShare(c *gin.Context) {
	if err := a.store.SetStatusBoardShareActive(c.Request.Context(), c.Param("id"), true); err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *APIServer) permanentlyDeleteStatusBoardShare(c *gin.Context) {
	deleted, err := a.store.DeleteStatusBoardShare(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	if !deleted {
		writeError(c.Writer, http.StatusConflict, "share_active", "请先停用共享链接，再执行永久删除")
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *APIServer) getPublicStatusBoardShare(c *gin.Context) {
	token := c.Param("token")
	if len(token) < 32 || len(token) > 128 {
		writeError(c.Writer, http.StatusNotFound, "share_not_found", "共享链接不存在或已取消")
		return
	}
	share, err := a.store.GetStatusBoardShareByToken(c.Request.Context(), token)
	if err != nil || !share.Active {
		writeError(c.Writer, http.StatusNotFound, "share_not_found", "共享链接不存在或已取消")
		return
	}
	service := a.requireStatusBoard(c)
	if service == nil {
		return
	}
	snapshot, err := service.Snapshot(c.Request.Context())
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "status_board_failed", err.Error())
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Robots-Tag", "noindex, nofollow")
	writeJSON(c.Writer, http.StatusOK, filterPublicStatusBoardSnapshot(snapshot, share))
}

func normalizeStatusBoardShareSelection(snapshot core.StatusBoardSnapshot, monitorIDs, itemIDs []string) ([]string, []string, error) {
	availableMonitors := make(map[string]struct{})
	availableItems := make(map[string]string)
	for _, group := range snapshot.Groups {
		availableMonitors[group.Monitor.ID] = struct{}{}
		for _, item := range group.Items {
			availableItems[item.ID] = group.Monitor.ID
		}
	}
	selectedMonitors := make(map[string]struct{})
	for _, id := range monitorIDs {
		if _, exists := availableMonitors[id]; !exists {
			return nil, nil, errors.New("分享选择了不存在的监控分组")
		}
		selectedMonitors[id] = struct{}{}
	}
	selectedItems := make(map[string]struct{})
	for _, id := range itemIDs {
		monitorID, exists := availableItems[id]
		if !exists {
			return nil, nil, errors.New("分享选择了不存在的看板项")
		}
		if _, covered := selectedMonitors[monitorID]; !covered {
			selectedItems[id] = struct{}{}
		}
	}
	if len(selectedMonitors) == 0 && len(selectedItems) == 0 {
		return nil, nil, errors.New("请至少选择一个监控分组或看板项")
	}
	if len(selectedMonitors)+len(selectedItems) > 200 {
		return nil, nil, errors.New("单个分享最多选择 200 个分组和看板项")
	}
	normalizedMonitors := mapKeys(selectedMonitors)
	normalizedItems := mapKeys(selectedItems)
	return normalizedMonitors, normalizedItems, nil
}

func mapKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func filterPublicStatusBoardSnapshot(snapshot core.StatusBoardSnapshot, share core.StatusBoardShare) publicStatusBoardResponse {
	monitorIDs := make(map[string]struct{}, len(share.MonitorIDs))
	for _, id := range share.MonitorIDs {
		monitorIDs[id] = struct{}{}
	}
	itemIDs := make(map[string]struct{}, len(share.ItemIDs))
	for _, id := range share.ItemIDs {
		itemIDs[id] = struct{}{}
	}
	response := publicStatusBoardResponse{Name: share.Name, GeneratedAt: time.Now().UTC(), Groups: []publicStatusBoardGroup{}}
	for _, group := range snapshot.Groups {
		_, includeGroup := monitorIDs[group.Monitor.ID]
		publicGroup := publicStatusBoardGroup{Monitor: publicStatusBoardMonitor{ID: group.Monitor.ID, Name: group.Monitor.Name, ModuleType: group.Monitor.ModuleType, Enabled: group.Monitor.Enabled}, Items: []publicStatusBoardItem{}}
		for _, item := range group.Items {
			if _, includeItem := itemIDs[item.ID]; !includeGroup && !includeItem {
				continue
			}
			activeRules := 0
			for _, state := range item.RuntimeState.Rules {
				if state.Active {
					activeRules++
				}
			}
			publicGroup.Items = append(publicGroup.Items, publicStatusBoardItem{ID: item.ID, Name: item.Name, Enabled: item.Enabled, SourceLabel: item.SourceLabel, Samples: item.Samples, Current: item.Current, ActiveTrendRules: activeRules})
		}
		if len(publicGroup.Items) > 0 {
			response.Groups = append(response.Groups, publicGroup)
		}
	}
	return response
}

func randomStatusBoardShareToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
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
