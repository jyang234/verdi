package gitx

import "context"

// ReachableFromHEAD reports whether commit is head itself or a real
// ancestor of head in dir's git history, treating a commit that does not
// resolve to any real object at all — e.g. one that lived only on a
// since-deleted, since-garbage-collected branch — as simply "not
// reachable" (false, nil) rather than an error.
//
// This is deliberately more forgiving than IsAncestor, which reports an
// unresolvable commit as an error so a caller expecting an already-real
// commit can tell "no" from "I can't tell" — the exact distinction
// spec/evidence-resilience's X-15 witness shows is wrong for a consumer
// reading synced evidence: the closure gate's ancestry check hard-failed
// operationally (git's own "fatal: Not a valid commit name") the moment a
// synced CI bundle carried a record referencing a commit whose source
// branch had since been deleted — a routine, expected shape, not an
// operational anomaly. ReachableFromHEAD folds that case into an honest
// "not reachable" instead, exactly like any other real-but-non-ancestor
// commit already reads.
//
// It is built from CommitExists (object presence — satisfiable by a
// locally-dangling object no ref reaches, X-11b's exact false green) plus
// IsAncestor (real ancestry), composed so that BOTH "the object does not
// exist at all" and "the object exists but no ref reaches it" read as the
// same honest false — the single "reachable from HEAD" predicate
// VL-009 (internal/lint/vl009.go), sync's quarantine check
// (cmd/verdi/sync_quarantine.go), and the closure gate's evidence loader
// (internal/evidence/records.go) all now share, rather than each
// approximating it differently.
//
// A dir that is not a git repository at all is still a real, surfaced
// error — only a resolvable-but-unreachable commit is folded into the
// false case.
func ReachableFromHEAD(ctx context.Context, dir, commit, head string) (bool, error) {
	exists, err := CommitExists(ctx, dir, commit)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	return IsAncestor(ctx, dir, commit, head)
}
