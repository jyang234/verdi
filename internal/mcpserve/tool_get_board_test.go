package mcpserve

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/forge"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/workbench"
)

// getBoardOut is the shape TestGetBoard_* tests decode: workbench's
// exported BoardProjection fields plus the review_unavailable disclosure
// (tool_get_board.go's boardResult, decoded field-by-field here rather
// than embedding workbench.BoardProjection, since JSON decode into an
// embedded exported struct works identically to a flat one).
type getBoardOut struct {
	Spec  string `json:"spec"`
	Title string `json:"title"`
	Mode  string `json:"mode"`
	Cards []struct {
		ID       string `json:"id"`
		Kind     string `json:"kind"`
		Anchored []struct {
			Anchor   string `json:"anchor"`
			Body     string `json:"body"`
			Resolved bool   `json:"resolved"`
		} `json:"anchored"`
	} `json:"cards"`
	ReviewUnavailable string `json:"review_unavailable"`
}

// TestGetBoard_Happy proves get_board projects the SAME board
// (workbench.LoadProjection) a human sees at /board/spec/{name} — spec,
// title, and the declared AC card, for an already-accepted spec (readonly
// mode: no design branch, no open MR).
func TestGetBoard_Happy(t *testing.T) {
	b, _, _ := newTestBackend(t)
	result := b.GetBoard(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry"}))
	var out getBoardOut
	toolResultJSON(t, result, &out)

	if out.Spec != "widget-retry" {
		t.Fatalf("Spec = %q, want widget-retry", out.Spec)
	}
	if out.Mode != "readonly" {
		t.Fatalf("Mode = %q, want readonly (accepted spec, no design branch)", out.Mode)
	}
	found := false
	for _, c := range out.Cards {
		if c.ID == "ac-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Cards = %+v, want a card for ac-1", out.Cards)
	}
	if out.ReviewUnavailable != "" {
		t.Fatalf("ReviewUnavailable = %q, want empty (no forge configured)", out.ReviewUnavailable)
	}
}

func TestGetBoard_Negative(t *testing.T) {
	b, _, _ := newTestBackend(t)
	ctx := context.Background()

	t.Run("missing ref", func(t *testing.T) {
		result := b.GetBoard(ctx, mustArgs(t, map[string]any{}))
		if !isToolError(result) {
			t.Fatal("get_board(no ref): want isError")
		}
	})

	t.Run("malformed ref", func(t *testing.T) {
		result := b.GetBoard(ctx, mustArgs(t, map[string]any{"ref": "not-a-ref"}))
		if !isToolError(result) {
			t.Fatal("get_board(malformed ref): want isError")
		}
	})

	t.Run("non-spec ref rejected", func(t *testing.T) {
		result := b.GetBoard(ctx, mustArgs(t, map[string]any{"ref": "adr/0001-outbox"}))
		if !isToolError(result) {
			t.Fatal("get_board(adr ref): want isError (get_board only projects specs)")
		}
	})

	t.Run("object fragment rejected", func(t *testing.T) {
		result := b.GetBoard(ctx, mustArgs(t, map[string]any{"ref": "spec/widget-retry#ac-1"}))
		if !isToolError(result) {
			t.Fatal("get_board(object fragment): want isError")
		}
	})

	t.Run("pinned ref rejected", func(t *testing.T) {
		_, _, adrCommit := newTestBackend(t)
		result := b.GetBoard(ctx, mustArgs(t, map[string]any{"ref": "spec/widget-retry@" + adrCommit}))
		if !isToolError(result) {
			t.Fatal("get_board(pinned ref): want isError (the board only ever projects the current working tree)")
		}
	})

	t.Run("unknown spec", func(t *testing.T) {
		result := b.GetBoard(ctx, mustArgs(t, map[string]any{"ref": "spec/does-not-exist"}))
		if !isToolError(result) {
			t.Fatal("get_board(unknown spec): want isError")
		}
	})
}

