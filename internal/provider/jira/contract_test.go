package jira_test

import (
	"testing"

	"github.com/jyang234/verdi/internal/provider"
	"github.com/jyang234/verdi/internal/provider/jira"
	"github.com/jyang234/verdi/internal/provider/jira/jiratest"
	"github.com/jyang234/verdi/internal/provider/providertest"
)

const testRollupField = "customfield_rollup"

// jiraHarness adapts an httptest-backed jira.Adapter to
// providertest.Harness — the payoff of the wave-2 port design (PLAN.md
// Phase 11): the exact same suite that drives internal/provider/fake also
// drives this real, HTTP-based adapter, unchanged.
type jiraHarness struct {
	server  *jiratest.Server
	adapter *jira.Adapter
}

func newJiraHarness(t *testing.T) providertest.Harness {
	t.Helper()
	server := jiratest.NewServer(testRollupField)
	t.Cleanup(server.Close)
	adapter := jira.New(jira.Config{
		BaseURL:     server.URL,
		RollupField: testRollupField,
		Token:       "test-token",
		HTTPClient:  server.Client(),
	})
	return &jiraHarness{server: server, adapter: adapter}
}

func (h *jiraHarness) Provider() provider.StoryProvider { return h.adapter }

func (h *jiraHarness) SeedStory(t *testing.T, story provider.Story) {
	t.Helper()
	_, key, err := provider.ParseStoryRef(story.Ref)
	if err != nil {
		t.Fatalf("ParseStoryRef(%q): %v", story.Ref, err)
	}
	// story.URL is intentionally not forwarded: the Jira adapter derives
	// Story.URL as BaseURL+"/browse/"+key, never from any seeded/"self"
	// value. The mock serves a realistic machine-facing REST self on its
	// own.
	h.server.SeedIssue(key, story.Title, story.Status)
}

// ExpectResolvedURL implements providertest.Harness (I-33): the Jira
// adapter constructs Story.URL from its own configuration as the human
// browse link, BaseURL+"/browse/"+key — here rooted at the harness's own
// mock server base, which is what the adapter under test is configured
// with. A malformed ref cannot occur here: the suite only calls this for
// stories it seeded through SeedStory, which already parsed the ref.
func (h *jiraHarness) ExpectResolvedURL(story provider.Story) string {
	_, key, err := provider.ParseStoryRef(story.Ref)
	if err != nil {
		panic("jiraHarness.ExpectResolvedURL: unparseable ref " + string(story.Ref) + ": " + err.Error())
	}
	return h.server.URL + "/browse/" + key
}

func (h *jiraHarness) SeedNotFound(t *testing.T, ref provider.StoryRef) {
	t.Helper()
	_, key, err := provider.ParseStoryRef(ref)
	if err != nil {
		t.Fatalf("ParseStoryRef(%q): %v", ref, err)
	}
	h.server.SeedNotFound(key)
}

func (h *jiraHarness) PublishedField(t *testing.T, story provider.StoryRef) (provider.Rollup, bool) {
	t.Helper()
	_, key, err := provider.ParseStoryRef(story)
	if err != nil {
		t.Fatalf("ParseStoryRef(%q): %v", story, err)
	}
	raw, ok := h.server.FieldValue(key)
	if !ok {
		return provider.Rollup{}, false
	}
	payload, err := decodeRollupPayload(raw)
	if err != nil {
		t.Fatalf("decoding published field for %q: %v", story, err)
	}
	criteria := make([]provider.CriterionStatus, len(payload.Criteria))
	for i, c := range payload.Criteria {
		criteria[i] = provider.CriterionStatus{ID: c.ID, Status: c.Status}
	}
	return provider.Rollup{
		Story:    story,
		Commit:   payload.Commit,
		Eligible: payload.Eligible,
		Criteria: criteria,
	}, true
}

func (h *jiraHarness) PublishRecordCount(t *testing.T, story provider.StoryRef) int {
	t.Helper()
	_, key, err := provider.ParseStoryRef(story)
	if err != nil {
		t.Fatalf("ParseStoryRef(%q): %v", story, err)
	}
	return h.server.PublishedCommitCount(key)
}

func (h *jiraHarness) CommentCount(t *testing.T, story provider.StoryRef) int {
	t.Helper()
	_, key, err := provider.ParseStoryRef(story)
	if err != nil {
		t.Fatalf("ParseStoryRef(%q): %v", story, err)
	}
	return h.server.CommentCount(key)
}

// TestJiraSatisfiesContract proves the Jira adapter passes the shared
// story-provider contract suite (04 §Testing), unchanged, against an
// httptest-backed harness — the same suite internal/provider/fake's
// contract_test.go runs against the fake.
func TestJiraSatisfiesContract(t *testing.T) {
	providertest.Run(t, newJiraHarness)
}
