package volunteers

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

const cols = `id, kind, name, bio, image_url, position, published, created_at, updated_at`

func scan(row pgx.Row) (*Volunteer, error) {
	var v Volunteer
	if err := row.Scan(&v.ID, &v.Kind, &v.Name, &v.Bio, &v.ImageURL, &v.Position, &v.Published, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *Repo) Create(ctx context.Context, req CreateVolunteerRequest) (*Volunteer, error) {
	published := true
	if req.Published != nil {
		published = *req.Published
	}
	const q = `
	INSERT INTO volunteers (kind, name, bio, image_url, position, published)
	VALUES ($1,$2,$3,$4,$5,$6)
	RETURNING ` + cols
	v, err := scan(r.pool.QueryRow(ctx, q, req.Kind, req.Name, req.Bio, req.ImageURL, req.Position, published))
	if err != nil {
		return nil, fmt.Errorf("create volunteer: %w", err)
	}
	return v, nil
}

func (r *Repo) List(ctx context.Context, includeUnpublished bool, kind string) ([]Volunteer, error) {
	q := `SELECT ` + cols + ` FROM volunteers WHERE ($1 OR published = true)`
	args := []any{includeUnpublished}
	if kind != "" {
		q += ` AND kind = $2`
		args = append(args, kind)
	}
	q += ` ORDER BY kind, position, created_at`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list volunteers: %w", err)
	}
	defer rows.Close()
	out := make([]Volunteer, 0)
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan volunteer: %w", err)
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

func (r *Repo) Update(ctx context.Context, id uuid.UUID, req UpdateVolunteerRequest) (*Volunteer, error) {
	const q = `
	UPDATE volunteers SET
	    kind       = COALESCE($2, kind),
	    name       = COALESCE($3, name),
	    bio        = COALESCE($4, bio),
	    image_url  = COALESCE($5, image_url),
	    position   = COALESCE($6, position),
	    published  = COALESCE($7, published),
	    updated_at = NOW()
	WHERE id = $1
	RETURNING ` + cols
	v, err := scan(r.pool.QueryRow(ctx, q, id, req.Kind, req.Name, req.Bio, req.ImageURL, req.Position, req.Published))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update volunteer: %w", err)
	}
	return v, nil
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM volunteers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete volunteer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
