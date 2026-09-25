package main

import "testing"

// TestProbeMergeGateRed fails on purpose. It lives only on the throwaway
// branch probe/merge-gate-red, which proves that a failing gate job turns
// the required merge-gate check red rather than skipped. Never merge it.
func TestProbeMergeGateRed(t *testing.T) {
	t.Fatal("deliberate failure: merge-gate aggregator probe")
}
