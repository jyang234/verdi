// Package execworkspace implements spec/execution-workspace's identity and
// state-grammar layer (spec §Workspace naming; ledger SI-10, SI-15): the
// deterministic <workspace-id> naming scheme for the two materialization
// request shapes, the immutable request-identity sidecar's canonical-JSON
// codec, the sibling path grammar under a store's data/execution/ root, and
// the lstat-based path typing every mutator of that grammar depends on.
//
// This package is deliberately narrow: no materialization (worktree
// creation, patch application), no git calls, no locking, and no gc
// reclaim decision live here — those are later lanes' concern, built on
// top of the primitives this package exports (spec/execution-workspace
// §Implementation seam names this unit "execution-workspace enforcement",
// one shared internal package beside internal/wtmanager, internal/gitx,
// internal/filelock; SI-15 assigns this package's name).
package execworkspace
