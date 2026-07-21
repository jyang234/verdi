package align

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// THE FINDING-IDENTITY TRUTH TABLE (spec/finding-identity, ledger L-N13)
//
// The complete stamp / carry / candidate behavior of the judged-reaffirmation
// machinery — ReconcileJudged (this file) and the disposition verb's live path
// (cmd/verdi/disposition.go) — enumerated over five axes, so the carried-from /
// collision space is closed BY SPECIFICATION rather than by another point fix.
// Every reachable cell is pinned: the RECONCILEJUDGED cells (does a finding
// carry a disposition, pre-fill a Candidate, or land in not-resurfaced) by
// TestReconcileJudged_TruthTable; the DISPOSITION-VERB cells (what a human
// confirmation writes) by the cmd/verdi tests named below. Impossible cells are
// named, each with the invariant that forbids it.
//
// AXES
//
//	source     fresh              a brand-new judged finding (RunJudged output)
//	           candidate          a single fresh finding under a BARE slug whose exact
//	                              identity missed but a same-id dispositioned prior
//	                              exists (ac-1)
//	           collision-member   one member of a within-run slug collision (ac-4)
//	           contract-violation the synthetic per-slug CV finding (ac-4)
//	           not-resurfaced     a prior dispositioned finding this round did not
//	                              re-emit; lives solely in not-resurfaced (ac-3)
//	id-class   bare <slug> | suffixed <slug><CollisionInfix><n> | reserved CV id.
//	           IsCollisionMachineryID is true for suffixed + reserved.
//	prior      none | live-dispositioned | not-resurfaced-AD | not-resurfaced-fixed
//	decision   (disposition verb) same-as-prior | differs | amend
//	recurrence byte-identical | reworded | reordered (a byte-identical member SET
//	           re-emitted in another order — canonical text ranking + the sorted CV
//	           join make it identical to byte-identical, L-N13 determinism)
//
// RECONCILEJUDGED CELLS — pinned by TestReconcileJudged_TruthTable
//
//	fresh, prior=none
//	    → undispositioned, NO Candidate, not in not-resurfaced (a plain new finding).
//	candidate (id-class=bare only), prior∈{live-dispositioned, not-resurfaced-AD,
//	not-resurfaced-fixed}, recurrence=reworded
//	    → undispositioned live + Candidate rendered (old ruling beside new text);
//	      the prior STAYS in not-resurfaced as the Candidate's backing record until a
//	      human confirms. Never an auto-carry (identity.go's frozen rule).
//	recurring-exact (any source/id-class), prior=live-dispositioned or
//	not-resurfaced-*, recurrence∈{byte-identical, reordered}
//	    → CARRIES the prior disposition/note/carried-from via the frozen Kind+ID+Text
//	      rule (carryExactMatch); NO Candidate; the matched prior is NOT in
//	      not-resurfaced. The ordinary ac-2 carry and the collision/CV carry alike —
//	      every member and the CV finding run the same carry.
//	collision-member or contract-violation, recurrence=reworded (the member's / CV's
//	text is no longer byte-identical)
//	    → undispositioned; NO Candidate (ac-4: a collision never pre-fills — the human
//	      resolves the slug's lineage); the prior lands in not-resurfaced under its
//	      own (suffixed / reserved) id.
//	not-resurfaced, this round does not re-emit it
//	    → PERSISTS verbatim in not-resurfaced across any number of further rounds
//	      (ac-3), budget-counted, until a human resolves it via the exit ramp.
//
// DISPOSITION-VERB CELLS (carried-from on human confirmation) — pinned in cmd/verdi
//
//	confirm a TRUE candidate (id-class=bare, a Candidate was rendered):
//	    decision=same    → REAFFIRMATION: stamp carried-from=<covers>; remove backing.
//	                       [TestRunDisposition_ConfirmsCandidate_Reaffirmation_StampsCarriedFrom]
//	    decision=differs → ESCALATION: no stamp; remove backing (superseded).
//	                       [TestRunDisposition_ConfirmsCandidate_Escalation_NoCarriedFrom]
//	    decision=amend   → recompute: a differing decision clears an inherited stamp,
//	                       a note-only amend keeps it.
//	                       [TestRunDisposition_Amend_RecomputesCarriedFrom]
//	confirm a COLLISION-MEMBER or CONTRACT-VIOLATION live finding sharing an id with a
//	not-resurfaced backing record (IsCollisionMachineryID; NO Candidate was rendered),
//	ANY decision:
//	    → disposition the LIVE finding; NEVER stamp; LEAVE the backing record for its
//	      exit ramp; disclose. (L-N13 presentation-predicated resolution)
//	      [TestRunDisposition_CollisionMember_SuffixedBackingShadow_NoLivePathReaffirmation,
//	       TestRunDisposition_ContractViolation_BackingShadow_NoLivePathReaffirmation]
//	resolve a not-resurfaced entry via the EXIT RAMP (no live finding at its id):
//	    →fixed  → RELEASE: remove; budget releases.  AD→AD → REAFFIRMATION: stamp.
//	    fixed→AD → REVERSAL: no stamp; prior-ruling lineage named. [dispositionNotResurfaced]
//	confirm an ORDINARY live finding with NO same-id not-resurfaced entry:
//	    → dispositioned; never a carried-from stamp (nothing to reaffirm).
//	      [TestRunDisposition_OrdinaryFinding_NoNotResurfacedEntry_Unaffected]
//
// IMPOSSIBLE CELLS (named, with the forbidding invariant)
//
//	candidate × id-class=suffixed — a Candidate is pre-filled ONLY for a single fresh
//	    finding under a bare slug; a collision (the sole source of suffixed ids) never
//	    yields a Candidate (ac-4), so no suffixed id is ever a Candidate.
//	candidate × recurrence=byte-identical — a byte-identical recurrence is an exact
//	    carry; the exact-identity match precedes the slug-only candidate path.
//	bare collision-member × same-id backing record — when a colliding slug owns a
//	    backing record collisionMemberIDs suffixes EVERY member (hasBacking), so no
//	    live member keeps the bare id; the bare id is the backing record's alone
//	    (judged-collision-backing-regeneration-drain / -same-round).
//	contract-violation × id-class=bare — the CV id is always the reserved
//	    ContractViolationIDPrefix shape, never a bare judge slug.
//	fresh × prior≠none — "fresh" means no prior exists at the id, by definition.
//
// DOWNSTREAM RESIDUAL (disclosed; loud, never silent). Once a human has confirmed a
// live collision member and left a distinct-content backing record at the same
// suffixed id (the sanctioned shape), a later round in which BOTH that member's text
// AND the backing record's text fail to reproduce would place two distinct-content
// entries under one suffixed id in not-resurfaced. That is rejected LOUDLY by
// Validate (a duplicate not-resurfaced id fails the report's self-validation) — never
// a silent laundering — and is the human's cue to resolve the backing record via its
// exit ramp once the collision clears (its first bare-of-live-members round). All
// byte-identical / reworded / reordered recurrences are sound (matched by content
// identity), so this is a narrow multi-reword edge, disclosed here.

