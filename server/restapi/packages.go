package restapi

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"urapt/server/middleware"
	"urapt/server/store"
	"urapt/shared/apt"
	"urapt/shared/deb"
	"urapt/shared/httputil"
	"urapt/shared/models"
)

// ListPackages lists packages in a (repo, distribution) with optional filters.
func (api *API) ListPackages(w http.ResponseWriter, r *http.Request) {
	repo, dist := api.loadDistro(w, r)
	if dist == nil {
		return
	}
	f := store.PackageFilters{
		ComponentID: r.URL.Query().Get("component"),
		Arch:        r.URL.Query().Get("arch"),
		Name:        r.URL.Query().Get("name"),
		Query:       r.URL.Query().Get("q"),
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	pkgs, total, err := api.Store.ListPackages(r.Context(), repo.ID, dist.ID, f, page, perPage)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to list packages")
		return
	}
	if pkgs == nil {
		pkgs = []*models.Package{}
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"items": pkgs, "page": pageOr(page), "per_page": perPageOr(perPage), "total": total,
	})
}

// PushPackage receives a multipart .deb upload, validates it, stores the blob,
// and records the package.
func (api *API) PushPackage(w http.ResponseWriter, r *http.Request) {
	repo, dist := api.requireDistroWrite(w, r)
	if dist == nil {
		return
	}
	id := middleware.IdentityFromContext(r.Context())

	// Stream the multipart upload to a temp file in the packages dir.
	tempPath, origName, componentName, err := api.receiveUpload(r)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, httputil.CodeBadRequest, err.Error())
		return
	}
	cleanup := func() { _ = os.Remove(tempPath) }
	defer func() { _ = os.Remove(tempPath) }()

	if componentName == "" {
		bad(w, "component field is required")
		return
	}
	component, err := api.Store.GetComponentByName(r.Context(), dist.ID, componentName)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, httputil.CodeBadRequest, "component not found in distribution")
		return
	}

	// Parse and hash the uploaded .deb.
	inspected, err := deb.Inspect(tempPath)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, httputil.CodeBadRequest, "invalid .deb: "+err.Error())
		return
	}
	ctrl := inspected.Control
	if ctrl.Get("Package") == "" || ctrl.Get("Version") == "" || ctrl.Get("Architecture") == "" {
		httputil.WriteError(w, http.StatusBadRequest, httputil.CodeBadRequest, "control missing Package/Version/Architecture")
		return
	}

	// Validate architecture is configured (or "all").
	pkgArch := ctrl.Get("Architecture")
	if pkgArch != "all" {
		ok, err := api.Store.HasArchitecture(r.Context(), dist.ID, pkgArch)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to check architecture")
			return
		}
		if !ok {
			httputil.WriteError(w, http.StatusBadRequest, httputil.CodeBadRequest, "architecture "+pkgArch+" not configured for distribution")
			return
		}
	}

	// Dedup on (repo, distro, component, name, version, arch).
	existing, err := api.Store.GetPackageByPoolPath(r.Context(), repo.ID,
		apt.PoolPath(component.Name, ctrl.Get("Source"), ctrl.Get("Package"), origName))
	_ = existing
	if err == nil {
		httputil.WriteError(w, http.StatusConflict, httputil.CodeConflict,
			"package "+ctrl.Get("Package")+"_"+ctrl.Get("Version")+"_"+pkgArch+" already exists")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to check duplicate")
		return
	}

	// Find-or-create the content-addressed blob.
	blobFileName := api.blobFilePath(inspected.SHA256)
	created, err := api.Store.CreateBlob(r.Context(), inspected.SHA256, blobFileName, inspected.Size)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to record blob")
		return
	}
	if created {
		// New blob: move the temp file into place.
		if err := os.Rename(tempPath, blobFileName); err != nil {
			_ = api.Store.DeleteBlob(r.Context(), inspected.SHA256)
			httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to store package file")
			return
		}
	} else {
		// Existing blob: increment ref count and discard the temp upload.
		if _, err := api.Store.IncBlobRef(r.Context(), inspected.SHA256); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to increment blob ref")
			return
		}
		cleanup()
	}

	pool := apt.PoolPath(component.Name, ctrl.Get("Source"), ctrl.Get("Package"), origName)
	pkg := &models.Package{
		RepositoryID:     repo.ID,
		DistributionID:   dist.ID,
		ComponentID:      component.ID,
		Name:             ctrl.Get("Package"),
		Version:          ctrl.Get("Version"),
		Architecture:     pkgArch,
		Source:           ctrl.Get("Source"),
		Maintainer:       ctrl.Get("Maintainer"),
		Priority:         ctrl.Get("Priority"),
		Section:          ctrl.Get("Section"),
		Origin:           ctrl.Get("Origin"),
		Homepage:         ctrl.Get("Homepage"),
		Description:      ctrl.Get("Description"),
		DescriptionMD5:   ctrl.DescriptionMD5(),
		Depends:          ctrl.Get("Depends"),
		PreDepends:       ctrl.Get("Pre-Depends"),
		Recommends:       ctrl.Get("Recommends"),
		Suggests:         ctrl.Get("Suggests"),
		Conflicts:        ctrl.Get("Conflicts"),
		Breaks:           ctrl.Get("Breaks"),
		Provides:         ctrl.Get("Provides"),
		Replaces:         ctrl.Get("Replaces"),
		Enhances:         ctrl.Get("Enhances"),
		InstalledSize:    parseInt64(ctrl.Get("Installed-Size")),
		Essential:        ctrl.Get("Essential"),
		BuiltUsing:       ctrl.Get("Built-Using"),
		Tag:              ctrl.Get("Tag"),
		RawControl:       ctrl.Raw,
		Filename:         blobFileName,
		PoolPath:         pool,
		Size:             inspected.Size,
		MD5sum:           inspected.MD5sum,
		SHA1:             inspected.SHA1,
		SHA256:           inspected.SHA256,
		UploadedByUserID: id.User.ID,
	}
	if err := api.Store.CreatePackage(r.Context(), pkg); err != nil {
		// Roll back the blob ref we added.
		if rc, _ := api.Store.DecBlobRef(r.Context(), inspected.SHA256); rc == 0 {
			_ = api.Store.DeleteBlob(r.Context(), inspected.SHA256)
			_ = os.Remove(blobFileName)
		}
		httputil.WriteError(w, http.StatusConflict, httputil.CodeConflict, "package already exists or invalid")
		return
	}
	api.Cache.Invalidate(repo.ID, dist.Name)
	_ = api.Store.RecordAudit(r.Context(), &id.User.ID, &repo.ID, "package.push", pkg.Name+"_"+pkg.Version, pkg.Architecture)
	httputil.WriteJSON(w, http.StatusCreated, pkg)
}

