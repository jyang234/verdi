// The real forge adapter over workbench.CommentFeed (W4 wave-close
// reconciliation; V1-P6 report residual 1). The board's review-mode mirror
// asks "is this spec under MR review, and what does its conversation say?"
// via workbench.CommentFeed — a consumer-defined port (04 §port pattern).
// V1-P6 shipped an interim CannedCommentFeed (a hermetic local JSON file)
// and left the real adaptation for the wave close. This is that adapter:
// it lives in cmd/verdi (not internal/workbench) so the workbench never
// imports internal/forge — the dependency direction the port pattern
// wants, and the one serve.go's existing wiring already reads in (it
// builds the forge best-effort here and hands finished dependencies to the
// workbench). It reuses forge.FindOpenMR — the one branch-scoped
// MR-discovery mechanism mcpserve/review.go (R4-I-29) and
// gate_threads.go's review-thread gate also share.
package main

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/workbench"
)

// forgeCommentFeed adapts a forge.Forge over workbench.CommentFeed: it
// maps a spec name to its design branch, finds that branch's open spec-MR,
// and joins the MR's comment feed with per-thread resolution state.
type forgeCommentFeed struct {
	f    forge.Forge
	root string
}

// newForgeCommentFeed wraps f for the checkout rooted at root.
func newForgeCommentFeed(f forge.Forge, root string) *forgeCommentFeed {
	return &forgeCommentFeed{f: f, root: root}
}

// ListMRComments implements workbench.CommentFeed. A spec named `name`
// lives on the design branch `design/<name>` (cmd/verdi/design.go's own
// convention). This finds the open MR whose source branch is that design
// branch (targeting the store's resolved default branch), lists its full
// comment feed in forge order, and stamps each comment's Resolved from its
// forge thread's resolution state. ok is false — the spec is not under
// review — when the default branch cannot be resolved or no such MR is
// open yet (a design branch not yet pushed/opened): honestly "nothing to
// mirror", never an error, mirroring mcpserve/review.go's posture.
func (a *forgeCommentFeed) ListMRComments(ctx context.Context, name string) ([]workbench.MRComment, bool, error) {
	branch := "design/" + name

	defaultBranch := lint.ResolveDefaultBranch(ctx, a.root)
	if defaultBranch == "" {
		return nil, false, nil
	}

	mrID, err := forge.FindOpenMR(ctx, a.f, defaultBranch, branch)
	if err != nil {
		return nil, false, fmt.Errorf("verdi: finding open spec-MR for %s: %w", name, err)
	}
	if mrID == "" {
		return nil, false, nil
	}

	comments, err := a.f.ListComments(ctx, mrID)
	if err != nil {
		return nil, false, fmt.Errorf("verdi: listing review comments for MR %s: %w", mrID, err)
	}

	// Resolution state is best-effort (mirrors mcpserve/review.go): a query
	// failure leaves every comment reported unresolved — the safe,
	// conservative default (never silently claiming "resolved" on
	// incomplete information), not a failure that hides the whole feed.
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

var _ workbench.CommentFeed = (*forgeCommentFeed)(nil)
