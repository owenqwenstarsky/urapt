// Package middleware provides HTTP middleware shared by the REST API and the
// APT endpoint: panic recovery, request logging, and bearer/basic identity
// injection.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"

	"urapt/server/auth"
	"urapt/shared/httputil"
)

// ctxKey is an unexported key type for context values.
type ctxKey int

const (
	keyIdentity ctxKey = iota
)

// IdentityFromContext returns the identity previously attached by RequireBearer
// or OptionalBearer, or nil.
func IdentityFromContext(ctx context.Context) *auth.Identity {
	v, _ := ctx.Value(keyIdentity).(*auth.Identity)
	return v
}

// withIdentity stores id in the context.
func withIdentity(ctx context.Context, id *auth.Identity) context.Context {
	return context.WithValue(ctx, keyIdentity, id)
}

// Recover catches panics and renders a uniform 500.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic", "err", rec, "stack", string(debug.Stack()))
				httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Log logs each request using slog.
func Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "status", rw.status)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// RequireBearer resolves a bearer token; on failure it renders 401. On success
// the identity is attached to the request context.
func RequireBearer(svc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := svc.ResolveBearer(r.Context(), r.Header)
			if err != nil {
				httputil.WriteError(w, http.StatusUnauthorized, httputil.CodeUnauthorized, "authentication required")
				return
			}
			next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), id)))
		})
	}
}

// OptionalBearer resolves a bearer token if present but never blocks; the
// identity (possibly nil) is attached to the context.
func OptionalBearer(svc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, _ := svc.ResolveBearer(r.Context(), r.Header)
			next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), id)))
		})
	}
}

// RequireBasic resolves HTTP Basic auth (password = API token); on failure it
// issues a 401 with a WWW-Authenticate challenge. Used by the APT endpoint for
// private repositories.
func RequireBasic(svc *auth.Service, realm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := svc.ResolveBasic(r.Context(), r.Header)
			if err != nil {
				httputil.ChallengeBasic(w, realm)
				return
			}
			next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), id)))
		})
	}
}
