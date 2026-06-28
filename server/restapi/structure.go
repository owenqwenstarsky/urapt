package restapi

import (
	"errors"
	"net/http"

	"urapt/server/store"
	apitypes "urapt/shared/api"
	"urapt/shared/httputil"
	"urapt/shared/models"
)

// validateName checks a distribution/component/architecture name.
func validateName(s string) bool { return nameRE.MatchString(s) }

// loadDistro fetches the {repo}/{dist} distribution, rendering errors.
func (api *API) loadDistro(w http.ResponseWriter, r *http.Request) (*models.Repository, *models.Distribution) {
	repo := api.requireRead(w, r)
	if repo == nil {
		return nil, nil
	}
	dist, err := api.Store.GetDistributionByName(r.Context(), repo.ID, r.PathValue("dist"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "distribution not found")
			return repo, nil
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to read distribution")
		return repo, nil
	}
	return repo, dist
}

// requireDistroWrite loads repo (write) + distribution.
func (api *API) requireDistroWrite(w http.ResponseWriter, r *http.Request) (*models.Repository, *models.Distribution) {
	repo := api.requireWrite(w, r)
	if repo == nil {
		return nil, nil
	}
	dist, err := api.Store.GetDistributionByName(r.Context(), repo.ID, r.PathValue("dist"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "distribution not found")
			return repo, nil
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to read distribution")
		return repo, nil
	}
	return repo, dist
}

// --- distributions ---

// ListDistributions returns the distributions in a repository.
func (api *API) ListDistributions(w http.ResponseWriter, r *http.Request) {
	repo := api.requireRead(w, r)
	if repo == nil {
		return
	}
	dists, err := api.Store.ListDistributions(r.Context(), repo.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to list distributions")
		return
	}
	if dists == nil {
		dists = []*models.Distribution{}
	}
	httputil.WriteJSON(w, http.StatusOK, dists)
}

// CreateDistribution adds a distribution to a repository.
func (api *API) CreateDistribution(w http.ResponseWriter, r *http.Request) {
	repo := api.requireWrite(w, r)
	if repo == nil {
		return
	}
	var req apitypes.CreateNamedRequest
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		bad(w, "invalid JSON body")
		return
	}
	if !validateName(req.Name) {
		bad(w, "name must be 1-64 chars of [a-z0-9][a-z0-9+.-]")
		return
	}
	if _, err := api.Store.GetDistributionByName(r.Context(), repo.ID, req.Name); err == nil {
		httputil.WriteError(w, http.StatusConflict, httputil.CodeConflict, "distribution already exists")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to check distribution")
		return
	}
	dist, err := api.Store.CreateDistribution(r.Context(), repo.ID, req.Name)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to create distribution")
		return
	}
	api.Cache.Invalidate(repo.ID, dist.Name)
	httputil.WriteJSON(w, http.StatusCreated, dist)
}

// DeleteDistribution removes a distribution (cascades to packages).
func (api *API) DeleteDistribution(w http.ResponseWriter, r *http.Request) {
	repo, dist := api.requireDistroWrite(w, r)
	if dist == nil {
		return
	}
	if err := api.Store.DeleteDistribution(r.Context(), repo.ID, dist.Name); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "distribution not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to delete distribution")
		return
	}
	// Note: cascaded package rows are gone; their blob ref counts are now
	// stale. Best-effort cleanup of orphan blobs is handled by package delete
	// in normal operation; bulk distribution deletion leaves blobs for now.
	api.Cache.Invalidate(repo.ID, dist.Name)
	w.WriteHeader(http.StatusNoContent)
}

// --- components ---

// ListComponents returns the components in a distribution.
func (api *API) ListComponents(w http.ResponseWriter, r *http.Request) {
	_, dist := api.loadDistro(w, r)
	if dist == nil {
		return
	}
	comps, err := api.Store.ListComponents(r.Context(), dist.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to list components")
		return
	}
	if comps == nil {
		comps = []*models.Component{}
	}
	httputil.WriteJSON(w, http.StatusOK, comps)
}

// CreateComponent adds a component to a distribution.
func (api *API) CreateComponent(w http.ResponseWriter, r *http.Request) {
	repo, dist := api.requireDistroWrite(w, r)
	if dist == nil {
		return
	}
	var req apitypes.CreateNamedRequest
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		bad(w, "invalid JSON body")
		return
	}
	if !validateName(req.Name) {
		bad(w, "name must be 1-64 chars of [a-z0-9][a-z0-9+.-]")
		return
	}
	if _, err := api.Store.GetComponentByName(r.Context(), dist.ID, req.Name); err == nil {
		httputil.WriteError(w, http.StatusConflict, httputil.CodeConflict, "component already exists")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to check component")
		return
	}
	comp, err := api.Store.CreateComponent(r.Context(), dist.ID, req.Name)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to create component")
		return
	}
	api.Cache.Invalidate(repo.ID, dist.Name)
	httputil.WriteJSON(w, http.StatusCreated, comp)
}

// DeleteComponent removes a component (blocked if packages reference it).
func (api *API) DeleteComponent(w http.ResponseWriter, r *http.Request) {
	repo, dist := api.requireDistroWrite(w, r)
	if dist == nil {
		return
	}
	comp, err := api.Store.GetComponentByName(r.Context(), dist.ID, r.PathValue("comp"))
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "component not found")
		return
	}
	if err := api.Store.DeleteComponent(r.Context(), dist.ID, comp.Name); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to delete component (packages may still reference it)")
		return
	}
	api.Cache.Invalidate(repo.ID, dist.Name)
	w.WriteHeader(http.StatusNoContent)
}

// --- architectures ---

// ListArchitectures returns the architectures in a distribution.
func (api *API) ListArchitectures(w http.ResponseWriter, r *http.Request) {
	_, dist := api.loadDistro(w, r)
	if dist == nil {
		return
	}
	arches, err := api.Store.ListArchitectures(r.Context(), dist.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to list architectures")
		return
	}
	if arches == nil {
		arches = []*models.Architecture{}
	}
	httputil.WriteJSON(w, http.StatusOK, arches)
}

// CreateArchitecture adds an architecture to a distribution.
func (api *API) CreateArchitecture(w http.ResponseWriter, r *http.Request) {
	repo, dist := api.requireDistroWrite(w, r)
	if dist == nil {
		return
	}
	var req apitypes.CreateNamedRequest
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		bad(w, "invalid JSON body")
		return
	}
	if !validateName(req.Name) {
		bad(w, "name must be 1-64 chars of [a-z0-9][a-z0-9+.-]")
		return
	}
	if req.Name == "all" {
		bad(w, "architecture 'all' is implicit and cannot be added")
		return
	}
	arch, err := api.Store.CreateArchitecture(r.Context(), dist.ID, req.Name)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to create architecture (already exists?)")
		return
	}
	api.Cache.Invalidate(repo.ID, dist.Name)
	httputil.WriteJSON(w, http.StatusCreated, arch)
}

// DeleteArchitecture removes an architecture from a distribution.
func (api *API) DeleteArchitecture(w http.ResponseWriter, r *http.Request) {
	repo, dist := api.requireDistroWrite(w, r)
	if dist == nil {
		return
	}
	if err := api.Store.DeleteArchitecture(r.Context(), dist.ID, r.PathValue("arch")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "architecture not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to delete architecture")
		return
	}
	api.Cache.Invalidate(repo.ID, dist.Name)
	w.WriteHeader(http.StatusNoContent)
}
