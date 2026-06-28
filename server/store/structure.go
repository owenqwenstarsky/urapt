package store

import (
	"context"
	"database/sql"
	"fmt"

	"urapt/shared/models"
)

// --- distributions ---

// CreateDistribution adds a distribution (suite) to a repository.
func (s *Store) CreateDistribution(ctx context.Context, repoID, name string) (*models.Distribution, error) {
	id := newID()
	now := s.now()
	_, err := s.exec(ctx, `INSERT INTO distributions (id, repository_id, name, created_at) VALUES (?, ?, ?, ?)`,
		id, repoID, name, now)
	if err != nil {
		return nil, fmt.Errorf("insert distribution: %w", err)
	}
	return &models.Distribution{ID: id, RepositoryID: repoID, Name: name, CreatedAt: now}, nil
}

// GetDistributionByName returns a distribution by name within a repository.
func (s *Store) GetDistributionByName(ctx context.Context, repoID, name string) (*models.Distribution, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, repository_id, name, created_at FROM distributions WHERE repository_id = ? AND name = ?`, repoID, name)
	d := &models.Distribution{}
	err := row.Scan(&d.ID, &d.RepositoryID, &d.Name, &d.CreatedAt)
	if isErrNoRows(err) {
		return nil, ErrNotFound
	}
	return d, err
}

// ListDistributions returns all distributions in a repository.
func (s *Store) ListDistributions(ctx context.Context, repoID string) ([]*models.Distribution, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, repository_id, name, created_at FROM distributions WHERE repository_id = ? ORDER BY name`, repoID)
	if err != nil {
		return nil, fmt.Errorf("list distributions: %w", err)
	}
	defer rows.Close()
	var out []*models.Distribution
	for rows.Next() {
		d := &models.Distribution{}
		if err := rows.Scan(&d.ID, &d.RepositoryID, &d.Name, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeleteDistribution removes a distribution and cascades to components,
// architectures, and packages.
func (s *Store) DeleteDistribution(ctx context.Context, repoID, name string) error {
	res, err := s.exec(ctx, `DELETE FROM distributions WHERE repository_id = ? AND name = ?`, repoID, name)
	if err != nil {
		return fmt.Errorf("delete distribution: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- components ---

// CreateComponent adds a component to a distribution.
func (s *Store) CreateComponent(ctx context.Context, distroID, name string) (*models.Component, error) {
	id := newID()
	now := s.now()
	_, err := s.exec(ctx, `INSERT INTO components (id, distribution_id, name, created_at) VALUES (?, ?, ?, ?)`,
		id, distroID, name, now)
	if err != nil {
		return nil, fmt.Errorf("insert component: %w", err)
	}
	return &models.Component{ID: id, DistributionID: distroID, Name: name, CreatedAt: now}, nil
}

// GetComponentByName returns a component by name within a distribution.
func (s *Store) GetComponentByName(ctx context.Context, distroID, name string) (*models.Component, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, distribution_id, name, created_at FROM components WHERE distribution_id = ? AND name = ?`, distroID, name)
	c := &models.Component{}
	err := row.Scan(&c.ID, &c.DistributionID, &c.Name, &c.CreatedAt)
	if isErrNoRows(err) {
		return nil, ErrNotFound
	}
	return c, err
}

// ListComponents returns all components in a distribution.
func (s *Store) ListComponents(ctx context.Context, distroID string) ([]*models.Component, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, distribution_id, name, created_at FROM components WHERE distribution_id = ? ORDER BY name`, distroID)
	if err != nil {
		return nil, fmt.Errorf("list components: %w", err)
	}
	defer rows.Close()
	var out []*models.Component
	for rows.Next() {
		c := &models.Component{}
		if err := rows.Scan(&c.ID, &c.DistributionID, &c.Name, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteComponent removes a component. The schema blocks deletion while
// packages reference it (ON DELETE RESTRICT); the handler checks first.
func (s *Store) DeleteComponent(ctx context.Context, distroID, name string) error {
	res, err := s.exec(ctx, `DELETE FROM components WHERE distribution_id = ? AND name = ?`, distroID, name)
	if err != nil {
		return fmt.Errorf("delete component: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountComponentsByDistro reports how many components a distribution has.
func (s *Store) CountComponentsByDistro(ctx context.Context, distroID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM components WHERE distribution_id = ?`, distroID).Scan(&n)
	return n, err
}

// --- architectures ---

// CreateArchitecture adds an architecture to a distribution.
func (s *Store) CreateArchitecture(ctx context.Context, distroID, name string) (*models.Architecture, error) {
	id := newID()
	now := s.now()
	_, err := s.exec(ctx, `INSERT INTO architectures (id, distribution_id, name, created_at) VALUES (?, ?, ?, ?)`,
		id, distroID, name, now)
	if err != nil {
		return nil, fmt.Errorf("insert architecture: %w", err)
	}
	return &models.Architecture{ID: id, DistributionID: distroID, Name: name, CreatedAt: now}, nil
}

// ListArchitectures returns all architectures in a distribution.
func (s *Store) ListArchitectures(ctx context.Context, distroID string) ([]*models.Architecture, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, distribution_id, name, created_at FROM architectures WHERE distribution_id = ? ORDER BY name`, distroID)
	if err != nil {
		return nil, fmt.Errorf("list architectures: %w", err)
	}
	defer rows.Close()
	var out []*models.Architecture
	for rows.Next() {
		a := &models.Architecture{}
		if err := rows.Scan(&a.ID, &a.DistributionID, &a.Name, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DeleteArchitecture removes an architecture from a distribution.
func (s *Store) DeleteArchitecture(ctx context.Context, distroID, name string) error {
	res, err := s.exec(ctx, `DELETE FROM architectures WHERE distribution_id = ? AND name = ?`, distroID, name)
	if err != nil {
		return fmt.Errorf("delete architecture: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// HasArchitecture reports whether a distribution declares the given arch.
func (s *Store) HasArchitecture(ctx context.Context, distroID, name string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM architectures WHERE distribution_id = ? AND name = ?`, distroID, name).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return n > 0, err
}
