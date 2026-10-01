// Package g declares a generic type whose methods implement a module
// interface. Interface dispatch to a generic type is not modeled, so a
// graph built over this package must refuse rather than miss the call.
package g

// Sizer is a module interface.
type Sizer interface {
	Size() int
}

type box[T any] struct{ items []T }

// Size makes every box a Sizer.
func (b box[T]) Size() int { return len(b.items) }

// New returns a box behind the interface.
func New() Sizer { return box[int]{} }

// Use dispatches through the interface.
func Use(s Sizer) int { return s.Size() }
