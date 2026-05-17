// Package cleanup walks the Azure Blob container and deletes any image
// that no database row references. Run from the admin endpoint
// POST /admin/images/cleanup. Safe: dry-run mode returns the list of
// orphans without deleting.
package cleanup

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BlobStore is the subset of *storage.Client we need.
type BlobStore interface {
	ListAll(ctx context.Context) ([]string, error)
	Delete(ctx context.Context, blobName string) error
	PublicURL() string
}

type Result struct {
	TotalBlobs   int      `json:"total_blobs"`
	Referenced   int      `json:"referenced"`
	Orphans      []string `json:"orphans"`
	Deleted      []string `json:"deleted,omitempty"`
	DeleteErrors []string `json:"delete_errors,omitempty"`
	DryRun       bool     `json:"dry_run"`
}

type Cleanup struct {
	pool *pgxpool.Pool
	blob BlobStore
}

func New(pool *pgxpool.Pool, blob BlobStore) *Cleanup {
	return &Cleanup{pool: pool, blob: blob}
}

// blobURLRe matches any URL pointing at our blob container. Used to scrape
// references out of free-form text columns (article body_md, achievement
// body, etc.) — much more robust than parsing markdown / HTML.
func (c *Cleanup) blobURLRegex() *regexp.Regexp {
	prefix := regexp.QuoteMeta(c.blob.PublicURL())
	return regexp.MustCompile(prefix + `[^\s)"'<>]+`)
}

// referencedSrcs collects every blob URL currently in use across the DB.
// It pulls:
//   - articles.body_md            (scrape inline image URLs)
//   - article_translations.body_md (same — translations can store images too)
//   - gallery.src
//   - achievements.image_url, achievements.body
//   - projects.image_url, projects.lede, projects.blocks (jsonb text)
//   - team.image_url
//   - volunteers.image_url
//
// Returns a set keyed by full URL.
func (c *Cleanup) referencedSrcs(ctx context.Context) (map[string]struct{}, error) {
	urlRe := c.blobURLRegex()
	refs := make(map[string]struct{})

	add := func(s string) {
		for _, m := range urlRe.FindAllString(s, -1) {
			refs[strings.TrimRight(m, ".,;:")] = struct{}{}
		}
	}

	// Single-field columns where the entire value IS the URL.
	directColumns := []struct {
		table  string
		column string
	}{
		{"gallery", "src"},
		{"achievements", "image_url"},
		{"projects", "image_url"},
		{"team", "image_url"},
		{"volunteers", "image_url"},
	}
	for _, dc := range directColumns {
		rows, err := c.pool.Query(ctx,
			fmt.Sprintf(`SELECT %s FROM %s WHERE %s <> ''`, dc.column, dc.table, dc.column))
		if err != nil {
			return nil, fmt.Errorf("scan %s.%s: %w", dc.table, dc.column, err)
		}
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				rows.Close()
				return nil, err
			}
			add(s)
		}
		rows.Close()
	}

	// Free-form text columns: regex out any blob URLs we find.
	freeFormQueries := []string{
		`SELECT body_md FROM articles`,
		`SELECT body_md FROM article_translations`,
		`SELECT body FROM achievements`,
		`SELECT lede FROM projects`,
		// JSONB → text — cast handles arrays/objects without parsing them.
		`SELECT blocks::text FROM projects`,
	}
	for _, q := range freeFormQueries {
		rows, err := c.pool.Query(ctx, q)
		if err != nil {
			// Skip tables that don't exist yet (migrations not run).
			log.Printf("cleanup: skipping query (%s): %v", q, err)
			continue
		}
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				rows.Close()
				return nil, err
			}
			add(s)
		}
		rows.Close()
	}

	return refs, nil
}

// Run scans the container and either lists or deletes every blob that
// no DB row references.
func (c *Cleanup) Run(ctx context.Context, dryRun bool) (*Result, error) {
	refs, err := c.referencedSrcs(ctx)
	if err != nil {
		return nil, err
	}

	names, err := c.blob.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	prefix := c.blob.PublicURL()
	out := &Result{
		TotalBlobs: len(names),
		DryRun:     dryRun,
	}
	for _, name := range names {
		full := prefix + name
		if _, used := refs[full]; used {
			out.Referenced++
			continue
		}
		out.Orphans = append(out.Orphans, name)
		if dryRun {
			continue
		}
		if err := c.blob.Delete(ctx, name); err != nil {
			out.DeleteErrors = append(out.DeleteErrors, fmt.Sprintf("%s: %v", name, err))
		} else {
			out.Deleted = append(out.Deleted, name)
		}
	}
	return out, nil
}
