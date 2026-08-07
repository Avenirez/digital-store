package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type ProductSeed struct {
	Title       string
	Slug        string
	Description string
	PriceIDR    float64
	ImageURL    string
	Stocks      []string
}

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

	products := []ProductSeed{
		{
			Title:       "Capcut Premium (7 Hari)",
			Slug:        "capcut-premium-7-days",
			Description: "Akun Capcut Premium 7 Hari privat, akses penuh semua fitur efek & template pro.",
			PriceIDR:    1000,
			ImageURL:    "/images/capcut.webp",
			Stocks:      []string{},
		},
	}

	for _, p := range products {
		var productID string
		err := pool.QueryRow(ctx, `
			INSERT INTO products (title, slug, description, price_idr, image_url, is_active)
			VALUES ($1, $2, $3, $4, $5, true)
			ON CONFLICT (slug) DO UPDATE SET 
				title = EXCLUDED.title,
				description = EXCLUDED.description,
				price_idr = EXCLUDED.price_idr,
				image_url = EXCLUDED.image_url
			RETURNING id
		`, p.Title, p.Slug, p.Description, p.PriceIDR, p.ImageURL).Scan(&productID)

		if err != nil {
			log.Printf("Error inserting product %s: %v", p.Title, err)
			continue
		}
	}

	fmt.Println("Ensured Capcut product entry exists. Use POST /api/v1/admin/stocks/bulk to insert account credentials securely.")
}
