// Family board links (spec/family-board-links): both directions of
// navigation the store's `implements` edge already encodes, rendered on
// the board — a story board's parent-feature affordance (ac-1), a
// feature board's stub-card links to every matching story anywhere in
// this checkout's store (ac-2, active or archived alike per dc-1's
// ADJ-28 completion reading), the live in-between disclosure when no
// story has landed yet but its design branch already exists (ac-3), and
// a disclosed notice in place of a dead link wherever an implements
// target does not resolve at all (ac-4). Every fact here is a
// store-derived, per-request enrichment attached AFTER buildProjection
// runs (dc-2) — the exact posture boarddiagram.go's
// attachDiagramEditorHrefs already established for refCardView.EditorHref
// — so nothing here is persisted (co-1: no new frontmatter field, no
// sidecar, nothing `verdi accept` could freeze).
package workbench

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/index"
)

// attachFamilyLinks enriches proj with every family-navigation fact this
// story adds, building its own fresh index.Index (dc-2: the same
// per-request posture corpus.go/boardpin.go/boardpeek.go already take in
// this exact package — a fresh walk every render, never cached) over
// root's current working tree.
func attachFamilyLinks(ctx context.Context, proj *BoardProjection, root string) error {
	ix, err := index.Build(root)
	if err != nil {
		return fmt.Errorf("workbench: family links: building index: %w", err)
	}
	attachParentFeatureLink(proj, ix)
	return attachStubStoryLinks(ctx, proj, ix, root)
}

// attachParentFeatureLink enriches every document-level implements-edge
// reference card with a working affordance to its target's own board
// (ac-1: "/board/spec/<feature-name>", not only the corpus page), or,
// when the target does not resolve anywhere in ix, a disclosed inline
// notice naming it in place of the affordance (ac-4, co-3) — never a
// silently inert card, never an href that would 404.
//
// Scoped to Type=="implements" DOCUMENT-level (From=="spec") edges only,
// mirroring AC-1's own text exactly: a decision-level implements edge is
// untouched, and so is any other edge type (co-1: presentation only, no
// widening of this story's scope). A target that parses but is not a
// spec-kind ref (a feature's own implements edge onto an ADR, the v0
// pattern stale-decline's own document edge already uses) is likewise
// left exactly as it rendered before this story — this mechanism is
// about the feature-AC family join alone.
func attachParentFeatureLink(proj *BoardProjection, ix *index.Index) {
	implementsTargets := make(map[string]bool, len(proj.Edges))
	for _, e := range proj.Edges {
		if e.Type == "implements" && e.From == "spec" {
			implementsTargets[e.To] = true
		}
	}
	if len(implementsTargets) == 0 {
		return
	}
	for i := range proj.RefCards {
		rc := &proj.RefCards[i]
		if !implementsTargets[rc.Ref] {
			continue
		}
		ref, err := artifact.ParseRef(rc.Ref)
		if err != nil || ref.Kind != artifact.KindSpec {
			continue
		}
		// The base feature ref, fragment (and any pin) dropped — the
		// board this card links to is the FEATURE's, not one AC's
		// (dc-1: "AC-1's forward direction resolves its own declared
		// target with a plain index lookup", the same existence check
		// dex's resolvableLinkURL already performs before minting a
		// permalink).
		featureRef := "spec/" + ref.Name
		if _, ok := ix.Get(featureRef); ok {
			rc.BoardHref = "/board/" + featureRef
			continue
		}
		rc.UnresolvedNotice = fmt.Sprintf("%s does not resolve in this checkout's store — no board to link to", rc.Ref)
	}
}

// attachStubStoryLinks enriches every declared stub card with AC-2's
// matched-story board link(s) and AC-3's live in-between disclosure
// (dc-1, dc-3, dc-4, ADJ-28's completion reading). Only a feature-class
// spec ever carries stubs (artifact's own validateStory/validateComponent
// both forbid the field), so this is a harmless no-op loop for any other
// wall — no class check needed here.
func attachStubStoryLinks(ctx context.Context, proj *BoardProjection, ix *index.Index, root string) error {
	featureRef := "spec/" + proj.Spec
	for i := range proj.StubViews {
		sv := &proj.StubViews[i]
		for _, storyRef := range matchingStoryRefs(ix, featureRef, sv.AcceptanceCriteria) {
			entry, ok := ix.Get(storyRef)
			if !ok {
				// A backlink target the index never indexed (a dangling
				// ref) is lint's own finding (VL-003), not this card's
				// job to resolve or explain.
				continue
			}
			sv.StoryLinks = append(sv.StoryLinks, stubStoryLinkView{
				Ref:      storyRef,
				Href:     "/board/" + storyRef,
				Archived: isArchivedStorePath(entry.Path),
			})
		}
		if len(sv.StoryLinks) > 0 {
			// ADJ-28: a match anywhere — active or archived — takes AC-2's
			// card. AC-3's live ref-check fires ONLY in the no-match-
			// anywhere case, so it never runs once a match resolves.
			continue
		}
		has, err := gitx.HasLocalBranch(ctx, root, "design/"+sv.Slug)
		if err != nil {
			return fmt.Errorf("workbench: family links: checking design/%s branch: %w", sv.Slug, err)
		}
		if has {
			// Verbatim per parent wl dc-5 / this story's dc-3 — no
			// paraphrase.
			sv.InstantiatedNotice = fmt.Sprintf("instantiated on design/%s, not yet in this checkout's active store", sv.Slug)
		}
		// Absent: today's plain un-instantiated stub card, unchanged.
	}
	return nil
}

// matchingStoryRefs returns the sorted, deduplicated refs of every story
// whose implements edge names one of featureRef's acIDs — the SAME
// backlink inversion internal/dex/featurelens.go's implementingStoryRefs
// and cmd/verdi/featurematrix.go's discoverImplementingStories already
// use (dc-1: ix.Backlinks(featureRef+"#"+acID) filtered to
// Type=="implemented-by"), generalized over a SET of AC ids since a
// stub may jointly (not necessarily individually) declare more than one
// (dc-4: any overlap at all is a match, rendered plainly, never merged
// or ranked). ix.Backlinks already sorts by (type, from) per AC id, but
// merging results across more than one AC id can interleave two already-
// sorted runs, so the combined slice is explicitly sorted here.
func matchingStoryRefs(ix *index.Index, featureRef string, acIDs []string) []string {
	seen := make(map[string]bool)
	var refs []string
	for _, acID := range acIDs {
		for _, bl := range ix.Backlinks(featureRef + "#" + acID) {
			if bl.Type != "implemented-by" {
				continue
			}
			if !seen[bl.From] {
				seen[bl.From] = true
				refs = append(refs, bl.From)
			}
		}
	}
	sort.Strings(refs)
	return refs
}

// isArchivedStorePath reports whether an indexed entry's backing file
// lives under specs/archive/ — the directory truth, not a guess from
// status (mirroring internal/dex/kindaxis.go's isArchivedSpec and
// internal/dex/permalink.go's pageBreadcrumb comment: "the directory
// truth ... never guess, drift toward honest"). A story's Status turns
// "closed" only once internal/store.ArchiveMove has moved its directory,
// so the two signals coincide in practice; this checks the one signal
// the rest of the codebase already treats as authoritative. path may use
// either separator convention; ToSlash normalizes before the check.
func isArchivedStorePath(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/specs/archive/")
}
