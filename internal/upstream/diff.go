package upstream

import (
	"fmt"
	"sort"
)

// DiffOp is a boundary-diff entry's operation.
type DiffOp string

const (
	DiffAdd    DiffOp = "add"
	DiffRemove DiffOp = "remove"
)

// BoundaryDiffEntry is one entry of a computed boundary diff, mirroring
// upstream's ContractChange shape (PLAN.md I-3, revised by spike S1:
// "groundwork diff emits plain text only — no --json flag exists"; verdi
// computes this itself from two strict-decoded contracts rather than
// parsing upstream's text view).
type BoundaryDiffEntry struct {
	Op       DiffOp `json:"op"`
	Surface  string `json:"surface"`
	Name     string `json:"name"`
	Breaking bool   `json:"breaking"`
}

// Surface names used by ComputeBoundaryDiff, chosen to match spike S1's
// captured `groundwork diff` text output verbatim ("+ route GET /healthz",
// "+ dependency audit-svc (http)") for the two surfaces that capture
// directly evidences; "published" and "consumed" extend the same treatment
// to the contract's other named-resource arrays for completeness.
const (
	SurfaceRoute      = "route"
	SurfaceDependency = "dependency"
	SurfacePublished  = "published"
	SurfaceConsumed   = "consumed"
)

// ComputeBoundaryDiff computes the boundary diff from base to branch: every
// route, dependency, published, or consumed resource present in one
// contract but not the other becomes one entry. Per I-3 ("mark removals
// breaking", matching spike S1's captured text output where only the "-"
// line carried "⚠ BREAKING"): an entry present in base but absent from
// branch (a removal) is breaking; an entry present in branch but absent
// from base (an addition) is not. Entries present in both contracts are
// not diffed at all — there is no "changed" op because none of the
// resource shapes this package models (route: method+route; the named
// resources: name+kind) has a mutable body once its identity fields match.
//
// Results are sorted by (surface, op, name) for determinism — a
// requirement for the byte-identical canonjson goldens the assembly
// package produces.
func ComputeBoundaryDiff(base, branch *BoundaryContract) []BoundaryDiffEntry {
	var entries []BoundaryDiffEntry

	entries = append(entries, diffRoutes(base.Entrypoints.HTTP, branch.Entrypoints.HTTP)...)
	entries = append(entries, diffNamedResources(SurfaceDependency, base.ExternalDependencies, branch.ExternalDependencies)...)
	entries = append(entries, diffNamedResources(SurfacePublished, base.Published, branch.Published)...)
	entries = append(entries, diffNamedResources(SurfaceConsumed, base.Consumed, branch.Consumed)...)

	sortDiffEntries(entries)
	return entries
}

func diffRoutes(base, branch []HTTPEntrypoint) []BoundaryDiffEntry {
	baseSet := make(map[string]bool, len(base))
	for _, e := range base {
		baseSet[routeName(e)] = true
	}
	branchSet := make(map[string]bool, len(branch))
	for _, e := range branch {
		branchSet[routeName(e)] = true
	}

	var out []BoundaryDiffEntry
	for name := range baseSet {
		if !branchSet[name] {
			out = append(out, BoundaryDiffEntry{Op: DiffRemove, Surface: SurfaceRoute, Name: name, Breaking: true})
		}
	}
	for name := range branchSet {
		if !baseSet[name] {
			out = append(out, BoundaryDiffEntry{Op: DiffAdd, Surface: SurfaceRoute, Name: name, Breaking: false})
		}
	}
	return out
}

func routeName(e HTTPEntrypoint) string { return fmt.Sprintf("%s %s", e.Method, e.Route) }

func diffNamedResources(surface string, base, branch []NamedResource) []BoundaryDiffEntry {
	baseSet := make(map[string]bool, len(base))
	for _, r := range base {
		baseSet[resourceName(r)] = true
	}
	branchSet := make(map[string]bool, len(branch))
	for _, r := range branch {
		branchSet[resourceName(r)] = true
	}

	var out []BoundaryDiffEntry
	for name := range baseSet {
		if !branchSet[name] {
			out = append(out, BoundaryDiffEntry{Op: DiffRemove, Surface: surface, Name: name, Breaking: true})
		}
	}
	for name := range branchSet {
		if !baseSet[name] {
			out = append(out, BoundaryDiffEntry{Op: DiffAdd, Surface: surface, Name: name, Breaking: false})
		}
	}
	return out
}

func resourceName(r NamedResource) string {
	if r.Kind == "" {
		return r.Name
	}
	return fmt.Sprintf("%s (%s)", r.Name, r.Kind)
}

// HasBreaking reports whether any entry in diffs is breaking — the value
// verdi cross-checks against `groundwork diff`'s own exit code (0 clean,
// 1 breaking) in tests and CI (I-3).
func HasBreaking(diffs []BoundaryDiffEntry) bool {
	for _, d := range diffs {
		if d.Breaking {
			return true
		}
	}
	return false
}

func sortDiffEntries(entries []BoundaryDiffEntry) {
	sort.Slice(entries, func(i, j int) bool { return diffLess(entries[i], entries[j]) })
}

// diffLess orders diff entries by (surface, op, name) — the total order
// ComputeBoundaryDiff's results are sorted into, for deterministic,
// byte-identical canonjson output.
func diffLess(a, b BoundaryDiffEntry) bool {
	if a.Surface != b.Surface {
		return a.Surface < b.Surface
	}
	if a.Op != b.Op {
		return a.Op < b.Op
	}
	return a.Name < b.Name
}
