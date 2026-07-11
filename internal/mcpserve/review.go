// Review-sticky mirrored population (V1-P7; 05 §MCP server's
// list_annotations row: "covers the R4 annotation types — open questions,
// scratch stickies, untyped relates-threads, and (mirrored) review
// stickies"). list_annotations is read-only (05's tool table: "R"), so
// this file only ever reads the forge's live comment feed — it never
// posts, resolves, or mutates anything (the board, V1-P6, owns the write
// side of the round-trip; add_annotation stays the only MCP write tool,
// 05: "the write surface stays add_annotation and nothing else").
package mcpserve

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/forge"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/lint"
)

// reviewMirroredAnnotations returns unpinned's live forge review comments
// whose [vd:<object-id>] token resolves against unpinned's OWN declared
// objects (02 §Record schemas' comment-token grammar; 05 §Review stickies
// and forge round-trip), mirrored into annotationItem shape (type:
// "review"). Returns (nil, nil) — never an error — for every case where
// there is honestly nothing to mirror rather than a failure:
//
//   - b.Forge is nil (no forge configured/reachable for this checkout);
//   - the current checkout is not on a design branch at all (05 §Review
//     stickies and forge round-trip: "accepted specs on main" have no
//     open MR to host review stickies against — "there is no open MR to
//     host them");
//   - the default branch cannot be resolved;
//   - no open MR is found whose source branch is the current design
//     branch (nothing pushed/opened yet);
//   - unpinned does not resolve to a spec at all (only spec objects are
//     ever [vd:<object-id>] token targets, §Object model).
//
// A genuine forge transport failure (the forge WAS reachable enough to
// attempt the call) is still returned as an error — this function does
// not swallow real operational failures, only "nothing to mirror" states.
func (b *Backend) reviewMirroredAnnotations(ctx context.Context, unpinned artifact.Ref) ([]annotationItem, error) {
	if b.Forge == nil {
		return nil, nil
	}

	branch, err := gitx.CurrentBranch(ctx, b.Root)
	if err != nil || !strings.HasPrefix(branch, "design/") {
		return nil, nil
	}

	spec, ok := b.readSpecFrontmatter(unpinned)
	if !ok {
		return nil, nil
	}
	declared := artifact.DeclaredObjectIDs(spec)
	if len(declared) == 0 {
		return nil, nil
	}

	defaultBranch := lint.ResolveDefaultBranch(ctx, b.Root)
	if defaultBranch == "" {
		return nil, nil
	}
	mrs, err := b.Forge.ListOpenMRs(ctx, defaultBranch)
	if err != nil {
		return nil, fmt.Errorf("mcpserve: listing open MRs for review population: %w", err)
	}
	var mrID string
	for _, mr := range mrs {
		if mr.SourceBranch == branch {
			mrID = mr.ID
			break
		}
	}
	if mrID == "" {
		return nil, nil
	}

	comments, err := b.Forge.ListComments(ctx, mrID)
	if err != nil {
		return nil, fmt.Errorf("mcpserve: listing review comments for MR %s: %w", mrID, err)
	}

	// Resolution state is best-effort: a query failure here still leaves
	// every mirrored item correctly populated with Status "open" (the
	// safe, disclosed-conservative default — never silently reporting
	// "resolved" on incomplete information), so it is not itself an error
	// this function propagates.
	resolvedThreads := map[string]bool{}
	if threads, terr := b.Forge.GetThreadResolution(ctx, mrID); terr == nil {
		for _, tr := range threads {
			resolvedThreads[tr.ThreadID] = tr.Resolved
		}
	}

	var items []annotationItem
	for _, c := range comments {
		objID, ok := forge.ParseCommentToken(c.Body)
		if !ok || !declared[objID] {
			// No resolvable token, or the token names an object on a
			// DIFFERENT spec/target than the one asked for — never this
			// call's concern: unanchored/foreign-target comments are the
			// board's inbox-tray population (05), not a per-ref
			// list_annotations result.
			continue
		}
		status := "open"
		if c.ThreadID != "" && resolvedThreads[c.ThreadID] {
			status = "resolved"
		}
		items = append(items, annotationItem{
			ID:       "review/" + c.ID,
			TS:       c.CreatedAt,
			Author:   c.Author,
			Type:     "review",
			Body:     c.Body,
			Status:   status,
			ObjectID: objID,
		})
	}
	return items, nil
}

// readSpecFrontmatter reads unpinned's backing file from the current
// working tree (review population only ever applies to a design branch's
// OWN in-progress spec — never a pinned historical commit) and decodes it
// as spec frontmatter. ok is false for any reason the ref isn't a
// decodable spec right now (not found, not kind spec, decode failure) —
// deliberately swallowed here rather than surfaced as an error, since a
// non-spec target (an ADR, a diagram) legitimately has no review-token
// population at all, not a failure.
func (b *Backend) readSpecFrontmatter(unpinned artifact.Ref) (*artifact.SpecFrontmatter, bool) {
	ix, err := b.buildIndex()
	if err != nil {
		return nil, false
	}
	entry, ok := ix.Get(unpinned.String())
	if !ok || entry.Kind != "spec" {
		return nil, false
	}
	raw, err := os.ReadFile(entry.Path)
	if err != nil {
		return nil, false
	}
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return nil, false
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return nil, false
	}
	return spec, true
}
