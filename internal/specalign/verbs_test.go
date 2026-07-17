// CLI verb inventory (deliverable 1d): every verb 05-surfaces.md §CLI's
// table names, plus the invention ledger's gate (I-7) and board (I-20)
// (which 05 §CLI's own table predates and dispatch.go recognizes
// alongside it), responds per its v0 status. Real v0 verbs never print
// "not implemented"; the two verbs PLAN.md §5 puts explicitly out of
// v0 scope (waivers, verify-artifact) always do, with the exact
// out-of-scope message.
//
// Grown at V1-P9 (item 4, the spec-align regrowth) to cover the v2
// surface: `build` (R4-I-6's `verdi build start`, superseding `feature
// start`) and `audit` (R4-I-10) were real, dispatched v1 verbs that this
// inventory had never named — dispatch.go's own verbPhase map already had
// them (phases 7 and 13 respectively), this test just hadn't grown to
// match. TestV1CLIVerbForms (below) additionally proves the v1
// argument-shape variants: `design start --kind feature|story`.
//
// Round 6 (spec/close-verb): `close` graduated from PLAN.md §5's
// out-of-v0-scope list to a real, dispatched verb (I-23's phase-0 stub
// flipped in cmd/verdi/close.go) — moved from outOfV0 to inV0 below, with
// its own hermeticity note next to serve/mcp/audit/align's.
//
// Round 6 (spec/worktree-manager): `gc` graduates the same way — I-23's
// phase-0 stub flipped in cmd/verdi/gc.go, scoped honestly to the
// managed-worktree reclamation slice only (dc-5). Moved from outOfV0 to
// inV0 below, with its own hermeticity note (gc can REMOVE a managed
// worktree, a real mutation this inventory check must never risk against
// the shared self-hosted checkout).
//
// Legibility-ergonomics round (spec/attest-helper): `attest` is an
// entirely NEW top-level verb (dc-1), not a graduated stub — added
// straight to inV0 below. Its own argument-count check runs before any
// store root is resolved (cmd/verdi/attest.go's cmdAttest), so a bare
// `verdi attest` (no args) against the shared self-hosted checkout fails
// on usage alone, deterministically, before touching anything — the same
// safety property `matrix`/`rollup`/`design`/`accept`/`board` already rely
// on in the default case below, needing no special-cased hermeticity
// branch of its own.
//
// Round 6 (spec/disposition-verb): `disposition` is a brand-new verb (no
// prior phase-0 stub) — dispatch.go's verbPhase gains a `disposition` key
// and a real dispatch call in the same change this test's own inV0 addition
// rides (ADJ-48: a branch adding a verb without matching inventory updates
// is a spec-align gap this addition closes). Added to inV0 below, with its
// own hermeticity note next to close's (both are mutating verbs whose bare,
// no-argument invocation fails on argument-shape parsing before resolving a
// store root or touching any file — safe to run against the live checkout).
package specalign

import "testing"

