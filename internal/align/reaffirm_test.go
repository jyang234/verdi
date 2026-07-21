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

func findingWithID(fs []artifact.Finding, id string) (artifact.Finding, bool) {
	for _, f := range fs {
		if f.ID == id {
			return f, true
		}
	}
	return artifact.Finding{}, false
}

// TestReconcileJudged_ConfirmedCollisionMemberWithBacking_ByteIdenticalRecurrenceDoesNotDrain
// is the downstream-soundness pin for judged-collision-suffixed-backing-shadow's
// fix. Once the disposition verb leaves a distinct-content backing record
// standing beside a now-dispositioned suffixed collision member (both at the
// SAME suffixed id — the sanctioned post-confirmation shape), a later
// byte-identical align recurrence must CARRY the dispositioned member forward
// and LEAVE the backing record — never conflate the two by their shared id and
// silently drain the backing record out of the accepted-deviation budget (the
// exact X-18 laundering this story exists to close).
//
// Red-first (before keying `matched` by CONTENT IDENTITY rather than by bare
// id): carrying the reproducing member set matched[<shared id>]=true, so the
// distinct-identity backing record — never itself reproduced — was dropped from
// NotResurfaced anyway.
func TestReconcileJudged_ConfirmedCollisionMemberWithBacking_ByteIdenticalRecurrenceDoesNotDrain(t *testing.T) {
	// The post-confirmation shape the disposition verb now persists: a
	// dispositioned suffixed member and a distinct-content backing record, both at
	// the same suffixed id.
	existingFindings := []artifact.Finding{
		dispositionedJudged("judged-dup-collision-2", "T1prime reworded", artifact.FindingAcceptedDeviation, "owner-ratified: confirmed live member"),
	}
	backing := []artifact.Finding{
		dispositionedJudged("judged-dup-collision-2", "T1 older ruling", artifact.FindingAcceptedDeviation, "owner-ratified: standing"),
	}
	// A byte-identical recurrence: the collision re-emits and its rank-1 member
	// reproduces "T1prime reworded" exactly, so it lands back at
	// judged-dup-collision-2 and carries. ("AAA ..." sorts first -> bare slug.)
	fresh := []artifact.Finding{
		freshJudged("judged-dup", "AAA ranks first"),
		freshJudged("judged-dup", "T1prime reworded"),
	}

	got := ReconcileJudged(fresh, existingFindings, backing)

	m, ok := findingWithID(got.Findings, "judged-dup-collision-2")
	if !ok || m.Disposition != artifact.FindingAcceptedDeviation || m.Text != "T1prime reworded" {
		t.Fatalf("collision-2 live member = %+v (present=%v), want its prior disposition carried onto the reproducing member", m, ok)
	}
	// The distinct-content backing record SURVIVES — never drained by the
	// shared-id carry.
	if len(got.NotResurfaced) != 1 || got.NotResurfaced[0].ID != "judged-dup-collision-2" || got.NotResurfaced[0].Text != "T1 older ruling" {
		t.Fatalf("NotResurfaced = %+v, want the distinct-content backing record still standing (never drained by the shared-id carry)", got.NotResurfaced)
	}
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

// TestReconcileJudged_CollisionNoBacking_MinTextMemberKeepsBareSlug pins the
// no-backing behavior after the L-N13 determinism cure (judged-collision-cv-
// emission-order): with no backing record for the slug, the LOWEST-TEXT-RANKED
// member keeps the bare slug and the rest are suffixed from -collision-2 by text
// rank — never the judge's incidental emission order. Here emission order and
// text order coincide ("first reading" < "second, different reading"), so the
// bare/suffixed assignment is the same as the pre-cure emission scheme; the
// swapped-order determinism the cure adds is pinned separately by
// TestReconcileJudged_CanonicalOrdering_EmissionOrderIndependent.
func TestReconcileJudged_CollisionNoBacking_MinTextMemberKeepsBareSlug(t *testing.T) {
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
		t.Fatalf("Findings = %+v, want the lowest-text member to keep the bare slug judged-dup (no backing record present)", got.Findings)
	}
	if !ids["judged-dup-collision-2"] {
		t.Fatalf("Findings = %+v, want the higher-text member suffixed -collision-2", got.Findings)
	}
}

