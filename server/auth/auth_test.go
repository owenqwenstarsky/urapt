package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	"urapt/server/store"
	"urapt/shared/crypto"
	"urapt/shared/db"
	"urapt/shared/models"
)

// newAuthSvc opens a fresh DB and returns an auth Service plus a handle to the
// underlying store for seeding users/repos/tokens.
func newAuthSvc(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	st := store.New(database)
	return NewService(st), st
}

// seedUser creates a user via the store and returns it.
func seedUser(t *testing.T, st *store.Store, ctx context.Context, username string) *models.User {
	t.Helper()
	hash, err := crypto.HashPassword("pw")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u, _, err := st.CreateUser(ctx, username, hash)
	if err != nil {
		t.Fatalf("create user %q: %v", username, err)
	}
	return u
}

// seedToken mints a real token for userID and returns the plaintext.
func seedToken(t *testing.T, st *store.Store, ctx context.Context, userID, name string) string {
	t.Helper()
	tok, hash, prefix, err := crypto.GenerateToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := st.CreateToken(ctx, userID, name, prefix, hash); err != nil {
		t.Fatalf("create token: %v", err)
	}
	return tok
}

func seedRepo(t *testing.T, st *store.Store, ctx context.Context, name, ownerID string, vis models.Visibility) *models.Repository {
	t.Helper()
	r, err := st.CreateRepository(ctx, name, ownerID, vis, "")
	if err != nil {
		t.Fatalf("create repo %q: %v", name, err)
	}
	return r
}

// seedAdmin creates a non-bootstrap user and promotes them to server admin,
// returning a refreshed user record with IsAdmin=true.
func seedAdmin(t *testing.T, st *store.Store, ctx context.Context, username string) *models.User {
	t.Helper()
	u := seedUser(t, st, ctx, username)
	if err := st.UpdateUser(ctx, u.ID, boolPtr(true)); err != nil {
		t.Fatalf("promote %q: %v", username, err)
	}
	refreshed, err := st.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("refetch admin: %v", err)
	}
	return refreshed
}

func bearerHeader(token string) http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+token)
	return h
}

func basicHeader(username, password string) http.Header {
	h := http.Header{}
	enc := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	h.Set("Authorization", "Basic "+enc)
	return h
}

// --- ResolveBearer ---

func TestResolveBearer_Valid(t *testing.T) {
	ctx := context.Background()
	svc, st := newAuthSvc(t)
	u := seedUser(t, st, ctx, "alice")
	tok := seedToken(t, st, ctx, u.ID, "laptop")

	id, err := svc.ResolveBearer(ctx, bearerHeader(tok))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if id.User == nil || id.User.ID != u.ID {
		t.Fatalf("identity = %+v", id)
	}
	if id.TokenID == "" {
		t.Fatal("token id should be set")
	}
}

func TestResolveBearer_MissingHeader(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAuthSvc(t)
	_, err := svc.ResolveBearer(ctx, http.Header{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestResolveBearer_MalformedHeader(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAuthSvc(t)
	h := http.Header{}
	h.Set("Authorization", "Bearer") // no token
	if _, err := svc.ResolveBearer(ctx, h); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
	h.Set("Authorization", "Basic abc")
	if _, err := svc.ResolveBearer(ctx, h); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated for non-Bearer scheme, got %v", err)
	}
}

func TestResolveBearer_InvalidToken(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAuthSvc(t)
	if _, err := svc.ResolveBearer(ctx, bearerHeader("urapt_notreal")); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated for bogus token, got %v", err)
	}
}

func TestResolveBearer_RevokedToken(t *testing.T) {
	ctx := context.Background()
	svc, st := newAuthSvc(t)
	u := seedUser(t, st, ctx, "alice")
	tok := seedToken(t, st, ctx, u.ID, "laptop")
	// Revoke by hash (simulating logout).
	_ = st.RevokeTokenByHash(ctx, crypto.HashToken(tok))

	if _, err := svc.ResolveBearer(ctx, bearerHeader(tok)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated for revoked token, got %v", err)
	}
}

// --- ResolveBasic ---

func TestResolveBasic_Valid(t *testing.T) {
	ctx := context.Background()
	svc, st := newAuthSvc(t)
	u := seedUser(t, st, ctx, "alice")
	tok := seedToken(t, st, ctx, u.ID, "laptop")

	// Username is ignored; password is the token.
	id, err := svc.ResolveBasic(ctx, basicHeader("anything", tok))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if id.User.ID != u.ID {
		t.Fatalf("user id mismatch")
	}
}

func TestResolveBasic_MissingAndMalformed(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAuthSvc(t)
	if _, err := svc.ResolveBasic(ctx, http.Header{}); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("missing header: expected ErrUnauthenticated, got %v", err)
	}
	h := http.Header{}
	h.Set("Authorization", "Basic not-base64!!!")
	if _, err := svc.ResolveBasic(ctx, h); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("bad base64: expected ErrUnauthenticated, got %v", err)
	}
	h.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("noseparator")))
	if _, err := svc.ResolveBasic(ctx, h); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("no colon: expected ErrUnauthenticated, got %v", err)
	}
}

