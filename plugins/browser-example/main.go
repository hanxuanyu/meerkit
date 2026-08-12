package main

import (
	browsermonitor "github.com/hanxuanyu/meerkit/plugins/browser-example/monitor"
	"github.com/hanxuanyu/meerkit/sdk"
)

func main() {
	runtime := sdk.NewPluginRuntime()
	client := runtime.Browser()
	runtime.Serve(sdk.NewProvider(
		browsermonitor.NewHTML(client),
		browsermonitor.NewCSSText(client),
		browsermonitor.NewResponse(client),
	))
}
