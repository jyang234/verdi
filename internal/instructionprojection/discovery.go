package instructionprojection

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/policyartifact"
)

// skipDirBasenames are directory names the discovery walk never
// descends into, matched at any depth (the same noise class
// internal/store's own DiscoverServices skips: ".git, .verdi/data,
// node_modules-ish noise"). testdata is DELIBERATELY NOT included here,
// unlike internal/store's own skip set (which also skips testdata and
// examples for its own, different reason): an instruction file placed
// under a testdata tree is still a file a real harness's discovery
// chain would find at that exact path (AC-1's "effective project-level
// instruction discovery chain, including nested instruction files"), so
// treating it as noise would let a genuine shadowing or unmanaged
// instruction file hide from this witness merely by living under a
// fixture directory — the opposite of this package's fail-closed
// purpose.
var skipDirBasenames = map[string]bool{
	".git":         true,
	"node_modules": true,
}

// discover walks root looking for every regular file or symlink whose
// basename is one of some adapter's declared discovery filenames,
// producing one Finding per (adapter, path) pair the adapter's own
// managed set does not account for. managed maps each adapter id to its
// own set of declared managed paths.
//
// It never follows a symlinked directory (filepath.WalkDir/ReadDir
// already does not: a symlink's own DirEntry never reports IsDir true)
// and never reads a symlinked file's target. A symlink matching a
// discovery filename is ALWAYS classified unmanaged/shadowing — even
// when its own path happens to equal a declared managed path — because
// its target is outside proof (contract) and a symlink can therefore
// never be proven to BE the adapter's generated content; the managed-
// file integrity check (verify.go) independently reaches the same
// fail-closed disposition for a managed path that turns out to be a
// symlink, so this rule is never the only place that catches it.
//
// Any walk error (most commonly an unreadable directory) is recorded as
// its own incomplete-discovery Finding rather than aborting the walk:
// CO-1's "silence is never a pass" applies to the walk's own
// completeness, not only to the files it manages to see. A directory
// that could not be read is skipped (its contents are simply unknown,
// not assumed absent) so the rest of the tree is still checked.
func discover(root string, adapters []policyartifact.Adapter, managed map[string]map[string]bool) ([]Finding, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("instructionprojection: resolving root: %w", err)
	}
	verdiData := filepath.Join(absRoot, ".verdi", "data")

	byFilename := map[string][]string{}
	for _, a := range adapters {
		for _, f := range a.DiscoveryFilenames {
			byFilename[f] = append(byFilename[f], a.ID)
		}
	}

	var findings []Finding
	walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		rel := path
		if r, rerr := filepath.Rel(absRoot, path); rerr == nil {
			rel = filepath.ToSlash(r)
		}
		if err != nil {
			findings = append(findings, Finding{Code: ReasonIncompleteDiscovery, Path: rel, Detail: err.Error()})
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if path == absRoot {
			return nil
		}
		if d.IsDir() {
			if skipDirBasenames[d.Name()] || path == verdiData {
				return fs.SkipDir
			}
			return nil
		}

		owners, ok := byFilename[d.Name()]
		if !ok {
			return nil
		}
		isSymlink := d.Type()&fs.ModeSymlink != 0
		for _, adapterID := range owners {
			if !isSymlink && managed[adapterID][rel] {
				continue // the adapter's own generated file; checked by verify.go's own integrity pass.
			}
			code := ReasonUnmanaged
			if strings.Contains(rel, "/") {
				code = ReasonShadowing
			}
			findings = append(findings, Finding{Adapter: adapterID, Code: code, Path: rel})
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("instructionprojection: walking %s: %w", root, walkErr)
	}
	return findings, nil
}