// TestReconcileJudged_CanonicalOrdering_EmissionOrderIndependent is the L-N13
// determinism cure's headline proof (judged-collision-cv-emission-order): the
// WHOLE collision machinery is a function of member CONTENT, never the judge's
// incidental emission order. The SAME member set emitted in a SWAPPED order
// must reproduce byte-identical suffixed ids AND a byte-identical
// contract-violation text — under BOTH the no-backing and backing schemes — so
// every prior disposition carries and nothing drains into not-resurfaced.
//
// Red-first (before the cure): the no-backing branch assigned the bare slug to
// the FIRST-EMITTED member and the CV join concatenated texts in EMISSION
// order, so a reorder flipped which member owned the bare id and changed the
// synthetic finding's text — the exact-identity carry then missed on the
// reshuffled members and on the CV finding, draining dispositioned priors into
// not-resurfaced and re-opening the X-18 re-adjudication churn.
func TestReconcileJudged_CanonicalOrdering_EmissionOrderIndependent(t *testing.T) {
	// Emission order (A, B) but text order B < A ("alpha" < "zebra"), so a
	// content-blind emission-order scheme would disagree with a text-rank one.
	a := freshJudged("judged-dup", "zebra reading")
	b := freshJudged("judged-dup", "alpha reading")

	idByText := func(r JudgedReconciliation) map[string]string {
		m := make(map[string]string)
		for _, f := range r.Findings {
			m[f.Text] = f.ID
		}
		return m
	}
	cvText := func(r JudgedReconciliation) string {
		for _, f := range r.Findings {
			if strings.HasPrefix(f.ID, "judged-contract-violation-") {
				return f.Text
			}
		}
		return ""
	}

	t.Run("no backing: ids and CV text are emission-order-independent", func(t *testing.T) {
		fwd := ReconcileJudged([]artifact.Finding{a, b}, nil, nil)
		rev := ReconcileJudged([]artifact.Finding{b, a}, nil, nil)

		gotFwd, gotRev := idByText(fwd), idByText(rev)
		if gotFwd["alpha reading"] != gotRev["alpha reading"] || gotFwd["zebra reading"] != gotRev["zebra reading"] {
			t.Fatalf("id assignment differs by emission order: fwd=%v rev=%v — the bare/suffixed assignment must be text-ranked, never emission-ordered", gotFwd, gotRev)
		}
		if cvText(fwd) != cvText(rev) {
			t.Fatalf("CV text differs by emission order:\n fwd=%q\n rev=%q — the join must be over canonically-sorted member texts", cvText(fwd), cvText(rev))
		}
	})

	t.Run("backing: ids and CV text are emission-order-independent", func(t *testing.T) {
		backing := []artifact.Finding{dispositionedJudged("judged-dup", "an old ruling", artifact.FindingAcceptedDeviation, "owner-ratified")}
		fwd := ReconcileJudged([]artifact.Finding{a, b}, nil, backing)
		rev := ReconcileJudged([]artifact.Finding{b, a}, nil, backing)

		gotFwd, gotRev := idByText(fwd), idByText(rev)
		if gotFwd["alpha reading"] != gotRev["alpha reading"] || gotFwd["zebra reading"] != gotRev["zebra reading"] {
			t.Fatalf("backing id assignment differs by emission order: fwd=%v rev=%v", gotFwd, gotRev)
		}
		if cvText(fwd) != cvText(rev) {
			t.Fatalf("backing CV text differs by emission order:\n fwd=%q\n rev=%q", cvText(fwd), cvText(rev))
		}
	})

	t.Run("byte-identical set in swapped order carries every prior, drains nothing", func(t *testing.T) {
		// Round 1 emits (A, B) and a human dispositions every member and the CV
		// finding.
		round1 := ReconcileJudged([]artifact.Finding{a, b}, nil, nil)
		dispositioned := make([]artifact.Finding, len(round1.Findings))
		for i, f := range round1.Findings {
			f.Disposition = artifact.FindingAcceptedDeviation
			f.Note = "owner-ratified"
			dispositioned[i] = f
		}

		// Round 2 re-emits the SAME member set in SWAPPED order (B, A). Every
		// member and the CV finding is a byte-identical recurrence — each must
		// carry, nothing may drain.
		round2 := ReconcileJudged([]artifact.Finding{b, a}, dispositioned, nil)
		if len(round2.NotResurfaced) != 0 {
			t.Fatalf("round2.NotResurfaced = %+v, want none — a byte-identical set re-emitted in swapped order must carry every prior, never drain", round2.NotResurfaced)
		}
		for _, f := range round2.Findings {
			if !f.Dispositioned() {
				t.Fatalf("round2 finding %s is UNDISPOSITIONED — a reordered byte-identical recurrence must carry its prior disposition", f.ID)
			}
		}
	})
}