// JudgedCandidate is spec/finding-identity ac-1's pre-fill context: the
// prior dispositioned finding a fresh, reworded judged finding's slug (its
// id — judge.go's rule/boundary-derived, tightened prompt contract) matches,
// carried alongside the fresh finding's own new text so a human sees
// exactly what changed before deciding anything. A Candidate is explicitly
// NOT a disposition (identity.go's frozen rule is never bypassed for it).
//
// Deliberately absent from the on-disk schema: ReconcileJudged recomputes
// every Candidate fresh, every call, from existingFindings/
// existingNotResurfaced — so a Candidate survives any number of unconfirmed
// `verdi align` regenerations exactly as durably as the not-resurfaced entry
// it is derived from, with no separate persisted field that could drift out
// of agreement with it.
type JudgedCandidate struct {
	OldDisposition artifact.FindingDisposition
	OldText        string
	OldNote        string
	// ArchiveSource, when non-empty, marks a CROSS-LEVEL candidate (ledger L-N14
	// companion): a fresh FEATURE-level judged finding whose slug matched no
	// feature-report prior but matched a dispositioned ruling in a CLOSED,
	// non-superseded implementing story's ARCHIVED report. It holds that archive's
	// spec ref (e.g. "spec/judge-ergonomics"), rendered beside the candidate so a
	// human sees the ruling's archive origin before confirming. Empty for an
	// ordinary same-report candidate. A cross-level candidate is still NOT a
	// disposition (identity.go's frozen rule is never bypassed); the archived
	// ruling is seated as its backing record so the ordinary human-confirms path
	// (cmd/verdi's disposition verb) stamps carried-from on confirmation.
	ArchiveSource string
}

