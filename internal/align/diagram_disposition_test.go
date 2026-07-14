package align

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestGenerate_DiagramFindingDispositionSurvivesRegeneration is obligation
// ac-4--behavioral's round-trip test: a divergent diagram finding is hand-
// dispositioned accepted-deviation with a note, then a second Generate call
// against the SAME unchanged inputs (with the first report's findings
// passed as ExistingFindings) must carry that disposition forward
// unchanged, through the exact SAME PreserveDispositions path an existing
// boundary finding already rides (identity.go) — no special case for a
// diagram finding.
func TestGenerate_DiagramFindingDispositionSurvivesRegeneration(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	writeDiagramFixture(t, repo.Dir, "loan-flow-base",
		"id: diagram/loan-flow-base\nkind: diagram\ntitle: Base\nowners: [platform-team]\nstatus: active\n",
		"flowchart LR\n    LegacyStep[\"LegacyStep\"]\n")
	writeDiagramFixture(t, repo.Dir, "loan-flow-target",
		"id: diagram/loan-flow-target\nkind: diagram\ntitle: Target\nclass: proposal\nstatus: accepted\nowners: [platform-team]\n"+
			"frozen: { at: 2026-07-14, commit: 3e91ab2 }\n"+
			"derived_from: { ref: diagram/loan-flow-base, digest: sha256:"+hex64Diagram+" }\n",
		"flowchart LR\n    LegacyStep[\"LegacyStep\"]\n")

	first, err := Generate(context.Background(), baseGenerateInput(t, repo.Dir, svcDir, repo.Head, spec))
	if err != nil {
		t.Fatalf("Generate (first): %v", err)
	}

	const wantID = "diagram-loan-flow-target"
	const note = "known, drawing to be updated next sprint"

	dispositioned := make([]artifact.Finding, len(first.Frontmatter.Findings))
	var found bool
	for i, f := range first.Frontmatter.Findings {
		if f.ID == wantID {
			found = true
			if f.Kind != artifact.FindingComputed {
				t.Fatalf("finding %s: Kind = %q, want computed", wantID, f.Kind)
			}
			f.Disposition = artifact.FindingAcceptedDeviation
			f.Note = note
		}
		dispositioned[i] = f
	}
	if !found {
		t.Fatalf("first report's findings %+v: missing expected divergent diagram finding %s", first.Frontmatter.Findings, wantID)
	}

	second, err := Generate(context.Background(), Input{
		Root: repo.Dir, Runner: seedComputeRunner(svcDir), Spec: spec, Covers: repo.Head,
		JudgeCmd: []string{writeFakeJudge(t, fakeJudgeOKScript)}, JudgeTimeout: time.Second,
		ExistingFindings: dispositioned,
	})
	if err != nil {
		t.Fatalf("Generate (second): %v", err)
	}

	var survived bool
	for _, f := range second.Frontmatter.Findings {
		if f.ID != wantID {
			continue
		}
		survived = true
		if f.Disposition != artifact.FindingAcceptedDeviation {
			t.Fatalf("finding %s: Disposition = %q, want accepted-deviation (must survive unchanged regeneration)", wantID, f.Disposition)
		}
		if f.Note != note {
			t.Fatalf("finding %s: Note = %q, want %q", wantID, f.Note, note)
		}
	}
	if !survived {
		t.Fatalf("second report's findings %+v: missing %s entirely", second.Frontmatter.Findings, wantID)
	}
}
