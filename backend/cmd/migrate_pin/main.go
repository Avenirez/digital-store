package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

	_, err = pool.Exec(ctx, "ALTER TABLE orders ADD COLUMN IF NOT EXISTS pin VARCHAR(10) DEFAULT '123456';")
	if err != nil {
		log.Fatalf("Migration error: %v", err)
	}

	fmt.Println("Successfully added 'pin' column to 'orders' table!")
}
