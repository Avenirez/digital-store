package database

import (
	"context"
	"log"
	"my-digital-store/backend/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureDefaultCapcutStock ensures that the default Capcut product entry exists in the database.
func EnsureDefaultCapcutStock(ctx context.Context, pool *pgxpool.Pool, cryptoSvc *service.CryptoService) {
	// Ensure Capcut product exists without deactivating other products
	var productID string
	productTitle := "Capcut Premium (7 Hari)"
	productSlug := "capcut-premium-7-days"
	productDesc := "Akun Capcut Premium 7 Hari privat, akses penuh semua fitur efek & template pro."
	priceIDR := 1000.00
	imageURL := "/images/capcut.webp"

	err := pool.QueryRow(ctx, `
		INSERT INTO products (title, slug, description, price_idr, image_url, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
		ON CONFLICT (slug) DO UPDATE SET 
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			price_idr = EXCLUDED.price_idr,
			image_url = EXCLUDED.image_url
		RETURNING id
	`, productTitle, productSlug, productDesc, priceIDR, imageURL).Scan(&productID)

	if err != nil {
		log.Printf("AutoSeed: Failed to insert/ensure Capcut product: %v", err)
		return
	}

	var availableStock int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_stocks WHERE product_id = $1 AND status = 'AVAILABLE'", productID).Scan(&availableStock)
	log.Printf("AutoSeed: Capcut product check completed (ID: %s). Available stocks: %d", productID, availableStock)
}
