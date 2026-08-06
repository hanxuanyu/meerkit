package main

import (
	"github.com/hanxuanyu/meerkit/plugins/tcp/monitor"
	"github.com/hanxuanyu/meerkit/sdk"
)

func main() { sdk.Serve(sdk.NewProvider(tcpmonitor.New())) }
