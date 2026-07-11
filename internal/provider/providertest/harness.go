package providertest

import (
	"testing"

	"github.com/OWNER/verdi/internal/provider"
)

// Harness lets Run drive an adapter under test and observe its published
// state abstractly, so the suite runs unchanged against any
// provider.StoryProvider implementation.
//
// Implementations should return fresh, isolated state from each call the
// NewHarness constructor passed to Run makes (Run calls it once per
// subtest) — e.g. a new fake.Provider, or a new httptest.Server plus a
// freshly constructed adapter pointed at it.
type Harness interface {
	// Provider returns the StoryProvider under test.
	Provider() provider.StoryProvider

	// SeedStory arranges for story.Ref to resolve to story.
	SeedStory(t *testing.T, story provider.Story)

	// SeedNotFound arranges for ref to fail Resolve with an error
	// satisfying errors.Is(err, provider.ErrNotFound), without a prior
	// SeedStory call for ref.
	SeedNotFound(t *testing.T, ref provider.StoryRef)

	// ExpectResolvedURL declares what URL the adapter under test must
	// produce for a seeded story (PLAN.md ledger I-33): store-backed
	// providers return the seeded story.URL unchanged; adapters that
	// construct the URL from their own configuration (e.g. the Jira
	// adapter's BaseURL+"/browse/"+key) return their constructed form.
	ExpectResolvedURL(story provider.Story) string

	// PublishedField returns the latest state Provider().PublishRollup
	// has durably recorded for story — the adapter's own machine field,
	// or an equivalent — and whether anything has been published yet.
	PublishedField(t *testing.T, story provider.StoryRef) (provider.Rollup, bool)

	// PublishRecordCount returns how many distinct commits have been
	// durably recorded for story. Republishing the same (story, commit)
	// must not increase it.
	PublishRecordCount(t *testing.T, story provider.StoryRef) int

	// CommentCount returns how many human comments the adapter has
	// fired for story so far.
	CommentCount(t *testing.T, story provider.StoryRef) int
}
