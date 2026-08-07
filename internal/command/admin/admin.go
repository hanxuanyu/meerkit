package admin

import (
	"fmt"

	"github.com/spf13/cobra"
	"meerkit/internal/app"
	"meerkit/internal/auth"
	"meerkit/internal/runtimeconfig"
	"meerkit/internal/store"
)

type ConfigLoader func(*cobra.Command, bool) (app.Config, error)

func New(load ConfigLoader) *cobra.Command {
	root := &cobra.Command{Use: "admin", Short: "Administrator recovery commands"}
	reset := &cobra.Command{Use: "reset-key", Short: "Reset the administrator access key and revoke sessions", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		config, err := load(command, false)
		if err != nil {
			return err
		}
		key, _ := command.Flags().GetString("key")
		if key == "" {
			return fmt.Errorf("provide --key")
		}
		connectionLifetime, connectionIdleTime := config.Storage.Database.ConnectionDurations()
		database, err := store.Open(command.Context(), store.Options{Type: store.DatabaseType(config.Storage.Database.Type), DSN: config.Storage.Database.DSN, DataDir: config.Storage.DataDir, AutoMigrate: config.Storage.Database.AutoMigrate, MaxOpenConns: config.Storage.Database.MaxOpenConns, MaxIdleConns: config.Storage.Database.MaxIdleConns, ConnMaxLifetime: connectionLifetime, ConnMaxIdleTime: connectionIdleTime})
		if err != nil {
			return err
		}
		defer database.Close()
		runtimeManager, err := runtimeconfig.New(command.Context(), database)
		if err != nil {
			return err
		}
		if err := auth.NewService(database, runtimeManager.Snapshot().SessionTTLDuration()).ResetKey(command.Context(), key); err != nil {
			return err
		}
		fmt.Fprintln(command.OutOrStdout(), "Administrator access key reset; all sessions revoked.")
		return nil
	}}
	reset.Flags().String("key", "", "new administrator access key")
	root.AddCommand(reset)
	return root
}
