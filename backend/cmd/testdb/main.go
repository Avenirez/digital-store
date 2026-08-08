package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	passwords := []string{"password", "postgres", "admin", "root", "", "123456", "12345678"}
	users := []string{"postgres", "user"}

	ctx := context.Background()

	for _, u := range users {
		for _, p := range passwords {
			connStr := fmt.Sprintf("postgres://%s:%s@localhost:5432/postgres?sslmode=disable", u, p)
			pool, err := pgxpool.New(ctx, connStr)
			if err != nil {
				continue
			}
			err = pool.Ping(ctx)
			pool.Close()
			if err == nil {
				fmt.Printf("SUCCESS! User: %s, Password: '%s'\n", u, p)
				return
			}
		}
	}
	fmt.Println("No matching password found.")
}
