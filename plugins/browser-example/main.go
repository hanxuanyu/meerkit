package main

import (
	"log/slog"
	"os"

	browsermonitor "github.com/hanxuanyu/meerkit/plugins/browser-example/monitor"
	"github.com/hanxuanyu/meerkit/sdk"
)

func main() {
	client, err := sdk.NewBrowserClientFromEnvironment()
	if err != nil {
		slog.Error("browser capability is unavailable", "error", err)
		os.Exit(1)
	}
	defer client.Close()
	sdk.Serve(sdk.NewProvider(
		browsermonitor.NewHTML(client),
		browsermonitor.NewCSSText(client),
		browsermonitor.NewResponse(client),
	))
}
