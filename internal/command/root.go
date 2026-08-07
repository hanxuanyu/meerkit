package command

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"meerkit/internal/app"
	admincommand "meerkit/internal/command/admin"
	plugincommand "meerkit/internal/command/plugin"
	servecommand "meerkit/internal/command/serve"
)

type usageError struct{ error }

func Execute(dependencies Dependencies, args []string) int {
	if dependencies.Stdin == nil {
		dependencies.Stdin = os.Stdin
	}
	if dependencies.Stdout == nil {
		dependencies.Stdout = os.Stdout
	}
	if dependencies.Stderr == nil {
		dependencies.Stderr = os.Stderr
	}
	root := NewRoot(dependencies)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(dependencies.Stderr, "Error:", err)
		var usage *usageError
		message := err.Error()
		if errors.As(err, &usage) || strings.Contains(message, "unknown command") || strings.Contains(message, "unknown flag") || strings.Contains(message, "requires") || strings.Contains(message, "accepts") {
			_ = root.Usage()
			return 2
		}
		return 1
	}
	return 0
}

func NewRoot(dependencies Dependencies) *cobra.Command {
	root := &cobra.Command{Use: "meerkit", Short: "Meerkit monitoring service", SilenceErrors: true, SilenceUsage: true}
	root.SetIn(dependencies.Stdin)
	root.SetOut(dependencies.Stdout)
	root.SetErr(dependencies.Stderr)
	root.PersistentFlags().String("config", "", "path to config.yaml")
	root.PersistentFlags().String("data-dir", "", "SQLite and runtime data directory")
	load := func(command *cobra.Command, create bool) (app.Config, error) {
		return app.LoadConfigWithOptions(configOptions(command, create))
	}
	servecommand.BindFlags(root)
	root.RunE = func(command *cobra.Command, args []string) error {
		if len(args) != 0 {
			return &usageError{fmt.Errorf("unexpected arguments: %s", strings.Join(args, " "))}
		}
		return servecommand.Run(command, dependencies.Frontend, load, dependencies.Version)
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &usageError{err} })
	root.AddCommand(servecommand.New(dependencies.Frontend, load, dependencies.Version), plugincommand.New(load), admincommand.New(load), &cobra.Command{Use: "version", Short: "Print version", Args: noArgs, Run: func(command *cobra.Command, _ []string) { fmt.Fprintln(command.OutOrStdout(), dependencies.Version) }})
	return root
}

func noArgs(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return &usageError{fmt.Errorf("this command accepts no arguments")}
	}
	return nil
}
func configOptions(command *cobra.Command, create bool) app.ConfigOptions {
	options := app.ConfigOptions{CreateDefault: create, Overrides: map[string]any{}, ChangedFlags: map[string]bool{}}
	options.ConfigFile, _ = command.Flags().GetString("config")
	if flag := command.Flags().Lookup("data-dir"); flag != nil && flag.Changed {
		value, _ := command.Flags().GetString("data-dir")
		options.Overrides["storage.data_dir"] = value
		options.ChangedFlags["data-dir"] = true
	}
	mappings := map[string]string{"log-dir": "logging.file.directory", "log-filename": "logging.file.filename", "access-log-filename": "logging.file.access.filename"}
	for name, path := range mappings {
		flag := command.Flags().Lookup(name)
		if flag == nil || !flag.Changed {
			continue
		}
		options.ChangedFlags[name] = true
		switch flag.Value.Type() {
		case "bool":
			value, _ := command.Flags().GetBool(name)
			options.Overrides[path] = value
		case "int":
			value, _ := command.Flags().GetInt(name)
			options.Overrides[path] = value
		default:
			value, _ := command.Flags().GetString(name)
			options.Overrides[path] = value
		}
	}
	if flag := command.Flags().Lookup("listen"); flag != nil && flag.Changed {
		options.Listen, _ = command.Flags().GetString("listen")
		options.ChangedFlags["listen"] = true
	}
	return options
}
