// Rung-4 cascade / re-affirmation enforcement, shared by `verdi build
// start` (buildstart.go) and `verdi gate` (gate.go) — 03 §The amendment
// ladder rung 4: "Re-affirm or supersede; the merge gate and verdi build
// start refuse a story whose edges carry unresolved stale flags." This is
// the ENFORCEMENT half of V1-P3's already-computed evidence.FoldCascade;
// this file only resolves the merged-supersession input (a local
// specs/active/ scan — see checkCascadeReaffirmation's doc comment for why)
// and consumes the fold's verdict.
//
// Deliberately distinct from the closure gate's `pending-supersession`
// flag (closuregate.go): that flag reads OPEN (unmerged) supersession MRs
// through the forge port and blocks closure only. This file reads MERGED
// supersessions from the local working tree and blocks build start/gate —
// 03's own text keeps the two mechanisms separate ("Cascade verdicts bind
// at supersession merge ... the race window is visible ... the closure
// gate (not the merge gate) refuses closure while the flag stands").
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/store"
)

// checkCascadeReaffirmation reports whether spec has any unresolved rung-4
// cascade verdict against a feature it implements. ok=true means there is
// nothing to block on: no merged supersession touches spec's edges at all,
// every touched object is carried/amended_advisory (CascadeUnaffected), or
// every amended object touched (CascadeStale) already carries a
// re-affirmation record. ok=false (CascadeStale missing a re-affirmation,
// or CascadeInvalidated — a dangling edge into a removed object, which no
// re-affirmation can resolve) names the blocking reason.
//
// Judgment call, disclosed: the superseding revision is read from the
// LOCAL specs/active/ directory (mirroring storyresolve's own
// directory-scan idiom, matchStoryRef), not fetched from a resolved
// default-branch ref the way gate's condition 1 reads spec status. Both
// call sites of this function (buildstart.go, pre-branch-cut; gate.go,
// build-branch working tree) already operate against a checkout that is
// expected to be up to date with main's merged supersessions — the same
// assumption every other locally-scanned check in this package already
// makes (e.g. computeStubMatch's feature lookup, accept.go). Re-reading via
// git-show against a resolved default branch, as gate condition 1 does,
// would be a defensible alternative; this phase picks the smaller,
// consistent-with-its-siblings option and discloses the choice rather than
// silently picking one.
func checkCascadeReaffirmation(root string, spec *artifact.SpecFrontmatter) (ok bool, reason string, err error) {
	byFeature := evidence.ImplementsByFeature(spec.Links)
	if len(byFeature) == 0 {
		return true, "", nil
	}

	featureNames := make([]string, 0, len(byFeature))
	for name := range byFeature {
		featureNames = append(featureNames, name)
	}
	sort.Strings(featureNames)

	for _, featureName := range featureNames {
		objectIDs := byFeature[featureName]
		superseding, ferr := findSupersedingSpec(root, "spec/"+featureName)
		if ferr != nil {
			return false, "", ferr
		}
		if superseding == nil || superseding.Supersession == nil {
			continue // no merged supersession (yet) — nothing to fold against
		}

		results, ferr := evidence.FoldCascade(*superseding.Supersession, []evidence.CascadeStory{
			{SpecRef: spec.ID, ObjectIDs: objectIDs},
		})
		if ferr != nil {
			return false, "", fmt.Errorf("folding rung-4 cascade for %s against %s: %w", spec.ID, superseding.ID, ferr)
		}
		result := results[0]

		switch result.Verdict {
		case evidence.CascadeUnaffected:
			continue
		case evidence.CascadeInvalidated:
			return false, fmt.Sprintf("%s: edges into %s are invalidated (removed object(s) %v) by superseding spec %s — supersede, re-map, or withdraw (03 §The amendment ladder rung 4)", spec.ID, featureName, result.Removed, superseding.ID), nil
		case evidence.CascadeStale:
			storySlug := store.RefSlug(spec.Story)
			var missing []string
			for _, objID := range result.Amended {
				exists, rerr := reaffirmationExists(root, storySlug, objID)
				if rerr != nil {
					return false, "", rerr
				}
				if !exists {
					missing = append(missing, objID)
				}
			}
			if len(missing) > 0 {
				sort.Strings(missing)
				return false, fmt.Sprintf("%s: stale against superseding spec %s (amended object(s) %v) — missing re-affirmation record(s) at reaffirmations/%s/<object-id>.md (03 §The amendment ladder rung 4)", spec.ID, superseding.ID, missing, storySlug), nil
			}
		}
	}
	return true, "", nil
}

// findSupersedingSpec scans specs/active/ for a spec carrying a
// `supersedes` link to targetRef (e.g. "spec/loan-mgmt"), returning nil
// (not an error) when none exists — the ordinary case for a feature that
// has never been superseded.
func findSupersedingSpec(root, targetRef string) (*artifact.SpecFrontmatter, error) {
	dir := filepath.Join(root, ".verdi", "specs", "active")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		spec, err := loadActiveSpecTolerant(root, e.Name())
		if err != nil || spec == nil {
			continue
		}
		for _, l := range spec.Links {
			if l.Type == artifact.LinkSupersedes && l.Ref == targetRef {
				return spec, nil
			}
		}
	}
	return nil, nil
}

// loadActiveSpecTolerant loads one active spec, returning
// (nil, nil) instead of propagating a decode error — this scan must not
// let one unrelated malformed spec directory (out of scope for this
// check) abort the whole cascade lookup.
func loadActiveSpecTolerant(root, name string) (*artifact.SpecFrontmatter, error) {
	path := filepath.Join(root, ".verdi", "specs", "active", name, "spec.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil //nolint:nilerr // tolerant scan, see doc comment
	}
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return nil, nil //nolint:nilerr
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return nil, nil //nolint:nilerr
	}
	return spec, nil
}

// reaffirmationExists reports whether a re-affirmation record exists for
// (storySlug, objectID) under storeRoot's reaffirmations/ directory
// (01 §Directory layout: reaffirmations/<story-slug>/<object-id>.md).
// Mirrors evidence.AttestationExists's existence-only posture exactly (a
// malformed re-affirmation is still a re-affirmation for this check's
// purposes; lint is where a malformed one gets caught).
func reaffirmationExists(root, storySlug, objectID string) (bool, error) {
	path := filepath.Join(root, ".verdi", "reaffirmations", storySlug, objectID+".md")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("checking reaffirmation %s: %w", path, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("reaffirmation path %s is a directory, not a file", path)
	}
	return true, nil
}
