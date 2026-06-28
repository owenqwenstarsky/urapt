package restapi

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"urapt/server/middleware"
	"urapt/server/store"
	apitypes "urapt/shared/api"
	"urapt/shared/httputil"
	"urapt/shared/models"
)

// nameRE is the shared validator for repository, distribution, component, and
// architecture names: lowercase, starting alphanumeric, allowing -+., length
// 1-64.
var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9+.\-]{0,63}$`)

// loadRepoByName fetches a repository by its {repo} path param, rendering the
// appropriate error. nil is returned only after an error has been written.
func (api *API) loadRepoByName(w http.ResponseWriter, r *http.Request) *models.Repository {
	name := r.PathValue("repo")
	repo, err := api.Store.GetRepositoryByName(r.Context(), name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "repository not found")
			return nil
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to read repository")
		return nil
	}
	return repo
}

// requireRead loads the repo and checks CanRead; returns the repo or nil (after
// writing an error).
func (api *API) requireRead(w http.ResponseWriter, r *http.Request) *models.Repository {
	repo := api.loadRepoByName(w, r)
	if repo == nil {
		return nil
	}
	id := middleware.IdentityFromContext(r.Context())
	ok, err := api.Auth.CanRead(r.Context(), id.User, repo)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "permission check failed")
		return nil
	}
	if !ok {
		httputil.WriteError(w, http.StatusForbidden, httputil.CodeForbidden, "no read access")
		return nil
	}
	return repo
}

// requireWrite loads the repo and checks CanWrite.
func (api *API) requireWrite(w http.ResponseWriter, r *http.Request) *models.Repository {
	repo := api.loadRepoByName(w, r)
	if repo == nil {
		return nil
	}
	id := middleware.IdentityFromContext(r.Context())
	ok, err := api.Auth.CanWrite(r.Context(), id.User, repo)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "permission check failed")
		return nil
	}
	if !ok {
		httputil.WriteError(w, http.StatusForbidden, httputil.CodeForbidden, "no write access")
		return nil
	}
	return repo
}

// requireManage loads the repo and checks CanManage.
func (api *API) requireManage(w http.ResponseWriter, r *http.Request) *models.Repository {
	repo := api.loadRepoByName(w, r)
	if repo == nil {
		return nil
	}
	id := middleware.IdentityFromContext(r.Context())
	ok, err := api.Auth.CanManage(r.Context(), id.User, repo)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "permission check failed")
		return nil
	}
	if !ok {
		httputil.WriteError(w, http.StatusForbidden, httputil.CodeForbidden, "manage access required")
		return nil
	}
	return repo
}

// ListRepositories returns repositories visible to the caller.
func (api *API) ListRepositories(w http.ResponseWriter, r *http.Request) {
	id := middleware.IdentityFromContext(r.Context())
	repos, err := api.Store.ListReposVisible(r.Context(), id.User.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to list repositories")
		return
	}
	if repos == nil {
		repos = []*models.Repository{}
	}
	httputil.WriteJSON(w, http.StatusOK, apitypes.ListResponse[*models.Repository]{Items: repos, Page: 1, PerPage: 100, Total: len(repos)})
}

// CreateRepository creates a new repository owned by the caller.
func (api *API) CreateRepository(w http.ResponseWriter, r *http.Request) {
	var req apitypes.CreateRepoRequest
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		bad(w, "invalid JSON body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !nameRE.MatchString(req.Name) {
		bad(w, "name must be 1-64 chars of [a-z0-9][a-z0-9+.-]")
		return
	}
	vis := models.Visibility(strings.ToLower(req.Visibility))
	if vis != models.VisibilityPublic && vis != models.VisibilityPrivate {
		bad(w, "visibility must be 'public' or 'private'")
		return
	}
	id := middleware.IdentityFromContext(r.Context())
	if _, err := api.Store.GetRepositoryByName(r.Context(), req.Name); err == nil {
		httputil.WriteError(w, http.StatusConflict, httputil.CodeConflict, "repository name already taken")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to check name")
		return
	}
	repo, err := api.Store.CreateRepository(r.Context(), req.Name, id.User.ID, vis, req.Description)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to create repository")
		return
	}
	_ = api.Store.RecordAudit(r.Context(), &id.User.ID, &repo.ID, "repo.create", repo.Name, "")
	httputil.WriteJSON(w, http.StatusCreated, repo)
}

// GetRepository returns a single repository.
func (api *API) GetRepository(w http.ResponseWriter, r *http.Request) {
	repo := api.requireRead(w, r)
	if repo == nil {
		return
	}
	httputil.WriteJSON(w, http.StatusOK, repo)
}

// UpdateRepository mutates a repository.
func (api *API) UpdateRepository(w http.ResponseWriter, r *http.Request) {
	repo := api.requireManage(w, r)
	if repo == nil {
		return
	}
	var req apitypes.UpdateRepoRequest
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		bad(w, "invalid JSON body")
		return
	}
	var vis *models.Visibility
	if req.Visibility != nil {
		v := models.Visibility(strings.ToLower(*req.Visibility))
		if v != models.VisibilityPublic && v != models.VisibilityPrivate {
			bad(w, "visibility must be 'public' or 'private'")
			return
		}
		vis = &v
	}
	if req.Name != nil {
		if !nameRE.MatchString(*req.Name) {
			bad(w, "name must be 1-64 chars of [a-z0-9][a-z0-9+.-]")
			return
		}
	}
	if err := api.Store.UpdateRepository(r.Context(), repo.ID, valStr(req.Name), vis, req.Description); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to update repository")
		return
	}
	api.Cache.InvalidateRepo(repo.ID)
	updated, _ := api.Store.GetRepositoryByID(r.Context(), repo.ID)
	httputil.WriteJSON(w, http.StatusOK, updated)
}

// DeleteRepository removes a repository and cleans up its blobs.
func (api *API) DeleteRepository(w http.ResponseWriter, r *http.Request) {
	repo := api.requireManage(w, r)
	if repo == nil {
		return
	}
	shas, err := api.Store.ListPackageBlobSHA256sByRepo(r.Context(), repo.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to list packages")
		return
	}
	if err := api.Store.DeletePackagesByRepo(r.Context(), repo.ID); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to delete packages")
		return
	}
	if err := api.Store.DeleteRepository(r.Context(), repo.ID); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to delete repository")
		return
	}
	for _, sha := range shas {
		rc, _ := api.Store.DecBlobRef(r.Context(), sha)
		if rc == 0 {
			_ = api.Store.DeleteBlob(r.Context(), sha)
			_ = os.Remove(api.blobFilePath(sha))
		}
	}
	api.Cache.InvalidateRepo(repo.ID)
	id := middleware.IdentityFromContext(r.Context())
	_ = api.Store.RecordAudit(r.Context(), &id.User.ID, &repo.ID, "repo.delete", repo.Name, "")
	w.WriteHeader(http.StatusNoContent)
}

// RepoPubkey returns the server's armored public key (convenience endpoint).
func (api *API) RepoPubkey(w http.ResponseWriter, r *http.Request) {
	if api.requireRead(w, r) == nil {
		return
	}
	api.ServerPubkey(w, r)
}

// ListMembers returns the members of a repository.
func (api *API) ListMembers(w http.ResponseWriter, r *http.Request) {
	repo := api.requireRead(w, r)
	if repo == nil {
		return
	}
	members, err := api.Store.ListMembers(r.Context(), repo.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to list members")
		return
	}
	if members == nil {
		members = []*models.RepositoryMember{}
	}
	httputil.WriteJSON(w, http.StatusOK, members)
}

// AddMember grants a user access on a repository.
func (api *API) AddMember(w http.ResponseWriter, r *http.Request) {
	repo := api.requireManage(w, r)
	if repo == nil {
		return
	}
	var req apitypes.AddMemberRequest
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		bad(w, "invalid JSON body")
		return
	}
	target, err := api.Store.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "user not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to read user")
		return
	}
	if !models.ValidAccess(req.Access) {
		bad(w, "access must be read, write, read-write, or admin")
		return
	}
	if target.ID == repo.OwnerUserID {
		bad(w, "cannot change owner's access")
		return
	}
	if err := api.Store.AddMember(r.Context(), repo.ID, target.ID, models.Access(req.Access)); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to add member")
		return
	}
	_ = api.Store.RecordAudit(r.Context(), &repo.OwnerUserID, &repo.ID, "member.add", req.Username, req.Access)
	httputil.WriteJSON(w, http.StatusCreated, &models.RepositoryMember{
		RepositoryID: repo.ID, UserID: target.ID, Access: models.Access(req.Access), User: target,
	})
}

// UpdateMember changes a member's access level.
func (api *API) UpdateMember(w http.ResponseWriter, r *http.Request) {
	repo := api.requireManage(w, r)
	if repo == nil {
		return
	}
	var req apitypes.UpdateMemberRequest
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		bad(w, "invalid JSON body")
		return
	}
	if !models.ValidAccess(req.Access) {
		bad(w, "access must be read, write, read-write, or admin")
		return
	}
	username := r.PathValue("username")
	target, err := api.Store.GetUserByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "user not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to read user")
		return
	}
	if target.ID == repo.OwnerUserID {
		bad(w, "cannot change owner's access")
		return
	}
	if err := api.Store.UpdateMemberAccess(r.Context(), repo.ID, target.ID, models.Access(req.Access)); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "member not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to update member")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, &models.RepositoryMember{
		RepositoryID: repo.ID, UserID: target.ID, Access: models.Access(req.Access), User: target,
	})
}

// RemoveMember revokes a user's access.
func (api *API) RemoveMember(w http.ResponseWriter, r *http.Request) {
	repo := api.requireManage(w, r)
	if repo == nil {
		return
	}
	username := r.PathValue("username")
	target, err := api.Store.GetUserByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "user not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to read user")
		return
	}
	if target.ID == repo.OwnerUserID {
		bad(w, "cannot remove owner")
		return
	}
	if err := api.Store.RemoveMember(r.Context(), repo.ID, target.ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "member not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to remove member")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// valStr returns s as a string pointer-or-nil.
func valStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// blobFilePath returns the on-disk path for a content-addressed blob.
func (api *API) blobFilePath(sha256 string) string {
	return filepath.Join(api.Config.PackagesDir, sha256+".deb")
}
