package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"my-digital-store/backend/internal/service"
)

type Account struct {
	Email    string
	Password string
}

func main() {
	_ = godotenv.Load("../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:password@127.0.0.1:5432/digitalstore?sslmode=disable"
	}

	aesKeyHex := os.Getenv("AES_KEY")
	if aesKeyHex == "" {
		aesKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}

	aesKey, err := hex.DecodeString(aesKeyHex)
	if err != nil || len(aesKey) != 32 {
		log.Fatalf("Invalid AES key: %v", err)
	}

	cryptoSvc, err := service.NewCryptoService(aesKey)
	if err != nil {
		log.Fatalf("Failed to initialize CryptoService: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// 1. Deactivate non-Capcut products
	_, _ = pool.Exec(ctx, "UPDATE products SET is_active = false WHERE slug != 'capcut-premium-7-days'")

	// 2. Ensure Capcut product is active and get its ID
	var productID string
	err = pool.QueryRow(ctx, `
		INSERT INTO products (title, slug, description, price_idr, image_url, is_active)
		VALUES ('Capcut Premium (7 Hari)', 'capcut-premium-7-days', 'Akun Capcut Premium 7 Hari privat, akses penuh semua fitur efek & template pro.', 1000, '/images/capcut.webp', true)
		ON CONFLICT (slug) DO UPDATE SET is_active = true
		RETURNING id
	`).Scan(&productID)

	if err != nil {
		log.Fatalf("Failed to insert/get Capcut product: %v", err)
	}

	// 3. Clear old dummy available stocks for Capcut to match exact user list
	_, _ = pool.Exec(ctx, "DELETE FROM product_stocks WHERE product_id = $1 AND status = 'AVAILABLE'", productID)

	// 4. User accounts list
	userAccounts := []Account{
		{Email: "blackbutterfly564@saovangtiles.site", Password: "masuk123"},
		{Email: "crazyswan547@submitreports.com", Password: "masuk123"},
		{Email: "heavymouse584@mailfirefly.com", Password: "masuk123"},
		{Email: "smallcat555@saovangtiles.site", Password: "masuk123"},
		{Email: "beautifullion284@phuongnhicare.com", Password: "masuk123"},
	}

	insertedCount := 0
	for _, acc := range userAccounts {
		encryptedPass, err := cryptoSvc.Encrypt(acc.Password)
		if err != nil {
			log.Printf("Failed to encrypt password for %s: %v", acc.Email, err)
			continue
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO product_stocks (product_id, email, password_encrypted, status)
			VALUES ($1, $2, $3, 'AVAILABLE')
		`, productID, acc.Email, encryptedPass)

		if err != nil {
			log.Printf("Failed to insert account %s: %v", acc.Email, err)
		} else {
			insertedCount++
			fmt.Printf("✅ Inserted account %d: %s\n", insertedCount, acc.Email)
		}
	}

	fmt.Printf("\n🎉 SUCCESS! Inserted %d real CapCut accounts into database for Capcut Premium (7 Hari)!\n", insertedCount)
}
