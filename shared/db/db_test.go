package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var n int
	err = db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table'`).Scan(&n)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	if n < 10 {
		t.Fatalf("expected at least 10 tables, got %d", n)
	}

	var v int
	err = db.QueryRowContext(context.Background(),
		`SELECT version FROM schema_migrations WHERE version=1`).Scan(&v)
	if err != nil {
		t.Fatalf("migration not recorded: %v", err)
	}
	if v != 1 {
		t.Fatalf("expected version 1, got %d", v)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open first: %v", err)
	}
	db.Close()
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Open second: %v", err)
	}
	defer db2.Close()
}
