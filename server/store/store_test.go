package store

import (
	"context"
	"path/filepath"
	"testing"

	"urapt/shared/crypto"
	"urapt/shared/db"
	"urapt/shared/models"
)

// newTestStore opens a fresh migrated SQLite database in a per-test temp
// directory and returns a Store over it. The database is closed automatically
// when the test finishes.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return New(database)
}

// createUser is a test helper that inserts a user with a bcrypt-hashed
// password and returns it. The password is hashed for realism so that
// password-verification paths can be exercised if needed.
func (s *Store) createUser(t *testing.T, ctx context.Context, username, password string) *models.User {
	t.Helper()
	hash, err := crypto.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u, admin, err := s.CreateUser(ctx, username, hash)
	if err != nil {
		t.Fatalf("create user %q: %v", username, err)
	}
	t.Logf("created user %q (admin=%v)", u.Username, admin)
	return u
}

// createToken is a test helper that mints a real API token (with hash and
// prefix) for userID and returns the plaintext token plus the stored row.
func (s *Store) createToken(t *testing.T, ctx context.Context, userID, name string) (string, *models.APIToken) {
	t.Helper()
	tok, hash, prefix, err := crypto.GenerateToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	stored, err := s.CreateToken(ctx, userID, name, prefix, hash)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	return tok, stored
}

// createRepo is a test helper that creates a repository owned by userID.
func (s *Store) createRepo(t *testing.T, ctx context.Context, name, ownerID string, vis models.Visibility) *models.Repository {
	t.Helper()
	r, err := s.CreateRepository(ctx, name, ownerID, vis, "")
	if err != nil {
		t.Fatalf("create repo %q: %v", name, err)
	}
	return r
}

// createDistro is a test helper that creates a distribution within repoID.
func (s *Store) createDistro(t *testing.T, ctx context.Context, repoID, name string) *models.Distribution {
	t.Helper()
	d, err := s.CreateDistribution(ctx, repoID, name)
	if err != nil {
		t.Fatalf("create distro %q: %v", name, err)
	}
	return d
}

// createComponent is a test helper that creates a component within distroID.
func (s *Store) createComponent(t *testing.T, ctx context.Context, distroID, name string) *models.Component {
	t.Helper()
	c, err := s.CreateComponent(ctx, distroID, name)
	if err != nil {
		t.Fatalf("create component %q: %v", name, err)
	}
	return c
}
