package commitdesign

import (
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designscaffold"
)

// TestScaffoldSpec_BytePin pins scaffoldSpec's exact output bytes for a
// slug fixture — the `want` literal below is UNCHANGED from before the
// producer switch (spec/creation-form ac-4's parity pin): with no store
// override present, the embedded commit-to-design canonical template
// (designscaffold templates/commitdesign.md) must reproduce the retired
// strings.Builder output byte-for-byte for every input the old producer
// handled — pins present, dispositions present (their absence legs ride
// the package's existing Run fixtures, which exercise empty boards).
// History: the pin originally proved the titleCase -> HumanizeName swap
// (spec/shared-homes ac-5); it now also proves the L-M12 switch changed
// nothing for a store without an override.
func TestScaffoldSpec_BytePin(t *testing.T) {
	pins := []artifact.Pin{
		{Ref: "adr/0001-outbox", X: 10, Y: 20},
		{Ref: "spec/other-thing", X: 30, Y: 40},
	}
	dispositions := []artifact.Disposition{
		{Sticky: "sticky-1", Disposition: artifact.DispositionIncorporated, Note: "captured"},
	}

	tmpl, err := designscaffold.Canonical("commitdesign.md")
	if err != nil {
		t.Fatalf("Canonical(commitdesign.md): %v", err)
	}
	got, err := scaffoldSpec(tmpl, "spec/code-health-two", "jira:VERDI-1", "code-health-two", pins, dispositions)
	if err != nil {
		t.Fatalf("scaffoldSpec: %v", err)
	}

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
