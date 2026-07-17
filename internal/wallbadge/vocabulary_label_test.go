// spec/vocabulary-surfaces ac-2, the wallbadge surface — the FLAG-immunity
// pin. The ladder badges (spec-stale, pending-supersession) are case-file
// FLAGS (03 §The amendment ladder), not lifecycle states, so their visible
// labels are FIXED and NOT vocabulary-addressable in v1: a vocabulary entry
// keyed `spec-stale` under `states:` does NOT rename the flag. This is the
// negative case for finding judged-ladder-flags-share-state-namespace —
// where routing flag ids through model.DisplayState quietly made
// vocabulary.states a shared namespace, letting a state entry rename a
// flag. The derivation record's Source, inputs, and records stay bare ids
// too (receipts are addressing, never display). Genuine lifecycle-state
// badges (e.g. dex terminal-status badges) remain DisplayState-resolved on
// their own surfaces; only the flags are fixed.
package wallbadge

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/model"
)

// ladderVocabModel is the ADVERSARIAL model: it keys BOTH ladder flag ids
// under `states:` (a legal verdi.model/v1 vocabulary — kernel validation
// constrains structure, never vocabulary keys), attempting to rename them.
// The pins below prove the attempt has no effect on the flag labels.
func ladderVocabModel() *model.Model {
	return &model.Model{
		Schema: "verdi.model/v1",
		Vocabulary: model.Vocabulary{
			States: map[string]string{
				"spec-stale":             "Drifted",
				"pending-supersession":   "Successor pending",
				"accepted-pending-build": "Ready to build",
			},
		},
	}
}

// TestSpecStaleBadge_FlagLabelNotVocabularyAddressable proves a states
// entry keyed `spec-stale` does NOT rename the flag: even given the
// adversarial model, the badge label stays the fixed id `spec-stale`
// (finding judged-ladder-flags-share-state-namespace).
func TestSpecStaleBadge_FlagLabelNotVocabularyAddressable(t *testing.T) {
	root, fm := writeStoreSpec(t, "widget-retry", ladderStorySpec)
	writeDeviationReport(t, root, "widget-retry", flaggedDeviationReportMD(ladderCoversSHA))
	snap := buildSnapshotFor(t, root)

	got, err := SpecStaleBadge(root, snap, fm.ID, 3, ladderVocabModel())
	if err != nil {
		t.Fatalf("SpecStaleBadge: %v", err)
	}
	if got == nil {
		t.Fatal("got nil badge, want flagged spec-stale")
	}
	if got.Label != "spec-stale" {
		t.Fatalf("Label = %q, want the FIXED flag id %q — a states entry keyed spec-stale must not rename the flag (judged-ladder-flags-share-state-namespace)", got.Label, "spec-stale")
	}
	if got.Source != "ladder:spec-stale" {
		t.Fatalf("Source = %q, want the bare rule id (receipts never rename)", got.Source)
	}
}

// TestPendingSupersessionBadge_FlagLabelNotVocabularyAddressable mirrors
// the same negative pin for the second ladder rung.
func TestPendingSupersessionBadge_FlagLabelNotVocabularyAddressable(t *testing.T) {
	loader := fakeSupersessionLoader{
		ok: true,
		candidates: []evidence.OpenSupersessionCandidate{{
			MRID:   "7",
			Digest: "sha256:cccc",
			Spec:   &artifact.SpecFrontmatter{Supersession: &artifact.Supersession{Amended: []artifact.SupersessionNote{{ID: "ac-1", Note: "tightened"}}}},
		}},
	}
	got, disclosure, err := PendingSupersessionBadge(context.Background(), loader, implementsLink("spec/parent-feature#ac-1"), ladderVocabModel())
	if err != nil {
		t.Fatalf("PendingSupersessionBadge: %v", err)
	}
	if disclosure != "" {
		t.Fatalf("disclosure = %q, want none for a flagged outcome", disclosure)
	}
	if got == nil {
		t.Fatal("got nil badge, want flagged pending-supersession")
	}
	if got.Label != "pending-supersession" {
		t.Fatalf("Label = %q, want the FIXED flag id %q — a states entry keyed pending-supersession must not rename the flag (judged-ladder-flags-share-state-namespace)", got.Label, "pending-supersession")
	}
	if got.Source != "ladder:pending-supersession" {
		t.Fatalf("Source = %q, want the bare rule id (receipts never rename)", got.Source)
	}
}
