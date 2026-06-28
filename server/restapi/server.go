package restapi

import (
	"net/http"

	apitypes "urapt/shared/api"
	"urapt/shared/httputil"
	"urapt/shared/version"
)

// ServerInfo returns build/setup metadata for the server.
func (api *API) ServerInfo(w http.ResponseWriter, r *http.Request) {
	users, err := api.Store.ListUsers(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to read users")
		return
	}
	fp := ""
	if api.Signer != nil {
		fp = api.Signer.Fingerprint()
	}
	httputil.WriteJSON(w, http.StatusOK, apitypes.ServerInfo{
		Version:               version.Version,
		NeedsSetup:            len(users) == 0,
		DefaultKeyFingerprint: fp,
		OpenRegistration:      api.Config.OpenRegistration,
	})
}

// ServerPubkey returns the ASCII-armored default signing key.
func (api *API) ServerPubkey(w http.ResponseWriter, r *http.Request) {
	if api.Signer == nil {
		httputil.WriteError(w, http.StatusServiceUnavailable, httputil.CodeInternal, "no signing key configured")
		return
	}
	pub, err := api.Signer.PublicKeyArmored()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, httputil.CodeInternal, "failed to read key")
		return
	}
	w.Header().Set("Content-Type", "application/pgp-keys")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(pub))
}
