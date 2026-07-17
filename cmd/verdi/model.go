// verdi model check (L-M1, spec/model-schema ac-3): validates a store's
// operating model — `.verdi/model.yaml` if present, else the embedded
// canonical default (model.Canonical()) — giving it the same fail-closed
// feedback `verdi lint` already gives a spec.
//
// Exit discipline (CLAUDE.md's 0/1/2 contract), per spec/model-schema
// ac-3 verbatim (frozen in both the story spec and its own obligation
// doc): 0 with an OK line naming the schema, class/transition counts,
// and the resolved model's digest, on any valid manifest — including the
// absent-model.yaml case (resolves to the canonical default) and a valid
// hand-written model.yaml (vocabulary/template changes only, dc-1's
// frontier); 1 with the ONE pinned frontier error text, printed
// verbatim, on a structurally deviant manifest — model.ErrFrontier is
// the sole condition that earns exit 1; 2 on operational trouble: a
// missing store, or an unreadable OR UNDECODABLE manifest — which
// explicitly includes a KERNEL VALIDATION rule violation (a bad
// scheme/kind, a missing obligations list, and so on), since ac-3's own
// text groups those with "undecodable," not with the frontier's exit 1.
// This plan's own Task 7 prose ("exit 1 on validation/frontier failure")
// reads more broadly than that frozen text; spec+obligation win per this
// build's precedence rule (reported in the phase report as a disclosed
// conflict) — implemented here exactly as ac-3 states it.
package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// runModelVerb dispatches `verdi model <subcommand>`. There is exactly
// one subcommand, `check` (mirroring runBuildVerb's single-subcommand
// shape for `verdi build start`); anything else is a usage error.
func runModelVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "check" {
		fmt.Fprintln(stderr, "usage: verdi model check")
		return 2
	}
	return cmdModelCheck(args[1:], stdout, stderr)
}

// cmdModelCheck is `verdi model check`'s real entry point: it resolves
// the store root and delegates to the testable core, runModelCheck.
func cmdModelCheck(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintf(stderr, "model check: unexpected extra argument %q\n", args[0])
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "model check:", err)
		return 2
	}
	return runModelCheck(root, stdout, stderr)
}

// runModelCheck is the testable core: given an already-resolved store
// root, resolves the store's full config (store.Open — verdi.yaml AND,
// Task 6, model.yaml in the same bottleneck) and reports on its Model.
func runModelCheck(root string, stdout, stderr io.Writer) int {
	cfg, err := store.Open(root)
	if err != nil {
		if errors.Is(err, model.ErrFrontier) {
			fmt.Fprintln(stderr, "model check:", err)
			return 1
		}
		fmt.Fprintln(stderr, "model check:", err)
		return 2
	}

	digest, err := cfg.Model.Digest()
	if err != nil {
		fmt.Fprintln(stderr, "model check:", err)
		return 2
	}

	transitions := 0
	for _, lc := range cfg.Model.Lifecycle {
		transitions += len(lc.Transitions)
	}

	fmt.Fprintf(stdout, "model: OK — %s, %d classes, %d transitions, digest %s\n",
		cfg.Model.Schema, len(cfg.Model.Classes), transitions, digest)
	return 0
}
