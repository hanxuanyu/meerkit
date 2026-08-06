package serve

import (
	"context"
	"io/fs"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"meerkit/internal/app"
	"meerkit/internal/application"
)

type ConfigLoader func(*cobra.Command, bool) (app.Config, error)

func New(frontend fs.FS, load ConfigLoader, version string) *cobra.Command {
	command := &cobra.Command{Use: "serve", Short: "Start the Meerkit service", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error { return Run(command, frontend, load, version) }}
	BindFlags(command)
	return command
}
func BindFlags(command *cobra.Command) {
	flags := command.Flags()
	flags.String("listen", "", "listen address, for example 0.0.0.0:8080")
	flags.String("retention", "", "record retention, for example 30d")
	flags.String("timezone", "", "scheduler timezone")
	flags.Int("max-concurrency", 0, "maximum concurrent checks")
	flags.String("log-level", "", "log level")
	flags.String("log-format", "", "log format: text, simple, or json")
	flags.String("log-dir", "", "log file directory")
	flags.String("log-filename", "", "log file name")
	flags.Bool("log-console", false, "enable business logging on console")
	flags.Bool("log-file-enabled", false, "enable file logging")
	flags.Bool("log-console-access", false, "enable HTTP access logging on console")
	flags.String("access-log-filename", "", "HTTP access log file name")
	flags.Bool("access-log-file-enabled", false, "enable HTTP access log file")
}
func Run(command *cobra.Command, frontend fs.FS, load ConfigLoader, version string) error {
	config, err := load(command, true)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return application.RunServer(ctx, config, frontend, application.ServerOptions{Version: version})
}
