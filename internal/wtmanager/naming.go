package wtmanager

import (
	"path/filepath"
	"strings"
)

// designPrefix is the one, direct, collision-free branch<->path mapping
// dc-1 chooses over any hash or second slugging scheme: a design branch's
// own spec name is already globally unique (02 §Identity and references,
// VL-002), so trimming this fixed prefix is the entire naming contract.
const designPrefix = "design/"

// worktreeName returns the deterministic directory name dc-1 maps branch
// to — branch's own spec name, i.e. branch with its "design/" prefix
// trimmed. Every managed worktree this package ever cuts is for a design
// branch (dc-1); a branch already lacking the prefix is returned
// unchanged; there is no second scheme to invent.
func worktreeName(branch string) string {
	return strings.TrimPrefix(branch, designPrefix)
}

// branchForWorktreeName inverts worktreeName — used by GC, which
// discovers managed worktrees by directory name and must recover the
// design branch each one is bound to.
func branchForWorktreeName(name string) string {
	return designPrefix + name
}

// worktreesRoot is the data-zone directory (co-1) every managed worktree
// and its lockfile live under: root/.verdi/data/worktrees/.
func worktreesRoot(root string) string {
	return filepath.Join(root, ".verdi", "data", "worktrees")
}

// worktreePath returns the deterministic path for branch's managed
// worktree under root.
func worktreePath(root, branch string) string {
	return filepath.Join(worktreesRoot(root), worktreeName(branch))
}

// WorktreePath is worktreePath exported read-only: the deterministic
// filesystem location EnsureWorktree cuts and reuses for branch under root
// (dc-1's one branch<->path mapping), for callers that must ADDRESS a
// managed worktree's tree without cutting one. Pure — it computes the path
// and touches nothing; the worktree may or may not exist on disk. Single-
// sourcing this mapping keeps a consumer (e.g. the workbench's diagram
// editor, resolving a per-branch board's exit target against the store that
// board addresses) from copy-pasting the naming contract.
func WorktreePath(root, branch string) string {
	return worktreePath(root, branch)
}

// lockPath returns branch's managed worktree's own lockfile path — a
// sibling of its worktree directory, never inside it (dc-2).
func lockPath(root, branch string) string {
	return worktreePath(root, branch) + ".lock"
}
