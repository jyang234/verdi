package strictfixture

import "fmt"

// Wrap formats an error with %v instead of wrapping it: errorlint's one
// finding.
func Wrap(err error) error {
	return fmt.Errorf("wrapped: %v", err)
}
