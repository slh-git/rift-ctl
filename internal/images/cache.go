package images

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/slh/rift-ctl/internal/database"
)

type Stats struct {
	Downloaded int
	Skipped    int
	Failed     int
}

func Cache(ctx context.Context, db *database.DB, dir string, workers int) (Stats, error) {
	if workers <= 0 {
		workers = 8
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Stats{}, err
	}
	records, err := db.ListImageRecords(ctx)
	if err != nil {
		return Stats{}, err
	}

	client := &http.Client{Timeout: 60 * time.Second}
	jobs := make(chan database.ImageRecord)
	results := make(chan result)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for rec := range jobs {
				results <- cacheOne(ctx, client, db, dir, rec)
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, rec := range records {
			select {
			case <-ctx.Done():
				return
			case jobs <- rec:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var stats Stats
	for res := range results {
		switch {
		case res.err != nil:
			stats.Failed++
			fmt.Fprintf(os.Stderr, "image failed: %s %s: %v\n", res.id, res.url, res.err)
		case res.downloaded:
			stats.Downloaded++
		default:
			stats.Skipped++
		}
	}
	if err := ctx.Err(); err != nil {
		return stats, err
	}
	return stats, nil
}

type result struct {
	id         string
	url        string
	downloaded bool
	err        error
}

func fail(rec database.ImageRecord, err error) result {
	return result{id: rec.ID, url: rec.ImageURL, err: err}
}

func cacheOne(ctx context.Context, client *http.Client, db *database.DB, dir string, rec database.ImageRecord) result {
	if strings.TrimSpace(rec.ImageURL) == "" {
		return fail(rec, fmt.Errorf("empty image URL"))
	}

	target := filepath.Join(dir, rec.ID+extension(rec.ImageURL))
	if rec.ImagePath != "" && fileExists(rec.ImagePath) {
		return result{}
	}
	if fileExists(target) {
		if err := db.SetImagePath(ctx, rec.ID, target); err != nil {
			return fail(rec, err)
		}
		return result{}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rec.ImageURL, nil)
	if err != nil {
		return fail(rec, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fail(rec, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fail(rec, fmt.Errorf("HTTP %s", resp.Status))
	}

	tmp := target + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fail(rec, err)
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return fail(rec, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return fail(rec, closeErr)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fail(rec, err)
	}
	if err := db.SetImagePath(ctx, rec.ID, target); err != nil {
		return fail(rec, err)
	}
	return result{id: rec.ID, url: rec.ImageURL, downloaded: true}
}

func extension(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ".png"
	}
	ext := strings.ToLower(path.Ext(u.Path))
	if ext == "" {
		return ".png"
	}
	return ext
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
