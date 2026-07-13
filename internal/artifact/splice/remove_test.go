package splice

// The trash gestures' splice ops (owner directive, round-7: dragging a
// wall element to the trash removes it from the record): RemoveObjectEntry
// takes a declared object's frontmatter entry out (never its body prose),
// RemoveDecisionLinksMatching splices every matching link out of one
// decision's links: in a single edit. Byte-precision proofs in the style
// of TestRemoveDecisionLink.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func TestRemoveObjectEntry(t *testing.T) {
	t.Run("middle element removal is exactly one line", func(t *testing.T) {
		d := mustParse(t, sampleSpec)
		edit, err := d.RemoveObjectEntry("ac-2")
		if err != nil {
			t.Fatalf("RemoveObjectEntry: %v", err)
		}
		out := applyAndValidate(t, d, edit)
		// The expected buffer is the sample minus ac-2's one entry line.
		want := strings.Replace(sampleSpec,
			"  - { id: ac-2, text: \"the reviewer sees: resubmission history for the document\", evidence: [behavioral, attestation], anchor: \"#ac-2\" }\n",
			"", 1)
		if !bytes.Equal(out, []byte(want)) {
			t.Fatalf("removal is not the exact line deletion:\n%s", out)
		}
		// The body prose and its anchor heading are NOT deleted.
		if !strings.Contains(string(out), "## ac-2") {
			t.Error("body heading for ac-2 was deleted (prose is never silently destroyed)")
		}
		for _, id := range untouchedObjects {
			if !bytes.Equal(objectSpanBytes(t, []byte(sampleSpec), id), objectSpanBytes(t, out, id)) {
				t.Errorf("untouched object %s changed", id)
			}
		}
	})

	t.Run("first and last elements of a block list", func(t *testing.T) {
		for _, id := range []string{"ac-1", "ac-3"} {
			d := mustParse(t, sampleSpec)
			edit, err := d.RemoveObjectEntry(id)
			if err != nil {
				t.Fatalf("RemoveObjectEntry(%s): %v", id, err)
			}
			out := applyAndValidate(t, d, edit)
			if strings.Contains(string(out), "{ id: "+id+",") {
				t.Errorf("%s entry still present:\n%s", id, out)
			}
			if !strings.Contains(string(out), "## "+id) {
				t.Errorf("%s body heading deleted", id)
			}
		}
	})

	t.Run("sole element removes the whole block key", func(t *testing.T) {
		d := mustParse(t, sampleSpec)
		edit, err := d.RemoveObjectEntry("co-1")
		if err != nil {
			t.Fatalf("RemoveObjectEntry: %v", err)
		}
		out := applyAndValidate(t, d, edit)
		want := strings.Replace(sampleSpec,
			"constraints:\n  - { id: co-1, text: \"retries are capped at: 3 attempts\", anchor: \"#co-1\" }\n",
			"", 1)
		if !bytes.Equal(out, []byte(want)) {
			t.Fatalf("sole-element removal did not take the whole block key:\n%s", out)
		}
		if !strings.Contains(string(out), "## co-1") {
			t.Error("body heading for co-1 was deleted")
		}
	})

	t.Run("a multi-line entry (decision with links) goes whole", func(t *testing.T) {
		d := mustParse(t, sampleSpec)
		edit, err := d.RemoveObjectEntry("dc-1")
		if err != nil {
			t.Fatalf("RemoveObjectEntry: %v", err)
		}
		out := applyAndValidate(t, d, edit)
		if strings.Contains(string(out), "excuse this flow") || strings.Contains(string(out), "exempts") {
			t.Errorf("dc-1's entry (both lines) not fully removed:\n%s", out)
		}
		if !strings.Contains(string(out), "{ id: dc-2,") || !strings.Contains(string(out), "{ id: dc-3,") {
			t.Errorf("removing dc-1 disturbed its siblings:\n%s", out)
		}
	})

	t.Run("append then remove restores the original buffer byte-for-byte", func(t *testing.T) {
		d := mustParse(t, sampleSpec)
		edits, err := d.AppendObject("oq-1", "does the retry cap hold?", nil)
		if err != nil {
			t.Fatalf("AppendObject: %v", err)
		}
		grown := applyAndValidate(t, d, edits...)

		d2 := mustParse(t, string(grown))
		rm, err := d2.RemoveObjectEntry("oq-1")
		if err != nil {
			t.Fatalf("RemoveObjectEntry: %v", err)
		}
		out, err := d2.Apply([]Edit{rm})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		// The frontmatter is restored exactly; the body keeps the appended
		// "## oq-1" section (prose is never silently destroyed), so compare
		// the frontmatter halves.
		wantFM, _, err := artifact.SplitFrontmatter([]byte(sampleSpec))
		if err != nil {
			t.Fatalf("SplitFrontmatter: %v", err)
		}
		gotFM, _, err := artifact.SplitFrontmatter(out)
		if err != nil {
			t.Fatalf("SplitFrontmatter: %v", err)
		}
		if !bytes.Equal(wantFM, gotFM) {
			t.Fatalf("append+remove is not the frontmatter identity:\n%s", gotFM)
		}
		if !strings.Contains(string(out), "## oq-1") {
			t.Error("removal deleted the body section (prose is never silently destroyed)")
		}
	})

	t.Run("flow-style lists: subset and sole element", func(t *testing.T) {
		flowSpec := `---
id: spec/flow-style
kind: spec
class: feature
title: "Flow style"
status: draft
owners: [t]
acceptance_criteria: [ { id: ac-1, text: "one", evidence: [attestation], anchor: "#ac-1" }, { id: ac-2, text: "two", evidence: [attestation], anchor: "#ac-2" } ]
open_questions: [ { id: oq-1, text: "sole", anchor: "#oq-1" } ]
---
# Flow style

## ac-1

Prose.

## ac-2

Prose.

## oq-1

Prose.
`
		d := mustParse(t, flowSpec)
		edit, err := d.RemoveObjectEntry("ac-2")
		if err != nil {
			t.Fatalf("RemoveObjectEntry(flow ac-2): %v", err)
		}
		out := applyAndValidate(t, d, edit)
		if !strings.Contains(string(out), `acceptance_criteria: [ { id: ac-1, text: "one", evidence: [attestation], anchor: "#ac-1" } ]`) {
			t.Errorf("flow subset removal broke the list:\n%s", out)
		}

		d2 := mustParse(t, flowSpec)
		edit2, err := d2.RemoveObjectEntry("oq-1")
		if err != nil {
			t.Fatalf("RemoveObjectEntry(flow sole oq-1): %v", err)
		}
		out2 := applyAndValidate(t, d2, edit2)
		if strings.Contains(string(out2), "open_questions") {
			t.Errorf("flow sole removal left the empty key behind:\n%s", out2)
		}
	})

	t.Run("negative paths fail closed", func(t *testing.T) {
		d := mustParse(t, sampleSpec)
		for name, id := range map[string]string{
			"unknown id":         "ac-99",
			"unknown prefix":     "zz-1",
			"missing block":      "oq-1",
			"empty id":           "",
			"decision that isnt": "dc-99",
		} {
			if _, err := d.RemoveObjectEntry(id); err == nil {
				t.Errorf("%s: succeeded, want error", name)
			}
		}
	})
}

