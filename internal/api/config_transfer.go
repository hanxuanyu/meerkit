package api

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/scrypt"

	"meerkit/internal/app"
	"meerkit/internal/auth"
	"meerkit/internal/core"
	runtimeapp "meerkit/internal/runtime"
	"meerkit/internal/statusboard"
	"meerkit/internal/store"
)

const (
	configBundleFormat       = "meerkit-config"
	configBundleVersion      = 1
	configBundleFilename     = "meerkit-config.json"
	maxConfigUploadBytes     = 10 << 20
	maxConfigJSONBytes       = 20 << 20
	maxConfigBundleEntries   = 8
	configEncryptionKeyBytes = 32
)

type configBundle struct {
	Format     string            `json:"format"`
	Version    int               `json:"version"`
	ExportedAt time.Time         `json:"exported_at"`
	Encrypted  bool              `json:"encrypted"`
	Data       *configBundleData `json:"data,omitempty"`
	Protected  *encryptedPayload `json:"protected_data,omitempty"`
}

type configBundleData struct {
	Contents             []string                   `json:"contents"`
	Runtime              app.RuntimeConfig          `json:"runtime"`
	Monitors             []core.Monitor             `json:"monitors,omitempty"`
	NotificationChannels []core.NotificationChannel `json:"notification_channels,omitempty"`
	StatusBoardItems     []core.StatusBoardItem     `json:"status_board_items,omitempty"`
	StatusBoardShares    []configBundleShare        `json:"status_board_shares,omitempty"`
	AdminKeyHash         string                     `json:"admin_key_hash,omitempty"`
}

