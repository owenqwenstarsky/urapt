package store

import "context"

// RecordAudit inserts a best-effort audit log entry. Errors are ignored at the
// call site's discretion; this helper returns the error for completeness.
func (s *Store) RecordAudit(ctx context.Context, userID, repositoryID *string, action, target, details string) error {
	_, err := s.exec(ctx, `INSERT INTO audit_log (id, user_id, repository_id, action, target, details, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		newID(), nullable(userID), nullable(repositoryID), action, target, details, s.now())
	return err
}

func nullable(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}
