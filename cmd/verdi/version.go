// verdi version / verdi --version (spec/uat-round-1 ac-1, CLI part;
// closes UAT-003: "Two builds were live during the UAT and disagreed
// about the store; nothing identified which one produced a result"):
// prints the build's one-line identification string — internal/buildinfo,
// the SAME function `verdi serve` calls once at startup (serve.go) and,
// separately (a Fable lane, workbench HTML/JS/templates), the workbench
// footer calls — so all three surfaces can never disagree about which
// build produced a result.
//
// Dispatched directly from dispatch.go's central intercept, before the
// verbPhase lookup, exactly like "help"/"--help"/"-h" (help.go): "version"
// and "--version" are top-level tokens, not phase-numbered verbs, so they
// are never added to verbPhase — keeping the CLI-verb inventory
// (internal/specalign's serialized registry, CLAUDE.md) untouched by this
// change.
package main

import (
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/buildinfo"
)

// cmdVersion is `verdi version`/`verdi --version`'s entry point. Exit 0
// unconditionally (co-2: "Help and version are clean exits") — there is
// no failure mode here: buildinfo.Line() never errors, honestly
// disclosing unavailable build info as "verdi (devel)" rather than
// fabricating one, so this verb has nothing of its own to fail on.
// Trailing arguments are accepted and ignored, the same permissive
// posture co-2's clean exits already imply.
func cmdVersion(stdout io.Writer) int {
	fmt.Fprintln(stdout, buildinfo.Line())
	return 0
}
