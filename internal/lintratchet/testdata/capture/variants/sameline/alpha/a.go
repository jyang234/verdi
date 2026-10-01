// Package alpha holds the ratchet fixture's findings.
package alpha

import (
	"context"
	"errors"
	"fmt"
)

// Counter is package-level mutable state.
var Counter int

// Op formats fresh's error with %v instead of wrapping it.
func Op(ctx context.Context) error {
	_ = ctx
	return fmt.Errorf("op: %v", fresh())
}

// fresh starts from a new context instead of taking one.
func fresh() error { return wait(context.Background()) }

func wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("fresh")
}
