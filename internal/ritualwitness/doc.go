// Package ritualwitness is the ritual effect harness (spec/ritual-effect-
// witness ac-1): it runs a ritual through a Driver against a fixturegit
// repository with a local bare remote, in one of two seeded states, senses
// the repository before and after, and reports each observed effect
// against internal/writescope's Declaration grammar as within the
// declaration, outside it, or unattributable, reading every effect as
// ledger SI-325, corrected by SI-329, does.
//
// Like internal/fixturegit, this is a Go test helper, not a production
// package (PLAN.md §4). The pieces: Build and BuildWith (fixture.go) seed
// a fixture; Capture (sensor.go) takes a Snapshot; Driver and its
// InProcess, Binary, Workbench, and MCP implementations (driver*.go) run a
// ritual and report its exit and git command log; Evaluate
// (evaluate*.go) judges a before/after pair and the log against a
// Declaration; Run and RunOn (harness.go) compose them; Outcome
// (outcome.go) folds a run's verdicts into pass, fail, or unproven.
//
// # Exit classes
//
// A Driver reports a verb's exit class — 0 clean, 1 verdict failure, 2
// operational refusal — or -1, no verb's exit, which RunOn refuses to
// judge. Exit 2 is not a refusal's signature by itself: a usage error, a
// workbench 4xx or 5xx answer, a Go panic, and a runtime fatal error (a
// deadlock, say) all read as 2 too. So Binary runs the child with
// GOTRACEBACK=single, which a fixture's own Env may override, and maps a
// Go panic trace or a line beginning "fatal error: " to -1; Workbench
// follows no redirect; and a witness of a refused declaration (index carry
// refused) asserts the refusal's own reason — the words of Result.Err —
// never exit 2 alone (ledger SI-334 (3), (4); re-review RR-B1). MCP maps a
// tool result whose isError is false to 0, one whose isError is true or a
// JSON-RPC error to 2, and a transport failure or a response that is
// neither to -1 (ledger SI-341 (5)). Binary sets every CI-context
// variable CIEnv names, each one the verdi binary's code names literally
// (SI-344 (2) discloses what that scan cannot see), on every run from its
// CI field, never from the test process, so a verb that reads one behaves
// the same locally and in CI; a case driving an in-process driver pins the
// same set in the test process with PinCIEnv. Binary also bounds a binary
// whose output a process it started holds open, and a Stdin reader that
// never reaches EOF (binaryWaitDelay).
//
// # Sensed
//
// Every ref of the repository and of the remote (refs/*, deletions
// included), and the remote's HEAD; the main worktree's HEAD
// (`symbolic-ref -q`, a full refname), HEAD tree, index with its flag bits
// (assume-unchanged, skip-worktree), status, and every non-ignored file's
// kind, executable bit, and content; the tree of every local branch's tip;
// every linked worktree's administrative entry (id, gitdir, HEAD, lock),
// the commit it was added at (the first entry of its HEAD reflog), its
// index, and its own refs (refs/worktree/*, refs/bisect/*); the local
// configuration; every file under hooks/ and info/; and every commit
// object in the repository or the remote, reachable or not, with its
// parents and its file list against its first parent. Every listing is
// read with -z, so no path depends on core.quotePath, and every path is
// canonical, so a macOS /var/folders spelling and git's
// /private/var/folders one compare equal. A store that is not the
// repository root keeps its prefix (Snapshot.Prefix), so declared paths
// stay store-relative. The sensors, and the fixture Build seeds, run with
// ambient global and system git configuration isolated (SI-329 (7′)), so
// an operator's core.excludesFile cannot change a classification.
//
// # Precedence and attribution
//
// An effect no declared field admits is outside, whoever made it: the
// state diff alone witnesses it. An admitted effect is within only when
// the command log attributes it to one of gitx's mutating primitives
// (mutatingPrimitives, held equal to writescope.Classification) run in
// the fixture or in a worktree the ritual added; otherwise it is
// unattributable, which Outcome reads as unproven, never as a pass.
// Attribution is per effect (SI-329 (8′)): a remote branch, its
// remote-tracking mirror, and its upstream configuration only to a push of
// that branch (the branch checked out in the push's worktree), with the
// values origin and refs/heads/<b>, and a remote deletion never; a HEAD
// switch only to a checkout naming its target; an index entry or a path a
// commit recorded only to an add or commit whose pathspec matches it. A
// checkout or fast-forward is credited only with the index entries of the
// tree difference it makes, and a commit without a pathspec with none. A
// working-tree file write is not a git mutation (parent dc-6) and is
// reported within with that reason, needing no attribution.
//
// @checked-out is the branch checked out, before the run, in the checkout
// the ritual acts on (ledger SI-348 (4)): the main worktree's by default,
// or, when a test names one in Fixture.Acting, a linked worktree's — a /b/
// route's managed worktree — so a ritual acting there moves its own branch
// within refs_move, and the root's branch is not @checked-out for it. An
// acting checkout that is no worktree registered before the run binds no
// branch (EvaluateIn). The main worktree's own readings of an @checked-out
// move (its index's tree difference, the hand-back exception) apply only
// when the ritual acts there; changes inside a linked worktree stay that
// worktree's.
//
// A created commit belongs to a worktree the ritual added (SI-329 (5′))
// only when it was made after the add — the commit the worktree started at
// does not reach it; an unknown start owns nothing — and either the
// worktree's HEAD reaches it, or the worktree is gone, the log shows a
// commit made in it, and none was logged in the fixture's own checkouts;
// in both, no fixture ref or HEAD may reach it except through a logged
// fast-forward of @checked-out to a commit the still-present worktree
// reaches, with no commit logged in the fixture's own checkouts (the
// context-execution hand-back); a gone worktree cannot show that reach,
// so the exception never applies to it. Without a command log, a commit
// whose ownership needs that logged exception is unattributable, never
// outside. A created commit carrying a foreign entry, owned or judged, is
// outside under every declaration but carried. A worktree counts as added only when the
// after-state lists it and the before-state does not, or the log shows
// both its add and its remove. Every branch move is also judged by its net
// tree difference against the stage paths, for the paths no created commit
// it brings in recorded, owned or judged.
//
// # Disclosed
//
// SI-325 (8)'s residual: gitx records a call before it runs (its observer
// and VERDI_GITLOG alike), so a failed mutating call can still be credited
// with an effect made outside gitx — a failed `git commit` logged in a
// worktree, followed by a move of that worktree's branch to a commit whose
// parent is the old tip, made with plain git, reads as within.
// TestHarness_SensorsAndVerdicts pins it (the "disclosed residual" case).
// spec/gitx-recorder-seam ac-1 narrows where such an effect can come from
// in a real verb, since the verdi binary runs no git outside gitx (ledger
// SI-359 (4)); but git that a child program runs itself is outside that
// contract (SI-359 (4b)), and the log still cannot tell a call that failed
// from one that succeeded, so the residual stays disclosed, not closed
// (SI-359 (9)).
//
// SI-329 (8′)'s residual, beside it: where a call names no paths — `add
// -A`, a commit without a pathspec, commit-tree — attribution of index
// entries and commit paths stays per worktree, so such a call is credited
// with any path in its worktree. spec/gitx-recorder-seam changes no
// call's argv, so this residual stays disclosed too (SI-359 (9)), and
// SI-359 (12) extends it to a commit-tree logged in a worktree the ritual
// added, which may attribute the commit it creates and only that one,
// matched by the call's parent argument.
//
// SI-325 (9), not sensed: ignored files; the reflog (but for a linked
// worktree's first entry, read only to find the commit it was added at);
// objects other than commits; and other files directly under the git
// directory (ORIG_HEAD, FETCH_HEAD, MERGE_HEAD, a stray file). SI-325 (9)
// also lists a mode-only change to an already-dirty file as unsensed;
// this harness reads every file's executable bit, so it senses that change
// after all.
//
// SI-329 (9′): git 2.50 and later write stash-like commit objects ("WIP
// on", "index on") for a plain true merge over dirty tracked work, and the
// harness counts them as created commits. No gitx primitive does such a
// merge.
//
// Also not sensed, beyond SI-325 (9): a directory entry of the working
// tree (a nested repository or a linked worktree inside the tree, which
// git lists as "dir/"), whose state the worktree sensors read instead; a
// pre-existing linked worktree's working-tree files and HEAD tree; special
// files (sockets, FIFOs); the remote's configuration and hooks; and
// per-worktree configuration (config.worktree).
//
// Classified outside, as no field admits it: a remote-tracking ref of a
// remote other than origin (SI-329 (7′)), and one of origin that does not
// mirror the remote's ref after the run.
//
// Narrowings of attribution, each failing toward unproven, never within:
// a branch move by commit is attributed only when the new tip's first
// parent is the old tip (SI-325 (8)), so a ritual making two commits on one
// branch in one run is unattributable; a mirror of a fetch is admitted but
// never attributed, since no gitx primitive fetches; and an added
// worktree whose start commit is unknown (no HEAD reflog, and a logged add
// naming neither a full commit id nor a branch) owns no commit.
package ritualwitness
