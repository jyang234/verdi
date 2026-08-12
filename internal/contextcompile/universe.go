// This file builds the raw, closed candidate universe (authority design
// §5): the deterministic union of every path/ref/opaque candidate the
// compiler may later classify. It performs no semantic classification and
// reads no committed, uncommitted, or store content — only the path/ref
// identities gitx and the caller's already-resolved authority lifts hand
// in.
package contextcompile

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// dataZonePrefix is the one excluded subtree boundary (authority design
// §5): "`.verdi/data/` is represented by one excluded subtree-boundary
// candidate; its descendants are neither enumerated nor named." Any
// head-tree or worktree-overlay path under this prefix collapses into the
// single dataZoneBoundaryPath candidate below instead of contributing its
// own candidate.
const dataZonePrefix = ".verdi/data/"

// dataZoneBoundaryPath is the fixed logical path the collapsed boundary
// candidate is addressed by.
const dataZoneBoundaryPath = ".verdi/data"

// Candidate is the compiler-local raw candidate the universe union
// produces before any applicability or classification decision (authority
// design §5). Object/Mode/Type are set only for head-tree candidates; every
// other source carries none of the three, since no other source reads a
// blob.
type Candidate struct {
	Source Source
	ID     string
	Path   string
	Ref    string
	Object string
	Mode   string
	Type   string
}

// UniverseInput is BuildUniverse's complete, caller-supplied raw material.
// LiftedStorePaths and LiftedContextPaths map a head-tree repo-relative
// path to the canonical ref its content is already represented under in
// the store-authority/declared-context sources (authority design §5: "a
// tracked path lifted here is not duplicated as a repository-file
// candidate"). ProjectionPaths names every file the pure renderer produced
// for the selected adapter (path only — Task 3 builds no projection
// content).
type UniverseInput struct {
	Head               string
	Tree               []gitx.TreeEntry
	WorktreePaths      []string
	LiftedStorePaths   map[string]string
	LiftedContextPaths map[string]string
	ProjectionPaths    []string
	Adapter            policyartifact.Adapter
}

// BuildUniverse builds the deterministic union of every candidate the
// compiler may classify (authority design §5). It performs no forbidden
// read: head-tree candidates carry the object/mode/type gitx already
// resolved from `git ls-tree`, worktree-overlay candidates carry only path
// identity, and store-authority/declared-context/opaque candidates carry
// only the ref/adapter identity the caller resolved elsewhere. Candidate
// order is not a return-value contract; callers that need a stable order
// sort by (Source, ID) themselves.
func BuildUniverse(in UniverseInput) ([]Candidate, error) {
	if in.Adapter.ID == "" || in.Adapter.Version == "" {
		return nil, fmt.Errorf("contextcompile: BuildUniverse: adapter id and version are both required for the opaque harness-vendor-base candidate")
	}

	lift, contextLifts, err := resolveLifts(in.LiftedStorePaths, in.LiftedContextPaths)
	if err != nil {
		return nil, err
	}

	var candidates []Candidate

	storeCandidates, err := liftedCandidates(SourceStoreAuthority, in.LiftedStorePaths)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, storeCandidates...)

	contextCandidates, err := liftedCandidates(SourceDeclaredContext, contextLifts)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, contextCandidates...)

	sawDataZone := false
	seenHeadTree := map[string]bool{}
	for _, e := range in.Tree {
		if err := validateCandidatePath(e.Path); err != nil {
			return nil, fmt.Errorf("contextcompile: BuildUniverse: head-tree: %w", err)
		}
		if inDataZone(e.Path) {
			sawDataZone = true
			continue
		}
		if lift[e.Path] {
			// Lifted into store-authority or declared-context: no separate
			// repository-file candidate (authority design §5).
			continue
		}
		if seenHeadTree[e.Path] {
			return nil, fmt.Errorf("contextcompile: BuildUniverse: head-tree: duplicate path %q", e.Path)
		}
		seenHeadTree[e.Path] = true
		candidates = append(candidates, Candidate{
			Source: SourceHeadTree,
			ID:     pathID(e.Path),
			Path:   e.Path,
			Object: e.Object,
			Mode:   e.Mode,
			Type:   e.Type,
		})
	}

	seenWorktree := map[string]bool{}
	for _, p := range in.WorktreePaths {
		if err := validateCandidatePath(p); err != nil {
			return nil, fmt.Errorf("contextcompile: BuildUniverse: worktree-overlay: %w", err)
		}
		if inDataZone(p) {
			sawDataZone = true
			continue
		}
		if seenWorktree[p] {
			return nil, fmt.Errorf("contextcompile: BuildUniverse: worktree-overlay: duplicate path %q", p)
		}
		seenWorktree[p] = true
		candidates = append(candidates, Candidate{
			Source: SourceWorktreeOverlay,
			ID:     pathID(p),
			Path:   p,
		})
	}

	if sawDataZone {
		candidates = append(candidates, Candidate{
			Source: SourceWorktreeOverlay,
			ID:     pathID(dataZoneBoundaryPath),
			Path:   dataZoneBoundaryPath,
		})
	}

	seenProjection := map[string]bool{}
	for _, p := range in.ProjectionPaths {
		if err := validateCandidatePath(p); err != nil {
			return nil, fmt.Errorf("contextcompile: BuildUniverse: projection: %w", err)
		}
		if seenProjection[p] {
			return nil, fmt.Errorf("contextcompile: BuildUniverse: projection: duplicate path %q", p)
		}
		seenProjection[p] = true
		candidates = append(candidates, Candidate{
			Source: SourceProjection,
			ID:     pathID(p),
			Path:   p,
		})
	}

	candidates = append(candidates, Candidate{
		Source: SourceOpaque,
		ID:     fmt.Sprintf("opaque:%s/%s/%s", OpaqueKindHarnessVendorBase, in.Adapter.ID, in.Adapter.Version),
	})

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Source != candidates[j].Source {
			return candidates[i].Source < candidates[j].Source
		}
		return candidates[i].ID < candidates[j].ID
	})

	return candidates, nil
}

