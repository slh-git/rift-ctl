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
	downloaded bool
	err        error
}

func cacheOne(ctx context.Context, client *http.Client, db *database.DB, dir string, rec database.ImageRecord) result {
	target := filepath.Join(dir, rec.ID+extension(rec.ImageURL))
	if rec.ImagePath != "" && fileExists(rec.ImagePath) {
		return result{}
	}
	if fileExists(target) {
		if err := db.SetImagePath(ctx, rec.ID, target); err != nil {
			return result{err: err}
		}
		return result{}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rec.ImageURL, nil)
	if err != nil {
		return result{err: err}
	}
	resp, err := client.Do(req)
	if err != nil {
		return result{err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return result{err: fmt.Errorf("%s: %s", rec.ID, resp.Status)}
	}

	tmp := target + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return result{err: err}
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return result{err: copyErr}
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return result{err: closeErr}
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return result{err: err}
	}
	if err := db.SetImagePath(ctx, rec.ID, target); err != nil {
		return result{err: err}
	}
	return result{downloaded: true}
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