// GetPackage returns a single package by id.
func (api *API) GetPackage(w http.ResponseWriter, r *http.Request) {
	repo := api.requireRead(w, r)
	if repo == nil {
		return
	}
	pkg, err := api.Store.GetPackageByID(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "package not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to read package")
		return
	}
	if pkg.RepositoryID != repo.ID {
		httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "package not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, pkg)
}

// GetPackageFile streams a package's .deb file (used by the CLI pull command).
func (api *API) GetPackageFile(w http.ResponseWriter, r *http.Request) {
	repo := api.requireRead(w, r)
	if repo == nil {
		return
	}
	pkg, err := api.Store.GetPackageByID(r.Context(), r.PathValue("id"))
	if err != nil || pkg.RepositoryID != repo.ID {
		httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "package not found")
		return
	}
	path := api.blobFilePath(pkg.SHA256)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(pkg.PoolPath)+`"`)
	http.ServeFile(w, r, path)
}

// DeletePackage removes a package and decrements its blob reference.
func (api *API) DeletePackage(w http.ResponseWriter, r *http.Request) {
	repo := api.requireWrite(w, r)
	if repo == nil {
		return
	}
	pkg, err := api.Store.GetPackageByID(r.Context(), r.PathValue("id"))
	if err != nil || pkg.RepositoryID != repo.ID {
		httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "package not found")
		return
	}
	if err := api.Store.DeletePackage(r.Context(), pkg.ID); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to delete package")
		return
	}
	if rc, _ := api.Store.DecBlobRef(r.Context(), pkg.SHA256); rc == 0 {
		_ = api.Store.DeleteBlob(r.Context(), pkg.SHA256)
		_ = os.Remove(api.blobFilePath(pkg.SHA256))
	}
	api.Cache.InvalidateRepo(repo.ID)
	id := middleware.IdentityFromContext(r.Context())
	_ = api.Store.RecordAudit(r.Context(), &id.User.ID, &repo.ID, "package.delete", pkg.Name+"_"+pkg.Version, pkg.Architecture)
	w.WriteHeader(http.StatusNoContent)
}

// receiveUpload streams a multipart upload (field "file" + "component") to a
// temp file in the packages directory, enforcing the size limit. Returns the
// temp path, original filename, component name, and any error.
func (api *API) receiveUpload(r *http.Request) (tempPath, origName, component string, err error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return "", "", "", errors.New("expected multipart/form-data")
	}
	maxSize := api.Config.MaxPackageSize

	f, err := os.CreateTemp(api.Config.PackagesDir, ".upload-*")
	if err != nil {
		return "", "", "", errors.New("failed to create temp file")
	}
	tempPath = f.Name()
	defer func() { _ = f.Close() }()

	gotFile := false
	gotComponent := false
	var written int64
	for {
		part, perr := reader.NextPart()
		if perr == io.EOF {
			break
		}
		if perr != nil {
			return tempPath, "", "", perr
		}
		switch part.FormName() {
		case "component":
			data, derr := io.ReadAll(io.LimitReader(part, 256))
			if derr != nil {
				return tempPath, "", "", derr
			}
			component = strings.TrimSpace(string(data))
			gotComponent = true
		case "file":
			origName = part.FileName()
			if origName == "" {
				return tempPath, "", "", errors.New("file field has no filename")
			}
			n, werr := io.Copy(f, io.LimitReader(part, maxSize+1))
			if werr != nil {
				return tempPath, origName, "", werr
			}
			written = n
			gotFile = true
		default:
			// ignore unknown fields
		}
	}
	if !gotFile {
		return tempPath, "", "", errors.New("missing 'file' field")
	}
	if !gotComponent {
		return tempPath, origName, "", errors.New("missing 'component' field")
	}
	if written > maxSize {
		return tempPath, origName, "", errors.New("package exceeds max size")
	}
	_ = gotComponent
	return tempPath, origName, component, nil
}

// parseInt64 parses a base-10 int64, returning 0 on error.
func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// pageOr defaults page to 1.
func pageOr(p int) int {
	if p < 1 {
		return 1
	}
	return p
}

// perPageOr defaults per-page to 25.
func perPageOr(p int) int {
	if p < 1 {
		return 25
	}
	if p > 100 {
		return 100
	}
	return p
}
