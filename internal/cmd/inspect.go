package cmd

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/slh/rift-ctl/internal/cards"
	"github.com/slh/rift-ctl/internal/database"
	"github.com/slh/rift-ctl/internal/images"
	"github.com/spf13/cobra"
)

func newInspectCmd() *cobra.Command {
	var asJSON bool
	var noImage bool
	cmd := &cobra.Command{
		Use:   "inspect <id-or-ref-or-query>",
		Short: "Show one stored card (falls back to search)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := database.Open(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			ctx := context.Background()
			card, err := lookupCardOrSearch(ctx, db, args[0])
			if err != nil {
				return err
			}

			if asJSON {
				return printJSON(card)
			}

			if !noImage {
				card = maybeShowImage(ctx, db, card)
			}
			return cards.FormatPretty(os.Stdout, card)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print raw API JSON instead of a pretty summary")
	cmd.Flags().BoolVar(&noImage, "no-image", false, "skip terminal image rendering")
	return cmd
}

func maybeShowImage(ctx context.Context, db *database.DB, card cards.Card) cards.Card {
	stdoutTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	if !stdoutTTY {
		if !imageFileReady(card.ImagePath) {
			fmt.Fprintln(os.Stderr, "image_path: not cached")
		}
		return card
	}

	if !imageFileReady(card.ImagePath) {
		if !confirmUpdateDB() {
			fmt.Fprintln(os.Stderr, "image_path: not cached")
			return card
		}
		if err := runUpdateDB(ctx, dbPath, false); err != nil {
			fmt.Fprintf(os.Stderr, "update-db failed: %v\n", err)
			return card
		}
		refreshed, err := lookupCard(ctx, db, card.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "reload after update-db failed: %v\n", err)
			return card
		}
		card = refreshed
		if !imageFileReady(card.ImagePath) {
			fmt.Fprintln(os.Stderr, "image still not cached after update-db")
			return card
		}
	}

	if err := images.Display(os.Stdout, card.ImagePath, images.DefaultWidth); err != nil {
		fmt.Fprintf(os.Stderr, "image render failed: %v\n", err)
	}
	return card
}

func confirmUpdateDB() bool {
	stdinTTY := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
	if !stdinTTY {
		return false
	}
	fmt.Fprint(os.Stderr, "Image not cached. Run update-db now? [y/N] ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

func imageFileReady(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func printJSON(card cards.Card) error {
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

func lookupCardOrSearch(ctx context.Context, db *database.DB, query string) (cards.Card, error) {
	card, err := lookupCard(ctx, db, query)
	if err == nil {
		return card, nil
	}
	if err != sql.ErrNoRows {
		return cards.Card{}, err
	}

	results, err := db.Search(ctx, query, 20, "")
	if err != nil {
		return cards.Card{}, err
	}
	switch len(results) {
	case 0:
		return cards.Card{}, fmt.Errorf("no card found for %q", query)
	case 1:
		return results[0].Card, nil
	default:
		return pickSearchResult(query, results)
	}
}

func pickSearchResult(query string, results []database.SearchResult) (cards.Card, error) {
	fmt.Fprintf(os.Stderr, "Multiple matches for %q:\n", query)
	printSearchResults(os.Stderr, results, true)

	stdinTTY := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
	if !stdinTTY {
		return cards.Card{}, fmt.Errorf("multiple matches; specify an id or ref")
	}

	fmt.Fprintf(os.Stderr, "Choose a card [1-%d]: ", len(results))
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return cards.Card{}, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(results) {
		return cards.Card{}, fmt.Errorf("invalid choice %q", strings.TrimSpace(line))
	}
	return results[n-1].Card, nil
}
