package designapp

import (
	"context"
	"errors"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/workbench"
)

// BoardLoader is the consumer-owned port over the existing board
// projection owner (internal/workbench.LoadProjection, "the board
// projection owner behind internal/mcpserve/tool_get_board.go" per the
// Task 1 brief). Ports are resolved wholesale by the caller: MCP's own
// adapter (internal/mcpserve) builds a live-or-absent review-feed loader
// from its Backend.Forge exactly as tool_get_board.go always has; the
// production default here (workbenchBoardLoader) never configures a
// forge, matching the "no forge configured" silent-and-legitimate case
// that function's own doc comment already names. designapp never learns
// forge/review-feed concepts itself — get_board's AC-8 scope is the
// deterministic board projection, not review-sticky mirroring (an
// unrelated, pre-existing I-1(b) concern).
type BoardLoader interface {
	LoadProjection(ctx context.Context, root, name string) (proj *workbench.BoardProjection, reviewNotice string, err error)
}

type workbenchBoardLoader struct{}

func (workbenchBoardLoader) LoadProjection(ctx context.Context, root, name string) (*workbench.BoardProjection, string, error) {
	return workbench.LoadProjection(ctx, root, name, nil, "", nil)
}

// GetBoardRequest names the one spec whose board projection to return.
type GetBoardRequest struct {
	Spec string
}

func (r GetBoardRequest) validate() error {
	ref, err := artifact.ParseRef(r.Spec)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return errors.New("designapp: get_board spec must be an unpinned whole spec ref")
	}
	return nil
}

// BoardResult is get_board's exact result shape: this envelope's own
// version (CO-2), the board projection itself, marshaled exactly as
// computed — get_board never reimplements the projection (AC-8) — plus the
// pre-existing, unrelated I-1(b) review-population disclosure (present
// only when a configured forge could not be consulted). Schema versions
// THIS envelope; the embedded projection's own fields keep workbench's
// existing grammar byte-for-byte, since the workbench splice path renders
// the same struct.
type BoardResult struct {
	Schema string `json:"schema"`
	*workbench.BoardProjection
	Identity          draftmutation.Identity `json:"identity"`
	ReviewUnavailable string                 `json:"review_unavailable,omitempty"`
}

// GetBoard returns the same deterministic board projection the human
// workbench renders (AC-8). It composes workbench.LoadProjection through
// s.Board and never re-derives element taxonomy, badges, or annotations
// itself.
func (s Service) GetBoard(ctx context.Context, start string, req GetBoardRequest) (*BoardResult, *Error) {
	if err := req.validate(); err != nil {
		return nil, inputInvalid("input-invalid", err.Error())
	}
	identity, typed := s.resolveIdentity(ctx, start, req.Spec)
	if typed != nil {
		return nil, typed
	}
	ref, err := artifact.ParseRef(identity.Spec)
	if err != nil {
		return nil, operational("authority-invalid", "parsing canonical spec identity", err)
	}
	if s.Board == nil {
		return nil, operational("board-loader-unavailable", "board loader is not configured", nil)
	}
	proj, reviewNotice, err := s.Board.LoadProjection(ctx, identity.Checkout, ref.Name)
	if errors.Is(err, workbench.ErrBoardNotFound) {
		return nil, notFound("board-not-found", "no such spec board: "+identity.Spec)
	}
	if err != nil {
		return nil, operational("board-load-failed", "loading board projection", err)
	}
	return &BoardResult{
		Schema: BoardResultSchema, BoardProjection: cloneBoardProjection(proj),
		Identity: identity, ReviewUnavailable: reviewNotice,
	}, nil
}

