package apiclient_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"urapt/server/aptrepo"
	"urapt/server/auth"
	"urapt/server/cache"
	"urapt/server/restapi"
	"urapt/server/store"
	"urapt/shared/apiclient"
	"urapt/shared/config"
	"urapt/shared/db"
	"urapt/shared/gpg"
)

func newServer(t *testing.T) (baseURL string, cleanup func()) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	st := store.New(database)
	authSvc := auth.NewService(st)
	key, err := gpg.GenerateKey("urapt-test <t>", 2048)
	if err != nil {
		t.Fatalf("gpg: %v", err)
	}
	sp := &signer{key: key}
	cfg := config.Defaults
	cfg.StoreDir = dir
	cfg.PackagesDir = filepath.Join(dir, "packages")
	cfg.DBPath = filepath.Join(dir, "test.db")
	_ = os.MkdirAll(cfg.PackagesDir, 0o755)

	idxCache := cache.New()
	root := chi.NewRouter()
	root.Mount("/api/v1", restapi.New(st, authSvc, sp, &cfg, idxCache))
	root.Mount("/apt", aptrepo.New(st, authSvc, key, idxCache))
	srv := newHTTPServer(root)
	return srv.URL, func() { srv.Close(); database.Close() }
}

type signer struct{ key *gpg.Key }

func (s *signer) PublicKeyArmored() (string, error) { return s.key.ArmoredPublic() }
func (s *signer) Fingerprint() string               { return s.key.Fingerprint }

func TestClientEndToEnd(t *testing.T) {
	baseURL, cleanup := newServer(t)
	defer cleanup()

	unauth := apiclient.New(baseURL, "")
	user, token, err := unauth.Register("alice", "supersecret")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if user.Username != "alice" || token == "" {
		t.Fatalf("bad register response: %+v", user)
	}

	c := apiclient.New(baseURL, token)

	if _, err := c.Me(); err != nil {
		t.Fatalf("Me: %v", err)
	}

	repo, err := c.CreateRepository("myrepo", "public", "test")
	if err != nil {
		t.Fatalf("CreateRepository: %v", err)
	}
	if repo.Name != "myrepo" {
		t.Fatalf("bad repo: %+v", repo)
	}

	if _, err := c.CreateDistribution("myrepo", "stable"); err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}
	if _, err := c.CreateComponent("myrepo", "stable", "main"); err != nil {
		t.Fatalf("CreateComponent: %v", err)
	}
	if _, err := c.CreateArchitecture("myrepo", "stable", "amd64"); err != nil {
		t.Fatalf("CreateArchitecture: %v", err)
	}

	// Build a fixture deb and push it.
	debBytes := buildDeb("hello", "1.0", "amd64")
	debPath := filepath.Join(t.TempDir(), "hello_1.0_amd64.deb")
	if err := os.WriteFile(debPath, debBytes, 0o644); err != nil {
		t.Fatalf("write deb: %v", err)
	}
	pkg, err := c.PushPackage("myrepo", "stable", "main", debPath)
	if err != nil {
		t.Fatalf("PushPackage: %v", err)
	}
	if pkg.Name != "hello" || pkg.SHA256 == "" {
		t.Fatalf("bad package: %+v", pkg)
	}

	list, err := c.ListPackages("myrepo", "stable", map[string]string{"name": "hello"})
	if err != nil {
		t.Fatalf("ListPackages: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 package, got %d", len(list.Items))
	}

	got, err := c.GetPackage("myrepo", pkg.ID)
	if err != nil {
		t.Fatalf("GetPackage: %v", err)
	}
	if got.ID != pkg.ID {
		t.Fatalf("GetPackage mismatch")
	}

	// Download and compare bytes.
	dlPath := filepath.Join(t.TempDir(), "pulled.deb")
	if err := c.DownloadPackage("myrepo", pkg.ID, dlPath); err != nil {
		t.Fatalf("DownloadPackage: %v", err)
	}
	dl, _ := os.ReadFile(dlPath)
	if !bytes.Equal(dl, debBytes) {
		t.Fatalf("downloaded bytes mismatch (%d vs %d)", len(dl), len(debBytes))
	}

	// apt-config helpers.
	if _, err := c.RepositoryPubkey("myrepo"); err != nil {
		t.Fatalf("RepositoryPubkey: %v", err)
	}

	// Delete the package.
	if err := c.DeletePackage("myrepo", pkg.ID); err != nil {
		t.Fatalf("DeletePackage: %v", err)
	}
	list, _ = c.ListPackages("myrepo", "stable", nil)
	if len(list.Items) != 0 {
		t.Fatalf("expected 0 packages after delete, got %d", len(list.Items))
	}
}
