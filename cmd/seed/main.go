// Command seed applies reference data from the embedded seeds/ sources.
//
// Usage:
//
//	DATABASE_URL=postgres://... seed run --source=templates --on-conflict=skip
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/seed"
	"github.com/rasulov-emirlan/zenflow-devices-api/internal/storage/postgresql"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		if err := runSeed(os.Args[2:]); err != nil {
			fatal("%v", err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func runSeed(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	source := fs.String("source", "templates", "seed source name")
	onConflict := fs.String("on-conflict", "skip", "skip|update|fail")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	oc, err := seed.ParseOnConflict(*onConflict)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := postgresql.OpenPool(ctx, dsn)
	if err != nil {
		return fmt.Errorf("open pool: %w", err)
	}
	defer pool.Close()

	s, err := seed.FromSource(*source, pool)
	if err != nil {
		return err
	}
	if err := s.Seed(ctx, seed.Options{OnConflict: oc}); err != nil {
		return fmt.Errorf("seed %s: %w", *source, err)
	}
	fmt.Printf("seeded source=%s on-conflict=%s\n", *source, oc)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: seed run --source=<name> [--on-conflict=skip|update|fail]")
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}
