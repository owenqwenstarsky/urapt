// Package api defines the request and response DTOs that form the urapt REST
// API contract. The server's restapi package produces these and the CLI's
// apiclient package consumes them; keeping them in one place prevents drift.
package api

import "urapt/shared/models"

// --- server / setup ---

// ServerInfo is the response from GET /server/info.
type ServerInfo struct {
	Version               string `json:"version"`
	NeedsSetup            bool   `json:"needs_setup"`
	DefaultKeyFingerprint string `json:"default_key_fingerprint"`
	OpenRegistration      bool   `json:"open_registration"`
}

// --- auth ---

// RegisterRequest is the body for POST /auth/register.
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=32,username"`
	Password string `json:"password" validate:"required,min=8,max=256"`
}

// LoginRequest is the body for POST /auth/login.
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// AuthResponse is returned by register and login.
type AuthResponse struct {
	User  *models.User `json:"user"`
	Token string       `json:"token"`
}

// CreateTokenRequest is the body for POST /me/tokens.
type CreateTokenRequest struct {
	Name string `json:"name" validate:"required,min=1,max=64"`
}

// ListResponse wraps a page of items.
type ListResponse[T any] struct {
	Items   []T `json:"items"`
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

// --- users (admin) ---

// UpdateUserRequest is the body for PATCH /users/:id.
type UpdateUserRequest struct {
	IsAdmin *bool `json:"is_admin,omitempty"`
}

// --- repositories ---

// CreateRepoRequest is the body for POST /repositories.
type CreateRepoRequest struct {
	Name        string `json:"name"`
	Visibility  string `json:"visibility"`
	Description string `json:"description,omitempty"`
}

// UpdateRepoRequest is the body for PATCH /repositories/:repo.
type UpdateRepoRequest struct {
	Name        *string `json:"name,omitempty"`
	Visibility  *string `json:"visibility,omitempty"`
	Description *string `json:"description,omitempty"`
}

// AddMemberRequest is the body for POST /repositories/:repo/members.
type AddMemberRequest struct {
	Username string `json:"username"`
	Access   string `json:"access"`
}

// UpdateMemberRequest is the body for PATCH /repositories/:repo/members/:username.
type UpdateMemberRequest struct {
	Access string `json:"access"`
}

// --- structure ---

// CreateNamedRequest is the body for creating a distribution/component/arch.
type CreateNamedRequest struct {
	Name string `json:"name"`
}
