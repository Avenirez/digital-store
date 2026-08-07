package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"my-digital-store/backend/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load("../../.env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatalf("DATABASE_URL must be set")
	}

	oldKeyHex := os.Getenv("OLD_AES_KEY")
	newKeyHex := os.Getenv("NEW_AES_KEY")

	if oldKeyHex == "" || newKeyHex == "" {
		fmt.Println("Gunakan alat ini untuk merotasi AES_KEY pada data terenkripsi di database.")
		fmt.Println("Cara penggunaan:")
		fmt.Println("  OLD_AES_KEY=<key_lama_hex> NEW_AES_KEY=<key_baru_hex> go run ./cmd/reencrypt")
		log.Fatalf("Kesalahan: OLD_AES_KEY dan NEW_AES_KEY wajib diset.")
	}

	oldKeyBytes, err := hex.DecodeString(oldKeyHex)
	if err != nil || len(oldKeyBytes) != 32 {
		log.Fatalf("OLD_AES_KEY harus 64 karakter hex (32 bytes): %v", err)
	}

	newKeyBytes, err := hex.DecodeString(newKeyHex)
	if err != nil || len(newKeyBytes) != 32 {
		log.Fatalf("NEW_AES_KEY harus 64 karakter hex (32 bytes): %v", err)
	}

	oldCrypto, err := service.NewCryptoService(oldKeyBytes)
	if err != nil {
		log.Fatalf("Gagal inisialisasi CryptoService lama: %v", err)
	}

	newCrypto, err := service.NewCryptoService(newKeyBytes)
	if err != nil {
		log.Fatalf("Gagal inisialisasi CryptoService baru: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Gagal koneksi database: %v", err)
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, "SELECT id, password_encrypted FROM product_stocks")
	if err != nil {
		log.Fatalf("Gagal mengambil data product_stocks: %v", err)
	}

	type stockRecord struct {
		id   string
		pass string
	}
	var records []stockRecord

	for rows.Next() {
		var r stockRecord
		if err := rows.Scan(&r.id, &r.pass); err == nil {
			records = append(records, r)
		}
	}
	rows.Close()

	log.Printf("Menemukan %d data stok untuk di-re-encrypt...", len(records))

	successCount := 0
	failCount := 0

	for _, r := range records {
		plainPass, err := oldCrypto.Decrypt(r.pass)
		if err != nil {
			log.Printf("[ERROR] Gagal dekripsi ID %s dengan OLD_AES_KEY: %v", r.id, err)
			failCount++
			continue
		}

		newEncrypted, err := newCrypto.Encrypt(plainPass)
		if err != nil {
			log.Printf("[ERROR] Gagal enkripsi ulang ID %s dengan NEW_AES_KEY: %v", r.id, err)
			failCount++
			continue
		}

		_, err = pool.Exec(ctx, "UPDATE product_stocks SET password_encrypted = $1 WHERE id = $2", newEncrypted, r.id)
		if err != nil {
			log.Printf("[ERROR] Gagal update DB untuk ID %s: %v", r.id, err)
			failCount++
		} else {
			successCount++
		}
	}

	fmt.Println("\n==========================================")
	fmt.Printf("Status Re-enkripsi Kunci AES:\n")
	fmt.Printf("  Sukses  : %d stok\n", successCount)
	fmt.Printf("  Gagal   : %d stok\n", failCount)
	fmt.Println("==========================================")
	if failCount == 0 {
		fmt.Println("Rotasi kunci selesai! Sekarang Anda dapat mengupdate AES_KEY di .env dengan NEW_AES_KEY.")
	}
}
