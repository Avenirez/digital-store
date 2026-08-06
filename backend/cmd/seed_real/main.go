package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"my-digital-store/backend/internal/service"

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

	// Clear old un-encrypted dummy stock
	_, _ = pool.Exec(ctx, "DELETE FROM product_stocks WHERE password_encrypted = 'encrypted_dummy_password' OR password_encrypted LIKE 'pass_%'")

	products := []ProductSeed{
		{
			Title:       "ChatGPT Plus (1 Bulan)",
			Slug:        "chatgpt-plus-1-month",
			Description: "Akses ChatGPT Plus versi GPT-4o, DALL-E 3, dan fitur analisis data tingkat lanjut selama 30 hari.",
			PriceIDR:    45000,
			ImageURL:    "https://images.unsplash.com/photo-1677442136019-21780efad99a?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"gpt_user1@ai.com:ChatGPTPlus#2026", "gpt_user2@ai.com:GPT4oPass#998"},
		},
		{
			Title:       "Netflix Premium 4K UHD (1 Bulan)",
			Slug:        "netflix-premium-4k-1-month",
			Description: "Akun Netflix Premium 4K Ultra HD Privat/Shared profile, garansi penuh 30 hari.",
			PriceIDR:    35000,
			ImageURL:    "https://images.unsplash.com/photo-1574375927938-d5a98e8ffe85?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"nf_prem1@stream.com:NfPass4K#881", "nf_prem2@stream.com:NfPass4K#882", "nf_prem3@stream.com:NfPass4K#883"},
		},
		{
			Title:       "Spotify Premium Individual (1 Bulan)",
			Slug:        "spotify-premium-1-month",
			Description: "Mendengarkan musik tanpa iklan, bisa download offline & bebas skip lagu.",
			PriceIDR:    18000,
			ImageURL:    "https://images.unsplash.com/photo-1614680376593-902f749f7bc2?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"sp_prem1@music.com:SpotifyPass#123", "sp_prem2@music.com:SpotifyPass#456"},
		},
		{
			Title:       "YouTube Premium (1 Bulan)",
			Slug:        "youtube-premium-1-month",
			Description: "Bebas iklan YouTube & YouTube Music, latar belakang & unduhan offline.",
			PriceIDR:    15000,
			ImageURL:    "https://images.unsplash.com/photo-1611162617213-7d7a39e9b1d7?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"yt_premium1@yt.com:YtPassNoAds#1", "yt_premium2@yt.com:YtPassNoAds#2"},
		},
		{
			Title:       "Canva Pro Lifetime / 1 Tahun",
			Slug:        "canva-pro-1-year",
			Description: "Akses semua elemen premium Canva, hilangkan background, brand kit, dan AI design generator.",
			PriceIDR:    25000,
			ImageURL:    "https://images.unsplash.com/photo-1626785774573-4b799315345d?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"canva_pro1@design.com:CanvaPro#2026", "canva_pro2@design.com:CanvaPro#2027"},
		},
		{
			Title:       "Claude AI Pro (1 Bulan)",
			Slug:        "claude-ai-pro-1-month",
			Description: "Akses ke Claude 3.5 Sonnet & Opus dengan batas pesan 5x lebih tinggi dari versi gratis.",
			PriceIDR:    55000,
			ImageURL:    "https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?auto=format&fit=crop&w=600&q=80",
			Stocks:      []string{"claude_pro1@anthropic.com:ClaudeSonnet#35"},
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
			parts := strings.Split(s, ":")
			email := parts[0]
			rawPass := parts[1]

			encryptedPass, err := cryptoSvc.Encrypt(rawPass)
			if err != nil {
				log.Printf("Error encrypting password: %v", err)
				continue
			}

			_, err = pool.Exec(ctx, `
				INSERT INTO product_stocks (product_id, email, password_encrypted, status)
				VALUES ($1, $2, $3, 'AVAILABLE')
			`, productID, email, encryptedPass)
			if err != nil {
				log.Printf("Error inserting stock for %s: %v", p.Title, err)
			}
		}

		fmt.Printf("Seeded real AES-encrypted stock for product: %s\n", p.Title)
	}

	fmt.Println("\nSuccessfully seeded real encrypted accounts!")
}
