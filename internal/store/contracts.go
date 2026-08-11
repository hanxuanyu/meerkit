package store

import (
	"context"
	"encoding/json"
	"time"

	"meerkit/internal/core"
)

type MonitorRepository interface {
	CreateMonitor(context.Context, core.Monitor) error
	UpdateMonitor(context.Context, core.Monitor) error
	UpdateRuntimeState(context.Context, string, core.RuntimeState) error
	DeleteMonitor(context.Context, string) error
	GetMonitor(context.Context, string) (core.Monitor, error)
	ListMonitors(context.Context) ([]core.Monitor, error)
	ListMonitorsPage(context.Context, MonitorListOptions) (PageResult[core.Monitor], error)
}

type RecordRepository interface {
	AddRecord(context.Context, core.MonitorRecord) error
	UpdateRecordNotificationEvents(context.Context, string, []core.RecordNotificationEvent) error
	ListRecords(context.Context, string, int) ([]core.MonitorRecord, error)
	GetRecord(context.Context, string, string) (core.MonitorRecord, error)
	ListRecordsPage(context.Context, string, RecordListOptions) (PageResult[core.MonitorRecord], error)
	LatestSuccessfulRecord(context.Context, string) (core.MonitorRecord, error)
	DeleteMonitorRecords(context.Context, string) (int64, error)
	PruneRecords(context.Context, time.Time) (int64, error)
}

type ChannelRepository interface {
	CreateChannel(context.Context, core.NotificationChannel) error
	UpdateChannel(context.Context, core.NotificationChannel) error
	DeleteChannel(context.Context, string) error
	GetChannel(context.Context, string) (core.NotificationChannel, error)
	ListChannels(context.Context) ([]core.NotificationChannel, error)
}

type NotificationRepository interface {
	CreateNotificationDelivery(context.Context, core.NotificationDeliveryRecord) error
	UpdateNotificationDeliveryContent(context.Context, string, string, string, string, json.RawMessage) error
	UpdateNotificationDeliveryNotifier(context.Context, string, string, string) error
	UpdateNotificationDeliveryResult(context.Context, string, string, core.NotificationDelivery) error
	GetInAppNotification(context.Context, string) (core.InAppNotification, error)
	GetInAppNotificationByEvent(context.Context, string, string) (core.InAppNotification, error)
	ListInAppNotificationsPage(context.Context, NotificationListOptions) (PageResult[core.InAppNotification], error)
	CountUnreadInAppNotifications(context.Context) (int, error)
	MarkInAppNotificationRead(context.Context, string) (core.InAppNotification, error)
	MarkAllInAppNotificationsRead(context.Context) (int64, error)
	DeleteReadInAppNotifications(context.Context) (int64, error)
	PruneInAppNotifications(context.Context, time.Time) (int64, error)
	PruneNotificationDeliveries(context.Context, time.Time) (int64, error)
}

type StatusBoardRepository interface {
	CreateStatusBoardItem(context.Context, core.StatusBoardItem) error
	UpdateStatusBoardItem(context.Context, core.StatusBoardItem) error
	GetStatusBoardItem(context.Context, string) (core.StatusBoardItem, error)
	ListStatusBoardItems(context.Context) ([]core.StatusBoardItem, error)
	ListStatusBoardItemsByMonitor(context.Context, string) ([]core.StatusBoardItem, error)
	DeleteStatusBoardItem(context.Context, string) error
	ResetStatusBoardRuntimeByMonitor(context.Context, string, time.Time) error
	CommitMonitorExecution(context.Context, core.MonitorRecord, string, core.RuntimeState, map[string]core.StatusItemRuntimeState) error
}

