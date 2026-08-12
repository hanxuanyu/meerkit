package sdk

import (
	"log/slog"
	"os"
	"strings"

	"github.com/hanxuanyu/meerkit/sdk/logging"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

func Serve(provider Provider) {
	NewPluginRuntime().Serve(provider)
}

func serveWithRuntime(provider Provider, runtime *PluginRuntime) {
	format := strings.ToLower(strings.TrimSpace(os.Getenv("MEERKIT_PLUGIN_LOG_FORMAT")))
	if format == "" {
		format = "simple"
	}
	if format != "text" && format != "simple" && format != "json" {
		format = "simple"
	}
	levelName := strings.TrimSpace(os.Getenv("MEERKIT_PLUGIN_LOG_LEVEL"))
	if levelName == "" {
		levelName = "info"
	}
	level, err := logging.ParseLevel(levelName)
	if err != nil {
		level = slog.LevelInfo
	}
	logger := logging.NewLogger(os.Stderr, format, level, false)
	if pluginID := strings.TrimSpace(os.Getenv("MEERKIT_PLUGIN_ID")); pluginID != "" {
		logger = logger.With("plugin_id", pluginID)
	}
	if version := strings.TrimSpace(os.Getenv("MEERKIT_PLUGIN_VERSION")); version != "" {
		logger = logger.With("version", version)
	}
	slog.SetDefault(logger)
	logger.Info("plugin process starting", "pid", os.Getpid(), "protocol_version", ProtocolVersion, "log_format", format, "log_level", level.String())
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         map[string]plugin.Plugin{"monitor": &MonitorPlugin{Impl: newLoggingProvider(provider, logger), Runtime: runtime}},
		GRPCServer: func(options []grpc.ServerOption) *grpc.Server {
			logger.Info("plugin RPC server initialized")
			return plugin.DefaultGRPCServer(options)
		},
		Logger: hclog.NewNullLogger(),
	})
}
