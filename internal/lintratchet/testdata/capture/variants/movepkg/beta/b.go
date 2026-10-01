// Package beta holds no finding unless a variant moves one here.
package beta

// Name returns the package's name.
func Name() string { return "beta" }

// Counter is package-level mutable state.
var Counter int
