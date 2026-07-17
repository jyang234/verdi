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
// root's current working tree. fixedBranch mirrors
// attachDiagramEditorHrefs' own parameter exactly (ADJ-70, closing
// judged-fbl-r4-5): "" for the serving checkout's unprefixed board, the
// design branch for a per-branch board — so every family href stays
// inside the store it was resolved from, never a root-relative address
// that ejects the operator to the serving checkout or 404s on a
// branch-only target.
func attachFamilyLinks(ctx context.Context, proj *BoardProjection, root, fixedBranch string) error {
	ix, err := index.Build(root)
	if err != nil {
		return fmt.Errorf("workbench: family links: building index: %w", err)
	}
	attachParentFeatureLink(proj, ix, fixedBranch)
	return attachStubStoryLinks(ctx, proj, ix, root, fixedBranch)
}

// attachParentFeatureLink enriches every document-level implements-edge
// reference card with a working affordance to its target feature's own
// SERVABLE surface (ac-1), or, when the target does not resolve anywhere
// in ix, a disclosed inline notice naming it in place of the affordance
// (ac-4, co-3) — never a silently inert card, never an href that would
// 404. An ACTIVE feature links to its board ("/board/spec/<feature>");
// an ARCHIVED feature — which the board route 404s on (servableSurface's
// doc comment) — links to its corpus page with its archived state
// disclosed, per ADJ-39's constraint-over-mandate ruling (dc-1's
// zone-agnostic target resolution "from either zone" met the co-3/ac-4
// no-404 constraint; the constraint governs).
//
// Scoped to Type=="implements" DOCUMENT-level (From=="spec") edges only,
// mirroring AC-1's own text exactly: a decision-level implements edge is
// untouched, and so is any other edge type (co-1: presentation only, no
// widening of this story's scope). A target that parses but is not a
// spec-kind ref (a feature's own implements edge onto an ADR, the v0
// pattern stale-decline's own document edge already uses) is likewise
// left exactly as it rendered before this story — this mechanism is
// about the feature-AC family join alone.
func attachParentFeatureLink(proj *BoardProjection, ix *index.Index, fixedBranch string) {
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
		// The board this card links to is the FEATURE's, not one AC's, so
		// the surface resolves off the base feature ref with its fragment
		// dropped (dc-1: "AC-1's forward direction resolves its own declared
		// target with a plain index lookup"). But a fragment-bearing edge
		// (spec/<feature>#<ac>) still requires the NAMED AC to resolve too:
		// a since-renamed or -removed AC leaves the feature resolving while
		// the family join's AC-level half dangles, so it takes ac-4's
		// disclosed notice rather than a live affordance vouching for a join
		// that no longer holds (fbl-r3-6, ADJ-64). ObjectIDs is the same
		// artifact.DeclaredObjectIDs set lint's VL-003 resolves fragments
		// against; a whole-feature edge (no fragment) resolves on the
		// feature alone, exactly as before.
		featureRef := "spec/" + ref.Name
		if entry, ok := ix.Get(featureRef); ok && (!ref.Fragment() || entry.ObjectIDs[ref.Object]) {
			href, archived := servableSurface(featureRef, entry, fixedBranch)
			if href == "" {
				// ADJ-70: a branch-resolved ARCHIVED target has no surface
				// that provably serves it (servableSurface's doc comment), so
				// the card takes ac-4's disclosed-notice posture — named
				// state, no href — rather than a root-relative link that
				// ejects the operator or 404s.
				rc.UnresolvedNotice = fmt.Sprintf("%s is archived in this branch's store — no per-branch surface serves the archive", rc.Ref)
				continue
			}
			rc.FeatureHref, rc.Archived = href, archived
			continue
		}
		rc.UnresolvedNotice = fmt.Sprintf("%s does not resolve in this checkout's store — no board to link to", rc.Ref)
	}
}

