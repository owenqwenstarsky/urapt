package store

import (
	"context"
	"database/sql"
	"fmt"

	"urapt/shared/models"
)

// CreateToken inserts a new API token row. The plaintext token is NOT stored;
// only its SHA-256 hash and display prefix are.
func (s *Store) CreateToken(ctx context.Context, userID, name, prefix, tokenHash string) (*models.APIToken, error) {
	id := newID()
	now := s.now()
	_, err := s.exec(ctx, `INSERT INTO api_tokens (id, user_id, name, prefix, token_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, id, userID, name, prefix, tokenHash, now)
	if err != nil {
		return nil, fmt.Errorf("insert token: %w", err)
	}
	return &models.APIToken{
		ID: id, UserID: userID, Name: name, Prefix: prefix, CreatedAt: now,
	}, nil
}

// GetTokenByHash returns the active (non-revoked) token with the given hash.
func (s *Store) GetTokenByHash(ctx context.Context, hash string) (*models.APIToken, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, prefix, created_at, last_used_at, revoked_at
		 FROM api_tokens WHERE token_hash = ? AND revoked_at IS NULL`, hash)
	t := &models.APIToken{}
	var lastUsed, revoked sql.NullString
	err := row.Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.CreatedAt, &lastUsed, &revoked)
	if isErrNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if lastUsed.Valid {
		v := lastUsed.String
		t.LastUsedAt = &v
	}
	if revoked.Valid {
		v := revoked.String
		t.RevokedAt = &v
	}
	return t, nil
}

// TouchToken updates last_used_at for a token.
func (s *Store) TouchToken(ctx context.Context, id string) error {
	_, err := s.exec(ctx, `UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, s.now(), id)
	return err
}

// ListTokens returns all tokens for a user (including revoked).
func (s *Store) ListTokens(ctx context.Context, userID string) ([]*models.APIToken, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, prefix, created_at, last_used_at, revoked_at
		 FROM api_tokens WHERE user_id = ? ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	defer rows.Close()
	var out []*models.APIToken
	for rows.Next() {
		t := &models.APIToken{}
		var lastUsed, revoked sql.NullString
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.CreatedAt, &lastUsed, &revoked); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			v := lastUsed.String
			t.LastUsedAt = &v
		}
		if revoked.Valid {
			v := revoked.String
			t.RevokedAt = &v
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RevokeToken marks the given token revoked. It must belong to userID.
func (s *Store) RevokeToken(ctx context.Context, userID, tokenID string) error {
	res, err := s.exec(ctx, `UPDATE api_tokens SET revoked_at = ? WHERE id = ? AND user_id = ?`,
		s.now(), tokenID, userID)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeTokenByHash marks the token with the given hash revoked (used for logout).
func (s *Store) RevokeTokenByHash(ctx context.Context, hash string) error {
	_, err := s.exec(ctx, `UPDATE api_tokens SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL`,
		s.now(), hash)
	return err
}
