package instructionprojection

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// projectionsDirRel is the store-relative directory holding one
// generated manifest per adapter: a sibling of policies/, overlays/,
// exemptions/, and profiles/ under .verdi/policy/ (contract-fixed path).
// manifestExt is the extension every manifest in it carries.
//
// The directory is part of .verdi/policy/'s own grammar
// (internal/policyartifact.ClassifyPolicyPath's projection-manifest row,
// restated as internal/policyauthority's knownPolicyDirs), admitted
// there as a GENERATED OUTPUT: policyauthority.Load recognizes the
// directory and deliberately skips its entries rather than reading them
// as authority, because a projection derives from the constitution and
// can never be an input to it (DC-1).
const (
	projectionsDirRel = ".verdi/policy/projections"
	manifestExt       = ".json"
)

// ErrOverlappingManagedPath is returned when two adapters declare the
// same managed projection path. Such a constitution is unsatisfiable:
// each adapter renders its own content (its identity is in the file), so
// one file cannot simultaneously be both adapters' generated projection.
// Writing it for whichever adapter came last would leave the other
// adapter's manifest asserting a digest the disk contradicts — a
// manifest that lies, which is worse than no manifest at all (CO-1). The
// wrapped error names both adapter ids and the path so the constitution,
// not the file, is what a reader is sent to fix.
var ErrOverlappingManagedPath = errors.New("instructionprojection: two adapters declare the same managed projection path")

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
// itself, so a caller that already holds a resolved store (or a test
// that needs authority captured at a specific moment) can drive it
// directly without a second Load.
func generate(root string, c *policyartifact.Constitution, policies map[string]*policyartifact.Policy, ep *policyauthority.EffectivePolicy) (*Result, error) {
	in, err := buildProjectionInput(policies, ep)
	if err != nil {
		return nil, err
	}

	adapters := sortedAdapters(c.Adapters)

	// Prove the whole projection surface is satisfiable BEFORE writing
	// anything: a partial write followed by a failure would leave files
	// on disk that no manifest describes.
	if _, err := managedPathOwners(adapters); err != nil {
		return nil, err
	}

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
	return projectionsDirRel + "/" + adapterID + manifestExt
}

// managedPathOwners maps every managed projection path to the ONE
// adapter that declares it, failing closed with ErrOverlappingManagedPath
// when two adapters claim the same path. Generate calls it before
// writing anything and Verify before judging anything, so an
// unsatisfiable constitution is refused by name at both entry points
// rather than silently resolving last-writer-wins. adapters must already
// be sorted by id (sortedAdapters), which makes the named pair in the
// error deterministic.
func managedPathOwners(adapters []policyartifact.Adapter) (map[string]string, error) {
	owners := make(map[string]string)
	// Collisions are detected CASE-FOLDED, the same uniform posture
	// discovery matching uses: on a case-insensitive filesystem
	// (APFS/NTFS) AGENTS.md and agents.md are ONE physical file, so a
	// byte-exact check would let two adapters "own" it — Generate would
	// exit 0 while writing a manifest the disk contradicts. Folding
	// refuses the pair on every platform; on a case-sensitive system
	// this refuses a layout that would genuinely be two files, which is
	// the fail-closed direction — a human renames one, and no manifest
	// ever lies (CO-1).
	folded := make(map[string]policyartifact.Adapter)
	spelling := make(map[string]string)
	for _, a := range adapters {
		for _, rel := range a.Managed {
			key := strings.ToLower(rel)
			if prev, ok := folded[key]; ok {
				return nil, fmt.Errorf("%w: %q (adapter %q) and %q (adapter %q) name the same projection file under case folding; one file can carry only one adapter's projection", ErrOverlappingManagedPath, spelling[key], prev.ID, rel, a.ID)
			}
			folded[key] = a
			spelling[key] = rel
			owners[rel] = a.ID
		}
	}
	return owners, nil
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