type PluginRepository interface {
	UpsertPlugin(context.Context, core.PluginInstallation) error
	ListPlugins(context.Context) ([]core.PluginInstallation, error)
	GetPlugin(context.Context, string, string) (core.PluginInstallation, error)
	TrustPluginSigner(context.Context, core.TrustedPluginSigner) error
	GetTrustedPluginSigner(context.Context, string) (core.TrustedPluginSigner, error)
	DeletePlugin(context.Context, string, string) error
	DisableOtherPluginVersions(context.Context, string, string) error
	CountPlugins(context.Context) (int, error)
	DeleteDevelopmentPlugins(context.Context) error
	CountMonitorReferences(context.Context, []string) (int, error)
	SaveDescriptorSnapshot(context.Context, core.ModuleDescriptor) error
	GetDescriptorSnapshot(context.Context, string, string) (core.ModuleDescriptor, error)
	MigrateMonitorConfigs(context.Context, map[string]MonitorMigrationTarget, MonitorConfigMigrator) error
}

type AuthRepository interface {
	AdminKeyHash(context.Context) (string, error)
	SetAdminKeyHash(context.Context, string, bool) error
	CreateAdminSession(context.Context, AdminSession) error
	GetAdminSession(context.Context, string) (AdminSession, error)
	RefreshAdminSession(context.Context, string, time.Time, time.Time) error
	DeleteAdminSession(context.Context, string) error
	DeleteExpiredAdminSessions(context.Context, time.Time) error
}

type SystemConfigRepository interface {
	GetSystemConfig(context.Context, string) (SystemConfig, error)
	ListSystemConfigs(context.Context) ([]SystemConfig, error)
	EnsureSystemConfig(context.Context, string, any) (SystemConfig, error)
	UpdateSystemConfig(context.Context, string, json.RawMessage, int) (SystemConfig, error)
}

type ConfigurationImport struct {
	Runtime              map[string]json.RawMessage
	Monitors             []core.Monitor
	NotificationChannels []core.NotificationChannel
	StatusBoardItems     []core.StatusBoardItem
	Replace              bool
	AdminKeyHash         string
}

type ConfigurationImportResult struct {
	Versions map[string]int
}

type ConfigurationTransferRepository interface {
	ImportConfiguration(context.Context, ConfigurationImport) (ConfigurationImportResult, error)
	AdminKeyHash(context.Context) (string, error)
}

type APIRepository interface {
	MonitorRepository
	RecordRepository
	ChannelRepository
	NotificationRepository
	StatusBoardRepository
	ConfigurationTransferRepository
	GetDescriptorSnapshot(context.Context, string, string) (core.ModuleDescriptor, error)
	Ping(context.Context) error
}

type RunnerRepository interface {
	GetMonitor(context.Context, string) (core.Monitor, error)
	LatestSuccessfulRecord(context.Context, string) (core.MonitorRecord, error)
	CommitMonitorExecution(context.Context, core.MonitorRecord, string, core.RuntimeState, map[string]core.StatusItemRuntimeState) error
	UpdateRecordNotificationEvents(context.Context, string, []core.RecordNotificationEvent) error
	GetChannel(context.Context, string) (core.NotificationChannel, error)
	CreateNotificationDelivery(context.Context, core.NotificationDeliveryRecord) error
	UpdateNotificationDeliveryNotifier(context.Context, string, string, string) error
}

type CleanupRepository interface {
	PruneRecords(context.Context, time.Time) (int64, error)
	PruneNotificationDeliveries(context.Context, time.Time) (int64, error)
	CountUnreadInAppNotifications(context.Context) (int, error)
}

type StatusBoardServiceRepository interface {
	GetMonitor(context.Context, string) (core.Monitor, error)
	ListMonitors(context.Context) ([]core.Monitor, error)
	ListRecords(context.Context, string, int) ([]core.MonitorRecord, error)
	ListStatusBoardItems(context.Context) ([]core.StatusBoardItem, error)
	ListStatusBoardItemsByMonitor(context.Context, string) ([]core.StatusBoardItem, error)
	GetChannel(context.Context, string) (core.NotificationChannel, error)
	GetDescriptorSnapshot(context.Context, string, string) (core.ModuleDescriptor, error)
}

type Repository interface {
	MonitorRepository
	RecordRepository
	ChannelRepository
	NotificationRepository
	StatusBoardRepository
	PluginRepository
	AuthRepository
	SystemConfigRepository
	Ping(context.Context) error
}

var _ Repository = (*Store)(nil)
