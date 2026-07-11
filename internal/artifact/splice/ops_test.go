package splice

import (
	"bytes"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

// sampleSpec mirrors spike S7's deliberately YAML-hostile sample: quoted
// values containing ": " (problem, ac-2), values starting with a literal
// '"' (outcome), a multi-entry AC list, a decision with an existing
// exempts link (dc-1), a plain decision with no links at all (dc-2), and
// a decision with an EMPTY links list (dc-3) for the S7-disclosed
// first-yarn insertion case.
const sampleSpec = `---
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

// untouchedObjects are the ids whose byte spans must be identical before
// and after any edit that does not name them.
var untouchedObjects = []string{"ac-1", "ac-3", "co-1", "dc-1"}

// objectSpanBytes re-locates an object's full element span in a buffer by
// the same node-position technique the splicer uses.
func objectSpanBytes(t *testing.T, buf []byte, id string) []byte {
	t.Helper()
	d, err := Parse(buf)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	elem, err := d.objectElem(id)
	if err != nil {
		t.Fatalf("objectElem(%s): %v", id, err)
	}
	start, end, err := d.span(elem)
	if err != nil {
		t.Fatalf("span(%s): %v", id, err)
	}
	return buf[start:end]
}

func mustParse(t *testing.T, src string) *Doc {
	t.Helper()
	d, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return d
}

func applyAndValidate(t *testing.T, d *Doc, edits ...Edit) []byte {
	t.Helper()
	out, err := d.Apply(edits)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := Validate(out); err != nil {
		t.Fatalf("Validate after splice: %v", err)
	}
	return out
}

// changedLines diffs two buffers line-wise and returns how many lines
// differ (counting adds/removes as changes).
func changedLines(a, b []byte) int {
	al := strings.Split(string(a), "\n")
	bl := strings.Split(string(b), "\n")
	n := 0
	max := len(al)
	if len(bl) > max {
		max = len(bl)
	}
	for i := 0; i < max; i++ {
		var av, bv string
		if i < len(al) {
			av = al[i]
		}
		if i < len(bl) {
			bv = bl[i]
		}
		if av != bv {
			n++
		}
	}
	return n
}

// TestSetObjectText_ByteStability is the S7 §2 proof re-established as a
// permanent test: a one-field edit changes exactly one line, the byte
// delta is exactly the inserted characters, and every untouched object's
// span is byte-identical.
func TestSetObjectText_ByteStability(t *testing.T) {
	d := mustParse(t, sampleSpec)
	before := []byte(sampleSpec)

	edit, err := d.SetObjectText("ac-2", "the reviewer sees: resubmission history for the document, oldest first")
	if err != nil {
		t.Fatalf("SetObjectText: %v", err)
	}
	out := applyAndValidate(t, d, edit)

	if got := changedLines(before, out); got != 1 {
		t.Fatalf("changed lines = %d, want exactly 1", got)
	}
	for _, id := range untouchedObjects {
		if !bytes.Equal(objectSpanBytes(t, before, id), objectSpanBytes(t, out, id)) {
			t.Errorf("untouched object %s is not byte-identical after the edit", id)
		}
	}
	// The body was never in the edit path.
	_, bodyBefore, _ := artifact.SplitFrontmatter(before)
	_, bodyAfter, _ := artifact.SplitFrontmatter(out)
	if !bytes.Equal(bodyBefore, bodyAfter) {
		t.Errorf("document body changed on a frontmatter-only edit")
	}
	if !strings.Contains(string(out), `text: "the reviewer sees: resubmission history for the document, oldest first"`) {
		t.Errorf("edited text not found in output")
	}
}

func TestSetObjectText_Cases(t *testing.T) {
	tests := []struct {
		name string
		id   string
		text string
		want string
	}{
		{"colon-hostile quoted value", "co-1", "retries are capped at: 5 attempts", `text: "retries are capped at: 5 attempts"`},
		{"plain unquoted scalar gets quoted on write", "ac-3", "plain text, now edited", `text: "plain text, now edited"`},
		{"leading-quote value", "ac-1", `"resubmitted" is a state, not a verb`, `text: "\"resubmitted\" is a state, not a verb"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := mustParse(t, sampleSpec)
			edit, err := d.SetObjectText(tc.id, tc.text)
			if err != nil {
				t.Fatalf("SetObjectText: %v", err)
			}
			out := applyAndValidate(t, d, edit)
			if !strings.Contains(string(out), tc.want) {
				t.Fatalf("output missing %q\n%s", tc.want, out)
			}
		})
	}
}

