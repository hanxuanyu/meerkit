package main

import (
	"testing"

	dnsmonitor "github.com/hanxuanyu/meerkit/plugins/network/monitor/dns"
	httpmonitor "github.com/hanxuanyu/meerkit/plugins/network/monitor/http"
	icmpmonitor "github.com/hanxuanyu/meerkit/plugins/network/monitor/icmp"
	tcpmonitor "github.com/hanxuanyu/meerkit/plugins/network/monitor/tcp"
	tlsmonitor "github.com/hanxuanyu/meerkit/plugins/network/monitor/tls"
	"github.com/hanxuanyu/meerkit/sdk"
)

func TestProviderExposesAllNetworkModules(t *testing.T) {
	provider := sdk.NewProvider(httpmonitor.New(), tcpmonitor.New(), dnsmonitor.New(), tlsmonitor.New(), icmpmonitor.New())
	modules, err := provider.ListModules()
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 5 {
		t.Fatalf("expected five modules, got %d", len(modules))
	}
	want := map[string]bool{"http": true, "tcp": true, "dns": true, "tls-certificate": true, "icmp": true}
	for _, module := range modules {
		delete(want, module.Type)
	}
	if len(want) != 0 {
		t.Fatalf("missing modules: %#v", want)
	}
}
