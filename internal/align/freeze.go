package align

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// FreezeInPlace produces the frozen closure edition of an already-adjudicated
// living deviation-report VERBATIM — the faithful freeze `verdi close` needs
// (03 §Alignment report: "at closure, `verdi align --freeze` ... produces the
// final edition and it becomes frozen"). The living, human-dispositioned report
// IS that final edition; freezing stamps it permanent, it does not re-derive
// it. Every finding, disposition, and note is carried over exactly as the human
// left them; the judged exchange (Integrity + JudgeIntegrity) and every body
// section EXCEPT the two trailing spec/finding-identity sections are kept
// byte-for-byte; only the `frozen:` stamp is added. The two trailing sections
// (candidates awaiting reaffirmation, not-resurfaced) are re-rendered to agree
// with the finalized frontmatter (verify finding D — a frozen report is
// post-confirmation, so it has no pending candidates and its not-resurfaced:
// must equal the frontmatter's), see reconcileFrozenBody. That reconciliation
// touches body PROSE only; the digest is a pure function of the frontmatter's
// computed findings, never the body, so it stays valid (below).
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

	// Reconcile the body's two trailing spec/finding-identity sections with the
	// FINAL frontmatter before freezing (verify finding D): a frozen report is
	// post-confirmation, so it has no pending candidates and its not-resurfaced:
	// must equal frozen.NotResurfaced. See reconcileFrozenBody.
	body := reconcileFrozenBody(existingBody, &frozen)

	return &Report{
		Frontmatter: &frozen,
		Body:        body,
		Markdown:    RenderMarkdown(&frozen, body),
	}, nil
}

// reconcileFrozenBody returns body with its two trailing spec/finding-identity
// sections (candidates awaiting reaffirmation, ac-1; not-resurfaced, ac-3)
// re-rendered to agree with the final frozen frontmatter, keeping every earlier
// section (computed, boundary diff, diagram alignment, judged) byte-for-byte.
//
// Verify finding D: FreezeInPlace used to reattach the Generate-time body
// verbatim, so a report frozen AFTER its candidates were confirmed carried
// stale "### Candidates awaiting reaffirmation" and "## Not resurfaced" sections
// the finalized frontmatter no longer had — the archived judge-ergonomics report
// rendered each of its four findings THREE times (once live, once as a stale
// candidate, once as a stale backing), the body disagreeing with its own
// authoritative frontmatter. A frozen report is post-confirmation: it has no
// pending candidates (the section resets to "(none)") and its not-resurfaced:
// must show exactly fm.NotResurfaced. Only the two trailing sections can be
// reconstructed from the frontmatter alone — the earlier sections carry
// Generate-time data (baseline diffs, diagram alignment) that is not in the
// frontmatter — so they are kept verbatim and only the trailing two are re-rendered
// from the final state (renderTrailingSections, the same rule Generate renders
// them with).
//
// A body predating these sections (no candidatesSectionMarker) is returned
// verbatim: there is nothing to reconcile, and the digest — a pure function of
// (covers, computed finding id/kind/text, baseline diffs), never the body prose —
// is unaffected either way, so VerifyDigest still recomputes the same string.
func reconcileFrozenBody(body string, fm *artifact.DeviationFrontmatter) string {
	idx := strings.Index(body, candidatesSectionMarker)
	if idx == -1 {
		return body
	}
	var b strings.Builder
	b.WriteString(body[:idx])
	renderTrailingSections(&b, fm.Findings, nil, fm.NotResurfaced)
	return b.String()
}
