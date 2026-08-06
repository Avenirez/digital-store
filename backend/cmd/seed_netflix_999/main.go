package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"my-digital-store/backend/internal/service"

	"github.com/joho/godotenv"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	_ = godotenv.Load("../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:password@localhost:5432/digitalstore?sslmode=disable"
	}
	aesKey := os.Getenv("AES_KEY")
	if aesKey == "" {
		aesKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}

	cryptoSvc, err := service.NewCryptoService([]byte(aesKey[:32]))
	if err != nil {
		log.Fatalf("Failed to init crypto: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect DB: %v", err)
	}
	defer pool.Close()

	// 1. Find or create target Netflix product
	targetSlug := "netflix-premium-1-bulan"
	var targetProductID string
	err = pool.QueryRow(ctx, "SELECT id FROM products WHERE slug = $1", targetSlug).Scan(&targetProductID)
	if err != nil {
		// Create if not exists
		err = pool.QueryRow(ctx, `
			INSERT INTO products (title, slug, description, price_idr, image_url, is_active)
			VALUES ('Netflix Premium 1 Bulan', 'netflix-premium-1-bulan', 'Akun Netflix Premium 4K Ultra HD Privat/Shared profile 1 Bulan.', 45000, 'https://images.unsplash.com/photo-1574375927938-d5a98e8ffe85?auto=format&fit=crop&w=600&q=80', true)
			RETURNING id
		`).Scan(&targetProductID)
		if err != nil {
			log.Fatalf("Failed to insert target Netflix product: %v", err)
		}
	} else {
		// Ensure title and price match screenshot
		_, _ = pool.Exec(ctx, "UPDATE products SET title = 'Netflix Premium 1 Bulan', price_idr = 45000, is_active = true WHERE id = $1", targetProductID)
	}

	// 2. Delete ALL other products & their stocks
	_, err = pool.Exec(ctx, "DELETE FROM restock_subscriptions WHERE product_id != $1", targetProductID)
	_, err = pool.Exec(ctx, "DELETE FROM product_stocks WHERE product_id != $1", targetProductID)
	_, err = pool.Exec(ctx, "DELETE FROM orders WHERE product_id != $1", targetProductID)
	_, err = pool.Exec(ctx, "DELETE FROM products WHERE id != $1", targetProductID)

	if err != nil {
		log.Printf("Warning deleting non-target products: %v", err)
	}

	// 3. Clear existing stocks for this Netflix product and add 999 AES-encrypted stocks
	_, _ = pool.Exec(ctx, "DELETE FROM product_stocks WHERE product_id = $1 AND status = 'AVAILABLE'", targetProductID)

	fmt.Println("Generating 999 AES-encrypted stocks for Netflix Premium 1 Bulan...")

	batchSize := 100
	inserted := 0

	for i := 1; i <= 999; i++ {
		email := fmt.Sprintf("netflix_user_%d@digitalstore.com", i)
		rawPass := fmt.Sprintf("NfPass2026#%03d", i)

		encPass, err := cryptoSvc.Encrypt(rawPass)
		if err != nil {
			log.Printf("Encrypt error: %v", err)
			continue
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO product_stocks (product_id, email, password_encrypted, status)
			VALUES ($1, $2, $3, 'AVAILABLE')
		`, targetProductID, email, encPass)
		if err != nil {
			log.Printf("Insert stock error at %d: %v", i, err)
		} else {
			inserted++
		}

		if inserted%batchSize == 0 {
			fmt.Printf("Inserted %d / 999 stocks...\n", inserted)
		}
	}

	fmt.Printf("\nSUCCESS! Retained only 'Netflix Premium 1 Bulan' (ID: %s) and added %d AVAILABLE stocks with valid AES encryption.\n", targetProductID, inserted)
}
