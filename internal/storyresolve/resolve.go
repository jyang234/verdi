// Package storyresolve resolves a user- or tool-supplied story/spec
// argument to a decoded active feature spec, per I-30's strict two-form
// contract. Extracted from cmd/verdi/matrix.go (phase 6) so phase 9's
// get_matrix MCP tool (internal/mcpserve) shares exactly the same
// resolution policy instead of re-implementing it (CLAUDE.md: "anything
// used by two or more packages lives in a shared internal/ package").
package storyresolve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OWNER/verdi/internal/artifact"
)

// Resolve resolves arg to a foldable spec under specs/active/ — 03 §The
// fold's "Scope: the fold is evaluated only for specs under
// specs/active/". Per I-30, arg is EXACTLY one of two forms: a spec ref
// ("spec/<name>"), loaded directly; or a scheme-prefixed story ref
// ("jira:LOAN-1482"), matched against every active feature spec's
// `story:` field. Any other argument is an operational error naming both
// accepted forms.
//
// A resolved spec-ref may be either a `class: feature` spec (folded at the
// feature level by the caller) or a story-grade spec — a round-four
// `class: story` spec, or a grandfathered v0 `class: feature` spec, both
// folded at the story level. Only a `class: component` spec (no story, no
// acceptance criteria) is rejected: there is nothing to fold. Routing
// between the feature and story folds is the caller's job, keyed on the
// resolved spec's Class (cmd/verdi/matrix.go).
func Resolve(root, arg string) (*artifact.SpecFrontmatter, error) {
	// (b) A spec ref: load it directly.
	if ref, err := artifact.ParseRef(arg); err == nil && ref.Kind == artifact.KindSpec {
		spec, loadErr := LoadActiveSpec(root, ref.Name)
		if loadErr != nil {
			return nil, loadErr
		}
		if spec.Class == artifact.ClassComponent {
			return nil, fmt.Errorf("spec %q is a component spec (no story, no acceptance criteria); matrix folds only feature and story specs", arg)
		}
		return spec, nil
	}

	// (a) A scheme-prefixed story ref: match it against every active
	// feature spec's story: field — UNCHANGED from pre-round-four (see
	// matchStoryRef's own doc comment for why this function deliberately
	// stays feature-class-only rather than also matching class: story
	// specs, even though round four gives stories their own story: field
	// too). The scheme (the part before ":") need not be a configured
	// provider — an unmatched story ref simply names no spec.
	if scheme, key, ok := strings.Cut(arg, ":"); ok && scheme != "" && key != "" {
		return matchStoryRef(root, arg)
	}

	return nil, fmt.Errorf("%q is neither a scheme-prefixed story ref (e.g. jira:LOAN-1482) nor a spec ref (e.g. spec/stale-decline); this verb accepts exactly those two forms", arg)
}

// matchStoryRef returns the single active class: feature spec whose
// story: field equals storyRef, erroring if none — or more than one —
// does. Deliberately UNCHANGED from pre-round-four (V1-P4 tried, then
// reverted, widening this to also match class: story specs — see the
// phase report's disclosed judgment call): this function is Resolve's
// shared seam, consumed by every ref/story-argument-taking verb
// (matrix, rollup, the verdict viewer, MCP tools, and more), all of which
// already depend on its feature-only resolution semantics against the
// real corpus (testdata/corpus/'s stale-decline, class: feature, and
// borrower-update-api, class: story, both legitimately carry
// story: jira:LOAN-1482 — a feature's OPTIONAL epic/objective story: field
// and a story's REQUIRED own story: field are different tracker refs that
// can coincide with no reserved-uniqueness rule against each other, so
// widening this shared function silently changes which spec several
// unrelated, already-shipped call sites resolve to). `verdi build start`'s
// own need to resolve a bare story ref against a class: story spec is
// solved locally, in cmd/verdi/buildstart.go's resolveBuildTarget, which
// layers on top of this function rather than changing its behavior.
func matchStoryRef(root, storyRef string) (*artifact.SpecFrontmatter, error) {
	dir := filepath.Join(root, ".verdi", "specs", "active")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}

	var matches []*artifact.SpecFrontmatter
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		spec, err := LoadActiveSpec(root, e.Name())
		if err != nil {
			return nil, err
		}
		if spec.Class != artifact.ClassFeature {
			continue
		}
		if spec.Story == storyRef {
			matches = append(matches, spec)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no active feature spec has story: %s", storyRef)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.ID
		}
		return nil, fmt.Errorf("story ref %q matches more than one active feature spec: %s", storyRef, strings.Join(names, ", "))
	}
}

