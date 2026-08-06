package instructionprojection

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/policyartifact"
)

// skipDirBasenames are directory names the discovery walk never
// descends into, matched at any depth.
//
//   - .git is the repository's own metadata: nothing there is a project
//     instruction file a harness reads.
//   - node_modules is a vendored dependency tree; its contents are
//     another project's files, not this project's instruction chain.
//
// testdata is DELIBERATELY NOT included here, unlike internal/store's own
// skip set (which also skips testdata and examples for its own, different
// reason): an instruction file placed under a testdata tree is still a
// file a real harness's discovery chain would find at that exact path
// (AC-1's "effective project-level instruction discovery chain, including
// nested instruction files"), so treating it as noise would let a genuine
// shadowing or unmanaged instruction file hide from this witness merely
// by living under a fixture directory — the opposite of this package's
// fail-closed purpose.
var skipDirBasenames = map[string]bool{
	".git":         true,
	"node_modules": true,
}

// skipRootRelDirs are root-relative subtrees the walk never descends
// into. .verdi/data is Verdi's OWN working data — machine-written,
// never committed (the repo's own rule), and never part of any harness's
// project instruction chain. It is excluded because it is Verdi's own
// scratch space, NOT because of DC-2's vendor-opaque harness boundary,
// which concerns prompt material Verdi cannot see at all and has nothing
// to do with this directory.
var skipRootRelDirs = []string{".verdi/data"}

// excludedSubtrees returns the walk's exclusion disclosure: every
// subtree discovery never examines, sorted, derived from the same rules
// the walk itself applies so the disclosure can never drift from the
// behavior. Every Report carries it, clean or not — an unexamined
// subtree that no report mentions is exactly the silence CO-1 forbids.
// Basename rules (.git, node_modules) apply at any depth; root-relative
// rules (.verdi/data) apply once, at that path.
func excludedSubtrees() []string {
	out := make([]string, 0, len(skipDirBasenames)+len(skipRootRelDirs))
	for name := range skipDirBasenames {
		out = append(out, name)
	}
	out = append(out, skipRootRelDirs...)
	sort.Strings(out)
	return out
}

// discover walks root looking for every regular file or symlink whose
// basename matches some adapter's declared discovery filename, producing
// one Finding per (adapter, path) pair no adapter's managed set
// accounts for. managed is the UNION of every adapter's managed paths.
//
// Satisfaction is cross-adapter by contract: AC-1 requires every
// discovered project instruction to be "generated and digest-matched",
// not managed by whichever adapter happens to discover it. In the
// realistic layout — codex manages AGENTS.md, claude-code manages
// CLAUDE.md but its harness also reads AGENTS.md — the AGENTS.md
// claude-code discovers IS generated and digest-matched, by codex's own
// managed-file check (verify.go), so it satisfies claude-code's
// discovery too. Nothing is trusted without proof: a path only counts as
// satisfied because some adapter's integrity pass independently verified
// its bytes this same run.
//
// Filename matching is CASE-FOLDED on every platform (strings.EqualFold
// against each declared discovery filename). On a case-insensitive
// filesystem (APFS, NTFS) a planted "agents.md" IS the file a harness
// asking for "AGENTS.md" opens, so byte-exact matching would let a real,
// harness-read instruction file produce no finding at all — a silent
// false clean. The rule is uniform rather than filesystem-detected so
// the same tree yields the same verdict everywhere (CO-3). The trade is
// deliberate and one-directional: on a CASE-SENSITIVE filesystem a
// "agents.md" the harness would never read is still reported as
// unmanaged/shadowing. That is a finding a human resolves by renaming or
// declaring the file — never a silent pass, which is the only failure
// direction this package refuses.
//
// Satisfaction, unlike matching, stays EXACT-CASE: only a byte-exact
// declared managed path counts, because that is the only path the
// managed-file check verified. On a case-insensitive filesystem this has
// a second disclosed direction: a PRE-EXISTING case-variant of a managed
// path (say agents.md where the adapter declares AGENTS.md) is the file
// Generate's write lands in — the OS preserves the incumbent spelling —
// so Verify reports it unmanaged even though its bytes are the generated
// projection. Fail-closed, a finding a human resolves by renaming to the
// declared spelling; never a silent pass, and never a manifest that lies
// (managedPathOwners refuses declared case-collisions outright).
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
func discover(root string, adapters []policyartifact.Adapter, managed map[string]bool) ([]Finding, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("instructionprojection: resolving root: %w", err)
	}
	skipFullPaths := make(map[string]bool, len(skipRootRelDirs))
	for _, rel := range skipRootRelDirs {
		skipFullPaths[filepath.Join(absRoot, filepath.FromSlash(rel))] = true
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
			if skipDirBasenames[d.Name()] || skipFullPaths[path] {
				return fs.SkipDir
			}
			return nil
		}

		isSymlink := d.Type()&fs.ModeSymlink != 0
		for _, a := range adapters {
			if !discoversFilename(a, d.Name()) {
				continue
			}
			if !isSymlink && managed[rel] {
				continue // generated by some adapter; its bytes are checked by verify.go's own integrity pass.
			}
			code := ReasonUnmanaged
			if strings.Contains(rel, "/") {
				code = ReasonShadowing
			}
			findings = append(findings, Finding{Adapter: a.ID, Code: code, Path: rel})
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("instructionprojection: walking %s: %w", root, walkErr)
	}
	return findings, nil
}

// discoversFilename reports whether basename matches any of a's declared
// discovery filenames under the case-folded rule documented on discover.
// It reports at most one match per adapter, so an adapter that declares
// two case variants of one name still yields a single finding.
func discoversFilename(a policyartifact.Adapter, basename string) bool {
	for _, f := range a.DiscoveryFilenames {
		if strings.EqualFold(basename, f) {
			return true
		}
	}
	return false
}
