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
	return &BoardResult{Schema: BoardResultSchema, BoardProjection: proj, Identity: identity, ReviewUnavailable: reviewNotice}, nil
}
