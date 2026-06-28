// Package restapi implements the urapt REST API: handlers, routing, and
// request validation. It is mounted under /api/v1 on the server.
package restapi

import (
	"context"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"

	"urapt/server/auth"
	"urapt/server/cache"
	"urapt/server/middleware"
	"urapt/server/store"
	"urapt/shared/config"
	"urapt/shared/httputil"
)

// SignerProvider returns the server's current signing key. It is supplied by
// the app; the APT endpoint also uses it to sign Release files.
type SignerProvider interface {
	PublicKeyArmored() (string, error)
	Fingerprint() string
}

// API holds the dependencies shared by all REST handlers.
type API struct {
	Store  *store.Store
	Auth   *auth.Service
	Signer SignerProvider
	Config *config.Config
	Cache  *cache.IndexCache
}

// New constructs the API and returns its http.Handler (the /api/v1 router).
func New(st *store.Store, authSvc *auth.Service, signer SignerProvider, cfg *config.Config, c *cache.IndexCache) http.Handler {
	api := &API{Store: st, Auth: authSvc, Signer: signer, Config: cfg, Cache: c}

	r := chi.NewRouter()
	r.Use(middleware.Recover)
	r.Use(middleware.Log)

	r.Get("/server/info", api.ServerInfo)
	r.Get("/server/pubkey", api.ServerPubkey)

	r.Post("/auth/register", api.Register)
	r.Post("/auth/login", api.Login)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireBearer(authSvc))

		r.Post("/auth/logout", api.Logout)
		r.Get("/me", api.Me)
		r.Get("/me/tokens", api.ListTokens)
		r.Post("/me/tokens", api.CreateToken)
		r.Delete("/me/tokens/{id}", api.RevokeToken)

		// repositories (read for visible, write for permitted)
		r.Get("/repositories", api.ListRepositories)
		r.Post("/repositories", api.CreateRepository)
		r.Get("/repositories/{repo}", api.GetRepository)
		r.Patch("/repositories/{repo}", api.UpdateRepository)
		r.Delete("/repositories/{repo}", api.DeleteRepository)
		r.Get("/repositories/{repo}/members", api.ListMembers)
		r.Post("/repositories/{repo}/members", api.AddMember)
		r.Patch("/repositories/{repo}/members/{username}", api.UpdateMember)
		r.Delete("/repositories/{repo}/members/{username}", api.RemoveMember)
		r.Get("/repositories/{repo}/pubkey", api.RepoPubkey)

		// structure
		r.Get("/repositories/{repo}/distributions", api.ListDistributions)
		r.Post("/repositories/{repo}/distributions", api.CreateDistribution)
		r.Delete("/repositories/{repo}/distributions/{dist}", api.DeleteDistribution)
		r.Get("/repositories/{repo}/distributions/{dist}/components", api.ListComponents)
		r.Post("/repositories/{repo}/distributions/{dist}/components", api.CreateComponent)
		r.Delete("/repositories/{repo}/distributions/{dist}/components/{comp}", api.DeleteComponent)
		r.Get("/repositories/{repo}/distributions/{dist}/architectures", api.ListArchitectures)
		r.Post("/repositories/{repo}/distributions/{dist}/architectures", api.CreateArchitecture)
		r.Delete("/repositories/{repo}/distributions/{dist}/architectures/{arch}", api.DeleteArchitecture)

		// packages
		r.Get("/repositories/{repo}/distributions/{dist}/packages", api.ListPackages)
		r.Post("/repositories/{repo}/distributions/{dist}/packages", api.PushPackage)
		r.Get("/repositories/{repo}/packages/{id}", api.GetPackage)
		r.Get("/repositories/{repo}/packages/{id}/file", api.GetPackageFile)
		r.Delete("/repositories/{repo}/packages/{id}", api.DeletePackage)

		r.Group(func(r chi.Router) {
			r.Use(api.RequireAdmin)
			r.Get("/users", api.ListUsers)
			r.Get("/users/{id}", api.GetUser)
			r.Patch("/users/{id}", api.UpdateUser)
			r.Delete("/users/{id}", api.DeleteUser)
		})
	})

	return r
}

// RequireAdmin is middleware that requires the caller to be a server admin.
func (api *API) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := middleware.IdentityFromContext(r.Context())
		if id == nil || !id.User.IsAdmin {
			httputil.WriteError(w, http.StatusForbidden, httputil.CodeForbidden, "admin privileges required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// validation patterns.
var (
	usernameRE  = regexp.MustCompile(`^[a-z0-9_-]{3,32}$`)
	passwordRE  = regexp.MustCompile(`^.{8,256}$`)
	tokenNameRE = regexp.MustCompile(`^.{1,64}$`)
)

// validateUsername returns true if s is an acceptable username.
func validateUsername(s string) bool { return usernameRE.MatchString(s) }

// validatePassword returns true if s is an acceptable password.
func validatePassword(s string) bool { return passwordRE.MatchString(s) }

// bad renders a 400 validation error.
func bad(w http.ResponseWriter, msg string) {
	httputil.WriteError(w, http.StatusBadRequest, httputil.CodeBadRequest, msg)
}

// contextKey for request-scoped values is not needed beyond middleware; this
// var keeps context imported if future handlers need it.
var _ = context.Background
