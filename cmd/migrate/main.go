// Command migrate is the operator CLI for database schema migrations.
//
// Subcommands:
//
//	migrate up            apply all pending migrations
//	migrate up N          apply up to N pending migrations
//	migrate down N        roll back N migrations (blocked when APP_ENV=prod)
//	migrate force V       mark DB at version V, clear dirty flag
//	migrate version       print current version and dirty flag
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/storage/postgresql"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fatal("DATABASE_URL is required")
	}

	mg, err := postgresql.NewMigrator(dsn)
	if err != nil {
		fatal("init migrator: %v", err)
	}
	defer func() { _ = mg.Close() }()

	switch os.Args[1] {
	case "up":
		if len(os.Args) == 2 {
			if err := mg.Up(); err != nil {
				fatal("%v", err)
			}
			fmt.Println("migrations up to date")
			return
		}
		// up N: not exposed by the Migrator API; Up() is all-or-nothing.
		// A partial up is rarely wanted and easy to foot-gun, so we reject it.
		fatal("`up N` is not supported; use `up` (all pending) or `down N`")
	case "down":
		if len(os.Args) != 3 {
			fatal("usage: migrate down N")
		}
		if env := os.Getenv("APP_ENV"); env == "prod" {
			fatal("`down` is disabled when APP_ENV=prod")
		}
		n, err := strconv.Atoi(os.Args[2])
		if err != nil || n <= 0 {
			fatal("invalid N: %q", os.Args[2])
		}
		if err := mg.Down(n); err != nil {
			fatal("%v", err)
		}
		fmt.Printf("rolled back %d migration(s)\n", n)
	case "force":
		if len(os.Args) != 3 {
			fatal("usage: migrate force V")
		}
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fatal("invalid V: %q", os.Args[2])
		}
		if err := mg.Force(v); err != nil {
			fatal("%v", err)
		}
		fmt.Printf("forced version to %d\n", v)
	case "version":
		v, dirty, err := mg.Version()
		if err != nil {
			fatal("%v", err)
		}
		fmt.Printf("version=%d dirty=%t\n", v, dirty)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: migrate <up|down N|force V|version>")
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}
