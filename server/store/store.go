// Package store provides the data-access layer for urapt-server: typed
// methods over the SQLite database backing all domain objects (users, tokens,
// repositories, distributions, components, architectures, packages, blobs,
// gpg keys, audit log).
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Store is the entrypoint to the data-access layer. All methods are safe for
// concurrent use; the underlying *sql.DB is configured with a single writer
// connection (see shared/db).
type Store struct {
	db  *sql.DB
	now func() string
}

// New constructs a Store wrapping db.
func New(db *sql.DB) *Store {
	return &Store{db: db, now: nowISO}
}

// DB returns the underlying database (used by app for raw queries if needed).
func (s *Store) DB() *sql.DB { return s.db }

// Now returns the current timestamp in the urapt canonical form.
func (s *Store) Now() string { return s.now() }

// newID returns a fresh UUIDv4 string.
func newID() string { return uuid.NewString() }

// nowISO returns the current UTC time in RFC3339 form.
func nowISO() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// exec is a small helper for ExecContext with a context.
func (s *Store) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}

// ErrNotFound is returned by Get-style methods when no row matches.
var ErrNotFound = fmt.Errorf("not found")

// isErrNoRows returns true if err is sql.ErrNoRows.
func isErrNoRows(err error) bool { return err == sql.ErrNoRows }
