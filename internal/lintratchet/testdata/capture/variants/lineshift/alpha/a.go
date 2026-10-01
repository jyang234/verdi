// Package alpha holds the ratchet fixture's findings.
package alpha

import (
	"context"
	"errors"
	"fmt"
)

// These three comment lines move every finding below them
// down three lines; a key holds no line number, so the
// findings keep their keys.

// Counter is package-level mutable state.
var Counter int

// Op formats fresh's error with %v instead of wrapping it.
func Op(ctx context.Context) error {
	_ = ctx
	return fmt.Errorf("op: %v", fresh())
}

// fresh returns an error.
func fresh() error { return errors.New("fresh") }