func TestRemoveDecisionLinksMatching(t *testing.T) {
	matchRef := func(want string) func(linkType, ref string) bool {
		return func(_, ref string) bool { return ref == want }
	}

	t.Run("all matching removes the whole links key", func(t *testing.T) {
		d := mustParse(t, sampleSpec)
		edit, n, err := d.RemoveDecisionLinksMatching("dc-1", matchRef("adr/0001-outbox-events"))
		if err != nil {
			t.Fatalf("RemoveDecisionLinksMatching: %v", err)
		}
		if n != 1 {
			t.Fatalf("count = %d, want 1", n)
		}
		out := applyAndValidate(t, d, edit)
		if strings.Contains(string(out), "exempts") {
			t.Errorf("removed link still present:\n%s", out)
		}
		if !strings.Contains(string(out), `- { id: dc-1, text: "excuse this flow from the outbox rule", anchor: "#dc-1" }`) {
			t.Errorf("dc-1 did not collapse to its linkless house-style form:\n%s", out)
		}
	})

	t.Run("a kept link survives verbatim in one edit", func(t *testing.T) {
		d := mustParse(t, sampleSpec)
		grow, err := d.AppendDecisionLink("dc-1", artifact.Link{Type: artifact.LinkSupersedes, Ref: "adr/0009-x"})
		if err != nil {
			t.Fatalf("AppendDecisionLink: %v", err)
		}
		twoLinks := applyAndValidate(t, d, grow)

		d2 := mustParse(t, string(twoLinks))
		edit, n, err := d2.RemoveDecisionLinksMatching("dc-1", matchRef("adr/0001-outbox-events"))
		if err != nil {
			t.Fatalf("RemoveDecisionLinksMatching: %v", err)
		}
		if n != 1 {
			t.Fatalf("count = %d, want 1", n)
		}
		out := applyAndValidate(t, d2, edit)
		if !strings.Contains(string(out), "links: [ { type: supersedes, ref: \"adr/0009-x\" } ]") {
			t.Errorf("kept link not preserved in house style:\n%s", out)
		}
	})

	t.Run("two of three removed in one edit keeps the middle survivor", func(t *testing.T) {
		d := mustParse(t, sampleSpec)
		e1, err := d.AppendDecisionLink("dc-1", artifact.Link{Type: artifact.LinkSupersedes, Ref: "adr/0009-x"})
		if err != nil {
			t.Fatalf("AppendDecisionLink: %v", err)
		}
		grown := applyAndValidate(t, d, e1)
		d2 := mustParse(t, string(grown))
		e2, err := d2.AppendDecisionLink("dc-1", artifact.Link{Type: artifact.LinkExempts, Ref: "adr/0001-outbox-events", Note: "again"})
		if err != nil {
			t.Fatalf("AppendDecisionLink: %v", err)
		}
		three := applyAndValidate(t, d2, e2)

		d3 := mustParse(t, string(three))
		edit, n, err := d3.RemoveDecisionLinksMatching("dc-1", matchRef("adr/0001-outbox-events"))
		if err != nil {
			t.Fatalf("RemoveDecisionLinksMatching: %v", err)
		}
		if n != 2 {
			t.Fatalf("count = %d, want 2 (both edges to the ref go in one batch)", n)
		}
		out := applyAndValidate(t, d3, edit)
		if strings.Contains(string(out), "adr/0001-outbox-events") {
			t.Errorf("a matched link survived:\n%s", out)
		}
		if !strings.Contains(string(out), "links: [ { type: supersedes, ref: \"adr/0009-x\" } ]") {
			t.Errorf("the unmatched survivor was lost or reshaped:\n%s", out)
		}
	})

	t.Run("an object referenced by two decisions clears in one batch", func(t *testing.T) {
		twoRefs := strings.Replace(sampleSpec,
			`  - { id: dc-2, text: "use the existing notification channel", anchor: "#dc-2" }`,
			`  - { id: dc-2, text: "use the existing notification channel", anchor: "#dc-2", links: [ { type: depends-on, ref: "spec/sample-flow#dc-3" } ] }`,
			1)
		twoRefs = strings.Replace(twoRefs,
			`links: [ { type: exempts, ref: adr/0001-outbox-events, note: "async by design" } ] }`,
			`links: [ { type: exempts, ref: adr/0001-outbox-events, note: "async by design" }, { type: supersedes, ref: "spec/sample-flow#dc-3" } ] }`,
			1)
		d := mustParse(t, twoRefs)
		fragMatch := matchRef("spec/sample-flow#dc-3")
		rmEntry, err := d.RemoveObjectEntry("dc-3")
		if err != nil {
			t.Fatalf("RemoveObjectEntry: %v", err)
		}
		rm1, n1, err := d.RemoveDecisionLinksMatching("dc-1", fragMatch)
		if err != nil {
			t.Fatalf("RemoveDecisionLinksMatching(dc-1): %v", err)
		}
		rm2, n2, err := d.RemoveDecisionLinksMatching("dc-2", fragMatch)
		if err != nil {
			t.Fatalf("RemoveDecisionLinksMatching(dc-2): %v", err)
		}
		if n1 != 1 || n2 != 1 {
			t.Fatalf("counts = %d, %d, want 1, 1", n1, n2)
		}
		out := applyAndValidate(t, d, rmEntry, rm1, rm2)
		if strings.Contains(string(out), "dc-3") && strings.Contains(string(out), "{ id: dc-3") {
			t.Errorf("dc-3 entry survived:\n%s", out)
		}
		if strings.Contains(string(out), "spec/sample-flow#dc-3") {
			t.Errorf("a link naming the removed object survived (VL-003 would trip):\n%s", out)
		}
		if !strings.Contains(string(out), "exempts") {
			t.Errorf("dc-1's unrelated exempts link was lost:\n%s", out)
		}
	})

	t.Run("negative paths fail closed", func(t *testing.T) {
		d := mustParse(t, sampleSpec)
		any := func(_, _ string) bool { return true }
		for name, try := range map[string]func() (Edit, int, error){
			"not a decision id": func() (Edit, int, error) { return d.RemoveDecisionLinksMatching("ac-1", any) },
			"missing decision":  func() (Edit, int, error) { return d.RemoveDecisionLinksMatching("dc-99", any) },
			"no links at all":   func() (Edit, int, error) { return d.RemoveDecisionLinksMatching("dc-2", any) },
			"empty links list":  func() (Edit, int, error) { return d.RemoveDecisionLinksMatching("dc-3", any) },
			"nothing matches": func() (Edit, int, error) {
				return d.RemoveDecisionLinksMatching("dc-1", matchRef("adr/none"))
			},
		} {
			if _, _, err := try(); err == nil {
				t.Errorf("%s: succeeded, want error", name)
			}
		}
	})
}
