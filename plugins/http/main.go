package main

import (
	"github.com/hanxuanyu/meerkit/plugins/http/monitor"
	"github.com/hanxuanyu/meerkit/sdk"
)

func main() { sdk.Serve(sdk.NewProvider(httpmonitor.New())) }
