package store

import (
	"context"
	"database/sql"
	"fmt"

	"urapt/shared/models"
)

// CreateRepository inserts a new repository owned by userID.
func (s *Store) CreateRepository(ctx context.Context, name, ownerUserID string, visibility models.Visibility, description string) (*models.Repository, error) {
	id := newID()
	now := s.now()
	_, err := s.exec(ctx, `INSERT INTO repositories (id, name, owner_user_id, visibility, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, id, name, ownerUserID, string(visibility), description, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert repository: %w", err)
	}
	return &models.Repository{
		ID: id, Name: name, OwnerUserID: ownerUserID, Visibility: visibility,
		Description: description, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// ListReposVisible returns repositories visible to userID: owned, public, or
// where the user is a member.
func (s *Store) ListReposVisible(ctx context.Context, userID string) ([]*models.Repository, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT r.id, r.name, r.owner_user_id, r.visibility, r.description, r.created_at, r.updated_at
		FROM repositories r
		WHERE r.owner_user_id = ?
		   OR r.visibility = 'public'
		   OR EXISTS (SELECT 1 FROM repository_members m WHERE m.repository_id = r.id AND m.user_id = ?)
		ORDER BY r.name`, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("list repos: %w", err)
	}
	defer rows.Close()
	var out []*models.Repository
	for rows.Next() {
		r := &models.Repository{}
		if err := rows.Scan(&r.ID, &r.Name, &r.OwnerUserID, &r.Visibility, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateRepository mutates a repository's name, visibility, and description.
// Empty strings leave the field unchanged.
func (s *Store) UpdateRepository(ctx context.Context, id, name string, visibility *models.Visibility, description *string) error {
	now := s.now()
	if name != "" {
		if _, err := s.exec(ctx, `UPDATE repositories SET name = ?, updated_at = ? WHERE id = ?`, name, now, id); err != nil {
			return fmt.Errorf("update repo name: %w", err)
		}
	}
	if visibility != nil {
		if _, err := s.exec(ctx, `UPDATE repositories SET visibility = ?, updated_at = ? WHERE id = ?`, string(*visibility), now, id); err != nil {
			return fmt.Errorf("update repo visibility: %w", err)
		}
	}
	if description != nil {
		if _, err := s.exec(ctx, `UPDATE repositories SET description = ?, updated_at = ? WHERE id = ?`, *description, now, id); err != nil {
			return fmt.Errorf("update repo description: %w", err)
		}
	}
	return nil
}

// DeleteRepository deletes a repository and all its child rows (cascade).
// The caller must have already handled blob ref-count cleanup.
func (s *Store) DeleteRepository(ctx context.Context, id string) error {
	if _, err := s.exec(ctx, `DELETE FROM repositories WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete repository: %w", err)
	}
	return nil
}

// GetRepositoryByName returns a repository by its unique name.
func (s *Store) GetRepositoryByName(ctx context.Context, name string) (*models.Repository, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, owner_user_id, visibility, description, created_at, updated_at
		 FROM repositories WHERE name = ?`, name)
	return scanRepo(row)
}

// GetRepositoryByID returns a repository by id.
func (s *Store) GetRepositoryByID(ctx context.Context, id string) (*models.Repository, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, owner_user_id, visibility, description, created_at, updated_at
		 FROM repositories WHERE id = ?`, id)
	return scanRepo(row)
}

func scanRepo(row *sql.Row) (*models.Repository, error) {
	r := &models.Repository{}
	err := row.Scan(&r.ID, &r.Name, &r.OwnerUserID, &r.Visibility, &r.Description, &r.CreatedAt, &r.UpdatedAt)
	if isErrNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

// GetMemberAccess returns the access level granted to userID on repoID, or
// ("", false) if the user is not an explicit member.
func (s *Store) GetMemberAccess(ctx context.Context, repoID, userID string) (models.Access, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT access FROM repository_members WHERE repository_id = ? AND user_id = ?`,
		repoID, userID)
	var access string
	err := row.Scan(&access)
	if isErrNoRows(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get member access: %w", err)
	}
	return models.Access(access), true, nil
}
