package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"carapulse/migrations"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "postgres DSN")
	dir := fs.String("dir", "./migrations", "migrations dir")
	action := fs.String("action", "", "up/down/status/version/redo")
	useEmbed := fs.Bool("embed", false, "use embedded migrations")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*dsn) == "" {
		return errors.New("dsn required")
	}
	if strings.TrimSpace(*action) == "" {
		return errors.New("action required")
	}

	goose.SetDialect("postgres")
	if *useEmbed {
		goose.SetBaseFS(migrations.EmbeddedFS)
		*dir = "."
	}

	db, err := sql.Open("postgres", *dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	switch *action {
	case "up":
		return goose.Up(db, *dir)
	case "down":
		return goose.Down(db, *dir)
	case "status":
		return goose.Status(db, *dir)
	case "version":
		_, err := goose.GetDBVersion(db)
		return err
	case "redo":
		return goose.Redo(db, *dir)
	default:
		return fmt.Errorf("unknown action %q", *action)
	}
}

