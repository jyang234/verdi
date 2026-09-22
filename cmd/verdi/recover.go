// verdi recover [--json] <spec-ref> [--apply <choice-id>] (spec/readiness-
// recovery-v2 ac-8..ac-10): the read-only CLI surface over
// internal/recovery's projection, plus the one mutating path this wave
// adds (--apply, over the two existing executors branchcut.Unwind and
// reclaim.Apply — co-3). It resolves ref to a feature spec's own ritual
// branches and store facts, derives the closed inventory of interrupted-
// ritual states ac-8 names, and prints exactly one line of this store's
// canonical JSON; the explicit --json flag and the legacy no-flag form
// delegate to the same call and are byte-identical (recovery.Canonical —
// sorted keys, digest-bound — mirroring journey.Canonical's own
// contract). No status flip, no stamp write, no staging, no commit, no
// forge call on the read path: the projection is never authority (co-2).
//
// EXIT CLASSIFICATION (R-RR3-1, ledger SI-220) — a DELIBERATE DIVERGENCE
// from journey.go's "exit 1 is unreachable" doctrine. journey.go's own
// doc comment reasons that a journey record never fails merely because it
// carries a blocker, since a blocker is a disclosed fact, not a verdict.
// `recover` differs: ac-8's three exit codes are themselves a verdict
// ABOUT RECOGNIZED STATE — "was at least one of the closed inventory's
// interrupted-ritual states recognized for this ref" — even though the
// projection's CONTENTS remain a non-authoritative disclosure (co-2). The
// parallel is `verdi close --preflight` ("an absent artifact is a
// verdict, never operational"), not `verdi journey`: exit 0 means nothing
// was recognized, exit 1 means at least one state was (Projection.
// Recognized()), exit 2 is operational (a malformed argument, no
// resolvable store root, an unresolvable ref, or any underlying
// git/decoding failure — including a run that ever issued a forbidden git
// command, R-RR3-10, which is an operational failure of this verb's own
// contract, never a verdict). `--apply` has its own three-way exit
// mapping (R-RR3-9): 0 when every postcondition holds, 1 for a refused
// choice, 2 operational; Task 4 implements that path's success case, this
// task only wires the stub's uniform refusal.
//
// recoverErr guarantees a stderr line is "recover: "-prefixed exactly
// once, mirroring journey.go's journeyErr — internal/recovery's own
// errors self-prefix "recovery: ..." (a different word), so no double-
// prefix collision is possible; this only ever adds the "recover: "
// prefix.
//
// Kept in its own file per the lint.go/sync.go/specstate.go/journey.go
// convention.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/recovery"
	"github.com/jyang234/verdi/internal/store"
)

// vocab:identity — CLI usage grammar
const recoverUsage = "usage: verdi recover [--json] <spec-ref> [--apply <choice-id>]"

// recoveryGitLogEnv (R-RR3-10) is a test-only observability seam: when
// set, cmdRecover appends every git command this run issued (dc-4/ac-9's
// own command surface) to the named file, one "<root>\t<argv...>" line
// per command. The verb itself never reads this file back.
const recoveryGitLogEnv = "VERDI_RECOVERY_GITLOG"

