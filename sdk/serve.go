package sdk

import "github.com/hashicorp/go-plugin"

func Serve(provider Provider) {
	plugin.Serve(&plugin.ServeConfig{HandshakeConfig: Handshake, Plugins: map[string]plugin.Plugin{"monitor": &MonitorPlugin{Impl: provider}}, GRPCServer: plugin.DefaultGRPCServer})
}
