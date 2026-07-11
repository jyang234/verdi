package fake_test

import (
	"testing"

	"github.com/OWNER/verdi/internal/provider"
	"github.com/OWNER/verdi/internal/provider/fake"
	"github.com/OWNER/verdi/internal/provider/providertest"
)

// fakeHarness adapts fake.Provider to providertest.Harness. It is the
// reference harness the suite is designed against; a future Jira adapter
// package supplies its own, httptest-backed, implementing the same
// interface unchanged (04 §Testing).
type fakeHarness struct {
	p *fake.Provider
}

func newFakeHarness(t *testing.T) providertest.Harness {
	t.Helper()
	return &fakeHarness{p: fake.New()}
}

func (h *fakeHarness) Provider() provider.StoryProvider { return h.p }

func (h *fakeHarness) SeedStory(t *testing.T, story provider.Story) {
	t.Helper()
	h.p.SeedStory(story)
}

func (h *fakeHarness) SeedNotFound(t *testing.T, ref provider.StoryRef) {
	t.Helper()
	// The fake already returns a wrapped provider.ErrNotFound for any
	// ref that was never seeded; nothing to arrange.
}

func (h *fakeHarness) PublishedField(t *testing.T, story provider.StoryRef) (provider.Rollup, bool) {
	t.Helper()
	return h.p.PublishedField(story)
}

func (h *fakeHarness) PublishRecordCount(t *testing.T, story provider.StoryRef) int {
	t.Helper()
	return h.p.PublishRecordCount(story)
}

func (h *fakeHarness) CommentCount(t *testing.T, story provider.StoryRef) int {
	t.Helper()
	return h.p.CommentCount(story)
}

// TestFakeSatisfiesContract proves the fake passes the shared
// story-provider contract suite (04 §Testing), as required for this
// phase.
func TestFakeSatisfiesContract(t *testing.T) {
	providertest.Run(t, newFakeHarness)
}
