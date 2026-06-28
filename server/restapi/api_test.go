package restapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"urapt/server/auth"
	"urapt/server/cache"
	"urapt/server/store"
	"urapt/shared/config"
	"urapt/shared/db"
	"urapt/shared/gpg"
)

type harness struct {
	t      *testing.T
	srv    *httptest.Server
	store  *store.Store
	token  string
	pkgDir string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	st := store.New(database)
	authSvc := auth.NewService(st)

	key, err := gpg.GenerateKey("urapt-test <test>", 2048)
	if err != nil {
		t.Fatalf("gpg key: %v", err)
	}
	sp := &testSigner{key: key}
	cfg := config.Defaults
	cfg.StoreDir = dir
	cfg.PackagesDir = filepath.Join(dir, "packages")
	cfg.DBPath = filepath.Join(dir, "test.db")
	if err := os.MkdirAll(cfg.PackagesDir, 0o755); err != nil {
		t.Fatalf("mkdir pkg dir: %v", err)
	}

	h := &harness{t: t, store: st, pkgDir: cfg.PackagesDir}
	mux := New(st, authSvc, sp, &cfg, cache.New())
	root := chi.NewRouter()
	root.Mount("/api/v1", mux)
	h.srv = httptest.NewServer(root)
	t.Cleanup(h.srv.Close)
	return h
}

func (h *harness) do(method, path, token string, body any) (int, []byte) {
	h.t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshal: %v", err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, h.srv.URL+path, r)
	if err != nil {
		h.t.Fatalf("new req: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatalf("do %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func (h *harness) serverInfo() (int, map[string]any) {
	code, body := h.do("GET", "/api/v1/server/info", "", nil)
	var m map[string]any
	_ = json.Unmarshal(body, &m)
	return code, m
}

func TestServerInfoNeedsSetup(t *testing.T) {
	h := newHarness(t)
	code, m := h.serverInfo()
	if code != 200 {
		t.Fatalf("status %d", code)
	}
	if m["needs_setup"] != true {
		t.Fatalf("expected needs_setup=true, got %v", m["needs_setup"])
	}
	if m["default_key_fingerprint"] == "" {
		t.Fatal("expected fingerprint")
	}
}

func TestRegisterFirstUserIsAdmin(t *testing.T) {
	h := newHarness(t)
	code, body := h.do("POST", "/api/v1/auth/register", "", map[string]string{
		"username": "alice", "password": "supersecret",
	})
	if code != 201 {
		t.Fatalf("register status %d body %s", code, body)
	}
	var resp struct {
		User  map[string]any `json:"user"`
		Token string         `json:"token"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("empty token")
	}
	if resp.User["is_admin"] != true {
		t.Fatalf("first user should be admin, got %v", resp.User["is_admin"])
	}
	h.token = resp.Token

	// /me with token
	code, body = h.do("GET", "/api/v1/me", h.token, nil)
	if code != 200 {
		t.Fatalf("me status %d", code)
	}

	// needs_setup should now be false
	_, m := h.serverInfo()
	if m["needs_setup"] != false {
		t.Fatalf("expected needs_setup=false after register")
	}
}

func TestRegisterRejectsBadUsername(t *testing.T) {
	h := newHarness(t)
	code, _ := h.do("POST", "/api/v1/auth/register", "", map[string]string{
		"username": "A", "password": "supersecret",
	})
	if code != 400 {
		t.Fatalf("expected 400 for short username, got %d", code)
	}
}

func TestLoginAndAuthFlow(t *testing.T) {
	h := newHarness(t)
	h.do("POST", "/api/v1/auth/register", "", map[string]string{
		"username": "bob", "password": "supersecret",
	})

	code, body := h.do("POST", "/api/v1/auth/login", "", map[string]string{
		"username": "bob", "password": "supersecret",
	})
	if code != 200 {
		t.Fatalf("login status %d", code)
	}
	var resp struct {
		Token string `json:"token"`
	}
	json.Unmarshal(body, &resp)
	if resp.Token == "" {
		t.Fatal("empty token")
	}

	// wrong password
	code, _ = h.do("POST", "/api/v1/auth/login", "", map[string]string{
		"username": "bob", "password": "wrongpassword",
	})
	if code != 401 {
		t.Fatalf("expected 401 for wrong password, got %d", code)
	}

	// me without token
	code, _ = h.do("GET", "/api/v1/me", "", nil)
	if code != 401 {
		t.Fatalf("expected 401 without token, got %d", code)
	}

	// tokens list
	code, _ = h.do("GET", "/api/v1/me/tokens", resp.Token, nil)
	if code != 200 {
		t.Fatalf("tokens list status %d", code)
	}

	// create token
	code, body = h.do("POST", "/api/v1/me/tokens", resp.Token, map[string]string{"name": "laptop"})
	if code != 201 {
		t.Fatalf("create token status %d", code)
	}

	// logout
	code, _ = h.do("POST", "/api/v1/auth/logout", resp.Token, nil)
	if code != 204 {
		t.Fatalf("logout status %d", code)
	}
	// token now revoked
	code, _ = h.do("GET", "/api/v1/me", resp.Token, nil)
	if code != 401 {
		t.Fatalf("expected 401 after logout, got %d", code)
	}
}

func TestUsersAdminOnly(t *testing.T) {
	h := newHarness(t)
	// register two users; first is admin
	_, body := h.do("POST", "/api/v1/auth/register", "", map[string]string{"username": "admin", "password": "supersecret"})
	var admin struct {
		Token string `json:"token"`
	}
	json.Unmarshal(body, &admin)

	_, body = h.do("POST", "/api/v1/auth/register", "", map[string]string{"username": "carol", "password": "supersecret"})
	var carol struct {
		User  map[string]any `json:"user"`
		Token string         `json:"token"`
	}
	json.Unmarshal(body, &carol)
	if carol.User["is_admin"] == true {
		t.Fatal("second user should not be admin")
	}

	// carol cannot list users
	code, _ := h.do("GET", "/api/v1/users", carol.Token, nil)
	if code != 403 {
		t.Fatalf("non-admin list users should be 403, got %d", code)
	}
	// admin can list users
	code, body = h.do("GET", "/api/v1/users", admin.Token, nil)
	if code != 200 {
		t.Fatalf("admin list users should be 200, got %d", code)
	}

	// admin cannot delete self
	code, _ = h.do("DELETE", "/api/v1/users/"+carol.User["id"].(string), admin.Token, nil)
	if code != 204 {
		t.Fatalf("admin delete carol should be 204, got %d", code)
	}
}

// keep context import used in case of future expansion
var _ = context.Background

// testSigner implements restapi.SignerProvider for tests.
type testSigner struct{ key *gpg.Key }

func (s *testSigner) PublicKeyArmored() (string, error) { return s.key.ArmoredPublic() }
func (s *testSigner) Fingerprint() string               { return s.key.Fingerprint }