type configBundleShare struct {
	ID         string    `json:"id"`
	Token      string    `json:"token"`
	Name       string    `json:"name"`
	MonitorIDs []string  `json:"monitor_ids"`
	ItemIDs    []string  `json:"item_ids"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
}

type encryptedPayload struct {
	Algorithm  string `json:"algorithm"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type configExportRequest struct {
	Encrypted            bool   `json:"encrypted"`
	Monitors             bool   `json:"monitors"`
	NotificationChannels bool   `json:"notification_channels"`
	StatusBoardItems     bool   `json:"status_board_items"`
	StatusBoardShares    bool   `json:"status_board_shares"`
	AdminKey             bool   `json:"admin_key"`
	EncryptionKey        string `json:"encryption_key"`
	EncryptionKeyConfirm string `json:"encryption_key_confirm"`
}

type configImportResult struct {
	Imported             bool                  `json:"imported"`
	AdminKeyImported     bool                  `json:"admin_key_imported"`
	Monitors             int                   `json:"monitors"`
	NotificationChannels int                   `json:"notification_channels"`
	StatusBoardItems     int                   `json:"status_board_items"`
	StatusBoardShares    int                   `json:"status_board_shares"`
	RuntimeTypes         int                   `json:"runtime_types"`
	Summary              configTransferSummary `json:"summary"`
}

type configImportPreview struct {
	Encrypted  bool                  `json:"encrypted"`
	ExportedAt time.Time             `json:"exported_at"`
	Mode       string                `json:"mode"`
	Summary    configTransferSummary `json:"summary"`
}

type configExportSummary struct {
	Encrypted                    bool `json:"encrypted"`
	RuntimeTypes                 int  `json:"runtime_types"`
	Monitors                     int  `json:"monitors"`
	MonitorsIncluded             bool `json:"monitors_included"`
	NotificationChannels         int  `json:"notification_channels"`
	NotificationChannelsIncluded bool `json:"notification_channels_included"`
	StatusBoardItems             int  `json:"status_board_items"`
	StatusBoardItemsIncluded     bool `json:"status_board_items_included"`
	StatusBoardShares            int  `json:"status_board_shares"`
	StatusBoardSharesIncluded    bool `json:"status_board_shares_included"`
	AdminKey                     bool `json:"admin_key"`
}

type configTransferItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type configTransferChanges struct {
	Added       []configTransferItem `json:"added"`
	Overwritten []configTransferItem `json:"overwritten"`
	Deleted     []configTransferItem `json:"deleted"`
}

type configTransferSummary struct {
	Runtime              configTransferChanges `json:"runtime"`
	Monitors             configTransferChanges `json:"monitors"`
	NotificationChannels configTransferChanges `json:"notification_channels"`
	StatusBoardItems     configTransferChanges `json:"status_board_items"`
	StatusBoardShares    configTransferChanges `json:"status_board_shares"`
	AdminKey             configTransferChanges `json:"admin_key"`
}

type parsedConfigImport struct {
	Bundle  configBundle
	Payload *configBundleData
	Mode    string
}

func (a *APIServer) exportConfiguration(c *gin.Context) {
	var request configExportRequest
	if err := decodeBody(c.Writer, c.Request, &request); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if request.Encrypted {
		if len(request.EncryptionKey) < 12 || request.EncryptionKey != request.EncryptionKeyConfirm {
			writeError(c.Writer, http.StatusBadRequest, "invalid_encryption_key", "配置包密码至少需要 12 个字符，且两次输入必须一致")
			return
		}
	}
	bundle := configBundle{Format: configBundleFormat, Version: configBundleVersion, ExportedAt: time.Now().UTC(), Encrypted: request.Encrypted}
	payload := configBundleData{Contents: []string{"runtime"}, Runtime: a.runtime.Snapshot()}
	if request.Monitors {
		monitors, err := a.store.ListMonitors(c.Request.Context())
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
			return
		}
		for index := range monitors {
			monitors[index].RuntimeState = json.RawMessage(`{}`)
		}
		payload.Monitors, payload.Contents = monitors, append(payload.Contents, "monitors")
	}
	if request.NotificationChannels {
		channels, err := a.store.ListChannels(c.Request.Context())
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
			return
		}
		for _, channel := range channels {
			if channel.BuiltIn || channel.ID == core.BuiltInNotificationChannelID {
				continue
			}
			payload.NotificationChannels = append(payload.NotificationChannels, channel)
		}
		payload.Contents = append(payload.Contents, "notification_channels")
	}
	if request.StatusBoardItems {
		items, err := a.store.ListStatusBoardItems(c.Request.Context())
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
			return
		}
		for index := range items {
			items[index].RuntimeState = core.StatusItemRuntimeState{Rules: map[string]core.TrendRuleState{}}
		}
		payload.StatusBoardItems, payload.Contents = items, append(payload.Contents, "status_board_items")
	}
	if request.StatusBoardShares {
		shares, err := a.store.ListStatusBoardShares(c.Request.Context())
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
			return
		}
		payload.StatusBoardShares = configBundleSharesFromDomain(shares)
		payload.Contents = append(payload.Contents, "status_board_shares")
	}
	if request.AdminKey {
		hash, err := a.store.AdminKeyHash(c.Request.Context())
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
			return
		}
		if hash == "" {
			writeError(c.Writer, http.StatusBadRequest, "admin_key_unavailable", "当前尚未设置管理员密钥")
			return
		}
		payload.AdminKeyHash = hash
		payload.Contents = append(payload.Contents, "admin_key")
	}
	if request.Encrypted {
		encoded, err := json.Marshal(payload)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "encoding_failed", err.Error())
			return
		}
		protected, err := encryptPayload(encoded, request.EncryptionKey)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "encryption_failed", err.Error())
			return
		}
		bundle.Protected = &protected
	} else {
		bundle.Data = &payload
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "encoding_failed", err.Error())
		return
	}
	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	entry, err := zipWriter.Create(configBundleFilename)
	if err == nil {
		_, err = entry.Write(data)
	}
	if err == nil {
		err = zipWriter.Close()
	} else {
		_ = zipWriter.Close()
	}
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "archive_failed", err.Error())
		return
	}
	exportSummary, _ := json.Marshal(configExportSummary{
		Encrypted:                    request.Encrypted,
		RuntimeTypes:                 5,
		Monitors:                     len(payload.Monitors),
		MonitorsIncluded:             request.Monitors,
		NotificationChannels:         len(payload.NotificationChannels),
		NotificationChannelsIncluded: request.NotificationChannels,
		StatusBoardItems:             len(payload.StatusBoardItems),
		StatusBoardItemsIncluded:     request.StatusBoardItems,
		StatusBoardShares:            len(payload.StatusBoardShares),
		StatusBoardSharesIncluded:    request.StatusBoardShares,
		AdminKey:                     payload.AdminKeyHash != "",
	})
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="meerkit-config-%s.zip"`, time.Now().UTC().Format("20060102-150405")))
	c.Header("X-Meerkit-Export-Summary", string(exportSummary))
	c.Data(http.StatusOK, "application/zip", archive.Bytes())
}

func (a *APIServer) previewConfigurationImport(c *gin.Context) {
	parsed, ok := a.parseConfigurationImport(c)
	if !ok {
		return
	}
	summary, err := a.buildConfigImportSummary(c.Request.Context(), *parsed.Payload, parsed.Mode == "replace")
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, configImportPreview{Encrypted: parsed.Bundle.Encrypted, ExportedAt: parsed.Bundle.ExportedAt, Mode: parsed.Mode, Summary: summary})
}

func (a *APIServer) importConfiguration(c *gin.Context) {
	parsed, ok := a.parseConfigurationImport(c)
	if !ok {
		return
	}
	payload, mode := parsed.Payload, parsed.Mode
	summary, err := a.buildConfigImportSummary(c.Request.Context(), *payload, mode == "replace")
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	adminHash := payload.AdminKeyHash
	config := payload.Runtime
	_, err = a.runtime.Import(c.Request.Context(), config, func(ctx context.Context, domains map[string]json.RawMessage) (map[string]int, error) {
		result, err := a.store.ImportConfiguration(ctx, store.ConfigurationImport{Runtime: domains, Monitors: payload.Monitors, NotificationChannels: payload.NotificationChannels, StatusBoardItems: payload.StatusBoardItems, StatusBoardShares: configBundleSharesToDomain(payload.StatusBoardShares), Replace: mode == "replace", AdminKeyHash: adminHash})
		if err != nil {
			return nil, err
		}
		return result.Versions, nil
	})
	if err != nil {
		writeError(c.Writer, http.StatusBadRequest, "import_failed", err.Error())
		return
	}
	if a.statusBoard != nil {
		a.statusBoard.Publish(statusboard.StreamEvent{Type: "configuration_imported"})
	}
	writeJSON(c.Writer, http.StatusOK, configImportResult{Imported: true, AdminKeyImported: adminHash != "", Monitors: len(payload.Monitors), NotificationChannels: len(payload.NotificationChannels), StatusBoardItems: len(payload.StatusBoardItems), StatusBoardShares: len(payload.StatusBoardShares), RuntimeTypes: 5, Summary: summary})
}

func (a *APIServer) parseConfigurationImport(c *gin.Context) (*parsedConfigImport, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxConfigUploadBytes+(1<<20))
	mode := c.PostForm("mode")
	if mode != "merge" && mode != "replace" {
		writeError(c.Writer, http.StatusBadRequest, "invalid_mode", "导入模式必须为 merge 或 replace")
		return nil, false
	}
	key := c.PostForm("encryption_key")
	header, err := c.FormFile("file")
	if err != nil {
		writeError(c.Writer, http.StatusBadRequest, "file_required", "请选择一个 ZIP 配置文件")
		return nil, false
	}
	if header.Size > maxConfigUploadBytes {
		writeError(c.Writer, http.StatusRequestEntityTooLarge, "file_too_large", "配置文件不能超过 10 MB")
		return nil, false
	}
	file, err := header.Open()
	if err != nil {
		writeError(c.Writer, http.StatusBadRequest, "file_unreadable", err.Error())
		return nil, false
	}
	defer file.Close()
	archive, err := io.ReadAll(io.LimitReader(file, maxConfigUploadBytes+1))
	if err != nil || len(archive) > maxConfigUploadBytes {
		writeError(c.Writer, http.StatusRequestEntityTooLarge, "file_too_large", "配置文件不能超过 10 MB")
		return nil, false
	}
	bundle, err := readConfigBundle(archive)
	if err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_bundle", err.Error())
		return nil, false
	}
	payload, err := decodeConfigBundle(bundle, key)
	if err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_bundle", err.Error())
		return nil, false
	}
	adminHash := payload.AdminKeyHash
	if adminHash != "" {
		if err := auth.ValidateAccessKeyHash(adminHash); err != nil {
			writeError(c.Writer, http.StatusBadRequest, "invalid_admin_key", "配置包中的管理员密钥无效")
			return nil, false
		}
	}
	if err := a.validateConfigBundle(c, *payload, mode == "replace"); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_bundle", err.Error())
		return nil, false
	}
	return &parsedConfigImport{Bundle: bundle, Payload: payload, Mode: mode}, true
}

func (a *APIServer) buildConfigImportSummary(ctx context.Context, payload configBundleData, replace bool) (configTransferSummary, error) {
	existingMonitors, err := a.store.ListMonitors(ctx)
	if err != nil {
		return configTransferSummary{}, err
	}
	existingChannels, err := a.store.ListChannels(ctx)
	if err != nil {
		return configTransferSummary{}, err
	}
	existingStatusItems, err := a.store.ListStatusBoardItems(ctx)
	if err != nil {
		return configTransferSummary{}, err
	}
	existingShares, err := a.store.ListStatusBoardShares(ctx)
	if err != nil {
		return configTransferSummary{}, err
	}

	monitorItems := make([]configTransferItem, 0, len(payload.Monitors))
	for _, monitor := range payload.Monitors {
		monitorItems = append(monitorItems, configTransferItem{ID: monitor.ID, Name: monitor.Name})
	}
	existingMonitorItems := make([]configTransferItem, 0, len(existingMonitors))
	for _, monitor := range existingMonitors {
		existingMonitorItems = append(existingMonitorItems, configTransferItem{ID: monitor.ID, Name: monitor.Name})
	}
	channelItems := make([]configTransferItem, 0, len(payload.NotificationChannels))
	for _, channel := range payload.NotificationChannels {
		if channel.BuiltIn || channel.ID == core.BuiltInNotificationChannelID {
			continue
		}
		channelItems = append(channelItems, configTransferItem{ID: channel.ID, Name: channel.Name})
	}
	existingChannelItems := make([]configTransferItem, 0, len(existingChannels))
	for _, channel := range existingChannels {
		if channel.BuiltIn || channel.ID == core.BuiltInNotificationChannelID {
			continue
		}
		existingChannelItems = append(existingChannelItems, configTransferItem{ID: channel.ID, Name: channel.Name})
	}
	statusItems := make([]configTransferItem, 0, len(payload.StatusBoardItems))
	for _, item := range payload.StatusBoardItems {
		statusItems = append(statusItems, configTransferItem{ID: item.ID, Name: item.Name})
	}
	existingStatusBoardItems := make([]configTransferItem, 0, len(existingStatusItems))
	for _, item := range existingStatusItems {
		existingStatusBoardItems = append(existingStatusBoardItems, configTransferItem{ID: item.ID, Name: item.Name})
	}
	existingShareItems := make([]configTransferItem, 0, len(existingShares))
	for _, share := range existingShares {
		existingShareItems = append(existingShareItems, configTransferItem{ID: share.ID, Name: share.Name})
	}
	shareItems := make([]configTransferItem, 0, len(payload.StatusBoardShares))
	for _, share := range payload.StatusBoardShares {
		shareItems = append(shareItems, configTransferItem{ID: share.ID, Name: share.Name})
	}

	summary := configTransferSummary{
		Runtime: configTransferChanges{Overwritten: []configTransferItem{
			{ID: "storage", Name: "存储策略"},
			{ID: "scheduler", Name: "调度器"},
			{ID: "logging", Name: "日志运行参数"},
			{ID: "plugins", Name: "插件日志"},
			{ID: "auth", Name: "认证策略"},
		}},
		Monitors:             compareConfigTransferItems(monitorItems, existingMonitorItems, replace),
		NotificationChannels: compareConfigTransferItems(channelItems, existingChannelItems, replace),
		StatusBoardItems:     compareConfigTransferItems(statusItems, existingStatusBoardItems, replace),
		StatusBoardShares:    compareConfigTransferItems(shareItems, existingShareItems, replace),
	}
	if payload.AdminKeyHash != "" {
		summary.AdminKey.Overwritten = []configTransferItem{{ID: "administrator_access_key", Name: "管理员密钥"}}
	}
	return summary, nil
}

func compareConfigTransferItems(incoming, existing []configTransferItem, replace bool) configTransferChanges {
	changes := configTransferChanges{Added: []configTransferItem{}, Overwritten: []configTransferItem{}, Deleted: []configTransferItem{}}
	existingByID := make(map[string]configTransferItem, len(existing))
	for _, item := range existing {
		existingByID[item.ID] = item
	}
	incomingIDs := make(map[string]struct{}, len(incoming))
	for _, item := range incoming {
		incomingIDs[item.ID] = struct{}{}
		if _, found := existingByID[item.ID]; found {
			changes.Overwritten = append(changes.Overwritten, item)
		} else {
			changes.Added = append(changes.Added, item)
		}
	}
	if replace {
		for _, item := range existing {
			if _, found := incomingIDs[item.ID]; !found {
				changes.Deleted = append(changes.Deleted, item)
			}
		}
	}
	sortConfigTransferItems(changes.Added)
	sortConfigTransferItems(changes.Overwritten)
	sortConfigTransferItems(changes.Deleted)
	return changes
}

func sortConfigTransferItems(items []configTransferItem) {
	sort.Slice(items, func(left, right int) bool {
		if items[left].Name == items[right].Name {
			return items[left].ID < items[right].ID
		}
		return items[left].Name < items[right].Name
	})
}

func configBundleSharesFromDomain(values []core.StatusBoardShare) []configBundleShare {
	result := make([]configBundleShare, 0, len(values))
	for _, share := range values {
		result = append(result, configBundleShare{ID: share.ID, Token: share.Token, Name: share.Name, MonitorIDs: share.MonitorIDs, ItemIDs: share.ItemIDs, Active: share.Active, CreatedAt: share.CreatedAt})
	}
	return result
}

func configBundleSharesToDomain(values []configBundleShare) []core.StatusBoardShare {
	result := make([]core.StatusBoardShare, 0, len(values))
	for _, share := range values {
		result = append(result, core.StatusBoardShare{ID: share.ID, Token: share.Token, Name: share.Name, MonitorIDs: share.MonitorIDs, ItemIDs: share.ItemIDs, Active: share.Active, CreatedAt: share.CreatedAt})
	}
	return result
}

func (a *APIServer) validateConfigBundle(c *gin.Context, bundle configBundleData, replace bool) error {
	now := time.Now().UTC()
	monitors := make(map[string]struct{}, len(bundle.Monitors))
	for index := range bundle.Monitors {
		monitor := &bundle.Monitors[index]
		if monitor.ID == "" || monitor.Name == "" || monitor.ModuleType == "" {
			return errors.New("监控项缺少必填字段")
		}
		if _, exists := monitors[monitor.ID]; exists {
			return fmt.Errorf("监控项 ID 重复：%s", monitor.ID)
		}
		monitors[monitor.ID] = struct{}{}
		if !json.Valid(monitor.ModuleConfig) || !json.Valid(monitor.ConditionConfig) {
			return fmt.Errorf("监控项 %q 的 JSON 配置无效", monitor.ID)
		}
		var conditions core.ConditionConfig
		if err := json.Unmarshal(monitor.ConditionConfig, &conditions); err != nil {
			return fmt.Errorf("监控项 %q 的触发条件无效", monitor.ID)
		}
		if conditions.NotificationPolicy != "" && conditions.NotificationPolicy != core.NotificationPolicyOnce && conditions.NotificationPolicy != core.NotificationPolicyEvery {
			return fmt.Errorf("监控项 %q 的通知策略无效", monitor.ID)
		}
		if err := runtimeapp.ValidateSchedules(monitor.Schedules, bundle.Runtime.Scheduler.Timezone); err != nil {
			return fmt.Errorf("监控项 %q 的调度配置无效：%w", monitor.ID, err)
		}
		if a.modules != nil {
			if module, ok := a.modules.Get(monitor.ModuleType); ok {
				if err := module.ValidateConfig(monitor.ModuleConfig); err != nil {
					return fmt.Errorf("监控项 %q 配置无效：%w", monitor.ID, err)
				}
			}
		}
		monitor.RuntimeState = json.RawMessage(`{}`)
		if monitor.CreatedAt.IsZero() {
			monitor.CreatedAt = now
		}
		if monitor.UpdatedAt.IsZero() {
			monitor.UpdatedAt = now
		}
	}
	channels := map[string]struct{}{core.BuiltInNotificationChannelID: {}}
	for index := range bundle.NotificationChannels {
		channel := &bundle.NotificationChannels[index]
		if channel.ID == "" || channel.Name == "" || channel.NotifierType == "" {
			return errors.New("通知渠道缺少必填字段")
		}
		if _, exists := channels[channel.ID]; exists {
			return fmt.Errorf("通知渠道 ID 重复：%s", channel.ID)
		}
		channels[channel.ID] = struct{}{}
		if !json.Valid(channel.Config) {
			return fmt.Errorf("通知渠道 %q 的 JSON 配置无效", channel.ID)
		}
		if a.notifiers != nil {
			if notifier, ok := a.notifiers.Get(channel.NotifierType); ok {
				if err := notifier.ValidateConfig(channel.Config); err != nil {
					return fmt.Errorf("通知渠道 %q 配置无效：%w", channel.ID, err)
				}
			}
		}
		if channel.CreatedAt.IsZero() {
			channel.CreatedAt = now
		}
		if channel.UpdatedAt.IsZero() {
			channel.UpdatedAt = now
		}
	}
	items := make(map[string]struct{}, len(bundle.StatusBoardItems))
	for index := range bundle.StatusBoardItems {
		item := &bundle.StatusBoardItems[index]
		if item.ID == "" || item.Name == "" || item.MonitorID == "" {
			return errors.New("状态看板项缺少必填字段")
		}
		if _, exists := items[item.ID]; exists {
			return fmt.Errorf("状态看板项 ID 重复：%s", item.ID)
		}
		items[item.ID] = struct{}{}
		if item.HistoryLimit < 20 || item.HistoryLimit > 200 {
			return fmt.Errorf("状态看板项 %q 的展示执行次数无效", item.ID)
		}
		item.RuntimeState = core.StatusItemRuntimeState{Rules: map[string]core.TrendRuleState{}}
		if item.CreatedAt.IsZero() {
			item.CreatedAt = now
		}
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = now
		}
	}
	existingMonitors, err := a.store.ListMonitors(c.Request.Context())
	if err != nil {
		return err
	}
	existingChannels, err := a.store.ListChannels(c.Request.Context())
	if err != nil {
		return err
	}
	existingItems, err := a.store.ListStatusBoardItems(c.Request.Context())
	if err != nil {
		return err
	}
	existingShares, err := a.store.ListStatusBoardShares(c.Request.Context())
	if err != nil {
		return err
	}
	if replace {
		existingMonitors, existingChannels, existingItems, existingShares = nil, []core.NotificationChannel{{ID: core.BuiltInNotificationChannelID}}, nil, nil
	}
	for _, monitor := range existingMonitors {
		monitors[monitor.ID] = struct{}{}
	}
	for _, channel := range existingChannels {
		channels[channel.ID] = struct{}{}
	}
	for _, item := range existingItems {
		items[item.ID] = struct{}{}
	}
	for _, monitor := range bundle.Monitors {
		for _, channelID := range monitor.NotificationChannelIDs {
			if _, ok := channels[channelID]; !ok {
				return fmt.Errorf("监控项 %q 引用了不存在的通知渠道 %q", monitor.ID, channelID)
			}
		}
	}
	for _, item := range bundle.StatusBoardItems {
		if _, ok := monitors[item.MonitorID]; !ok {
			return fmt.Errorf("看板项 %q 引用了不存在的监控项 %q", item.ID, item.MonitorID)
		}
		for _, channelID := range item.NotificationChannelIDs {
			if _, ok := channels[channelID]; !ok {
				return fmt.Errorf("看板项 %q 引用了不存在的通知渠道 %q", item.ID, channelID)
			}
		}
	}
	shareIDs := make(map[string]struct{}, len(bundle.StatusBoardShares))
	tokens := make(map[string]string, len(existingShares)+len(bundle.StatusBoardShares))
	for _, share := range existingShares {
		tokens[share.Token] = share.ID
	}
	for index := range bundle.StatusBoardShares {
		share := &bundle.StatusBoardShares[index]
		if share.ID == "" || strings.TrimSpace(share.Name) == "" || share.Token == "" {
			return errors.New("共享链接缺少必填字段")
		}
		if _, exists := shareIDs[share.ID]; exists {
			return fmt.Errorf("共享链接 ID 重复：%s", share.ID)
		}
		shareIDs[share.ID] = struct{}{}
		decodedToken, err := base64.RawURLEncoding.DecodeString(share.Token)
		if err != nil || len(decodedToken) != 32 {
			return fmt.Errorf("共享链接 %q 的令牌无效", share.ID)
		}
		if ownerID, exists := tokens[share.Token]; exists && ownerID != share.ID {
			return fmt.Errorf("共享链接 %q 的令牌与其他链接重复", share.ID)
		}
		tokens[share.Token] = share.ID
		if len(share.MonitorIDs) == 0 && len(share.ItemIDs) == 0 {
			return fmt.Errorf("共享链接 %q 未选择任何看板内容", share.ID)
		}
		selectedMonitors := make(map[string]struct{}, len(share.MonitorIDs))
		for _, monitorID := range share.MonitorIDs {
			if _, duplicate := selectedMonitors[monitorID]; duplicate {
				return fmt.Errorf("共享链接 %q 包含重复的监控分组", share.ID)
			}
			selectedMonitors[monitorID] = struct{}{}
			if _, exists := monitors[monitorID]; !exists {
				return fmt.Errorf("共享链接 %q 引用了不存在的监控项 %q", share.ID, monitorID)
			}
		}
		selectedItems := make(map[string]struct{}, len(share.ItemIDs))
		for _, itemID := range share.ItemIDs {
			if _, duplicate := selectedItems[itemID]; duplicate {
				return fmt.Errorf("共享链接 %q 包含重复的看板项", share.ID)
			}
			selectedItems[itemID] = struct{}{}
			if _, exists := items[itemID]; !exists {
				return fmt.Errorf("共享链接 %q 引用了不存在的看板项 %q", share.ID, itemID)
			}
		}
		if share.CreatedAt.IsZero() {
			share.CreatedAt = now
		}
	}
	return nil
}

func readConfigBundle(data []byte) (configBundle, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return configBundle{}, errors.New("文件不是有效的 ZIP 压缩包")
	}
	if len(archive.File) == 0 || len(archive.File) > maxConfigBundleEntries {
		return configBundle{}, errors.New("ZIP 内容数量无效")
	}
	var payload []byte
	for _, file := range archive.File {
		if file.Name != configBundleFilename || file.FileInfo().IsDir() {
			continue
		}
		if file.UncompressedSize64 > maxConfigJSONBytes {
			return configBundle{}, errors.New("配置内容超过大小限制")
		}
		reader, err := file.Open()
		if err != nil {
			return configBundle{}, err
		}
		payload, err = io.ReadAll(io.LimitReader(reader, maxConfigJSONBytes+1))
		_ = reader.Close()
		if err != nil {
			return configBundle{}, err
		}
		if len(payload) > maxConfigJSONBytes {
			return configBundle{}, errors.New("配置内容超过大小限制")
		}
	}
	if len(payload) == 0 {
		return configBundle{}, errors.New("ZIP 中缺少 meerkit-config.json")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var bundle configBundle
	if err := decoder.Decode(&bundle); err != nil {
		return configBundle{}, errors.New("配置清单 JSON 无效")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return configBundle{}, errors.New("配置清单包含额外内容")
	}
	if bundle.Format != configBundleFormat || bundle.Version != configBundleVersion {
		return configBundle{}, errors.New("不支持的配置包版本")
	}
	if bundle.Encrypted {
		if bundle.Protected == nil || bundle.Data != nil {
			return configBundle{}, errors.New("加密配置包载荷无效")
		}
	} else if bundle.Data == nil || bundle.Protected != nil {
		return configBundle{}, errors.New("未加密配置包载荷无效")
	}
	return bundle, nil
}

func decodeConfigBundle(bundle configBundle, password string) (*configBundleData, error) {
	var payload configBundleData
	if bundle.Encrypted {
		if strings.TrimSpace(password) == "" {
			return nil, errors.New("该配置包已加密，请输入配置包密码")
		}
		plain, err := decryptPayload(*bundle.Protected, password)
		if err != nil {
			return nil, errors.New("配置包密码错误或文件已损坏")
		}
		if err := decodeStrictJSON(plain, &payload); err != nil {
			return nil, errors.New("加密配置内容无效")
		}
	} else {
		payload = *bundle.Data
	}
	contents := make(map[string]bool, len(payload.Contents))
	for _, value := range payload.Contents {
		if value != "runtime" && value != "monitors" && value != "notification_channels" && value != "status_board_items" && value != "status_board_shares" && value != "admin_key" {
			return nil, fmt.Errorf("配置包包含未知内容：%s", value)
		}
		if contents[value] {
			return nil, errors.New("配置包内容重复")
		}
		contents[value] = true
	}
	if !contents["runtime"] {
		return nil, errors.New("配置包必须包含动态配置")
	}
	if contents["admin_key"] != (payload.AdminKeyHash != "") || (len(payload.Monitors) > 0 && !contents["monitors"]) || (len(payload.NotificationChannels) > 0 && !contents["notification_channels"]) || (len(payload.StatusBoardItems) > 0 && !contents["status_board_items"]) || (len(payload.StatusBoardShares) > 0 && !contents["status_board_shares"]) {
		return nil, errors.New("配置内容与清单不一致")
	}
	if err := payload.Runtime.Validate(); err != nil {
		return nil, fmt.Errorf("动态配置无效：%w", err)
	}
	return &payload, nil
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("contains multiple JSON values")
		}
		return err
	}
	return nil
}

func encryptPayload(plain []byte, password string) (encryptedPayload, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return encryptedPayload{}, err
	}
	key, err := scrypt.Key([]byte(password), salt, 32768, 8, 1, configEncryptionKeyBytes)
	if err != nil {
		return encryptedPayload{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return encryptedPayload{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return encryptedPayload{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return encryptedPayload{}, err
	}
	ciphertext := gcm.Seal(nil, nonce, plain, []byte(configBundleFormat))
	return encryptedPayload{Algorithm: "AES-256-GCM-SCRYPT", Salt: base64.RawStdEncoding.EncodeToString(salt), Nonce: base64.RawStdEncoding.EncodeToString(nonce), Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext)}, nil
}

func decryptPayload(value encryptedPayload, password string) ([]byte, error) {
	if value.Algorithm != "AES-256-GCM-SCRYPT" {
		return nil, errors.New("unsupported encryption algorithm")
	}
	salt, err := base64.RawStdEncoding.DecodeString(value.Salt)
	if err != nil || len(salt) < 8 {
		return nil, errors.New("invalid salt")
	}
	nonce, err := base64.RawStdEncoding.DecodeString(value.Nonce)
	if err != nil {
		return nil, errors.New("invalid nonce")
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(value.Ciphertext)
	if err != nil {
		return nil, errors.New("invalid ciphertext")
	}
	key, err := scrypt.Key([]byte(password), salt, 32768, 8, 1, configEncryptionKeyBytes)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid nonce")
	}
	return gcm.Open(nil, nonce, ciphertext, []byte(configBundleFormat))
}
