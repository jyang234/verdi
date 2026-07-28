package align

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestApplyArchivedRulings_CrossLevelCandidatePreFilled is ledger L-N14's
// companion (cross-level re-recording awareness): a fresh FEATURE-level judged
// finding that ReconcileJudged left a plain NEW finding — no feature-report prior
// at its slug — but whose slug a CLOSED implementing story's ARCHIVED report
// dispositioned, becomes a CANDIDATE citing the archive, with the archived ruling
// seated as its backing record (so the disposition verb stamps carried-from on
// confirmation). Nothing auto-carries: the fresh finding stays undispositioned.
//
// Red-first: against the stub applyArchivedRulings (returns recon unchanged) no
// candidate is pre-filled and no backing is seated.
func TestApplyArchivedRulings_CrossLevelCandidatePreFilled(t *testing.T) {
	fresh := artifact.Finding{ID: "judged-retry-semantics", Kind: artifact.FindingJudged, Text: "NEW feature-level wording"}
	recon := ReconcileJudged([]artifact.Finding{fresh}, nil, nil)
	if _, ok := recon.Candidates[fresh.ID]; ok {
		t.Fatal("setup: a fresh finding with no feature-report prior must have no same-report candidate")
	}

	archived := []ArchivedRuling{{
		Finding: artifact.Finding{ID: "judged-retry-semantics", Kind: artifact.FindingJudged, Text: "OLD story-level wording", Disposition: artifact.FindingAcceptedDeviation, Note: "owner-ratified"},
		Source:  "spec/judge-ergonomics",
	}}
	out := applyArchivedRulings(recon, archived)

	cand, ok := out.Candidates[fresh.ID]
	if !ok {
		t.Fatalf("Candidates = %+v, want a cross-level candidate for %s", out.Candidates, fresh.ID)
	}
	if cand.ArchiveSource != "spec/judge-ergonomics" {
		t.Fatalf("candidate ArchiveSource = %q, want the archive ref", cand.ArchiveSource)
	}
	if cand.OldText != "OLD story-level wording" || cand.OldDisposition != artifact.FindingAcceptedDeviation {
		t.Fatalf("candidate = %+v, want the archived ruling's old text/disposition", cand)
	}

	var seated bool
	for _, nr := range out.NotResurfaced {
		if nr.ID == fresh.ID && nr.Text == "OLD story-level wording" {
			seated = true
		}
	}
	if !seated {
		t.Fatalf("NotResurfaced = %+v, want the archived ruling seated as the candidate's backing record", out.NotResurfaced)
	}
	if out.Findings[0].Dispositioned() {
		t.Fatalf("fresh finding = %+v, want it left UNDISPOSITIONED — nothing auto-carries across levels", out.Findings[0])
	}

	// The candidate + seated backing shape is a valid living report frontmatter
	// (an undispositioned live finding beside its same-id, distinct-text backing).
	fm := artifact.DeviationFrontmatter{Schema: "verdi.deviation/v1", Covers: strings.Repeat("a", 40), Findings: out.Findings, NotResurfaced: out.NotResurfaced}
	if err := fm.Validate(); err != nil {
		t.Fatalf("cross-level candidate+backing shape does not validate: %v", err)
	}
}

// TestApplyArchivedRulings_FeatureReportPriorTakesPrecedence proves the feature's
// OWN prior governs a slug the archive also carries: a same-report candidate that
// ReconcileJudged already pre-filled is never overwritten with the archive's
// ruling, and never marked with an ArchiveSource.
func TestApplyArchivedRulings_FeatureReportPriorTakesPrecedence(t *testing.T) {
	fresh := artifact.Finding{ID: "judged-s", Kind: artifact.FindingJudged, Text: "new feature text"}
	featurePrior := artifact.Finding{ID: "judged-s", Kind: artifact.FindingJudged, Text: "feature's own old text", Disposition: artifact.FindingAcceptedDeviation, Note: "n"}
	recon := ReconcileJudged([]artifact.Finding{fresh}, []artifact.Finding{featurePrior}, nil)
	if _, ok := recon.Candidates[fresh.ID]; !ok {
		t.Fatal("setup: expected a same-report candidate from the feature-report prior")
	}

	archived := []ArchivedRuling{{
		Finding: artifact.Finding{ID: "judged-s", Kind: artifact.FindingJudged, Text: "archive's old text", Disposition: artifact.FindingAcceptedDeviation, Note: "n"},
		Source:  "spec/other-story",
	}}
	out := applyArchivedRulings(recon, archived)

	cand := out.Candidates[fresh.ID]
	if cand.ArchiveSource != "" {
		t.Fatalf("candidate ArchiveSource = %q, want empty — the feature-report prior takes precedence", cand.ArchiveSource)
	}
	if cand.OldText != "feature's own old text" {
		t.Fatalf("candidate OldText = %q, want the feature-report prior's own text", cand.OldText)
	}
}

