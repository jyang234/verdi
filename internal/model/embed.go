package model

import (
	"bytes"
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

// CanonicalYAML returns a defensive copy of the embedded canonical.yaml
// bytes verbatim (spec/init-wizard ac-2/ac-3, ledger L-N5, design doc §12
// W-4): the one raw-bytes source a hand-built model.yaml derives from.
// There is no Model→YAML marshal seam anywhere in this codebase (the
// module-wide "hand-render, strict-decode-verify" posture
// internal/workbench/obligationauthor.go's renderObligation already
// documents); a caller that needs to WRITE a model.yaml describing the
// canonical shape plus a vocabulary: block (internal/initwizard) starts
// from these bytes and appends to them, rather than re-deriving the
// classes:/lifecycle: block a third time (canonical.go's Go-literal twin
// is already the second). A copy, never the shared embedded slice
// itself — mirroring Canonical()'s own "never a shared, cached pointer"
// promise one level down, so a caller mutating (or appending to) its own
// copy can never corrupt what a later call, in this process or another
// goroutine, reads.
func CanonicalYAML() []byte {
	return bytes.Clone(canonicalYAML)
}
