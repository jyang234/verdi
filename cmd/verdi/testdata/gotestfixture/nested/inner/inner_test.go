// Package inner lives in its own module (nested/go.mod): a producer ref
// naming nested/inner points into another module, not the fixture module
// root the ref's package path is relative to.
package inner

import "testing"

func TestInner(t *testing.T) {}