func TestSetObjectText_Negative(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"unknown id", "ac-99"},
		{"unknown prefix", "zz-1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := mustParse(t, sampleSpec)
			if _, err := d.SetObjectText(tc.id, "x"); err == nil {
				t.Fatalf("SetObjectText(%s) succeeded, want error", tc.id)
			}
		})
	}
}

// TestAppendDecisionLink covers all three insertion shapes, including the
// two S7 disclosed as unproven: no links key at all, and an empty list.
func TestAppendDecisionLink(t *testing.T) {
	link := artifact.Link{Type: artifact.LinkExempts, Ref: "adr/0001-outbox-events", Note: "board-drawn"}
	tests := []struct {
		name string
		dc   string
		want string
	}{
		{"append after last element (S7-proven)", "dc-1",
			`note: "async by design" }, { type: exempts, ref: "adr/0001-outbox-events", note: "board-drawn" } ]`},
		{"no links key at all (first yarn)", "dc-2",
			`anchor: "#dc-2", links: [ { type: exempts, ref: "adr/0001-outbox-events", note: "board-drawn" } ] }`},
		{"empty links list (S7 disclosed case)", "dc-3",
			`links: [ { type: exempts, ref: "adr/0001-outbox-events", note: "board-drawn" } ] }`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := mustParse(t, sampleSpec)
			edit, err := d.AppendDecisionLink(tc.dc, link)
			if err != nil {
				t.Fatalf("AppendDecisionLink: %v", err)
			}
			out := applyAndValidate(t, d, edit)
			if !strings.Contains(string(out), tc.want) {
				t.Fatalf("output missing %q\n%s", tc.want, out)
			}
			// The re-decoded spec really carries the new edge.
			fm, _, _ := artifact.SplitFrontmatter(out)
			spec, err := artifact.DecodeSpec(fm)
			if err != nil {
				t.Fatalf("re-decode: %v", err)
			}
			var found bool
			for _, dec := range spec.Decisions {
				if dec.ID != tc.dc {
					continue
				}
				for _, l := range dec.Links {
					if l.Type == artifact.LinkExempts && l.Ref == link.Ref && l.Note == link.Note {
						found = true
					}
				}
			}
			if !found {
				t.Fatalf("decision %s does not carry the appended edge after re-decode", tc.dc)
			}
			// Untouched objects stay byte-identical.
			for _, id := range untouchedObjects {
				if id == tc.dc {
					continue
				}
				if !bytes.Equal(objectSpanBytes(t, []byte(sampleSpec), id), objectSpanBytes(t, out, id)) {
					t.Errorf("untouched object %s changed", id)
				}
			}
		})
	}
}

func TestAppendDecisionLink_Negative(t *testing.T) {
	d := mustParse(t, sampleSpec)
	if _, err := d.AppendDecisionLink("ac-1", artifact.Link{Type: artifact.LinkExempts, Ref: "adr/x"}); err == nil {
		t.Fatal("AppendDecisionLink on an AC succeeded, want error")
	}
	if _, err := d.AppendDecisionLink("dc-99", artifact.Link{Type: artifact.LinkExempts, Ref: "adr/x"}); err == nil {
		t.Fatal("AppendDecisionLink on a missing decision succeeded, want error")
	}
}

