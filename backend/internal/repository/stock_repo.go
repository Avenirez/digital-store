package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Stock represents a single digital credential stock item.
type Stock struct {
	ID                string `json:"id"`
	ProductID         string `json:"product_id"`
	Email             string `json:"email"`
	PasswordEncrypted string `json:"-"`         // Never exposed in JSON
	Password          string `json:"password,omitempty"` // Decrypted, set by service layer
	AdditionalInfo    string `json:"additional_info,omitempty"`
	Status            string `json:"status"`
	OrderID           string `json:"order_id,omitempty"`
	SoldAt            string `json:"sold_at,omitempty"`
}

// Stock statuses
const (
	StockStatusAvailable = "AVAILABLE"
	StockStatusReserved  = "RESERVED"
	StockStatusSold      = "SOLD"
)

// StockRepo handles all stock-related database operations.
type StockRepo struct {
	db *pgxpool.Pool
}

// NewStockRepo creates a new StockRepo.
func NewStockRepo(db *pgxpool.Pool) *StockRepo {
	return &StockRepo{db: db}
}

// CountAvailable returns the number of AVAILABLE stocks for a given product.
func (r *StockRepo) CountAvailable(ctx context.Context, productID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM product_stocks WHERE product_id = $1 AND status = 'AVAILABLE'`,
		productID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("stock_repo.CountAvailable: %w", err)
	}
	return count, nil
}

// ReserveOne atomically reserves a single AVAILABLE stock for an order using
// PostgreSQL's FOR UPDATE SKIP LOCKED to prevent race conditions.
// Returns the reserved stock ID, or pgx.ErrNoRows if none available.
func (r *StockRepo) ReserveOne(ctx context.Context, tx pgx.Tx, productID, orderID string) (*Stock, error) {
	query := `
		UPDATE product_stocks
		SET status = 'RESERVED', order_id = $1
		WHERE id = (
			SELECT id FROM product_stocks
			WHERE product_id = $2 AND status = 'AVAILABLE'
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, product_id, email, password_encrypted, COALESCE(additional_info, ''), status
	`

	var s Stock
	err := tx.QueryRow(ctx, query, orderID, productID).Scan(
		&s.ID, &s.ProductID, &s.Email, &s.PasswordEncrypted, &s.AdditionalInfo, &s.Status,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("stock_repo.ReserveOne: %w", err)
	}
	s.OrderID = orderID
	return &s, nil
}

// MarkSoldByOrderID transitions all RESERVED stocks for an order to SOLD status.
func (r *StockRepo) MarkSoldByOrderID(ctx context.Context, orderID string) (int64, error) {
	query := `
		UPDATE product_stocks
		SET status = 'SOLD', sold_at = NOW()
		WHERE order_id = $1 AND status = 'RESERVED'
	`
	tag, err := r.db.Exec(ctx, query, orderID)
	if err != nil {
		return 0, fmt.Errorf("stock_repo.MarkSoldByOrderID: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ReleaseByOrderID reverts RESERVED stocks back to AVAILABLE (used when orders expire).
func (r *StockRepo) ReleaseByOrderID(ctx context.Context, orderID string) (int64, error) {
	query := `
		UPDATE product_stocks
		SET status = 'AVAILABLE', order_id = NULL
		WHERE order_id = $1 AND status = 'RESERVED'
	`
	tag, err := r.db.Exec(ctx, query, orderID)
	if err != nil {
		return 0, fmt.Errorf("stock_repo.ReleaseByOrderID: %w", err)
	}
	return tag.RowsAffected(), nil
}

// GetByOrderID fetches all stock items assigned to a specific order (for credential delivery).
func (r *StockRepo) GetByOrderID(ctx context.Context, orderID string) ([]Stock, error) {
	query := `
		SELECT id, product_id, email, password_encrypted, COALESCE(additional_info, ''),
			   status, COALESCE(order_id::text, ''), COALESCE(sold_at::text, '')
		FROM product_stocks
		WHERE order_id = $1
		ORDER BY email
	`

	rows, err := r.db.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("stock_repo.GetByOrderID: %w", err)
	}
	defer rows.Close()

	var stocks []Stock
	for rows.Next() {
		var s Stock
		if err := rows.Scan(
			&s.ID, &s.ProductID, &s.Email, &s.PasswordEncrypted,
			&s.AdditionalInfo, &s.Status, &s.OrderID, &s.SoldAt,
		); err != nil {
			return nil, fmt.Errorf("stock_repo.GetByOrderID scan: %w", err)
		}
		stocks = append(stocks, s)
	}
	return stocks, rows.Err()
}

// BulkInsertItem represents a single item for bulk stock insertion.
type BulkInsertItem struct {
	Email             string
	PasswordEncrypted string
	AdditionalInfo    string
}

// BulkInsert batch-inserts multiple stock items for a product.
// Passwords should already be AES-encrypted before calling this method.
func (r *StockRepo) BulkInsert(ctx context.Context, productID string, items []BulkInsertItem) (int64, error) {
	if len(items) == 0 {
		return 0, nil
	}

	// Use COPY protocol for maximum throughput on large batches
	query := `
		INSERT INTO product_stocks (product_id, email, password_encrypted, additional_info, status)
		VALUES ($1, $2, $3, $4, 'AVAILABLE')
	`

	batch := &pgx.Batch{}
	for _, item := range items {
		batch.Queue(query, productID, item.Email, item.PasswordEncrypted, item.AdditionalInfo)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	var totalInserted int64
	for range items {
		tag, err := br.Exec()
		if err != nil {
			return totalInserted, fmt.Errorf("stock_repo.BulkInsert: %w", err)
		}
		totalInserted += tag.RowsAffected()
	}

	return totalInserted, nil
}
