package fake_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jyang234/verdi/internal/provider"
	"github.com/jyang234/verdi/internal/provider/fake"
)

func TestProvider_Resolve_Unseeded(t *testing.T) {
	p := fake.New()
	_, err := p.Resolve(context.Background(), "jira:MISSING-1")
	if !errors.Is(err, provider.ErrNotFound) {
		t.Fatalf("Resolve error = %v, want errors.Is(err, provider.ErrNotFound)", err)
	}
}

func TestProvider_FailResolve(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"not found", fmt.Errorf("simulated: %w", provider.ErrNotFound)},
		{"unauthorized", fmt.Errorf("simulated: %w", provider.ErrUnauthorized)},
		{"unavailable", fmt.Errorf("simulated: %w", provider.ErrUnavailable)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := fake.New()
			ref := provider.StoryRef("jira:X-1")
			p.SeedStory(provider.Story{Ref: ref, Title: "X"})
			p.FailResolve(ref, tc.err)

			_, err := p.Resolve(context.Background(), ref)
			if !errors.Is(err, tc.err) {
				t.Fatalf("Resolve error = %v, want errors.Is(err, %v)", err, tc.err)
			}
		})
	}
}

func TestProvider_ClearResolveFailure(t *testing.T) {
	p := fake.New()
	ref := provider.StoryRef("jira:X-1")
	story := provider.Story{Ref: ref, Title: "X"}
	p.SeedStory(story)
	p.FailResolve(ref, provider.ErrUnavailable)

	if _, err := p.Resolve(context.Background(), ref); !errors.Is(err, provider.ErrUnavailable) {
		t.Fatalf("Resolve error = %v before clearing, want ErrUnavailable", err)
	}

	p.ClearResolveFailure(ref)
	got, err := p.Resolve(context.Background(), ref)
	if err != nil {
		t.Fatalf("Resolve error = %v after ClearResolveFailure, want nil", err)
	}
	if got != story {
		t.Fatalf("Resolve = %+v, want %+v", got, story)
	}
}

func TestProvider_Resolve_ContextCanceled(t *testing.T) {
	p := fake.New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Resolve(ctx, "jira:X-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Resolve error = %v, want errors.Is(err, context.Canceled)", err)
	}
}

func TestProvider_QueuePublishError(t *testing.T) {
	p := fake.New()
	story := provider.StoryRef("jira:X-1")
	roll := provider.Rollup{Story: story, Ref: "spec/x", Commit: "c1"}

	wantErr := fmt.Errorf("simulated: %w", provider.ErrUnauthorized)
	p.QueuePublishError(story, wantErr)

	// The queued failure applies to exactly one call.
	if err := p.PublishRollup(context.Background(), roll); !errors.Is(err, wantErr) {
		t.Fatalf("first PublishRollup error = %v, want errors.Is(err, %v)", err, wantErr)
	}
	if err := p.PublishRollup(context.Background(), roll); err != nil {
		t.Fatalf("second PublishRollup error = %v, want nil (queued failure is consumed after one use)", err)
	}
	if got := p.PublishRecordCount(story); got != 1 {
		t.Fatalf("PublishRecordCount = %d, want 1 (the failed call must not record anything)", got)
	}
}

func TestProvider_PublishRollup_ContextCanceled(t *testing.T) {
	p := fake.New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.PublishRollup(ctx, provider.Rollup{Story: "jira:X-1", Ref: "spec/x", Commit: "c1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PublishRollup error = %v, want errors.Is(err, context.Canceled)", err)
	}
}

func TestProvider_PublishedField_Unpublished(t *testing.T) {
	p := fake.New()
	_, ok := p.PublishedField("jira:X-1")
	if ok {
		t.Fatalf("PublishedField ok = true for a story that was never published, want false")
	}
}

func TestProvider_Comments_TracksContentAndOrder(t *testing.T) {
	p := fake.New()
	story := provider.StoryRef("jira:X-1")

	first := provider.Rollup{
		Story:  story,
		Ref:    "spec/x",
		Commit: "c1",
		Criteria: []provider.CriterionStatus{
			{ID: "ac-1", Status: "pending"},
		},
	}
	second := first
	second.Commit = "c2"
	second.Criteria = []provider.CriterionStatus{
		{ID: "ac-1", Status: "evidenced"},
	}

	if err := p.PublishRollup(context.Background(), first); err != nil {
		t.Fatalf("first PublishRollup error = %v, want nil", err)
	}
	if err := p.PublishRollup(context.Background(), second); err != nil {
		t.Fatalf("second PublishRollup error = %v, want nil", err)
	}

	comments := p.Comments(story)
	if len(comments) != 2 {
		t.Fatalf("len(Comments) = %d, want 2 (both publishes changed AC statuses from the prior state)", len(comments))
	}
	if comments[0].Rollup.Commit != "c1" || comments[1].Rollup.Commit != "c2" {
		t.Fatalf("Comments = %+v, want commits in publish order [c1, c2]", comments)
	}
}

func TestProvider_Comments_ReturnsACopy(t *testing.T) {
	p := fake.New()
	story := provider.StoryRef("jira:X-1")
	roll := provider.Rollup{Story: story, Ref: "spec/x", Commit: "c1"}
	if err := p.PublishRollup(context.Background(), roll); err != nil {
		t.Fatalf("PublishRollup error = %v, want nil", err)
	}

	got := p.Comments(story)
	got[0].Rollup.Commit = "mutated"

	again := p.Comments(story)
	if again[0].Rollup.Commit != "c1" {
		t.Fatalf("Comments()[0].Rollup.Commit = %q after mutating a prior copy, want unaffected %q", again[0].Rollup.Commit, "c1")
	}
}
