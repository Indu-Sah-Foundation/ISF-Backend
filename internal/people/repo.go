package people

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

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) Create(ctx context.Context, name, email string) (*Person, error) {
	var p Person

	const q = `		
		INSERT INTO people (name, email)
		VALUES ($1, $2)
		RETURNING id, name, email, created_at 
	`
	err := r.pool.QueryRow(ctx, q, name, email).Scan(&p.ID, &p.Name, &p.Email, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create person: %w", err)
	}

	return &p, nil
}

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Person, error) {
	var p Person
	const q = `
		SELECT id, name, email, created_at
		FROM people
		WHERE id = $1
	`
	err := r.pool.QueryRow(ctx, q, id).Scan(&p.ID, &p.Name, &p.Email, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get person: %w", err)
	}
	return &p, nil
}

func (r *Repo) List(ctx context.Context, limit, offset int) ([]Person, error) {
	const q = `SELECT id, name, email, created_at FROM people ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}
	defer rows.Close()
	people := make([]Person, 0)
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.ID, &p.Name, &p.Email, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan person: %w", err)
		}
		people = append(people, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("Iterate people: %w", err)
	}
	return people, nil

}
