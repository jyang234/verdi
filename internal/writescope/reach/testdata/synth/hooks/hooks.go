// Package hooks registers a mutating hook at initialization, in a package
// variable a verb's arm later runs: the mutation is the init function's
// (it names the hook), never the arm's.
package hooks

import (
	"context"

	"example.com/synth/gitx"
)

// registered holds the hooks init registers.
var registered []func() error

func init() {
	registered = append(registered, func() error { return gitx.Publish(context.Background(), ".") })
}

// All returns the registered hooks.
func All() []func() error { return registered }
