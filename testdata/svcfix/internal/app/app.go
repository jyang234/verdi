// Package app holds svcfix's business logic: the two operations behind
// the boundary contract's HTTP entrypoints.
package app

import (
	"context"

	"example.com/svcfix/internal/audit"
	"example.com/svcfix/internal/bus"
)

// Service implements the refund use cases.
type Service struct {
	audit *audit.Store
	bus   *bus.Bus
}

// New wires a Service from its two dependencies.
func New(a *audit.Store, b *bus.Bus) *Service {
	return &Service{audit: a, bus: b}
}

// GetRefund returns the refund status for id. A fixture stub — it always
// echoes the id back.
func (s *Service) GetRefund(ctx context.Context, id string) (string, error) {
	return id, nil
}

// PublishRefund writes an audit record for id and then publishes it —
// satisfying .flowmap.yaml's audit-before-publish obligation (require
// audit#Write before bus#Publish).
func (s *Service) PublishRefund(ctx context.Context, id string) error {
	if err := s.audit.Write(ctx, id); err != nil {
		return err
	}
	return s.bus.Publish(ctx, id)
}
