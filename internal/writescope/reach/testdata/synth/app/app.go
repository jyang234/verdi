// Package app holds one function per reachability shape the analysis must
// resolve: a direct call, an interface call, a closure, a method value, a
// function value, a package-level variable holding a function, a standard
// library callback, a read-only path, and an unreached mutation.
package app

import (
	"context"
	"net/http"

	"example.com/synth/gitx"
)

// Direct calls the mutating primitive directly.
func Direct(ctx context.Context) error { return gitx.Mutate(ctx, ".") }

// Doer is a consumer-defined interface.
type Doer interface {
	Do(ctx context.Context) error
}

type mutator struct{}

// Do mutates.
func (mutator) Do(ctx context.Context) error { return gitx.Mutate(ctx, ".") }

type reader struct{}

// Do only reads.
func (*reader) Do(ctx context.Context) error {
	_, err := gitx.Read(ctx, ".")
	return err
}

// ViaInterface reaches the mutation only through an interface call.
func ViaInterface(ctx context.Context, d Doer) error { return d.Do(ctx) }

// ViaClosure reaches the mutation only through a closure.
func ViaClosure(ctx context.Context) error {
	f := func() error { return gitx.Mutate(ctx, ".") }
	return f()
}

// ViaMethodValue reaches the mutation only through a method value.
func ViaMethodValue(ctx context.Context) error {
	g := mutator{}.Do
	return g(ctx)
}

// ViaFuncValue reaches the mutation only through a function value.
func ViaFuncValue(ctx context.Context) error {
	h := gitx.Mutate
	return h(ctx, ".")
}

var hook = gitx.Mutate

// ViaPackageVar reaches the mutation only through a package-level variable.
func ViaPackageVar(ctx context.Context) error { return hook(ctx, ".") }

// ReadOnly reaches only the read-only primitive.
func ReadOnly(ctx context.Context) error {
	_, err := gitx.Read(ctx, ".")
	return err
}

// Publish reaches the second mutating primitive.
func Publish(ctx context.Context) error { return gitx.Publish(ctx, ".") }

// Unreached is the only caller of gitx.Other, and no entry reaches it.
func Unreached(ctx context.Context) error { return gitx.Other(ctx) }

type handler struct{}

// ServeHTTP is called by the standard library, never by module code.
func (handler) ServeHTTP(w http.ResponseWriter, r *http.Request) { _ = gitx.Mutate(r.Context(), ".") }

// NewHandler converts a module type to a standard-library interface.
func NewHandler() http.Handler { return handler{} }
