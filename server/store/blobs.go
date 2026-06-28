package store

import (
	"context"
	"fmt"
)

// GetBlob returns a blob by its sha256, or ErrNotFound.
func (s *Store) GetBlob(ctx context.Context, sha256 string) (filename string, size, refCount int64, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT filename, size, ref_count FROM blobs WHERE sha256 = ?`, sha256).
		Scan(&filename, &size, &refCount)
	if isErrNoRows(err) {
		return "", 0, 0, ErrNotFound
	}
	return filename, size, refCount, err
}

// CreateBlob creates a new blob row with ref_count=1. It returns
// (created=true) when a new row was inserted, or (created=false) when the
// blob already existed (in which case its ref_count is left unchanged here;
// use IncBlobRef to bump it).
func (s *Store) CreateBlob(ctx context.Context, sha256, filename string, size int64) (created bool, err error) {
	now := s.now()
	res, err := s.exec(ctx, `INSERT OR IGNORE INTO blobs (sha256, filename, size, ref_count, created_at) VALUES (?, ?, ?, 1, ?)`,
		sha256, filename, size, now)
	if err != nil {
		return false, fmt.Errorf("insert blob: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// IncBlobRef atomically increments a blob's ref_count and returns the new value.
func (s *Store) IncBlobRef(ctx context.Context, sha256 string) (int64, error) {
	res, err := s.exec(ctx, `UPDATE blobs SET ref_count = ref_count + 1 WHERE sha256 = ?`, sha256)
	if err != nil {
		return 0, fmt.Errorf("inc blob: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return 0, ErrNotFound
	}
	var rc int64
	if err := s.db.QueryRowContext(ctx, `SELECT ref_count FROM blobs WHERE sha256 = ?`, sha256).Scan(&rc); err != nil {
		return 0, err
	}
	return rc, nil
}

// DecBlobRef atomically decrements a blob's ref_count and returns the new
// value. When it reaches 0 the caller should delete the on-disk file and call
// DeleteBlob.
func (s *Store) DecBlobRef(ctx context.Context, sha256 string) (int64, error) {
	res, err := s.exec(ctx, `UPDATE blobs SET ref_count = ref_count - 1 WHERE sha256 = ? AND ref_count > 0`, sha256)
	if err != nil {
		return 0, fmt.Errorf("dec blob: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var rc int64
		if e := s.db.QueryRowContext(ctx, `SELECT ref_count FROM blobs WHERE sha256 = ?`, sha256).Scan(&rc); e != nil {
			if isErrNoRows(e) {
				return 0, ErrNotFound
			}
			return 0, e
		}
		return rc, nil
	}
	var rc int64
	if err := s.db.QueryRowContext(ctx, `SELECT ref_count FROM blobs WHERE sha256 = ?`, sha256).Scan(&rc); err != nil {
		return 0, err
	}
	return rc, nil
}

// DeleteBlob removes a blob row.
func (s *Store) DeleteBlob(ctx context.Context, sha256 string) error {
	_, err := s.exec(ctx, `DELETE FROM blobs WHERE sha256 = ?`, sha256)
	return err
}
