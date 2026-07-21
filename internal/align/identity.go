package align

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/jyang234/verdi/internal/artifact"
)

// Identity returns f's stable content identity — the rule PreserveDispositions
// uses to decide whether a newly regenerated finding is "the same finding"
// as one in a prior report, eligible to inherit its disposition.
//
// Identity rule (PLAN.md Phase 8: "align preserves existing dispositions
// across regeneration when the finding is unchanged (match by stable
// finding identity — content hash or id; document the identity rule)"):
// Identity is a content hash over (Kind, ID, Text) — deliberately NOT ID
// alone. A declared boundary's finding id
// (boundary-<from>-<to>-<via>, computed.go) is stable across regenerations
// even when its VERDICT changes (holds -> violated): a human's "fixed"
// disposition made while the boundary held must never silently carry over
// once it starts failing — that would let a real regression hide behind a
// stale disposition. Folding Text into identity means ANY content change
// (a different verdict, a different witness, different judge wording under
// the same slugged id) is treated as a genuinely new finding: undispositioned
// until a human looks at it again — fail-closed, per CLAUDE.md's
// three-valued honesty ("silence is never a pass"). Disposition and Note
// are excluded from the hash on purpose: they are exactly the state being
// preserved, not part of what identifies the finding.
//
// spec/finding-identity (D6-35, ledger L-N2, adjudicated at Task 0 of the
// extensibility phase 2 design wave): this rule is UNCHANGED BYTE-FOR-BYTE
// by that story and remains the ONLY identity rule for a computed or
// conflict finding — the existing holds-to-violated negative test above
// keeps passing unmodified, exactly as this comment already required. What
// changed is layered ENTIRELY ON TOP, scoped to Kind == FindingJudged only
// (ReconcileJudged, reaffirm.go), because a judged finding's Text is the
// judge's own free-form prose: two runs of a real judge exchange over the
// identical underlying issue essentially never produce byte-identical text,
// so this Identity rule alone discards every judged disposition on every
// regeneration, whether or not the underlying issue changed (X-18's named
// second-order cost: a discarded disposition is simply re-recorded and
// re-accepted next pass, consuming fresh spec-stale budget for a standing,
// already-settled adjudication). The cure is NOT a second, looser identity
// rule for judged findings — silently matching by id alone would reopen
// exactly the stale-disposition hole this rule exists to close (03
// §508-512), letting a real escalation (a low-confidence cosmetic reading
// followed by a high-confidence real regression under the same slug) hide
// behind an inherited disposition. Instead: a judged finding's rule/
// boundary-derived id (the "slug", judge.go's tightened prompt contract)
// becomes an UNTRUSTED PRE-FILL HINT only. When THIS Identity function's
// exact match fails but the id still matches a prior dispositioned judged
// finding, ReconcileJudged pre-fills a CANDIDATE — the old ruling and old
// text rendered beside the new text — never a carried disposition; a human
// must confirm it, exactly as X-16 already requires for a fresh finding. A
// confirmed reaffirmation is recorded as an ordinary human disposition (this
// function plays no part in it) carrying artifact.Finding.CarriedFrom as
// provenance. So: Identity itself never changes, is never called with a
// looser formula, and is never bypassed for a computed or conflict finding —
// slug-primary matching is additive machinery that only ever produces a
// human-facing candidate, never an automatic disposition.
func Identity(f artifact.Finding) string {
	return identityOf(string(f.Kind), f.ID, f.Text)
}

// ConflictIdentity is Identity's decision-conflict-report analogue (03
// §Decision-conflict gate: "the same finding-identity rule" as the
// build-branch report — see Identity's own doc comment for why (kind, id,
// text) rather than id alone): the stable content identity
// PreserveConflictDispositions uses to decide whether a regenerated
// artifact.ConflictFinding is "the same finding" as one in a prior report,
// eligible to inherit its disposition (and, downstream, its computed
// CODEOWNERS routing — decision_judge.go).
func ConflictIdentity(f artifact.ConflictFinding) string {
	return identityOf(string(f.Kind), f.ID, f.Text)
}

// identityOf is the shared content-hash formula both Identity and
// ConflictIdentity apply over (kind, id, text) — the one hash rule this
// package uses for every finding shape it produces (CLAUDE.md: don't
// invent a second one).
func identityOf(kind, id, text string) string {
	h := sha256.New()
	h.Write([]byte(kind))
	h.Write([]byte{0})
	h.Write([]byte(id))
	h.Write([]byte{0})
	h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}

// PreserveDispositions carries a disposition (and note) forward from
// existing (a prior report's findings, nil/empty for a first run) to each
// entry of newFindings whose Identity matches — everything else (new
// findings, and findings whose content changed) is left as newFindings
// gave it (undispositioned, per computed.go/judged.go's own construction).
// Output preserves newFindings' order. Thin wrapper over preserve —
// see preserve's doc comment for the shared mechanics.
func PreserveDispositions(newFindings, existing []artifact.Finding) []artifact.Finding {
	return preserve(newFindings, existing, Identity, func(dst *artifact.Finding, src artifact.Finding) {
		dst.Disposition = src.Disposition
		dst.Note = src.Note
	})
}

// PreserveConflictDispositions is PreserveDispositions' decision-conflict-
// report analogue, matching by ConflictIdentity instead of Identity —
// otherwise byte-identical logic (03 §Decision-conflict gate: "the same
// ... finding-identity rule" as the build-branch report). Thin wrapper
// over preserve.
func PreserveConflictDispositions(newFindings, existing []artifact.ConflictFinding) []artifact.ConflictFinding {
	return preserve(newFindings, existing, ConflictIdentity, func(dst *artifact.ConflictFinding, src artifact.ConflictFinding) {
		dst.Disposition = src.Disposition
		dst.Note = src.Note
	})
}

// preserve is the one generic disposition-carry-forward mechanism
// PreserveDispositions and PreserveConflictDispositions both wrap: index
// existing by identity(f), then for each entry of new whose identity
// matches a prior entry, carry(dst, prior) copies forward whatever fields
// the caller considers "state being preserved" (disposition and note, for
// both current callers — see Identity's own doc comment for why the rest
// of a finding's content is deliberately part of the identity, not the
// carried state). Output preserves new's order and length.
func preserve[T any](new, existing []T, identity func(T) string, carry func(dst *T, src T)) []T {
	byIdentity := make(map[string]T, len(existing))
	for _, f := range existing {
		byIdentity[identity(f)] = f
	}

	out := make([]T, len(new))
	for i, f := range new {
		if prior, ok := byIdentity[identity(f)]; ok {
			carry(&f, prior)
		}
		out[i] = f
	}
	return out
}
