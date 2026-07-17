package model

import (
	_ "embed"
	"fmt"
)

//go:embed canonical.yaml
var canonicalYAML []byte

// Canonical returns the embedded default model (internal/model/
// canonical.yaml, go:embed — precedent internal/dex/assets.go): what
// store.Open (internal/store/open.go) resolves an ABSENT .verdi/
// model.yaml to, so a store with no manifest at all changes nothing
// about how it behaves today (the load-bearing parity claim this
// phase's own exit gate and every sibling stub depend on).
//
// A fresh Model is decoded on every call — never a shared, cached
// pointer — so no caller can mutate a process-wide singleton out from
// under another. Decoding routes through the same DecodeModel entry
// point every hand-written model.yaml uses, including its own
// checkFrontier pass against canonicalModel (canonical.go): Canonical()
// therefore re-proves, on every call, that the embedded asset still
// agrees with the Go literal it must never drift from — not only in
// TestCanonicalYAMLMatchesGoLiteral (embed_test.go), but at runtime.
//
// Canonical() carries no error return (the plan's fixed signature)
// because a decode failure here is a packaging defect, never a
// user-facing condition — embed_test.go proves it unreachable, so a
// panic (never a silently swallowed failure) is the honest response if
// it somehow occurs anyway.
func Canonical() *Model {
	m, err := DecodeModel(canonicalYAML)
	if err != nil {
		panic(fmt.Sprintf("model: embedded canonical.yaml failed to decode: %v (packaging defect, not a user-facing condition — see embed_test.go)", err))
	}
	return m
}
