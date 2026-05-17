package achievements

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

const cols = `id, slot, title, organization, place, body, image_url, awarded_at, position, published, created_at, updated_at`

func scan(row pgx.Row) (*Achievement, error) {
	var a Achievement
	if err := row.Scan(&a.ID, &a.Slot, &a.Title, &a.Organization, &a.Place, &a.Body, &a.ImageURL, &a.AwardedAt, &a.Position, &a.Published, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Repo) Create(ctx context.Context, req CreateAchievementRequest) (*Achievement, error) {
	published := true
	if req.Published != nil {
		published = *req.Published
	}
	const q = `
	INSERT INTO achievements (slot, title, organization, place, body, image_url, awarded_at, position, published)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	RETURNING ` + cols
	a, err := scan(r.pool.QueryRow(ctx, q,
		req.Slot, req.Title, req.Organization, req.Place, req.Body, req.ImageURL, req.AwardedAt, req.Position, published,
	))
	if err != nil {
		return nil, fmt.Errorf("create achievement: %w", err)
	}
	return a, nil
}

func (r *Repo) List(ctx context.Context, includeUnpublished bool) ([]Achievement, error) {
	const q = `SELECT ` + cols + ` FROM achievements WHERE ($1 OR published = true) ORDER BY position, created_at`
	rows, err := r.pool.Query(ctx, q, includeUnpublished)
	if err != nil {
		return nil, fmt.Errorf("list achievements: %w", err)
	}
	defer rows.Close()
	out := make([]Achievement, 0)
	for rows.Next() {
		a, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan achievement: %w", err)
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (r *Repo) Update(ctx context.Context, id uuid.UUID, req UpdateAchievementRequest) (*Achievement, error) {
	const q = `
	UPDATE achievements SET
	    title        = COALESCE($2, title),
	    organization = COALESCE($3, organization),
	    place        = COALESCE($4, place),
	    body         = COALESCE($5, body),
	    image_url    = COALESCE($6, image_url),
	    awarded_at   = COALESCE($7, awarded_at),
	    position     = COALESCE($8, position),
	    published    = COALESCE($9, published),
	    updated_at   = NOW()
	WHERE id = $1
	RETURNING ` + cols
	a, err := scan(r.pool.QueryRow(ctx, q, id,
		req.Title, req.Organization, req.Place, req.Body, req.ImageURL, req.AwardedAt, req.Position, req.Published,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update achievement: %w", err)
	}
	return a, nil
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM achievements WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete achievement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
