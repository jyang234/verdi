// Package e declares a generic type that implements a module interface
// only with a method promoted from an embedded field: its own methods
// alone do not cover the interface, its method set does.
package e

import "io"

// Stager is a module interface.
type Stager interface {
	Stage() error
	Close() error
}

type gen[T any] struct{ io.Closer }

// Stage is the generic type's own method; Close is promoted.
func (gen[T]) Stage() error { return nil }

// Use dispatches through the interface.
func Use(s Stager) error { return s.Stage() }

// New returns a gen behind the interface.
func New() Stager { return gen[int]{} }
