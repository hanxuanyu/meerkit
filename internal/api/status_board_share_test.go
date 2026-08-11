package api

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"meerkit/internal/core"
)

func TestNormalizeStatusBoardShareSelectionSupportsGroupsAndItems(t *testing.T) {
	snapshot := shareTestSnapshot()
	monitors, items, err := normalizeStatusBoardShareSelection(snapshot, []string{"monitor-1", "monitor-1"}, []string{"item-1", "item-2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(monitors) != 1 || monitors[0] != "monitor-1" || len(items) != 1 || items[0] != "item-2" {
		t.Fatalf("unexpected normalized selection: monitors=%v items=%v", monitors, items)
	}
	if _, _, err := normalizeStatusBoardShareSelection(snapshot, nil, nil); err == nil {
		t.Fatal("empty share selection was accepted")
	}
	if _, _, err := normalizeStatusBoardShareSelection(snapshot, []string{"missing"}, nil); err == nil {
		t.Fatal("missing monitor selection was accepted")
	}
}

func TestPublicStatusBoardSnapshotDoesNotExposeMonitorConfiguration(t *testing.T) {
	snapshot := shareTestSnapshot()
	response := filterPublicStatusBoardSnapshot(snapshot, core.StatusBoardShare{Name: "Public", MonitorIDs: []string{"monitor-1"}})
	if len(response.Groups) != 1 || len(response.Groups[0].Items) != 1 || response.Groups[0].Items[0].ActiveTrendRules != 1 {
		t.Fatalf("unexpected public snapshot: %+v", response)
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range [][]byte{[]byte("module-secret"), []byte("condition-secret"), []byte("channel-secret"), []byte("item-2")} {
		if bytes.Contains(data, secret) {
			t.Fatalf("public snapshot exposed %q: %s", secret, data)
		}
	}
}

func TestStatusBoardShareTokensAreRandom(t *testing.T) {
	first, err := randomStatusBoardShareToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := randomStatusBoardShareToken()
	if err != nil {
		t.Fatal(err)
	}
	if first == second || len(first) < 40 {
		t.Fatal("share token generation is invalid")
	}
}

func shareTestSnapshot() core.StatusBoardSnapshot {
	now := time.Now().UTC()
	active := core.StatusItemRuntimeState{Rules: map[string]core.TrendRuleState{"rule": {Active: true}}}
	item := core.StatusBoardItemView{StatusBoardItem: core.StatusBoardItem{ID: "item-1", Name: "Availability", MonitorID: "monitor-1", Enabled: true, RuntimeState: active}, SourceLabel: "Health", Samples: []core.StatusSample{{RecordID: "record-1", StartedAt: now, Display: "OK", Level: core.StatusLevelSuccess}}}
	item.Current = &item.Samples[0]
	return core.StatusBoardSnapshot{Groups: []core.StatusBoardGroup{
		{Monitor: core.Monitor{ID: "monitor-1", Name: "API", ModuleType: "http", Enabled: true, ModuleConfig: json.RawMessage(`{"token":"module-secret"}`), ConditionConfig: json.RawMessage(`{"value":"condition-secret"}`), NotificationChannelIDs: []string{"channel-secret"}}, Items: []core.StatusBoardItemView{item}},
		{Monitor: core.Monitor{ID: "monitor-2", Name: "DB", ModuleType: "sql", Enabled: true}, Items: []core.StatusBoardItemView{{StatusBoardItem: core.StatusBoardItem{ID: "item-2", Name: "Latency", MonitorID: "monitor-2", Enabled: true}}}},
	}}
}
