package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/slh/rift-ctl/internal/database"
	"github.com/slh/rift-ctl/internal/images"
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
			printSearchResults(os.Stdout, results, false)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum number of results")
	cmd.Flags().StringVar(&setID, "set", "", "filter by set id, e.g. UNL")
	return cmd
}

func printSearchResults(w io.Writer, results []database.SearchResult, numbered bool) {
	if !writerIsTTY(w) {
		for i, r := range results {
			c := r.Card
			if numbered {
				fmt.Fprintf(w, "  %d. %s\n", i+1, c.ID)
			} else {
				fmt.Fprintln(w, c.ID)
			}
		}
		return
	}

	cols, cellWidth, gap := images.GalleryLayout(w)
	for i := 0; i < len(results); i += cols {
		end := i + cols
		if end > len(results) {
			end = len(results)
		}
		row := results[i:end]
		printSearchCaptions(w, row, numbered, i, cellWidth, gap)

		paths := make([]string, len(row))
		for j, r := range row {
			if imageFileReady(r.Card.ImagePath) {
				paths[j] = r.Card.ImagePath
			}
		}
		if err := images.DisplayGalleryRow(w, paths, cellWidth, gap); err != nil {
			fmt.Fprintf(os.Stderr, "image render failed: %v\n", err)
		}
	}
}

func printSearchCaptions(w io.Writer, row []database.SearchResult, numbered bool, start, cellWidth, gap int) {
	parts := make([]string, len(row))
	for i, r := range row {
		label := r.Card.ID
		if numbered {
			label = fmt.Sprintf("%d.%s", start+i+1, r.Card.ID)
		}
		if len(label) > cellWidth {
			label = label[:cellWidth]
		}
		parts[i] = fmt.Sprintf("%-*s", cellWidth, label)
	}
	fmt.Fprintln(w, strings.Join(parts, strings.Repeat(" ", gap)))
}

func writerIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}
