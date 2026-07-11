// Package audit is svcfix's audit-log writer: the "require" side of
// .flowmap.yaml's audit-before-publish obligation.
package audit

import "context"

// Store is the audit log. A fixture stub — it does not actually persist
// anything.
type Store struct{}

// New returns a ready-to-use audit Store.
func New() *Store { return &Store{} }

// Write records that id was audited.
func (s *Store) Write(ctx context.Context, id string) error {
	return nil
}
