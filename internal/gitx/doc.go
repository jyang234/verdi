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
// replays each ref resolution after the first, on their happy paths only
// — every other answer, every error included, is still the exec's own.
package gitx
