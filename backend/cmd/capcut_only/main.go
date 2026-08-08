package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:password@127.0.0.1:5432/digitalstore?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer pool.Close()

	// 1. Deactivate non-Capcut products
	res, err := pool.Exec(ctx, "UPDATE products SET is_active = false WHERE slug != 'capcut-premium-7-days'")
	if err != nil {
		log.Printf("Error deactivating products: %v", err)
	} else {
		fmt.Printf("✅ Deactivated %d non-Capcut products.\n", res.RowsAffected())
	}

	// 2. Ensure Capcut product is active
	var productID string
	err = pool.QueryRow(ctx, `
		INSERT INTO products (title, slug, description, price_idr, image_url, is_active)
		VALUES ('Capcut Premium (7 Hari)', 'capcut-premium-7-days', 'Akun Capcut Premium 7 Hari privat, akses penuh semua fitur efek & template pro.', 1000, '/images/capcut.webp', true)
		ON CONFLICT (slug) DO UPDATE SET is_active = true
		RETURNING id
	`).Scan(&productID)

	if err != nil {
		log.Fatalf("Error ensuring Capcut product: %v", err)
	}

	fmt.Printf("✅ Capcut Premium product active (ID: %s).\n", productID)
}
