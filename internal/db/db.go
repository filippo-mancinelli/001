package db

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

func Connect() error {
	url := os.Getenv("DB_URL")
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		return err
	}
	Pool = pool
	return nil
}
