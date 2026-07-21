package gitx

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Reachability is the three-valued result of a "reachable from HEAD" query in
// a repository that may be shallow. A binary bool cannot stay honest across a
// shallow clone: absence of a commit below the shallow horizon is not the same
// fact as a commit's proven non-ancestry in complete history, yet a bool
// collapses both into one "false". Reachability keeps them apart so a shallow
// checkout — GitHub Actions' pull_request checkout is sometimes shallow even
// with fetch-depth: 0 (P2-10b) — can prove YES (real, visible ancestry) but
// never a false NO (a genuinely-reachable commit that merely sits beyond the
// horizon).
type Reachability int

const (
	// Unreachable is a PROVEN negative: in COMPLETE history the commit is
	// head itself's non-ancestor — either it resolves to a real object that
	// no ref reaches (X-11b's dangling object) or it does not resolve at all
	// in a repository that holds its whole history (X-15's since-deleted
	// branch). It is the zero value, so a Reachability that some future code
	// forgets to set reads as the fail-closed "not reachable", never as a
	// silent Reachable. Unreachable is returned ONLY when the repository is
	// not shallow — a full clone's absence IS proof.
	Unreachable Reachability = iota
	// Reachable is a PROVEN positive: commit is head itself or a real,
	// visible ancestor of head. Positive answers are shallow-independent —
	// ancestry that is fully visible within the horizon is real proof — so
	// Reachable is returned identically for a shallow and a full clone.
	Reachable
	// UnprovableShallow is the honest third value: the answer WOULD be a
	// negative (the object is absent, or present but not an ancestor) but the
	// repository is shallow (`git rev-parse --is-shallow-repository`), so the
	// history that could turn the answer positive may simply be below the
	// horizon and unfetched. It is never a proof of unreachability and is
	// never returned for a positive answer. Consumers render it as a
	// disclosed-unproven notice (constitution 2, three-valued honesty),
	// never as a verdict failure and never as a silent exclusion of honest
	// evidence.
	UnprovableShallow
)

// String returns the legible name used in disclosure and test output.
func (r Reachability) String() string {
	switch r {
	case Reachable:
		return "reachable"
	case UnprovableShallow:
		return "unprovable-shallow"
	default:
		return "unreachable"
	}
}

// ReachableFromHEAD reports whether commit is head itself or a real ancestor
// of head in dir's git history, as a three-valued Reachability rather than a
// bool so a SHALLOW repository stays honest (P2-10b): a positive answer is
// real proof (Reachable), a negative answer in a full clone is real proof
// (Unreachable), but a negative answer in a shallow clone is UnprovableShallow
// — never a false claim of unreachability about a commit whose reachable
// history merely sits below the horizon and was never fetched.
//
// It composes CommitExists (object presence — satisfiable by a locally-
// dangling object no ref reaches, X-11b's exact false green) and IsAncestor
// (real ancestry): a commit that both resolves AND is a real ancestor of head
// is Reachable. Every other case is a would-be negative — BOTH "the object
// does not exist at all" and "the object exists but is not an ancestor" — and
// its honesty then depends on whether dir is shallow:
//
//   - a full clone folds the negative into the honest Unreachable, exactly as
//     before (X-11b's dangling pin still reds; X-15's since-deleted branch is
//     still an honest "not reachable", never git's operational "Not a valid
//     commit name"); and
//   - a shallow clone folds it into UnprovableShallow, because absence below
//     the horizon is not proof — the exact GitHub-Actions shallow-checkout
//     shape where a genuinely-reachable frozen.commit read as "not reachable"
//     content-dependently by horizon depth (PRs #186, #192).
//
// This is the single "reachable from HEAD" predicate VL-009 and VL-003
// (internal/lint/vl009.go, vl003.go), sync's quarantine check
// (cmd/verdi/sync_quarantine.go), and the closure gate's evidence loader
// (internal/evidence/records.go) all share, so the asymmetric-honesty rule
// (shallow proves YES, never NO) is decided here once rather than approximated
// per consumer.
//
// A dir that is not a git repository at all is still a real, surfaced error —
// only a resolvable-but-negative or shallow-hidden commit is folded into a
// non-error outcome. Out of scope (disclosed here so the boundary is explicit,
// not silently assumed): a grafted or partial (promisor) clone, where
// `--is-shallow-repository` is false yet objects can still be absent behind a
// graft or a promisor remote; those read as Unreachable exactly as a full
// clone does, because this predicate keys on the shallow marker alone.
func ReachableFromHEAD(ctx context.Context, dir, commit, head string) (Reachability, error) {
	exists, err := CommitExists(ctx, dir, commit)
	if err != nil {
		return Unreachable, err
	}
	if exists {
		anc, err := IsAncestor(ctx, dir, commit, head)
		if err != nil {
			return Unreachable, err
		}
		if anc {
			// Positive proof — shallow-independent.
			return Reachable, nil
		}
	}

	// Would-be negative (object absent, or present but not an ancestor). In a
	// shallow clone this is not proof of unreachability: the history that
	// could flip it positive may be below the horizon and unfetched.
	shallow, err := shallowRepository(ctx, dir)
	if err != nil {
		return Unreachable, err
	}
	if shallow {
		return UnprovableShallow, nil
	}
	return Unreachable, nil
}

