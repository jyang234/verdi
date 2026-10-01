// Package ritualwitness is the ritual effect harness (spec/ritual-effect-
// witness ac-1): it runs a ritual through a Driver against a fixturegit
// repository with a local bare remote, in one of two seeded states, senses
// the repository before and after, and reports each observed effect
// against internal/writescope's Declaration grammar as within the
// declaration, outside it, or unattributable, reading every effect as
// ledger SI-325 does.
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
// included); the main worktree's HEAD (`symbolic-ref -q`, a full refname),
// HEAD tree, index, status, and every non-ignored file's kind, executable
// bit, and content; every linked worktree's administrative entry (id,
// gitdir, HEAD, lock) and its index; the local configuration; every file
// under hooks/ and info/; and every commit object in the repository or the
// remote, reachable or not, with its parents and its file list against its
// first parent. Every listing is read with -z, so no path depends on
// core.quotePath, and every path is canonical, so a macOS /var/folders
// spelling and git's /private/var/folders one compare equal. A store that
// is not the repository root keeps its prefix (Snapshot.Prefix), so
// declared paths stay store-relative.
//
// # Precedence
//
// An effect no declared field admits is outside, whoever made it: the
// state diff alone witnesses it. An admitted effect is within only when
// the command log attributes it to one of gitx's mutating primitives
// (mutatingPrimitives, held equal to writescope.Classification) run in
// the fixture or in a worktree the ritual added; otherwise it is
// unattributable, which Outcome reads as unproven, never as a pass. A
// working-tree file write is not a git mutation (parent dc-6) and is
// reported within with that reason, needing no attribution; a commit an
// added worktree's HEAD reaches is attributed to that worktree by the
// state alone (SI-325 (5)).
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
// SI-325 (9), not sensed: ignored files; the reflog; objects other than
// commits; and other files directly under the git directory (ORIG_HEAD,
// FETCH_HEAD, MERGE_HEAD, a stray file). SI-325 (9) also lists a
// mode-only change to an already-dirty file as unsensed; this harness
// reads every file's executable bit, so it senses that change after all.
//
// Also not sensed, beyond SI-325 (9): a directory entry of the working
// tree (a nested repository or a linked worktree inside the tree, which
// git lists as "dir/"), whose state the worktree sensors read instead; a
// pre-existing linked worktree's working-tree files (only its HEAD,
// index, and administrative entry are sensed, per SI-325 (7)); special
// files (sockets, FIFOs); the remote's configuration and hooks; and
// per-worktree configuration (config.worktree).
//
// Narrowings of attribution, each failing toward unproven, never within:
// a branch move by commit is attributed only when the new tip's first
// parent is the old tip (SI-325 (8)), so a ritual making two commits on one
// branch in one run is unattributable; a remote-tracking ref is a mirror
// only for the remote named origin, mapped by its default refspec; and a
// mirror of a fetch is admitted but never attributed, since no gitx
// primitive fetches.
package ritualwitness
