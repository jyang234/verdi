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
// rule's "can't prove it, don't guess" posture (VL-010's own DiffBase=="".
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
		if result.State == specstate.AcceptedPendingBuild && len(result.Disclosures) > 0 {
			findings = append(findings, Finding{Rule: "VL-004", Path: d.RelPath, Severity: SeverityDisclosure, Message: strings.Join(result.Disclosures, "; ")})
		}
	}
	return findings
}
