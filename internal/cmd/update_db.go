package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/slh/rift-ctl/internal/database"
	"github.com/slh/rift-ctl/internal/fetch"
	"github.com/slh/rift-ctl/internal/images"
	"github.com/spf13/cobra"
)

func newUpdateDBCmd() *cobra.Command {
	var skipImages bool
	cmd := &cobra.Command{
		Use:   "update-db",
		Short: "Download cards into the local database",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdateDB(cmd.Context(), dbPath, skipImages)
		},
	}
	cmd.Flags().BoolVar(&skipImages, "skip-images", false, "store image URLs without downloading files")
	return cmd
}

func runUpdateDB(ctx context.Context, path string, skipImages bool) error {
	db, err := database.Open(path)
	if err != nil {
		return err
	}
	defer db.Close()

	fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cards, err := fetch.NewClient().FetchAll(fetchCtx)
	if err != nil {
		return err
	}
	for _, card := range cards {
		if err := db.UpsertCard(fetchCtx, card); err != nil {
			return err
		}
	}
	if err := db.RebuildSearchIndex(fetchCtx); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := db.SetMeta(fetchCtx, "source", "riftcodex"); err != nil {
		return err
	}
	if err := db.SetMeta(fetchCtx, "last_updated", now); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "updated %d cards in %s\n", len(cards), path)
	if skipImages {
		fmt.Fprintln(os.Stdout, "skipped image cache")
		return nil
	}

	imageCtx, imageCancel := context.WithTimeout(ctx, 30*time.Minute)
	defer imageCancel()
	stats, err := images.Cache(imageCtx, db, database.ImageCacheDir(path), 8)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "images: %d downloaded, %d skipped, %d failed\n", stats.Downloaded, stats.Skipped, stats.Failed)
	return nil
}
