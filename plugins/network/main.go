package main

import (
	dnsmonitor "github.com/hanxuanyu/meerkit/plugins/network/monitor/dns"
	httpmonitor "github.com/hanxuanyu/meerkit/plugins/network/monitor/http"
	icmpmonitor "github.com/hanxuanyu/meerkit/plugins/network/monitor/icmp"
	tcpmonitor "github.com/hanxuanyu/meerkit/plugins/network/monitor/tcp"
	tlsmonitor "github.com/hanxuanyu/meerkit/plugins/network/monitor/tls"
	"github.com/hanxuanyu/meerkit/sdk"
)

func main() {
	runtime := sdk.NewPluginRuntime()
	runtime.Serve(sdk.NewProvider(
		httpmonitor.New(),
		tcpmonitor.New(),
		dnsmonitor.New(),
		tlsmonitor.New(),
		icmpmonitor.New(),
	))
}
