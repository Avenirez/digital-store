package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RestockSubscription represents a restock notification subscription.
type RestockSubscription struct {
	ID         string `json:"id"`
	ProductID  string `json:"product_id"`
	Email      string `json:"email"`
	IsNotified bool   `json:"is_notified"`
	CreatedAt  string `json:"created_at"`
}

// RestockRepo handles restock subscription database operations.
type RestockRepo struct {
	db *pgxpool.Pool
}

// NewRestockRepo creates a new RestockRepo.
func NewRestockRepo(db *pgxpool.Pool) *RestockRepo {
	return &RestockRepo{db: db}
}

// Subscribe adds an email to the restock notification list for a product.
// Uses ON CONFLICT to ensure idempotency (same email + product won't duplicate).
// If already notified, resets the notification flag so they get notified again.
func (r *RestockRepo) Subscribe(ctx context.Context, productID, email string) error {
	query := `
		INSERT INTO restock_subscriptions (product_id, email)
		VALUES ($1, $2)
		ON CONFLICT (product_id, email) DO UPDATE
		SET is_notified = FALSE, created_at = NOW()
	`

	_, err := r.db.Exec(ctx, query, productID, email)
	if err != nil {
		return fmt.Errorf("restock_repo.Subscribe: %w", err)
	}
	return nil
}

// GetPendingByProduct fetches all un-notified subscribers for a given product.
func (r *RestockRepo) GetPendingByProduct(ctx context.Context, productID string) ([]RestockSubscription, error) {
	query := `
		SELECT id, product_id, email, is_notified, created_at::text
		FROM restock_subscriptions
		WHERE product_id = $1 AND is_notified = FALSE
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("restock_repo.GetPendingByProduct: %w", err)
	}
	defer rows.Close()

	var subs []RestockSubscription
	for rows.Next() {
		var s RestockSubscription
		if err := rows.Scan(&s.ID, &s.ProductID, &s.Email, &s.IsNotified, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("restock_repo.GetPendingByProduct scan: %w", err)
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

// MarkNotified marks a list of subscription IDs as notified.
func (r *RestockRepo) MarkNotified(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	query := `
		UPDATE restock_subscriptions
		SET is_notified = TRUE
		WHERE id = ANY($1)
	`

	_, err := r.db.Exec(ctx, query, ids)
	if err != nil {
		return fmt.Errorf("restock_repo.MarkNotified: %w", err)
	}
	return nil
}
