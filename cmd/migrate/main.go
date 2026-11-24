// Command migrate applies the SQL files in cmd/migrate/sql/ to the configured
// database, in filename order.
//
// The files are embedded in the binary, so deploying the proxy does not
// require shipping the repository or having psql installed. Every migration is
// written to be idempotent, so re-running this is safe and is the intended way
// to bring an existing database up to date.
//
// Usage:
//
//	DB_URL=postgres://... go run ./cmd/migrate
//	DB_URL=postgres://... go run ./cmd/migrate -dry-run
package main

import (
	"database/sql"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

//go:embed all:sql
var migrations embed.FS

func main() {
	dryRun := flag.Bool("dry-run", false, "list the migrations without applying them")
	flag.Parse()

	entries, err := fs.ReadDir(migrations, "sql")
	if err != nil {
		log.Fatalf("failed to read embedded migrations: %v", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	if len(names) == 0 {
		log.Fatal("no migrations are embedded in this binary")
	}

	if *dryRun {
		fmt.Printf("%d migration(s) would be applied, in order:\n", len(names))
		for _, n := range names {
			fmt.Printf("  %s\n", n)
		}
		return
	}

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL is required but not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	db.SetConnMaxLifetime(time.Minute)
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to reach the database: %v", err)
	}

	for _, name := range names {
		body, err := migrations.ReadFile("sql/" + name)
		if err != nil {
			log.Fatalf("failed to read %s: %v", name, err)
		}

		start := time.Now()
		// Each file runs as one statement batch. A migration that fails leaves
		// the earlier ones applied, which is fine: they are all idempotent, so
		// fixing the problem and re-running picks up where it stopped.
		if _, err := db.Exec(string(body)); err != nil {
			log.Fatalf("migration %s failed: %v", name, err)
		}
		fmt.Printf("applied %-32s %s\n", name, time.Since(start).Round(time.Millisecond))
	}

	fmt.Printf("\n%d migration(s) applied. Database is up to date.\n", len(names))
}
