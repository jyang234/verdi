package align

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

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

	out := make([]artifact.Finding, 0, len(fresh)+1)
	candidates := make(map[string]JudgedCandidate)
	matched := make(map[string]bool, len(prior))

	// carryExactMatch applies identity.go's frozen exact-identity carry-forward
	// (ac-2) to f: if a prior dispositioned judged finding is byte-identical in
	// Kind+ID+Text, f inherits its Disposition/Note/CarriedFrom (a prior
	// CarriedFrom on an already-reaffirmed finding that keeps reproducing
	// byte-identically survives too) and that prior is marked resurfaced (so it
	// never also lands in NotResurfaced). This is the frozen rule itself, not
	// slug-matching — fail-closed is preserved by byte-identity — and EVERY
	// path that emits a fresh judged finding runs it, the collision branch
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
		matched[p.ID] = true
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
			// stable-within-this-run suffix. Disclosed limitation: because a
			// slug that collides has, by definition, no way to identify which
			// group member is "the same one" next run, a disambiguated id's
			// carry-forward is only as stable as the judge's own emission
			// order — the synthetic violation finding makes the situation
			// itself visible every time it recurs, which is the honest ceiling
			// here.
			// Each disambiguated member's id is stable-within-this-run, so a
			// byte-identical recurrence of THIS exact collision carries its
			// prior disposition on the frozen Kind+ID+Text rule — the id is
			// computed BEFORE the match so the disambiguated id is what
			// identity hashes over. A reworded member simply misses the match
			// and lands undispositioned, exactly as any non-identical judged
			// recurrence does (judged-judged-slug-collision-carry).
			for i, f := range group {
				if i > 0 {
					f.ID = fmt.Sprintf("%s-collision-%d", id, i+1)
				}
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

	var notResurfaced []artifact.Finding
	for _, p := range prior {
		if p.Kind != artifact.FindingJudged || !p.Dispositioned() {
			continue
		}
		if !matched[p.ID] {
			notResurfaced = append(notResurfaced, p)
		}
	}

	return JudgedReconciliation{Findings: out, Candidates: candidates, NotResurfaced: notResurfaced}
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
	return artifact.Finding{
		ID:   "judged-contract-violation-" + strings.TrimPrefix(id, "judged-"),
		Kind: artifact.FindingJudged,
		Text: fmt.Sprintf("judge contract violation: %d findings shared slug %q within one run — a rule/boundary-derived slug must be a stable per-finding-class identifier within a run (spec/finding-identity ac-4); texts: %s", len(group), id, strings.Join(texts, " | ")),
	}
}
