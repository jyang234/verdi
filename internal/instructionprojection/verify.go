package instructionprojection

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// Reason is one of the fixed, closed set of finding classifications
// Verify ever reports.
type Reason string

const (
	// ReasonDrift: a managed file exists but its bytes differ from the
	// freshly regenerated content.
	ReasonDrift Reason = "drift"
	// ReasonTruncated: a drift whose on-disk bytes are a proper prefix
	// of the regenerated content (a strict subclass of drift).
	ReasonTruncated Reason = "truncated"
	// ReasonMissing: a managed file or manifest is absent — or present
	// only as a symlink, which is never treated as the real file (its
	// target is outside proof).
	ReasonMissing Reason = "missing"
	// ReasonManifestDrift: a manifest exists but differs from the
	// freshly recomputed canonical manifest.
	ReasonManifestDrift Reason = "manifest-drift"
	// ReasonUnmanaged: the discovery walk found a file whose basename is
	// a declared discovery filename, at repo ROOT level, whose path is
	// not in that adapter's managed set.
	ReasonUnmanaged Reason = "unmanaged"
	// ReasonShadowing: the same as ReasonUnmanaged, but nested in a
	// subdirectory.
	ReasonShadowing Reason = "shadowing"
	// ReasonOrphanManifest: an entry under .verdi/policy/projections/
	// that is not the manifest of any currently declared adapter — a
	// record of a projection nothing regenerates and nothing verifies
	// (typically left behind by a removed or renamed adapter).
	ReasonOrphanManifest Reason = "orphan-manifest"
	// ReasonIncompleteDiscovery: the walk (or a targeted read) could not
	// fully enumerate or read some part of the tree — the chain is
	// unproven, never a pass.
	ReasonIncompleteDiscovery Reason = "incomplete-discovery"
)

// Finding is one witnessed departure from a clean projection state.
// Adapter is "" for a finding attributable to no single adapter's own
// chain: a walk-level incomplete-discovery error, or an orphan manifest
// whose adapter no longer exists to attribute it to. Expected/Actual
// are content digests, populated only where a byte comparison produced
// the finding (drift, truncated, manifest-drift); Detail carries free-
// text context (a walk or read error, or why a symlink was refused).
type Finding struct {
	Adapter  string
	Code     Reason
	Path     string
	Expected string
	Actual   string
	Detail   string
}

// Report is Verify's fail-closed result.
type Report struct {
	Findings []Finding
	// ExcludedSubtrees names every subtree the discovery walk never
	// entered, sorted, and is ALWAYS populated — on a clean report as
	// much as a failing one. It is a disclosure, not a verdict: what
	// this proof did not examine is stated rather than left silent
	// (CO-1). See discovery.go's excludedSubtrees for each rule and its
	// grounding.
	ExcludedSubtrees []string
}

// Clean reports whether r witnessed zero findings of any kind — the
// only state authoritative launch may treat as a proven, drift-free
// projection chain. It is deliberately findings-based: ExcludedSubtrees
// is always non-empty and never makes a report unclean, because those
// subtrees are excluded by contract, not by a failure to examine them.
func (r *Report) Clean() bool {
	return r != nil && len(r.Findings) == 0
}

// Verify loads and resolves root's constitution store, then recomputes
// every adapter's projection content and manifest from that CURRENT
// resolution — never from what a previously written manifest merely
// claims — and classifies every difference between that recomputation
// and the files actually on disk. A legacy store returns
// policyauthority.ErrNotAdopted unchanged.
func Verify(root string) (*Report, error) {
	store, err := policyauthority.Load(root)
	if err != nil {
		return nil, err
	}
	ep, err := policyauthority.Resolve(store)
	if err != nil {
		return nil, fmt.Errorf("instructionprojection: %w", err)
	}
	return verify(root, store, ep)
}

// verify is Verify's store-agnostic core: it never performs a Load of
// its own, so a caller that already holds a resolved Store and
// EffectivePolicy gets the same verdict from that same authority without
// re-reading the store. Verify's public entry point delegates here once
// its own Load+Resolve succeeds. Every adapter's expected content and
// manifest bytes come from the one shared Render seam (design §7,
// SI-87(c)), using every one of ep's own policy ids as the full
// selection, before the read-only comparisons below run unchanged.
func verify(root string, store *policyauthority.Store, ep *policyauthority.EffectivePolicy) (*Report, error) {
	adapters := sortedAdapters(store.Constitution.Adapters)

	// An overlapping managed path is an unsatisfiable constitution, not
	// a drift: reporting findings against the FILES would point a reader
	// at a file no regeneration can fix. Fail closed naming the
	// constitution's own conflict instead (see managedPathOwners).
	owners, err := managedPathOwners(adapters)
	if err != nil {
		return nil, err
	}
	// A symlinked ANCESTOR is not a drift either: the bytes behind it may
	// match exactly, and reporting a clean chain for a file that does not
	// live in this repository is the one outcome this package must never
	// produce. Refuse before reading anything (symlink.go). A symlinked
	// FINAL path stays a named finding — the per-file passes below own
	// that disposition, and it is equally fail-closed.
	if err := checkProjectionPathsSafe(root, adapters, false); err != nil {
		return nil, err
	}
	managed := make(map[string]bool, len(owners))
	for rel := range owners {
		managed[rel] = true
	}

	sel := fullSelection(ep)

	var findings []Finding

	for _, adapter := range adapters {
		rendered, err := Render(store, ep, adapter, sel)
		if err != nil {
			return nil, fmt.Errorf("instructionprojection: adapter %s: %w", adapter.ID, err)
		}

		for _, rf := range rendered.Files {
			full := filepath.Join(root, filepath.FromSlash(rf.Path))
			if f := verifyManagedFile(full, rf.Path, rf.Content, rf.Digest); f != nil {
				f.Adapter = adapter.ID
				findings = append(findings, *f)
			}
		}

		manifestRel := adapterManifestRelPath(adapter.ID)
		manifestFull := filepath.Join(root, filepath.FromSlash(manifestRel))
		if f := verifyManifestFile(manifestFull, manifestRel, rendered.Manifest); f != nil {
			f.Adapter = adapter.ID
			findings = append(findings, *f)
		}
	}

	findings = append(findings, orphanManifestFindings(root, adapters)...)

	discoveryFindings, err := discover(root, adapters, managed)
	if err != nil {
		return nil, err
	}
	findings = append(findings, discoveryFindings...)

	sortFindings(findings)
	return &Report{Findings: findings, ExcludedSubtrees: excludedSubtrees()}, nil
}

