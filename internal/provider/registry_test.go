package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jyang234/verdi/internal/provider"
	"github.com/jyang234/verdi/internal/provider/fake"
)

func TestRegistry_Provider(t *testing.T) {
	jiraAdapter := fake.New()
	reg := provider.NewRegistry(map[string]provider.StoryProvider{
		"jira": jiraAdapter,
	})

	t.Run("known scheme", func(t *testing.T) {
		p, err := reg.Provider("jira")
		if err != nil {
			t.Fatalf("Provider(%q) error = %v, want nil", "jira", err)
		}
		if p != jiraAdapter {
			t.Fatalf("Provider(%q) returned a different adapter than registered", "jira")
		}
	})

	t.Run("unknown scheme", func(t *testing.T) {
		_, err := reg.Provider("gitlab")
		if err == nil {
			t.Fatalf("Provider(%q) error = nil, want ErrUnknownScheme", "gitlab")
		}
		if !errors.Is(err, provider.ErrUnknownScheme) {
			t.Fatalf("Provider(%q) error = %v, want errors.Is(err, provider.ErrUnknownScheme)", "gitlab", err)
		}
	})
}

func TestRegistry_CopiesInputMap(t *testing.T) {
	m := map[string]provider.StoryProvider{"jira": fake.New()}
	reg := provider.NewRegistry(m)

	// Mutating the caller's map after construction must not affect the
	// registry.
	delete(m, "jira")
	m["gitlab"] = fake.New()

	if _, err := reg.Provider("jira"); err != nil {
		t.Fatalf("Provider(%q) error = %v after caller mutated its map, want nil (registry copies)", "jira", err)
	}
	if _, err := reg.Provider("gitlab"); err == nil {
		t.Fatalf("Provider(%q) error = nil, want ErrUnknownScheme (registry copies, should not see post-construction additions)", "gitlab")
	}
}

func TestRegistry_Resolve(t *testing.T) {
	jiraAdapter := fake.New()
	story := provider.Story{Ref: "jira:LOAN-1482", Title: "Loan flow", Status: "in-progress", URL: "https://example.invalid/LOAN-1482"}
	jiraAdapter.SeedStory(story)

	reg := provider.NewRegistry(map[string]provider.StoryProvider{"jira": jiraAdapter})

	t.Run("happy path delegates to the registered adapter", func(t *testing.T) {
		got, err := reg.Resolve(context.Background(), story.Ref)
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v, want nil", story.Ref, err)
		}
		if got != story {
			t.Fatalf("Resolve(%q) = %+v, want %+v", story.Ref, got, story)
		}
	})

	t.Run("unknown scheme", func(t *testing.T) {
		_, err := reg.Resolve(context.Background(), "gitlab:platform#482")
		if !errors.Is(err, provider.ErrUnknownScheme) {
			t.Fatalf("Resolve error = %v, want errors.Is(err, provider.ErrUnknownScheme)", err)
		}
	})

	t.Run("malformed ref", func(t *testing.T) {
		_, err := reg.Resolve(context.Background(), "not-a-valid-ref")
		if !errors.Is(err, provider.ErrInvalidRef) {
			t.Fatalf("Resolve error = %v, want errors.Is(err, provider.ErrInvalidRef)", err)
		}
	})
}

func TestRegistry_PublishRollup(t *testing.T) {
	jiraAdapter := fake.New()
	reg := provider.NewRegistry(map[string]provider.StoryProvider{"jira": jiraAdapter})

	roll := provider.Rollup{
		Story:  "jira:LOAN-1482",
		Ref:    "spec/loan-flow",
		Commit: "abc123",
		Criteria: []provider.CriterionStatus{
			{ID: "ac-1", Text: "x", Status: "evidenced", Summary: "y"},
		},
		Eligible: true,
	}

	t.Run("happy path delegates to the registered adapter", func(t *testing.T) {
		if err := reg.PublishRollup(context.Background(), roll); err != nil {
			t.Fatalf("PublishRollup error = %v, want nil", err)
		}
		got, ok := jiraAdapter.PublishedField(roll.Story)
		if !ok || got.Commit != roll.Commit {
			t.Fatalf("underlying adapter did not record the publish: got=%+v ok=%v", got, ok)
		}
	})

	t.Run("unknown scheme", func(t *testing.T) {
		bad := roll
		bad.Story = "gitlab:platform#482"
		err := reg.PublishRollup(context.Background(), bad)
		if !errors.Is(err, provider.ErrUnknownScheme) {
			t.Fatalf("PublishRollup error = %v, want errors.Is(err, provider.ErrUnknownScheme)", err)
		}
	})

	t.Run("malformed story ref", func(t *testing.T) {
		bad := roll
		bad.Story = "not-a-valid-ref"
		err := reg.PublishRollup(context.Background(), bad)
		if !errors.Is(err, provider.ErrInvalidRef) {
			t.Fatalf("PublishRollup error = %v, want errors.Is(err, provider.ErrInvalidRef)", err)
		}
	})
}
