package lint

import (
	"fmt"
	"os"
	"strings"

	"github.com/jyang234/verdi/internal/specstate"
)

// vl004 enforces the legacy-draft COMPATIBILITY reading of merge-signaled
// spec acceptance (docs/superpowers/specs/2026-08-01-merge-signals-spec-
// acceptance-design.md, "Artifact compatibility"): a spec's persisted
// `status:` field is no longer the source of truth for whether it is a
// live proposal or already accepted — internal/specstate's Git-derived
// projection is. This rule's own job narrows to exactly one thing: when a
// legacy `status: draft` document's exact bytes are ALREADY reachable
// from the configured default branch, disclose the compatibility reading
// (merge-accepted, never an active draft) rather than staying silent or
// misreporting it. It never re-litigates a bare proposal (never yet
// landed) as a violation — landing legality is the forge ruleset's job
// (the design's "Pull-request gates" section), not this rule's.
//
// Only evaluated at the default-branch PR boundary (Context.
// TargetsDefaultBoundary, I-14's original VL-004 scoping, kept verbatim):
// off that boundary — an ordinary design-branch lint — this rule says
// nothing at all, matching the old EnforceDraftGate posture exactly.
//
// An UNRESOLVABLE default branch is never a silent pass while a CI run is
// actually attempting to enforce the boundary (InCI with a known
// TargetBranch — the shape a PR pipeline always has): the old
// EnforceDraftGate-gated code silently fell through to "not enforced"
// whenever DefaultBranch was empty, which is exactly the fail-OPEN gap a
// prior phase's review adjudication named (three-valued honesty,
// constitution 2 — "cannot be proven" must never read as "passed"). A
// disclosure naming the missing proof is the fix; a plain local run with
// no CI signal at all still stays silent, matching every other git-aware
// rule's "can't prove it, don't guess" posture (VL-010's own DiffBase=="").
//
// The SAME "never a silent pass" posture applies once specstate is
// actually consulted, per doc, below: specstate.Result.State ==
// specstate.Unproven is exactly the same "cannot be proven" shape as an
// unresolvable default branch, just discovered one level deeper (a
// provable default branch whose reachability/supersession proof for THIS
// path still fails — e.g. no first-parent landing commit provable, or the
// default-branch active-spec corpus scan is itself incomplete). Falling
// through silently there would be the identical fail-open gap the
// unresolvable-default-branch fix above closes, just moved one call
// deeper — Check's per-document switch below handles it explicitly,
// alongside every other reachable specstate.State, so no state is ever
// dropped without a deliberate, commented reason.
type vl004 struct{}

func (vl004) ID() string { return "VL-004" }

func (vl004) Check(in *RunInput) []Finding {
	if in.LintCtx.DefaultBranch == "" {
		if in.LintCtx.InCI && in.LintCtx.TargetBranch != "" {
			return []Finding{{
				Rule:     "VL-004",
				Severity: SeverityDisclosure,
				// vocab:identity — machinery diagnostic naming the missing proof, not a verdict
				Message: fmt.Sprintf("this run targets %q in CI, but no default branch could be resolved (no CI_DEFAULT_BRANCH, no configured origin/HEAD, no unambiguous local origin/main or origin/master): legacy status: draft compatibility on the default branch cannot be proven", in.LintCtx.TargetBranch),
			}}
		}
		return nil
	}
	if !in.LintCtx.TargetsDefaultBoundary() {
		return nil
	}

	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Kind != "spec" {
			continue
		}
		if d.Status != "draft" {
			continue // statusless and every other legacy value need no compatibility check here
		}

		content, err := os.ReadFile(d.Path)
		if err != nil {
			findings = append(findings, Finding{Rule: "VL-004", Path: d.RelPath, Message: fmt.Sprintf("reading %s: %v", d.RelPath, err)})
			continue
		}
		result, err := in.Projector.Resolve(in.Ctx, in.Root, specstate.Candidate{Path: d.RelPath, Content: content})
		if err != nil {
			findings = append(findings, Finding{Rule: "VL-004", Path: d.RelPath, Message: fmt.Sprintf("resolving Git-derived state for %s: %v", d.RelPath, err)})
			continue
		}

		// Every reachable specstate.State is handled explicitly below —
		// none may fall through silently (fix-round-1 finding 1: an
		// Unproven verdict is exactly as much a "cannot be proven, never a
		// silent pass" case as the unresolvable-default-branch guard
		// above, just discovered one level deeper).
		switch result.State {
		case specstate.AcceptedPendingBuild:
			// The compatibility case this rule exists for: exact bytes
			// already reachable from default, legacy draft. Disclosures
			// is non-empty here exactly when migrationDisclosures fired
			// (probeLegacyStatus read "draft" on this same content) — the
			// guard is defensive, not a second decision.
			if len(result.Disclosures) > 0 {
				findings = append(findings, Finding{Rule: "VL-004", Path: d.RelPath, Severity: SeverityDisclosure, Message: strings.Join(result.Disclosures, "; ")})
			}
		case specstate.Unproven:
			// Reachability or supersession-completeness could not be
			// proven for this exact path (no provable first-parent
			// landing commit, or an incomplete default-branch active-spec
			// scan) — never a silent pass. Disclose the missing proof
			// specstate itself names.
			findings = append(findings, Finding{Rule: "VL-004", Path: d.RelPath, Severity: SeverityDisclosure, Message: strings.Join(result.Disclosures, "; ")})
		case specstate.Proposed:
			// Recorded accepted reading (fix-round-1 finding 1's own
			// scope): the default branch either doesn't have this path at
			// all (RelationNew — a live proposal still under review) or
			// holds different bytes at it (RelationDiverged — a proposal
			// whose reviewed head has moved past what's on default). Both
			// are ordinary, unremarkable proposal states, not a
			// compatibility question about an ALREADY-LANDED draft; this
			// rule's own job (its doc comment above) is narrowly that one
			// compatibility question, so neither needs a finding.
		case specstate.Superseded, specstate.Closed:
			// A legacy `status: draft` document whose exact bytes
			// specstate nonetheless proves reachable at a computed
			// TERMINAL state (a later successor names it as predecessor,
			// or it sits in the archive zone) is not a live "is this
			// really still a draft" compatibility question either — its
			// Git-derived state is already unambiguous and further along
			// than "merge-accepted." Nothing to disclose.
		}
	}
	return findings
}
