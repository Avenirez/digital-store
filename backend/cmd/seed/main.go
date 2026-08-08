package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductItem struct {
	Title       string
	Slug        string
	Description string
	PriceIDR    float64
	ImageURL    string
	InitialStock int
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:password@127.0.0.1:5432/digitalstore?sslmode=disable"
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	sqlFile, err := os.ReadFile("migrations/001_init.sql")
	if err != nil {
		log.Fatalf("Failed to read migration file: %v", err)
	}

	_, err = pool.Exec(ctx, string(sqlFile))
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	fmt.Println("✅ Database migration applied successfully!")

	products := []ProductItem{
		{
			Title:       "Capcut Premium (7 Hari)",
			Slug:        "capcut-premium-7-days",
			Description: "Akun Capcut Premium 7 Hari privat, akses penuh semua fitur efek & template pro.",
			PriceIDR:    1000,
			ImageURL:    "/images/capcut.webp",
			InitialStock: 10,
		},
		{
			Title:       "Netflix Premium 1 Bulan",
			Slug:        "netflix-premium-1-bulan",
			Description: "Akun Netflix Premium UHD 4K, 1 profil 1 pengguna, Garansi full 30 hari.",
			PriceIDR:    45000,
			ImageURL:    "https://images.unsplash.com/photo-1574375927938-d5a98e8ffe85?w=500&auto=format&fit=crop&q=80",
			InitialStock: 8,
		},
		{
			Title:       "Spotify Premium 1 Bulan",
			Slug:        "spotify-premium-1-bulan",
			Description: "Akun Spotify Premium Individu/Family Plan, Tanpa iklan, Bebas download offline.",
			PriceIDR:    25000,
			ImageURL:    "https://images.unsplash.com/photo-1614680376593-902f749f7b9c?w=500&auto=format&fit=crop&q=80",
			InitialStock: 12,
		},
		{
			Title:       "ChatGPT Plus 1 Bulan",
			Slug:        "chatgpt-plus-1-bulan",
			Description: "Akses ChatGPT Plus dengan GPT-4o & DALL-E 3. Privat & Garansi full 30 hari.",
			PriceIDR:    95000,
			ImageURL:    "https://images.unsplash.com/photo-1677442136019-21780efad99a?w=500&auto=format&fit=crop&q=80",
			InitialStock: 5,
		},
		{
			Title:       "YouTube Premium 1 Bulan",
			Slug:        "youtube-premium-1-bulan",
			Description: "Akun YouTube Premium 1 Bulan, Bebas iklan & termasuk YouTube Music Premium.",
			PriceIDR:    15000,
			ImageURL:    "https://images.unsplash.com/photo-1611162617213-7d7a39e9b1d7?w=500&auto=format&fit=crop&q=80",
			InitialStock: 15,
		},
		{
			Title:       "Canva Pro 1 Tahun",
			Slug:        "canva-pro-lifetime-1-tahun",
			Description: "Akun Canva Pro 1 Tahun, Akses penuh semua elemen & template desain premium.",
			PriceIDR:    25000,
			ImageURL:    "https://images.unsplash.com/photo-1626785774573-4b799315345d?w=500&auto=format&fit=crop&q=80",
			InitialStock: 20,
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
				image_url = EXCLUDED.image_url,
				is_active = true
			RETURNING id
		`, p.Title, p.Slug, p.Description, p.PriceIDR, p.ImageURL).Scan(&productID)

		if err != nil {
			log.Printf("Error inserting product %s: %v", p.Title, err)
			continue
		}

		// Check current stock count
		var currentStock int
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_stocks WHERE product_id = $1 AND status = 'AVAILABLE'", productID).Scan(&currentStock)

		if currentStock < p.InitialStock {
			needed := p.InitialStock - currentStock
			for i := 1; i <= needed; i++ {
				email := fmt.Sprintf("user_%s_%d@store.local", p.Slug, currentStock+i)
				pass := "EncryptedPass123!"
				_, _ = pool.Exec(ctx, `
					INSERT INTO product_stocks (product_id, email, password_encrypted, status)
					VALUES ($1, $2, $3, 'AVAILABLE')
				`, productID, email, pass)
			}
			fmt.Printf("✅ Added %d initial stocks for %s\n", needed, p.Title)
		}
	}

	fmt.Println("✅ All products and initial stocks seeded successfully!")
}
