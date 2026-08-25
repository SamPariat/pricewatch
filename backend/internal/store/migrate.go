// Package store holds the Postgres-backed Repository adapter and migration
// runner. SQL access is confined to this package so the domain layer never
// imports database/sql or pgx directly.
package store

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"

	"github.com/SamPariat/pricewatch/migrations"
)

// Migrate applies every pending goose migration embedded in the migrations
// package. It opens its own short-lived *sql.DB because goose needs
// database/sql, not the pgxpool.Pool the rest of the app uses.
func Migrate(databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("migrate: open: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("migrate: set dialect: %w", err)
	}
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("migrate: up: %w", err)
	}
	return nil
}
