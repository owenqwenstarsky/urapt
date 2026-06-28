package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"urapt/shared/models"
)

// PackageListResponse is the shape returned by the packages list endpoint.
type PackageListResponse struct {
	Items   []*models.Package `json:"items"`
	Page    int               `json:"page"`
	PerPage int               `json:"per_page"`
	Total   int               `json:"total"`
}

// ListPackages lists packages in a (repo, distribution) with optional filters.
func (c *Client) ListPackages(repo, dist string, filters map[string]string) (*PackageListResponse, error) {
	var resp PackageListResponse
	path := addQuery("/api/v1/repositories/"+repo+"/distributions/"+dist+"/packages", filters)
	if err := c.getJSON(path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetPackage returns a single package by id.
func (c *Client) GetPackage(repo, id string) (*models.Package, error) {
	var p models.Package
	if err := c.getJSON("/api/v1/repositories/"+repo+"/packages/"+id, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// PushPackage uploads a .deb file to a repository/distribution/component. The
// upload is streamed via multipart/form-data.
func (c *Client) PushPackage(repo, dist, component, filePath string) (*models.Package, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		_ = writer.WriteField("component", component)
		part, err := writer.CreateFormFile("file", filepath.Base(filePath))
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, f); err != nil {
			pw.CloseWithError(err)
			return
		}
		_ = writer.Close()
	}()

	req, err := http.NewRequest("POST", c.BaseURL+"/api/v1/repositories/"+repo+"/distributions/"+dist+"/packages", pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, parseErrorBody(resp.StatusCode, data)
	}
	var p models.Package
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &p, nil
}

// DownloadPackage fetches a package's .deb file and writes it to outFile. If
// outFile is empty, the bytes are written to stdout.
func (c *Client) DownloadPackage(repo, id, outFile string) error {
	req, err := http.NewRequest("GET", c.BaseURL+"/api/v1/repositories/"+repo+"/packages/"+id+"/file", nil)
	if err != nil {
		return err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return parseErrorBody(resp.StatusCode, data)
	}
	var w io.Writer
	if outFile == "" || outFile == "-" {
		w = os.Stdout
	} else {
		f, err := os.Create(outFile)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

// DeletePackage removes a package by id.
func (c *Client) DeletePackage(repo, id string) error {
	return c.deleteJSON("/api/v1/repositories/" + repo + "/packages/" + id)
}

// ParsePackageSpec splits a "name[@version][:arch]" specifier into its parts.
// A string that looks like a UUID is treated as an id.
func ParsePackageSpec(spec string) (id, name, version, arch string) {
	spec = strings.TrimSpace(spec)
	if isUUID(spec) {
		return spec, "", "", ""
	}
	// split :arch
	if i := strings.IndexByte(spec, ':'); i >= 0 {
		arch = spec[i+1:]
		spec = spec[:i]
	}
	// split @version
	if i := strings.IndexByte(spec, '@'); i >= 0 {
		version = spec[i+1:]
		spec = spec[:i]
	}
	name = spec
	return "", name, version, arch
}

// isUUID reports whether s looks like a UUIDv4.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	return s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}
