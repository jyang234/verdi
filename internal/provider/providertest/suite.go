package providertest

import (
	"context"
	"errors"
	"testing"

	"github.com/jyang234/verdi/internal/provider"
)

// Run executes the story-provider contract suite (04 §Testing) against
// the harness newHarness builds. newHarness is called once per subtest
// so each gets isolated state.
func Run(t *testing.T, newHarness func(t *testing.T) Harness) {
	t.Helper()
	t.Run("resolve happy path", func(t *testing.T) {
		testResolveHappyPath(t, newHarness(t))
	})
	t.Run("resolve not found", func(t *testing.T) {
		testResolveNotFound(t, newHarness(t))
	})
	t.Run("publish idempotency", func(t *testing.T) {
		testPublishIdempotency(t, newHarness(t))
	})
	t.Run("comment only on change", func(t *testing.T) {
		testCommentOnlyOnChange(t, newHarness(t))
	})
}

func testResolveHappyPath(t *testing.T, h Harness) {
	t.Helper()
	story := provider.Story{
		Ref:    "test:HAPPY-1",
		Title:  "Happy path story",
		Status: "in-progress",
		URL:    "https://example.invalid/HAPPY-1",
	}
	h.SeedStory(t, story)

	got, err := h.Provider().Resolve(context.Background(), story.Ref)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v, want nil", story.Ref, err)
	}
	if got.Ref != story.Ref {
		t.Fatalf("Resolve(%q).Ref = %q, want %q", story.Ref, got.Ref, story.Ref)
	}
	if got.Title != story.Title {
		t.Fatalf("Resolve(%q).Title = %q, want %q", story.Ref, got.Title, story.Title)
	}
	if got.Status != story.Status {
		t.Fatalf("Resolve(%q).Status = %q, want %q", story.Ref, got.Status, story.Status)
	}
	// URL is compared against the harness's declaration rather than the
	// seeded value (I-33): an adapter that constructs the URL from its own
	// configuration cannot echo an arbitrary seeded URL back.
	if want := h.ExpectResolvedURL(story); got.URL != want {
		t.Fatalf("Resolve(%q).URL = %q, want %q", story.Ref, got.URL, want)
	}
}

func testResolveNotFound(t *testing.T, h Harness) {
	t.Helper()
	ref := provider.StoryRef("test:MISSING-1")
	h.SeedNotFound(t, ref)

	_, err := h.Provider().Resolve(context.Background(), ref)
	if err == nil {
		t.Fatalf("Resolve(%q) error = nil, want a not-found error", ref)
	}
	if !errors.Is(err, provider.ErrNotFound) {
		t.Fatalf("Resolve(%q) error = %v, want errors.Is(err, provider.ErrNotFound)", ref, err)
	}
}

func testPublishIdempotency(t *testing.T, h Harness) {
	t.Helper()
	story := provider.StoryRef("test:IDEM-1")
	roll := provider.Rollup{
		Story:  story,
		Ref:    "spec/idem-1",
		Commit: "abc123",
		Criteria: []provider.CriterionStatus{
			{ID: "ac-1", Text: "does the thing", Status: "evidenced", Summary: "covered"},
		},
		Eligible: true,
	}

	ctx := context.Background()
	if err := h.Provider().PublishRollup(ctx, roll); err != nil {
		t.Fatalf("first PublishRollup error = %v, want nil", err)
	}
	if err := h.Provider().PublishRollup(ctx, roll); err != nil {
		t.Fatalf("second PublishRollup (same story+commit) error = %v, want nil", err)
	}

	if got := h.PublishRecordCount(t, story); got != 1 {
		t.Fatalf("PublishRecordCount = %d after two publishes of the same (story, commit), want 1 (an update, not a duplicate)", got)
	}
	field, ok := h.PublishedField(t, story)
	if !ok {
		t.Fatalf("PublishedField(%q) ok = false, want true after publish", story)
	}
	if field.Commit != roll.Commit || field.Eligible != roll.Eligible {
		t.Fatalf("PublishedField(%q) = %+v, want it to reflect the published rollup %+v", story, field, roll)
	}

	// A different commit for the same story is a distinct record.
	roll2 := roll
	roll2.Commit = "def456"
	if err := h.Provider().PublishRollup(ctx, roll2); err != nil {
		t.Fatalf("PublishRollup (new commit) error = %v, want nil", err)
	}
	if got := h.PublishRecordCount(t, story); got != 2 {
		t.Fatalf("PublishRecordCount = %d after publishing a new commit, want 2", got)
	}
}

func testCommentOnlyOnChange(t *testing.T, h Harness) {
	t.Helper()
	story := provider.StoryRef("test:COMMENT-1")
	base := provider.Rollup{
		Story:  story,
		Ref:    "spec/comment-1",
		Commit: "c1",
		Criteria: []provider.CriterionStatus{
			{ID: "ac-1", Text: "does the thing", Status: "pending", Summary: "no evidence yet"},
		},
		Eligible: false,
	}

	ctx := context.Background()
	if err := h.Provider().PublishRollup(ctx, base); err != nil {
		t.Fatalf("initial PublishRollup error = %v, want nil", err)
	}
	baseline := h.CommentCount(t, story)

	// Re-publish with unchanged AC statuses (a new commit, since the
	// idempotency case already covers the same-commit path): no new
	// comment.
	unchanged := base
	unchanged.Commit = "c2"
	if err := h.Provider().PublishRollup(ctx, unchanged); err != nil {
		t.Fatalf("unchanged republish PublishRollup error = %v, want nil", err)
	}
	if got := h.CommentCount(t, story); got != baseline {
		t.Fatalf("CommentCount after unchanged republish = %d, want %d (no comment when AC statuses are unchanged)", got, baseline)
	}

	// Now change an AC status: exactly one new comment.
	changed := unchanged
	changed.Commit = "c3"
	changed.Criteria = []provider.CriterionStatus{
		{ID: "ac-1", Text: "does the thing", Status: "evidenced", Summary: "now covered"},
	}
	changed.Eligible = true
	if err := h.Provider().PublishRollup(ctx, changed); err != nil {
		t.Fatalf("changed republish PublishRollup error = %v, want nil", err)
	}
	if got := h.CommentCount(t, story); got != baseline+1 {
		t.Fatalf("CommentCount after changed republish = %d, want %d (exactly one new comment on AC-status change)", got, baseline+1)
	}
}
