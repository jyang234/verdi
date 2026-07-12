package splice

import (
	"bytes"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

// sampleSpecWithStubs is sampleSpec (ops_test.go) plus an existing stubs:
// block in the house block-style-of-flow-maps shape, so AppendStub/
// AppendSpikeStub's "existing block gains a line" case has a fixture to
// append to that is otherwise byte-identical to sampleSpec.
const sampleSpecWithStubs = `---
id: spec/sample-flow
kind: spec
class: feature
title: "Sample flow (splice fixture)"
status: draft
owners: [platform-team]
problem: { text: "a borrower whose document was rejected: has no way to resubmit", anchor: "#problem" }
outcome: { text: "\"resubmitted\" documents route straight back to review", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can resubmit a rejected document", evidence: [attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "the reviewer sees: resubmission history for the document", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: plain unquoted text stays readable, evidence: [static, attestation], anchor: "#ac-3" }
constraints:
  - { id: co-1, text: "retries are capped at: 3 attempts", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "excuse this flow from the outbox rule", anchor: "#dc-1",
      links: [ { type: exempts, ref: adr/0001-outbox-events, note: "async by design" } ] }
  - { id: dc-2, text: "use the existing notification channel", anchor: "#dc-2" }
  - { id: dc-3, text: "decision with an empty links list", anchor: "#dc-3", links: [] }
stubs:
  - { slug: existing-stub, acceptance_criteria: [ac-1] }
---
# Sample flow

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.

## ac-2

Prose.

## ac-3

Prose.

## co-1

Prose.

## dc-1

Prose.

## dc-2

Prose.

## dc-3

Prose.
`

// TestAppendStub covers both frontmatter shapes: an existing stubs: block
// (gains a line) and the first-stub case (key absent — the same
// first-yarn insertion pattern AppendDecisionLink and AppendObject prove).
func TestAppendStub(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		slug   string
		acIDs  []string
		wantFM string
	}{
		{"existing stubs block gains a line", sampleSpecWithStubs, "new-stub", []string{"ac-2", "ac-3"},
			"\n  - { slug: new-stub, acceptance_criteria: [ac-2, ac-3] }"},
		{"absent stubs block created before the closing delimiter", sampleSpec, "new-stub", []string{"ac-1"},
			"stubs:\n  - { slug: new-stub, acceptance_criteria: [ac-1] }\n---"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := mustParse(t, tc.src)
			edit, err := d.AppendStub(tc.slug, tc.acIDs)
			if err != nil {
				t.Fatalf("AppendStub: %v", err)
			}
			out := applyAndValidate(t, d, edit)
			if !strings.Contains(string(out), tc.wantFM) {
				t.Fatalf("output missing %q\n%s", tc.wantFM, out)
			}

			fm, _, _ := artifact.SplitFrontmatter(out)
			spec, err := artifact.DecodeSpec(fm)
			if err != nil {
				t.Fatalf("re-decode: %v", err)
			}
			var found bool
			for _, st := range spec.Stubs {
				if st.Slug != tc.slug {
					continue
				}
				found = true
				if st.Spike {
					t.Fatalf("stub %s: Spike = true, want false", tc.slug)
				}
				if len(st.AcceptanceCriteria) != len(tc.acIDs) {
					t.Fatalf("stub %s: AcceptanceCriteria = %v, want %v", tc.slug, st.AcceptanceCriteria, tc.acIDs)
				}
			}
			if !found {
				t.Fatalf("spec does not carry the appended stub %s after re-decode", tc.slug)
			}

			for _, id := range untouchedObjects {
				if !bytes.Equal(objectSpanBytes(t, []byte(tc.src), id), objectSpanBytes(t, out, id)) {
					t.Errorf("untouched object %s changed", id)
				}
			}
		})
	}
}

func TestAppendStub_Negative(t *testing.T) {
	d := mustParse(t, sampleSpec)
	if _, err := d.AppendStub("", []string{"ac-1"}); err == nil {
		t.Fatal("AppendStub with empty slug succeeded, want error")
	}
	if _, err := d.AppendStub("new-stub", nil); err == nil {
		t.Fatal("AppendStub with no acceptance criteria succeeded, want error")
	}
}