// TestAppendObject covers both frontmatter shapes (existing block-style
// block; absent block inserted before the closing delimiter) and proves
// the appended body heading resolves the new object's anchor.
func TestAppendObject(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		text   string
		ev     []artifact.EvidenceKind
		wantFM string
	}{
		{"existing block gains a line", "co-2", "notices localize per region", nil,
			"\n  - { id: co-2, text: \"notices localize per region\", anchor: \"#co-2\" }"},
		{"absent block is created before the closing delimiter", "oq-1", "what about partial refunds?", nil,
			"open_questions:\n  - { id: oq-1, text: \"what about partial refunds?\", anchor: \"#oq-1\" }\n---"},
		{"acceptance criterion carries evidence", "ac-4", "audits are queryable", []artifact.EvidenceKind{artifact.EvidenceAttestation},
			`{ id: ac-4, text: "audits are queryable", evidence: [attestation], anchor: "#ac-4" }`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := mustParse(t, sampleSpec)
			edits, err := d.AppendObject(tc.id, tc.text, tc.ev)
			if err != nil {
				t.Fatalf("AppendObject: %v", err)
			}
			out := applyAndValidate(t, d, edits...)
			if !strings.Contains(string(out), tc.wantFM) {
				t.Fatalf("output missing %q\n%s", tc.wantFM, out)
			}
			if !strings.Contains(string(out), "\n## "+tc.id+"\n") {
				t.Fatalf("body heading for %s missing", tc.id)
			}
			for _, id := range untouchedObjects {
				if !bytes.Equal(objectSpanBytes(t, []byte(sampleSpec), id), objectSpanBytes(t, out, id)) {
					t.Errorf("untouched object %s changed", id)
				}
			}
		})
	}
}

func TestAppendObject_NewACWithoutEvidenceFails(t *testing.T) {
	d := mustParse(t, sampleSpec)
	if _, err := d.AppendObject("ac-4", "x", nil); err == nil {
		t.Fatal("AppendObject(ac without evidence) succeeded, want error")
	}
}

// TestApply_BatchTailToHead proves two edits computed against one parse
// apply cleanly in one batch (S7: "batch same-write edits and apply
// tail-to-head").
func TestApply_BatchTailToHead(t *testing.T) {
	d := mustParse(t, sampleSpec)
	e1, err := d.SetObjectText("ac-1", "edited first")
	if err != nil {
		t.Fatalf("SetObjectText: %v", err)
	}
	e2, err := d.AppendDecisionLink("dc-2", artifact.Link{Type: artifact.LinkSupersedes, Ref: "adr/0001-outbox-events"})
	if err != nil {
		t.Fatalf("AppendDecisionLink: %v", err)
	}
	out := applyAndValidate(t, d, e1, e2)
	if !strings.Contains(string(out), `text: "edited first"`) {
		t.Fatal("first edit missing")
	}
	if !strings.Contains(string(out), `links: [ { type: supersedes, ref: "adr/0001-outbox-events" } ]`) {
		t.Fatal("second edit missing")
	}
}

// TestValidate_RejectsInvalidSplice: validate-before-write refuses a
// result that no longer decodes (S7 §5 — the invalid intermediate state
// must never reach the working tree).
func TestValidate_RejectsInvalidSplice(t *testing.T) {
	broken := strings.Replace(sampleSpec, `{ id: co-1,`, `{ id co-1,`, 1)
	if err := Validate([]byte(broken)); err == nil {
		t.Fatal("Validate accepted a non-decoding buffer")
	}
	// An anchor that no longer resolves is also a validation failure.
	noHeading := strings.Replace(sampleSpec, "## co-1", "## co-one", 1)
	if err := Validate([]byte(noHeading)); err == nil {
		t.Fatal("Validate accepted a dangling anchor")
	}
}

func TestParse_Negative(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"no opening delimiter", "id: x\n"},
		{"no closing delimiter", "---\nid: x\n"},
		{"frontmatter not a mapping", "---\n- a\n- b\n---\nbody\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.src)); err == nil {
				t.Fatal("Parse succeeded, want error")
			}
		})
	}
}

func TestNextID(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		prefix   string
		want     string
	}{
		{"empty", nil, "oq", "oq-1"},
		{"dense", []string{"ac-1", "ac-2", "ac-3"}, "ac", "ac-4"},
		{"gap is filled", []string{"co-1", "co-3"}, "co", "co-2"},
		{"foreign prefixes ignored", []string{"ac-1", "dc-2"}, "oq", "oq-1"},
		{"non-numeric suffix ignored", []string{"dc-final", "dc-1"}, "dc", "dc-2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NextID(tc.existing, tc.prefix); got != tc.want {
				t.Fatalf("NextID = %q, want %q", got, tc.want)
			}
		})
	}
}
