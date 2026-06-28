package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"urapt/shared/models"
)

// CreateUser inserts a new user. If the users table is empty, the user is made
// an admin (the bootstrap-admin rule).
func (s *Store) CreateUser(ctx context.Context, username, passwordHash string) (*models.User, bool, error) {
	username = strings.TrimSpace(username)
	lc := strings.ToLower(username)
	id := newID()
	now := s.now()

	var admin bool
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return nil, false, fmt.Errorf("count users: %w", err)
	}
	admin = count == 0

	_, err := s.exec(ctx, `INSERT INTO users (id, username, username_lc, password_hash, is_admin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, username, lc, passwordHash, boolToInt(admin), now, now)
	if err != nil {
		return nil, false, fmt.Errorf("insert user: %w", err)
	}
	return &models.User{
		ID: id, Username: username, IsAdmin: admin, CreatedAt: now, UpdatedAt: now,
	}, admin, nil
}

// GetUserByID returns the user with the given id.
func (s *Store) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, username, is_admin, created_at, updated_at FROM users WHERE id = ?`, id)
	return scanUser(row)
}

// GetUserByUsername returns the user with the given (case-insensitive) username.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, username, is_admin, created_at, updated_at FROM users WHERE username_lc = ?`,
		strings.ToLower(strings.TrimSpace(username)))
	return scanUser(row)
}

// ListUsers returns all users ordered by username.
func (s *Store) ListUsers(ctx context.Context) ([]*models.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, username, is_admin, created_at, updated_at FROM users ORDER BY username_lc`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var out []*models.User
	for rows.Next() {
		u, err := scanUserRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// GetUserPasswordHash returns the stored bcrypt hash for a user.
func (s *Store) GetUserPasswordHash(ctx context.Context, id string) (string, error) {
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, id).Scan(&hash)
	if isErrNoRows(err) {
		return "", ErrNotFound
	}
	return hash, err
}

// UpdateUser updates mutable fields. If isAdmin is nil, it is left unchanged.
func (s *Store) UpdateUser(ctx context.Context, id string, isAdmin *bool) error {
	now := s.now()
	if isAdmin != nil {
		if _, err := s.exec(ctx, `UPDATE users SET is_admin = ?, updated_at = ? WHERE id = ?`,
			boolToInt(*isAdmin), now, id); err != nil {
			return fmt.Errorf("update user: %w", err)
		}
	}
	return nil
}

// DeleteUser removes a user. The caller should prevent self-deletion.
func (s *Store) DeleteUser(ctx context.Context, id string) error {
	if _, err := s.exec(ctx, `DELETE FROM users WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func scanUser(row *sql.Row) (*models.User, error) {
	u := &models.User{}
	err := row.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
	if isErrNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func scanUserRows(rows *sql.Rows) (*models.User, error) {
	u := &models.User{}
	if err := rows.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, err
	}
	return u, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
