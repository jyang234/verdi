// Package bus is svcfix's event bus: the "before" side of .flowmap.yaml's
// audit-before-publish obligation.
package bus

import "context"

// Bus publishes refund events. A fixture stub — it does not actually
// publish anything.
type Bus struct{}

// New returns a ready-to-use Bus.
func New() *Bus { return &Bus{} }

// Publish announces that id's refund was processed.
func (b *Bus) Publish(ctx context.Context, id string) error {
	return nil
}
