package store

import (
	"context"
	"errors"
	"testing"
)

func TestSaveGPGKeyAndGetDefault(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	k, err := s.SaveGPGKey(ctx, "ABCD1234", "user-1", "PUB-ARMORED", "PRIV-ARMORED", true)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if k.ID == "" || k.Fingerprint != "ABCD1234" || !k.IsDefault {
		t.Fatalf("key = %+v", k)
	}

	got, err := s.GetDefaultGPGKey(ctx)
	if err != nil {
		t.Fatalf("get default: %v", err)
	}
	if got.Fingerprint != "ABCD1234" || got.PublicKeyArmored != "PUB-ARMORED" {
		t.Fatalf("got = %+v", got)
	}
	if got.PrivateKeyArmored != "PRIV-ARMORED" {
		t.Fatal("private key armor should be retrieved from DB")
	}
	if !got.IsDefault {
		t.Fatal("expected is_default=true")
	}
}

func TestGetDefaultGPGKey_None(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, err := s.GetDefaultGPGKey(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound when no key, got %v", err)
	}
}

func TestGetGPGKeyPublic(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	_, _ = s.SaveGPGKey(ctx, "ABCD1234", "user-1", "PUB-ARMORED", "PRIV-ARMORED", true)

	pub, err := s.GetGPGKeyPublic(ctx)
	if err != nil {
		t.Fatalf("get public: %v", err)
	}
	if pub != "PUB-ARMORED" {
		t.Fatalf("pub = %q", pub)
	}
	// No default key.
	s2 := newTestStore(t)
	if _, err := s2.GetGPGKeyPublic(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- audit ---

func TestRecordAudit(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	// audit_log has FK constraints on user_id/repo_id, so use real rows.
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, "public")
	uid, repoID := u.ID, repo.ID

	// Insert with both user and repo set.
	if err := s.RecordAudit(ctx, &uid, &repoID, "push", "pkg-1", "uploaded myapp"); err != nil {
		t.Fatalf("record: %v", err)
	}
	// Insert with nil pointers (system action).
	if err := s.RecordAudit(ctx, nil, nil, "startup", "server", "started"); err != nil {
		t.Fatalf("record nil: %v", err)
	}

	// Verify rows exist. The audit_log table is write-only from the store API;
	// query directly to confirm persistence.
	var n int
	err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log`).Scan(&n)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 audit rows, got %d", n)
	}

	// Verify nullable columns are stored correctly.
	var uidVal, repoVal *string
	row := s.DB().QueryRowContext(ctx, `SELECT user_id, repository_id FROM audit_log WHERE action = 'startup'`)
	if err := row.Scan(&uidVal, &repoVal); err != nil {
		t.Fatalf("scan startup row: %v", err)
	}
	if uidVal != nil || repoVal != nil {
		t.Fatalf("expected nil user_id/repository_id for system action, got %v %v", uidVal, repoVal)
	}
}
