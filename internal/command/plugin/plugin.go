package plugin

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"meerkit/internal/app"
	"meerkit/internal/monitor"
	pluginruntime "meerkit/internal/plugin"
	"meerkit/internal/runtimeconfig"
	"meerkit/internal/store"
)

type ConfigLoader func(*cobra.Command, bool) (app.Config, error)

func New(load ConfigLoader) *cobra.Command {
	root := &cobra.Command{Use: "plugin", Short: "Manage local monitor plugin packages"}
	importCommand := &cobra.Command{Use: "import <archive>", Short: "Import a zip or tar.gz plugin package", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error { return importArchive(command, load, args[0]) }}
	importCommand.Flags().Bool("enable", false, "enable the plugin after import")
	importCommand.Flags().Bool("confirm-unverified", false, "confirm the risk of enabling an unsigned plugin")
	root.AddCommand(importCommand, &cobra.Command{Use: "scan", Short: "Scan the plugin inbox", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error { return scan(command, load) }})
	return root
}
func manager(command *cobra.Command, load ConfigLoader) (*pluginruntime.Manager, *store.Store, error) {
	config, err := load(command, false)
	if err != nil {
		return nil, nil, err
	}
	database, err := store.OpenStore(config.Storage.DataDir)
	if err != nil {
		return nil, nil, err
	}
	runtimeManager, err := runtimeconfig.New(command.Context(), database)
	if err != nil {
		database.Close()
		return nil, nil, err
	}
	runtime := runtimeManager.Snapshot()
	value, err := pluginruntime.NewManager(database, monitor.NewRegistry(), pluginruntime.ManagerOptions{DataDir: config.Storage.DataDir, LogLevel: runtime.Plugins.LogLevel, LogFormat: runtime.Plugins.LogFormat})
	if err != nil {
		database.Close()
		return nil, nil, err
	}
	return value, database, nil
}
func importArchive(command *cobra.Command, load ConfigLoader, path string) error {
	value, database, err := manager(command, load)
	if err != nil {
		return err
	}
	defer database.Close()
	defer value.Close()
	enable, _ := command.Flags().GetBool("enable")
	confirm, _ := command.Flags().GetBool("confirm-unverified")
	installation, err := value.Import(command.Context(), path, pluginruntime.ImportOptions{Enable: enable, AllowUnverified: confirm})
	if err != nil {
		return err
	}
	data, _ := json.MarshalIndent(installation, "", "  ")
	fmt.Fprintln(command.OutOrStdout(), string(data))
	return nil
}
func scan(command *cobra.Command, load ConfigLoader) error {
	value, database, err := manager(command, load)
	if err != nil {
		return err
	}
	defer database.Close()
	defer value.Close()
	installations, err := value.Scan(command.Context())
	if err != nil {
		return err
	}
	fmt.Fprintf(command.OutOrStdout(), "Imported %d plugin package(s).\n", len(installations))
	return nil
}
