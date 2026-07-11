package cmd

import (
	"fmt"
	"os"

	"github.com/slh/rift-ctl/internal/database"
	"github.com/spf13/cobra"
)

var dbPath string

func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "riftctl",
		Short: "Local Riftbound card data tools",
	}
	cmd.PersistentFlags().StringVar(&dbPath, "db", database.DefaultPath(), "path to SQLite card database")
	cmd.AddCommand(newUpdateDBCmd(), newSearchCmd(), newInspectCmd())
	return cmd
}
