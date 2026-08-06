package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/jackc/pgx/v5/pgxpool"
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
		log.Fatalf("Failed to connect DB: %v", err)
	}
	defer pool.Close()

	var productID string
	err = pool.QueryRow(ctx, "SELECT id FROM products WHERE slug = $1", "netflix-premium-4k-1-month").Scan(&productID)
	if err != nil {
		log.Fatalf("Failed to find Netflix product: %v", err)
	}

	for i := 1; i <= 10; i++ {
		email := fmt.Sprintf("netflix_user_%d@stream.com", i)
		password := fmt.Sprintf("nf_pass_sim_%d", i)
		_, err := pool.Exec(ctx, `
			INSERT INTO product_stocks (product_id, email, password_encrypted, status)
			VALUES ($1, $2, $3, 'AVAILABLE')
		`, productID, email, password)

		if err != nil {
			log.Printf("Error adding stock %d: %v", i, err)
		}
	}

	var count int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_stocks WHERE product_id = $1 AND status = 'AVAILABLE'", productID).Scan(&count)

	fmt.Printf("Berhasil menambahkan 10 stok akun Netflix! Total stok tersedia sekarang: %d pcs.\n", count)
}
