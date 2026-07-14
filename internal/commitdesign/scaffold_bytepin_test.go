package commitdesign

import (
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestScaffoldSpec_BytePin pins scaffoldSpec's exact output bytes for a
// slug fixture (spec/shared-homes ac-5: commitdesign's titleCase is
// deleted for designscaffold.HumanizeName). Slug-constrained inputs
// (kebab-case spec names, artifact.ValidateName's own charset) make the
// two implementations' outputs identical — titleCase's byte-indexed
// upper-casing and HumanizeName's rune-safe upper-casing agree on every
// ASCII-only slug, which is all a spec name can ever be. This test proves
// the swap leaves scaffoldSpec's output byte-for-byte unchanged.
func TestScaffoldSpec_BytePin(t *testing.T) {
	pins := []artifact.Pin{
		{Ref: "adr/0001-outbox", X: 10, Y: 20},
		{Ref: "spec/other-thing", X: 30, Y: 40},
	}
	dispositions := []artifact.Disposition{
		{Sticky: "sticky-1", Disposition: artifact.DispositionIncorporated, Note: "captured"},
	}

	got := scaffoldSpec("spec/code-health-two", "jira:VERDI-1", "code-health-two", pins, dispositions)

	const want = `---
id: spec/code-health-two
kind: spec
title: "Code Health Two"
owners: [unassigned]
class: feature
status: draft
story: jira:VERDI-1
context:
  - adr/0001-outbox
  - spec/other-thing
acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static] }
dispositions:
  - { sticky: sticky-1, disposition: incorporated }
---
# Code Health Two

TODO: design notes.

Drafted by commit-to-design from board "spec/code-health-two". Every board sticky above is
carried as ` + "`open-question`" + ` until the commit-to-design skill (or a human)
promotes it to ` + "`incorporated`" + ` or ` + "`contradicted`" + ` (I-5).
`

	if got != want {
		t.Fatalf("scaffoldSpec output byte-pin mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
