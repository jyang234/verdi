// Package gitx is a synthetic stand-in for internal/gitx: two mutating
// primitives, one read-only primitive, one mutating primitive no entry
// reaches, and the git-directory locator the taint detector sources from.
package gitx

import "context"

// Mutate is the synthetic mutating primitive.
func Mutate(ctx context.Context, dir string) error { return nil }

// Publish is the second mutating primitive: only the workbench API's
// actions reach it, so a test can tell an action's work from its route's.
func Publish(ctx context.Context, dir string) error { return nil }

// Prune is mutating; only a value-only package-level variable names it.
func Prune(ctx context.Context) error { return nil }

// Read is the synthetic read-only primitive.
func Read(ctx context.Context, dir string) (string, error) { return dir, nil }

// Other is mutating, and no entry reaches it.
func Other(ctx context.Context) error { return nil }

// CommonDir stands in for gitx.CommonDir: it returns the git directory.
func CommonDir(ctx context.Context, dir string) (string, error) { return dir, nil }

// Location carries an exported method, so the export census sees methods.
type Location struct{ Dir string }

// String is an exported method on an exported type.
func (l Location) String() string { return l.Dir }

type hidden struct{}

// Visible is exported but its receiver type is not: a caller holding a
// hidden value can still call it.
func (hidden) Visible() {}

// StageAll is an exported variable of function type: callable API that is
// no declared function.
var StageAll = func(ctx context.Context, dir string) error { return Mutate(ctx, dir) }

// Stager is an exported interface.
type Stager interface {
	Stage(ctx context.Context) error
}

type stager struct{}

// Stage is an exported method of an unexported type.
func (stager) Stage(ctx context.Context) error { return Mutate(ctx, ".") }

// NewStager returns the unexported implementation behind the interface.
func NewStager() Stager { return stager{} }

// Loc is an exported alias of a type this package declares: its methods
// are Location's.
type Loc = Location
