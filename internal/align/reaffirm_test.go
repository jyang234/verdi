package align

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func dispositionedJudged(id, text string, disposition artifact.FindingDisposition, note string) artifact.Finding {
	return artifact.Finding{ID: id, Kind: artifact.FindingJudged, Text: text, Disposition: disposition, Note: note}
}

// TestNotResurfacedIDsCanNeverBeACIDs is spec/finding-identity
// judged-spec-stale-own-text-judged-id-prefix's ground-truth invariant: the
// evidence.SpecStale trigger-(a) scan over OwnNotResurfaced was unreachable by
// construction because a not-resurfaced entry's id can NEVER equal an
// acceptance-criterion id. ReconcileJudged is the SOLE producer of the
// not-resurfaced: section (report.go's Generate writes only
// judgedRecon.NotResurfaced), it emits only judged-kind entries there, every
// judged id is "judged-"-prefixed (judge.go, the synthetic AbsenceFindingID,
// and contract-violation ids alike), and artifact's acIDRe requires ^ac- — so
// the two id namespaces are provably disjoint. This pins both halves, so
// re-adding the dead OwnNotResurfaced scan under the old premise would break
// here.
func TestNotResurfacedIDsCanNeverBeACIDs(t *testing.T) {
	// A spread of every judged id shape that can land in not-resurfaced: an
	// ordinary judged slug, the synthetic coverage-absence id, and a
	// contract-violation id.
	existing := []artifact.Finding{
		dispositionedJudged("judged-retry-semantics", "t1", artifact.FindingAcceptedDeviation, "n"),
		dispositionedJudged(AbsenceFindingID, "t2", artifact.FindingAcceptedDeviation, "n"),
		dispositionedJudged("judged-contract-violation-foo", "t3", artifact.FindingFixed, "n"),
	}
	// Nothing resurfaces this run -> every prior dispositioned judged finding
	// lands in NotResurfaced.
	got := ReconcileJudged(nil, existing, nil)
	if len(got.NotResurfaced) != len(existing) {
		t.Fatalf("NotResurfaced = %d entries, want %d (nothing resurfaced)", len(got.NotResurfaced), len(existing))
	}
	for _, f := range got.NotResurfaced {
		if f.Kind != artifact.FindingJudged {
			t.Fatalf("not-resurfaced entry %s kind = %s, want judged (only judged-kind ever persists here)", f.ID, f.Kind)
		}
		if !strings.HasPrefix(f.ID, "judged-") {
			t.Fatalf("not-resurfaced id %q is not judged-prefixed", f.ID)
		}
		// The load-bearing half: a not-resurfaced id is never a valid AC id, so
		// a StoryACIDs/featureACIDs set (built from AC ids) can never contain
		// it. Evidence/Text are populated so Validate fails ONLY on the id shape.
		acShaped := artifact.AcceptanceCriterion{ID: f.ID, Text: "x", Evidence: []artifact.EvidenceKind{artifact.EvidenceBehavioral}}
		if err := acShaped.Validate(); err == nil {
			t.Fatalf("not-resurfaced id %q validated as an acceptance-criterion id — the trigger-(a) scan would NOT be unreachable", f.ID)
		}
	}
	// Sanity: a real AC id DOES validate under the identical construction, so
	// the rejection above is about the id shape, not a vacuously-failing check.
	realAC := artifact.AcceptanceCriterion{ID: "ac-1", Text: "x", Evidence: []artifact.EvidenceKind{artifact.EvidenceBehavioral}}
	if err := realAC.Validate(); err != nil {
		t.Fatalf("ac-1 should be a valid AC id: %v", err)
	}
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

// TestReconcileJudged_CollisionRecurrence_CarriesForwardByExactIdentity is
// spec/finding-identity judged-judged-slug-collision-carry's fix proof: a
// deterministically recurring, BYTE-IDENTICAL collision (the same slug shared
// by the same 2+ fresh findings, run after run) carries its prior human
// disposition forward on every disambiguated member AND on the synthetic
// judge-contract-violation finding — exactly the exact-identity carry
// (identity.go's frozen Kind+ID+Text rule) every other judged finding gets,
// making contractViolationFinding's own doc claim ("survives via ordinary
// exact-identity matching on this synthetic finding") finally true.
//
// Red-first (before the fix): the collision branch emitted every member and
// the synthetic finding undispositioned, never consulting priorByIdentity —
// so a byte-identical recurrence dropped all three prior dispositions into
// not-resurfaced while undispositioned byte-identical twins sat in findings:,
// forcing a human to re-disposition the same disclosed violation every
// regeneration.
func TestReconcileJudged_CollisionRecurrence_CarriesForwardByExactIdentity(t *testing.T) {
	// Round 1: two fresh findings collide on one slug. ReconcileJudged
	// disambiguates the members and appends the synthetic violation finding —
	// all three undispositioned on a first run.
	fresh := []artifact.Finding{
		freshJudged("judged-dup", "first reading"),
		freshJudged("judged-dup", "second, different reading"),
	}
	round1 := ReconcileJudged(fresh, nil, nil)
	if len(round1.Findings) != 3 {
		t.Fatalf("round1.Findings = %+v, want 3 (both members + the synthetic violation)", round1.Findings)
	}

	// A human dispositions all three as accepted-deviation — the frozen,
	// disclosed collision as of this covering head.
	const note = "owner-ratified: disclosed collision, tracked"
	dispositioned := make([]artifact.Finding, len(round1.Findings))
	for i, f := range round1.Findings {
		f.Disposition = artifact.FindingAcceptedDeviation
		f.Note = note
		dispositioned[i] = f
	}

	// Round 2: the judge deterministically re-emits the SAME collision, byte
	// for byte. Every disambiguated member and the synthetic violation is now
	// a byte-identical recurrence, so each CARRIES its prior disposition — no
	// undispositioned twin, nothing draining into not-resurfaced.
	round2 := ReconcileJudged(fresh, dispositioned, nil)

	if len(round2.NotResurfaced) != 0 {
		t.Fatalf("round2.NotResurfaced = %+v, want none — a byte-identical collision recurrence must carry, never drain into not-resurfaced", round2.NotResurfaced)
	}
	if len(round2.Findings) != 3 {
		t.Fatalf("round2.Findings = %+v, want 3", round2.Findings)
	}
	for _, f := range round2.Findings {
		if !f.Dispositioned() {
			t.Fatalf("round2 finding %s is UNDISPOSITIONED — its byte-identical prior disposition must have carried (contractViolationFinding's doc claim)", f.ID)
		}
		if f.Disposition != artifact.FindingAcceptedDeviation || f.Note != note {
			t.Fatalf("round2 finding %s = %+v, want the prior disposition carried verbatim", f.ID, f)
		}
	}
	// A collision never pre-fills a candidate (ac-4: the human resolves the
	// slug's lineage); an exact-identity carry is not a candidate either.
	if len(round2.Candidates) != 0 {
		t.Fatalf("round2.Candidates = %+v, want none for a colliding slug", round2.Candidates)
	}
}

// TestReconcileJudged_CollisionRecurrence_RewordedMemberDoesNotCarry proves
// the fix stays fail-closed: a NON-identical recurrence (the collision's
// second member reworded) does NOT carry — only the byte-identical members
// (here, the first member and, because the group's texts changed, NOT the
// synthetic violation) carry; the reworded member lands undispositioned and
// its prior ruling persists in not-resurfaced, exactly as a non-identical
// judged recurrence behaves today.
func TestReconcileJudged_CollisionRecurrence_RewordedMemberDoesNotCarry(t *testing.T) {
	fresh1 := []artifact.Finding{
		freshJudged("judged-dup", "first reading"),
		freshJudged("judged-dup", "second reading"),
	}
	round1 := ReconcileJudged(fresh1, nil, nil)
	dispositioned := make([]artifact.Finding, len(round1.Findings))
	for i, f := range round1.Findings {
		f.Disposition = artifact.FindingAcceptedDeviation
		f.Note = "owner-ratified"
		dispositioned[i] = f
	}

	// Round 2: the first member recurs byte-identically; the second is
	// reworded (so the synthetic violation's text, which quotes both, also
	// changes and is no longer byte-identical).
	fresh2 := []artifact.Finding{
		freshJudged("judged-dup", "first reading"),
		freshJudged("judged-dup", "second reading, now reworded"),
	}
	round2 := ReconcileJudged(fresh2, dispositioned, nil)

	byID := make(map[string]artifact.Finding, len(round2.Findings))
	for _, f := range round2.Findings {
		byID[f.ID] = f
	}
	if m := byID["judged-dup"]; m.Disposition != artifact.FindingAcceptedDeviation {
		t.Fatalf("first member %+v, want its byte-identical prior disposition carried", m)
	}
	if m := byID["judged-dup-collision-2"]; m.Dispositioned() {
		t.Fatalf("reworded second member %+v, want UNDISPOSITIONED (non-identical recurrence never carries)", m)
	}
	// The reworded member's prior ruling and the now-stale synthetic
	// violation's prior ruling both persist in not-resurfaced (unmatched).
	nrIDs := make(map[string]bool, len(round2.NotResurfaced))
	for _, f := range round2.NotResurfaced {
		nrIDs[f.ID] = true
	}
	if !nrIDs["judged-dup-collision-2"] {
		t.Fatalf("NotResurfaced = %+v, want the reworded member's prior ruling persisted", round2.NotResurfaced)
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

// TestReconcileJudged_CollisionWithBacking_BackingRecordSurvivesRecurrence is
// spec/finding-identity judged-collision-backing-regeneration-drain's headline
// fix proof. When a slug collides among fresh findings AND that same slug owns
// a not-resurfaced backing record, NO live member may keep the bare slug: every
// member is suffixed (collisionMemberIDs) so the backing record alone owns the
// bare id. That dissolves the id-keyed drain by construction — a byte-identical
// recurrence of the collision can never mark the backing record "resurfaced" by
// matching a live member that merely shares its bare id.
//
// Red-first (before the fix): the base member kept the bare slug, so once a
// human dispositioned it, a byte-identical recurrence's carryExactMatch set
// matched[bareID] and the distinct-identity backing record was silently dropped
// from NotResurfaced — decrementing the spec-stale/feature-close budget with no
// human act.
func TestReconcileJudged_CollisionWithBacking_BackingRecordSurvivesRecurrence(t *testing.T) {
	backing := []artifact.Finding{
		dispositionedJudged("judged-dup", "an old ruling under the same slug", artifact.FindingAcceptedDeviation, "owner-ratified"),
	}
	fresh := []artifact.Finding{
		freshJudged("judged-dup", "first reading"),
		freshJudged("judged-dup", "second, different reading"),
	}

	round1 := ReconcileJudged(fresh, nil, backing)

	// No live member owns the bare slug — the backing record alone keeps it, so
	// its exit ramp stays reachable and the bookkeeping can never conflate them.
	for _, f := range round1.Findings {
		if f.ID == "judged-dup" {
			t.Fatalf("Findings has a live member on the bare slug %+v — with a backing record present every member must be suffixed", f)
		}
	}
	// Both members survive (suffixed) plus the one synthetic violation.
	if len(round1.Findings) != 3 {
		t.Fatalf("round1.Findings = %+v, want 3 (both suffixed members + the synthetic violation)", round1.Findings)
	}
	// The backing record survives, standing alone under the bare slug.
	if len(round1.NotResurfaced) != 1 || round1.NotResurfaced[0].ID != "judged-dup" {
		t.Fatalf("round1.NotResurfaced = %+v, want the backing record to survive under the bare slug", round1.NotResurfaced)
	}

	// A human dispositions every live member (their suffixed ids are what the
	// report now carries).
	dispositioned := make([]artifact.Finding, len(round1.Findings))
	for i, f := range round1.Findings {
		f.Disposition = artifact.FindingAcceptedDeviation
		f.Note = "owner-ratified: disclosed collision"
		dispositioned[i] = f
	}

	// Round 2: the judge re-emits the SAME collision byte-for-byte, the backing
	// record still standing. The backing record must NOT drain.
	round2 := ReconcileJudged(fresh, dispositioned, round1.NotResurfaced)

	if len(round2.NotResurfaced) != 1 || round2.NotResurfaced[0].ID != "judged-dup" {
		t.Fatalf("round2.NotResurfaced = %+v, want the backing record still standing — a byte-identical collision recurrence must never drain it", round2.NotResurfaced)
	}
	if round2.NotResurfaced[0].Disposition != artifact.FindingAcceptedDeviation || round2.NotResurfaced[0].Note != "owner-ratified" {
		t.Fatalf("round2 backing = %+v, want it left verbatim (budget unchanged)", round2.NotResurfaced[0])
	}
	// Every live member still carries its prior disposition on the frozen
	// Kind+ID+Text rule — the text-rank id assignment is deterministic.
	for _, f := range round2.Findings {
		if !f.Dispositioned() {
			t.Fatalf("round2 member %s is UNDISPOSITIONED — a byte-identical recurrence must carry its prior disposition", f.ID)
		}
	}
}

// TestReconcileJudged_CollisionWithBacking_TextRankIsEmissionOrderStable proves
// the backing-case id assignment is a function of member TEXT, not the judge's
// incidental emission order: the SAME member set emitted in a different order
// reproduces the SAME id->text pairing, so a dispositioned member carries
// forward even when the judge reshuffles its output between runs.
func TestReconcileJudged_CollisionWithBacking_TextRankIsEmissionOrderStable(t *testing.T) {
	backing := []artifact.Finding{
		dispositionedJudged("judged-dup", "an old ruling under the same slug", artifact.FindingAcceptedDeviation, "owner-ratified"),
	}
	order1 := []artifact.Finding{freshJudged("judged-dup", "alpha reading"), freshJudged("judged-dup", "beta reading")}
	order2 := []artifact.Finding{freshJudged("judged-dup", "beta reading"), freshJudged("judged-dup", "alpha reading")}

	idByText := func(r JudgedReconciliation) map[string]string {
		m := make(map[string]string)
		for _, f := range r.Findings {
			m[f.Text] = f.ID
		}
		return m
	}
	got1 := idByText(ReconcileJudged(order1, nil, backing))
	got2 := idByText(ReconcileJudged(order2, nil, backing))

	if got1["alpha reading"] != got2["alpha reading"] || got1["beta reading"] != got2["beta reading"] {
		t.Fatalf("id assignment differs by emission order: order1=%v order2=%v — text-rank must be order-independent", got1, got2)
	}
}

// TestReconcileJudged_CollisionWithBacking_CollisionClears_RegeneratesCleanly
// is judged-collision-backing-regeneration-drain's other state-table entry:
// once a collision that owned a backing record clears, the report regenerates
// with every not-resurfaced id distinct — so Generate's self-validation (unique
// ids, report.go) can never reject it with a duplicate. Because the fix never
// let a live member share the backing record's bare id, the prior suffixed
// members drift into not-resurfaced under their OWN ids, never colliding with
// the bare-id backing record.
func TestReconcileJudged_CollisionWithBacking_CollisionClears_RegeneratesCleanly(t *testing.T) {
	// A resolved collision as of the prior report: two suffixed members
	// dispositioned in findings:, the backing record standing in not-resurfaced.
	existingFindings := []artifact.Finding{
		dispositionedJudged("judged-dup-collision-1", "first reading", artifact.FindingAcceptedDeviation, "n"),
		dispositionedJudged("judged-dup-collision-2", "second, different reading", artifact.FindingAcceptedDeviation, "n"),
	}
	backing := []artifact.Finding{
		dispositionedJudged("judged-dup", "an old ruling under the same slug", artifact.FindingAcceptedDeviation, "owner-ratified"),
	}
	// The collision clears — a fresh run emits a single, unrelated finding.
	fresh := []artifact.Finding{freshJudged("judged-elsewhere", "a wholly different reading")}

	got := ReconcileJudged(fresh, existingFindings, backing)

	seen := make(map[string]bool, len(got.NotResurfaced))
	for _, f := range got.NotResurfaced {
		if seen[f.ID] {
			t.Fatalf("NotResurfaced = %+v, want every id distinct — a duplicate would brick regeneration", got.NotResurfaced)
		}
		seen[f.ID] = true
	}
	for _, id := range []string{"judged-dup", "judged-dup-collision-1", "judged-dup-collision-2"} {
		if !seen[id] {
			t.Fatalf("NotResurfaced = %+v, want %s persisted", got.NotResurfaced, id)
		}
	}
}

// TestReconcileJudged_CollisionNoBacking_BaseMemberKeepsBareSlug pins the
// unchanged no-backing behavior (judged-collision-backing-regeneration-drain's
// "keep the no-backing case exactly as today"): with no backing record for the
// slug, the first-emitted member keeps the bare slug and later members are
// suffixed from -collision-2 — no gratuitous id churn that would break an
// existing dispositioned base member's carry-forward.
func TestReconcileJudged_CollisionNoBacking_BaseMemberKeepsBareSlug(t *testing.T) {
	fresh := []artifact.Finding{
		freshJudged("judged-dup", "first reading"),
		freshJudged("judged-dup", "second, different reading"),
	}

	got := ReconcileJudged(fresh, nil, nil)

	ids := make(map[string]bool, len(got.Findings))
	for _, f := range got.Findings {
		ids[f.ID] = true
	}
	if !ids["judged-dup"] {
		t.Fatalf("Findings = %+v, want the base member to keep the bare slug judged-dup (no backing record present)", got.Findings)
	}
	if !ids["judged-dup-collision-2"] {
		t.Fatalf("Findings = %+v, want the later member suffixed -collision-2", got.Findings)
	}
}
