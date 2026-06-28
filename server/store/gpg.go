package store

import (
	"context"
	"fmt"

	"urapt/shared/models"
)

// SaveGPGKey inserts a GPG key row.
func (s *Store) SaveGPGKey(ctx context.Context, fingerprint, userID, pubArmored, privArmored string, isDefault bool) (*models.GPGKey, error) {
	id := newID()
	now := s.now()
	_, err := s.exec(ctx, `INSERT INTO gpg_keys (id, fingerprint, user_id, public_key_armored, private_key_armored, is_default, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, fingerprint, userID, pubArmored, privArmored, boolToInt(isDefault), now)
	if err != nil {
		return nil, fmt.Errorf("insert gpg key: %w", err)
	}
	return &models.GPGKey{
		ID: id, Fingerprint: fingerprint, UserID: userID,
		PublicKeyArmored: pubArmored, IsDefault: isDefault, CreatedAt: now,
	}, nil
}

// GetDefaultGPGKey returns the default signing key, or ErrNotFound if none.
func (s *Store) GetDefaultGPGKey(ctx context.Context) (*models.GPGKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, fingerprint, user_id, public_key_armored, private_key_armored, is_default, created_at
		 FROM gpg_keys WHERE is_default = 1 LIMIT 1`)
	k := &models.GPGKey{}
	var isDefault int
	err := row.Scan(&k.ID, &k.Fingerprint, &k.UserID, &k.PublicKeyArmored, &k.PrivateKeyArmored, &isDefault, &k.CreatedAt)
	if isErrNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	k.IsDefault = isDefault == 1
	return k, nil
}

// GetGPGKeyPublic returns just the armored public key of the default key.
func (s *Store) GetGPGKeyPublic(ctx context.Context) (string, error) {
	row := s.db.QueryRowContext(ctx, `SELECT public_key_armored FROM gpg_keys WHERE is_default = 1 LIMIT 1`)
	var pub string
	err := row.Scan(&pub)
	if isErrNoRows(err) {
		return "", ErrNotFound
	}
	return pub, err
}
