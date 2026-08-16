package main

import (
	browsermonitor "github.com/hanxuanyu/meerkit/plugins/browser-example/monitor"
	"github.com/hanxuanyu/meerkit/sdk"
)

func main() {
	runtime := sdk.NewPluginRuntime()
	runtime.Serve(sdk.NewProvider(browsermonitor.NewModules(runtime.Browser())...))
}
