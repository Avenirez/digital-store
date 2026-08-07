package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"my-digital-store/backend/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type Account struct {
	Email    string
	Password string
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
		log.Fatalf("Failed to init crypto service: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer pool.Close()

	// 1. Delete all products except CapCut
	_, err = pool.Exec(ctx, "DELETE FROM products WHERE slug != $1", "capcut-premium-7-days")
	if err != nil {
		log.Printf("Warning: error deleting old products: %v", err)
	}

	// 2. Ensure Capcut product exists
	var productID string
	productTitle := "Capcut Premium (7 Hari)"
	productSlug := "capcut-premium-7-days"
	productDesc := "Akun Capcut Premium 7 Hari privat, akses penuh semua fitur efek & template pro."
	priceIDR := 10000.00
	imageURL := "/images/capcut.webp"

	err = pool.QueryRow(ctx, `
		INSERT INTO products (title, slug, description, price_idr, image_url, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
		ON CONFLICT (slug) DO UPDATE SET 
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			price_idr = EXCLUDED.price_idr,
			image_url = EXCLUDED.image_url
		RETURNING id
	`, productTitle, productSlug, productDesc, priceIDR, imageURL).Scan(&productID)

	if err != nil {
		log.Fatalf("Failed to create/get product %s: %v", productTitle, err)
	}

	// 3. Clear existing stocks for CapCut to reset to exactly 5 accounts
	_, _ = pool.Exec(ctx, "DELETE FROM product_stocks WHERE product_id = $1", productID)

	accounts := []Account{
		{Email: "blackbutterfly564@saovangtiles.site", Password: "masuk123"},
		{Email: "crazyswan547@submitreports.com", Password: "masuk123"},
		{Email: "heavymouse584@mailfirefly.com", Password: "masuk123"},
		{Email: "smallcat555@saovangtiles.site", Password: "masuk123"},
		{Email: "beautifullion284@phuongnhicare.com", Password: "masuk123"},
	}

	insertedCount := 0
	for _, acc := range accounts {
		encryptedPass, err := cryptoSvc.Encrypt(acc.Password)
		if err != nil {
			log.Printf("Error encrypting password for %s: %v", acc.Email, err)
			continue
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO product_stocks (product_id, email, password_encrypted, status)
			VALUES ($1, $2, $3, 'AVAILABLE')
		`, productID, acc.Email, encryptedPass)

		if err != nil {
			log.Printf("Error inserting stock %s: %v", acc.Email, err)
		} else {
			insertedCount++
			fmt.Printf("✓ Berhasil memasukkan stok: %s\n", acc.Email)
		}
	}

	var totalStock int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_stocks WHERE product_id = $1 AND status = 'AVAILABLE'", productID).Scan(&totalStock)

	fmt.Println("\n==========================================")
	fmt.Printf("Produk: %s\n", productTitle)
	fmt.Printf("Total Stok Akun Capcut Tersedia di DB: %d akun\n", totalStock)
	fmt.Println("==========================================")
}
