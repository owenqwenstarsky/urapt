package store

import (
	"context"
	"errors"
	"testing"
)

// --- distributions ---

func TestCreateDistributionAndGetByName(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, "public")

	d := s.createDistro(t, ctx, repo.ID, "stable")
	if d.Name != "stable" || d.RepositoryID != repo.ID {
		t.Fatalf("distro = %+v", d)
	}

	got, err := s.GetDistributionByName(ctx, repo.ID, "stable")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != d.ID {
		t.Fatal("id mismatch")
	}
	if _, err := s.GetDistributionByName(ctx, repo.ID, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListDistributions(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, "public")
	s.createDistro(t, ctx, repo.ID, "stable")
	s.createDistro(t, ctx, repo.ID, "unstable")
	s.createDistro(t, ctx, repo.ID, "oldstable")

	ds, err := s.ListDistributions(ctx, repo.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(ds) != 3 {
		t.Fatalf("expected 3 distros, got %d", len(ds))
	}
	// Ordered by name.
	if ds[0].Name != "oldstable" || ds[1].Name != "stable" || ds[2].Name != "unstable" {
		names := []string{ds[0].Name, ds[1].Name, ds[2].Name}
		t.Fatalf("expected sorted, got %v", names)
	}
}

func TestDeleteDistribution(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, "public")
	s.createDistro(t, ctx, repo.ID, "stable")

	if err := s.DeleteDistribution(ctx, repo.ID, "stable"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetDistributionByName(ctx, repo.ID, "stable"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	if err := s.DeleteDistribution(ctx, repo.ID, "stable"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete should be ErrNotFound, got %v", err)
	}
}

// --- components ---

func TestCreateComponentAndList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, "public")
	d := s.createDistro(t, ctx, repo.ID, "stable")

	s.createComponent(t, ctx, d.ID, "main")
	s.createComponent(t, ctx, d.ID, "contrib")
	s.createComponent(t, ctx, d.ID, "non-free")

	cs, err := s.ListComponents(ctx, d.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(cs) != 3 {
		t.Fatalf("expected 3 components, got %d", len(cs))
	}
	if cs[0].Name != "contrib" {
		t.Fatalf("expected sorted; first = %q", cs[0].Name)
	}
}

func TestDeleteComponent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, "public")
	d := s.createDistro(t, ctx, repo.ID, "stable")
	s.createComponent(t, ctx, d.ID, "main")

	if err := s.DeleteComponent(ctx, d.ID, "main"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.DeleteComponent(ctx, d.ID, "main"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete should be ErrNotFound, got %v", err)
	}
}

func TestCountComponentsByDistro(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, "public")
	d := s.createDistro(t, ctx, repo.ID, "stable")

	n, err := s.CountComponentsByDistro(ctx, d.ID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
	s.createComponent(t, ctx, d.ID, "main")
	s.createComponent(t, ctx, d.ID, "contrib")
	n, _ = s.CountComponentsByDistro(ctx, d.ID)
	if n != 2 {
		t.Fatalf("expected 2, got %d", n)
	}
}

// --- architectures ---

func TestCreateArchitectureAndList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, "public")
	d := s.createDistro(t, ctx, repo.ID, "stable")

	if _, err := s.CreateArchitecture(ctx, d.ID, "amd64"); err != nil {
		t.Fatalf("create amd64: %v", err)
	}
	if _, err := s.CreateArchitecture(ctx, d.ID, "arm64"); err != nil {
		t.Fatalf("create arm64: %v", err)
	}

	arches, err := s.ListArchitectures(ctx, d.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(arches) != 2 {
		t.Fatalf("expected 2 arches, got %d", len(arches))
	}
	if arches[0].Name != "amd64" || arches[1].Name != "arm64" {
		t.Fatalf("expected sorted, got %q %q", arches[0].Name, arches[1].Name)
	}
}

func TestDeleteArchitecture(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, "public")
	d := s.createDistro(t, ctx, repo.ID, "stable")
	_, _ = s.CreateArchitecture(ctx, d.ID, "amd64")

	if err := s.DeleteArchitecture(ctx, d.ID, "amd64"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.DeleteArchitecture(ctx, d.ID, "amd64"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete should be ErrNotFound, got %v", err)
	}
}

func TestHasArchitecture(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	repo := s.createRepo(t, ctx, "repo", u.ID, "public")
	d := s.createDistro(t, ctx, repo.ID, "stable")
	_, _ = s.CreateArchitecture(ctx, d.ID, "amd64")

	has, err := s.HasArchitecture(ctx, d.ID, "amd64")
	if err != nil {
		t.Fatalf("has amd64: %v", err)
	}
	if !has {
		t.Fatal("expected has=true for amd64")
	}
	has, err = s.HasArchitecture(ctx, d.ID, "arm64")
	if err != nil {
		t.Fatalf("has arm64: %v", err)
	}
	if has {
		t.Fatal("expected has=false for arm64")
	}
}
