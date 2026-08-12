package main

import (
	"github.com/hanxuanyu/meerkit/plugins/tcp/monitor"
	"github.com/hanxuanyu/meerkit/sdk"
)

func main() {
	runtime := sdk.NewPluginRuntime()
	runtime.Serve(sdk.NewProvider(tcpmonitor.New()))
}
