package workbench

import (
	stdhtml "html"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/specstate"
)

// acceptanceObligation is the formal obligation the human-review label
// keeps secondary: AC-6/DC-15's profile-required review.
const acceptanceObligation = "AC-6/DC-15: the profile-required review of the exact proposed head authorizes merge"

// TestReviewAcceptanceFor (SI-368 (3), (32) B3): the retired wall shell's
// review/acceptance row moves, claim for claim, into the Review tab. An
// accepted revision reads proven, the owner's merge having made it
// reachable from the default branch, witnessed by its Git-derived state.
// An open review is unproven: the wall mirrors the proposal's merge
// request, the owner's merge of it is the single acceptance decision,
// and the formal obligation is witnessed beside the open request. Any
// other wall is unproven, human review not having accepted the proposal:
// an authoring wall names its branch and the pull request it proposes
// from it; a wall that takes no edit proposes nothing from itself (SI-368
// (28)(b)), so it names the proposal's own design branch instead; both
// keep the owner's merge as the single acceptance decision, the formal
// obligation, and the state it was derived from. Every row is human
// review.
func TestReviewAcceptanceFor(t *testing.T) {
	accepted := string(specstate.AcceptedPendingBuild)
	for _, tc := range []struct {
		name       string
		state      string
		mode       boardModeKind
		branch     string
		want       readinesspilot.State
		summary    string
		guidance   string
		witnesses  []string
		noGuidance bool
	}{
		{
			name: "an accepted revision", state: accepted, mode: modeReadOnly, branch: "design/x",
			want:      readinesspilot.StateProven,
			summary:   "This revision is accepted: the owner's merge made it reachable from the default branch.",
			witnesses: []string{"Git-derived state " + accepted}, noGuidance: true,
		},
		{
			name: "an open review", state: "proposed", mode: modeReview, branch: "design/x",
			want:      readinesspilot.StateUnproven,
			summary:   "Human review is open: this wall mirrors the proposal's merge request.",
			guidance:  "The owner's merge of the open merge request is the single acceptance decision — no second ceremony.",
			witnesses: []string{"an open merge request mirrors this spec", acceptanceObligation},
		},
		{
			name: "an authoring wall", state: "proposed", mode: modeAuthoring, branch: "design/x",
			want:      readinesspilot.StateUnproven,
			summary:   "Human review has not accepted this proposal yet.",
			guidance:  "Derive the semantic review packet (below), open a pull request from design/x, and request the owner's review — the owner's merge is the single acceptance decision.",
			witnesses: []string{"Git-derived state proposed", acceptanceObligation + "; no separate acceptance command exists"},
		},
		{
			name: "a read-only wall not yet accepted", state: "proposed", mode: modeReadOnly, branch: "design/y",
			want:      readinesspilot.StateUnproven,
			summary:   "Human review has not accepted this proposal yet.",
			guidance:  "Request the owner's review of the proposal from its own design branch — the owner's merge is the single acceptance decision.",
			witnesses: []string{"Git-derived state proposed", acceptanceObligation + "; no separate acceptance command exists"},
		},
		{
			name: "a sealed render's legacy status", state: "draft", mode: modeReadOnly, branch: "design/remote-only",
			want:      readinesspilot.StateUnproven,
			summary:   "Human review has not accepted this proposal yet.",
			guidance:  "Request the owner's review of the proposal from its own design branch — the owner's merge is the single acceptance decision.",
			witnesses: []string{"Git-derived state draft", acceptanceObligation + "; no separate acceptance command exists"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := reviewAcceptanceFor(tc.state, tc.mode, tc.branch)
			if got.State != tc.want || got.Summary != tc.summary || !reflect.DeepEqual(got.Witnesses, tc.witnesses) {
				t.Fatalf("reviewAcceptanceFor = %+v\nwant state %s, summary %q, witnesses %q", got, tc.want, tc.summary, tc.witnesses)
			}
			if tc.noGuidance {
				if got.Guidance != "" {
					t.Fatalf("a proven row carries guidance %q", got.Guidance)
				}
				return
			}
			if got.Guidance != tc.guidance {
				t.Fatalf("guidance =\n  %q\nwant\n  %q", got.Guidance, tc.guidance)
			}
			if !strings.Contains(got.Guidance, "is the single acceptance decision") || !strings.Contains(got.Guidance, "owner's merge") {
				t.Fatalf("guidance %q does not keep the owner's merge as the single acceptance decision", got.Guidance)
			}
		})
	}
}