// cmdRecover is `verdi recover`'s entry point, invoked by dispatch.go.
// Its own argument-shape validation runs before any store root is
// resolved (mirroring journey.go/attest.go/matrix.go/design.go's usage-
// first posture), so a bare or malformed invocation fails fast and
// identically regardless of cwd.
func cmdRecover(args []string, stdout, stderr io.Writer) int {
	ref, applyID, apply, ok := parseRecoverArgs(args)
	if !ok {
		fmt.Fprintln(stderr, recoverUsage)
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		recoverErr(stderr, err)
		return 2
	}
	cfg, err := store.Open(root)
	if err != nil {
		recoverErr(stderr, err)
		return 2
	}

	// R-RR3-10: attach the command log for the WHOLE run, so any forbidden
	// git command this run issues — whether during Gather/Derive's own
	// read-only facts or (later, Task 4) an executor's mutation — is
	// caught.
	var log recovery.CommandLog
	ctx := gitx.WithObserver(context.Background(), &log)

	exit := 0
	if apply {
		if _, applyErr := recovery.Apply(ctx, cfg, ref, applyID, stderr); applyErr != nil {
			recoverErr(stderr, applyErr)
			exit = 2
		}
		// Task 4 replaces this branch with the full postcondition report
		// (R-RR3-9); the Task 3 stub (recovery.ErrNotImplemented) never
		// succeeds, so the success path above is presently unreachable.
	} else {
		proj, loadErr := recovery.Loader{Root: root}.Load(ctx, ref)
		switch {
		case loadErr != nil:
			recoverErr(stderr, loadErr)
			exit = 2
		default:
			data, canonErr := recovery.Canonical(proj)
			switch {
			case canonErr != nil:
				recoverErr(stderr, canonErr)
				exit = 2
			default:
				if _, werr := stdout.Write(data); werr != nil {
					recoverErr(stderr, werr)
					exit = 2
				} else if proj.Recognized() {
					exit = 1
				}
			}
		}
	}

	if path := os.Getenv(recoveryGitLogEnv); path != "" {
		if err := appendRecoveryGitLog(path, root, log.Entries()); err != nil {
			recoverErr(stderr, err)
			return 2
		}
	}

	// R-RR3-10: a recovery run that ever issued a forbidden git command is
	// an operational failure of this verb's OWN contract, never a
	// verdict — this overrides whatever exit the run above computed.
	if forbidden := log.Forbidden(); len(forbidden) > 0 {
		for _, entry := range forbidden {
			fmt.Fprintf(stderr, "recover: forbidden git command issued: git %s\n", strings.Join(entry, " "))
		}
		return 2
	}

	return exit
}

// parseRecoverArgs implements recoverUsage's grammar: an optional leading
// --json, a required spec-ref, and an optional trailing "--apply
// <choice-id>" pair. ok is false for every shape the usage string does
// not admit (including a bare invocation, --apply with no preceding ref,
// two positional refs, or --json alone) — the caller prints recoverUsage
// and exits 2 without ever resolving a store root.
func parseRecoverArgs(args []string) (ref, applyID string, apply, ok bool) {
	rest := args
	if len(rest) > 0 && rest[0] == "--json" {
		rest = rest[1:]
	}
	if len(rest) == 0 || strings.HasPrefix(rest[0], "-") {
		return "", "", false, false
	}
	ref = rest[0]
	rest = rest[1:]
	switch len(rest) {
	case 0:
		return ref, "", false, true
	case 2:
		if rest[0] != "--apply" || rest[1] == "" || strings.HasPrefix(rest[1], "-") {
			return "", "", false, false
		}
		return ref, rest[1], true, true
	default:
		return "", "", false, false
	}
}

// appendRecoveryGitLog appends one "<root>\t<argv joined by spaces>" line
// per entry to path (creating it if absent). CommandLog.Observe
// deliberately never records a per-command dir (its own doc comment:
// "Forbidden's contract is over argv shape alone"), and every command a
// recovery run issues runs against this one root, so root is repeated on
// every line rather than fabricated per entry.
func appendRecoveryGitLog(path, root string, entries [][]string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("appending recovery git log %s: %w", path, err)
	}
	var writeErr error
	for _, entry := range entries {
		if _, err := fmt.Fprintf(f, "%s\t%s\n", root, strings.Join(entry, " ")); err != nil {
			writeErr = err
			break
		}
	}
	if cerr := f.Close(); writeErr == nil {
		writeErr = cerr
	}
	if writeErr != nil {
		return fmt.Errorf("appending recovery git log %s: %w", path, writeErr)
	}
	return nil
}

// recoverErr writes err to stderr as one line, guaranteeing the line
// starts with "recover: " exactly once — mirroring journey.go's
// journeyErr. internal/recovery's own errors self-prefix "recovery: ..."
// (a distinct word, "recovery" not "recover"), so this never double-
// prefixes; it only ever adds the verb's own "recover: " prefix.
func recoverErr(stderr io.Writer, err error) {
	msg := err.Error()
	if !strings.HasPrefix(msg, "recover: ") {
		msg = "recover: " + msg
	}
	fmt.Fprintln(stderr, msg)
}
