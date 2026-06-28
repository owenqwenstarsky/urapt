// Package models defines the domain types used across urapt's database and API
// boundaries. These structs mirror the SQLite schema and are serialized into
// API DTOs by the restapi and apiclient packages.
package models

// User is an urapt account.
type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	IsAdmin   bool   `json:"is_admin"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// APIToken is a revocable authentication token. The plaintext token is only
// returned at creation/login time; only its hash and a short display prefix
// are persisted.
type APIToken struct {
	ID         string  `json:"id"`
	UserID     string  `json:"user_id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	Token      string  `json:"token,omitempty"` // plaintext, only on creation
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	RevokedAt  *string `json:"revoked_at,omitempty"`
}

// Visibility is whether a repository is world-readable.
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

// Access is a user's role on a repository.
type Access string

const (
	AccessRead      Access = "read"
	AccessWrite     Access = "write"
	AccessReadWrite Access = "read-write"
	AccessAdmin     Access = "admin"
)

// ValidAccess reports whether s is a recognized access level.
func ValidAccess(s string) bool {
	switch Access(s) {
	case AccessRead, AccessWrite, AccessReadWrite, AccessAdmin:
		return true
	}
	return false
}

// Repository is a named APT repository owned by a user.
type Repository struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	OwnerUserID string     `json:"owner_user_id"`
	Owner       *User      `json:"owner,omitempty"`
	Visibility  Visibility `json:"visibility"`
	Description string     `json:"description"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
}

// RepositoryMember is a user's access grant on a repository.
type RepositoryMember struct {
	RepositoryID string `json:"repository_id"`
	UserID       string `json:"user_id"`
	User         *User  `json:"user,omitempty"`
	Access       Access `json:"access"`
	CreatedAt    string `json:"created_at"`
}

// Distribution (suite) within a repository.
type Distribution struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Name         string `json:"name"`
	CreatedAt    string `json:"created_at"`
}

// Component within a distribution (e.g. main, contrib).
type Component struct {
	ID             string `json:"id"`
	DistributionID string `json:"distribution_id"`
	Name           string `json:"name"`
	CreatedAt      string `json:"created_at"`
}

// Architecture configured for a distribution (e.g. amd64). The special
// architecture "all" is implicit and never stored.
type Architecture struct {
	ID             string `json:"id"`
	DistributionID string `json:"distribution_id"`
	Name           string `json:"name"`
	CreatedAt      string `json:"created_at"`
}

// Package is one uploaded .deb version and its extracted control metadata.
type Package struct {
	ID               string `json:"id"`
	RepositoryID     string `json:"repository_id"`
	DistributionID   string `json:"distribution_id"`
	ComponentID      string `json:"component_id"`
	Name             string `json:"name"`
	Version          string `json:"version"`
	Architecture     string `json:"architecture"`
	Source           string `json:"source,omitempty"`
	Maintainer       string `json:"maintainer,omitempty"`
	Priority         string `json:"priority,omitempty"`
	Section          string `json:"section,omitempty"`
	Origin           string `json:"origin,omitempty"`
	Homepage         string `json:"homepage,omitempty"`
	Description      string `json:"description,omitempty"`
	DescriptionMD5   string `json:"description_md5,omitempty"`
	Depends          string `json:"depends,omitempty"`
	PreDepends       string `json:"pre_depends,omitempty"`
	Recommends       string `json:"recommends,omitempty"`
	Suggests         string `json:"suggests,omitempty"`
	Conflicts        string `json:"conflicts,omitempty"`
	Breaks           string `json:"breaks,omitempty"`
	Provides         string `json:"provides,omitempty"`
	Replaces         string `json:"replaces,omitempty"`
	Enhances         string `json:"enhances,omitempty"`
	InstalledSize    int64  `json:"installed_size,omitempty"`
	Essential        string `json:"essential,omitempty"`
	BuiltUsing       string `json:"built_using,omitempty"`
	Tag              string `json:"tag,omitempty"`
	RawControl       string `json:"raw_control"`
	Filename         string `json:"filename"`
	PoolPath         string `json:"pool_path"`
	Size             int64  `json:"size"`
	MD5sum           string `json:"md5sum"`
	SHA1             string `json:"sha1"`
	SHA256           string `json:"sha256"`
	UploadedByUserID string `json:"uploaded_by_user_id"`
	CreatedAt        string `json:"created_at"`
}

// Blob is a content-addressed .deb file on disk, reference-counted for dedup.
type Blob struct {
	SHA256    string `json:"sha256"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	RefCount  int64  `json:"ref_count"`
	CreatedAt string `json:"created_at"`
}

// GPGKey is a server-managed signing key.
type GPGKey struct {
	ID                string `json:"id"`
	Fingerprint       string `json:"fingerprint"`
	UserID            string `json:"user_id"`
	PublicKeyArmored  string `json:"public_key_armored"`
	PrivateKeyArmored string `json:"-"` // never serialized in API responses
	IsDefault         bool   `json:"is_default"`
	CreatedAt         string `json:"created_at"`
}

// AuditLogEntry is a best-effort record of a mutating action.
type AuditLogEntry struct {
	ID           string  `json:"id"`
	UserID       *string `json:"user_id,omitempty"`
	RepositoryID *string `json:"repository_id,omitempty"`
	Action       string  `json:"action"`
	Target       string  `json:"target"`
	Details      string  `json:"details,omitempty"`
	CreatedAt    string  `json:"created_at"`
}
