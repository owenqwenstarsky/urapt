package store

import (
	"context"
	"errors"
	"testing"

	"urapt/shared/models"
)

func TestCreateRepository(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")

	r := s.createRepo(t, ctx, "myrepo", u.ID, models.VisibilityPublic)
	if r.Name != "myrepo" || r.OwnerUserID != u.ID || r.Visibility != models.VisibilityPublic {
		t.Fatalf("repo = %+v", r)
	}
	if r.ID == "" || r.CreatedAt == "" {
		t.Fatal("id/created_at should be set")
	}
}

func TestGetRepositoryByNameAndID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	r := s.createRepo(t, ctx, "myrepo", u.ID, models.VisibilityPublic)

	byName, err := s.GetRepositoryByName(ctx, "myrepo")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if byName.ID != r.ID {
		t.Fatalf("id mismatch")
	}
	byID, err := s.GetRepositoryByID(ctx, r.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if byID.Name != "myrepo" {
		t.Fatalf("name = %q", byID.Name)
	}
	if _, err := s.GetRepositoryByName(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListReposVisible(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	alice := s.createUser(t, ctx, "alice", "p")
	bob := s.createUser(t, ctx, "bob", "p")
	carol := s.createUser(t, ctx, "carol", "p")

	// alice owns: pub-own, priv-own
	pubOwn := s.createRepo(t, ctx, "pub-own", alice.ID, models.VisibilityPublic)
	privOwn := s.createRepo(t, ctx, "priv-own", alice.ID, models.VisibilityPrivate)
	// bob owns: pub-bob, priv-bob, and grants carol read on priv-bob
	pubBob := s.createRepo(t, ctx, "pub-bob", bob.ID, models.VisibilityPublic)
	privBob := s.createRepo(t, ctx, "priv-bob", bob.ID, models.VisibilityPrivate)
	_ = pubOwn
	_ = privOwn
	_ = pubBob
	if err := s.AddMember(ctx, privBob.ID, carol.ID, models.AccessRead); err != nil {
		t.Fatalf("add member: %v", err)
	}

	// Alice sees: her two + bob's public. NOT bob's private.
	aliceRepos, err := s.ListReposVisible(ctx, alice.ID)
	if err != nil {
		t.Fatalf("alice list: %v", err)
	}
	if len(aliceRepos) != 3 {
		t.Fatalf("alice should see 3 repos, got %d", len(aliceRepos))
	}

	// Carol sees: pub-own, pub-bob (both public) + priv-bob (member). NOT priv-own.
	carolRepos, err := s.ListReposVisible(ctx, carol.ID)
	if err != nil {
		t.Fatalf("carol list: %v", err)
	}
	if len(carolRepos) != 3 {
		t.Fatalf("carol should see 3 repos, got %d", len(carolRepos))
	}
	// Verify priv-bob is among carol's repos.
	foundPrivBob := false
	for _, r := range carolRepos {
		if r.ID == privBob.ID {
			foundPrivBob = true
		}
		if r.ID == privOwn.ID {
			t.Fatal("carol should not see alice's private repo")
		}
	}
	if !foundPrivBob {
		t.Fatal("carol should see priv-bob as a member")
	}
}

func TestUpdateRepository(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	r := s.createRepo(t, ctx, "myrepo", u.ID, models.VisibilityPublic)

	// Update name only.
	if err := s.UpdateRepository(ctx, r.ID, "newname", nil, nil); err != nil {
		t.Fatalf("update name: %v", err)
	}
	got, _ := s.GetRepositoryByID(ctx, r.ID)
	if got.Name != "newname" || got.Visibility != models.VisibilityPublic {
		t.Fatalf("name=%q vis=%s", got.Name, got.Visibility)
	}

	// Update visibility only.
	priv := models.VisibilityPrivate
	_ = s.UpdateRepository(ctx, r.ID, "", &priv, nil)
	got, _ = s.GetRepositoryByID(ctx, r.ID)
	if got.Visibility != models.VisibilityPrivate || got.Name != "newname" {
		t.Fatalf("vis=%s name=%q", got.Visibility, got.Name)
	}

	// Update description only.
	desc := "a repo"
	_ = s.UpdateRepository(ctx, r.ID, "", nil, &desc)
	got, _ = s.GetRepositoryByID(ctx, r.ID)
	if got.Description != "a repo" {
		t.Fatalf("desc=%q", got.Description)
	}
}

func TestDeleteRepository(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := s.createUser(t, ctx, "alice", "p")
	r := s.createRepo(t, ctx, "myrepo", u.ID, models.VisibilityPublic)

	if err := s.DeleteRepository(ctx, r.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetRepositoryByID(ctx, r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

// --- members ---

func TestAddMember_Upsert(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	alice := s.createUser(t, ctx, "alice", "p")
	bob := s.createUser(t, ctx, "bob", "p")
	repo := s.createRepo(t, ctx, "repo", alice.ID, models.VisibilityPrivate)

	// Initial grant: read.
	if err := s.AddMember(ctx, repo.ID, bob.ID, models.AccessRead); err != nil {
		t.Fatalf("add: %v", err)
	}
	access, ok, err := s.GetMemberAccess(ctx, repo.ID, bob.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok || access != models.AccessRead {
		t.Fatalf("expected read grant, got ok=%v access=%s", ok, access)
	}

	// Upsert to write.
	if err := s.AddMember(ctx, repo.ID, bob.ID, models.AccessWrite); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	access, ok, _ = s.GetMemberAccess(ctx, repo.ID, bob.ID)
	if !ok || access != models.AccessWrite {
		t.Fatalf("expected write after upsert, got %s", access)
	}
}

func TestGetMemberAccess_NonMember(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	alice := s.createUser(t, ctx, "alice", "p")
	bob := s.createUser(t, ctx, "bob", "p")
	repo := s.createRepo(t, ctx, "repo", alice.ID, models.VisibilityPrivate)

	_, ok, err := s.GetMemberAccess(ctx, repo.ID, bob.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ok {
		t.Fatal("non-member should return ok=false")
	}
}

func TestUpdateMemberAccess_NotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	alice := s.createUser(t, ctx, "alice", "p")
	bob := s.createUser(t, ctx, "bob", "p")
	repo := s.createRepo(t, ctx, "repo", alice.ID, models.VisibilityPrivate)

	if err := s.UpdateMemberAccess(ctx, repo.ID, bob.ID, models.AccessRead); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update non-member should be ErrNotFound, got %v", err)
	}
}

func TestRemoveMember(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	alice := s.createUser(t, ctx, "alice", "p")
	bob := s.createUser(t, ctx, "bob", "p")
	repo := s.createRepo(t, ctx, "repo", alice.ID, models.VisibilityPrivate)

	_ = s.AddMember(ctx, repo.ID, bob.ID, models.AccessRead)
	if err := s.RemoveMember(ctx, repo.ID, bob.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	_, ok, _ := s.GetMemberAccess(ctx, repo.ID, bob.ID)
	if ok {
		t.Fatal("member should be gone after remove")
	}
	// Removing again returns ErrNotFound.
	if err := s.RemoveMember(ctx, repo.ID, bob.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second remove should be ErrNotFound, got %v", err)
	}
}

func TestListMembers(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	alice := s.createUser(t, ctx, "alice", "p")
	bob := s.createUser(t, ctx, "bob", "p")
	carol := s.createUser(t, ctx, "carol", "p")
	repo := s.createRepo(t, ctx, "repo", alice.ID, models.VisibilityPrivate)

	_ = s.AddMember(ctx, repo.ID, carol.ID, models.AccessRead)
	_ = s.AddMember(ctx, repo.ID, bob.ID, models.AccessWrite)

	members, err := s.ListMembers(ctx, repo.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	// Ordered by username: bob, carol.
	if members[0].User.Username != "bob" || members[0].Access != models.AccessWrite {
		t.Fatalf("member[0] = %+v", members[0])
	}
	if members[1].User.Username != "carol" || members[1].Access != models.AccessRead {
		t.Fatalf("member[1] = %+v", members[1])
	}
}
