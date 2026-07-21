package align

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func dispositionedJudged(id, text string, disposition artifact.FindingDisposition, note string) artifact.Finding {
	return artifact.Finding{ID: id, Kind: artifact.FindingJudged, Text: text, Disposition: disposition, Note: note}
}

func freshJudged(id, text string) artifact.Finding {
	return artifact.Finding{ID: id, Kind: artifact.FindingJudged, Text: text}
}

// TestReconcileJudged_ExactMatch_CarriesForward proves AC-2's frozen rule
// still governs the ordinary case: a fresh judged finding whose (kind, id,
// text) is byte-identical to a prior dispositioned one carries its
// disposition (and any prior CarriedFrom) forward automatically — no
// candidate, no human action required, exactly PreserveDispositions'
// existing behavior for computed findings.
func TestReconcileJudged_ExactMatch_CarriesForward(t *testing.T) {
	existing := []artifact.Finding{dispositionedJudged("judged-a", "same text", artifact.FindingFixed, "reviewed")}
	fresh := []artifact.Finding{freshJudged("judged-a", "same text")}

	got := ReconcileJudged(fresh, existing, nil)

	if len(got.Findings) != 1 {
		t.Fatalf("Findings = %+v, want 1", got.Findings)
	}
	if got.Findings[0].Disposition != artifact.FindingFixed || got.Findings[0].Note != "reviewed" {
		t.Fatalf("Findings[0] = %+v, want disposition/note carried forward (exact match)", got.Findings[0])
	}
	if len(got.Candidates) != 0 {
		t.Fatalf("Candidates = %+v, want none (exact match is not a candidate)", got.Candidates)
	}
	if len(got.NotResurfaced) != 0 {
		t.Fatalf("NotResurfaced = %+v, want none (the finding resurfaced exactly)", got.NotResurfaced)
	}
}

// TestReconcileJudged_SlugMatch_RewordedText_BecomesCandidate is spec/
// finding-identity ac-1's headline case, driven against the reconciliation
// core directly: a same-slug, reworded-text regeneration must NEVER
// silently carry the old disposition — it becomes a Candidate, and the
// fresh finding itself stays undispositioned.
func TestReconcileJudged_SlugMatch_RewordedText_BecomesCandidate(t *testing.T) {
	existing := []artifact.Finding{dispositionedJudged("judged-retry-semantics", "old judge prose", artifact.FindingAcceptedDeviation, "owner-ratified")}
	fresh := []artifact.Finding{freshJudged("judged-retry-semantics", "reworded judge prose, same underlying issue")}

	got := ReconcileJudged(fresh, existing, nil)

	if len(got.Findings) != 1 {
		t.Fatalf("Findings = %+v, want 1", got.Findings)
	}
	if got.Findings[0].Dispositioned() {
		t.Fatalf("Findings[0] = %+v, want UNDISPOSITIONED — a slug match must never be silently carried", got.Findings[0])
	}
	if got.Findings[0].Text != "reworded judge prose, same underlying issue" {
		t.Fatalf("Findings[0].Text = %q, want the fresh judge's own new text preserved", got.Findings[0].Text)
	}
	cand, ok := got.Candidates["judged-retry-semantics"]
	if !ok {
		t.Fatal("Candidates missing judged-retry-semantics — the pre-fill context AC-1 requires")
	}
	if cand.OldDisposition != artifact.FindingAcceptedDeviation || cand.OldText != "old judge prose" || cand.OldNote != "owner-ratified" {
		t.Fatalf("Candidate = %+v, want old ruling+old text+old note preserved beside the new text", cand)
	}
}

// TestReconcileJudged_Escalation_PresentsBothTexts_NothingSilentlyCarries is
// ac-2's masking scenario: a low-confidence cosmetic ruling (accepted-
// deviation) followed, on a later run under the identical slug, by a
// reworded high-confidence finding. The candidate must present both texts;
// nothing about the finding's own fresh Disposition is set automatically
// regardless of what the OLD ruling was.
func TestReconcileJudged_Escalation_PresentsBothTexts_NothingSilentlyCarries(t *testing.T) {
	existing := []artifact.Finding{dispositionedJudged("judged-retry-semantics", "looks cosmetic (confidence 0.35)", artifact.FindingAcceptedDeviation, "low confidence, deferred")}
	fresh := []artifact.Finding{freshJudged("judged-retry-semantics", "this is a real regression (confidence 0.93)")}

	got := ReconcileJudged(fresh, existing, nil)

	if got.Findings[0].Dispositioned() {
		t.Fatalf("Findings[0] = %+v, want UNDISPOSITIONED (escalation must never inherit the old ruling)", got.Findings[0])
	}
	cand := got.Candidates["judged-retry-semantics"]
	if cand.OldText != "looks cosmetic (confidence 0.35)" {
		t.Fatalf("Candidate.OldText = %q, want the OLD low-confidence text preserved for side-by-side comparison", cand.OldText)
	}
	if got.Findings[0].Text != "this is a real regression (confidence 0.93)" {
		t.Fatalf("Findings[0].Text = %q, want the NEW high-confidence text", got.Findings[0].Text)
	}
}