// TestApplyArchivedRulings_ExactCarryNotOverridden proves an exact within-report
// carry (the fresh finding byte-identical to a dispositioned feature-report prior)
// is left carried, never demoted to a cross-level candidate.
func TestApplyArchivedRulings_ExactCarryNotOverridden(t *testing.T) {
	prior := artifact.Finding{ID: "judged-s", Kind: artifact.FindingJudged, Text: "identical text", Disposition: artifact.FindingAcceptedDeviation, Note: "n"}
	fresh := artifact.Finding{ID: "judged-s", Kind: artifact.FindingJudged, Text: "identical text"}
	recon := ReconcileJudged([]artifact.Finding{fresh}, []artifact.Finding{prior}, nil)
	if !recon.Findings[0].Dispositioned() {
		t.Fatal("setup: expected an exact within-report carry")
	}
	archived := []ArchivedRuling{{
		Finding: artifact.Finding{ID: "judged-s", Kind: artifact.FindingJudged, Text: "archive text", Disposition: artifact.FindingAcceptedDeviation, Note: "n"},
		Source:  "spec/other",
	}}
	out := applyArchivedRulings(recon, archived)
	if _, ok := out.Candidates[fresh.ID]; ok {
		t.Fatalf("Candidates = %+v, want NO candidate — an exact carry is never demoted to a cross-level candidate", out.Candidates)
	}
	if !out.Findings[0].Dispositioned() {
		t.Fatalf("finding = %+v, want the exact carry preserved", out.Findings[0])
	}
}

// TestApplyArchivedRulings_NoArchiveMatch_StaysNewFinding proves a fresh finding
// whose slug matches no archived ruling stays a plain new finding — the archive
// consultation adds nothing and seats nothing.
func TestApplyArchivedRulings_NoArchiveMatch_StaysNewFinding(t *testing.T) {
	fresh := artifact.Finding{ID: "judged-unmatched", Kind: artifact.FindingJudged, Text: "new"}
	recon := ReconcileJudged([]artifact.Finding{fresh}, nil, nil)
	archived := []ArchivedRuling{{
		Finding: artifact.Finding{ID: "judged-different", Kind: artifact.FindingJudged, Text: "x", Disposition: artifact.FindingAcceptedDeviation, Note: "n"},
		Source:  "spec/other",
	}}
	out := applyArchivedRulings(recon, archived)
	if len(out.Candidates) != 0 {
		t.Fatalf("Candidates = %+v, want none", out.Candidates)
	}
	if len(out.NotResurfaced) != 0 {
		t.Fatalf("NotResurfaced = %+v, want none seated", out.NotResurfaced)
	}
}

// TestRenderCandidates_MarksArchiveSource proves a cross-level candidate's
// rendered block cites its archive origin, so a human reviewing the report sees
// the ruling came from an archived story before confirming — while an ordinary
// same-report candidate (no ArchiveSource) renders exactly as before.
func TestRenderCandidates_MarksArchiveSource(t *testing.T) {
	findings := []artifact.Finding{
		{ID: "judged-cross", Kind: artifact.FindingJudged, Text: "new cross text"},
		{ID: "judged-same", Kind: artifact.FindingJudged, Text: "new same text"},
	}
	candidates := map[string]JudgedCandidate{
		"judged-cross": {OldDisposition: artifact.FindingAcceptedDeviation, OldText: "old cross", ArchiveSource: "spec/judge-ergonomics"},
		"judged-same":  {OldDisposition: artifact.FindingAcceptedDeviation, OldText: "old same"},
	}
	body := RenderBody(findings, candidates, nil, nil, nil, nil)
	if !strings.Contains(body, "from archived spec/judge-ergonomics") {
		t.Fatalf("cross-level candidate does not cite its archive source:\n%s", body)
	}
	// The same-report candidate's prior-ruling line carries no archive marker.
	if strings.Contains(body, "judged-same** CANDIDATE") {
		// locate the same-report block and ensure it has no "from archived"
		if strings.Count(body, "from archived") != 1 {
			t.Fatalf("expected exactly one 'from archived' marker (only the cross-level candidate):\n%s", body)
		}
	}
}
