package api

import (
	"testing"

	"meerkit/internal/core"
)

func TestPaginatePluginsFiltersAndPaginates(t *testing.T) {
	values := []core.PluginInstallation{
		{ID: "meerkit.http", Name: "HTTP", Version: "1.0.0", Vendor: "Meerkit", Enabled: true, Status: "healthy", TrustState: "official", Modules: []core.PluginModule{{Type: "http", Name: "HTTP 请求"}}},
		{ID: "meerkit.tcp", Name: "TCP", Version: "1.0.0", Vendor: "Meerkit", Enabled: false, Status: "disabled", TrustState: "official", Modules: []core.PluginModule{{Type: "tcp", Name: "TCP 连接"}}},
		{ID: "example.dns", Name: "DNS", Version: "2.0.0", Vendor: "Example", Enabled: true, Status: "degraded", TrustState: "trusted", Modules: []core.PluginModule{{Type: "dns", Name: "DNS 查询"}}},
	}

	result := paginatePlugins(values, pluginListOptions{Page: 1, PageSize: 10, Search: "请求", Status: "enabled", TrustState: "official"})
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != "meerkit.http" {
		t.Fatalf("unexpected filtered result: %#v", result)
	}

	result = paginatePlugins(values, pluginListOptions{Page: 9, PageSize: 2})
	if result.Page != 2 || result.TotalPages != 2 || len(result.Items) != 1 || result.Items[0].ID != "example.dns" {
		t.Fatalf("unexpected paginated result: %#v", result)
	}
}

func TestPaginatePluginsLimitsPageSize(t *testing.T) {
	result := paginatePlugins(nil, pluginListOptions{Page: -1, PageSize: 500})
	if result.Page != 1 || result.PageSize != 100 || result.Total != 0 || result.TotalPages != 0 || len(result.Items) != 0 {
		t.Fatalf("unexpected empty page: %#v", result)
	}
}