// servableSurface resolves the one workbench surface that serves a
// RESOLVED family target, honoring ADJ-39's (2026-07-16) constraint-over-
// mandate ruling for both navigation directions this story renders. It
// returns the servable href and whether the target lives in the archive
// zone (so the card can disclose that state).
//
// The board route (/board/spec/<name>) serves the ACTIVE zone ONLY —
// boardSpecServer.specDir (boardspec.go) reads .verdi/specs/active/<name>
// alone, so an archived spec 404s there (directory.go's boardServable
// posture names this exactly). The corpus page (/a/spec/<name>) is
// zone-agnostic: index.Build's walk indexes specs/archive/ alike
// (artifact.ClassifyPath), and corpusHandler serves any indexed
// non-external entry — so it is the surface that serves an archived spec.
// An active match therefore keeps parent ac-2's plain board link; an
// archived match links to the servable corpus page instead of the dead
// board href co-3/ac-4 forbid.
//
// This never assumes servability: it keys off the resolved entry's own
// directory zone (isArchivedStorePath, the "directory truth" dc-1/dc-3
// already treat as authoritative — the same signal boardServable
// computes). On the SERVING checkout (fixedBranch == "") a resolved spec
// entry always HAS a servable surface (the corpus page serves every
// indexed spec — TestArchivedSpec_ServableSurfaces proves the archive
// case, and TestCorpusHandler_ServesArchivedSpec guards it), so no
// resolved match is ever linkless there.
//
// On a PER-BRANCH board (fixedBranch != "", ADJ-70 — the branch-prefixed-
// addressing sweep judged-fbl-r4-5 was filed for): an ACTIVE match is
// servable BY CONSTRUCTION at its branch's own board — the index this
// resolver's callers built walks the branch worktree, and /b/{branch}
// serves that same worktree — so the href is the shared branchBoardHref
// address, never the root-relative one that ejected the operator to the
// serving checkout and 404ed on a branch-only target. An ARCHIVED match
// has NO surface that provably serves it (the /a/ corpus is root-only,
// handler.go mounts no /b/ corpus, and it reads the SERVING checkout's
// tree — a branch-only archived spec 404s there too), so this returns no
// href at all and the caller renders its disclosed no-link state: ADJ-39's
// disclosure-only fallback, previously unreachable, is exactly this
// branch-archived case.
func servableSurface(ref string, entry *index.Entry, fixedBranch string) (href string, archived bool) {
	if isArchivedStorePath(entry.Path) {
		if fixedBranch != "" {
			return "", true
		}
		return "/a/" + ref, true
	}
	if fixedBranch != "" {
		return branchBoardHref(fixedBranch, strings.TrimPrefix(ref, "spec/")), false
	}
	return "/board/" + ref, false
}

// attachStubStoryLinks enriches every declared stub card with AC-2's
// matched-story board link(s) and AC-3's live in-between disclosure
// (dc-1, dc-3, dc-4, ADJ-28's completion reading). Only a feature-class
// spec ever carries stubs (artifact's own validateStory/validateComponent
// both forbid the field), so this is a harmless no-op loop for any other
// wall — no class check needed here.
func attachStubStoryLinks(ctx context.Context, proj *BoardProjection, ix *index.Index, root, fixedBranch string) error {
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
			// An active match links to its board (parent ac-2 verbatim; on a
			// per-branch board, that branch's own board — ADJ-70); an
			// archived match links to its SERVABLE corpus page rather than
			// the board route that 404s on the archive zone, its archived
			// state disclosed (ADJ-39, servableSurface's doc comment) — and
			// on a per-branch board, where no surface provably serves the
			// archive, it takes the disclosed no-href card instead (ADJ-70:
			// never an href that can 404, never a silent omission).
			href, archived := servableSurface(storyRef, entry, fixedBranch)
			link := stubStoryLinkView{
				Ref:      storyRef,
				Href:     href,
				Archived: archived,
			}
			if href == "" {
				link.UnservableNotice = "archived in this branch's store — no per-branch surface serves the archive"
			}
			sv.StoryLinks = append(sv.StoryLinks, link)
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
