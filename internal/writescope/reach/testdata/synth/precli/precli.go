// Package precli is a synthetic CLI binary whose code outside its verbs'
// arms mutates: its dispatcher's preamble, a package-level variable whose
// initializer calls a mutating function, and (through package hooks) an
// init function. A package-level variable that only names a mutating
// function runs nothing at initialization.
package precli

import (
	"context"

	"example.com/synth/gitx"
	"example.com/synth/hooks"
)

// booted runs a mutation while the package initializes, before any verb.
var booted = boot()

// later only names a mutating primitive; nothing runs it at initialization.
var later = gitx.Prune

func boot() error { return gitx.Other(context.Background()) }

// Run mutates before it dispatches, then dispatches two verbs: lint runs
// the hooks package hooks registered, later runs the named primitive.
func Run(args []string) int {
	_ = gitx.Mutate(context.Background(), ".")
	if len(args) == 0 {
		return 2
	}
	if args[0] == "lint" {
		for _, h := range hooks.All() {
			_ = h()
		}
		return 0
	}
	if args[0] == "later" {
		_ = later(context.Background())
		return 0
	}
	if booted != nil {
		return 1
	}
	return 2
}