// TestWriteReviewAcceptance (SI-368 (32) B3): the Review tab's acceptance
// section is labelled human review in plain words, with the formal
// concern secondary; its primary line is the guidance (the fact of a
// proven row), the fact follows, and the formal obligation and each
// witness are rows — no control, no JSON.
func TestWriteReviewAcceptance(t *testing.T) {
	for _, tc := range []struct {
		name string
		a    reviewAcceptance
	}{
		{"unproven", reviewAcceptanceFor("proposed", modeAuthoring, "design/x")},
		{"proven", reviewAcceptanceFor(string(specstate.AcceptedPendingBuild), modeReadOnly, "design/x")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeReviewAcceptance(&b, tc.a)
			html := b.String()
			if !strings.HasPrefix(html, `<section class="record-section" data-testid="record-review-acceptance" data-concern-id="review/acceptance" data-state="`+string(tc.a.State)+`"`) {
				t.Fatalf("the section does not open as the acceptance section in state %s:\n%s", tc.a.State, html)
			}
			if label, n := testIDElementText(html, "record-human-review"); n != 1 || label != "Human review · review/acceptance" {
				t.Errorf("human-review label = %q (%d), want the plain label with the formal concern secondary", label, n)
			}
			primary, fact := tc.a.Guidance, tc.a.Summary
			if primary == "" {
				primary, fact = tc.a.Summary, ""
			}
			if got, n := testIDElementText(html, "record-acceptance-primary"); n != 1 || got != primary {
				t.Errorf("primary line = %q (%d), want %q", got, n, primary)
			}
			if got, n := testIDElementText(html, "record-acceptance-fact"); fact != "" && (n != 1 || got != fact) {
				t.Errorf("fact = %q (%d), want %q", got, n, fact)
			} else if fact == "" && n != 0 {
				t.Errorf("a proven row repeats its fact")
			}
			if strings.Index(html, `record-acceptance-primary`) > strings.Index(html, `record-acceptance-formal`) {
				t.Error("the formal facts precede the primary line")
			}
			formal := html[strings.Index(html, `data-testid="record-acceptance-formal"`):]
			if !strings.Contains(formal, `<dt class="record-key">state</dt><dd class="record-value record-mono"><span class="record-text">`+string(tc.a.State)+`</span></dd>`) {
				t.Errorf("the formal rows lack the state %s:\n%s", tc.a.State, formal)
			}
			for _, w := range tc.a.Witnesses {
				if !strings.Contains(formal, `<dt class="record-key">witness</dt><dd class="record-value record-mono"><span class="record-text">`+stdhtml.EscapeString(w)+`</span></dd>`) {
					t.Errorf("the formal rows lack the witness %q:\n%s", w, formal)
				}
			}
			for _, never := range []string{"<button", "<form", "<a ", "{"} {
				if strings.Contains(html, never) {
					t.Errorf("the section carries %q", never)
				}
			}
		})
	}
}

// TestRecordDrawer_ReviewTabCarriesAcceptance (SI-368 (32) B3): the
// drawer's Review panel carries the acceptance section between its intro
// and the body the packet fills, from the wall's own state, mode and
// branch — on the authoring wall the pull request it proposes from its
// branch, on the sealed render the proposal's own design branch.
func TestRecordDrawer_ReviewTabCarriesAcceptance(t *testing.T) {
	authoring := badgeRenderProjection(modeAuthoring)
	view := testASDView()
	view.StateFormal = "proposed"
	view.Branch = "design/x"
	drawer := renderRecordDrawer(authoring, view)
	panel := drawer[strings.Index(drawer, `data-testid="record-panel-review"`):]
	panel = panel[:strings.Index(panel, `data-testid="record-panel-context"`)]
	intro, section, body := strings.Index(panel, `record-drawer-intro`), strings.Index(panel, `data-testid="record-review-acceptance"`), strings.Index(panel, `data-record-body="review"`)
	if intro < 0 || section < 0 || body < 0 || intro > section || section > body {
		t.Fatalf("the Review panel does not carry the acceptance section between its intro (%d) and its body (%d): %d\n%s", intro, body, section, panel)
	}
	if !strings.Contains(panel, "open a pull request from design/x") {
		t.Errorf("the authoring wall's acceptance does not name its branch:\n%s", panel)
	}
	if strings.Count(drawer, `data-testid="record-review-acceptance"`) != 1 {
		t.Error("the drawer carries the acceptance section other than once")
	}

	sealed := sealedASDView("design/remote-only", "origin/design/remote-only", &BoardProjection{Status: "draft", Mode: modeReadOnly})
	got := renderRecordDrawer(badgeRenderProjection(modeReadOnly), sealed)
	if !strings.Contains(got, "Request the owner&#39;s review of the proposal from its own design branch") || !strings.Contains(got, "Git-derived state draft") {
		t.Errorf("the sealed render's acceptance is not its own state's:\n%s", got)
	}
}
