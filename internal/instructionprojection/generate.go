package instructionprojection

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// projectionsDirRel is the store-relative directory holding one
// generated manifest per adapter: a sibling of policies/, overlays/,
// exemptions/, and profiles/ under .verdi/policy/ (contract-fixed path).
//
// This extends .verdi/policy/'s own directory grammar
// (internal/policyartifact.ClassifyPolicyPath, restated as
// internal/policyauthority's knownPolicyDirs), and internal/
// policyauthority.Load currently fails closed on ANY unrecognized
// directory under .verdi/policy/ — including this one. Writing here is
// nonetheless the contracted path for this package (an out-of-write-set
// change to internal/policyauthority is the only fix; this package must
// not make it — see the package-level conflict disclosure in doc.go and
// this build's final report). Confirmed by experiment: once this
// directory exists, ANY subsequent policyauthority.Load(root) call
// (including this package's own Verify, or a second Generate) fails
// with "unexpected directory \"projections\"" rather than succeeding or
// returning ErrNotAdopted.
const projectionsDirRel = ".verdi/policy/projections"

// Result reports exactly what Generate wrote: per adapter, every managed
// file's repo-relative path and content digest, plus the manifest's own
// repo-relative path and digest.
type Result struct {
	Adapters []AdapterResult
}

// AdapterResult is one adapter's generated output.
type AdapterResult struct {
	AdapterID      string
	AdapterVersion string
	Files          []FileDigest
	ManifestPath   string
	ManifestDigest string
}

// FileDigest is one written file's repo-relative path and content
// address ("sha256:"+hex of its exact bytes).
type FileDigest struct {
	Path   string
	Digest string
}

// Generate loads and resolves root's constitution store
// (internal/policyauthority.Load + Resolve), then renders and atomically
// writes every adapter's managed projection files and one manifest per
// adapter, sorted by adapter id. A legacy store (no .verdi/policy/)
// returns policyauthority.ErrNotAdopted unchanged and writes nothing — a
// project that has not adopted a constitution has no policy to project.
func Generate(root string) (*Result, error) {
	store, err := policyauthority.Load(root)
	if err != nil {
		return nil, err
	}
	ep, err := policyauthority.Resolve(store)
	if err != nil {
		return nil, fmt.Errorf("instructionprojection: %w", err)
	}
	return generate(root, store.Constitution, store.Policies, ep)
}

// generate is Generate's store-agnostic core: it never calls Load
// itself, so tests (and Verify's own reuse of buildProjectionInput) can
// drive it directly from an already-resolved store without repeating the
// Load that .verdi/policy/projections/ would break on a second call.
func generate(root string, c *policyartifact.Constitution, policies map[string]*policyartifact.Policy, ep *policyauthority.EffectivePolicy) (*Result, error) {
	in, err := buildProjectionInput(policies, ep)
	if err != nil {
		return nil, err
	}

	adapters := sortedAdapters(c.Adapters)

	res := &Result{}
	for _, adapter := range adapters {
		content := renderProjection(adapter, in)
		contentDig := contentDigest(content)

		files := make([]FileDigest, 0, len(adapter.Managed))
		for _, rel := range adapter.Managed {
			full := filepath.Join(root, filepath.FromSlash(rel))
			if err := atomicfile.Write(full, content, 0o644); err != nil {
				return nil, fmt.Errorf("instructionprojection: adapter %s: writing %s: %w", adapter.ID, rel, err)
			}
			files = append(files, FileDigest{Path: rel, Digest: contentDig})
		}

		m := buildManifest(adapter, in, files)
		mBytes, err := manifestBytes(m)
		if err != nil {
			return nil, fmt.Errorf("instructionprojection: adapter %s: canonicalizing manifest: %w", adapter.ID, err)
		}
		manifestRel := adapterManifestRelPath(adapter.ID)
		manifestFull := filepath.Join(root, filepath.FromSlash(manifestRel))
		if err := atomicfile.Write(manifestFull, mBytes, 0o644); err != nil {
			return nil, fmt.Errorf("instructionprojection: adapter %s: writing manifest: %w", adapter.ID, err)
		}

		res.Adapters = append(res.Adapters, AdapterResult{
			AdapterID:      adapter.ID,
			AdapterVersion: adapter.Version,
			Files:          files,
			ManifestPath:   manifestRel,
			ManifestDigest: contentDigest(mBytes),
		})
	}
	return res, nil
}

// adapterManifestRelPath returns the repo-relative slash path of
// adapterID's own manifest.
func adapterManifestRelPath(adapterID string) string {
	return projectionsDirRel + "/" + adapterID + ".json"
}

// sortedAdapters returns a copy of adapters sorted by id.
// policyartifact.DecodeConstitution already normalizes this order, but
// the contract's own "sorted by id" rule is restated here defensively —
// a caller handed an unsealed or hand-built Constitution must still get
// the canonical order, never Go's incidental slice order (CO-3).
func sortedAdapters(adapters []policyartifact.Adapter) []policyartifact.Adapter {
	out := append([]policyartifact.Adapter{}, adapters...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
