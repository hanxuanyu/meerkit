package admin

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"meerkit/internal/app"
	"meerkit/internal/auth"
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
			key = os.Getenv("MEERKIT_ADMIN_KEY")
		}
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("provide --key or MEERKIT_ADMIN_KEY")
		}
		database, err := store.OpenStore(config.Storage.DataDir)
		if err != nil {
			return err
		}
		defer database.Close()
		ttl, _ := time.ParseDuration(config.Security.SessionTTL)
		if err := auth.NewService(database, ttl).ResetKey(command.Context(), key); err != nil {
			return err
		}
		fmt.Fprintln(command.OutOrStdout(), "Administrator access key reset; all sessions revoked.")
		return nil
	}}
	reset.Flags().String("key", "", "new administrator access key")
	root.AddCommand(reset)
	return root
}
