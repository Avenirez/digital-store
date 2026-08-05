package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Product represents a product row from the database.
type Product struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	Description string  `json:"description"`
	PriceIDR    float64 `json:"price_idr"`
	ImageURL    string  `json:"image_url,omitempty"`
	IsActive    bool    `json:"is_active"`
	StockCount  int     `json:"stock_count"` // Computed: live available stock
	CreatedAt   string  `json:"created_at"`
}

// ProductRepo handles all product-related database operations.
type ProductRepo struct {
	db *pgxpool.Pool
}

// NewProductRepo creates a new ProductRepo.
func NewProductRepo(db *pgxpool.Pool) *ProductRepo {
	return &ProductRepo{db: db}
}

// ListActive returns all active products with their live available stock count.
// Products are ordered by creation date (newest first).
func (r *ProductRepo) ListActive(ctx context.Context) ([]Product, error) {
	query := `
		SELECT
			p.id, p.title, p.slug, p.description, p.price_idr, COALESCE(p.image_url, ''),
			p.is_active, p.created_at::text,
			COALESCE((
				SELECT COUNT(*) FROM product_stocks ps
				WHERE ps.product_id = p.id AND ps.status = 'AVAILABLE'
			), 0) AS stock_count
		FROM products p
		WHERE p.is_active = TRUE
		ORDER BY p.created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("product_repo.ListActive: %w", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(
			&p.ID, &p.Title, &p.Slug, &p.Description, &p.PriceIDR, &p.ImageURL,
			&p.IsActive, &p.CreatedAt, &p.StockCount,
		); err != nil {
			return nil, fmt.Errorf("product_repo.ListActive scan: %w", err)
		}
		products = append(products, p)
	}

	return products, rows.Err()
}

// GetBySlug returns a single product by its URL slug, including stock count.
// Returns pgx.ErrNoRows if the product is not found.
func (r *ProductRepo) GetBySlug(ctx context.Context, slug string) (*Product, error) {
	query := `
		SELECT
			p.id, p.title, p.slug, p.description, p.price_idr, COALESCE(p.image_url, ''),
			p.is_active, p.created_at::text,
			COALESCE((
				SELECT COUNT(*) FROM product_stocks ps
				WHERE ps.product_id = p.id AND ps.status = 'AVAILABLE'
			), 0) AS stock_count
		FROM products p
		WHERE p.slug = $1 AND p.is_active = TRUE
	`

	var p Product
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&p.ID, &p.Title, &p.Slug, &p.Description, &p.PriceIDR, &p.ImageURL,
		&p.IsActive, &p.CreatedAt, &p.StockCount,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("product_repo.GetBySlug: %w", err)
	}

	return &p, nil
}

// GetByID returns a single product by its UUID.
func (r *ProductRepo) GetByID(ctx context.Context, id string) (*Product, error) {
	query := `
		SELECT
			p.id, p.title, p.slug, p.description, p.price_idr, COALESCE(p.image_url, ''),
			p.is_active, p.created_at::text,
			COALESCE((
				SELECT COUNT(*) FROM product_stocks ps
				WHERE ps.product_id = p.id AND ps.status = 'AVAILABLE'
			), 0) AS stock_count
		FROM products p
		WHERE p.id = $1
	`

	var p Product
	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.Title, &p.Slug, &p.Description, &p.PriceIDR, &p.ImageURL,
		&p.IsActive, &p.CreatedAt, &p.StockCount,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("product_repo.GetByID: %w", err)
	}

	return &p, nil
}
