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

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/forge"
	forgegithub "github.com/jyang234/verdi/internal/forge/github"
	"github.com/jyang234/verdi/internal/gitx"
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
			Source:    "gate:review-threads-resolved",
			Reason:    `no forge configured/reachable, so review-thread resolution state cannot be queried (not read as "no threads" — constitution 2/10)`,
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

// forgeBestEffort constructs the real forge adapter for root's
// configured/detected kind (sync.go's buildForge/loadManifest/
// forge.DetectKind, reused verbatim — no second construction path). It
// returns two facts the read surfaces both need (I-1):
//
//   - f: the live forge, or nil when one cannot be built.
//   - configuredKind: the forge kind NAMED in verdi.yaml or auto-detected
//     from the remote ("" when none is — DetectKind failed). This is the
//     "is a forge configured at all" signal, resolved WITHOUT network (a
//     local git remote read only), so a caller can tell a
//     configured-but-unreachable forge (disclose) apart from a genuinely
//     unconfigured checkout (stay silent) — I-1(b).
//
// Construction is best-effort: gate/serve/mcp must never hard-fail merely
// because forge config is incomplete or absent. When configuredKind is set
// but credentials are absent, f is nil AND configuredKind is non-empty —
// the disclosed-unavailable state. checkReviewThreadsCondition
// discloses-unproven on a nil forge (mirrors closuregate.go's own
// nil-forge tolerance).
//
// forgeCredentialsPresent keeps this network-silent under `go test`
// (CLAUDE.md: no network in any test): a bare fixture repo, even one with
// `forge: gitlab` in verdi.yaml, exports no forge credentials, so f is nil
// and no adapter method is ever reachable. Local reachability is the flip
// side of the same gate: a developer who exports the same credentials the
// adapters read gets a live forge from `verdi serve`, no CI required.
func forgeBestEffort(ctx context.Context, root string) (f forge.Forge, configuredKind string) {
	manifest, err := loadManifest(root)
	if err != nil {
		return nil, ""
	}
	remoteURL, _ := gitx.RemoteURL(ctx, root, "origin") // best-effort: only used for auto-detect
	kind, err := forge.DetectKind(manifest.Forge, remoteURL)
	if err != nil {
		return nil, ""
	}
	// A forge IS configured from here on (kind names it). Whether it is
	// REACHABLE depends on credentials being present in the environment.
	if !forgeCredentialsPresent(kind, remoteURL) {
		return nil, kind
	}
	built, err := buildForge(kind, remoteURL)
	if err != nil {
		return nil, kind
	}
	return built, kind
}

// buildForgeBestEffort returns just the live forge (or nil) for callers
// that do not need the configured-but-unavailable distinction (gate, mcp
// standalone). serve.go/mcp.go read configuredKind through forgeBestEffort
// directly to drive their disclosure.
func buildForgeBestEffort(ctx context.Context, root string) forge.Forge {
	f, _ := forgeBestEffort(ctx, root)
	return f
}

// forgeCredentialsPresent reports whether a live adapter can be built for
// kind — the auth TOKEN the buildForge (sync.go) adapters read must be
// present, and the repo IDENTIFIER must be resolvable. Present, the forge
// can authenticate (in the forge's own CI or a local shell that exported the
// token); absent, no live adapter is built, so we never build a forge doomed
// to a 401/404. It keeps the check hermetic under `go test`: a fixture repo
// exports no token, so this is false regardless of which forge kind its
// verdi.yaml names. remoteURL is the origin remote (best-effort; "" when
// none) — for github the identifier falls back to it (D6-14), so it is
// consulted only after the token is confirmed present.
func forgeCredentialsPresent(kind, remoteURL string) bool {
	switch kind {
	case "gitlab":
		return os.Getenv("CI_PROJECT_ID") != "" && os.Getenv("CI_JOB_TOKEN") != ""
	case "github":
		// GITHUB_TOKEN is the auth verdi cannot synthesize; without it there
		// is nothing to build (and a bare fixture repo exports none, so this
		// stays false under `go test`). The identifier may come from GitHub
		// Actions' env OR (D6-14) the origin URL, so a local shell that
		// exported only GITHUB_TOKEN still yields a live forge.
		if os.Getenv("GITHUB_TOKEN") == "" {
			return false
		}
		if os.Getenv("GITHUB_REPOSITORY") != "" {
			return true
		}
		_, _, ok := forgegithub.OwnerRepoFromURL(remoteURL)
		return ok
	default:
		return false
	}
}

// reviewUnavailableReason renders the disclosed-unavailable notice for a
// configured-but-unreachable forge (I-1(b)) — one message shared by the
// board chrome and the mcp list_annotations disclosure field so both read
// surfaces say the same thing. Rendered through the shared
// internal/disclosure seam (spec/disclosure-seam-v2, ac-1) — the same
// Render function lint's Finding.String() and gate's disclosed conditions
// use, so equivalent disclosed-unproven states read in one vocabulary
// wherever they appear (spec/disclosure-legibility#ac-1).
func reviewUnavailableReason(kind string) string {
	return disclosure.Render(reviewUnavailableDisclosure(kind))
}

// reviewUnavailableDisclosure is the structured seam value behind
// reviewUnavailableReason — the ONE decision point for the
// configured-but-unreachable review-feed disclosure. serve.go hands it to
// the workbench's /disclosures page (workbench.Deps.Disclosures,
// spec/disclosures-panel ac-1) so the panel enumerates the same value the
// board chrome and list_annotations render, never a re-derived copy.
func reviewUnavailableDisclosure(kind string) disclosure.Disclosure {
	text := fmt.Sprintf("forge %q is configured (verdi.yaml) but no credentials are available to reach it; review state cannot be shown", kind)
	return disclosure.New("mcp:review-feed", "", text)
}
