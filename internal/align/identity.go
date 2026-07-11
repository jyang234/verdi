package align

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/OWNER/verdi/internal/artifact"
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
// Output preserves newFindings' order.
func PreserveDispositions(newFindings, existing []artifact.Finding) []artifact.Finding {
	byIdentity := make(map[string]artifact.Finding, len(existing))
	for _, f := range existing {
		byIdentity[Identity(f)] = f
	}

	out := make([]artifact.Finding, len(newFindings))
	for i, f := range newFindings {
		if prior, ok := byIdentity[Identity(f)]; ok {
			f.Disposition = prior.Disposition
			f.Note = prior.Note
		}
		out[i] = f
	}
	return out
}

// PreserveConflictDispositions is PreserveDispositions' decision-conflict-
// report analogue, matching by ConflictIdentity instead of Identity —
// otherwise byte-identical logic (03 §Decision-conflict gate: "the same
// ... finding-identity rule" as the build-branch report).
func PreserveConflictDispositions(newFindings, existing []artifact.ConflictFinding) []artifact.ConflictFinding {
	byIdentity := make(map[string]artifact.ConflictFinding, len(existing))
	for _, f := range existing {
		byIdentity[ConflictIdentity(f)] = f
	}

	out := make([]artifact.ConflictFinding, len(newFindings))
	for i, f := range newFindings {
		if prior, ok := byIdentity[ConflictIdentity(f)]; ok {
			f.Disposition = prior.Disposition
			f.Note = prior.Note
		}
		out[i] = f
	}
	return out
}
