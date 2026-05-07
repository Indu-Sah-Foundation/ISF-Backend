package auth

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

func (r *Repo) GetByEmail(ctx context.Context, email string) (*User, error) {
	const q = `SELECT id, email, password_hash, role, created_at FROM users WHERE email = $1`
	var u User
	err := r.pool.QueryRow(ctx, q, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

func (r *Repo) Create(ctx context.Context, email, passwordHash, role string) (*User, error) {
	const q = `
    INSERT INTO users (email, password_hash, role)
    VALUES ($1, $2, $3)
    RETURNING id, email, password_hash, role, created_at
    `
	var u User
	err := r.pool.QueryRow(ctx, q, email, passwordHash, role).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	const q = `SELECT id, email, password_hash, role, created_at FROM users WHERE id = $1`
	var u User
	err := r.pool.QueryRow(ctx, q, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}
