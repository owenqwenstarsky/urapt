// Package aptrepo implements the APT repository endpoint served under /apt/:repo.
// It generates Release/InRelease/Packages indices on demand from the database
// (cached in memory) and streams .deb files from the content-addressed store.
// Public repositories allow anonymous reads; private repositories require HTTP
// Basic auth where the password is an API token.
package aptrepo

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"urapt/server/auth"
	"urapt/server/cache"
	"urapt/server/middleware"
	"urapt/server/store"
	"urapt/shared/apt"
	"urapt/shared/gpg"
	"urapt/shared/httputil"
	"urapt/shared/models"
)

// APTRepo is the APT endpoint handler.
type APTRepo struct {
	Store  *store.Store
	Auth   *auth.Service
	Signer *gpg.Key
	Cache  *cache.IndexCache
}

// New returns the APT endpoint http.Handler (to be mounted at /apt).
func New(st *store.Store, authSvc *auth.Service, signer *gpg.Key, c *cache.IndexCache) http.Handler {
	a := &APTRepo{Store: st, Auth: authSvc, Signer: signer, Cache: c}

	r := chi.NewRouter()
	r.Use(middleware.Recover)
	r.Use(middleware.Log)

	r.Get("/{repo}/dists/{suite}/InRelease", a.InRelease)
	r.Get("/{repo}/dists/{suite}/Release", a.Release)
	r.Get("/{repo}/dists/{suite}/Release.gpg", a.ReleaseGpg)
	r.Get("/{repo}/dists/{suite}/{component}/{binary}/Packages", a.Packages)
	r.Get("/{repo}/dists/{suite}/{component}/{binary}/Packages.gz", a.PackagesGz)
	r.Get("/{repo}/dists/{suite}/{component}/{binary}/Packages.xz", a.PackagesXz)
	r.Get("/{repo}/pool/{component}/{letter}/{src}/{filename}", a.Pool)

	return r
}

// authorize loads the repository and enforces read access. For public repos
// anonymous access is allowed; for private repos HTTP Basic (password = token)
// is required. Returns the repo and true on success; on failure an error has
// been written.
func (a *APTRepo) authorize(w http.ResponseWriter, r *http.Request) (*models.Repository, bool) {
	repo, err := a.Store.GetRepositoryByName(r.Context(), r.PathValue("repo"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "repository not found")
			return nil, false
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to read repository")
		return nil, false
	}

	if repo.Visibility == models.VisibilityPublic {
		return repo, true
	}

	// Private repository: require HTTP Basic with token as password.
	id, err := a.Auth.ResolveBasic(r.Context(), r.Header)
	if err != nil {
		httputil.ChallengeBasic(w, "urapt "+repo.Name)
		return nil, false
	}
	ok, err := a.Auth.CanRead(r.Context(), id.User, repo)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "permission check failed")
		return nil, false
	}
	if !ok {
		httputil.ChallengeBasic(w, "urapt "+repo.Name)
		return nil, false
	}
	return repo, true
}

// loadSuite returns the generated indices for (repo, suite), building and
// caching them on first access or after invalidation.
func (a *APTRepo) loadSuite(r *http.Request, repo *models.Repository, suiteName string) (*apt.Indices, error) {
	if idx := a.Cache.Get(repo.ID, suiteName); idx != nil {
		return idx, nil
	}

	dist, err := a.Store.GetDistributionByName(r.Context(), repo.ID, suiteName)
	if err != nil {
		return nil, errMiss{err}
	}
	comps, err := a.Store.ListComponents(r.Context(), dist.ID)
	if err != nil {
		return nil, err
	}
	arches, err := a.Store.ListArchitectures(r.Context(), dist.ID)
	if err != nil {
		return nil, err
	}
	suitePkgs, err := a.Store.ListSuitePackages(r.Context(), repo.ID, dist.ID)
	if err != nil {
		return nil, err
	}

	componentNames := make([]string, 0, len(comps))
	for _, c := range comps {
		componentNames = append(componentNames, c.Name)
	}
	archNames := make([]string, 0, len(arches))
	for _, a2 := range arches {
		archNames = append(archNames, a2.Name)
	}
	rows := make([]apt.PackageRow, 0, len(suitePkgs))
	for _, sp := range suitePkgs {
		rows = append(rows, apt.PackageRow{
			Component:      sp.ComponentName,
			Name:           sp.Name,
			Version:        sp.Version,
			Architecture:   sp.Architecture,
			PoolPath:       sp.PoolPath,
			Size:           sp.Size,
			MD5sum:         sp.MD5sum,
			SHA1:           sp.SHA1,
			SHA256:         sp.SHA256,
			DescriptionMD5: sp.DescriptionMD5,
			RawControl:     sp.RawControl,
		})
	}

	suite := &apt.Suite{
		Origin:        "urapt " + repo.Name,
		Label:         "urapt " + repo.Name,
		Suite:         dist.Name,
		Description:   repo.Description,
		Components:    componentNames,
		Architectures: archNames,
		Packages:      rows,
	}
	idx, err := apt.Generate(suite, a.Signer)
	if err != nil {
		return nil, err
	}
	a.Cache.Put(repo.ID, suiteName, idx)
	return idx, nil
}

