package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Order represents an order row from the database.
type Order struct {
	ID            string  `json:"id"`
	OrderNumber   string  `json:"order_number"`
	CustomerEmail string  `json:"customer_email"`
	ProductID     string  `json:"product_id"`
	Quantity      int     `json:"quantity"`
	TotalAmount   float64 `json:"total_amount"`
	Status        string  `json:"status"`
	PaymentURL    string  `json:"payment_url,omitempty"`
	DuitkuRef     string  `json:"duitku_ref,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// Order statuses
const (
	OrderStatusPending = "PENDING"
	OrderStatusPaid    = "PAID"
	OrderStatusExpired = "EXPIRED"
	OrderStatusFailed  = "FAILED"
)

// OrderRepo handles all order-related database operations.
type OrderRepo struct {
	db *pgxpool.Pool
}

// NewOrderRepo creates a new OrderRepo.
func NewOrderRepo(db *pgxpool.Pool) *OrderRepo {
	return &OrderRepo{db: db}
}

// Create inserts a new order and returns its generated UUID.
func (r *OrderRepo) Create(ctx context.Context, order *Order) error {
	query := `
		INSERT INTO orders (order_number, customer_email, product_id, quantity, total_amount, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at::text, updated_at::text
	`

	err := r.db.QueryRow(ctx, query,
		order.OrderNumber,
		order.CustomerEmail,
		order.ProductID,
		order.Quantity,
		order.TotalAmount,
		order.Status,
	).Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)

	if err != nil {
		return fmt.Errorf("order_repo.Create: %w", err)
	}
	return nil
}

// GetByID fetches a single order by its UUID.
func (r *OrderRepo) GetByID(ctx context.Context, id string) (*Order, error) {
	query := `
		SELECT id, order_number, customer_email, COALESCE(product_id::text, ''),
			   quantity, total_amount, status,
			   COALESCE(snap_token, ''), COALESCE(snap_token, ''),
			   created_at::text, updated_at::text
		FROM orders WHERE id = $1
	`

	var o Order
	err := r.db.QueryRow(ctx, query, id).Scan(
		&o.ID, &o.OrderNumber, &o.CustomerEmail, &o.ProductID,
		&o.Quantity, &o.TotalAmount, &o.Status,
		&o.PaymentURL, &o.DuitkuRef,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("order_repo.GetByID: %w", err)
	}
	return &o, nil
}

// GetByOrderNumber fetches a single order by order number.
func (r *OrderRepo) GetByOrderNumber(ctx context.Context, orderNumber string) (*Order, error) {
	query := `
		SELECT id, order_number, customer_email, COALESCE(product_id::text, ''),
			   quantity, total_amount, status,
			   COALESCE(snap_token, ''), COALESCE(snap_token, ''),
			   created_at::text, updated_at::text
		FROM orders WHERE order_number = $1
	`

	var o Order
	err := r.db.QueryRow(ctx, query, orderNumber).Scan(
		&o.ID, &o.OrderNumber, &o.CustomerEmail, &o.ProductID,
		&o.Quantity, &o.TotalAmount, &o.Status,
		&o.PaymentURL, &o.DuitkuRef,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("order_repo.GetByOrderNumber: %w", err)
	}
	return &o, nil
}

// GetByOrderNumberAndEmail fetches an order for guest lookup (requires both fields to match).
func (r *OrderRepo) GetByOrderNumberAndEmail(ctx context.Context, orderNumber, email string) (*Order, error) {
	query := `
		SELECT id, order_number, customer_email, COALESCE(product_id::text, ''),
			   quantity, total_amount, status,
			   COALESCE(snap_token, ''), COALESCE(snap_token, ''),
			   created_at::text, updated_at::text
		FROM orders
		WHERE order_number = $1 AND customer_email = $2
	`

	var o Order
	err := r.db.QueryRow(ctx, query, orderNumber, email).Scan(
		&o.ID, &o.OrderNumber, &o.CustomerEmail, &o.ProductID,
		&o.Quantity, &o.TotalAmount, &o.Status,
		&o.PaymentURL, &o.DuitkuRef,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("order_repo.GetByOrderNumberAndEmail: %w", err)
	}
	return &o, nil
}

// UpdateStatus transitions an order to a new status.
func (r *OrderRepo) UpdateStatus(ctx context.Context, id, status string) error {
	query := `
		UPDATE orders SET status = $2, updated_at = NOW()
		WHERE id = $1
	`
	tag, err := r.db.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("order_repo.UpdateStatus: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("order_repo.UpdateStatus: order %s not found", id)
	}
	return nil
}

// SetPaymentInfo stores the Duitku payment URL and reference on an order.
func (r *OrderRepo) SetPaymentInfo(ctx context.Context, id, paymentURL, reference string) error {
	query := `
		UPDATE orders SET snap_token = $2, updated_at = NOW()
		WHERE id = $1
	`
	// We store paymentURL in snap_token column for compatibility.
	// The reference is stored in additional_param or we can just use order_number.
	_, err := r.db.Exec(ctx, query, id, paymentURL)
	if err != nil {
		return fmt.Errorf("order_repo.SetPaymentInfo: %w", err)
	}
	return nil
}

// ExpirePendingOrders expires all orders that have been PENDING longer than the
// given threshold duration. Returns the list of expired order IDs so the caller
// can release their reserved stocks.
func (r *OrderRepo) ExpirePendingOrders(ctx context.Context, threshold time.Duration) ([]string, error) {
	query := `
		UPDATE orders
		SET status = 'EXPIRED', updated_at = NOW()
		WHERE status = 'PENDING' AND created_at < NOW() - $1::interval
		RETURNING id
	`

	rows, err := r.db.Query(ctx, query, threshold.String())
	if err != nil {
		return nil, fmt.Errorf("order_repo.ExpirePendingOrders: %w", err)
	}
	defer rows.Close()

	var expiredIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("order_repo.ExpirePendingOrders scan: %w", err)
		}
		expiredIDs = append(expiredIDs, id)
	}

	return expiredIDs, rows.Err()
}
