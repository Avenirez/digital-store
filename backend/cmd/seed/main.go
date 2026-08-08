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
	
	// First try connecting to postgres DB to create digitalstore if needed
	baseURL := "postgres://postgres:password@127.0.0.1:5432/postgres?sslmode=disable"
	basePool, err := pgxpool.New(ctx, baseURL)
	if err != nil {
		log.Printf("Warning: failed to connect to base postgres DB: %v", err)
	} else {
		_, _ = basePool.Exec(ctx, "CREATE DATABASE digitalstore;")
		basePool.Close()
	}

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

	// Seed sample products if empty
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM products").Scan(&count)
	if err == nil && count == 0 {
		seedQuery := `
		INSERT INTO products (title, slug, description, price_idr, image_url) VALUES
		('Netflix Premium 1 Bulan', 'netflix-premium-1-bulan', 'Akun Netflix Premium UHD 4K, 1 profil 1 pengguna, Garansi 30 hari.', 45000, 'https://images.unsplash.com/photo-1574375927938-d5a98e8ffe85?w=500&auto=format&fit=crop&q=80'),
		('Spotify Premium 1 Bulan', 'spotify-premium-1-bulan', 'Akun Spotify Premium Individu/Family Plan, Tanpa iklan, Bebas download.', 25000, 'https://images.unsplash.com/photo-1614680376593-902f749f7b9c?w=500&auto=format&fit=crop&q=80'),
		('ChatGPT Plus 1 Bulan', 'chatgpt-plus-1-bulan', 'Akses ChatGPT Plus dengan GPT-4o & DALL-E 3. Privat & Garansi full 30 hari.', 95000, 'https://images.unsplash.com/photo-1677442136019-21780efad99a?w=500&auto=format&fit=crop&q=80');
		`
		_, err = pool.Exec(ctx, seedQuery)
		if err != nil {
			log.Printf("Failed to seed sample products: %v", err)
		} else {
			fmt.Println("✅ Sample products seeded successfully!")
		}
	}
}
