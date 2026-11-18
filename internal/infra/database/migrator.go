package database

import (
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

//go:embed migration/*.sql
var migrationFS embed.FS

func Migrate(dsn string, command string) error {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	switch command {
	case "up":
		return goose.Up(db, "migration")
	case "down":
		return goose.Down(db, "migration")
	case "status":
		return goose.Status(db, "migration")
	default:
		return fmt.Errorf("unknown migration command %q", command)
	}
}