// ArchivedRuling pairs a dispositioned judged finding drawn from a CLOSED,
// non-superseded implementing story's ARCHIVED deviation report (its findings: or
// not-resurfaced: section) with the archive it came from — the cross-level
// reaffirmation source (ledger L-N14 companion, the D6-35 slug-drift residual's
// cross-level case). Source is the archived spec ref (e.g. "spec/judge-ergonomics").
// Threaded into Generate only for a feature-context align (cmd/verdi/align.go
// gathers them; story aligns pass none), and consumed by applyArchivedRulings.
type ArchivedRuling struct {
	Finding artifact.Finding
	Source  string
}

// JudgedReconciliation is ReconcileJudged's output.
type JudgedReconciliation struct {
	// Findings mirrors fresh's own findings — each either carrying its prior
	// disposition (and CarriedFrom) forward on an EXACT identity match
	// (identity.go's frozen rule, ac-2, unaffected by this story),
	// undispositioned with a paired entry in Candidates (a slug-only match,
	// ac-1), or undispositioned with neither (a genuinely new finding) —
	// plus one synthetic, disclosed contract-violation finding per colliding
	// slug (ac-4), appended after fresh's own entries in fresh's id order.
	Findings []artifact.Finding
	// Candidates is keyed by finding id, populated only for a slug-only
	// match (ac-1's pre-fill, ac-2's escalation case) — never for an exact
	// match (nothing to compare; already carried) and never for a colliding
	// slug (ambiguous which of the collision's members, if either,
	// continues the slug's lineage — the human resolves it, ac-4).
	Candidates map[string]JudgedCandidate
	// NotResurfaced is every prior dispositioned judged finding — drawn from
	// BOTH existingFindings and existingNotResurfaced, so an entry already
	// persisted there stays persisted across any number of further
	// non-reproducing rounds — that fresh does not resurface under an EXACT
	// identity match this round (ac-3). A finding resurfacing only as a
	// slug-match Candidate STAYS listed here too: it is the Candidate's own
	// backing record, removed only by a human's explicit confirmation
	// (cmd/verdi's disposition verb), never by ReconcileJudged itself.
	NotResurfaced []artifact.Finding
}

