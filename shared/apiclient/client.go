// Package apiclient is the typed HTTP client used by the urapt CLI to talk to
// the urapt-server REST API. It wraps net/http with the shared API DTOs.
package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	apitypes "urapt/shared/api"
	"urapt/shared/httputil"
	"urapt/shared/models"
)

// Client is an authenticated HTTP client for the urapt REST API.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// New constructs a client. baseURL must not have a trailing slash.
func New(baseURL, token string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Token: token, HTTP: http.DefaultClient}
}

// APIError is an error returned by the server (the error envelope).
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s (HTTP %d)", e.Message, e.Status)
}

// IsAPIError reports whether err is an *APIError and returns it.
func IsAPIError(err error) (*APIError, bool) {
	if e, ok := err.(*APIError); ok {
		return e, true
	}
	return nil, false
}

// do performs a JSON request and unmarshals the response into out (if non-nil
// and status is 2xx). On non-2xx it returns an *APIError.
func (c *Client) do(method, path string, body any, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, r)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var env struct {
			Error httputil.APIError `json:"error"`
		}
		_ = json.Unmarshal(data, &env)
		return &APIError{Status: resp.StatusCode, Code: env.Error.Code, Message: env.Error.Message}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
	}
	return nil
}

// getJSON, postJSON, patchJSON, deleteJSON are convenience wrappers.
func (c *Client) getJSON(path string, out any) error { return c.do("GET", path, nil, out) }
func (c *Client) postJSON(path string, body, out any) error {
	return c.do("POST", path, body, out)
}
func (c *Client) patchJSON(path string, body, out any) error {
	return c.do("PATCH", path, body, out)
}
func (c *Client) deleteJSON(path string) error { return c.do("DELETE", path, nil, nil) }

// --- server ---

// ServerInfo fetches /server/info.
func (c *Client) ServerInfo() (*apitypes.ServerInfo, error) {
	var info apitypes.ServerInfo
	if err := c.getJSON("/api/v1/server/info", &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// ServerPubkey fetches the armored default public key.
func (c *Client) ServerPubkey() (string, error) {
	req, _ := http.NewRequest("GET", c.BaseURL+"/api/v1/server/pubkey", nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", parseErrorBody(resp.StatusCode, data)
	}
	return string(data), nil
}

// parseErrorBody builds an *APIError from a non-2xx response body, decoding the
// {"error": {...}} envelope when present.
func parseErrorBody(status int, data []byte) error {
	var env struct {
		Error httputil.APIError `json:"error"`
	}
	if err := json.Unmarshal(data, &env); err == nil && env.Error.Message != "" {
		return &APIError{Status: status, Code: env.Error.Code, Message: env.Error.Message}
	}
	return &APIError{Status: status, Message: strings.TrimSpace(string(data))}
}

// --- auth ---

// Register creates an account and returns the user + token.
func (c *Client) Register(username, password string) (*models.User, string, error) {
	var resp apitypes.AuthResponse
	if err := c.postJSON("/api/v1/auth/register", apitypes.RegisterRequest{Username: username, Password: password}, &resp); err != nil {
		return nil, "", err
	}
	return resp.User, resp.Token, nil
}

// Login authenticates and returns the user + token.
func (c *Client) Login(username, password string) (*models.User, string, error) {
	var resp apitypes.AuthResponse
	if err := c.postJSON("/api/v1/auth/login", apitypes.LoginRequest{Username: username, Password: password}, &resp); err != nil {
		return nil, "", err
	}
	return resp.User, resp.Token, nil
}

// Logout revokes the current token.
func (c *Client) Logout() error { return c.postJSON("/api/v1/auth/logout", nil, nil) }

// Me returns the current user.
func (c *Client) Me() (*models.User, error) {
	var u models.User
	if err := c.getJSON("/api/v1/me", &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// ListTokens returns the caller's tokens.
func (c *Client) ListTokens() ([]*models.APIToken, error) {
	var resp apitypes.ListResponse[*models.APIToken]
	if err := c.getJSON("/api/v1/me/tokens", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreateToken issues a new named token.
func (c *Client) CreateToken(name string) (*models.APIToken, error) {
	var t models.APIToken
	if err := c.postJSON("/api/v1/me/tokens", apitypes.CreateTokenRequest{Name: name}, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// RevokeToken revokes a token by id.
func (c *Client) RevokeToken(id string) error { return c.deleteJSON("/api/v1/me/tokens/" + id) }

// addQuery attaches query parameters to path.
func addQuery(path string, params map[string]string) string {
	if len(params) == 0 {
		return path
	}
	v := url.Values{}
	for k, val := range params {
		if val != "" {
			v.Set(k, val)
		}
	}
	q := v.Encode()
	if q == "" {
		return path
	}
	return path + "?" + q
}

// keep multipart referenced (used by PushPackage in Phase 9).
var _ = multipart.NewWriter