// orphanManifestFindings enumerates .verdi/policy/projections/ and
// reports every entry that is not a currently declared adapter's own
// manifest. Without this pass the per-adapter checks above only ever ask
// whether the manifests that SHOULD exist do; a manifest belonging to a
// removed or renamed adapter would then verify clean forever, leaving an
// authority-shaped record of a projection nothing regenerates and
// nothing checks (CO-1). An absent directory is not a finding here (the
// per-adapter pass already reports each missing manifest); a directory
// that exists but cannot be read is incomplete-discovery, never assumed
// empty. Any entry that is not a currently declared adapter's manifest
// is an orphan. Through the PUBLIC Verify the reachable orphan domain is
// exactly <kebab>.json files for undeclared adapters: policyauthority's
// stricter grammar already rejects a stray subdirectory, non-.json file,
// or non-kebab name at Load, before this pass runs — this function's
// broader classification is defense-in-depth behind that gate, not an
// independently reachable surface.
func orphanManifestFindings(root string, adapters []policyartifact.Adapter) []Finding {
	dir := filepath.Join(root, filepath.FromSlash(projectionsDirRel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []Finding{{Code: ReasonIncompleteDiscovery, Path: projectionsDirRel, Detail: err.Error()}}
	}
	expected := make(map[string]bool, len(adapters))
	for _, a := range adapters {
		expected[a.ID+manifestExt] = true
	}
	var findings []Finding
	for _, e := range entries {
		if expected[e.Name()] {
			continue
		}
		findings = append(findings, Finding{
			Code:   ReasonOrphanManifest,
			Path:   projectionsDirRel + "/" + e.Name(),
			Detail: "no adapter currently declared by the constitution owns this manifest",
		})
	}
	return findings
}

// verifyManagedFile compares full's on-disk state against wantContent,
// returning nil when it byte-matches exactly.
func verifyManagedFile(full, rel string, wantContent []byte, wantDigest string) *Finding {
	lst, err := os.Lstat(full)
	switch {
	case os.IsNotExist(err):
		return &Finding{Code: ReasonMissing, Path: rel}
	case err != nil:
		return &Finding{Code: ReasonIncompleteDiscovery, Path: rel, Detail: err.Error()}
	}
	if lst.Mode()&os.ModeSymlink != 0 {
		return &Finding{Code: ReasonMissing, Path: rel, Detail: "path is a symlink; a symlink target is outside proof and is never treated as the managed file"}
	}
	got, rerr := os.ReadFile(full)
	if rerr != nil {
		return &Finding{Code: ReasonIncompleteDiscovery, Path: rel, Detail: rerr.Error()}
	}
	if bytes.Equal(got, wantContent) {
		return nil
	}
	gotDigest := contentDigest(got)
	if len(got) < len(wantContent) && bytes.Equal(wantContent[:len(got)], got) {
		return &Finding{Code: ReasonTruncated, Path: rel, Expected: wantDigest, Actual: gotDigest}
	}
	return &Finding{Code: ReasonDrift, Path: rel, Expected: wantDigest, Actual: gotDigest}
}

// verifyManifestFile compares full's on-disk state against want, the
// freshly recomputed canonical manifest bytes.
func verifyManifestFile(full, rel string, want []byte) *Finding {
	lst, err := os.Lstat(full)
	switch {
	case os.IsNotExist(err):
		return &Finding{Code: ReasonMissing, Path: rel}
	case err != nil:
		return &Finding{Code: ReasonIncompleteDiscovery, Path: rel, Detail: err.Error()}
	}
	if lst.Mode()&os.ModeSymlink != 0 {
		return &Finding{Code: ReasonMissing, Path: rel, Detail: "path is a symlink; a symlink target is outside proof and is never treated as the manifest"}
	}
	got, rerr := os.ReadFile(full)
	if rerr != nil {
		return &Finding{Code: ReasonIncompleteDiscovery, Path: rel, Detail: rerr.Error()}
	}
	if bytes.Equal(got, want) {
		return nil
	}
	return &Finding{Code: ReasonManifestDrift, Path: rel, Expected: contentDigest(want), Actual: contentDigest(got)}
}

// sortFindings orders findings by (adapter, code, path) — the
// contract's own deterministic finding order (CO-3).
func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Adapter != findings[j].Adapter {
			return findings[i].Adapter < findings[j].Adapter
		}
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		return findings[i].Path < findings[j].Path
	})
}
