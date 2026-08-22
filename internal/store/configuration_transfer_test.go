package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"meerkit/internal/app"
	"meerkit/internal/core"
)

func TestImportConfigurationMergeAndReplace(t *testing.T) {
	ctx := context.Background()
	database, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ensureRuntimeRows(t, database)
	originalHash, _ := bcrypt.GenerateFromPassword([]byte("original-admin-key"), bcrypt.MinCost)
	if err := database.SetAdminKeyHash(ctx, string(originalHash), false); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	oldChannel := core.NotificationChannel{ID: "channel-old", Name: "Old", NotifierType: "webhook", Enabled: true, Config: json.RawMessage(`{"url":"https://old.example"}`), CreatedAt: now, UpdatedAt: now}
	if err := database.CreateChannel(ctx, oldChannel); err != nil {
		t.Fatal(err)
	}
	oldMonitor := transferTestMonitor("monitor-old", "Old monitor", oldChannel.ID, now)
	if err := database.CreateMonitor(ctx, oldMonitor); err != nil {
		t.Fatal(err)
	}
	share := core.StatusBoardShare{ID: "share-old", Token: "share-token", Name: "Old share", MonitorIDs: []string{oldMonitor.ID}, Active: true, CreatedAt: now}
	if err := database.CreateStatusBoardShare(ctx, share); err != nil {
		t.Fatal(err)
	}

	updatedChannel := oldChannel
	updatedChannel.Name = "Updated"
	newMonitor := transferTestMonitor("monitor-new", "New monitor", oldChannel.ID, now)
	newShare := core.StatusBoardShare{ID: "share-new", Token: "new-share-token", Name: "New share", MonitorIDs: []string{newMonitor.ID}, Active: true, CreatedAt: now}
	result, err := database.ImportConfiguration(ctx, ConfigurationImport{Runtime: runtimeDomains(t, app.DefaultRuntimeConfig()), NotificationChannels: []core.NotificationChannel{updatedChannel}, Monitors: []core.Monitor{newMonitor}, StatusBoardShares: []core.StatusBoardShare{newShare}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Versions) != 6 {
		t.Fatalf("runtime versions = %v", result.Versions)
	}
	monitors, err := database.ListMonitors(ctx)
	if err != nil || len(monitors) != 2 {
		t.Fatalf("merged monitors = %d, err=%v", len(monitors), err)
	}
	channel, err := database.GetChannel(ctx, oldChannel.ID)
	if err != nil || channel.Name != "Updated" {
		t.Fatalf("merged channel = %+v, err=%v", channel, err)
	}
	if got, err := database.AdminKeyHash(ctx); err != nil || got != string(originalHash) {
		t.Fatalf("admin hash changed during merge: %v", err)
	}
	if shares, err := database.ListStatusBoardShares(ctx); err != nil || len(shares) != 2 {
		t.Fatalf("merge did not preserve and add status board shares: %+v, err=%v", shares, err)
	}

	newHash, _ := bcrypt.GenerateFromPassword([]byte("replacement-admin-key"), bcrypt.MinCost)
	session := AdminSession{TokenHash: "session", CSRFToken: "csrf", ExpiresAt: now.Add(time.Hour), LastSeenAt: now, CreatedAt: now}
	if err := database.CreateAdminSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	replacementChannel := core.NotificationChannel{ID: "channel-replacement", Name: "Replacement", NotifierType: "smtp", Enabled: false, Config: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
	replacementMonitor := transferTestMonitor("monitor-replacement", "Replacement monitor", replacementChannel.ID, now)
	replacementItem := core.StatusBoardItem{ID: "item-replacement", Name: "Replacement item", MonitorID: replacementMonitor.ID, Enabled: true, Source: core.StatusItemSource{Kind: core.StatusSourceConditionOverall, ValueType: core.StatusValueBoolean}, HistoryLimit: 60, RuntimeState: core.StatusItemRuntimeState{Rules: map[string]core.TrendRuleState{}}, CreatedAt: now, UpdatedAt: now}
	replacementShare := core.StatusBoardShare{ID: "share-replacement", Token: "replacement-share-token", Name: "Replacement share", ItemIDs: []string{replacementItem.ID}, Active: false, CreatedAt: now}
	_, err = database.ImportConfiguration(ctx, ConfigurationImport{Runtime: runtimeDomains(t, app.DefaultRuntimeConfig()), NotificationChannels: []core.NotificationChannel{replacementChannel}, Monitors: []core.Monitor{replacementMonitor}, StatusBoardItems: []core.StatusBoardItem{replacementItem}, StatusBoardShares: []core.StatusBoardShare{replacementShare}, Replace: true, AdminKeyHash: string(newHash)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.GetMonitor(ctx, oldMonitor.ID); !IsNoRows(err) {
		t.Fatalf("old monitor still exists: %v", err)
	}
	if _, err := database.GetChannel(ctx, oldChannel.ID); !IsNoRows(err) {
		t.Fatalf("old channel still exists: %v", err)
	}
	if _, err := database.GetChannel(ctx, core.BuiltInNotificationChannelID); err != nil {
		t.Fatalf("built-in channel was removed: %v", err)
	}
	if _, err := database.GetStatusBoardItem(ctx, replacementItem.ID); err != nil {
		t.Fatalf("replacement board item missing: %v", err)
	}
	if shares, err := database.ListStatusBoardShares(ctx); err != nil || len(shares) != 1 || shares[0].ID != replacementShare.ID || shares[0].Token != replacementShare.Token || shares[0].Active {
		t.Fatalf("replace did not install status board shares: %+v, err=%v", shares, err)
	}
	if got, err := database.AdminKeyHash(ctx); err != nil || got != string(newHash) {
		t.Fatalf("admin hash was not replaced: %v", err)
	}
	if _, err := database.GetAdminSession(ctx, session.TokenHash); !IsNoRows(err) {
		t.Fatalf("admin session was not revoked: %v", err)
	}
}

func ensureRuntimeRows(t *testing.T, database *Store) {
	t.Helper()
	config := app.DefaultRuntimeConfig()
	for configType, value := range map[string]any{app.SystemConfigStorage: config.Storage, app.SystemConfigScheduler: config.Scheduler, app.SystemConfigLogging: config.Logging, app.SystemConfigPlugins: config.Plugins, app.SystemConfigAuth: config.Auth, app.SystemConfigMCP: config.MCP} {
		if _, err := database.EnsureSystemConfig(context.Background(), configType, value); err != nil {
			t.Fatal(err)
		}
	}
}

func runtimeDomains(t *testing.T, config app.RuntimeConfig) map[string]json.RawMessage {
	t.Helper()
	result := map[string]json.RawMessage{}
	for configType, value := range map[string]any{app.SystemConfigStorage: config.Storage, app.SystemConfigScheduler: config.Scheduler, app.SystemConfigLogging: config.Logging, app.SystemConfigPlugins: config.Plugins, app.SystemConfigAuth: config.Auth, app.SystemConfigMCP: config.MCP} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		result[configType] = data
	}
	return result
}

func transferTestMonitor(id, name, channelID string, now time.Time) core.Monitor {
	return core.Monitor{ID: id, Name: name, ModuleType: "test", ModuleVersion: "1", ModuleConfigVersion: "1", Schedules: []string{"@hourly"}, Enabled: true, ModuleConfig: json.RawMessage(`{}`), ConditionConfig: json.RawMessage(`{"logic":"ALL","rules":[]}`), NotificationChannelIDs: []string{channelID}, RuntimeState: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
}
