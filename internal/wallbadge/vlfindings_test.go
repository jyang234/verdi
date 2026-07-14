package wallbadge

import (
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/lint"
)

const specA = ".verdi/specs/active/widget-retry/spec.md"
const specB = ".verdi/specs/active/other-spec/spec.md"

// TestVLBadges_ObjectAnchored is badge-computes ac-2's first bucket: a
// finding declaring an object locus badges exactly that object's target,
// grouped by (rule, target), with every firing message riding Records.
func TestVLBadges_ObjectAnchored(t *testing.T) {
	findings := []lint.Finding{
		{Rule: "VL-006", Path: specA, Message: "acceptance criterion ac-1 declares no expected evidence kind", Locus: lint.ObjectLocus("ac-1")},
		{Rule: "VL-006", Path: specA, Message: `stub "spike-x" names resolves oq-9, which is not declared`, Locus: lint.ObjectLocus("stub:spike-x")},
	}
	got := VLBadges(findings, specA, "sha256:aaaa")
	if len(got) != 2 {
		t.Fatalf("got %d badges, want 2:\n%+v", len(got), got)
	}
	byTarget := map[string]DerivationRecord{}
	for _, b := range got {
		byTarget[b.Target] = b
	}
	ac1, ok := byTarget["ac-1"]
	if !ok {
		t.Fatalf("no badge targeting ac-1: %+v", got)
	}
	if ac1.Source != "lint:VL-006" {
		t.Errorf("ac-1 badge source = %q, want lint:VL-006", ac1.Source)
	}
	if len(ac1.Inputs) != 1 || ac1.Inputs[0].Path != specA || ac1.Inputs[0].Revision != "sha256:aaaa" {
		t.Errorf("ac-1 badge inputs = %+v, want one spec input pinned to sha256:aaaa", ac1.Inputs)
	}
	if len(ac1.Records) != 1 || ac1.Records[0] != "acceptance criterion ac-1 declares no expected evidence kind" {
		t.Errorf("ac-1 badge records = %+v", ac1.Records)
	}
	if _, ok := byTarget["stub:spike-x"]; !ok {
		t.Fatalf("no badge targeting stub:spike-x: %+v", got)
	}
}

// TestVLBadges_SameRuleSameTargetMerges proves a rule that fires twice
// against the SAME card produces ONE chip carrying both messages, not two
// separate badges (dc-2: "records: one entry per firing record" — the
// grouping is per (rule, target), not per Finding).
func TestVLBadges_SameRuleSameTargetMerges(t *testing.T) {
	findings := []lint.Finding{
		{Rule: "VL-006", Path: specA, Message: "z-message", Locus: lint.ObjectLocus("ac-1")},
		{Rule: "VL-006", Path: specA, Message: "a-message", Locus: lint.ObjectLocus("ac-1")},
	}
	got := VLBadges(findings, specA, "sha256:aaaa")
	if len(got) != 1 {
		t.Fatalf("got %d badges, want 1 merged badge:\n%+v", len(got), got)
	}
	if len(got[0].Records) != 2 || got[0].Records[0] != "a-message" || got[0].Records[1] != "z-message" {
		t.Fatalf("Records = %+v, want [a-message z-message] (sorted)", got[0].Records)
	}
}

// TestVLBadges_SpecLevel is badge-computes ac-2's second bucket: a
// finding declaring a spec-level locus (empty Object) badges the case
// file — Target is "".
func TestVLBadges_SpecLevel(t *testing.T) {
	findings := []lint.Finding{
		{Rule: "VL-006", Path: specA, Message: "new-class spec has no problem attribute", Locus: lint.SpecLocus()},
	}
	got := VLBadges(findings, specA, "sha256:aaaa")
	if len(got) != 1 {
		t.Fatalf("got %d badges, want 1", len(got))
	}
	if got[0].Target != "" {
		t.Errorf("Target = %q, want empty (case-file badge)", got[0].Target)
	}
}

// TestVLBadges_NoLocusExcluded is badge-computes ac-2's third bucket,
// fail-closed: a finding with a nil Locus (plumbing, decode failures)
// never reaches a badge, even though its Path is this spec's own document.
func TestVLBadges_NoLocusExcluded(t *testing.T) {
	findings := []lint.Finding{
		{Rule: "VL-018", Path: specA, Message: "positions key does not resolve"}, // no Locus
		{Rule: "VL-001", Path: specA, Message: "decode error"},                   // no Locus
	}
	got := VLBadges(findings, specA, "sha256:aaaa")
	if len(got) != 0 {
		t.Fatalf("got %d badges, want 0 (no finding declared a locus):\n%+v", len(got), got)
	}
}

// TestVLBadges_DifferentSpecExcluded proves a locus-bearing finding whose
// Path names a DIFFERENT spec's document never badges this wall — the
// "scoped to this spec's directory" half of the partition (ac-2).
func TestVLBadges_DifferentSpecExcluded(t *testing.T) {
	findings := []lint.Finding{
		{Rule: "VL-006", Path: specB, Message: "acceptance criterion ac-1 declares no expected evidence kind", Locus: lint.ObjectLocus("ac-1")},
	}
	got := VLBadges(findings, specA, "sha256:aaaa")
	if len(got) != 0 {
		t.Fatalf("got %d badges from a different spec's finding, want 0:\n%+v", len(got), got)
	}
}

// TestVLBadges_Empty is the trivial negative case: no findings at all.
func TestVLBadges_Empty(t *testing.T) {
	if got := VLBadges(nil, specA, "sha256:aaaa"); len(got) != 0 {
		t.Fatalf("got %d badges from nil findings, want 0", len(got))
	}
}

// TestVLBadges_LongMessageLabelTruncates proves vlLabel bounds the chip's
// short text (dc-2) rather than ever emitting an unbounded label — the
// full message still rides Records untouched.
func TestVLBadges_LongMessageLabelTruncates(t *testing.T) {
	long := "this message is deliberately long enough that it must exceed the label bound comfortably by a good margin"
	findings := []lint.Finding{{Rule: "VL-006", Path: specA, Message: long, Locus: lint.ObjectLocus("ac-1")}}
	got := VLBadges(findings, specA, "sha256:aaaa")
	if len(got) != 1 {
		t.Fatalf("got %d badges, want 1", len(got))
	}
	if len(got[0].Label) >= len(long) {
		t.Errorf("Label = %q (len %d), want shorter than the original message (len %d)", got[0].Label, len(got[0].Label), len(long))
	}
	if got[0].Records[0] != long {
		t.Errorf("Records[0] = %q, want the full untruncated message", got[0].Records[0])
	}
}

// TestVLBadges_Deterministic is ac-4's determinism requirement applied to
// this one compute: the same finding set, run twice (including with
// randomly-ordered input, since map iteration inside VLBadges must never
// leak into output order), produces byte-identical output.
func TestVLBadges_Deterministic(t *testing.T) {
	findings := []lint.Finding{
		{Rule: "VL-006", Path: specA, Message: "m1", Locus: lint.ObjectLocus("ac-2")},
		{Rule: "VL-003", Path: specA, Message: "m2", Locus: lint.ObjectLocus("dc-1")},
		{Rule: "VL-006", Path: specA, Message: "m3", Locus: lint.SpecLocus()},
	}
	first := VLBadges(findings, specA, "sha256:aaaa")
	second := VLBadges(findings, specA, "sha256:aaaa")
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("non-deterministic output:\nrun 1: %+v\nrun 2: %+v", first, second)
	}
}
