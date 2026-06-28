package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"urapt/shared/models"
)

// SuitePackage is a package row joined with its component and distribution
// names, used by the APT index generator.
type SuitePackage struct {
	models.Package
	ComponentName    string
	DistributionName string
}

// CreatePackage inserts a new package row.
func (s *Store) CreatePackage(ctx context.Context, p *models.Package) error {
	if p.ID == "" {
		p.ID = newID()
	}
	if p.CreatedAt == "" {
		p.CreatedAt = s.now()
	}
	_, err := s.exec(ctx, `INSERT INTO packages (
		id, repository_id, distribution_id, component_id,
		name, version, architecture, source, maintainer, priority, section,
		origin, homepage, description, description_md5, depends, pre_depends,
		recommends, suggests, conflicts, breaks, provides, replaces, enhances,
		installed_size, essential, built_using, tag, raw_control, filename,
		pool_path, size, md5sum, sha1, sha256, uploaded_by_user_id, created_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.RepositoryID, p.DistributionID, p.ComponentID,
		p.Name, p.Version, p.Architecture, p.Source, p.Maintainer, p.Priority, p.Section,
		p.Origin, p.Homepage, p.Description, p.DescriptionMD5, p.Depends, p.PreDepends,
		p.Recommends, p.Suggests, p.Conflicts, p.Breaks, p.Provides, p.Replaces, p.Enhances,
		p.InstalledSize, p.Essential, p.BuiltUsing, p.Tag, p.RawControl, p.Filename,
		p.PoolPath, p.Size, p.MD5sum, p.SHA1, p.SHA256, p.UploadedByUserID, p.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert package: %w", err)
	}
	return nil
}

// GetPackageByID returns a package by id.
func (s *Store) GetPackageByID(ctx context.Context, id string) (*models.Package, error) {
	row := s.db.QueryRowContext(ctx, packageCols+` FROM packages WHERE id = ?`, id)
	return scanPackage(row)
}

// GetPackageByPoolPath returns a package within a repository by its pool_path.
func (s *Store) GetPackageByPoolPath(ctx context.Context, repoID, poolPath string) (*models.Package, error) {
	row := s.db.QueryRowContext(ctx, packageCols+` FROM packages WHERE repository_id = ? AND pool_path = ?`, repoID, poolPath)
	return scanPackage(row)
}

// PackageFilters controls ListPackages filtering.
type PackageFilters struct {
	ComponentID string
	Arch        string
	Name        string
	Query       string
}

// ListPackages lists packages within a (repo, distribution) with optional
// filters and pagination.
func (s *Store) ListPackages(ctx context.Context, repoID, distroID string, f PackageFilters, page, perPage int) ([]*models.Package, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}
	var where []string
	var args []any
	where = append(where, "repository_id = ?", "distribution_id = ?")
	args = append(args, repoID, distroID)
	if f.ComponentID != "" {
		where = append(where, "component_id = ?")
		args = append(args, f.ComponentID)
	}
	if f.Arch != "" {
		where = append(where, "(architecture = ? OR architecture = 'all')")
		args = append(args, f.Arch)
	}
	if f.Name != "" {
		where = append(where, "name = ?")
		args = append(args, f.Name)
	}
	if f.Query != "" {
		where = append(where, "(name LIKE ? OR description LIKE ?)")
		args = append(args, "%"+f.Query+"%", "%"+f.Query+"%")
	}
	q := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM packages WHERE `+q, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count packages: %w", err)
	}
	args2 := append(args, perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, packageCols+` FROM packages WHERE `+q+` ORDER BY name, version LIMIT ? OFFSET ?`, args2...)
	if err != nil {
		return nil, 0, fmt.Errorf("list packages: %w", err)
	}
	defer rows.Close()
	var out []*models.Package
	for rows.Next() {
		p, err := scanPackageRows(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// ListSuitePackages returns all packages in a (repo, distribution) joined with
// their component and distribution names, for APT index generation.
func (s *Store) ListSuitePackages(ctx context.Context, repoID, distroID string) ([]SuitePackage, error) {
	cols := qualifiedPackageCols("p")
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+cols+`, c.name, d.name
		FROM packages p
		JOIN components c ON c.id = p.component_id
		JOIN distributions d ON d.id = p.distribution_id
		WHERE p.repository_id = ? AND p.distribution_id = ?`, repoID, distroID)
	if err != nil {
		return nil, fmt.Errorf("list suite packages: %w", err)
	}
	defer rows.Close()
	var out []SuitePackage
	for rows.Next() {
		var sp SuitePackage
		if err := scanPackageColsFull(rows.Scan, &sp.Package, &sp.ComponentName, &sp.DistributionName); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

// scanPackageColsFull scans the 37 package columns plus the joined component
// name and distribution name.
func scanPackageColsFull(scan scanFunc, p *models.Package, compName, distName *string) error {
	return scan(
		&p.ID, &p.RepositoryID, &p.DistributionID, &p.ComponentID,
		&p.Name, &p.Version, &p.Architecture, &p.Source, &p.Maintainer, &p.Priority, &p.Section,
		&p.Origin, &p.Homepage, &p.Description, &p.DescriptionMD5, &p.Depends, &p.PreDepends,
		&p.Recommends, &p.Suggests, &p.Conflicts, &p.Breaks, &p.Provides, &p.Replaces, &p.Enhances,
		&p.InstalledSize, &p.Essential, &p.BuiltUsing, &p.Tag, &p.RawControl, &p.Filename,
		&p.PoolPath, &p.Size, &p.MD5sum, &p.SHA1, &p.SHA256, &p.UploadedByUserID, &p.CreatedAt,
		compName, distName,
	)
}

// ListPackageBlobSHA256sByRepo returns the sha256 of every package in a repo
// (with duplicates), used during repository deletion to decrement ref counts.
func (s *Store) ListPackageBlobSHA256sByRepo(ctx context.Context, repoID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT sha256 FROM packages WHERE repository_id = ?`, repoID)
	if err != nil {
		return nil, fmt.Errorf("list repo blobs: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var sha string
		if err := rows.Scan(&sha); err != nil {
			return nil, err
		}
		out = append(out, sha)
	}
	return out, rows.Err()
}

// DeletePackage removes a package row by id.
func (s *Store) DeletePackage(ctx context.Context, id string) error {
	_, err := s.exec(ctx, `DELETE FROM packages WHERE id = ?`, id)
	return err
}

// DeletePackagesByRepo removes all package rows in a repository.
func (s *Store) DeletePackagesByRepo(ctx context.Context, repoID string) error {
	_, err := s.exec(ctx, `DELETE FROM packages WHERE repository_id = ?`, repoID)
	return err
}

const packageColNames = `id, repository_id, distribution_id, component_id, name, version, architecture, source, maintainer, priority, section, origin, homepage, description, description_md5, depends, pre_depends, recommends, suggests, conflicts, breaks, provides, replaces, enhances, installed_size, essential, built_using, tag, raw_control, filename, pool_path, size, md5sum, sha1, sha256, uploaded_by_user_id, created_at`

const packageCols = `SELECT ` + packageColNames

// qualifiedPackageCols returns the package column list prefixed with alias.,
// e.g. "p.id, p.repository_id, ...", for use in JOINs.
func qualifiedPackageCols(alias string) string {
	parts := strings.Split(packageColNames, ", ")
	for i, p := range parts {
		parts[i] = alias + "." + p
	}
	return strings.Join(parts, ", ")
}

func scanPackage(row *sql.Row) (*models.Package, error) {
	p := &models.Package{}
	if err := scanPackageCols(row.Scan, p); err != nil {
		if isErrNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func scanPackageRows(rows *sql.Rows) (*models.Package, error) {
	p := &models.Package{}
	if err := scanPackageCols(rows.Scan, p); err != nil {
		return nil, err
	}
	return p, nil
}

// scanFunc abstracts *sql.Row.Scan and *sql.Rows.Scan.
type scanFunc func(dest ...any) error

func scanPackageCols(scan scanFunc, p *models.Package) error {
	return scan(
		&p.ID, &p.RepositoryID, &p.DistributionID, &p.ComponentID,
		&p.Name, &p.Version, &p.Architecture, &p.Source, &p.Maintainer, &p.Priority, &p.Section,
		&p.Origin, &p.Homepage, &p.Description, &p.DescriptionMD5, &p.Depends, &p.PreDepends,
		&p.Recommends, &p.Suggests, &p.Conflicts, &p.Breaks, &p.Provides, &p.Replaces, &p.Enhances,
		&p.InstalledSize, &p.Essential, &p.BuiltUsing, &p.Tag, &p.RawControl, &p.Filename,
		&p.PoolPath, &p.Size, &p.MD5sum, &p.SHA1, &p.SHA256, &p.UploadedByUserID, &p.CreatedAt,
	)
}