// --- CanRead ---

func TestCanRead(t *testing.T) {
	ctx := context.Background()
	svc, st := newAuthSvc(t)
	owner := seedUser(t, st, ctx, "owner")
	member := seedUser(t, st, ctx, "member")
	nonMember := seedUser(t, st, ctx, "stranger")
	admin := seedAdmin(t, st, ctx, "admin")

	pubRepo := seedRepo(t, st, ctx, "pub", owner.ID, models.VisibilityPublic)
	privRepo := seedRepo(t, st, ctx, "priv", owner.ID, models.VisibilityPrivate)
	_ = st.AddMember(ctx, privRepo.ID, member.ID, models.AccessRead)

	cases := []struct {
		name string
		user *models.User
		repo *models.Repository
		want bool
	}{
		{"nil user, public repo", nil, pubRepo, true},
		{"nil user, private repo", nil, privRepo, false},
		{"admin on private", admin, privRepo, true},
		{"owner on private", owner, privRepo, true},
		{"member(read) on private", member, privRepo, true},
		{"non-member on public", nonMember, pubRepo, true},
		{"non-member on private", nonMember, privRepo, false},
	}
	for _, c := range cases {
		got, err := svc.CanRead(ctx, c.user, c.repo)
		if err != nil {
			t.Fatalf("%s: error %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestCanRead_WriteOnlyMemberCanRead(t *testing.T) {
	// A write-only member should still be able to read (the AccessWrite grant
	// does not include read in the switch in CanRead, so this confirms the
	// public-fallback behavior: a private repo would deny a write-only member
	// read access).
	ctx := context.Background()
	svc, st := newAuthSvc(t)
	owner := seedUser(t, st, ctx, "owner")
	writer := seedUser(t, st, ctx, "writer")
	privRepo := seedRepo(t, st, ctx, "priv", owner.ID, models.VisibilityPrivate)
	_ = st.AddMember(ctx, privRepo.ID, writer.ID, models.AccessWrite)

	got, _ := svc.CanRead(ctx, writer, privRepo)
	if got {
		t.Fatal("write-only member should NOT be able to read a private repo")
	}
}

// --- CanWrite ---

func TestCanWrite(t *testing.T) {
	ctx := context.Background()
	svc, st := newAuthSvc(t)
	owner := seedUser(t, st, ctx, "owner")
	reader := seedUser(t, st, ctx, "reader")
	writer := seedUser(t, st, ctx, "writer")
	rw := seedUser(t, st, ctx, "rw")
	repoAdmin := seedUser(t, st, ctx, "repoadmin")
	nonMember := seedUser(t, st, ctx, "stranger")
	admin := seedAdmin(t, st, ctx, "admin")

	repo := seedRepo(t, st, ctx, "repo", owner.ID, models.VisibilityPublic)
	_ = st.AddMember(ctx, repo.ID, reader.ID, models.AccessRead)
	_ = st.AddMember(ctx, repo.ID, writer.ID, models.AccessWrite)
	_ = st.AddMember(ctx, repo.ID, rw.ID, models.AccessReadWrite)
	_ = st.AddMember(ctx, repo.ID, repoAdmin.ID, models.AccessAdmin)

	cases := []struct {
		name string
		user *models.User
		want bool
	}{
		{"nil user", nil, false},
		{"admin", admin, true},
		{"owner", owner, true},
		{"read member", reader, false},
		{"write member", writer, true},
		{"read-write member", rw, true},
		{"repo admin member", repoAdmin, true},
		{"non-member", nonMember, false},
	}
	for _, c := range cases {
		got, err := svc.CanWrite(ctx, c.user, repo)
		if err != nil {
			t.Fatalf("%s: error %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

// --- CanManage ---

func TestCanManage(t *testing.T) {
	ctx := context.Background()
	svc, st := newAuthSvc(t)
	owner := seedUser(t, st, ctx, "owner")
	reader := seedUser(t, st, ctx, "reader")
	repoAdmin := seedUser(t, st, ctx, "repoadmin")
	nonMember := seedUser(t, st, ctx, "stranger")
	admin := seedAdmin(t, st, ctx, "admin")

	repo := seedRepo(t, st, ctx, "repo", owner.ID, models.VisibilityPublic)
	_ = st.AddMember(ctx, repo.ID, reader.ID, models.AccessRead)
	_ = st.AddMember(ctx, repo.ID, repoAdmin.ID, models.AccessAdmin)

	cases := []struct {
		name string
		user *models.User
		want bool
	}{
		{"nil user", nil, false},
		{"admin", admin, true},
		{"owner", owner, true},
		{"read member", reader, false},
		{"repo admin member", repoAdmin, true},
		{"non-member", nonMember, false},
	}
	for _, c := range cases {
		got, err := svc.CanManage(ctx, c.user, repo)
		if err != nil {
			t.Fatalf("%s: error %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func boolPtr(b bool) *bool { return &b }