// TestGetBoard_ReviewDisclosureStates drives the same I-1(b) three states
// list_annotations' review population proves (review_test.go), through
// get_board instead: no forge configured is silent; a configured-but-
// unreachable forge discloses via review_unavailable; a live, reachable
// forge with an open MR flips the board into review mode and mirrors a
// token-anchored comment onto its object's card.
func TestGetBoard_ReviewDisclosureStates(t *testing.T) {
	t.Run("no forge configured: silent, authoring mode from branch state alone", func(t *testing.T) {
		repo := buildReviewFixture(t)
		b := &Backend{Root: repo.Dir}
		result := b.GetBoard(context.Background(), mustArgs(t, map[string]any{"ref": "spec/loan-update"}))
		var out getBoardOut
		toolResultJSON(t, result, &out)
		if out.Mode != "authoring" {
			t.Fatalf("Mode = %q, want authoring (draft spec on a design branch, no review feed)", out.Mode)
		}
		if out.ReviewUnavailable != "" {
			t.Fatalf("ReviewUnavailable = %q, want empty (no forge configured is silent)", out.ReviewUnavailable)
		}
	})

	t.Run("forge configured but unavailable: disclosed, never silent", func(t *testing.T) {
		repo := buildReviewFixture(t)
		b := &Backend{Root: repo.Dir, ReviewUnavailable: `forge "gitlab" is configured but no credentials are available`}
		result := b.GetBoard(context.Background(), mustArgs(t, map[string]any{"ref": "spec/loan-update"}))
		var out getBoardOut
		toolResultJSON(t, result, &out)
		if out.ReviewUnavailable == "" {
			t.Fatal("ReviewUnavailable = empty, want the configured-but-unavailable reason (never silent, constitution 2/10)")
		}
	})

	t.Run("live forge: review mode, token-anchored comment mirrored onto its card", func(t *testing.T) {
		repo := buildReviewFixture(t)
		t.Setenv("CI_DEFAULT_BRANCH", "main")
		f := forgefake.New()
		f.SeedOpenMR("main", forge.OpenMR{ID: "5", SourceBranch: "design/loan-update", Title: "Loan update"})
		f.SeedComment("5", forge.Comment{ID: "c1", ThreadID: "t1", Body: "[vd:ac-2] outcome AC reads implementation-scoped — reword?", Author: "reviewer", CreatedAt: "2026-07-11T18:00:00Z"})
		f.SeedThreadResolution("5", forge.ThreadResolution{ThreadID: "t1", Resolved: true, ResolvedBy: "reviewer"})

		b := &Backend{Root: repo.Dir, Forge: f}
		result := b.GetBoard(context.Background(), mustArgs(t, map[string]any{"ref": "spec/loan-update"}))
		var out getBoardOut
		toolResultJSON(t, result, &out)

		if out.Mode != "review" {
			t.Fatalf("Mode = %q, want review (an open spec-MR mirrors as the board's fourth input)", out.Mode)
		}
		if out.ReviewUnavailable != "" {
			t.Fatalf("ReviewUnavailable = %q, want empty (live forge, reachable)", out.ReviewUnavailable)
		}
		var card *struct {
			ID       string `json:"id"`
			Kind     string `json:"kind"`
			Anchored []struct {
				Anchor   string `json:"anchor"`
				Body     string `json:"body"`
				Resolved bool   `json:"resolved"`
			} `json:"anchored"`
		}
		for i := range out.Cards {
			if out.Cards[i].ID == "ac-2" {
				card = &out.Cards[i]
			}
		}
		if card == nil {
			t.Fatalf("Cards = %+v, want an ac-2 card", out.Cards)
		}
		if len(card.Anchored) != 1 || card.Anchored[0].Resolved != true {
			t.Fatalf("ac-2's Anchored = %+v, want exactly one RESOLVED review sticky", card.Anchored)
		}
	})
}

// TestGetBoard_MatchesWorkbenchProjection proves get_board never
// reimplements the projection: its result mirrors
// workbench.LoadProjection's own output exactly for the same inputs.
func TestGetBoard_MatchesWorkbenchProjection(t *testing.T) {
	b, _, _ := newTestBackend(t)
	ctx := context.Background()

	direct, _, err := workbench.LoadProjection(ctx, b.Root, "widget-retry", nil, "")
	if err != nil {
		t.Fatalf("workbench.LoadProjection: %v", err)
	}

	result := b.GetBoard(ctx, mustArgs(t, map[string]any{"ref": "spec/widget-retry"}))
	var out getBoardOut
	toolResultJSON(t, result, &out)

	if out.Spec != direct.Spec || out.Title != direct.Title || out.Mode != string(direct.Mode) {
		t.Fatalf("get_board result diverges from workbench.LoadProjection: got %+v, direct spec=%s title=%s mode=%s", out, direct.Spec, direct.Title, direct.Mode)
	}
	if len(out.Cards) != len(direct.Cards) {
		t.Fatalf("get_board returned %d card(s), workbench.LoadProjection computed %d", len(out.Cards), len(direct.Cards))
	}
}
