// verdi gate's spec-MR review-thread condition (V1-P7; 05 §CLI's gate
// row: "on spec MRs additionally blocks on unresolved review threads
// (resolved-or-graduated, §Review stickies and forge round-trip)").
//
// WIRING: joins gate_decisionconflict.go's runSpecMRGate as its second
// spec-MR condition — ONE call site, exactly as that file's own doc
// comment anticipated ("V1-P7's review-thread condition joins this same
// set later"). No restructuring of gate.go/gate_decisionconflict.go
// beyond that one call site plus the forge/default-branch plumbing this
// file adds to runSpecMRGate.
package main

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/OWNER/verdi/internal/forge"
	"github.com/OWNER/verdi/internal/gitx"
)

// checkReviewThreadsCondition evaluates 05's spec-MR review-thread
// readiness rule.
//
// JUDGMENT CALL / DISCLOSED GAP — recorded at this phase's review as
// R4-I-28 (PLAN-V1.md §7), per this phase's brief ("if 05's text
// under-determines how a thread proves it 'points at a spec commit',
// STOP -> NEEDS_CONTEXT rather than inventing"): 05 §Review stickies and
// forge round-trip states the readiness rule at two different levels of
// mechanical determinacy —
//
//  1. "Spec-MR readiness requires all review threads resolved —
//     forge-native resolution state, deterministic on both GitLab and
//     GitHub." Mechanically checkable: GetThreadResolution's Resolved
//     field, forge-native, queried per SUBSTANTIVE thread only
//     (comments.go's ThreadResolution doc comment operationalizes 05's
//     "substantive" as "a thread the forge itself treats as resolvable" —
//     both adapters already exclude bare/general comments from this
//     population).
//  2. "Resolving a substantive thread must either point at a spec commit
//     that addressed it, or mint a declared open-question or constraint
//     object on the spec — the resolved-or-graduated rule." This second
//     sentence describes what a reviewer's resolve action SHOULD mean,
//     but 05 (and 02's comment-token/annotation schemas) define no
//     mechanism connecting one specific resolved thread to one specific
//     commit, or to one specific minted open-question/constraint object:
//     there is no commit-sha field on a resolution event on either forge,
//     no id linking a thread to an `open_questions`/`constraints` entry,
//     and neither forge exposes such a linkage natively (S6 findings).
//     Inventing one here — e.g. regex-scanning a resolving reply comment
//     for a commit sha, or matching thread ids to object ids by
//     proximity/ordering — would be exactly the "resolve a spec ambiguity
//     silently... from what similar tools do" CLAUDE.md forbids.
//
// This condition therefore encodes ONLY (1) — the one half 05 itself
// calls the deterministic readiness rule — and does not attempt to verify
// (2)'s substantive content (that a given resolution really did point at
// a commit, or that a given minted object really does correspond to a
// given thread). This is a disclosed narrowing, not a silent one
// (constitution 2/10, three-valued honesty): recorded here, in the
// invention ledger, and in this phase's report. A future round that
// defines a concrete linkage mechanism (e.g. a `[vd:<object-id>]`-shaped
// token recorded in the resolving reply or on the minted object) can
// tighten this condition without changing its signature.
//
// f may be nil (no forge configured or reachable) — the condition then
// discloses-unproven rather than reading the missing input as "no
// threads" (mirrors closuregate.go's checkPendingSupersessionCondition).
// When f is non-nil but no open MR is found for branch (a local run
// before the design branch is even pushed), there is nothing to prove —
// no MR means no review threads exist yet — so the condition passes
// outright, mirroring checkPendingSupersessionCondition's own
// "nothing to implement, nothing to prove" trivial pass.
func checkReviewThreadsCondition(ctx context.Context, f forge.Forge, defaultBranchRef, branch string) (gateCondition, error) {
	name := "spec-MR: review threads resolved (forge-native resolution state)"
	if f == nil {
		return gateCondition{
			Name:      name,
			Disclosed: true,
			Reason:    `disclosed-unproven: no forge configured/reachable, so review-thread resolution state cannot be queried (not read as "no threads" — constitution 2/10)`,
		}, nil
	}

	mrID, err := forge.FindOpenMR(ctx, f, defaultBranchRef, branch)
	if err != nil {
		return gateCondition{}, fmt.Errorf("listing open MRs to find this design branch's spec MR: %w", err)
	}
	if mrID == "" {
		return gateCondition{Name: name, OK: true}, nil
	}

	threads, err := f.GetThreadResolution(ctx, mrID)
	if err != nil {
		return gateCondition{}, fmt.Errorf("querying review-thread resolution state for MR %s: %w", mrID, err)
	}

	var unresolved []string
	for _, tr := range threads {
		if !tr.Resolved {
			unresolved = append(unresolved, tr.ThreadID)
		}
	}
	if len(unresolved) > 0 {
		sort.Strings(unresolved)
		return gateCondition{Name: name, Reason: fmt.Sprintf("unresolved review thread(s): %v", unresolved)}, nil
	}
	return gateCondition{Name: name, OK: true}, nil
}

// buildForgeBestEffort constructs the real forge adapter for root's
// configured/detected kind (sync.go's buildForge/loadManifest/
// forge.DetectKind, reused verbatim — no second construction path), or
// returns nil on ANY failure or absence of real connection details (no
// verdi.yaml, no forge: key and no auto-detectable remote, unknown kind,
// or — forgeCredentialsPresent — no CI-provided project/repo identifier
// in the environment). gate must never hard-fail merely because forge
// configuration is incomplete or absent — an accepted, disclosed
// limitation of a local/offline gate run; checkReviewThreadsCondition
// discloses-unproven on a nil forge instead (mirrors closuregate.go's own
// nil-forge tolerance for checkPendingSupersessionCondition).
//
// The forgeCredentialsPresent guard also keeps this function itself
// network-silent under `go test` (CLAUDE.md: no network in any test):
// every fixture repo in this codebase's test suite sets `forge: gitlab`
// (never github) precisely so that, running inside this project's own
// GitHub Actions CI, GITHUB_REPOSITORY is real but CI_PROJECT_ID is not —
// this guard makes that convention load-bearing rather than incidental,
// so a test that happens to drive the real `gate` entry point
// (TestCmdGate_SpecMR_EntryPoint) never reaches a live HTTP call.
func buildForgeBestEffort(ctx context.Context, root string) forge.Forge {
	manifest, err := loadManifest(root)
	if err != nil {
		return nil
	}
	remoteURL, _ := gitx.RemoteURL(ctx, root, "origin") // best-effort: only used for auto-detect
	kind, err := forge.DetectKind(manifest.Forge, remoteURL)
	if err != nil {
		return nil
	}
	if !forgeCredentialsPresent(kind) {
		return nil
	}
	f, err := buildForge(kind)
	if err != nil {
		return nil
	}
	return f
}

// forgeCredentialsPresent reports whether the CI-provided environment
// variables buildForge (sync.go) reads actually name a real project/repo
// — the signal that this run is genuinely inside the forge's own CI,
// never present in a local dev shell or a test process.
func forgeCredentialsPresent(kind string) bool {
	switch kind {
	case "gitlab":
		return os.Getenv("CI_PROJECT_ID") != ""
	case "github":
		return os.Getenv("GITHUB_REPOSITORY") != ""
	default:
		return false
	}
}