// ReconcileJudged implements spec/finding-identity's whole judged-
// disposition reaffirmation mechanism (ledger L-N2, as adjudicated at the
// extensibility phase 2 design wave's Task 0): fresh is this run's freshly
// computed judged findings (RunJudged's own output, always undispositioned);
// existingFindings/existingNotResurfaced are the prior report's own
// findings: and not-resurfaced: sections (report.go's Generate reads them
// from Input.ExistingFindings/Input.ExistingNotResurfaced, both filtered to
// Kind == FindingJudged before calling here — though this function also
// filters defensively itself, see below).
//
// Scoped to judged findings ONLY: a computed or conflict finding keeps using
// PreserveDispositions/PreserveConflictDispositions (identity.go) entirely
// unchanged — this function never reasons about them, matching identity.go's
// own doc comment ("slug-primary matching ... branches on Kind ==
// FindingJudged only").
func ReconcileJudged(fresh, existingFindings, existingNotResurfaced []artifact.Finding) JudgedReconciliation {
	prior := make([]artifact.Finding, 0, len(existingFindings)+len(existingNotResurfaced))
	prior = append(prior, existingFindings...)
	prior = append(prior, existingNotResurfaced...)

	// Only a dispositioned judged finding is a prior "ruling" worth
	// reaffirming or persisting — an undispositioned prior (should not occur
	// on a report that ever reached a freeze-eligible state, closuregate.go
	// condition 4) has nothing to carry or pre-fill.
	priorByIdentity := make(map[string]artifact.Finding, len(prior))
	priorByID := make(map[string]artifact.Finding, len(prior))
	for _, f := range prior {
		if f.Kind != artifact.FindingJudged || !f.Dispositioned() {
			continue
		}
		priorByIdentity[Identity(f)] = f
		priorByID[f.ID] = f
	}

	// backingByID marks each slug that owns a not-resurfaced backing record this
	// run, so that when such a slug ALSO collides among fresh findings,
	// collisionMemberIDs suffixes EVERY member — the backing record alone keeps
	// the bare slug (judged-collision-backing-regeneration-drain): its exit ramp
	// stays reachable and the id-keyed NotResurfaced rebuild below can never mark
	// it resurfaced by matching a live member that merely shares its bare id.
	//
	// A slug owns a backing record in EITHER of two ways, both detected here:
	//
	//   (1) an entry already sitting in existingNotResurfaced (a record a prior
	//       round already persisted) — filled unconditionally just below; and
	//
	//   (2) a dispositioned judged prior BORN into not-resurfaced THIS round: a
	//       live prior at slug S in existingFindings that a fresh collision at S
	//       reproduces on none of the members keeping the bare id — filled after
	//       the fresh grouping below (judged-collision-backing-same-round).
	//
	// Source (2) is the round the record is born, and it was the hole: with only
	// (1), hasBacking was false that exact round, the first-emitted member kept
	// bare S, and the unmatched prior landed in not-resurfaced under the same
	// bare S — the forbidden live-member-shadows-backing overlap. Validate
	// rejects that overlap once on disk, so it could only ever be an in-flight
	// computation state, which (2) now closes.
	backingByID := make(map[string]bool, len(existingNotResurfaced))
	for _, f := range existingNotResurfaced {
		if f.Kind == artifact.FindingJudged && f.Dispositioned() {
			backingByID[f.ID] = true
		}
	}

	// Group fresh by id first (ac-4's collision rule): a slug shared by 2+
	// fresh findings this round is the judge violating its own contract —
	// disclosed as its own finding, never silently deduped or arbitrarily
	// paired with a Candidate. order preserves fresh's own first-seen id
	// order for deterministic output.
	order := make([]string, 0, len(fresh))
	byID := make(map[string][]artifact.Finding, len(fresh))
	for _, f := range fresh {
		if f.Kind != artifact.FindingJudged {
			continue
		}
		if _, ok := byID[f.ID]; !ok {
			order = append(order, f.ID)
		}
		byID[f.ID] = append(byID[f.ID], f)
	}

	// backingByID source (2) (judged-collision-backing-same-round): a colliding
	// slug whose backing record is BORN this round. priorByID[id] is a
	// dispositioned judged prior at the bare slug (existingFindings or
	// existingNotResurfaced — the latter already covered unconditionally above).
	// Under the no-backing scheme exactly ONE member keeps the bare id: the
	// lowest-text-ranked member (collisionMemberIDs' text-rank-0, NOT the
	// incidental first-emitted — L-N13's determinism contract), which
	// carryExactMatch carries the prior forward onto only if their text matches.
	// When it does NOT, the prior is unmatched — it lands in not-resurfaced at the
	// bare slug while a different live member sits on that same bare slug, the
	// forbidden overlap. So mark the slug backed whenever the bare-id member would
	// not carry the prior: every member is then suffixed and the newly-born
	// backing record stands alone under the bare id. Consulting the canonical
	// min-text member (never group[0]) keeps this born-this-round decision
	// emission-order-independent, exactly like the id assignment it guards.
	for _, id := range order {
		group := byID[id]
		if len(group) < 2 {
			continue
		}
		if p, ok := priorByID[id]; ok && minMemberText(group) != p.Text {
			backingByID[id] = true
		}
	}

	out := make([]artifact.Finding, 0, len(fresh)+1)
	candidates := make(map[string]JudgedCandidate)
	matched := make(map[string]bool, len(prior))

	// carryExactMatch applies identity.go's frozen exact-identity carry-forward
	// (ac-2) to f: if a prior dispositioned judged finding is byte-identical in
	// Kind+ID+Text, f inherits its Disposition/Note/CarriedFrom (a prior
	// CarriedFrom on an already-reaffirmed finding that keeps reproducing
	// byte-identically survives too) and that prior is marked resurfaced by its
	// CONTENT IDENTITY (so it never also lands in NotResurfaced). Keying matched
	// by Identity — not the bare id — is load-bearing once a dispositioned
	// collision member coexists with a DISTINCT-content backing record at the
	// SAME suffixed id (the confirmed-collision-member shape, judged-collision-
	// suffixed-backing-shadow): carrying the reproducing member must mark only
	// THAT identity resurfaced, never conflate and silently drain the same-id
	// backing record whose own text did not reproduce. This is the frozen rule
	// itself, not slug-matching — fail-closed is preserved by byte-identity — and
	// EVERY path that emits a fresh judged finding runs it, the collision branch
	// included (judged-judged-slug-collision-carry); the single source of the
	// carry so no path drifts from another.
	carryExactMatch := func(f artifact.Finding) (artifact.Finding, bool) {
		p, ok := priorByIdentity[Identity(f)]
		if !ok {
			return f, false
		}
		f.Disposition = p.Disposition
		f.Note = p.Note
		f.CarriedFrom = p.CarriedFrom
		matched[Identity(p)] = true
		return f, true
	}

	for _, id := range order {
		group := byID[id]
		if len(group) > 1 {
			// ac-4's collision rule is "never dedupe" — never silently
			// collapse the group into one entry — but deviation-report.md's
			// pre-existing schema (internal/artifact.DeviationFrontmatter.
			// Validate) requires every finding id to be unique within one
			// report, load-bearing well beyond this story (e.g. disposition.go's
			// whole-line body-text matching keys off it). Disambiguating each
			// colliding member's id (rather than keeping N findings sharing one
			// id, which would simply fail to decode) is the way to keep every
			// one of them independently visible and independently
			// dispositionable — nothing is merged or lost, only the id gains a
			// stable-within-this-run suffix (collisionMemberIDs). Its id is
			// computed BEFORE the match, so a byte-identical recurrence carries
			// its prior disposition on the frozen Kind+ID+Text rule and a
			// reworded member misses the match and lands undispositioned, exactly
			// as any non-identical judged recurrence does
			// (judged-judged-slug-collision-carry).
			for _, f := range collisionMemberIDs(id, group, backingByID[id]) {
				carried, _ := carryExactMatch(f)
				out = append(out, carried)
			}
			// The synthetic contract-violation finding is deterministic given
			// a deterministic group, so a recurring collision's OWN disposition
			// survives via the same exact-identity carry — making
			// contractViolationFinding's doc claim true.
			cv, _ := carryExactMatch(contractViolationFinding(id, group))
			out = append(out, cv)
			continue
		}

		f := group[0]
		if carried, ok := carryExactMatch(f); ok {
			// Exact content match: identity.go's frozen rule already says
			// this is "the same finding" — ordinary carry-forward, ac-2.
			out = append(out, carried)
			continue
		}
		if p, ok := priorByID[id]; ok {
			// Slug-only match: a Candidate, ac-1 — never silently carried.
			// Deliberately NOT marked matched: the prior stays in
			// NotResurfaced as this Candidate's own backing record until a
			// human confirms it (cmd/verdi's disposition verb removes it
			// then), so the pre-fill survives any number of further
			// unconfirmed regenerations with no new persisted field needed.
			candidates[id] = JudgedCandidate{OldDisposition: p.Disposition, OldText: p.Text, OldNote: p.Note}
		}
		out = append(out, f)
	}

	// A prior lands in not-resurfaced iff its exact CONTENT IDENTITY (not merely
	// its id) failed to reproduce this round — so two priors sharing a suffixed
	// id (a carried member and its distinct-content backing record) are tracked
	// independently and only the genuinely-vanished one persists (judged-
	// collision-suffixed-backing-shadow).
	var notResurfaced []artifact.Finding
	for _, p := range prior {
		if p.Kind != artifact.FindingJudged || !p.Dispositioned() {
			continue
		}
		if !matched[Identity(p)] {
			notResurfaced = append(notResurfaced, p)
		}
	}

	return JudgedReconciliation{Findings: out, Candidates: candidates, NotResurfaced: notResurfaced}
}