// TestReconcileJudged_CollisionBackingBornThisRound_SuffixesEveryMember is
// spec/finding-identity judged-collision-backing-same-round's headline fix
// proof: the bare-slug protection collisionMemberIDs provides (suffix EVERY
// member so the backing record alone keeps the bare slug) must also fire in
// the exact round the backing record is BORN — when the prior report carries a
// LIVE dispositioned judged finding at slug S (in existingFindings, not yet in
// not-resurfaced) and a fresh run emits a 2+ member collision at S that
// reproduces none of it. The prior, matched by no member, lands in
// NotResurfaced at bare S THIS round; if any live member also kept bare S, that
// is precisely the live-member-shadows-backing-record state
// judged-collision-backing-regeneration-drain's "dissolved by construction"
// claim (and artifact.Validate's same-kind overlap rejection) say can never
// exist.
//
// Red-first (before the fix): backingByID was built from existingNotResurfaced
// ONLY, so hasBacking was false the round the backing was born — the
// first-emitted member kept bare S while the prior landed in NotResurfaced
// under the same bare S, the forbidden overlap. disposition.go would then treat
// the bare-slug live member as a genuine slug-only candidate and, on a matching
// decision, silently resolve the prior with reaffirmation provenance and no
// ac-1 side-by-side ever shown.
func TestReconcileJudged_CollisionBackingBornThisRound_SuffixesEveryMember(t *testing.T) {
	// The prior report: a single LIVE dispositioned judged finding at the slug —
	// NOT a collision, NOT yet in not-resurfaced.
	existingFindings := []artifact.Finding{
		dispositionedJudged("judged-dup", "the original single ruling under this slug", artifact.FindingAcceptedDeviation, "owner-ratified"),
	}
	// The fresh run NEWLY emits a 2+ member collision at the same slug, byte-
	// identical to none of the prior (the born-this-round scenario).
	fresh := []artifact.Finding{
		freshJudged("judged-dup", "first fresh reading"),
		freshJudged("judged-dup", "second, different fresh reading"),
	}

	round1 := ReconcileJudged(fresh, existingFindings, nil)

	// No live member may occupy the bare slug — the backing record born this
	// round alone owns it, so its exit ramp stays reachable and no live member
	// shadows it.
	for _, f := range round1.Findings {
		if f.ID == "judged-dup" {
			t.Fatalf("Findings has a live member on the bare slug %+v — a collision whose backing record is BORN this round must still suffix EVERY member", f)
		}
	}
	// Both members survive (suffixed) plus the one synthetic violation.
	if len(round1.Findings) != 3 {
		t.Fatalf("round1.Findings = %+v, want 3 (both suffixed members + the synthetic violation)", round1.Findings)
	}
	// The prior ruling lands in NotResurfaced at the bare slug, verbatim — no
	// silent resolution, and (ReconcileJudged never stamps) NO carried-from.
	if len(round1.NotResurfaced) != 1 || round1.NotResurfaced[0].ID != "judged-dup" {
		t.Fatalf("round1.NotResurfaced = %+v, want the prior ruling standing alone under the bare slug", round1.NotResurfaced)
	}
	if round1.NotResurfaced[0].Disposition != artifact.FindingAcceptedDeviation || round1.NotResurfaced[0].Note != "owner-ratified" {
		t.Fatalf("round1.NotResurfaced[0] = %+v, want the prior ruling preserved verbatim", round1.NotResurfaced[0])
	}
	if round1.NotResurfaced[0].CarriedFrom != "" {
		t.Fatalf("round1.NotResurfaced[0].CarriedFrom = %q, want empty — a born-this-round backing record is never a reaffirmation", round1.NotResurfaced[0].CarriedFrom)
	}
	// A collision never pre-fills a candidate (ac-4: the human resolves lineage).
	if len(round1.Candidates) != 0 {
		t.Fatalf("round1.Candidates = %+v, want none for a colliding slug", round1.Candidates)
	}

	// Second round (regression-pinned): the backing record now pre-exists in
	// not-resurfaced. Feeding round1 forward — the members dispositioned, the
	// backing record in existingNotResurfaced — the pre-existing-backing
	// protection (condition 1) keeps the record standing, unchanged.
	dispositioned := make([]artifact.Finding, len(round1.Findings))
	for i, f := range round1.Findings {
		f.Disposition = artifact.FindingAcceptedDeviation
		f.Note = "owner-ratified: disclosed collision"
		dispositioned[i] = f
	}
	round2 := ReconcileJudged(fresh, dispositioned, round1.NotResurfaced)
	if len(round2.NotResurfaced) != 1 || round2.NotResurfaced[0].ID != "judged-dup" {
		t.Fatalf("round2.NotResurfaced = %+v, want the backing record still standing (second-round protection unchanged)", round2.NotResurfaced)
	}
	for _, f := range round2.Findings {
		if f.ID == "judged-dup" {
			t.Fatalf("round2.Findings has a live member on the bare slug %+v — the pre-existing backing protection must still hold", f)
		}
	}
}

