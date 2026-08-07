package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"my-digital-store/backend/internal/service"

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

	// Clear and deactivate non-Capcut products
	_, _ = pool.Exec(ctx, "UPDATE products SET is_active = false WHERE slug != 'capcut-premium-7-days'")
	_, _ = pool.Exec(ctx, "DELETE FROM product_stocks WHERE product_id IN (SELECT id FROM products WHERE slug != 'capcut-premium-7-days')")

	products := []ProductSeed{
		{
			Title:       "Capcut Premium (7 Hari)",
			Slug:        "capcut-premium-7-days",
			Description: "Akun Capcut Premium 7 Hari privat, akses penuh semua fitur efek & template pro.",
			PriceIDR:    10000,
			ImageURL:    "/images/capcut.webp",
			Stocks: []string{
				"blackbutterfly564@saovangtiles.site:masuk123",
				"crazyswan547@submitreports.com:masuk123",
				"heavymouse584@mailfirefly.com:masuk123",
				"smallcat555@saovangtiles.site:masuk123",
				"beautifullion284@phuongnhicare.com:masuk123",
			},
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

		// Reset stocks to exactly the 5 specified accounts
		_, _ = pool.Exec(ctx, "DELETE FROM product_stocks WHERE product_id = $1", productID)

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

	fmt.Println("\nSuccessfully updated database: ONLY Capcut Premium (7 Hari) with 5 accounts!")
}