// TestReconcileJudged_CollidingSlugs_DiscloseContractViolation_NeverDedupe
// is ac-4's collision rule: two fresh findings sharing one slug within a
// single run must both survive verbatim (never deduped) plus a synthetic,
// disclosed contract-violation finding naming the collision.
func TestReconcileJudged_CollidingSlugs_DiscloseContractViolation_NeverDedupe(t *testing.T) {
	fresh := []artifact.Finding{
		freshJudged("judged-dup", "first reading"),
		freshJudged("judged-dup", "second, different reading"),
	}

	got := ReconcileJudged(fresh, nil, nil)

	// Both original findings survive, plus exactly one synthetic disclosure.
	if len(got.Findings) != 3 {
		t.Fatalf("Findings = %+v, want 3 (both colliding findings + 1 disclosure)", got.Findings)
	}
	// Every id is unique — deviation-report.md's pre-existing schema
	// (internal/artifact.DeviationFrontmatter.Validate) requires it, and
	// disposition.go's whole-line body matching depends on it — so "never
	// dedupe" is satisfied by disambiguating ids, never by keeping two
	// findings sharing one id (which would simply fail to decode).
	seenIDs := make(map[string]bool, 3)
	for _, f := range got.Findings {
		if seenIDs[f.ID] {
			t.Fatalf("Findings = %+v, want every id unique even for a colliding slug", got.Findings)
		}
		seenIDs[f.ID] = true
	}
	var sawFirst, sawSecond, sawViolation int
	for _, f := range got.Findings {
		switch {
		case f.ID == "judged-dup" && f.Text == "first reading":
			sawFirst++
		case f.ID == "judged-dup-collision-2" && f.Text == "second, different reading":
			sawSecond++
		case strings.HasPrefix(f.ID, "judged-contract-violation-"):
			sawViolation++
			if f.Kind != artifact.FindingJudged {
				t.Fatalf("violation finding kind = %q, want judged", f.Kind)
			}
			if f.Dispositioned() {
				t.Fatalf("violation finding must be undispositioned fresh, got %+v", f)
			}
			if !strings.Contains(f.Text, "first reading") || !strings.Contains(f.Text, "second, different reading") {
				t.Fatalf("violation finding text = %q, want it to quote both colliding texts", f.Text)
			}
		}
	}
	if sawFirst != 1 || sawSecond != 1 || sawViolation != 1 {
		t.Fatalf("Findings = %+v, want exactly one of each (first, second, violation)", got.Findings)
	}
	// No candidate pairing for an ambiguous collision — the human resolves
	// which (if either) finding continues the slug's lineage.
	if len(got.Candidates) != 0 {
		t.Fatalf("Candidates = %+v, want none for a colliding slug", got.Candidates)
	}
}

// TestReconcileJudged_DriftingSlug_LandsInNotResurfaced is ac-3's core:
// a prior dispositioned finding whose slug the fresh judge run simply does
// not re-emit at all (drifted away, not reworded) lands in NotResurfaced,
// verbatim — never silently dropped.
func TestReconcileJudged_DriftingSlug_LandsInNotResurfaced(t *testing.T) {
	existing := []artifact.Finding{dispositionedJudged("judged-vanished", "an old accepted deviation", artifact.FindingAcceptedDeviation, "n")}
	fresh := []artifact.Finding{freshJudged("judged-unrelated", "a completely different finding")}

	got := ReconcileJudged(fresh, existing, nil)

	if len(got.NotResurfaced) != 1 || got.NotResurfaced[0].ID != "judged-vanished" {
		t.Fatalf("NotResurfaced = %+v, want the vanished finding preserved verbatim", got.NotResurfaced)
	}
	if got.NotResurfaced[0].Disposition != artifact.FindingAcceptedDeviation || got.NotResurfaced[0].Note != "n" {
		t.Fatalf("NotResurfaced[0] = %+v, want disposition/note preserved verbatim", got.NotResurfaced[0])
	}
}

