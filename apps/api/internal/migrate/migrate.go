// Package migrate runs schema migrations embedded in the binary against the
// configured Postgres instance. It wraps goose so callers do not need to know
// about database/sql or the migration filesystem layout.
package migrate

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // register pgx as database/sql driver
	"github.com/pressly/goose/v3"

	"github.com/benbotsford/trivia/migrations"
)

// Up runs all pending migrations to completion. It opens a dedicated
// database/sql connection because goose requires one — the application
// continues to use pgxpool for everything else.
func Up(databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migration db: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
