package policyartifact

import (
	"fmt"

	"github.com/jyang234/verdi/internal/canonjson"
)

// checkSealed proves a decoded value was produced by this package's own
// Decode* seam and has not been modified since (the SI-21 forgery
// posture applied to constitution authority): seal is the unexported
// canonical-content digest minted at decode time, and recompute is the
// value's current canonical-content digest. Both failures are
// operational errors — a value outside decode provenance never yields a
// digest and never enters effective-policy resolution.
func checkSealed(what, id, seal string, v interface{}) error {
	if seal == "" {
		return fmt.Errorf("policyartifact: %s %q was not produced by its Decode function", what, id)
	}
	d, err := canonjson.Digest(v)
	if err != nil {
		return err
	}
	if d != seal {
		return fmt.Errorf("policyartifact: %s %q was modified after decode", what, id)
	}
	return nil
}
