package showcasealign

import "testing"

// TestShowcaseLintClean proves `verdi lint` exits 0 against a freshly
// provisioned showcase store. cmd/verdi/lint.go's runLintVerb only flips
// the exit code to 1 when a finding's severity is above
// lint.SeverityDisclosure, so this assertion permits VL-017 disclosure
// notices (a bare-clone-shaped store's disclosed-unproven open-question
// check) without treating them as failure — consistent with
// internal/lint's own TestClean_CorpusLintsGreen /
// TestV2FixtureCorpus_LintsClean, which this store's construction
// mirrors (see helpers_test.go's provisionShowcaseStore doc comment).
//
// docs/design/plans/2026-07-14-public-rollout-plan.md's Phase 1 (story
// showcase-corpus-renovation) already made examples/showcase lint-clean
// by content; 08-revision-notes.md's "Public rollout — showcase ac-1
// evidence" entry records that evidence. This test is the MECHANICAL
// re-proof spec/public-showcase ac-2 (the drift gate, story
// showcase-drift-gate) requires going forward — a failure here is a real
// content regression in examples/showcase, never something to fix by
// loosening this assertion.
func TestShowcaseLintClean(t *testing.T) {
	store := provisionShowcaseStore(t)
	stdout, stderr, code := runBinary(t, store, "lint")
	if code != 0 {
		t.Fatalf("showcase store must lint clean; exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}
