// Package auth implements identity resolution (bearer/basic) and the
// repository-scoped permission checks used by the REST API and the APT
// endpoint.
package auth

import (
	"context"
	"errors"
	"net/http"

	"urapt/server/store"
	"urapt/shared/crypto"
	"urapt/shared/httputil"
	"urapt/shared/models"
)

// Identity is the resolved caller: a user and (for REST) the token used.
type Identity struct {
	User    *models.User
	TokenID string
}

// Service resolves identities and answers permission questions.
type Service struct {
	store *store.Store
}

// NewService constructs an auth Service.
func NewService(s *store.Store) *Service {
	return &Service{store: s}
}

// ErrUnauthenticated is returned when no valid identity can be established.
var ErrUnauthenticated = errors.New("unauthenticated")

// ResolveBearer resolves an "Authorization: Bearer <token>" header to an
// identity. Returns ErrUnauthenticated if absent or invalid.
func (s *Service) ResolveBearer(ctx context.Context, h http.Header) (*Identity, error) {
	token, ok := httputil.ParseBearer(h)
	if !ok {
		return nil, ErrUnauthenticated
	}
	return s.resolveToken(ctx, token)
}

// ResolveBasic resolves an "Authorization: Basic" header where the password is
// an API token. Returns ErrUnauthenticated if absent/invalid.
func (s *Service) ResolveBasic(ctx context.Context, h http.Header) (*Identity, error) {
	_, password, ok := httputil.ParseBasic(h)
	if !ok {
		return nil, ErrUnauthenticated
	}
	return s.resolveToken(ctx, password)
}

func (s *Service) resolveToken(ctx context.Context, token string) (*Identity, error) {
	if token == "" {
		return nil, ErrUnauthenticated
	}
	hash := crypto.HashToken(token)
	t, err := s.store.GetTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrUnauthenticated
		}
		return nil, err
	}
	user, err := s.store.GetUserByID(ctx, t.UserID)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	_ = s.store.TouchToken(ctx, t.ID)
	return &Identity{User: user, TokenID: t.ID}, nil
}

// CanRead reports whether user may read (download/list within) repo.
func (s *Service) CanRead(ctx context.Context, user *models.User, repo *models.Repository) (bool, error) {
	if user == nil {
		return repo.Visibility == models.VisibilityPublic, nil
	}
	if user.IsAdmin || repo.OwnerUserID == user.ID {
		return true, nil
	}
	access, ok, err := s.store.GetMemberAccess(ctx, repo.ID, user.ID)
	if err != nil {
		return false, err
	}
	if ok {
		switch access {
		case models.AccessRead, models.AccessReadWrite, models.AccessAdmin:
			return true, nil
		}
	}
	return repo.Visibility == models.VisibilityPublic, nil
}

// CanWrite reports whether user may push packages to repo.
func (s *Service) CanWrite(ctx context.Context, user *models.User, repo *models.Repository) (bool, error) {
	if user == nil {
		return false, nil
	}
	if user.IsAdmin || repo.OwnerUserID == user.ID {
		return true, nil
	}
	access, ok, err := s.store.GetMemberAccess(ctx, repo.ID, user.ID)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	switch access {
	case models.AccessWrite, models.AccessReadWrite, models.AccessAdmin:
		return true, nil
	}
	return false, nil
}

// CanManage reports whether user may manage repo settings and members.
func (s *Service) CanManage(ctx context.Context, user *models.User, repo *models.Repository) (bool, error) {
	if user == nil {
		return false, nil
	}
	if user.IsAdmin || repo.OwnerUserID == user.ID {
		return true, nil
	}
	access, ok, err := s.store.GetMemberAccess(ctx, repo.ID, user.ID)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return access == models.AccessAdmin, nil
}
