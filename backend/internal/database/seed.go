package database

import (
	"context"
	"log"
	"my-digital-store/backend/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountSeed struct {
	Email    string
	Password string
}

// EnsureDefaultCapcutStock ensures that the Capcut product and its 5 default accounts exist in the database.
func EnsureDefaultCapcutStock(ctx context.Context, pool *pgxpool.Pool, cryptoSvc *service.CryptoService) {
	// 1. Deactivate non-Capcut products
	_, _ = pool.Exec(ctx, "UPDATE products SET is_active = false WHERE slug != 'capcut-premium-7-days'")

	// 2. Ensure Capcut product exists
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
			image_url = EXCLUDED.image_url,
			is_active = true
		RETURNING id
	`, productTitle, productSlug, productDesc, priceIDR, imageURL).Scan(&productID)

	if err != nil {
		log.Printf("AutoSeed: Failed to insert/ensure Capcut product: %v", err)
		return
	}

	accounts := []AccountSeed{
		{Email: "blackbutterfly564@saovangtiles.site", Password: "masuk123"},
		{Email: "crazyswan547@submitreports.com", Password: "masuk123"},
		{Email: "heavymouse584@mailfirefly.com", Password: "masuk123"},
		{Email: "smallcat555@saovangtiles.site", Password: "masuk123"},
		{Email: "beautifullion284@phuongnhicare.com", Password: "masuk123"},
	}

	insertedCount := 0
	for _, acc := range accounts {
		// Check if stock with this email already exists for this product
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM product_stocks WHERE product_id = $1 AND email = $2
			)
		`, productID, acc.Email).Scan(&exists)

		if err != nil || exists {
			continue
		}

		encryptedPass, err := cryptoSvc.Encrypt(acc.Password)
		if err != nil {
			log.Printf("AutoSeed: Error encrypting password for %s: %v", acc.Email, err)
			continue
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO product_stocks (product_id, email, password_encrypted, status)
			VALUES ($1, $2, $3, 'AVAILABLE')
		`, productID, acc.Email, encryptedPass)

		if err != nil {
			log.Printf("AutoSeed: Error inserting stock %s: %v", acc.Email, err)
		} else {
			insertedCount++
		}
	}

	var totalStock int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_stocks WHERE product_id = $1 AND status = 'AVAILABLE'", productID).Scan(&totalStock)
	log.Printf("AutoSeed: Capcut Premium stock check completed. Available stocks: %d (newly inserted: %d)", totalStock, insertedCount)
}
