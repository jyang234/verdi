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
	// ReasonIncompleteDiscovery: the walk (or a targeted read) could not
	// fully enumerate or read some part of the tree — the chain is
	// unproven, never a pass.
	ReasonIncompleteDiscovery Reason = "incomplete-discovery"
)

// Finding is one witnessed departure from a clean projection state.
// Adapter is "" for a walk-level incomplete-discovery finding that is
// not attributable to any single adapter's own chain. Expected/Actual
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
}

// Clean reports whether r witnessed zero findings of any kind — the
// only state authoritative launch may treat as a proven, drift-free
// projection chain.
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
	return verify(root, store.Constitution, store.Policies, ep)
}

// verify is Verify's store-agnostic core. Splitting it out lets a caller
// supply a Store/EffectivePolicy obtained from a Load that happened
// BEFORE .verdi/policy/projections/ existed on disk — the only way this
// package's own tests can exercise a true generate-then-verify round
// trip given the confirmed conflict documented at generate.go's
// projectionsDirRel: the public Verify's own Load call fails on any real
// root a prior Generate has already touched, because
// policyauthority.Load rejects "projections" as an unrecognized
// directory under .verdi/policy/. verify itself never calls Load, so it
// is unaffected and is exactly what Verify's public entry point
// delegates to once its own Load+Resolve succeeds.
func verify(root string, c *policyartifact.Constitution, policies map[string]*policyartifact.Policy, ep *policyauthority.EffectivePolicy) (*Report, error) {
	in, err := buildProjectionInput(policies, ep)
	if err != nil {
		return nil, err
	}

	adapters := sortedAdapters(c.Adapters)

	managed := make(map[string]map[string]bool, len(adapters))
	for _, a := range adapters {
		set := make(map[string]bool, len(a.Managed))
		for _, rel := range a.Managed {
			set[rel] = true
		}
		managed[a.ID] = set
	}

	var findings []Finding

	for _, adapter := range adapters {
		content := renderProjection(adapter, in)
		wantDigest := contentDigest(content)

		files := make([]FileDigest, 0, len(adapter.Managed))
		for _, rel := range adapter.Managed {
			full := filepath.Join(root, filepath.FromSlash(rel))
			if f := verifyManagedFile(full, rel, content, wantDigest); f != nil {
				f.Adapter = adapter.ID
				findings = append(findings, *f)
			}
			files = append(files, FileDigest{Path: rel, Digest: wantDigest})
		}

		m := buildManifest(adapter, in, files)
		wantManifest, merr := manifestBytes(m)
		if merr != nil {
			return nil, fmt.Errorf("instructionprojection: adapter %s: canonicalizing manifest: %w", adapter.ID, merr)
		}
		manifestRel := adapterManifestRelPath(adapter.ID)
		manifestFull := filepath.Join(root, filepath.FromSlash(manifestRel))
		if f := verifyManifestFile(manifestFull, manifestRel, wantManifest); f != nil {
			f.Adapter = adapter.ID
			findings = append(findings, *f)
		}
	}

	discoveryFindings, err := discover(root, adapters, managed)
	if err != nil {
		return nil, err
	}
	findings = append(findings, discoveryFindings...)

	sortFindings(findings)
	return &Report{Findings: findings}, nil
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
