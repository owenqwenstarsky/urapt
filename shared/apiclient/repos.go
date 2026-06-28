package apiclient

import (
	"io"
	"net/http"

	apitypes "urapt/shared/api"
	"urapt/shared/models"
)

// newGetReq builds a GET request to url.
func newGetReq(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return req, nil
}

// readBody fully reads a response body.
func readBody(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}

// --- repositories ---

// ListRepositories returns repositories visible to the caller.
func (c *Client) ListRepositories() ([]*models.Repository, error) {
	var resp apitypes.ListResponse[*models.Repository]
	if err := c.getJSON("/api/v1/repositories", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreateRepository creates a new repository.
func (c *Client) CreateRepository(name, visibility, description string) (*models.Repository, error) {
	var repo models.Repository
	if err := c.postJSON("/api/v1/repositories", apitypes.CreateRepoRequest{
		Name: name, Visibility: visibility, Description: description,
	}, &repo); err != nil {
		return nil, err
	}
	return &repo, nil
}

// GetRepository returns a single repository by name.
func (c *Client) GetRepository(name string) (*models.Repository, error) {
	var repo models.Repository
	if err := c.getJSON("/api/v1/repositories/"+name, &repo); err != nil {
		return nil, err
	}
	return &repo, nil
}

// UpdateRepository mutates a repository. nil arguments leave fields unchanged.
func (c *Client) UpdateRepository(name string, newName *string, visibility *string, description *string) (*models.Repository, error) {
	var repo models.Repository
	if err := c.patchJSON("/api/v1/repositories/"+name, apitypes.UpdateRepoRequest{
		Name: newName, Visibility: visibility, Description: description,
	}, &repo); err != nil {
		return nil, err
	}
	return &repo, nil
}

// DeleteRepository removes a repository.
func (c *Client) DeleteRepository(name string) error {
	return c.deleteJSON("/api/v1/repositories/" + name)
}

// RepositoryPubkey returns the server's armored public key (repo-scoped path).
func (c *Client) RepositoryPubkey(name string) (string, error) {
	req, err := newGetReq(c.BaseURL + "/api/v1/repositories/" + name + "/pubkey")
	if err != nil {
		return "", err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := readBody(resp)
	if resp.StatusCode >= 400 {
		return "", parseErrorBody(resp.StatusCode, data)
	}
	return string(data), nil
}

// --- members ---

// ListMembers returns the members of a repository.
func (c *Client) ListMembers(repo string) ([]*models.RepositoryMember, error) {
	var out []*models.RepositoryMember
	if err := c.getJSON("/api/v1/repositories/"+repo+"/members", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddMember grants a user access on a repository.
func (c *Client) AddMember(repo, username, access string) (*models.RepositoryMember, error) {
	var m models.RepositoryMember
	if err := c.postJSON("/api/v1/repositories/"+repo+"/members",
		apitypes.AddMemberRequest{Username: username, Access: access}, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// UpdateMember changes a member's access.
func (c *Client) UpdateMember(repo, username, access string) (*models.RepositoryMember, error) {
	var m models.RepositoryMember
	if err := c.patchJSON("/api/v1/repositories/"+repo+"/members/"+username,
		apitypes.UpdateMemberRequest{Access: access}, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// RemoveMember revokes a user's access.
func (c *Client) RemoveMember(repo, username string) error {
	return c.deleteJSON("/api/v1/repositories/" + repo + "/members/" + username)
}

// --- distributions ---

// ListDistributions returns the distributions in a repository.
func (c *Client) ListDistributions(repo string) ([]*models.Distribution, error) {
	var out []*models.Distribution
	if err := c.getJSON("/api/v1/repositories/"+repo+"/distributions", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateDistribution adds a distribution.
func (c *Client) CreateDistribution(repo, name string) (*models.Distribution, error) {
	var d models.Distribution
	if err := c.postJSON("/api/v1/repositories/"+repo+"/distributions",
		apitypes.CreateNamedRequest{Name: name}, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// DeleteDistribution removes a distribution.
func (c *Client) DeleteDistribution(repo, name string) error {
	return c.deleteJSON("/api/v1/repositories/" + repo + "/distributions/" + name)
}

// --- components ---

// ListComponents returns the components in a distribution.
func (c *Client) ListComponents(repo, dist string) ([]*models.Component, error) {
	var out []*models.Component
	if err := c.getJSON("/api/v1/repositories/"+repo+"/distributions/"+dist+"/components", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateComponent adds a component.
func (c *Client) CreateComponent(repo, dist, name string) (*models.Component, error) {
	var comp models.Component
	if err := c.postJSON("/api/v1/repositories/"+repo+"/distributions/"+dist+"/components",
		apitypes.CreateNamedRequest{Name: name}, &comp); err != nil {
		return nil, err
	}
	return &comp, nil
}

// DeleteComponent removes a component.
func (c *Client) DeleteComponent(repo, dist, name string) error {
	return c.deleteJSON("/api/v1/repositories/" + repo + "/distributions/" + dist + "/components/" + name)
}

// --- architectures ---

// ListArchitectures returns the architectures in a distribution.
func (c *Client) ListArchitectures(repo, dist string) ([]*models.Architecture, error) {
	var out []*models.Architecture
	if err := c.getJSON("/api/v1/repositories/"+repo+"/distributions/"+dist+"/architectures", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateArchitecture adds an architecture.
func (c *Client) CreateArchitecture(repo, dist, name string) (*models.Architecture, error) {
	var a models.Architecture
	if err := c.postJSON("/api/v1/repositories/"+repo+"/distributions/"+dist+"/architectures",
		apitypes.CreateNamedRequest{Name: name}, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// DeleteArchitecture removes an architecture.
func (c *Client) DeleteArchitecture(repo, dist, name string) error {
	return c.deleteJSON("/api/v1/repositories/" + repo + "/distributions/" + dist + "/architectures/" + name)
}