// applyArchivedRulings layers cross-level re-recording awareness (ledger L-N14
// companion) ON TOP of a JudgedReconciliation computed from the FEATURE report's
// OWN priors, for a feature-context align only (recon's own callers pass the
// gathered implementing-story archives; a story align passes none). It never
// touches ReconcileJudged's within-report truth table: it only acts on a fresh
// feature finding ReconcileJudged already left as a plain NEW finding — no
// feature-report prior at its slug, so no exact carry and no same-report
// candidate — that a CLOSED, non-superseded implementing story's ARCHIVED report
// dispositioned under the same judged slug. For each such finding it pre-fills a
// CANDIDATE citing the archive (JudgedCandidate.ArchiveSource) and seats the
// archived ruling in NotResurfaced as that candidate's backing record, so the
// ordinary human-confirms path (cmd/verdi's disposition verb) stamps carried-from
// on confirmation — never an auto-carry (identity.go's frozen rule is never
// bypassed), same discipline as a same-report candidate, nothing silent.
//
// Feature-report priors ALWAYS take precedence: a slug ReconcileJudged already
// resolved (an exact carry, so f.Dispositioned; or a same-report candidate, so
// recon.Candidates[f.ID] is set) is skipped — the feature's own prior, not an
// archive, governs it. Collision-machinery ids never pre-fill (ac-4). Because a
// resolved slug is skipped, the seated backing's id never collides with a
// feature-report prior already in NotResurfaced — the candidate+backing shape
// Validate permits (an undispositioned live finding beside its same-id backing).
func applyArchivedRulings(recon JudgedReconciliation, archived []ArchivedRuling) JudgedReconciliation {
	if len(archived) == 0 {
		return recon
	}

	// Index archived dispositioned judged rulings by slug (id). The caller sorts
	// `archived` deterministically (by source, then id), so first-per-slug wins
	// deterministically when two implementing stories archived the same slug.
	archByID := make(map[string]ArchivedRuling, len(archived))
	for _, a := range archived {
		if a.Finding.Kind != artifact.FindingJudged || !a.Finding.Dispositioned() {
			continue
		}
		if _, ok := archByID[a.Finding.ID]; !ok {
			archByID[a.Finding.ID] = a
		}
	}

	candidates := recon.Candidates
	if candidates == nil {
		candidates = make(map[string]JudgedCandidate)
	}
	notResurfaced := recon.NotResurfaced

	for _, f := range recon.Findings {
		// Feature-report priors always take precedence, in fresh order for
		// determinism: an exact carry (dispositioned) or a same-report candidate is
		// already resolved by the feature's own prior; a collision member/CV never
		// pre-fills (ac-4). Only a genuinely NEW feature finding consults the archives.
		if f.Dispositioned() {
			continue
		}
		if _, ok := candidates[f.ID]; ok {
			continue
		}
		if artifact.IsCollisionMachineryID(f.ID) {
			continue
		}
		a, ok := archByID[f.ID]
		if !ok {
			continue
		}
		// A cross-level slug match: pre-fill a candidate citing the archive and seat
		// the archived ruling as its backing record, so the ordinary human-confirms
		// path (cmd/verdi's disposition verb) stamps carried-from on confirmation —
		// never an auto-carry (the fresh finding stays undispositioned). The seated
		// id equals f.ID; f is undispositioned and (skipped above) owns no
		// feature-report prior at this slug, so the candidate+backing shape validates
		// and the seated id is unique in not-resurfaced.
		candidates[f.ID] = JudgedCandidate{
			OldDisposition: a.Finding.Disposition,
			OldText:        a.Finding.Text,
			OldNote:        a.Finding.Note,
			ArchiveSource:  a.Source,
		}
		notResurfaced = append(notResurfaced, a.Finding)
	}

	recon.Candidates = candidates
	recon.NotResurfaced = notResurfaced
	return recon
}

