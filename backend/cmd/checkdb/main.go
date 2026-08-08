package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	// 1. Check all databases on localhost
	baseURL := "postgres://postgres:password@127.0.0.1:5432/postgres?sslmode=disable"
	basePool, err := pgxpool.New(ctx, baseURL)
	if err != nil {
		log.Fatalf("Failed to connect to postgres base DB: %v", err)
	}

	rows, err := basePool.Query(ctx, "SELECT datname FROM pg_database WHERE datistemplate = false;")
	if err == nil {
		fmt.Println("--- Databases in PostgreSQL ---")
		for rows.Next() {
			var name string
			rows.Scan(&name)
			fmt.Println("DB:", name)
		}
		rows.Close()
	}
	basePool.Close()

	// 2. Check products in digitalstore DB
	dbURL := "postgres://postgres:password@127.0.0.1:5432/digitalstore?sslmode=disable"
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to digitalstore DB: %v", err)
	}
	defer pool.Close()

	pRows, err := pool.Query(ctx, `
		SELECT p.id, p.title, p.slug, p.price_idr, COUNT(s.id) AS avail_stocks
		FROM products p
		LEFT JOIN product_stocks s ON s.product_id = p.id AND s.status = 'AVAILABLE'
		GROUP BY p.id, p.title, p.slug, p.price_idr;
	`)
	if err != nil {
		log.Fatalf("Failed to query products: %v", err)
	}
	defer pRows.Close()

	fmt.Println("\n--- Products in 'digitalstore' Database ---")
	for pRows.Next() {
		var id, title, slug string
		var price float64
		var count int
		pRows.Scan(&id, &title, &slug, &price, &count)
		fmt.Printf("- Product: %s | Slug: %s | Price: Rp%.0f | Available Stock: %d\n", title, slug, price, count)
	}
}
