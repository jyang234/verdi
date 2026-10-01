// Package ritualwitness is the ritual effect harness (spec/ritual-effect-
// witness ac-1): it runs a ritual through a Driver against a fixturegit
// repository with a local bare remote, in one of two seeded states, senses
// the repository before and after, and reports each observed effect
// against internal/writescope's Declaration grammar as within the
// declaration, outside it, or unattributable, reading every effect as
// ledger SI-325, corrected by SI-329, does.
//
// Like internal/fixturegit, this is a Go test helper, not a production
// package (PLAN.md §4). The pieces: Build (fixture.go) seeds a fixture;
// Capture (sensor.go) takes a Snapshot; Driver and InProcess (driver.go)
// run a ritual and report its exit and git command log; Evaluate
// (evaluate*.go) judges a before/after pair and the log against a
// Declaration; Run and RunOn (harness.go) compose them; Outcome
// (outcome.go) folds a run's verdicts into pass, fail, or unproven.
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
// A created commit belongs to a worktree the ritual added (SI-329 (5′))
// only when it was made after the add — the commit the worktree started at
// does not reach it; an unknown start owns nothing — and either the
// worktree's HEAD reaches it, or the worktree is gone, the log shows a
// commit made in it, and none was logged in the fixture's own checkouts;
// in both, no fixture ref or HEAD may reach it except through a logged
// fast-forward of @checked-out (the context-execution hand-back). Without
// a command log, a commit whose ownership needs that logged exception is
// unattributable, never outside. A worktree counts as added only when the
// after-state lists it and the before-state does not, or the log shows
// both its add and its remove. Every branch move is also judged by its net
// tree difference against the stage paths, for the paths no created commit
// it brings in recorded, owned or judged.
//
// # Disclosed
//
// SI-325 (8)'s residual: gitx's observer fires before a command runs, so
// a failed mutating call can still be credited with an effect made outside
// gitx — a failed `git commit` logged in a worktree, followed by a move of
// that worktree's branch to a commit whose parent is the old tip, made
// with plain git, reads as within. TestHarness_SensorsAndVerdicts pins it
// (the "disclosed residual" case), so closing it — spec/gitx-recorder-seam
// ac-1, no git execution outside gitx — turns that case red and forces
// this disclosure to change.
//
// SI-329 (8′)'s residual, beside it: where a call names no paths — `add
// -A`, a commit without a pathspec, commit-tree — attribution of index
// entries and commit paths stays per worktree, so such a call is credited
// with any path in its worktree. spec/gitx-recorder-seam ac-1 closes it.
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
