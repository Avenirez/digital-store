package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/jackc/pgx/v5/pgxpool"
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
			Title:       "ChatGPT Plus (1 Bulan)",
			Slug:        "chatgpt-plus-1-month",
			Description: "Akses ChatGPT Plus versi GPT-4o, DALL-E 3, dan fitur analisis data tingkat lanjut selama 30 hari.",
			PriceIDR:    45000,
			ImageURL:    "https://images.unsplash.com/photo-1677442136019-21780efad99a?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"user1@gpt.com:pass123", "user2@gpt.com:pass123"},
		},
		{
			Title:       "Netflix Premium 4K UHD (1 Bulan)",
			Slug:        "netflix-premium-4k-1-month",
			Description: "Akun Netflix Premium 4K Ultra HD Privat/Shared profile, garansi penuh 30 hari.",
			PriceIDR:    35000,
			ImageURL:    "https://images.unsplash.com/photo-1574375927938-d5a98e8ffe85?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"nf1@stream.com:pass456", "nf2@stream.com:pass456", "nf3@stream.com:pass456"},
		},
		{
			Title:       "Spotify Premium Individual (1 Bulan)",
			Slug:        "spotify-premium-1-month",
			Description: "Mendengarkan musik tanpa iklan, bisa download offline & bebas skip lagu.",
			PriceIDR:    18000,
			ImageURL:    "https://images.unsplash.com/photo-1614680376593-902f749f7bc2?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"sp1@music.com:sp123", "sp2@music.com:sp123"},
		},
		{
			Title:       "YouTube Premium (1 Bulan)",
			Slug:        "youtube-premium-1-month",
			Description: "Bebas iklan YouTube & YouTube Music, latar belakang & unduhan offline.",
			PriceIDR:    15000,
			ImageURL:    "https://images.unsplash.com/photo-1611162617213-7d7a39e9b1d7?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"yt1@yt.com:ytpass1", "yt2@yt.com:ytpass2"},
		},
		{
			Title:       "Canva Pro Lifetime / 1 Tahun",
			Slug:        "canva-pro-1-year",
			Description: "Akses semua elemen premium Canva, hilangkan background, brand kit, dan AI design generator.",
			PriceIDR:    25000,
			ImageURL:    "https://images.unsplash.com/photo-1626785774573-4b799315345d?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"canva1@design.com:cv123", "canva2@design.com:cv123"},
		},
		{
			Title:       "Claude AI Pro (1 Bulan)",
			Slug:        "claude-ai-pro-1-month",
			Description: "Akses ke Claude 3.5 Sonnet & Opus dengan batas pesan 5x lebih tinggi dari versi gratis.",
			PriceIDR:    55000,
			ImageURL:    "https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"c1@claude.ai:cp123"},
		},
		{
			Title:       "Midjourney Basic Plan (1 Bulan)",
			Slug:        "midjourney-basic-1-month",
			Description: "Generasi gambar AI kualitas terbaik dengan 200 menit waktu GPU cepat per bulan.",
			PriceIDR:    65000,
			ImageURL:    "https://images.unsplash.com/photo-1620712943543-bcc4688e7485?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"mj1@art.com:mj123", "mj2@art.com:mj123"},
		},
		{
			Title:       "Grammarly Premium (1 Bulan)",
			Slug:        "grammarly-premium-1-month",
			Description: "Pemeriksa tata bahasa bahasa Inggris tingkat lanjut, kejelasan kalimat, dan deteksi plagiarisme.",
			PriceIDR:    20000,
			ImageURL:    "https://images.unsplash.com/photo-1455390582262-044cdead277a?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"gram1@write.com:gr123"},
		},
		{
			Title:       "CapCut Pro PC / Mobile (1 Bulan)",
			Slug:        "capcut-pro-1-month",
			Description: "Akses seluruh fitur editor video premium CapCut: animasi pro, efek AI & tanpa watermark.",
			PriceIDR:    22000,
			ImageURL:    "/images/capcut.webp",
			Stocks:      []string{"cc1@video.com:cc123", "cc2@video.com:cc123"},
		},
		{
			Title:       "Disney+ Hotstar Premium (1 Bulan)",
			Slug:        "disney-plus-hotstar-1-month",
			Description: "Streaming film Disney, Marvel, Star Wars & siaran olahraga dalam kualitas HD.",
			PriceIDR:    30000,
			ImageURL:    "https://images.unsplash.com/photo-1586899028174-e7098604235b?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"disney1@stream.com:d123"},
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

		for _, s := range p.Stocks {
			_, err := pool.Exec(ctx, `
				INSERT INTO product_stocks (product_id, email, password_encrypted, status)
				VALUES ($1, $2, $3, 'AVAILABLE')
			`, productID, s, "encrypted_dummy_password")
			if err != nil {
				log.Printf("Error inserting stock for %s: %v", p.Title, err)
			}
		}

		fmt.Printf("Added product: %s (%s)\n", p.Title, productID)
	}

	fmt.Println("\nSuccessfully seeded 10 products with stock data!")
}