// minMemberText returns the lexicographically smallest Text among group's
// members — the text that, under the no-backing collision scheme, lands on the
// bare slug (collisionMemberIDs' text-rank-0 member). backingByID source (2)
// consults it (never group[0], the incidental first-emitted member) so the
// born-this-round backing decision is emission-order-independent, exactly like
// the id assignment it guards (L-N13).
func minMemberText(group []artifact.Finding) string {
	m := group[0].Text
	for _, f := range group[1:] {
		if f.Text < m {
			m = f.Text
		}
	}
	return m
}

// collisionMemberIDs assigns a disambiguated id to each member of a within-run
// slug collision (a slug 2+ fresh judged findings shared this run). Every id is
// assigned by TEXT RANK — members sorted by text, ties broken by emission index
// (SliceStable over an identity-initialized index slice) — so the id->text
// pairing is a function of member CONTENT, never the judge's incidental
// emission order (L-N13's determinism contract, judged-collision-cv-emission-
// order): a byte-identical recurrence of the same member set reproduces the
// same ids regardless of how the judge orders its output, so each member then
// carries its prior disposition on the frozen Kind+ID+Text rule (carryExactMatch)
// no matter how the output was reshuffled. The returned slice stays in fresh's
// original emission order — only ids are rewritten — so RenderBody's finding
// order stays the judge's own under both schemes. artifact.CollisionInfix is the
// shared schema seam for the reserved id shape, never duplicated across packages.
//
// hasBacking selects only WHICH ranks get suffixed, never the ranking itself:
//
//   - hasBacking == false (judged-judged-slug-collision-carry): the
//     lowest-text-ranked member keeps the bare slug and each higher-ranked
//     member becomes "<slug><CollisionInfix><n>" (n from 2, by text rank).
//     There is no backing record on the bare slug to shadow, so keeping a live
//     member on it is safe and — because the bare id now follows text, not
//     emission — an already-dispositioned bare member's id never churns on a
//     reorder.
//
//   - hasBacking == true (judged-collision-backing-regeneration-drain): NO
//     member keeps the bare slug — the not-resurfaced backing record alone owns
//     it, so the backing record's own exit ramp (cmd/verdi's disposition verb)
//     stays reachable while the collision persists AND ReconcileJudged's
//     NotResurfaced rebuild can never mark the backing record resurfaced by
//     matching a live member that merely shares its bare id. Every member is
//     suffixed by text rank. (Disclosed edge: if a human resolves the backing
//     record between runs, hasBacking flips to false and the scheme reverts to
//     bare-base — the affected members land undispositioned in not-resurfaced,
//     still budget-counted, never silently dropped.)
func collisionMemberIDs(slug string, group []artifact.Finding, hasBacking bool) []artifact.Finding {
	out := make([]artifact.Finding, len(group))
	copy(out, group)
	rank := make([]int, len(out))
	for i := range rank {
		rank[i] = i
	}
	sort.SliceStable(rank, func(a, b int) bool {
		return out[rank[a]].Text < out[rank[b]].Text
	})
	for r, idx := range rank {
		if !hasBacking && r == 0 {
			continue // lowest-text member keeps the bare slug (no backing to shadow)
		}
		out[idx].ID = fmt.Sprintf("%s%s%d", slug, artifact.CollisionInfix, r+1)
	}
	return out
}

