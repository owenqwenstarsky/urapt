package store

import (
	"context"
	"errors"
	"testing"

	"urapt/shared/models"
)

// newPackage creates a minimal valid Package row ready to insert.
func newPackage(repoID, distroID, compID, userID string) *models.Package {
	return &models.Package{
		RepositoryID:     repoID,
		DistributionID:   distroID,
		ComponentID:      compID,
		Name:             "myapp-hello",
		Version:          "1.0.0",
		Architecture:     "amd64",
		Maintainer:       "Test <test@example.com>",
		Description:      "a test package",
		RawControl:       "Package: myapp-hello\nVersion: 1.0.0\n",
		Filename:         "myapp-hello_1.0.0_amd64.deb",
		PoolPath:         "pool/main/m/myapp-hello/myapp-hello_1.0.0_amd64.deb",
		Size:             712,
		MD5sum:           "d41d8cd98f00b204e9800998ecf8427e",
		SHA1:             "da39a3ee5e6b4b0d3255bfef95601890afd80709",
		SHA256:           "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		UploadedByUserID: userID,
	}
}

func TestCreatePackageAndGetByID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, models.VisibilityPublic)
	d := s.createDistro(t, ctx, repo.ID, "stable")
	c := s.createComponent(t, ctx, d.ID, "main")
	_ = c

	p := newPackage(repo.ID, d.ID, c.ID, u.ID)
	if err := s.CreatePackage(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID == "" || p.CreatedAt == "" {
		t.Fatal("id/created_at should be populated by CreatePackage")
	}

	got, err := s.GetPackageByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Name != "myapp-hello" || got.Version != "1.0.0" || got.Architecture != "amd64" {
		t.Fatalf("got = %+v", got)
	}
	if got.SHA256 != p.SHA256 {
		t.Fatalf("sha256 mismatch")
	}
}

func TestGetPackageByID_NotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, err := s.GetPackageByID(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetPackageByPoolPath(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, models.VisibilityPublic)
	d := s.createDistro(t, ctx, repo.ID, "stable")
	c := s.createComponent(t, ctx, d.ID, "main")

	p := newPackage(repo.ID, d.ID, c.ID, u.ID)
	_ = s.CreatePackage(ctx, p)

	got, err := s.GetPackageByPoolPath(ctx, repo.ID, p.PoolPath)
	if err != nil {
		t.Fatalf("get by pool path: %v", err)
	}
	if got.ID != p.ID {
		t.Fatal("id mismatch")
	}
	// Wrong repo.
	if _, err := s.GetPackageByPoolPath(ctx, "other-repo", p.PoolPath); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for wrong repo, got %v", err)
	}
}

