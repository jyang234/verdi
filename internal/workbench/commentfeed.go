// Review mode's comment source (05 §Review stickies and forge
// round-trip: "the board pulls the MR's full comment feed on every
// render"). The interface is defined HERE, at the consumer (04 §port
// pattern; the V1-P6 brief's "Stubs" note): V1-P7 owns internal/forge's
// comment methods, and the wave-close reconciliation adapts that port
// over this interface — the workbench never imports internal/forge.
package workbench

import (
	"context"
	"fmt"
	"os"

	"github.com/OWNER/verdi/internal/artifact"
)

// MRComment is one comment of a spec-MR's feed, in the only shape the
// board needs: who said what, and whether the forge thread is resolved.
type MRComment struct {
	ID       string `json:"id"`
	Author   string `json:"author"`
	Body     string `json:"body"`
	Resolved bool   `json:"resolved"`
}

// CommentFeed answers "is this spec under MR review, and what does its
// review conversation say?" — the review-mode mirror's fourth projection
// input. Implementations: the canned file feed below (hermetic e2e), a
// test fake, and (post V1-P7) an adapter over internal/forge's port.
type CommentFeed interface {
	// ListMRComments returns the full comment feed of the named spec's
	// open spec-MR in forge order, or ok=false when the spec has no open
	// MR (the spec is then not under review).
	ListMRComments(ctx context.Context, specName string) (comments []MRComment, ok bool, err error)
}

// commentToken extracts the leading [vd:<object-id>] token's object id,
// or "" when the body carries none. The grammar is single-sourced in
// internal/artifact (02 §Record schemas' owner); the board resolves the
// returned id against the spec's declared objects (a non-resolving token
// routes to the inbox tray, never dropped — projection.go), so this
// package parses the token grammar only, exactly as the forge/gate/mcp
// side does (W4 M-3: the two copies are unified on
// artifact.ParseCommentToken).
func commentToken(body string) string {
	id, _ := artifact.ParseCommentToken(body)
	return id
}

// CannedCommentFeed is a CommentFeed backed by one strict-decoded JSON
// file mapping spec name → comment list — the hermetic double the e2e
// harness wires through `verdi serve` (no network in any test,
// CLAUDE.md). Specs absent from the file are not under review.
type CannedCommentFeed struct {
	feeds map[string][]MRComment
}

// LoadCannedCommentFeed strict-decodes path into a CannedCommentFeed.
func LoadCannedCommentFeed(path string) (*CannedCommentFeed, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("workbench: reading canned comment feed: %w", err)
	}
	var feeds map[string][]MRComment
	if err := artifact.DecodeStrictJSON(data, &feeds); err != nil {
		return nil, fmt.Errorf("workbench: canned comment feed %s: %w", path, err)
	}
	return &CannedCommentFeed{feeds: feeds}, nil
}

// ListMRComments implements CommentFeed.
func (c *CannedCommentFeed) ListMRComments(_ context.Context, specName string) ([]MRComment, bool, error) {
	comments, ok := c.feeds[specName]
	if !ok {
		return nil, false, nil
	}
	return comments, true, nil
}
