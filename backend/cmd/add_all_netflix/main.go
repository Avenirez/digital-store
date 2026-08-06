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

	// Find products matching Netflix (including 'Netflix Premium 1 Bulan')
	rows, err := pool.Query(ctx, "SELECT id, title, slug FROM products WHERE title ILIKE '%netflix%' OR slug ILIKE '%netflix%'")
	if err != nil {
		log.Fatalf("Query error: %v", err)
	}
	defer rows.Close()

	type Prod struct {
		ID    string
		Title string
		Slug  string
	}
	var list []Prod
	for rows.Next() {
		var p Prod
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug); err == nil {
			list = append(list, p)
		}
	}

	if len(list) == 0 {
		log.Println("No netflix products found in DB.")
		return
	}

	for _, p := range list {
		fmt.Printf("Adding stock to product: %s (%s)\n", p.Title, p.Slug)
		for i := 1; i <= 10; i++ {
			email := fmt.Sprintf("nf_%s_%d@stream.com", p.Slug, i)
			password := fmt.Sprintf("pass_%d", i)
			_, _ = pool.Exec(ctx, `
				INSERT INTO product_stocks (product_id, email, password_encrypted, status)
				VALUES ($1, $2, $3, 'AVAILABLE')
			`, p.ID, email, password)
		}
	}

	fmt.Println("Done adding 10 stocks to all Netflix products!")
}