// errMiss wraps a not-found error from a sub-load.
type errMiss struct{ err error }

func (e errMiss) Error() string { return e.err.Error() }
func (e errMiss) Unwrap() error { return e.err }

// indicesOrWrite loads indices and writes a 404/500 on failure.
func (a *APTRepo) indicesOrWrite(w http.ResponseWriter, r *http.Request, repo *models.Repository, suite string) *apt.Indices {
	idx, err := a.loadSuite(r, repo, suite)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "suite not found")
			return nil
		}
		slog.Error("apt generate failed", "err", err, "repo", repo.Name, "suite", suite)
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to generate indices")
		return nil
	}
	return idx
}

// Release serves the unsigned Release file.
func (a *APTRepo) Release(w http.ResponseWriter, r *http.Request) {
	repo, ok := a.authorize(w, r)
	if !ok {
		return
	}
	idx := a.indicesOrWrite(w, r, repo, r.PathValue("suite"))
	if idx == nil {
		return
	}
	writeBytes(w, "text/plain; charset=utf-8", idx.Release)
}

// InRelease serves the clearsigned InRelease file.
func (a *APTRepo) InRelease(w http.ResponseWriter, r *http.Request) {
	repo, ok := a.authorize(w, r)
	if !ok {
		return
	}
	idx := a.indicesOrWrite(w, r, repo, r.PathValue("suite"))
	if idx == nil {
		return
	}
	if len(idx.InRelease) == 0 {
		httputil.WriteError(w, http.StatusServiceUnavailable, httputil.CodeInternal, "indices not signed")
		return
	}
	writeBytes(w, "text/plain; charset=utf-8", idx.InRelease)
}

// ReleaseGpg serves the detached signature Release.gpg.
func (a *APTRepo) ReleaseGpg(w http.ResponseWriter, r *http.Request) {
	repo, ok := a.authorize(w, r)
	if !ok {
		return
	}
	idx := a.indicesOrWrite(w, r, repo, r.PathValue("suite"))
	if idx == nil {
		return
	}
	if len(idx.ReleaseGpg) == 0 {
		httputil.WriteError(w, http.StatusServiceUnavailable, httputil.CodeInternal, "indices not signed")
		return
	}
	writeBytes(w, "application/pgp-signature", idx.ReleaseGpg)
}

// Packages serves the Packages index for a (component, arch).
func (a *APTRepo) Packages(w http.ResponseWriter, r *http.Request) {
	a.servePackages(w, r, "")
}
func (a *APTRepo) PackagesGz(w http.ResponseWriter, r *http.Request) {
	a.servePackages(w, r, "gz")
}
func (a *APTRepo) PackagesXz(w http.ResponseWriter, r *http.Request) {
	a.servePackages(w, r, "xz")
}

func (a *APTRepo) servePackages(w http.ResponseWriter, r *http.Request, encoding string) {
	repo, ok := a.authorize(w, r)
	if !ok {
		return
	}
	idx := a.indicesOrWrite(w, r, repo, r.PathValue("suite"))
	if idx == nil {
		return
	}
	binary := r.PathValue("binary")
	if !strings.HasPrefix(binary, "binary-") {
		httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "not found")
		return
	}
	arch := strings.TrimPrefix(binary, "binary-")
	relPath := r.PathValue("component") + "/binary-" + arch + "/Packages"
	var data []byte
	var contentType string
	switch encoding {
	case "":
		data = idx.Packages[relPath]
		contentType = "text/plain; charset=utf-8"
	case "gz":
		data = idx.PackagesGz[relPath+".gz"]
		contentType = "application/gzip"
	case "xz":
		data = idx.PackagesXz[relPath+".xz"]
		contentType = "application/x-xz"
	}
	if data == nil {
		httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "index not found")
		return
	}
	writeBytes(w, contentType, data)
}

// Pool streams a .deb file from the content-addressed store.
func (a *APTRepo) Pool(w http.ResponseWriter, r *http.Request) {
	repo, ok := a.authorize(w, r)
	if !ok {
		return
	}
	poolPath := strings.Join([]string{
		"pool", r.PathValue("component"), r.PathValue("letter"), r.PathValue("src"), r.PathValue("filename"),
	}, "/")
	pkg, err := a.Store.GetPackageByPoolPath(r.Context(), repo.ID, poolPath)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "package not found")
		return
	}
	http.ServeFile(w, r, pkg.Filename)
}

// writeBytes writes raw bytes with a content type and 200 status.
func writeBytes(w http.ResponseWriter, contentType string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
