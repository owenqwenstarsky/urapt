// Package db opens the urapt SQLite database (pure-Go modernc driver, no CGO),
// enables WAL mode and foreign keys, and applies embedded SQL migrations.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // register the pure-Go SQLite driver
)

//go:embed all:migrations
var migrationsFS embed.FS

// Open opens (or creates) the SQLite database at path, applies pragmas and
// pending migrations, and returns the *sql.DB. The parent directory is created
// if missing.
func Open(path string) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if err := mkdirAll(dir); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite serial writers; reads still concurrent under WAL via SetMaxOpenConns handling

	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := Migrate(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// Migrate applies any embedded SQL migrations not yet recorded in
// schema_migrations.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	names, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	var files []string
	for _, e := range names {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		version, err := migrationVersion(f)
		if err != nil {
			return fmt.Errorf("parse migration name %s: %w", f, err)
		}
		var applied int
		err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if applied > 0 {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			return fmt.Errorf("apply migration %s: %w", f, err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, version, nowISO()); err != nil {
			return fmt.Errorf("record migration %d: %w", version, err)
		}
	}
	return nil
}

// nowISO returns the current UTC time in RFC3339 form.
func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// migrationVersion extracts the leading numeric component of a migration
// filename such as "0001_init.sql" -> 1.
func migrationVersion(name string) (int, error) {
	name = strings.TrimSuffix(name, ".sql")
	var num string
	for _, r := range name {
		if r >= '0' && r <= '9' {
			num += string(r)
			continue
		}
		break
	}
	if num == "" {
		return 0, fmt.Errorf("no leading digits in %q", name)
	}
	return strconv.Atoi(num)
}

func mkdirAll(dir string) error {
	if dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
