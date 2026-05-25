package contacts

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) Create(ctx context.Context, email, message, ip string) (*Contact, error) {
	const q = `
	INSERT INTO contacts (email, message, ip)
	VALUES ($1, $2, $3)
	RETURNING id, email, message, ip, read, created_at`
	var c Contact
	if err := r.pool.QueryRow(ctx, q, email, message, ip).Scan(
		&c.ID, &c.Email, &c.Message, &c.IP, &c.Read, &c.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}
	return &c, nil
}

func (r *Repo) List(ctx context.Context, limit, offset int) ([]Contact, error) {
	const q = `
	SELECT id, email, message, ip, read, created_at
	FROM contacts
	ORDER BY created_at DESC
	LIMIT $1 OFFSET $2`
	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()
	out := make([]Contact, 0)
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.ID, &c.Email, &c.Message, &c.IP, &c.Read, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repo) UnreadCount(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM contacts WHERE read = FALSE`).Scan(&n)
	return n, err
}

func (r *Repo) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*Contact, error) {
	const q = `
	UPDATE contacts
	SET read = COALESCE($2, read)
	WHERE id = $1
	RETURNING id, email, message, ip, read, created_at`
	var c Contact
	if err := r.pool.QueryRow(ctx, q, id, req.Read).Scan(
		&c.ID, &c.Email, &c.Message, &c.IP, &c.Read, &c.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update contact: %w", err)
	}
	return &c, nil
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM contacts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
