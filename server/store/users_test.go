package store

import (
	"context"
	"errors"
	"testing"

	"urapt/shared/crypto"
)

func TestCreateUser_BootstrapAdmin(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// First user becomes admin.
	u1 := s.createUser(t, ctx, "alice", "pw1")
	if !u1.IsAdmin {
		t.Fatalf("first user should be admin, got is_admin=%v", u1.IsAdmin)
	}

	// Subsequent users are not admin.
	u2 := s.createUser(t, ctx, "bob", "pw2")
	if u2.IsAdmin {
		t.Fatalf("second user should not be admin")
	}
	u3 := s.createUser(t, ctx, "carol", "pw3")
	if u3.IsAdmin {
		t.Fatalf("third user should not be admin")
	}
}

func TestCreateUser_DuplicateUsernameRejected(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	s.createUser(t, ctx, "alice", "pw1")
	if _, _, err := s.CreateUser(ctx, "alice", "hash"); err == nil {
		t.Fatal("expected error creating duplicate username")
	}
}

func TestGetUserByID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "pw1")

	got, err := s.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.ID != u.ID || got.Username != "alice" {
		t.Fatalf("got %+v", got)
	}

	if _, err := s.GetUserByID(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetUserByUsername_CaseInsensitive(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	s.createUser(t, ctx, "Alice", "pw")

	for _, q := range []string{"alice", "ALICE", "AlIcE"} {
		got, err := s.GetUserByUsername(ctx, q)
		if err != nil {
			t.Fatalf("lookup %q: %v", q, err)
		}
		if got.Username != "Alice" {
			t.Fatalf("expected original casing 'Alice', got %q", got.Username)
		}
	}
	if _, err := s.GetUserByUsername(ctx, "bob"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing user, got %v", err)
	}
}

func TestGetUserPasswordHash(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "supersecret")

	hash, err := s.GetUserPasswordHash(ctx, u.ID)
	if err != nil {
		t.Fatalf("get hash: %v", err)
	}
	if !crypto.VerifyPassword(hash, "supersecret") {
		t.Fatal("bcrypt hash did not verify against original password")
	}
	if crypto.VerifyPassword(hash, "wrong") {
		t.Fatal("bcrypt hash verified against wrong password")
	}
}

func TestListUsers(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	s.createUser(t, ctx, "carol", "p")
	s.createUser(t, ctx, "alice", "p")
	s.createUser(t, ctx, "bob", "p")

	users, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(users))
	}
	// Ordered by username_lc.
	if users[0].Username != "alice" || users[1].Username != "bob" || users[2].Username != "carol" {
		names := []string{users[0].Username, users[1].Username, users[2].Username}
		t.Fatalf("expected alphabetical order, got %v", names)
	}
}

func TestUpdateUser(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	// First user is bootstrap admin; create a second non-admin user to test
	// promotion/demotion on.
	s.createUser(t, ctx, "admin", "p")
	u := s.createUser(t, ctx, "alice", "p")
	if u.IsAdmin {
		t.Fatal("non-bootstrap user should not be admin")
	}

	admin := true
	if err := s.UpdateUser(ctx, u.ID, &admin); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := s.GetUserByID(ctx, u.ID)
	if !got.IsAdmin {
		t.Fatal("expected is_admin=true after update")
	}

	off := false
	_ = s.UpdateUser(ctx, u.ID, &off)
	got, _ = s.GetUserByID(ctx, u.ID)
	if got.IsAdmin {
		t.Fatal("expected is_admin=false after demotion")
	}

	// nil leaves it unchanged.
	_ = s.UpdateUser(ctx, u.ID, nil)
	got, _ = s.GetUserByID(ctx, u.ID)
	if got.IsAdmin {
		t.Fatal("nil isAdmin should leave it false")
	}
}

