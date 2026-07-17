package align

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

// FreezeInPlace produces the frozen closure edition of an already-adjudicated
// living deviation-report VERBATIM — the faithful freeze `verdi close` needs
// (03 §Alignment report: "at closure, `verdi align --freeze` ... produces the
// final edition and it becomes frozen"). The living, human-dispositioned report
// IS that final edition; freezing stamps it permanent, it does not re-derive
// it. Every finding, disposition, and note is carried over exactly as the human
// left them; the judged exchange (Integrity + JudgeIntegrity) and the rendered
// body are kept byte-for-byte; only the `frozen:` stamp is added.
//
// The digest is REUSED unchanged, which is both correct and necessary: it is a
// pure function of (covers, computed-finding id/kind/text, baseline-diffs) and
// covers neither dispositions nor the frozen stamp (computed.go's digestInput),
// so a verbatim freeze leaves it valid — VerifyDigest still recomputes the same
// string. It is also the only recomputable-consistent choice here: the baseline
// diffs the digest is taken over are not stored in the frontmatter, so the
// digest cannot be recomputed from a decoded report alone; it can only be
// carried forward.
//
// Contrast Generate's --freeze path, which regenerates (re-Computes and re-runs
// the judge) before stamping: because the judge is non-reproducible (03
// §Alignment report), a re-run emits fresh content-hash finding identities that
// PreserveDispositions cannot match, silently dropping every disposition. That
// regeneration is the bug FreezeInPlace exists to avoid; the caller
// (cmd/verdi's runAlignForSpec) routes an eligible living report here instead.
//
// Preconditions (the caller establishes them — a living report that covers the
// freeze commit with every finding dispositioned; the merge gate already
// required exactly this before merge, 03 §Gates condition 3): existing is
// non-nil and not already frozen, and frozenAt is a YYYY-MM-DD stamp derived
// from the covered commit's own date (never wall clock). FreezeInPlace
// re-checks the structural preconditions and fails closed (CLAUDE.md: never
// fake success), it does not silently paper over a caller that violated them.
func FreezeInPlace(existing *artifact.DeviationFrontmatter, existingBody, frozenAt string) (*Report, error) {
	if existing == nil {
		return nil, fmt.Errorf("align: FreezeInPlace: existing report is required")
	}
	if existing.Frozen != nil {
		return nil, fmt.Errorf("align: FreezeInPlace: report is already frozen (at %s, commit %s); a frozen report is immutable", existing.Frozen.At, existing.Frozen.Commit)
	}
	if frozenAt == "" {
		return nil, fmt.Errorf("align: FreezeInPlace: frozenAt is required")
	}

	// Value copy so the caller's decoded report is never mutated. Findings and
	// the pointer fields (JudgeIntegrity, Provenance) share backing storage with
	// existing, but FreezeInPlace never writes through them — it only sets Frozen
	// on the copy — so the freeze is genuinely verbatim.
	frozen := *existing
	stamp := artifact.NewFrozen(frozenAt, existing.Covers)
	frozen.Frozen = &stamp

	// Self-validate the stamped frontmatter before handing it back as a valid
	// frozen report (CLAUDE.md: "never fake success").
	if err := frozen.Validate(); err != nil {
		return nil, fmt.Errorf("align: FreezeInPlace: stamped frontmatter failed self-validation: %w", err)
	}

	return &Report{
		Frontmatter: &frozen,
		Body:        existingBody,
		Markdown:    RenderMarkdown(&frozen, existingBody),
	}, nil
}