func TestListPackages_FiltersAndPagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, models.VisibilityPublic)
	d := s.createDistro(t, ctx, repo.ID, "stable")
	main := s.createComponent(t, ctx, d.ID, "main")
	contrib := s.createComponent(t, ctx, d.ID, "contrib")

	// Insert 5 packages: 3 in main, 2 in contrib; mix of amd64 and arm64.
	pkgs := []*models.Package{
		newPackage(repo.ID, d.ID, main.ID, u.ID),
		newPackage(repo.ID, d.ID, main.ID, u.ID),
		newPackage(repo.ID, d.ID, main.ID, u.ID),
		newPackage(repo.ID, d.ID, contrib.ID, u.ID),
		newPackage(repo.ID, d.ID, contrib.ID, u.ID),
	}
	names := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	arches := []string{"amd64", "amd64", "arm64", "amd64", "arm64"}
	for i, p := range pkgs {
		p.Name = names[i]
		p.Architecture = arches[i]
		p.PoolPath = "pool/main/m/" + names[i] + "/" + names[i] + "_1.0.0_" + arches[i] + ".deb"
		p.Filename = names[i] + "_1.0.0_" + arches[i] + ".deb"
		if err := s.CreatePackage(ctx, p); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}

	// All.
	list, total, err := s.ListPackages(ctx, repo.ID, d.ID, PackageFilters{}, 1, 100)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 5 || len(list) != 5 {
		t.Fatalf("expected total=5 len=5, got total=%d len=%d", total, len(list))
	}

	// Filter by component.
	list, total, _ = s.ListPackages(ctx, repo.ID, d.ID, PackageFilters{ComponentID: main.ID}, 1, 100)
	if total != 3 || len(list) != 3 {
		t.Fatalf("main filter: expected 3, got total=%d len=%d", total, len(list))
	}

	// Filter by arch (matches arch OR 'all').
	_, total, _ = s.ListPackages(ctx, repo.ID, d.ID, PackageFilters{Arch: "amd64"}, 1, 100)
	if total != 3 { // alpha, beta, delta
		t.Fatalf("amd64 filter: expected 3, got %d", total)
	}
	_, total, _ = s.ListPackages(ctx, repo.ID, d.ID, PackageFilters{Arch: "arm64"}, 1, 100)
	if total != 2 { // gamma, epsilon
		t.Fatalf("arm64 filter: expected 2, got %d", total)
	}

	// Filter by exact name.
	list, total, _ = s.ListPackages(ctx, repo.ID, d.ID, PackageFilters{Name: "beta"}, 1, 100)
	if total != 1 || len(list) != 1 || list[0].Name != "beta" {
		t.Fatalf("name filter: total=%d list=%v", total, list)
	}

	// Query (LIKE on name/description).
	_, total, _ = s.ListPackages(ctx, repo.ID, d.ID, PackageFilters{Query: "a test"}, 1, 100)
	if total != 5 {
		t.Fatalf("query filter: expected all 5 to match description, got %d", total)
	}

	// Pagination: page 1, perPage 2 -> 2 items, total 5.
	list, total, _ = s.ListPackages(ctx, repo.ID, d.ID, PackageFilters{}, 1, 2)
	if total != 5 || len(list) != 2 {
		t.Fatalf("page 1: total=%d len=%d", total, len(list))
	}
	// page 3 -> only 1 item.
	list, _, _ = s.ListPackages(ctx, repo.ID, d.ID, PackageFilters{}, 3, 2)
	if len(list) != 1 {
		t.Fatalf("page 3: expected 1 item, got %d", len(list))
	}
	_ = list
}

func TestDeletePackage(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, models.VisibilityPublic)
	d := s.createDistro(t, ctx, repo.ID, "stable")
	c := s.createComponent(t, ctx, d.ID, "main")
	p := newPackage(repo.ID, d.ID, c.ID, u.ID)
	_ = s.CreatePackage(ctx, p)

	if err := s.DeletePackage(ctx, p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetPackageByID(ctx, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeletePackagesByRepo(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, models.VisibilityPublic)
	d := s.createDistro(t, ctx, repo.ID, "stable")
	c := s.createComponent(t, ctx, d.ID, "main")
	_ = s.CreatePackage(ctx, newPackage(repo.ID, d.ID, c.ID, u.ID))
	_ = s.CreatePackage(ctx, newPackage(repo.ID, d.ID, c.ID, u.ID))

	if err := s.DeletePackagesByRepo(ctx, repo.ID); err != nil {
		t.Fatalf("delete by repo: %v", err)
	}
	list, total, _ := s.ListPackages(ctx, repo.ID, d.ID, PackageFilters{}, 1, 100)
	if total != 0 || len(list) != 0 {
		t.Fatalf("expected no packages after DeletePackagesByRepo, got total=%d", total)
	}
}

func TestListSuitePackages(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, models.VisibilityPublic)
	d := s.createDistro(t, ctx, repo.ID, "stable")
	c := s.createComponent(t, ctx, d.ID, "main")
	p := newPackage(repo.ID, d.ID, c.ID, u.ID)
	_ = s.CreatePackage(ctx, p)

	suite, err := s.ListSuitePackages(ctx, repo.ID, d.ID)
	if err != nil {
		t.Fatalf("list suite: %v", err)
	}
	if len(suite) != 1 {
		t.Fatalf("expected 1 suite package, got %d", len(suite))
	}
	if suite[0].ComponentName != "main" || suite[0].DistributionName != "stable" {
		t.Fatalf("component=%q distro=%q", suite[0].ComponentName, suite[0].DistributionName)
	}
	if suite[0].Name != "myapp-hello" {
		t.Fatalf("name=%q", suite[0].Name)
	}
}

