package gallery

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

const cols = `id, src, title, caption, size, tags, position, published, created_at, updated_at`

func scan(row pgx.Row) (*Item, error) {
	var i Item
	if err := row.Scan(&i.ID, &i.Src, &i.Title, &i.Caption, &i.Size, &i.Tags, &i.Position, &i.Published, &i.CreatedAt, &i.UpdatedAt); err != nil {
		return nil, err
	}
	if i.Tags == nil {
		i.Tags = []string{}
	}
	return &i, nil
}

func (r *Repo) Create(ctx context.Context, req CreateItemRequest) (*Item, error) {
	size := req.Size
	if size == "" {
		size = "M"
	}
	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}
	published := true
	if req.Published != nil {
		published = *req.Published
	}
	const q = `
	INSERT INTO gallery (src, title, caption, size, tags, position, published)
	VALUES ($1,$2,$3,$4,$5,$6,$7)
	RETURNING ` + cols
	item, err := scan(r.pool.QueryRow(ctx, q, req.Src, req.Title, req.Caption, size, tags, req.Position, published))
	if err != nil {
		return nil, fmt.Errorf("create gallery item: %w", err)
	}
	return item, nil
}

func (r *Repo) List(ctx context.Context, includeUnpublished bool) ([]Item, error) {
	const q = `SELECT ` + cols + ` FROM gallery WHERE ($1 OR published = true) ORDER BY position, created_at`
	rows, err := r.pool.Query(ctx, q, includeUnpublished)
	if err != nil {
		return nil, fmt.Errorf("list gallery: %w", err)
	}
	defer rows.Close()
	out := make([]Item, 0)
	for rows.Next() {
		it, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan gallery item: %w", err)
		}
		out = append(out, *it)
	}
	return out, rows.Err()
}

// AllSrcs returns every blob URL referenced by gallery (regardless of
// published flag). Used by the orphan-cleanup job.
func (r *Repo) AllSrcs(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT src FROM gallery`)
	if err != nil {
		return nil, fmt.Errorf("list gallery srcs: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repo) Update(ctx context.Context, id uuid.UUID, req UpdateItemRequest) (*Item, error) {
	const q = `
	UPDATE gallery SET
	    src        = COALESCE($2, src),
	    title      = COALESCE($3, title),
	    caption    = COALESCE($4, caption),
	    size       = COALESCE($5, size),
	    tags       = COALESCE($6, tags),
	    position   = COALESCE($7, position),
	    published  = COALESCE($8, published),
	    updated_at = NOW()
	WHERE id = $1
	RETURNING ` + cols
	item, err := scan(r.pool.QueryRow(ctx, q, id, req.Src, req.Title, req.Caption, req.Size, req.Tags, req.Position, req.Published))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update gallery item: %w", err)
	}
	return item, nil
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) (string, error) {
	var src string
	if err := r.pool.QueryRow(ctx, `DELETE FROM gallery WHERE id = $1 RETURNING src`, id).Scan(&src); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("delete gallery item: %w", err)
	}
	return src, nil
}

// ---------------------------------------------------------------------------
// Tag CRUD — admin-curated tag names that drive the image-tagging UI.
// ---------------------------------------------------------------------------

func (r *Repo) ListTags(ctx context.Context) ([]Tag, error) {
	const q = `SELECT id, name, position, created_at FROM gallery_tags ORDER BY position ASC, name ASC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list gallery tags: %w", err)
	}
	defer rows.Close()
	out := make([]Tag, 0)
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Position, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan gallery tag: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repo) CreateTag(ctx context.Context, req CreateTagRequest) (*Tag, error) {
	const q = `INSERT INTO gallery_tags (name, position) VALUES ($1, $2) RETURNING id, name, position, created_at`
	var t Tag
	if err := r.pool.QueryRow(ctx, q, req.Name, req.Position).Scan(&t.ID, &t.Name, &t.Position, &t.CreatedAt); err != nil {
		return nil, fmt.Errorf("create gallery tag: %w", err)
	}
	return &t, nil
}

func (r *Repo) DeleteTag(ctx context.Context, id uuid.UUID) error {
	res, err := r.pool.Exec(ctx, `DELETE FROM gallery_tags WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete gallery tag: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