// TestAppendSpikeStub covers the same two frontmatter shapes as
// TestAppendStub, proving the spike: true / resolves: shape renders and
// round-trips correctly (DC-4's flag-discriminated sibling).
func TestAppendSpikeStub(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		slug   string
		oqIDs  []string
		wantFM string
	}{
		{"existing stubs block gains a line", sampleSpecWithStubs, "retry-strategy-spike", []string{"oq-1", "oq-2"},
			"\n  - { slug: retry-strategy-spike, spike: true, resolves: [oq-1, oq-2] }"},
		{"absent stubs block created before the closing delimiter", sampleSpec, "retry-strategy-spike", []string{"oq-1"},
			"stubs:\n  - { slug: retry-strategy-spike, spike: true, resolves: [oq-1] }\n---"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := mustParse(t, tc.src)
			edit, err := d.AppendSpikeStub(tc.slug, tc.oqIDs)
			if err != nil {
				t.Fatalf("AppendSpikeStub: %v", err)
			}
			out := applyAndValidate(t, d, edit)
			if !strings.Contains(string(out), tc.wantFM) {
				t.Fatalf("output missing %q\n%s", tc.wantFM, out)
			}

			fm, _, _ := artifact.SplitFrontmatter(out)
			spec, err := artifact.DecodeSpec(fm)
			if err != nil {
				t.Fatalf("re-decode: %v", err)
			}
			var found bool
			for _, st := range spec.Stubs {
				if st.Slug != tc.slug {
					continue
				}
				found = true
				if !st.Spike {
					t.Fatalf("stub %s: Spike = false, want true", tc.slug)
				}
				if len(st.AcceptanceCriteria) != 0 {
					t.Fatalf("stub %s: AcceptanceCriteria = %v, want empty", tc.slug, st.AcceptanceCriteria)
				}
				if len(st.Resolves) != len(tc.oqIDs) {
					t.Fatalf("stub %s: Resolves = %v, want %v", tc.slug, st.Resolves, tc.oqIDs)
				}
			}
			if !found {
				t.Fatalf("spec does not carry the appended spike stub %s after re-decode", tc.slug)
			}

			for _, id := range untouchedObjects {
				if !bytes.Equal(objectSpanBytes(t, []byte(tc.src), id), objectSpanBytes(t, out, id)) {
					t.Errorf("untouched object %s changed", id)
				}
			}
		})
	}
}

func TestAppendSpikeStub_Negative(t *testing.T) {
	d := mustParse(t, sampleSpec)
	if _, err := d.AppendSpikeStub("", []string{"oq-1"}); err == nil {
		t.Fatal("AppendSpikeStub with empty slug succeeded, want error")
	}
	if _, err := d.AppendSpikeStub("retry-strategy-spike", nil); err == nil {
		t.Fatal("AppendSpikeStub with no resolves ids succeeded, want error")
	}
}

// TestAppendStub_ThenAppendSpikeStub_BothLandInOneBlock proves the two
// stub kinds share the one stubs: list — no parallel spike_stubs: block —
// by appending one of each to an already-populated stubs: block.
func TestAppendStub_ThenAppendSpikeStub_BothLandInOneBlock(t *testing.T) {
	d := mustParse(t, sampleSpecWithStubs)
	e1, err := d.AppendStub("plain-two", []string{"ac-2"})
	if err != nil {
		t.Fatalf("AppendStub: %v", err)
	}
	out1, err := d.Apply([]Edit{e1})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	d2, err := Parse(out1)
	if err != nil {
		t.Fatalf("re-Parse: %v", err)
	}
	e2, err := d2.AppendSpikeStub("spike-one", []string{"oq-1"})
	if err != nil {
		t.Fatalf("AppendSpikeStub: %v", err)
	}
	out2 := applyAndValidate(t, d2, e2)

	fm, _, _ := artifact.SplitFrontmatter(out2)
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if len(spec.Stubs) != 3 {
		t.Fatalf("len(Stubs) = %d, want 3 (existing-stub, plain-two, spike-one)", len(spec.Stubs))
	}
}