func TestListPackageBlobSHA256sByRepo(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, models.VisibilityPublic)
	d := s.createDistro(t, ctx, repo.ID, "stable")
	c := s.createComponent(t, ctx, d.ID, "main")

	p1 := newPackage(repo.ID, d.ID, c.ID, u.ID)
	p1.Name = "alpha"
	p1.SHA256 = "aaa"
	p1.PoolPath = "pool/main/m/alpha/alpha.deb"
	p1.Filename = "alpha.deb"
	_ = s.CreatePackage(ctx, p1)
	p2 := newPackage(repo.ID, d.ID, c.ID, u.ID)
	p2.Name = "beta"
	p2.SHA256 = "bbb"
	p2.PoolPath = "pool/main/m/beta/beta.deb"
	p2.Filename = "beta.deb"
	_ = s.CreatePackage(ctx, p2)

	shas, err := s.ListPackageBlobSHA256sByRepo(ctx, repo.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(shas) != 2 {
		t.Fatalf("expected 2 shas, got %d", len(shas))
	}
}

// --- blobs ---

func TestCreateBlob_NewAndExisting(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	created, err := s.CreateBlob(ctx, "sha-aaa", "hello.deb", 712)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !created {
		t.Fatal("first CreateBlob should report created=true")
	}
	// Second time, same sha -> not created, no error.
	created, err = s.CreateBlob(ctx, "sha-aaa", "hello.deb", 712)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if created {
		t.Fatal("second CreateBlob should report created=false (already exists)")
	}
}

func TestGetBlob(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	_, _ = s.CreateBlob(ctx, "sha-aaa", "hello.deb", 712)

	filename, size, refCount, err := s.GetBlob(ctx, "sha-aaa")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if filename != "hello.deb" || size != 712 || refCount != 1 {
		t.Fatalf("got filename=%q size=%d ref=%d", filename, size, refCount)
	}
	if _, _, _, err := s.GetBlob(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIncBlobRef(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	_, _ = s.CreateBlob(ctx, "sha-aaa", "hello.deb", 712)

	rc, err := s.IncBlobRef(ctx, "sha-aaa")
	if err != nil {
		t.Fatalf("inc: %v", err)
	}
	if rc != 2 {
		t.Fatalf("expected ref=2 after inc, got %d", rc)
	}
	rc, _ = s.IncBlobRef(ctx, "sha-aaa")
	if rc != 3 {
		t.Fatalf("expected ref=3, got %d", rc)
	}
	// Inc on missing blob.
	if _, err := s.IncBlobRef(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDecBlobRef(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	_, _ = s.CreateBlob(ctx, "sha-aaa", "hello.deb", 712)
	_, _ = s.IncBlobRef(ctx, "sha-aaa") // ref=2

	rc, err := s.DecBlobRef(ctx, "sha-aaa")
	if err != nil {
		t.Fatalf("dec: %v", err)
	}
	if rc != 1 {
		t.Fatalf("expected ref=1 after dec, got %d", rc)
	}
	rc, _ = s.DecBlobRef(ctx, "sha-aaa")
	if rc != 0 {
		t.Fatalf("expected ref=0, got %d", rc)
	}
	// Dec below 0 should not go negative; ref_count > 0 guard.
	rc, _ = s.DecBlobRef(ctx, "sha-aaa")
	if rc != 0 {
		t.Fatalf("expected ref to stay at 0, got %d", rc)
	}
	// Dec on missing blob.
	if _, err := s.DecBlobRef(ctx, "totally-missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing, got %v", err)
	}
}

func TestDeleteBlob(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	_, _ = s.CreateBlob(ctx, "sha-aaa", "hello.deb", 712)

	if err := s.DeleteBlob(ctx, "sha-aaa"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, _, _, err := s.GetBlob(ctx, "sha-aaa"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