// dispositionAll returns a copy of r.Findings with every finding dispositioned
// accepted-deviation — the shape a prior round hands forward as
// existingFindings once a human ratified a disclosed collision.
func dispositionAll(r JudgedReconciliation) []artifact.Finding {
	out := make([]artifact.Finding, len(r.Findings))
	for i, f := range r.Findings {
		f.Disposition = artifact.FindingAcceptedDeviation
		f.Note = "owner-ratified"
		out[i] = f
	}
	return out
}

// TestReconcileJudged_TruthTable is L-N13's ONE enumerating pin: one row per
// reachable RECONCILEJUDGED cell of the finding-identity truth table documented
// in reaffirm.go's block comment (source × id-class × prior × recurrence →
// carry / Candidate / not-resurfaced). The DISPOSITION-VERB (decision → stamp)
// cells are pinned by the cmd/verdi tests that comment names, not re-driven
// here. Impossible cells are asserted as impossible where testable (a collision
// member reworded is NOT a Candidate; a suffixed id is never a Candidate).
func TestReconcileJudged_TruthTable(t *testing.T) {
	dj, fj := dispositionedJudged, freshJudged
	// The dispositioned round-1 collision output (members + CV), reused by the
	// contract-violation rows so the CV's prior is a genuine ReconcileJudged
	// artifact, not a hand-typed id.
	collisionPrior := dispositionAll(ReconcileJudged([]artifact.Finding{fj("judged-dup", "alpha"), fj("judged-dup", "beta")}, nil, nil))

	cases := []struct {
		name             string
		existingFindings []artifact.Finding
		existingNR       []artifact.Finding
		fresh            []artifact.Finding
		target           string
		wantAbsentLive   bool // target is NOT among live findings (drift / NR-only)
		wantDisposed     bool // target live finding carries a disposition
		wantCandidate    bool // target has a rendered Candidate
		wantNR           []string
	}{
		{
			name:   "fresh / bare / prior=none: a plain new finding",
			fresh:  []artifact.Finding{fj("judged-new", "brand new")},
			target: "judged-new",
		},
		{
			name:             "recurring-exact / bare / live-dispositioned / byte-identical: ac-2 carry",
			existingFindings: []artifact.Finding{dj("judged-a", "same", artifact.FindingFixed, "n")},
			fresh:            []artifact.Finding{fj("judged-a", "same")},
			target:           "judged-a",
			wantDisposed:     true,
		},
		{
			name:             "candidate / bare / live-dispositioned / reworded: ac-1",
			existingFindings: []artifact.Finding{dj("judged-a", "old", artifact.FindingAcceptedDeviation, "n")},
			fresh:            []artifact.Finding{fj("judged-a", "reworded")},
			target:           "judged-a",
			wantCandidate:    true,
			wantNR:           []string{"judged-a"},
		},
		{
			name:          "candidate / bare / not-resurfaced-AD / reworded: resurfaces as candidate",
			existingNR:    []artifact.Finding{dj("judged-b", "archived", artifact.FindingAcceptedDeviation, "n")},
			fresh:         []artifact.Finding{fj("judged-b", "reworded")},
			target:        "judged-b",
			wantCandidate: true,
			wantNR:        []string{"judged-b"},
		},
		{
			name:             "not-resurfaced / bare / drifted away / not reproduced: ac-3",
			existingFindings: []artifact.Finding{dj("judged-gone", "old", artifact.FindingAcceptedDeviation, "n")},
			fresh:            []artifact.Finding{fj("judged-other", "unrelated")},
			target:           "judged-gone",
			wantAbsentLive:   true,
			wantNR:           []string{"judged-gone"},
		},
		{
			name:           "not-resurfaced / bare / already-persisted / still not reproduced: persists",
			existingNR:     []artifact.Finding{dj("judged-gone", "old", artifact.FindingAcceptedDeviation, "n")},
			fresh:          []artifact.Finding{fj("judged-other", "still unrelated")},
			target:         "judged-gone",
			wantAbsentLive: true,
			wantNR:         []string{"judged-gone"},
		},
		{
			name: "collision-member / suffixed / byte-identical: carries, never a candidate",
			existingFindings: []artifact.Finding{
				dj("judged-dup", "alpha", artifact.FindingAcceptedDeviation, "n"),
				dj("judged-dup-collision-2", "beta", artifact.FindingAcceptedDeviation, "n"),
			},
			fresh:        []artifact.Finding{fj("judged-dup", "alpha"), fj("judged-dup", "beta")},
			target:       "judged-dup-collision-2",
			wantDisposed: true,
		},
		{
			name: "collision-member / suffixed / reordered: byte-identical SET swapped still carries",
			existingFindings: []artifact.Finding{
				dj("judged-dup", "alpha", artifact.FindingAcceptedDeviation, "n"),
				dj("judged-dup-collision-2", "beta", artifact.FindingAcceptedDeviation, "n"),
			},
			fresh:        []artifact.Finding{fj("judged-dup", "beta"), fj("judged-dup", "alpha")}, // emission swapped
			target:       "judged-dup-collision-2",
			wantDisposed: true,
		},
		{
			name: "collision-member / suffixed / reworded: undispositioned, NO candidate, prior to NR",
			existingFindings: []artifact.Finding{
				dj("judged-dup", "alpha", artifact.FindingAcceptedDeviation, "n"),
				dj("judged-dup-collision-2", "beta", artifact.FindingAcceptedDeviation, "n"),
			},
			fresh:         []artifact.Finding{fj("judged-dup", "alpha"), fj("judged-dup", "beta reworded")},
			target:        "judged-dup-collision-2",
			wantCandidate: false,
			wantNR:        []string{"judged-dup-collision-2"},
		},
		{
			name:             "contract-violation / reserved / byte-identical: carries",
			existingFindings: collisionPrior,
			fresh:            []artifact.Finding{fj("judged-dup", "alpha"), fj("judged-dup", "beta")},
			target:           "judged-contract-violation-dup",
			wantDisposed:     true,
		},
		{
			name:             "contract-violation / reserved / reworded members: CV misses, prior to NR",
			existingFindings: collisionPrior,
			fresh:            []artifact.Finding{fj("judged-dup", "alpha"), fj("judged-dup", "gamma")}, // beta->gamma changes the CV text
			target:           "judged-contract-violation-dup",
			wantCandidate:    false, // never a candidate — a fresh CV is emitted live, undispositioned
			wantNR:           []string{"judged-contract-violation-dup", "judged-dup-collision-2"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ReconcileJudged(tc.fresh, tc.existingFindings, tc.existingNR)

			live, present := findingWithID(got.Findings, tc.target)
			if tc.wantAbsentLive {
				if present {
					t.Fatalf("target %q present among live findings %+v, want absent (drift / not-resurfaced only)", tc.target, got.Findings)
				}
			} else {
				if !present {
					t.Fatalf("target %q absent among live findings %+v, want present", tc.target, got.Findings)
				}
				if live.Dispositioned() != tc.wantDisposed {
					t.Fatalf("target %q dispositioned=%v, want %v (finding=%+v)", tc.target, live.Dispositioned(), tc.wantDisposed, live)
				}
			}

			if _, isCand := got.Candidates[tc.target]; isCand != tc.wantCandidate {
				t.Fatalf("target %q Candidate=%v, want %v (Candidates=%+v)", tc.target, isCand, tc.wantCandidate, got.Candidates)
			}

			gotNR := make(map[string]bool, len(got.NotResurfaced))
			for _, f := range got.NotResurfaced {
				gotNR[f.ID] = true
			}
			if len(gotNR) != len(tc.wantNR) {
				t.Fatalf("not-resurfaced ids = %v, want exactly %v", nrIDs(got.NotResurfaced), tc.wantNR)
			}
			for _, id := range tc.wantNR {
				if !gotNR[id] {
					t.Fatalf("not-resurfaced ids = %v, want %q present", nrIDs(got.NotResurfaced), id)
				}
			}
		})
	}
}