// shallowCache memoizes each dir's shallow state. Its correct SCOPE is one
// logical operation: within a single lint/gate/sync run — or one MCP request —
// a repository's shallowness cannot change under verdi, so a single probe per
// dir spares the evidence loops (one query per commit directory / record) a
// redundant `git rev-parse` on every would-be-negative. The probe runs only on
// the negative path, so the healthy common case (a full clone, or an evidence
// record whose commit is a real ancestor) never consults it at all.
//
// For a SHORT-LIVED consumer the process IS one operation, so a process-lifetime
// memo is exactly right: `verdi lint`, the closure gate, and `verdi sync` each
// run once and exit, never reshaping a checkout out from under themselves. But
// not every consumer is short-lived — the earlier "all consumers are short-lived
// CLI/gate runs" claim was false. The persistent MCP server (internal/mcpserve:
// get_matrix routes through evidence.LoadRecords, which calls ReachableFromHEAD)
// serves many requests over a long lifetime, and between two requests an
// external actor CAN reshape its checkout full->shallow (a `git fetch --depth`,
// a re-checkout). The one direction that then goes stale DANGEROUSLY is a cached
// `false` (probed while full): a would-be-negative would read the stale `false`
// and return a PROVEN Unreachable — a false NO — where the now-shallow repo owes
// UnprovableShallow. (A cached `true` gone stale is only over-cautious:
// UnprovableShallow where Unreachable has since become provable — never a false
// claim, so it needs no cure.) Long-lived consumers therefore call
// ResetShallowCache at each operation boundary; short-lived ones never do,
// keeping their full memoization.
var shallowCache sync.Map // dir string -> bool

// ResetShallowCache clears the process-global shallow-state memo. A long-lived
// consumer (the persistent MCP server — see shallowCache's doc) calls it at each
// operation boundary so a checkout reshaped full->shallow between operations is
// re-probed fresh, never answered from a stale cached `false` that would turn a
// would-be-negative into a false PROVEN Unreachable. A short-lived CLI/gate run
// never calls it: its whole process is one operation, so its memoization stays
// intact and every consumer keeps proving YES as cheaply as before. Safe to call
// concurrently with in-flight ReachableFromHEAD calls — a cleared entry simply
// re-probes on its next miss (sync.Map semantics); the worst case is a redundant
// `git rev-parse`, never an incorrect result.
func ResetShallowCache() {
	shallowCache.Clear()
}

// shallowRepository reports whether dir is a shallow clone via git's own
// `git rev-parse --is-shallow-repository` predicate — git's supported answer
// to exactly this question, distinct from IsShallow's marker-file stat (which
// serves sync_ancestor.go's fetch-walk truncation disclosure, a different
// concern). It is only ever called once dir is already known to be a real git
// repository (CommitExists surfaced a non-repo dir as an error upstream), so a
// probe failure here is genuinely operational and is surfaced, never guessed.
func shallowRepository(ctx context.Context, dir string) (bool, error) {
	if v, ok := shallowCache.Load(dir); ok {
		return v.(bool), nil
	}
	out, err := run(ctx, dir, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, fmt.Errorf("gitx: shallowRepository(%s): %w", dir, err)
	}
	shallow := strings.TrimSpace(string(out)) == "true"
	shallowCache.Store(dir, shallow)
	return shallow, nil
}
