package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/slh/rift-ctl/internal/database"
	"github.com/slh/rift-ctl/internal/images"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var (
		limit      int
		setFlag    string
		typeFlag   string
		superFlag  string
		rarityFlag string
		domainFlag string
		domainAny  bool
		energyFlag string
		mightFlag  string
		powerFlag  string
		signature  bool
		altArt     bool
		newCard    bool
	)

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search the local full-text card index",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := buildSearchFilter(cmd, args, limit, setFlag, typeFlag, superFlag, rarityFlag, domainFlag, domainAny, energyFlag, mightFlag, powerFlag, signature, altArt, newCard)
			if err != nil {
				return err
			}

			db, err := database.Open(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			results, err := db.Search(context.Background(), f)
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
	cmd.Flags().StringVar(&setFlag, "set", "", "filter by set id(s), comma-separated, e.g. UNL,OGN")
	cmd.Flags().StringVar(&typeFlag, "type", "", "filter by type(s), comma-separated")
	cmd.Flags().StringVar(&superFlag, "supertype", "", "filter by supertype(s), comma-separated")
	cmd.Flags().StringVar(&rarityFlag, "rarity", "", "filter by rarity(ies), comma-separated")
	cmd.Flags().StringVar(&domainFlag, "domain", "", "filter by domain(s), comma-separated")
	cmd.Flags().BoolVar(&domainAny, "domain-any", false, "match any listed domain instead of all")
	cmd.Flags().StringVar(&energyFlag, "energy", "", "energy range, e.g. 2, 2-4, -3, 2-")
	cmd.Flags().StringVar(&mightFlag, "might", "", "might range, e.g. 2, 2-4, -3, 2-")
	cmd.Flags().StringVar(&powerFlag, "power", "", "power range, e.g. 2, 2-4, -3, 2-")
	cmd.Flags().BoolVar(&signature, "signature", false, "only signature cards")
	cmd.Flags().BoolVar(&altArt, "alt-art", false, "only alternate-art cards")
	cmd.Flags().BoolVar(&newCard, "new", false, "only cards marked new")
	return cmd
}

func buildSearchFilter(
	cmd *cobra.Command,
	args []string,
	limit int,
	setFlag, typeFlag, superFlag, rarityFlag, domainFlag string,
	domainAny bool,
	energyFlag, mightFlag, powerFlag string,
	signature, altArt, newCard bool,
) (database.SearchFilter, error) {
	f := database.SearchFilter{Limit: limit, DomainAny: domainAny}
	if len(args) > 0 {
		f.Query = args[0]
	}
	f.SetIDs = splitCSV(setFlag)
	f.Types = splitCSV(typeFlag)
	f.Supertypes = splitCSV(superFlag)
	f.Rarities = splitCSV(rarityFlag)
	f.Domains = splitCSV(domainFlag)

	var err error
	if f.Energy, err = parseIntRange(energyFlag); err != nil {
		return f, fmt.Errorf("energy: %w", err)
	}
	if f.Might, err = parseIntRange(mightFlag); err != nil {
		return f, fmt.Errorf("might: %w", err)
	}
	if f.Power, err = parseIntRange(powerFlag); err != nil {
		return f, fmt.Errorf("power: %w", err)
	}

	if cmd.Flags().Changed("signature") {
		v := signature
		f.Signature = &v
	}
	if cmd.Flags().Changed("alt-art") {
		v := altArt
		f.AlternateArt = &v
	}
	if cmd.Flags().Changed("new") {
		v := newCard
		f.New = &v
	}

	if strings.TrimSpace(f.Query) == "" && !f.HasFacets() {
		return f, fmt.Errorf("provide a search query or at least one filter")
	}
	return f, nil
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseIntRange accepts N, N-M, -M, or N-.
func parseIntRange(s string) (database.IntRange, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return database.IntRange{}, nil
	}
	if !strings.Contains(s, "-") {
		n, err := strconv.Atoi(s)
		if err != nil {
			return database.IntRange{}, fmt.Errorf("invalid range %q", s)
		}
		return database.IntRange{Min: &n, Max: &n}, nil
	}
	// Leading hyphen alone means max-only only when followed by digits: "-3"
	// "2-4", "2-", "-3" are valid. Bare "-" is invalid.
	parts := strings.SplitN(s, "-", 2)
	left, right := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	var r database.IntRange
	if left != "" {
		n, err := strconv.Atoi(left)
		if err != nil {
			return database.IntRange{}, fmt.Errorf("invalid range %q", s)
		}
		r.Min = &n
	}
	if right != "" {
		n, err := strconv.Atoi(right)
		if err != nil {
			return database.IntRange{}, fmt.Errorf("invalid range %q", s)
		}
		r.Max = &n
	}
	if r.Min == nil && r.Max == nil {
		return database.IntRange{}, fmt.Errorf("invalid range %q", s)
	}
	if r.Min != nil && r.Max != nil && *r.Min > *r.Max {
		return database.IntRange{}, fmt.Errorf("invalid range %q: min > max", s)
	}
	return r, nil
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