// TestReconcileJudged_NotResurfaced_PersistsAcrossFurtherRegenerations
// proves not-resurfaced entries stay persisted across MULTIPLE
// regenerations that keep failing to reproduce them — not just one round —
// by feeding a prior round's own NotResurfaced output back in as
// existingNotResurfaced, mirroring how align.Generate threads it through
// Input.ExistingNotResurfaced round to round.
func TestReconcileJudged_NotResurfaced_PersistsAcrossFurtherRegenerations(t *testing.T) {
	existing := []artifact.Finding{dispositionedJudged("judged-vanished", "an old accepted deviation", artifact.FindingAcceptedDeviation, "n")}
	fresh := []artifact.Finding{freshJudged("judged-unrelated", "round 1")}

	round1 := ReconcileJudged(fresh, existing, nil)
	if len(round1.NotResurfaced) != 1 {
		t.Fatalf("round1.NotResurfaced = %+v, want 1", round1.NotResurfaced)
	}

	// Round 2: existingFindings is now round1's own (fresh) findings; the
	// not-resurfaced entry is fed back in as existingNotResurfaced, exactly
	// as loadExistingReport/align.Input would thread it from disk.
	fresh2 := []artifact.Finding{freshJudged("judged-unrelated", "round 2, still nothing about judged-vanished")}
	round2 := ReconcileJudged(fresh2, round1.Findings, round1.NotResurfaced)

	if len(round2.NotResurfaced) != 1 || round2.NotResurfaced[0].ID != "judged-vanished" {
		t.Fatalf("round2.NotResurfaced = %+v, want judged-vanished still persisted a second round", round2.NotResurfaced)
	}
}

// TestReconcileJudged_NotResurfaced_ResurfacesAsCandidate proves
// not-resurfaced:'s pre-fill-UI consumer: a finding sitting in
// existingNotResurfaced (not existingFindings) that resurfaces under its
// same slug, reworded, still pre-fills as a Candidate exactly as if it had
// still been live in Findings — and it stays listed in NotResurfaced as its
// backing record (removed only by a human's explicit confirmation via
// `verdi disposition`, cmd/verdi).
func TestReconcileJudged_NotResurfaced_ResurfacesAsCandidate(t *testing.T) {
	notResurfaced := []artifact.Finding{dispositionedJudged("judged-back-again", "archived old text", artifact.FindingAcceptedDeviation, "n")}
	fresh := []artifact.Finding{freshJudged("judged-back-again", "resurfaced, reworded text")}

	got := ReconcileJudged(fresh, nil, notResurfaced)

	if got.Findings[0].Dispositioned() {
		t.Fatalf("Findings[0] = %+v, want UNDISPOSITIONED (a resurfacing not-resurfaced entry is still a candidate, not an auto-carry)", got.Findings[0])
	}
	cand, ok := got.Candidates["judged-back-again"]
	if !ok || cand.OldText != "archived old text" {
		t.Fatalf("Candidates = %+v, want judged-back-again paired with its archived old text", got.Candidates)
	}
	if len(got.NotResurfaced) != 1 || got.NotResurfaced[0].ID != "judged-back-again" {
		t.Fatalf("NotResurfaced = %+v, want the backing record to stay persisted until a human confirms", got.NotResurfaced)
	}
}

// TestReconcileJudged_ComputedFindingsIgnored proves ReconcileJudged only
// ever reasons about Kind == FindingJudged — a computed finding passed in
// by mistake is neither matched against nor emitted, since Generate is
// responsible for routing computed findings through PreserveDispositions
// instead (AC-2's byte-unchanged computed path).
func TestReconcileJudged_ComputedFindingsIgnored(t *testing.T) {
	existing := []artifact.Finding{{ID: "computed-a", Kind: artifact.FindingComputed, Text: "t", Disposition: artifact.FindingFixed}}
	fresh := []artifact.Finding{freshJudged("judged-a", "t2")}

	got := ReconcileJudged(fresh, existing, nil)

	if len(got.NotResurfaced) != 0 {
		t.Fatalf("NotResurfaced = %+v, want none (a computed finding is never ReconcileJudged's concern)", got.NotResurfaced)
	}
}
