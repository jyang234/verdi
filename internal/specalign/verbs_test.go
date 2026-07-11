// CLI verb inventory (deliverable 1d): every verb 05-surfaces.md §CLI's
// table names, plus the invention ledger's gate (I-7) and board (I-20)
// (which 05 §CLI's own table predates and dispatch.go recognizes
// alongside it), responds per its v0 status. Real v0 verbs never print
// "not implemented"; the four verbs PLAN.md §5 puts explicitly out of
// v0 scope (close, gc, waivers, verify-artifact) always do, with the
// exact out-of-scope message.
package specalign

import "testing"

func TestV0CLIVerbInventory(t *testing.T) {
	root := verdiRepoRoot

	// 05-surfaces.md §CLI's table (minus dex's own "build" subcommand,
	// handled specially below) plus I-7's `gate` and I-20's `board`.
	inV0 := []string{
		"lint", "design", "accept", "feature", "align", "sync",
		"serve", "mcp", "matrix", "rollup", "dex", "gate", "board",
	}
	// PLAN.md §5 scope discipline, verbatim: "Explicitly out of v0 (not
	// stubbed — absent ...): `verdi close` automation, `verdi gc`,
	// `verdi waivers` audit verb, `verdi verify-artifact`".
	outOfV0 := []string{"close", "gc", "waivers", "verify-artifact"}

	for _, verb := range inV0 {
		t.Run("real_"+verb, func(t *testing.T) {
			var stderr string
			switch verb {
			case "serve", "mcp":
				// Both resolve the store root before doing anything else
				// (socket bind, lock acquire, ...); running from a
				// rootless tempdir fails fast and honestly instead of
				// blocking on a long-running server, while still
				// proving the verb is dispatched as real.
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
