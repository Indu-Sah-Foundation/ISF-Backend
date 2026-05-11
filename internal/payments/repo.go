package payments

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

func (r *Repo) Create(ctx context.Context, d *Donation) error {
	const q = `
    INSERT INTO donations (amount_cents, currency, email, name, status, stripe_session_id)
    VALUES ($1, $2, $3, $4, $5, $6)
    RETURNING id, created_at, updated_at
    `

	return r.pool.QueryRow(ctx, q, d.AmountCents, d.Currency, d.Email, d.Name, d.Status, d.StripeSessionID).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
}

func (r *Repo) MarkPaid(ctx context.Context, sessionID, paymentIntentID string) error {
	const q = `
    UPDATE donations
    SET status = 'paid',
        stripe_payment_intent_id = $2,
        updated_at = NOW()
    WHERE stripe_session_id = $1
    `
	tag, err := r.pool.Exec(ctx, q, sessionID, paymentIntentID)
	if err != nil {
		return fmt.Errorf("mark paid: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) MarkFailed(ctx context.Context, sessionID string) error {
	const q = `UPDATE donations SET status = 'failed', updated_at = NOW() WHERE stripe_session_id = $1`
	_, err := r.pool.Exec(ctx, q, sessionID)
	return err
}

func (r *Repo) List(ctx context.Context, status string, limit, offset int) ([]Donation, error) {
	const q = `
    SELECT id, amount_cents, currency, email, name, status,
           stripe_session_id, stripe_payment_intent_id, created_at, updated_at
    FROM donations
    WHERE ($1 = '' OR status = $1)
    ORDER BY created_at DESC
    LIMIT $2 OFFSET $3
    `
	rows, err := r.pool.Query(ctx, q, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list donations: %w", err)
	}
	defer rows.Close()
	out := make([]Donation, 0)
	for rows.Next() {
		var d Donation
		if err := rows.Scan(&d.ID, &d.AmountCents, &d.Currency, &d.Email, &d.Name, &d.Status, &d.StripeSessionID, &d.StripePaymentIntentID, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan donation: %w", err)
		}
		out = append(out, d)

	}
	return out, rows.Err()
}

func (r *Repo) RecordEvent(ctx context.Context, eventID string) (firstTime bool, err error) {
	const q = `INSERT INTO stripe_events (id) VALUES ($1) ON CONFLICT (id) DO NOTHING`
	tag, err := r.pool.Exec(ctx, q, eventID)
	if err != nil {
		return false, fmt.Errorf("record event: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Donation, error) {
	const q = `
    SELECT id, amount_cents, currency, email, name, status,
           stripe_session_id, stripe_payment_intent_id, created_at, updated_at
    FROM donations WHERE id = $1
    `
	var d Donation
	err := r.pool.QueryRow(ctx, q, id).Scan(&d.ID, &d.AmountCents, &d.Currency, &d.Email, &d.Name, &d.Status, &d.StripeSessionID, &d.StripePaymentIntentID, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get donation: %w", err)
	}
	return &d, nil
}
