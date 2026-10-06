// Package pollwitness pins, byte for byte, what the wall's poll and the
// shared readiness loader derive: lane P1's byte-identity witnesses
// (ledger SI-352; spec/readiness-recovery-v2 ac-2, ac-4).
//
// Lane P1 cuts the git cost of the readiness load and of the wall badges'
// reachability checks, and every one of those reductions must leave every
// derived byte unchanged. These witnesses were captured before any
// reduction landed (at af8eb775), over two fixtures:
//
//   - the e2e fixture: examples/showcase built layer by layer with its
//     golden SHAs, plus what the e2e harness (cmd/e2eharness) adds to it —
//     the loansvc service root, the committed .gitattributes and
//     .verdi/.gitignore, the obligation-quality adoption ancestry graft,
//     and the untracked mutable and derived zones;
//   - the real-store-shaped fixture: that store with an origin remote whose
//     default branch is origin/main, an archived (closed) implementing
//     story landed there, and HEAD on a feature branch ahead of it carrying
//     a new implementing story whose context pins name a reachable commit,
//     the same commit abbreviated, and a dangling commit — the shapes the
//     real store's walls carry.
//
// For one story and one feature on each fixture they pin the readiness
// snapshot, the Readiness section that the CLI composition, MCP
// get_document and the board Document tab render, the readiness page,
// and the wall badges. A golden in testdata/ changes only by a deliberate
// re-capture (VERDI_POLLWITNESS_UPDATE=1), which is itself a byte change
// that needs its own authority.
//
// It is a test-only package (every .go file here is a _test.go file),
// following internal/showcasealign's, internal/specalign's and
// internal/corpus's precedent.
package pollwitness
