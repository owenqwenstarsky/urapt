package store

import (
	"context"
	"fmt"

	"urapt/shared/models"
)

// AddMember grants a user access on a repository. If a grant already exists it
// is updated.
func (s *Store) AddMember(ctx context.Context, repoID, userID string, access models.Access) error {
	now := s.now()
	_, err := s.exec(ctx, `INSERT INTO repository_members (repository_id, user_id, access, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(repository_id, user_id) DO UPDATE SET access = excluded.access`,
		repoID, userID, string(access), now)
	if err != nil {
		return fmt.Errorf("upsert member: %w", err)
	}
	return nil
}

// UpdateMemberAccess changes a user's access on a repository.
func (s *Store) UpdateMemberAccess(ctx context.Context, repoID, userID string, access models.Access) error {
	res, err := s.exec(ctx, `UPDATE repository_members SET access = ? WHERE repository_id = ? AND user_id = ?`,
		string(access), repoID, userID)
	if err != nil {
		return fmt.Errorf("update member: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveMember revokes a user's access on a repository.
func (s *Store) RemoveMember(ctx context.Context, repoID, userID string) error {
	res, err := s.exec(ctx, `DELETE FROM repository_members WHERE repository_id = ? AND user_id = ?`, repoID, userID)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListMembers returns all members of a repository with their user records.
func (s *Store) ListMembers(ctx context.Context, repoID string) ([]*models.RepositoryMember, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.repository_id, m.user_id, m.access, m.created_at,
		       u.id, u.username, u.is_admin, u.created_at, u.updated_at
		FROM repository_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.repository_id = ?
		ORDER BY u.username`, repoID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()
	var out []*models.RepositoryMember
	for rows.Next() {
		m := &models.RepositoryMember{User: &models.User{}}
		if err := rows.Scan(
			&m.RepositoryID, &m.UserID, &m.Access, &m.CreatedAt,
			&m.User.ID, &m.User.Username, &m.User.IsAdmin, &m.User.CreatedAt, &m.User.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
