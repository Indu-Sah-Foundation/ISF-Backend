package team

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

const cols = `id, kind, name, role, extras, bio, motto, image_url, position, published, created_at, updated_at`

func scan(row pgx.Row) (*Member, error) {
	var m Member
	if err := row.Scan(&m.ID, &m.Kind, &m.Name, &m.Role, &m.Extras, &m.Bio, &m.Motto, &m.ImageURL, &m.Position, &m.Published, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *Repo) Create(ctx context.Context, req CreateMemberRequest) (*Member, error) {
	published := true
	if req.Published != nil {
		published = *req.Published
	}
	const q = `
	INSERT INTO team (kind, name, role, extras, bio, motto, image_url, position, published)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	RETURNING ` + cols
	m, err := scan(r.pool.QueryRow(ctx, q, req.Kind, req.Name, req.Role, req.Extras, req.Bio, req.Motto, req.ImageURL, req.Position, published))
	if err != nil {
		return nil, fmt.Errorf("create team member: %w", err)
	}
	return m, nil
}

func (r *Repo) List(ctx context.Context, includeUnpublished bool, kind string) ([]Member, error) {
	q := `SELECT ` + cols + ` FROM team WHERE ($1 OR published = true)`
	args := []any{includeUnpublished}
	if kind != "" {
		q += ` AND kind = $2`
		args = append(args, kind)
	}
	q += ` ORDER BY kind, position, created_at`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list team: %w", err)
	}
	defer rows.Close()
	out := make([]Member, 0)
	for rows.Next() {
		m, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (r *Repo) Update(ctx context.Context, id uuid.UUID, req UpdateMemberRequest) (*Member, error) {
	const q = `
	UPDATE team SET
	    kind       = COALESCE($2, kind),
	    name       = COALESCE($3, name),
	    role       = COALESCE($4, role),
	    extras     = COALESCE($5, extras),
	    bio        = COALESCE($6, bio),
	    motto      = COALESCE($7, motto),
	    image_url  = COALESCE($8, image_url),
	    position   = COALESCE($9, position),
	    published  = COALESCE($10, published),
	    updated_at = NOW()
	WHERE id = $1
	RETURNING ` + cols
	m, err := scan(r.pool.QueryRow(ctx, q, id, req.Kind, req.Name, req.Role, req.Extras, req.Bio, req.Motto, req.ImageURL, req.Position, req.Published))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update team member: %w", err)
	}
	return m, nil
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM team WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete team member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