func nrIDs(fs []artifact.Finding) []string {
	ids := make([]string, len(fs))
	for i, f := range fs {
		ids[i] = f.ID
	}
	return ids
}

// TestReconcileJudged_ConfirmedCollisionMemberWithBacking_DoubleNonReproductionIsLoud
// pins the disclosed downstream residual of judged-collision-suffixed-backing-
// shadow's fix (reaffirm.go's truth-table block comment, "DOWNSTREAM RESIDUAL").
// Once a confirmed collision member and a distinct-content backing record share
// one suffixed id, a round in which NEITHER text reproduces places two distinct
// entries under that id in not-resurfaced. This is caught LOUDLY — the report's
// own self-validation (artifact.Validate, which Generate runs) rejects a
// duplicate not-resurfaced id — never a silent laundering; the human's cue to
// resolve the backing record via its exit ramp once the collision clears.
func TestReconcileJudged_ConfirmedCollisionMemberWithBacking_DoubleNonReproductionIsLoud(t *testing.T) {
	existingFindings := []artifact.Finding{dispositionedJudged("judged-dup-collision-2", "TA confirmed member", artifact.FindingAcceptedDeviation, "n")}
	backing := []artifact.Finding{dispositionedJudged("judged-dup-collision-2", "TB backing record", artifact.FindingAcceptedDeviation, "n")}
	// The collision's rank-2 slot turns over entirely: neither TA nor TB reproduces.
	fresh := []artifact.Finding{freshJudged("judged-dup", "M0 sorts first"), freshJudged("judged-dup", "TC wholly new")}

	got := ReconcileJudged(fresh, existingFindings, backing)

	same := 0
	for _, f := range got.NotResurfaced {
		if f.ID == "judged-dup-collision-2" {
			same++
		}
	}
	if same != 2 {
		t.Fatalf("not-resurfaced = %+v, want both distinct-content priors under the shared suffixed id (the toxic shape)", got.NotResurfaced)
	}
	fm := &artifact.DeviationFrontmatter{
		Schema:        "verdi.deviation/v1",
		Covers:        strings.Repeat("a", 40),
		Findings:      got.Findings,
		NotResurfaced: got.NotResurfaced,
		Digest:        "sha256:" + strings.Repeat("0", 64),
	}
	if err := fm.Validate(); err == nil {
		t.Fatal("Validate accepted a duplicate not-resurfaced id — the residual must fail LOUDLY, never silently launder")
	}
}

