package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/slh/rift-ctl/internal/cards"
	"github.com/slh/rift-ctl/internal/database"
	"github.com/spf13/cobra"
)

func newInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <id-or-ref>",
		Short: "Show one stored card",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := database.Open(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			card, err := lookupCard(context.Background(), db, args[0])
			if err != nil {
				return err
			}
			var raw any
			if err := json.Unmarshal([]byte(card.APIJSON), &raw); err != nil {
				return err
			}
			pretty, err := json.MarshalIndent(raw, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, string(pretty))
			if card.ImagePath != "" {
				fmt.Fprintf(os.Stdout, "image_path: %s\n", card.ImagePath)
			} else {
				fmt.Fprintln(os.Stdout, "image_path: not cached")
			}
			return nil
		},
	}
}

func lookupCard(ctx context.Context, db *database.DB, s string) (cards.Card, error) {
	card, err := db.GetByID(ctx, s)
	if err == nil {
		return card, nil
	}
	if err != sql.ErrNoRows {
		return cards.Card{}, err
	}
	ref, parseErr := cards.ParseShortRef(s)
	if parseErr != nil {
		return cards.Card{}, err
	}
	return db.GetByRef(ctx, ref)
}
