package restapi

import (
	"errors"
	"net/http"

	"urapt/server/middleware"
	"urapt/server/store"
	apitypes "urapt/shared/api"
	"urapt/shared/httputil"
	"urapt/shared/models"
)

// ListUsers returns all users (admin only).
func (api *API) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := api.Store.ListUsers(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to list users")
		return
	}
	if users == nil {
		users = []*models.User{}
	}
	httputil.WriteJSON(w, http.StatusOK, apitypes.ListResponse[*models.User]{Items: users, Page: 1, PerPage: 100, Total: len(users)})
}

// GetUser returns a single user by id (admin only).
func (api *API) GetUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, err := api.Store.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "user not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to read user")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, user)
}

// UpdateUser mutates a user (currently only is_admin) (admin only).
func (api *API) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req apitypes.UpdateUserRequest
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		bad(w, "invalid JSON body")
		return
	}
	target, err := api.Store.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "user not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to read user")
		return
	}
	caller := middleware.IdentityFromContext(r.Context())
	if req.IsAdmin != nil {
		if *req.IsAdmin && !caller.User.IsAdmin {
			httputil.WriteError(w, http.StatusForbidden, httputil.CodeForbidden, "cannot grant admin")
			return
		}
		if target.ID == caller.User.ID && !*req.IsAdmin {
			httputil.WriteError(w, http.StatusBadRequest, httputil.CodeBadRequest, "cannot revoke your own admin")
			return
		}
	}
	if err := api.Store.UpdateUser(r.Context(), id, req.IsAdmin); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to update user")
		return
	}
	updated, _ := api.Store.GetUserByID(r.Context(), id)
	httputil.WriteJSON(w, http.StatusOK, updated)
}

// DeleteUser removes a user (admin only). Self-deletion is blocked.
func (api *API) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	caller := middleware.IdentityFromContext(r.Context())
	if id == caller.User.ID {
		httputil.WriteError(w, http.StatusBadRequest, httputil.CodeBadRequest, "cannot delete your own account")
		return
	}
	if err := api.Store.DeleteUser(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to delete user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
