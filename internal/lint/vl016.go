package lint

import (
	"fmt"
	"path"
	"strings"

	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/store"
)

// vl016 enforces "spike path fence: a build branch built from a spike:
// true story touches only paths matched by verdi.yaml's spike_paths:
// allowlist; any other path in the diff fails closed" (02 §Lint rules;
// 01 §Store manifest). Like VL-010, it diffs Context.DiffBase..HEAD
// (I-14) and is silent — not a false pass, but nothing it can prove —
// when DiffBase is unknown.
//
// Judgment call (recorded here and in the phase report): 02 gives no
// explicit "which branch belongs to which story" plumbing anywhere in this
// codebase (the same gap VL-004/VL-010 already accept for their own
// branch-scoped checks, per I-14's "otherwise" posture). This rule treats
// the diff as a spike build-branch diff whenever it touches at least one
// path under a spike story's own spec directory (a signal that the branch
// is evidently building/exploring that spike) — every path in a diff that
// touches a spike's own directory must then stay inside spike_paths: or be
// inside that same spike directory. A diff that never touches any spike's
// own directory is not fenced, even if some other spike exists elsewhere
// in the store: the fence is about a *branch's* diff, not the store's
// static inventory. The smallest reversible reading given no other
// branch-to-story binding exists in v1.
type vl016 struct{}

func (vl016) ID() string { return "VL-016" }

func (vl016) Check(in *RunInput) []Finding {
	if in.LintCtx.DiffBase == "" {
		return nil
	}

	spikeDirs := spikeStoryDirs(in.Snapshot.Docs)
	if len(spikeDirs) == 0 {
		return nil
	}

	entries, err := gitx.DiffNameStatus(in.Ctx, in.Root, in.LintCtx.DiffBase, "HEAD")
	if err != nil {
		return []Finding{{Rule: "VL-016", Path: "", Message: fmt.Sprintf("computing diff %s..HEAD: %v", in.LintCtx.DiffBase, err)}}
	}

	activated := false
	for _, e := range entries {
		if withinAnyDir(spikeDirs, e.Path) || (e.OldPath != "" && withinAnyDir(spikeDirs, e.OldPath)) {
			activated = true
			break
		}
	}
	// D-5: on the canonical spike BUILD branch, the spike's own spec directory
	// is committed and frozen during the design phase — before build start
	// even cuts the build branch — so the branch's own diff (DiffBase..HEAD)
	// never touches it, and the touchesSpike signal above can never fire. Also
	// activate when the current branch is that spike's build branch
	// (feature/<spike-name>), keying off the originating spec the way build
	// start's own story resolution does, rather than re-deriving the branch's
	// identity from the diff's contents — the one signal the frozen-directory
	// workflow makes absent from the diff.
	if !activated && isSpikeBuildBranch(in.Snapshot.Docs, in.LintCtx.CurrentBranch) {
		activated = true
	}
	if !activated {
		return nil
	}

	allowlist := spikePathsOf(in.Snapshot.Manifest)

	var findings []Finding
	seen := map[string]bool{}
	for _, e := range entries {
		for _, p := range diffPaths(e) {
			if p == "" || seen[p] {
				continue
			}
			if withinAnyDir(spikeDirs, p) {
				continue
			}
			if matchesAnySpikePath(allowlist, p) {
				continue
			}
			seen[p] = true
			findings = append(findings, Finding{Rule: "VL-016", Path: p, Message: fmt.Sprintf("path %q is outside verdi.yaml's spike_paths: allowlist %v — a spike build branch's diff must stay inside the fence (02 §Lint rules, 01 §Store manifest)", p, allowlist)})
		}
	}
	return findings
}

// diffPaths returns every path a DiffEntry touches: just Path for
// add/modify/delete, both Path and OldPath for a rename.
func diffPaths(e gitx.DiffEntry) []string {
	if e.Status == "R" {
		return []string{e.OldPath, e.Path}
	}
	return []string{e.Path}
}

// spikeStoryDirs returns the spec directory (RelPath, no trailing slash)
// of every decoded spike: true story in docs.
func spikeStoryDirs(docs []*Document) []string {
	var dirs []string
	for _, d := range docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Spec == nil || !d.Spec.Spike {
			continue
		}
		dirs = append(dirs, specDirOf(d))
	}
	return dirs
}

// isSpikeBuildBranch reports whether branch is the build branch of a spike
// story spec — feature/<name>, where <name> is a spike spec's own directory
// name (buildstart.go names the build branch feature/<specRef.Name>, and a
// spec's directory is specs/active/<specRef.Name>/). This is the "which
// branch is a spike branch" signal that survives the spike spec directory
// being frozen before the build branch exists (D-5), unlike the diff-content
// heuristic.
func isSpikeBuildBranch(docs []*Document, branch string) bool {
	const prefix = "feature/"
	if !strings.HasPrefix(branch, prefix) {
		return false
	}
	name := strings.TrimPrefix(branch, prefix)
	for _, d := range docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Spec == nil || !d.Spec.Spike {
			continue
		}
		if path.Base(specDirOf(d)) == name {
			return true
		}
	}
	return false
}

// withinAnyDir reports whether p is dir itself or lies under it, for any
// dir in dirs.
func withinAnyDir(dirs []string, p string) bool {
	for _, dir := range dirs {
		if p == dir || strings.HasPrefix(p, dir+"/") {
			return true
		}
	}
	return false
}

// spikePathsOf returns m.SpikePaths, or nil for a nil manifest — a pointer
// receiver so a store without a decoded manifest (VL-016 still runs) never
// panics.
func spikePathsOf(m *store.Manifest) []string {
	if m == nil {
		return nil
	}
	return m.SpikePaths
}

// matchesAnySpikePath reports whether p matches any of patterns.
func matchesAnySpikePath(patterns []string, p string) bool {
	for _, pat := range patterns {
		if matchesSpikePath(pat, p) {
			return true
		}
	}
	return false
}

// matchesSpikePath reports whether p matches pattern — either an ordinary
// path.Match glob (single-segment "*" wildcards), or, for a pattern ending
// in "/**", a recursive directory-prefix match: go's path.Match does not
// support "**" as cross-segment recursion (each "*" stops at a "/"), so
// this rule implements the recursive convenience itself, matching every
// path under (not just directly inside) the named directory.
func matchesSpikePath(pattern, p string) bool {
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return p == prefix || strings.HasPrefix(p, prefix+"/")
	}
	ok, err := path.Match(pattern, p)
	return err == nil && ok
}
