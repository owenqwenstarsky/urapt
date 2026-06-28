package restapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"urapt/server/middleware"
	"urapt/server/store"
	apitypes "urapt/shared/api"
	"urapt/shared/crypto"
	"urapt/shared/httputil"
	"urapt/shared/models"
)

// Register creates a new account. The first account becomes the admin. When
// open_registration is false and an account already exists, registration is
// closed to non-admins.
func (api *API) Register(w http.ResponseWriter, r *http.Request) {
	var req apitypes.RegisterRequest
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		bad(w, "invalid JSON body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !validateUsername(req.Username) {
		bad(w, "username must be 3-32 chars of [a-z0-9_-]")
		return
	}
	if !validatePassword(req.Password) {
		bad(w, "password must be 8-256 chars")
		return
	}

	users, err := api.Store.ListUsers(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to read users")
		return
	}
	if len(users) > 0 && !api.Config.OpenRegistration {
		httputil.WriteError(w, http.StatusForbidden, httputil.CodeForbidden, "registration is closed")
		return
	}

	if _, err := api.Store.GetUserByUsername(r.Context(), req.Username); err == nil {
		httputil.WriteError(w, http.StatusConflict, httputil.CodeConflict, "username already taken")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to check username")
		return
	}

	hash, err := crypto.HashPassword(req.Password)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to hash password")
		return
	}
	user, _, err := api.Store.CreateUser(r.Context(), req.Username, hash)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to create user")
		return
	}

	token, err := api.issueToken(r.Context(), user.ID, "login")
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to issue token")
		return
	}
	_ = api.Store.RecordAudit(r.Context(), &user.ID, nil, "user.register", user.Username, "")
	httputil.WriteJSON(w, http.StatusCreated, apitypes.AuthResponse{User: user, Token: token})
}

// Login authenticates a user and issues a new API token.
func (api *API) Login(w http.ResponseWriter, r *http.Request) {
	var req apitypes.LoginRequest
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		bad(w, "invalid JSON body")
		return
	}
	user, err := api.Store.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, httputil.CodeUnauthorized, "invalid credentials")
		return
	}
	// Need the password hash; fetch via a dedicated method.
	hash, err := api.Store.GetUserPasswordHash(r.Context(), user.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to read user")
		return
	}
	if !crypto.VerifyPassword(hash, req.Password) {
		httputil.WriteError(w, http.StatusUnauthorized, httputil.CodeUnauthorized, "invalid credentials")
		return
	}
	token, err := api.issueToken(r.Context(), user.ID, "login")
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to issue token")
		return
	}
	_ = api.Store.RecordAudit(r.Context(), &user.ID, nil, "user.login", user.Username, "")
	httputil.WriteJSON(w, http.StatusOK, apitypes.AuthResponse{User: user, Token: token})
}

// Logout revokes the caller's current token.
func (api *API) Logout(w http.ResponseWriter, r *http.Request) {
	id := middleware.IdentityFromContext(r.Context())
	if err := api.Store.RevokeToken(r.Context(), id.User.ID, id.TokenID); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to revoke token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Me returns the caller's user record.
func (api *API) Me(w http.ResponseWriter, r *http.Request) {
	id := middleware.IdentityFromContext(r.Context())
	httputil.WriteJSON(w, http.StatusOK, id.User)
}

// ListTokens returns the caller's tokens.
func (api *API) ListTokens(w http.ResponseWriter, r *http.Request) {
	id := middleware.IdentityFromContext(r.Context())
	tokens, err := api.Store.ListTokens(r.Context(), id.User.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to list tokens")
		return
	}
	if tokens == nil {
		tokens = []*models.APIToken{}
	}
	httputil.WriteJSON(w, http.StatusOK, apitypes.ListResponse[*models.APIToken]{Items: tokens, Page: 1, PerPage: 100, Total: len(tokens)})
}

// CreateToken issues a new named token for the caller.
func (api *API) CreateToken(w http.ResponseWriter, r *http.Request) {
	var req apitypes.CreateTokenRequest
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		bad(w, "invalid JSON body")
		return
	}
	if !tokenNameRE.MatchString(req.Name) {
		bad(w, "name must be 1-64 chars")
		return
	}
	id := middleware.IdentityFromContext(r.Context())
	token, row, err := api.issueTokenRow(r.Context(), id.User.ID, req.Name)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to issue token")
		return
	}
	row.Token = token
	_ = api.Store.RecordAudit(r.Context(), &id.User.ID, nil, "token.create", req.Name, "")
	httputil.WriteJSON(w, http.StatusCreated, row)
}

// RevokeToken revokes one of the caller's tokens by id.
func (api *API) RevokeToken(w http.ResponseWriter, r *http.Request) {
	id := middleware.IdentityFromContext(r.Context())
	tokenID := r.PathValue("id")
	if err := api.Store.RevokeToken(r.Context(), id.User.ID, tokenID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, httputil.CodeNotFound, "token not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to revoke token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// issueToken generates a token, persists its hash, and returns the plaintext.
func (api *API) issueToken(ctx context.Context, userID, name string) (string, error) {
	token, hash, prefix, err := crypto.GenerateToken()
	if err != nil {
		return "", err
	}
	if _, err := api.Store.CreateToken(ctx, userID, name, prefix, hash); err != nil {
		return "", err
	}
	return token, nil
}

// issueTokenRow is like issueToken but also returns the persisted token row.
func (api *API) issueTokenRow(ctx context.Context, userID, name string) (string, *models.APIToken, error) {
	token, hash, prefix, err := crypto.GenerateToken()
	if err != nil {
		return "", nil, err
	}
	row, err := api.Store.CreateToken(ctx, userID, name, prefix, hash)
	if err != nil {
		return "", nil, err
	}
	return token, row, nil
}
