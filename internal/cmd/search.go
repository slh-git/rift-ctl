package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/slh/rift-ctl/internal/database"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var limit int
	var setID string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the local full-text card index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := database.Open(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			results, err := db.Search(context.Background(), args[0], limit, setID)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Fprintln(os.Stderr, "no matches")
				return nil
			}
			for _, r := range results {
				c := r.Card
				fmt.Fprintf(os.Stdout, "%s  %s  [%s] %s  %s\n", c.ID, c.Name, c.SetID, c.Type, strings.Join(c.Domains, ", "))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum number of results")
	cmd.Flags().StringVar(&setID, "set", "", "filter by set id, e.g. UNL")
	return cmd
}
