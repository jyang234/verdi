// Package gitx is a minimal set of git plumbing helpers, execed rather than
// linked (no cgo, no go-git dependency — PLAN.md §2: "internal/gitx/ ...
// technique; no semantic ownership"). It grows only as later phases need
// more of git; phase 3 needs exactly four operations:
//
//   - RevParse: resolve any git revision expression (a ref, a commit, or
//     "<rev>:<path>") to the object id git would resolve it to.
//   - HashObject: the git blob SHA-1 of a file's current on-disk content,
//     independent of whether that content is staged or committed
//     (I-15: "dirty working files hashed as git would hash the blob").
//   - LsFiles: the paths git tracks under a directory, respecting
//     .gitignore — the store's committed-zone enumeration.
//   - Show: a file's content as it existed at a specific commit, needed to
//     resolve pinned refs (kind/name@commit) to historical content.
//
// Every function execs the system git binary and wraps a non-zero exit with
// the command and its stderr, so failures are legible without a debugger.
// The one exception is a read session (WithReadSession): within one
// request it answers Show's and BlobAt's reads from one batch process and
// replays each identical ref-read argv (and a full id's whole-tree
// listing, and the directory's prefix) after its first run, on their
// happy paths only — every other answer, every error included, is still
// the exec's own.
//
// Every exec passes one observe point first (observer.go): a gitx.Observer
// attached to the call's context sees the call, and when the test-only
// VERDI_GITLOG environment variable names a file, the call's record is
// appended to it (gitlog.go, GitLogEnv). VERDI_GITLOG is a test hook the
// product never sets; it lets a test record the built binary's whole git
// command log (spec/gitx-recorder-seam dc-2). The log holds one record per
// git execution (ledger SI-359 (1), (3)), so a read session's batch
// process is recorded once, when it starts, and a replayed read, which
// runs no git, records nothing; an observer implementing SessionObserver
// is told of each replay and each batched name instead.
package gitx
