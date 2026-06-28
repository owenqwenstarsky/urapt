package restapi

import (
	"encoding/json"
	"testing"
)

func TestRepoCreateAndPermissions(t *testing.T) {
	h := newHarness(t)
	// alice (owner/admin), bob (non-admin)
	_, body := h.do("POST", "/api/v1/auth/register", "", map[string]string{"username": "alice", "password": "supersecret"})
	var alice struct {
		User  map[string]any `json:"user"`
		Token string         `json:"token"`
	}
	json.Unmarshal(body, &alice)

	_, body = h.do("POST", "/api/v1/auth/register", "", map[string]string{"username": "bob", "password": "supersecret"})
	var bob struct {
		User  map[string]any `json:"user"`
		Token string         `json:"token"`
	}
	json.Unmarshal(body, &bob)

	// alice creates a private repo
	code, body := h.do("POST", "/api/v1/repositories", alice.Token, map[string]any{
		"name": "myrepo", "visibility": "private", "description": "test",
	})
	if code != 201 {
		t.Fatalf("create repo status %d body %s", code, body)
	}

	// bob cannot see it (private, not a member)
	code, _ = h.do("GET", "/api/v1/repositories/myrepo", bob.Token, nil)
	if code != 403 {
		t.Fatalf("bob should be forbidden from private repo, got %d", code)
	}

	// alice grants bob read access
	code, _ = h.do("POST", "/api/v1/repositories/myrepo/members", alice.Token, map[string]string{
		"username": "bob", "access": "read",
	})
	if code != 201 {
		t.Fatalf("add member status %d", code)
	}

	// bob can now read
	code, _ = h.do("GET", "/api/v1/repositories/myrepo", bob.Token, nil)
	if code != 200 {
		t.Fatalf("bob should read after grant, got %d", code)
	}

	// bob cannot write (read only)
	code, _ = h.do("POST", "/api/v1/repositories/myrepo/distributions", bob.Token, map[string]string{"name": "stable"})
	if code != 403 {
		t.Fatalf("bob read-only should not write, got %d", code)
	}

	// alice upgrades bob to write
	h.do("PATCH", "/api/v1/repositories/myrepo/members/bob", alice.Token, map[string]string{"access": "write"})
	code, _ = h.do("POST", "/api/v1/repositories/myrepo/distributions", bob.Token, map[string]string{"name": "stable"})
	if code != 201 {
		t.Fatalf("bob with write should create distro, got %d", code)
	}

	// add component and arch
	h.do("POST", "/api/v1/repositories/myrepo/distributions/stable/components", alice.Token, map[string]string{"name": "main"})
	code, _ = h.do("POST", "/api/v1/repositories/myrepo/distributions/stable/architectures", alice.Token, map[string]string{"name": "amd64"})
	if code != 201 {
		t.Fatalf("add arch status %d", code)
	}
	// 'all' arch rejected
	code, _ = h.do("POST", "/api/v1/repositories/myrepo/distributions/stable/architectures", alice.Token, map[string]string{"name": "all"})
	if code != 400 {
		t.Fatalf("all arch should be rejected, got %d", code)
	}

	// list distros/components/arches
	code, _ = h.do("GET", "/api/v1/repositories/myrepo/distributions", alice.Token, nil)
	if code != 200 {
		t.Fatalf("list distros status %d", code)
	}
	code, _ = h.do("GET", "/api/v1/repositories/myrepo/distributions/stable/components", alice.Token, nil)
	if code != 200 {
		t.Fatalf("list components status %d", code)
	}

	// remove member
	code, _ = h.do("DELETE", "/api/v1/repositories/myrepo/members/bob", alice.Token, nil)
	if code != 204 {
		t.Fatalf("remove member status %d", code)
	}
	code, _ = h.do("GET", "/api/v1/repositories/myrepo", bob.Token, nil)
	if code != 403 {
		t.Fatalf("bob should be forbidden after removal, got %d", code)
	}
}

func TestPublicRepoReadableByAll(t *testing.T) {
	h := newHarness(t)
	_, body := h.do("POST", "/api/v1/auth/register", "", map[string]string{"username": "owner", "password": "supersecret"})
	var owner struct {
		Token string `json:"token"`
	}
	json.Unmarshal(body, &owner)
	_, body = h.do("POST", "/api/v1/auth/register", "", map[string]string{"username": "stranger", "password": "supersecret"})
	var stranger struct {
		Token string `json:"token"`
	}
	json.Unmarshal(body, &stranger)

	h.do("POST", "/api/v1/repositories", owner.Token, map[string]any{"name": "pubrepo", "visibility": "public"})
	code, _ := h.do("GET", "/api/v1/repositories/pubrepo", stranger.Token, nil)
	if code != 200 {
		t.Fatalf("stranger should read public repo, got %d", code)
	}
	// but stranger cannot write
	code, _ = h.do("POST", "/api/v1/repositories/pubrepo/distributions", stranger.Token, map[string]string{"name": "x"})
	if code != 403 {
		t.Fatalf("stranger should not write public repo, got %d", code)
	}
}