// TestReconcileJudged_WordCollisionSlug_RendersOrdinaryCandidate is the
// candidate-path companion to the disposition-verb consumers-agree pin
// (spec/finding-identity judged-reserved-id-shape-substring-match): a fresh
// reworded finding at a bare slug that merely contains the WORD "collision" —
// this story's own judged-collision-cv-emission-order — gets an ordinary ac-1
// Candidate. ReconcileJudged has no machinery guard, so this path was always
// correct; the fix anchors the CLASSIFIER (artifact.IsCollisionMachineryID) so
// the OTHER consumers keyed off it — Validate's overlap relaxation and the
// disposition live path — agree that this bare slug is a candidate, not
// machinery (the L-N13 consumers-agree property restored).
func TestReconcileJudged_WordCollisionSlug_RendersOrdinaryCandidate(t *testing.T) {
	const slug = "judged-collision-cv-emission-order"
	fresh := []artifact.Finding{
		{ID: slug, Kind: artifact.FindingJudged, Text: "a reworded reading of the emission-order rule"},
	}
	existingNotResurfaced := []artifact.Finding{
		dispositionedJudged(slug, "the older ruling text", artifact.FindingAcceptedDeviation, "owner-ratified"),
	}
	got := ReconcileJudged(fresh, nil, existingNotResurfaced)
	if _, ok := got.Candidates[slug]; !ok {
		t.Fatalf("Candidates = %+v, want an ac-1 Candidate rendered for the bare word-\"collision\" slug", got.Candidates)
	}
	// The classifier AGREES with the candidate path: this is an ordinary bare
	// slug, never collision machinery. (Red before the anchoring fix: the bare
	// "-collision-" substring classified it true.)
	if artifact.IsCollisionMachineryID(slug) {
		t.Fatalf("%q classified as machinery — a word-\"collision\" bare slug is an ordinary candidate", slug)
	}
}