// resolveLifts merges the store and declared-context lift maps into one
// path->lifted set and returns the effective declared-context lift map that
// candidate construction must use. Authority design §5 fixes the source
// precedence store-authority > declared-context > head-tree, so a path
// claimed by both lift maps is not a conflict: store authority wins it and
// the declared-context lift for that path is suppressed here, before any
// candidate is built. Suppression is per path, not per ref — an uncontested
// second path lifting to the same ref still yields that declared-context
// candidate. Every other lift validation (canonical path, non-empty ref)
// still fails closed for both maps, and the merged lifted set continues to
// suppress the head-tree duplicate for the path.
func resolveLifts(store, ctx map[string]string) (map[string]bool, map[string]string, error) {
	lifted := make(map[string]bool, len(store)+len(ctx))
	for path, ref := range store {
		if err := validateCandidatePath(path); err != nil {
			return nil, nil, fmt.Errorf("contextcompile: BuildUniverse: store-authority lift: %w", err)
		}
		if ref == "" {
			return nil, nil, fmt.Errorf("contextcompile: BuildUniverse: store-authority lift: path %q has an empty ref", path)
		}
		lifted[path] = true
	}
	effectiveCtx := make(map[string]string, len(ctx))
	for path, ref := range ctx {
		if err := validateCandidatePath(path); err != nil {
			return nil, nil, fmt.Errorf("contextcompile: BuildUniverse: declared-context lift: %w", err)
		}
		if ref == "" {
			return nil, nil, fmt.Errorf("contextcompile: BuildUniverse: declared-context lift: path %q has an empty ref", path)
		}
		if _, storeOwned := store[path]; storeOwned {
			// Store authority outranks declared context for this path
			// (authority design §5): drop the declared-context lift rather
			// than emitting a second candidate for the same path.
			continue
		}
		effectiveCtx[path] = ref
		lifted[path] = true
	}
	return lifted, effectiveCtx, nil
}

// liftedCandidates turns one lift map's distinct ref values into sorted,
// deduplicated ref-addressed candidates for source. Two different paths
// legally lift into the same ref (e.g. two obligations resolving from one
// accepted-spec file), so ref — not path — is the dedup key; the source
// candidate itself carries no path (authority design §5: "artifact-backed
// entries use ref:<canonical-ref>").
func liftedCandidates(source Source, lifts map[string]string) ([]Candidate, error) {
	seen := map[string]bool{}
	var refs []string
	for _, ref := range lifts {
		if seen[ref] {
			continue
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	sort.Strings(refs)

	candidates := make([]Candidate, 0, len(refs))
	for _, ref := range refs {
		candidates = append(candidates, Candidate{
			Source: source,
			ID:     refID(ref),
			Ref:    ref,
		})
	}
	return candidates, nil
}

// inDataZone reports whether p falls under the one excluded `.verdi/data/`
// subtree boundary.
func inDataZone(p string) bool {
	return strings.HasPrefix(p, dataZonePrefix)
}

// pathID and refID spell the two canonical logical-ID forms authority
// design §5 fixes: "repository-backed entries use path:<repo-relative-
// path>, artifact-backed entries use ref:<canonical-ref>".
func pathID(p string) string { return "path:" + p }
func refID(r string) string  { return "ref:" + r }

// validateCandidatePath fails closed on any path grammar BuildUniverse
// cannot safely address: absolute paths, `.` / `..` segments, doubled
// separators, and a leading/trailing separator all break the path:<path>
// identity contract or would let a candidate escape the repository it was
// discovered in.
func validateCandidatePath(p string) error {
	if p == "" {
		return fmt.Errorf("empty candidate path")
	}
	if strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") {
		return fmt.Errorf("noncanonical candidate path %q: leading or trailing separator", p)
	}
	if strings.Contains(p, "\\") {
		return fmt.Errorf("noncanonical candidate path %q: backslash separator", p)
	}
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "":
			return fmt.Errorf("noncanonical candidate path %q: empty path segment", p)
		case ".", "..":
			return fmt.Errorf("noncanonical candidate path %q: %q segment", p, seg)
		}
	}
	return nil
}
