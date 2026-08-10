package experimentdecision

import "fmt"

// errPrefix is the fixed prefix every error this package returns carries
// (CLAUDE.md's error-wrapping convention, mirrored from internal/
// experiment's own "experiment: " prefix).
const errPrefix = "experimentdecision: "

// errf formats an operational error with the package's fixed prefix.
func errf(format string, args ...interface{}) error {
	return fmt.Errorf(errPrefix+format, args...)
}

// errfWrap formats an operational error with the package's fixed prefix,
// wrapping cause with %w so errors.Is/errors.As reach through it.
func errfWrap(msg string, cause error) error {
	return fmt.Errorf(errPrefix+"%s: %w", msg, cause)
}