func TestV0CLIVerbInventory(t *testing.T) {
	root := verdiRepoRoot

	// 05-surfaces.md §CLI's table (minus dex's own "build" subcommand,
	// handled specially below) plus I-7's `gate`, I-20's `board`, R4-I-6's
	// `build`, R4-I-10's `audit`, round-6's `close`, and the
	// legibility-ergonomics round's `attest` (spec/attest-helper dc-1).
	inV0 := []string{
		"lint", "design", "accept", "feature", "build", "align", "sync",
		"serve", "mcp", "matrix", "rollup", "dex", "gate", "board", "audit",
		"close", "gc", "attest", "disposition",
	}
	// PLAN.md §5 scope discipline, verbatim (as amended: `close`/`gc`
	// graduated to real, round 6): "Explicitly out of v0 (not stubbed —
	// absent ...): `verdi gc`, `verdi waivers` audit verb, `verdi
	// verify-artifact`".
	outOfV0 := []string{"waivers", "verify-artifact"}

	for _, verb := range inV0 {
		t.Run("real_"+verb, func(t *testing.T) {
			var stderr string
			switch verb {
			case "serve", "mcp", "audit", "align", "gc":
				// serve/mcp resolve the store root before doing anything
				// else (socket bind, lock acquire, ...); audit (bare, no
				// args) resolves the store root and then actually RUNS the
				// exemption/spec-stale sweep against it, which can
				// auto-file a conflict record into the working tree at
				// threshold (03 §Exemption audit) — a real mutation this
				// inventory check must never risk against the shared
				// self-hosted checkout. align takes no argument at all —
				// it infers its spec from the CURRENT BRANCH, so against
				// the live checkout it is branch-state-dependent: on main
				// it fails fast, but on a real design/build branch (the
				// round-5 self-hosted arena's normal state) it runs the
				// REAL alignment — execing verdi.yaml's live judge_cmd
				// (`claude -p`, a network call with a 2m timeout;
				// CLAUDE.md: no network in any test) and writing a real
				// deviation-report.md into the shared working tree
				// (round5-divergences.md D-14, the same hermeticity class
				// as D-11's checklist-probe fix). gc (spec/worktree-manager)
				// resolves the store root and then actually SCANS/RECLAIMS
				// managed worktrees, including real `git worktree remove`
				// calls for any that are eligible — a real mutation this
				// inventory check must never risk against the shared
				// self-hosted checkout either. All five fail fast and
				// honestly from a rootless tempdir instead, while still
				// proving the verb is dispatched as real.
				_, stderr, _ = runBinary(t, t.TempDir(), verb)
			case "close", "disposition":
				// `close` runs a real, mutating ritual (closure branch,
				// quartet archive-move, commit, publish); `disposition`
				// writes a reviewer's decision into a deviation report in
				// place (spec/disposition-verb) — both mutations this
				// inventory check must never risk against the shared
				// self-hosted checkout, the same hermeticity concern as
				// serve/mcp/audit/align above. Bare `close`/`disposition`
				// (no arguments at all) each fail on argument parsing
				// BEFORE resolving a store root or touching any file,
				// deterministically, regardless of environment (CI or
				// not) — the one invocation shape safe to run anywhere.
				_, stderr, _ = runBinary(t, t.TempDir(), verb)
			case "dex":
				_, stderr, _ = runBinary(t, root, "dex", "build", "-o", t.TempDir())
			default:
				_, stderr, _ = runBinary(t, root, verb)
			}
			assertNotOutOfV0(t, verb, stderr)
		})
	}

	for _, verb := range outOfV0 {
		t.Run("outofscope_"+verb, func(t *testing.T) {
			_, stderr, code := runBinary(t, root, verb)
			if code != 2 {
				t.Errorf("verdi %s: exit = %d, want 2 (operational error)", verb, code)
			}
			const want = "not implemented (out of v0 scope)\n"
			if stderr != want {
				t.Errorf("verdi %s: stderr = %q, want exactly %q", verb, stderr, want)
			}
		})
	}
}

// TestV1CLIVerbForms proves the v2-surface argument-shape variants item 4
// of this phase's brief names: `design start --kind feature|story` both
// dispatch to the real implementation (never "not implemented"), from a
// rootless tempdir so a missing store root fails fast rather than
// mutating anything.
//
// Round 6: the `close <story|feature>` positional-argument-shape subtests
// that used to prove close's (then out-of-scope) answer was unchanged by
// argument shape are retired — close is real now, and TestV0CLIVerbInventory
// above already covers its dispatch; a story-ref-shaped vs. a bare
// spec-name-shaped argument against a real, mutating verb is exercised by
// cmd/verdi/close_test.go's own hermetic fixturegit suite, not this
// live-checkout inventory (which deliberately never risks a real mutation).
func TestV1CLIVerbForms(t *testing.T) {
	t.Run("design_start_kind_feature", func(t *testing.T) {
		_, stderr, code := runBinary(t, t.TempDir(), "design", "start", "--kind", "feature", "--name", "specalign-probe-feature")
		assertNotOutOfV0(t, "design", stderr)
		if code != 2 {
			t.Errorf("verdi design start --kind feature (no store root): exit = %d, want 2 (operational error)", code)
		}
	})

	t.Run("design_start_kind_story", func(t *testing.T) {
		_, stderr, code := runBinary(t, t.TempDir(), "design", "start", "jira:SPECALIGN-1", "--kind", "story", "--name", "specalign-probe-story")
		assertNotOutOfV0(t, "design", stderr)
		if code != 2 {
			t.Errorf("verdi design start --kind story (no store root): exit = %d, want 2 (operational error)", code)
		}
	})
}
