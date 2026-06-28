package aptrepo_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"urapt/server/aptrepo"
	"urapt/server/auth"
	"urapt/server/cache"
	"urapt/server/restapi"
	"urapt/server/store"
	"urapt/shared/config"
	"urapt/shared/db"
	"urapt/shared/gpg"
)

// newServer spins up a REST + APT server backed by a temp DB and store, with a
// pre-created owner token. It returns the server, the owner token, and the
// armored public key.
func newServer(t *testing.T) (*httptest.Server, string, string) {
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
	if err := mkdirAll(cfg.PackagesDir); err != nil {
		t.Fatalf("mkdir pkg dir: %v", err)
	}

	idxCache := cache.New()
	root := chi.NewRouter()
	root.Mount("/api/v1", restapi.New(st, authSvc, sp, &cfg, idxCache))
	root.Mount("/apt", aptrepo.New(st, authSvc, key, idxCache))
	srv := httptest.NewServer(root)
	t.Cleanup(srv.Close)

	// Register owner.
	body := post(t, srv, "/api/v1/auth/register", "", map[string]string{"username": "owner", "password": "supersecret"})
	token := str(body, "token")
	pub, err := key.ArmoredPublic()
	if err != nil {
		t.Fatalf("armored public: %v", err)
	}
	return srv, token, pub
}

type testSigner struct{ key *gpg.Key }

func (s *testSigner) PublicKeyArmored() (string, error) { return s.key.ArmoredPublic() }
func (s *testSigner) Fingerprint() string               { return s.key.Fingerprint }

func post(t *testing.T, srv *httptest.Server, path, token string, body any) []byte {
	t.Helper()
	b := jsonMarshal(body)
	req, _ := http.NewRequest("POST", srv.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	defer resp.Body.Close()
	return readAll(resp.Body)
}

func get(t *testing.T, srv *httptest.Server, path, token string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest("GET", srv.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode, readAll(resp.Body)
}

func setupRepo(t *testing.T, srv *httptest.Server, token string) {
	post(t, srv, "/api/v1/repositories", token, map[string]any{"name": "myrepo", "visibility": "public"})
	post(t, srv, "/api/v1/repositories/myrepo/distributions", token, map[string]string{"name": "stable"})
	post(t, srv, "/api/v1/repositories/myrepo/distributions/stable/components", token, map[string]string{"name": "main"})
	post(t, srv, "/api/v1/repositories/myrepo/distributions/stable/architectures", token, map[string]string{"name": "amd64"})
}

func TestAPTIndicesAndPool(t *testing.T) {
	srv, token, pub := newServer(t)
	setupRepo(t, srv, token)

	// Push a real .deb via the REST API.
	debBytes := buildDeb("foo", "1.0", "amd64")
	upload(t, srv, token, "myrepo", "stable", "main", "foo_1.0_amd64.deb", debBytes)

	// Fetch the Packages index.
	code, body := get(t, srv, "/apt/myrepo/dists/stable/main/binary-amd64/Packages", "")
	if code != 200 {
		t.Fatalf("Packages status %d body %s", code, body)
	}
	if !bytes.Contains(body, []byte("Package: foo")) || !bytes.Contains(body, []byte("Filename: pool/main/f/foo/foo_1.0_amd64.deb")) {
		t.Fatalf("Packages index missing entry:\n%s", body)
	}

	// Fetch Release.
	code, rel := get(t, srv, "/apt/myrepo/dists/stable/Release", "")
	if code != 200 {
		t.Fatalf("Release status %d", code)
	}
	if !bytes.Contains(rel, []byte("Suite: stable")) || !bytes.Contains(rel, []byte("Components: main")) {
		t.Fatalf("Release missing fields:\n%s", rel)
	}

	// Fetch InRelease (clearsigned) and verify against the public key.
	code, inrel := get(t, srv, "/apt/myrepo/dists/stable/InRelease", "")
	if code != 200 {
		t.Fatalf("InRelease status %d", code)
	}
	if !bytes.Contains(inrel, []byte("BEGIN PGP SIGNED MESSAGE")) {
		t.Fatalf("InRelease not clearsigned:\n%s", inrel)
	}
	if _, err := gpg.VerifyClearSign(pub, inrel); err != nil {
		t.Fatalf("verify InRelease: %v", err)
	}

	// Fetch Release.gpg and verify as a detached signature of Release.
	code, sig := get(t, srv, "/apt/myrepo/dists/stable/Release.gpg", "")
	if code != 200 {
		t.Fatalf("Release.gpg status %d", code)
	}
	if err := gpg.VerifyDetached(pub, rel, sig); err != nil {
		t.Fatalf("verify Release.gpg: %v", err)
	}

	// Fetch the pool .deb and compare bytes.
	code, deb := get(t, srv, "/apt/myrepo/pool/main/f/foo/foo_1.0_amd64.deb", "")
	if code != 200 {
		t.Fatalf("pool status %d", code)
	}
	if !bytes.Equal(deb, debBytes) {
		t.Fatalf("pool bytes mismatch (%d vs %d)", len(deb), len(debBytes))
	}
}

func TestPrivateRepoRequiresAuth(t *testing.T) {
	srv, token, _ := newServer(t)
	// private repo
	post(t, srv, "/api/v1/repositories", token, map[string]any{"name": "priv", "visibility": "private"})
	post(t, srv, "/api/v1/repositories/priv/distributions", token, map[string]string{"name": "stable"})
	post(t, srv, "/api/v1/repositories/priv/distributions/stable/components", token, map[string]string{"name": "main"})
	post(t, srv, "/api/v1/repositories/priv/distributions/stable/architectures", token, map[string]string{"name": "amd64"})
	upload(t, srv, token, "priv", "stable", "main", "foo_1.0_amd64.deb", buildDeb("foo", "1.0", "amd64"))

	// anonymous → 401
	code, _ := get(t, srv, "/apt/priv/dists/stable/Release", "")
	if code != 401 {
		t.Fatalf("anonymous private repo should be 401, got %d", code)
	}

	// with token as basic auth password → 200
	req, _ := http.NewRequest("GET", srv.URL+"/apt/priv/dists/stable/Release", nil)
	req.SetBasicAuth("owner", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("authenticated private repo should be 200, got %d", resp.StatusCode)
	}
}

// keep context import used
var _ = context.Background