// buildBranchPrefix is `verdi feature start`'s branch-naming convention
// (cmd/verdi/feature.go: `branch := "feature/" + specRef.Name`) — the
// convention ResolveBuildSpec inverts to infer, with no argument, which
// spec a build branch belongs to.
const buildBranchPrefix = "feature/"

// ResolveBuildSpec infers the build-head spec a build branch is for, given
// only the currently checked-out branch's short name — the resolution
// `verdi align` and `verdi gate` use (PLAN.md Phase 8, 05 §CLI: neither
// verb takes a story/spec argument; both "generate ... for the build head"
// / gate it). branch must have the "feature/<name>" shape `verdi build
// start` (and its deprecation alias, `feature start`) cuts; anything else
// (a detached HEAD, main, a design/ branch, ...) is an operational error
// naming the expected convention rather than silently guessing which spec
// is in scope. V1-P4: the resolved spec may be a round-four class: story
// spec (the actual buildable unit, 03 §Lifecycle: the feature-first
// cascade) or a grandfathered v0 class: feature spec (A8, pre-round-four's
// single-level model) — build start cuts the SAME "feature/<name>" branch
// shape for both, so this inference must accept both classes exactly as
// storyresolve.Resolve's own doc comment already does; only a class:
// component spec (no story, no acceptance criteria — nothing to gate or
// align) is rejected.
func ResolveBuildSpec(root, branch string) (*artifact.SpecFrontmatter, error) {
	name, ok := strings.CutPrefix(branch, buildBranchPrefix)
	if !ok || name == "" {
		return nil, fmt.Errorf("storyresolve: current branch %q is not a build branch (want feature/<name>, cut by `verdi build start`)", branch)
	}
	spec, err := LoadActiveSpec(root, name)
	if err != nil {
		return nil, err
	}
	if spec.Class == artifact.ClassComponent {
		return nil, fmt.Errorf("storyresolve: build branch %q resolves to %s, a component spec (no story, no acceptance criteria)", branch, spec.ID)
	}
	return spec, nil
}

// designBranchPrefix is `verdi design start`'s branch-naming convention
// (cmd/verdi/design.go: `branch := "design/" + name`) — the convention
// ResolveDesignSpec inverts, mirroring ResolveBuildSpec's own pattern for
// build branches.
const designBranchPrefix = "design/"

// ResolveDesignSpec infers the spec a design branch is for, given only the
// currently checked-out branch's short name — the resolution `verdi
// align`'s design-branch mode (03 §Decision-conflict gate) uses. branch
// must have the "design/<name>" shape `design start` cuts. Unlike
// ResolveBuildSpec (feature class only), a design branch legally carries
// either class `verdi design start --kind` scaffolds (05 §CLI: "--kind
// selects the two-scope spec class"): feature or story. Only a component
// spec (no decisions block usage envisioned by 03's three-tier model, and
// no story to build) is rejected.
func ResolveDesignSpec(root, branch string) (*artifact.SpecFrontmatter, error) {
	name, ok := strings.CutPrefix(branch, designBranchPrefix)
	if !ok || name == "" {
		return nil, fmt.Errorf("storyresolve: current branch %q is not a design branch (want design/<name>, cut by `verdi design start`)", branch)
	}
	spec, err := LoadActiveSpec(root, name)
	if err != nil {
		return nil, err
	}
	if spec.Class != artifact.ClassFeature && spec.Class != artifact.ClassStory {
		return nil, fmt.Errorf("storyresolve: design branch %q resolves to %s, a component spec (03 §Decision-conflict gate applies to feature/story specs only)", branch, spec.ID)
	}
	return spec, nil
}

// LoadActiveSpec reads and strict-decodes specs/active/<name>/spec.md.
func LoadActiveSpec(root, name string) (*artifact.SpecFrontmatter, error) {
	path := filepath.Join(root, ".verdi", "specs", "active", name, "spec.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return spec, nil
}

// LoadSpec reads and strict-decodes <name>/spec.md from either
// specs/active/ or specs/archive/ (active preferred), returning
// (nil, nil) when neither exists. A supersedes target may legitimately
// live in archive — an accepted/closed predecessor spec remains a valid
// rung-3 chain-edge target — so callers resolving such edges must consult
// both zones, mirroring internal/align's own readSpecByName.
func LoadSpec(root, name string) (*artifact.SpecFrontmatter, error) {
	for _, statusDir := range []string{"active", "archive"} {
		path := filepath.Join(root, ".verdi", "specs", statusDir, name, "spec.md")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		spec, err := artifact.DecodeSpec(fm)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return spec, nil
	}
	return nil, nil
}
