package projects

import (
	"context"
	"encoding/json"
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

const baseCols = `id, slug, kind, title, label, lede, image_url, image_variant, blocks, position, published, created_at, updated_at`

// scan a single row using pgx — blocks come back as raw JSONB which we
// decode into our typed Blocks struct.
func scanProject(row pgx.Row) (*Project, error) {
	var p Project
	var blocksRaw []byte
	if err := row.Scan(&p.ID, &p.Slug, &p.Kind, &p.Title, &p.Label, &p.Lede, &p.ImageURL, &p.ImageVariant, &blocksRaw, &p.Position, &p.Published, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	if len(blocksRaw) > 0 {
		if err := json.Unmarshal(blocksRaw, &p.Blocks); err != nil {
			return nil, fmt.Errorf("decode blocks: %w", err)
		}
	}
	if p.Blocks == nil {
		p.Blocks = Blocks{}
	}
	return &p, nil
}

func (r *Repo) Create(ctx context.Context, req CreateProjectRequest) (*Project, error) {
	blocksJSON, err := json.Marshal(req.Blocks)
	if err != nil {
		return nil, fmt.Errorf("marshal blocks: %w", err)
	}
	if req.ImageVariant == "" {
		req.ImageVariant = "default"
	}
	published := true
	if req.Published != nil {
		published = *req.Published
	}
	const q = `
	INSERT INTO projects (slug, kind, title, label, lede, image_url, image_variant, blocks, position, published)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10)
	RETURNING ` + baseCols
	return scanProject(r.pool.QueryRow(ctx, q,
		req.Slug, req.Kind, req.Title, req.Label, req.Lede, req.ImageURL, req.ImageVariant,
		string(blocksJSON), req.Position, published,
	))
}

func (r *Repo) Get(ctx context.Context, id uuid.UUID) (*Project, error) {
	const q = `SELECT ` + baseCols + ` FROM projects WHERE id = $1`
	p, err := scanProject(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	return p, nil
}

func (r *Repo) GetBySlug(ctx context.Context, slug string) (*Project, error) {
	const q = `SELECT ` + baseCols + ` FROM projects WHERE slug = $1`
	p, err := scanProject(r.pool.QueryRow(ctx, q, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	return p, nil
}

// List returns projects ordered by (kind ASC, position ASC). `includeUnpublished`
// is for admin views; public endpoints pass false.
func (r *Repo) List(ctx context.Context, includeUnpublished bool, kind string) ([]Project, error) {
	q := `SELECT ` + baseCols + ` FROM projects WHERE ($1 OR published = true)`
	args := []any{includeUnpublished}
	if kind != "" {
		q += ` AND kind = $2`
		args = append(args, kind)
	}
	q += ` ORDER BY kind, position, created_at`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	out := make([]Project, 0)
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (r *Repo) Update(ctx context.Context, id uuid.UUID, req UpdateProjectRequest) (*Project, error) {
	var blocksJSON *string
	if req.Blocks != nil {
		bs, err := json.Marshal(*req.Blocks)
		if err != nil {
			return nil, fmt.Errorf("marshal blocks: %w", err)
		}
		s := string(bs)
		blocksJSON = &s
	}
	const q = `
	UPDATE projects SET
	    kind          = COALESCE($2, kind),
	    title         = COALESCE($3, title),
	    label         = COALESCE($4, label),
	    lede          = COALESCE($5, lede),
	    image_url     = COALESCE($6, image_url),
	    image_variant = COALESCE($7, image_variant),
	    blocks        = COALESCE($8::jsonb, blocks),
	    position      = COALESCE($9, position),
	    published     = COALESCE($10, published),
	    updated_at    = NOW()
	WHERE id = $1
	RETURNING ` + baseCols
	p, err := scanProject(r.pool.QueryRow(ctx, q, id,
		req.Kind, req.Title, req.Label, req.Lede, req.ImageURL, req.ImageVariant,
		blocksJSON, req.Position, req.Published,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update project: %w", err)
	}
	return p, nil
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
