package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load("../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:password@localhost:5432/digitalstore?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer pool.Close()

	// 1. Deactivate all non-Capcut products and remove their available stocks
	_, _ = pool.Exec(ctx, "UPDATE products SET is_active = false WHERE slug != $1", "capcut-premium-7-days")
	_, _ = pool.Exec(ctx, "DELETE FROM product_stocks WHERE product_id IN (SELECT id FROM products WHERE slug != $1)", "capcut-premium-7-days")

	// 2. Ensure Capcut product exists
	var productID string
	productTitle := "Capcut Premium (7 Hari)"
	productSlug := "capcut-premium-7-days"
	productDesc := "Akun Capcut Premium 7 Hari privat, akses penuh semua fitur efek & template pro."
	priceIDR := 1000.00
	imageURL := "/images/capcut.webp"

	err = pool.QueryRow(ctx, `
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
		log.Fatalf("Failed to create/get product %s: %v", productTitle, err)
	}

	// Hardcoded credentials have been removed for security.
	// Use POST /api/v1/admin/stocks/bulk to import product stocks securely.
	fmt.Println("Gunakan endpoint admin /api/v1/admin/stocks/bulk untuk menambahkan stok produk secara aman.")
}
