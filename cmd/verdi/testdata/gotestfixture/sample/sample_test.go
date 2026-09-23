// Package sample is a tiny, dependency-free fixture module (no imports
// beyond the standard testing package) that the go-test producer's
// integration tests (cmd/verdi/testproducer_integration_test.go) run the
// REAL go toolchain against: one test per outcome shape the producer must
// read from a real `go test -json` stream. This module is intentionally
// excluded from the parent verdi module's own build/test/vet (its own
// go.mod, and everything under a directory named testdata/ is skipped by
// `go`'s "./..." pattern regardless).
package sample

import "testing"

// TestPass always passes.
func TestPass(t *testing.T) {}

// TestFail always fails.
func TestFail(t *testing.T) {
	t.Fatal("intentional failure for the go-test producer's real-exec integration test")
}

// TestSkip always skips.
func TestSkip(t *testing.T) {
	t.Skip("intentional skip for the go-test producer's real-exec integration test")
}

// TestAttr passes after emitting a Go 1.25 test attribute, which
// `go test -json` reports as an "attr" event carrying Key and Value.
func TestAttr(t *testing.T) {
	t.Attr("fixture", "attr")
}

// TestParentFails fails after its only subtest passed: the parent's own
// terminal event, not the subtest's, decides its outcome.
func TestParentFails(t *testing.T) {
	t.Run("sub", func(t *testing.T) {})
	t.Fatal("intentional parent failure after a passing subtest")
}

// TestPrefixBar passes; no test named TestPrefix exists, so a producer ref
// naming TestPrefix must never read this test's outcome.
func TestPrefixBar(t *testing.T) {}