// contractViolationFinding synthesizes ac-4's disclosed judge-contract-
// violation finding for a slug two or more fresh findings shared within one
// run — a rule/boundary-derived slug is defined to be a stable
// per-finding-class identifier WITHIN a run (judge.go's tightened prompt
// contract), so two findings landing on the same one is the judge itself
// violating its own contract, never a signal to silently dedupe (which would
// hide which of the two a human actually dispositioned). Deterministic given
// a deterministic group (id + member order/content), so a recurring
// collision's OWN disposition survives via ordinary exact-identity matching
// on this synthetic finding, exactly like any other judged finding: the
// collision branch runs the same priorByIdentity carry-forward
// (carryExactMatch) over this finding and every disambiguated member that
// every other path applies (judged-judged-slug-collision-carry) — so this
// claim holds without special-casing the not-resurfaced bookkeeping.
func contractViolationFinding(id string, group []artifact.Finding) artifact.Finding {
	texts := make([]string, len(group))
	for i, f := range group {
		texts[i] = f.Text
	}
	// Join over CANONICALLY-SORTED texts so this synthetic finding's Text is a
	// function of the member text SET, never the judge's incidental emission
	// order (L-N13, judged-collision-cv-emission-order). A byte-identical member
	// set re-emitted in a swapped order then reproduces this finding
	// byte-for-byte, and its own disposition survives via ordinary exact-identity
	// matching (carryExactMatch) — making this function's determinism claim
	// hold under reorder, not only under a stable emission order.
	sort.Strings(texts)
	return artifact.Finding{
		ID:   artifact.ContractViolationIDPrefix + strings.TrimPrefix(id, "judged-"),
		Kind: artifact.FindingJudged,
		Text: fmt.Sprintf("judge contract violation: %d findings shared slug %q within one run — a rule/boundary-derived slug must be a stable per-finding-class identifier within a run (spec/finding-identity ac-4); texts: %s", len(group), id, strings.Join(texts, " | ")),
	}
}
