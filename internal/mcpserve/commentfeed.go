// Adapts Backend.Forge over workbench.CommentFeed for get_board's
// review-mode mirror (05 §MCP server's get_board row: "mode-appropriate
// annotations a human sees in `verdi serve`" — the review-comment feed is
// the fourth projection input, 05 §Workbench "Board as projection"). This
// reuses forge.FindOpenMR, the same branch-scoped MR-discovery mechanism
// review.go's reviewMirroredAnnotations and cmd/verdi/reviewfeed.go's
// forgeCommentFeed both already share (R4-I-29) — mcpserve cannot import
// cmd/verdi's adapter (package main), and internal/workbench must never
// import internal/forge (04 §port pattern: the interface is defined at
// workbench, the consumer), so this is a third, package-local instance of
// the same small adapter shape, not a reimplementation of the projection.
package mcpserve

import (
	"context"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/workbench"
)

// backendCommentFeed adapts f (a Backend.Forge) over workbench.CommentFeed
// for the checkout rooted at root.
type backendCommentFeed struct {
	f    forge.Forge
	root string
}

// ListMRComments implements workbench.CommentFeed. A spec named `name`
// lives on the design branch `design/<name>` (the same convention
// cmd/verdi/design.go and cmd/verdi/reviewfeed.go use). ok is false —
// "not under review" — when the default branch cannot be resolved or no
// open MR is found for that design branch: honestly "nothing to mirror",
// never an error (mirrors review.go's posture).
func (a backendCommentFeed) ListMRComments(ctx context.Context, name string) ([]workbench.MRComment, bool, error) {
	branch := "design/" + name

	defaultBranch := lint.ResolveDefaultBranch(ctx, a.root)
	if defaultBranch == "" {
		return nil, false, nil
	}

	mrID, err := forge.FindOpenMR(ctx, a.f, defaultBranch, branch)
	if err != nil {
		return nil, false, err
	}
	if mrID == "" {
		return nil, false, nil
	}

	comments, err := a.f.ListComments(ctx, mrID)
	if err != nil {
		return nil, false, err
	}

	// Resolution state is best-effort (mirrors review.go/reviewfeed.go): a
	// query failure leaves every comment reported unresolved — the safe,
	// conservative default, not a failure that hides the whole feed.
	resolved := map[string]bool{}
	if threads, terr := a.f.GetThreadResolution(ctx, mrID); terr == nil {
		for _, tr := range threads {
			resolved[tr.ThreadID] = tr.Resolved
		}
	}

	out := make([]workbench.MRComment, 0, len(comments))
	for _, c := range comments {
		out = append(out, workbench.MRComment{
			ID:       c.ID,
			Author:   c.Author,
			Body:     c.Body,
			Resolved: c.ThreadID != "" && resolved[c.ThreadID],
		})
	}
	return out, true, nil
}

var _ workbench.CommentFeed = backendCommentFeed{}