// cloneBoardProjection returns a deep copy of the loader's projection.
// GetBoard hands the result to a caller it does not control, and the
// BoardLoader port is free to retain, cache, or share the value it
// returned; handing back that same pointer would let a caller mutate the
// port's own state through the result (Task 1's contract: results are
// "deterministic, deep-copy safe"). This is a copy, never a second
// projection: not one field's value is computed, reordered, or reinterpreted
// here — the element taxonomy stays workbench's alone (AC-8).
//
// The initial struct assignment copies every scalar AND workbench's own
// unexported render vocabulary, which this package cannot name (and must
// not: it holds a shared, read-only *model.Model, so copying the handle
// is the correct depth). Every exported field that carries a slice or map
// is then replaced with its own copy, descending into the nested
// collections the element types carry. TestBoardProjectionCloneCoverage
// ratchets the field inventory this walk is written against, so a field
// added upstream fails a test here instead of silently staying aliased.
func cloneBoardProjection(p *workbench.BoardProjection) *workbench.BoardProjection {
	if p == nil {
		return nil
	}
	clone := *p
	clone.Cards = cloneSlice(p.Cards)
	for i := range clone.Cards {
		clone.Cards[i].Anchored = cloneSlice(p.Cards[i].Anchored)
		clone.Cards[i].Obligations = cloneSlice(p.Cards[i].Obligations)
		// Badges (here and on stubs and the case file) is the one nested
		// element type carrying collections of its own. workbench's badge
		// view type is unexported, so it cannot be named in a shared
		// helper's signature; the copy is written out at each of its three
		// sites instead, against the same ratcheted field inventory.
		clone.Cards[i].Badges = cloneSlice(p.Cards[i].Badges)
		for j := range clone.Cards[i].Badges {
			src := p.Cards[i].Badges[j]
			clone.Cards[i].Badges[j].Inputs = cloneSlice(src.Inputs)
			clone.Cards[i].Badges[j].Records = cloneSlice(src.Records)
			clone.Cards[i].Badges[j].Disclosures = cloneSlice(src.Disclosures)
			clone.Cards[i].Badges[j].Provenance = cloneSlice(src.Provenance)
		}
	}
	clone.RefCards = cloneSlice(p.RefCards)
	clone.Edges = cloneSlice(p.Edges)
	clone.Stickies = cloneSlice(p.Stickies)
	clone.Tray = cloneSlice(p.Tray)
	clone.StubViews = cloneSlice(p.StubViews)
	for i := range clone.StubViews {
		clone.StubViews[i].Resolves = cloneSlice(p.StubViews[i].Resolves)
		clone.StubViews[i].AcceptanceCriteria = cloneSlice(p.StubViews[i].AcceptanceCriteria)
		clone.StubViews[i].StoryLinks = cloneSlice(p.StubViews[i].StoryLinks)
		clone.StubViews[i].Badges = cloneSlice(p.StubViews[i].Badges)
		for j := range clone.StubViews[i].Badges {
			src := p.StubViews[i].Badges[j]
			clone.StubViews[i].Badges[j].Inputs = cloneSlice(src.Inputs)
			clone.StubViews[i].Badges[j].Records = cloneSlice(src.Records)
			clone.StubViews[i].Badges[j].Disclosures = cloneSlice(src.Disclosures)
			clone.StubViews[i].Badges[j].Provenance = cloneSlice(src.Provenance)
		}
	}
	clone.ACCoverage = cloneMap(p.ACCoverage)
	clone.OQClaims = cloneMap(p.OQClaims)
	clone.CreateFields = cloneSlice(p.CreateFields)
	clone.Notices = cloneSlice(p.Notices)
	clone.CaseFileBadges = cloneSlice(p.CaseFileBadges)
	for j := range clone.CaseFileBadges {
		src := p.CaseFileBadges[j]
		clone.CaseFileBadges[j].Inputs = cloneSlice(src.Inputs)
		clone.CaseFileBadges[j].Records = cloneSlice(src.Records)
		clone.CaseFileBadges[j].Disclosures = cloneSlice(src.Disclosures)
		clone.CaseFileBadges[j].Provenance = cloneSlice(src.Provenance)
	}
	clone.CaseFileDisclosures = cloneSlice(p.CaseFileDisclosures)
	return &clone
}

// cloneSlice copies one slice, preserving the nil/empty distinction — a
// nil slice and an empty one marshal differently (null vs []), and this
// package's whole point is that two adapters serialize identically.
func cloneSlice[T any](in []T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	copy(out, in)
	return out
}

// cloneMap copies one map, preserving nil for the same reason cloneSlice
// does.
func cloneMap[K comparable, V any](in map[K]V) map[K]V {
	if in == nil {
		return nil
	}
	out := make(map[K]V, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
