// Package sample is a tiny, dependency-free fixture module (no imports
// beyond the standard testing package) that
// TestRealNamedGoTestRunner_ExecPath (cmd/verdi/testproducer_integration_test.go)
// runs the REAL go toolchain against, proving realNamedGoTestRunner's exec
// path end to end: one passing test, one failing test, and one skipped
// test, exercising all three real outcomes readNamedTestOutcomes maps to
// an evidence verdict (contract 5). This module is intentionally excluded
// from the parent verdi module's own build/test/vet (its own go.mod, and
// everything under a directory named testdata/ is skipped by `go`'s "./..."
// pattern regardless).
package sample

import "testing"

// TestPass always passes.
func TestPass(t *testing.T) {}

// TestFail always fails — its own package, run only by the one hermetic
// integration test that deliberately invokes it, never by this module's
// own `make verify` (testdata/ is never part of "./...").
func TestFail(t *testing.T) {
	t.Fatal("intentional failure for the go-test producer's real-exec integration test")
}

// TestSkip always skips.
func TestSkip(t *testing.T) {
	t.Skip("intentional skip for the go-test producer's real-exec integration test")
}
