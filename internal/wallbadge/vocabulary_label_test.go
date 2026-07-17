// spec/vocabulary-surfaces ac-2, the wallbadge surface: the ladder
// badges' visible labels resolve through the resolved model's
// state-display lookup (the identical model.DisplayState fallback-to-id
// resolution every other surface uses) — while the derivation record's
// Source, inputs, and records stay on bare ids (receipts are addressing,
// never display). One case per badge so a regression on either fails
// independently; the nil-model fallback is every pre-existing test in
// this package passing unchanged with a nil argument.
package wallbadge

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/model"
)

// ladderVocabModel renames both ladder flags through the vocabulary's
// free-keyed state map — a legal verdi.model/v1 vocabulary (kernel
// validation constrains structure, never vocabulary keys).
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

// TestSpecStaleBadge_ModelVocabularyLabel proves the flagged badge's
// LABEL is the model's display resolution while everything evidentiary
// stays raw.
func TestSpecStaleBadge_ModelVocabularyLabel(t *testing.T) {
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
	if got.Label != "Drifted" {
		t.Fatalf("Label = %q, want the renamed display word %q", got.Label, "Drifted")
	}
	if got.Source != "ladder:spec-stale" {
		t.Fatalf("Source = %q, want the bare rule id (receipts never rename)", got.Source)
	}
}

// TestPendingSupersessionBadge_ModelVocabularyLabel mirrors the same
// proof for the second ladder rung.
func TestPendingSupersessionBadge_ModelVocabularyLabel(t *testing.T) {
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
	if got.Label != "Successor pending" {
		t.Fatalf("Label = %q, want the renamed display word %q", got.Label, "Successor pending")
	}
	if got.Source != "ladder:pending-supersession" {
		t.Fatalf("Source = %q, want the bare rule id (receipts never rename)", got.Source)
	}
}