func TestDeleteUser(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")

	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetUserByID(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

// --- tokens ---

func TestCreateTokenAndGetByHash(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")

	plaintext, stored := s.createToken(t, ctx, u.ID, "laptop")
	if stored.Name != "laptop" {
		t.Fatalf("name = %q", stored.Name)
	}
	if stored.Prefix == "" {
		t.Fatal("prefix should be set")
	}

	got, err := s.GetTokenByHash(ctx, crypto.HashToken(plaintext))
	if err != nil {
		t.Fatalf("get by hash: %v", err)
	}
	if got.ID != stored.ID {
		t.Fatalf("id mismatch: %s vs %s", got.ID, stored.ID)
	}
	if got.RevokedAt != nil {
		t.Fatal("fresh token should not be revoked")
	}
}

func TestGetTokenByHash_RevokedExcluded(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	plaintext, stored := s.createToken(t, ctx, u.ID, "laptop")

	if err := s.RevokeToken(ctx, u.ID, stored.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := s.GetTokenByHash(ctx, crypto.HashToken(plaintext)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked token should not be returned, got %v", err)
	}
}

func TestRevokeToken_OwnershipEnforced(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	alice := s.createUser(t, ctx, "alice", "p")
	bob := s.createUser(t, ctx, "bob", "p")
	_, aliceToken := s.createToken(t, ctx, alice.ID, "alice-laptop")

	// Bob cannot revoke Alice's token.
	if err := s.RevokeToken(ctx, bob.ID, aliceToken.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user revoke should be ErrNotFound, got %v", err)
	}
	// Alice can revoke her own.
	if err := s.RevokeToken(ctx, alice.ID, aliceToken.ID); err != nil {
		t.Fatalf("own revoke: %v", err)
	}
}

func TestRevokeTokenByHash(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	plaintext, _ := s.createToken(t, ctx, u.ID, "laptop")

	if err := s.RevokeTokenByHash(ctx, crypto.HashToken(plaintext)); err != nil {
		t.Fatalf("revoke by hash: %v", err)
	}
	if _, err := s.GetTokenByHash(ctx, crypto.HashToken(plaintext)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked token should not be found, got %v", err)
	}
	// Revoking again is a no-op (no error, no rows affected).
	if err := s.RevokeTokenByHash(ctx, crypto.HashToken(plaintext)); err != nil {
		t.Fatalf("idempotent revoke: %v", err)
	}
}

func TestListTokens_IncludesRevoked(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	_, t1 := s.createToken(t, ctx, u.ID, "a")
	_, t2 := s.createToken(t, ctx, u.ID, "b")
	_ = s.RevokeToken(ctx, u.ID, t1.ID)

	list, err := s.ListTokens(ctx, u.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 tokens (incl revoked), got %d", len(list))
	}
	for _, tk := range list {
		if tk.ID == t1.ID && tk.RevokedAt == nil {
			t.Fatal("revoked token should have RevokedAt set")
		}
		if tk.ID == t2.ID && tk.RevokedAt != nil {
			t.Fatal("active token should not be revoked")
		}
	}
}

func TestListTokens_ScopedToUser(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	alice := s.createUser(t, ctx, "alice", "p")
	bob := s.createUser(t, ctx, "bob", "p")
	s.createToken(t, ctx, alice.ID, "alice-token")
	s.createToken(t, ctx, bob.ID, "bob-token")

	aliceList, _ := s.ListTokens(ctx, alice.ID)
	if len(aliceList) != 1 || aliceList[0].UserID != alice.ID {
		t.Fatalf("alice should see only her token, got %d", len(aliceList))
	}
}

func TestTouchToken(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	plaintext, stored := s.createToken(t, ctx, u.ID, "laptop")

	if stored.LastUsedAt != nil {
		t.Fatal("fresh token should have nil LastUsedAt")
	}
	if err := s.TouchToken(ctx, stored.ID); err != nil {
		t.Fatalf("touch: %v", err)
	}
	// Look up via the hash to confirm last_used_at was written.
	got, err := s.GetTokenByHash(ctx, crypto.HashToken(plaintext))
	if err != nil {
		t.Fatalf("get by hash: %v", err)
	}
	if got.LastUsedAt == nil {
		t.Fatal("expected LastUsedAt set after touch")
	}
}
