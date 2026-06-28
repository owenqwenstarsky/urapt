// Package httputil provides small helpers for JSON I/O, uniform error
// rendering, and parsing of Authorization headers shared across the REST API
// and the APT endpoint.
package httputil

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// APIError is the uniform JSON error body: {"error": {...}}.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// ErrorEnvelope wraps an APIError.
type ErrorEnvelope struct {
	Error APIError `json:"error"`
}

// WriteJSON writes v as JSON with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// ReadJSON decodes r.Body into v. It limits the body to maxBytes.
func ReadJSON(r *http.Request, v any, maxBytes int64) error {
	if maxBytes > 0 {
		r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}

// WriteError renders a uniform error response.
func WriteError(w http.ResponseWriter, status int, code, message string, details ...any) {
	e := APIError{Code: code, Message: message}
	if len(details) > 0 {
		e.Details = details[0]
	}
	WriteJSON(w, status, ErrorEnvelope{Error: e})
}

// WriteErrorf is WriteError with printf-style message formatting.
func WriteErrorf(w http.ResponseWriter, status int, code, format string, args ...any) {
	WriteError(w, status, code, fmt.Sprintf(format, args...))
}

// Common error code constants.
const (
	CodeBadRequest      = "bad_request"
	CodeUnauthorized    = "unauthorized"
	CodeForbidden       = "forbidden"
	CodeNotFound        = "not_found"
	CodeConflict        = "conflict"
	CodePayloadTooLarge = "payload_too_large"
	CodeInternal        = "internal"
)

// ParseBearer extracts the token from an "Authorization: Bearer <token>"
// header. ok is false if the header is absent or malformed.
func ParseBearer(h http.Header) (token string, ok bool) {
	v := h.Get("Authorization")
	if v == "" {
		return "", false
	}
	parts := strings.SplitN(v, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	t := strings.TrimSpace(parts[1])
	if t == "" {
		return "", false
	}
	return t, true
}

// ParseBasic extracts username/password from an "Authorization: Basic"
// header. ok is false if absent or malformed.
func ParseBasic(h http.Header) (username, password string, ok bool) {
	v := h.Get("Authorization")
	if v == "" {
		return "", "", false
	}
	parts := strings.SplitN(v, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Basic") {
		return "", "", false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(parts[1]))
	if err != nil {
		return "", "", false
	}
	idx := strings.IndexByte(string(raw), ':')
	if idx < 0 {
		return "", "", false
	}
	return string(raw[:idx]), string(raw[idx+1:]), true
}

// ChallengeBasic writes a 401 with a WWW-Authenticate Basic challenge.
func ChallengeBasic(w http.ResponseWriter, realm string) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm=%q, charset="UTF-8"`, realm))
	WriteError(w, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
}
