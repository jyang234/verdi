package workbench

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/dex"
	"github.com/jyang234/verdi/internal/journey"
	"github.com/jyang234/verdi/internal/readinesspilot"
)

const readinessFixtureHead = "4f1c9d2ab7e0"

// readinessFixture is the canonical mixed-state cockpit fixture on the
// F-01 corrected contract: a target title, the four plain area labels,
// and the current-focus-first attention order.
func readinessFixture() readinesspilot.Snapshot {
	return readinesspilot.Snapshot{
		TargetRef:     "spec/pilot",
		TargetTitle:   "Pilot decline flow",
		TargetClass:   "story",
		Branch:        "design/pilot",
		Head:          readinessFixtureHead,
		RequestDigest: "sha256:" + strings.Repeat("ab", 32),
		Areas: []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateUnproven},
			{ID: readinesspilot.AreaSuccess, Label: "Define success", State: readinesspilot.StateViolated},
			{ID: readinesspilot.AreaContext, Label: "Check constraints", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaReview, Label: "Get approval", State: readinesspilot.StateUnproven},
		},
		CurrentFocus: readinesspilot.AreaShape,
		Attention: []readinesspilot.Concern{
			readinessConcernQuestion(),
			readinessConcernCoverage(),
			readinessConcernAction(),
			readinessConcernSignoff(),
		},
		AllConcerns: []readinesspilot.Concern{
			readinessConcernProblem(),
			readinessConcernQuestion(),
			readinessConcernCoverage(),
			readinessConcernVerdict(),
			readinessConcernAction(),
			readinessConcernSignoff(),
		},
		StaleNotice: "Startup snapshot at " + readinessFixtureHead + "; restart verdi serve after an edit.",
	}
}

func readinessConcernProblem() readinesspilot.Concern {
	return readinesspilot.Concern{
		ID: "shape/problem", Area: readinesspilot.AreaShape,
		State: readinesspilot.StateProven, Blocking: true, Timing: readinesspilot.TimingCurrent,
		Summary: "Problem statement is present", Witnesses: []string{},
		Destination: readinesspilot.Destination{CLI: []string{}},
	}
}

func readinessConcernQuestion() readinesspilot.Concern {
	return readinesspilot.Concern{
		ID: "shape/question/q-alpha", Area: readinesspilot.AreaShape,
		State: readinesspilot.StateUnproven, Blocking: true, Timing: readinesspilot.TimingCurrent,
		Summary:   "Declared open question remains unresolved",
		Witnesses: []string{"q-alpha"},
		Destination: readinesspilot.Destination{
			BoardPath: "/board/spec/pilot", CLI: []string{},
		},
	}
}

func readinessConcernCoverage() readinesspilot.Concern {
	return readinesspilot.Concern{
		ID: "success/blocker/obligation-quality/coverage", Area: readinesspilot.AreaSuccess,
		State: readinesspilot.StateViolated, Blocking: true, Timing: readinesspilot.TimingCurrent,
		WorkClass: journey.ClassMechanical,
		Summary:   "Coverage gate must be green",
		Witnesses: []string{"coverage gate output names the red step", "gate run 41 is red"},
		Destination: readinesspilot.Destination{
			CLI: []string{"verdi", "gate", "run", "--target", "spec/pilot"},
		},
	}
}

func readinessConcernVerdict() readinesspilot.Concern {
	return readinesspilot.Concern{
		ID: "context/verdict", Area: readinesspilot.AreaContext,
		State: readinesspilot.StateProven, Blocking: true, Timing: readinesspilot.TimingCurrent,
		Summary: "Policy-conflict verdict", Witnesses: []string{},
		Destination: readinesspilot.Destination{CLI: []string{}},
	}
}

func readinessConcernAction() readinesspilot.Concern {
	return readinesspilot.Concern{
		ID: "review/action", Area: readinesspilot.AreaReview,
		State: readinesspilot.StateUnproven, Blocking: true, Timing: readinesspilot.TimingCurrent,
		Summary:   "Lifecycle and safe-action posture can advance review",
		Witnesses: []string{"safe review action is unavailable"},
		Destination: readinesspilot.Destination{
			CLI: []string{"verdi", "journey", "--target", "spec/pilot"},
		},
	}
}

func readinessConcernSignoff() readinesspilot.Concern {
	return readinesspilot.Concern{
		ID: "review/blocker/gov-signoff", Area: readinesspilot.AreaReview,
		State: readinesspilot.StateViolated, Blocking: false, Timing: readinesspilot.TimingEventual,
		WorkClass: journey.ClassGovernance,
		Summary:   "Governance signoff will be required",
		Witnesses: []string{"principal profile names a governance signoff"},
		Destination: readinesspilot.Destination{
			CLI: []string{"verdi", "journey", "--target", "spec/pilot"},
		},
	}
}

// readinessAllProvenFixture is the fully proven variant: empty attention,
// no current focus, all four steps complete.
func readinessAllProvenFixture() readinesspilot.Snapshot {
	problem := readinessConcernProblem()
	contributor := readinesspilot.Concern{
		ID: "success/contributor/unit-suite", Area: readinesspilot.AreaSuccess,
		State: readinesspilot.StateProven, Blocking: false, Timing: readinesspilot.TimingCurrent,
		Summary: "Journey evidence contributor unit-suite", Witnesses: []string{"unit-suite run 128 is green"},
		Destination: readinesspilot.Destination{CLI: []string{}},
	}
	verdict := readinessConcernVerdict()
	action := readinesspilot.Concern{
		ID: "review/action", Area: readinesspilot.AreaReview,
		State: readinesspilot.StateProven, Blocking: true, Timing: readinesspilot.TimingCurrent,
		Summary: "Lifecycle and safe-action posture can advance review", Witnesses: []string{"journey-advance"},
		Destination: readinesspilot.Destination{CLI: []string{}},
	}
	return readinesspilot.Snapshot{
		TargetRef:     "spec/pilot",
		TargetTitle:   "Pilot decline flow",
		TargetClass:   "story",
		Branch:        "design/pilot",
		Head:          readinessFixtureHead,
		RequestDigest: "sha256:" + strings.Repeat("ab", 32),
		Areas: []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaSuccess, Label: "Define success", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaContext, Label: "Check constraints", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaReview, Label: "Get approval", State: readinesspilot.StateProven},
		},
		CurrentFocus: "",
		Attention:    []readinesspilot.Concern{},
		AllConcerns:  []readinesspilot.Concern{problem, contributor, verdict, action},
		StaleNotice:  "Startup snapshot at " + readinessFixtureHead + "; restart verdi serve after an edit.",
	}
}

// TestReadinessFixture_ContractValid pins both fixtures to the approved
// corrected contract (TargetTitle, plain labels, current-focus-first
// attention): a fixture Validate() rejects would make every assertion
// below untrustworthy.
func TestReadinessFixture_ContractValid(t *testing.T) {
	if err := readinessFixture().Validate(); err != nil {
		t.Fatalf("mixed fixture violates the readiness contract: %v", err)
	}
	if err := readinessAllProvenFixture().Validate(); err != nil {
		t.Fatalf("all-proven fixture violates the readiness contract: %v", err)
	}
}

func renderReadinessFixture(t *testing.T, snap readinesspilot.Snapshot) string {
	t.Helper()
	out, err := renderReadiness(snap)
	if err != nil {
		t.Fatalf("renderReadiness: %v", err)
	}
	return string(out)
}

// sectionOf extracts the substring of html between the opening marker and
// the next occurrence of until, so assertions can scope themselves to one
// region of the page.
func sectionOf(t *testing.T, html, from, until string) string {
	t.Helper()
	start := strings.Index(html, from)
	if start < 0 {
		t.Fatalf("page does not contain %q", from)
	}
	rest := html[start:]
	if until == "" {
		return rest
	}
	end := strings.Index(rest[len(from):], until)
	if end < 0 {
		t.Fatalf("page does not contain %q after %q", until, from)
	}
	return rest[:len(from)+end]
}

func TestReadinessRender_OrientationLeadsWithTitle(t *testing.T) {
	snap := readinessFixture()
	html := renderReadinessFixture(t, snap)

	// REAL DOM order, not presence alone: title → step → purpose must all
	// precede the target technical metadata.
	title := strings.Index(html, `<h2 class="readiness-title">Pilot decline flow</h2>`)
	step := strings.Index(html, `Step 1 of 4 — Define the work`)
	purpose := strings.Index(html, `This is a startup snapshot of readiness for the current design work.`)
	target := strings.Index(html, `readiness-target-tech`)
	if title < 0 || step < 0 || purpose < 0 || target < 0 {
		t.Fatalf("page is missing orientation pieces (title=%d step=%d purpose=%d target=%d)", title, step, purpose, target)
	}
	if !(title < step && step < purpose && purpose < target) {
		t.Fatalf("orientation order is wrong: title=%d step=%d purpose=%d target-tech=%d", title, step, purpose, target)
	}

	// The old leading metadata card is gone from this page entirely.
	if strings.Contains(html, `metadata-card`) {
		t.Fatal("page still renders the shell's leading metadata card")
	}

	// The target technical details retain every exact fact, unnormalized.
	tech := sectionOf(t, html, `readiness-target-tech`, `</details>`)
	for _, want := range []string{
		`<summary>Target technical details</summary>`,
		`<dt>Target</dt><dd><code>spec/pilot</code></dd>`,
		`<dt>Class</dt><dd><code>` + snap.TargetClass + `</code></dd>`,
		`<dt>Branch</dt><dd><code>design/pilot</code></dd>`,
		`<dt>Head</dt><dd><code>` + readinessFixtureHead + `</code></dd>`,
		`<dt>Request digest</dt><dd><code>` + snap.RequestDigest + `</code></dd>`,
	} {
		if !strings.Contains(tech, want) {
			t.Fatalf("target technical details are missing %q:\n%s", want, tech)
		}
	}

	// The title itself never falls back to the technical ref, and the
	// stale notice stays inside the orientation block.
	orient := sectionOf(t, html, `<section class="readiness-orient"`, `</section>`)
	if strings.Contains(sectionOf(t, orient, `<h2 class="readiness-title">`, `</h2>`), "spec/pilot") {
		t.Fatalf("orientation title uses the technical ref:\n%s", orient)
	}
	if !strings.Contains(orient, `class="readiness-stale"`) {
		t.Fatalf("stale notice left the orientation block:\n%s", orient)
	}
}

func TestReadinessRender_RailOrderPlainStatesAndFocus(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	rail := sectionOf(t, html, `<nav class="readiness-rail"`, `</nav>`)

	type station struct{ area, label, plain, formal string }
	want := []station{
		{"shape-proposal", "Define the work", "Not enough evidence yet", "unproven"},
		{"show-success", "Define success", "Needs attention", "violated-with-witness"},
		{"check-context", "Check constraints", "Ready", "proven"},
		{"request-review", "Get approval", "Not enough evidence yet", "unproven"},
	}
	prev := -1
	for i, st := range want {
		idx := strings.Index(rail, `data-area-id="`+st.area+`"`)
		if idx < 0 {
			t.Fatalf("rail is missing station %q", st.area)
		}
		if idx < prev {
			t.Fatalf("rail station %q is out of the fixed order", st.area)
		}
		prev = idx
		block := sectionOf(t, rail, `data-area-id="`+st.area+`"`, `</li>`)
		if !strings.Contains(block, `data-state="`+st.formal+`"`) {
			t.Fatalf("station %q lost its exact formal state:\n%s", st.area, block)
		}
		if !strings.Contains(block, st.label) {
			t.Fatalf("station %q is missing plain label %q", st.area, st.label)
		}
		if !strings.Contains(block, `>`+st.plain+`<`) {
			t.Fatalf("station %q is missing plain state %q:\n%s", st.area, st.plain, block)
		}
		if !strings.Contains(block, `href="#area-`+st.area+`"`) {
			t.Fatalf("station %q lost its fragment anchor", st.area)
		}
		wantNum := []string{"1", "2", "3", "4"}[i]
		if !strings.Contains(block, `<span class="readiness-station-num">`+wantNum+`</span>`) {
			t.Fatalf("station %q is missing step number %s:\n%s", st.area, wantNum, block)
		}
	}
	if got := strings.Count(rail, `aria-current="step"`); got != 1 {
		t.Fatalf("rail carries %d aria-current markers, want exactly 1", got)
	}
	focus := sectionOf(t, rail, `data-area-id="shape-proposal"`, `</li>`)
	if !strings.Contains(focus, `aria-current="step"`) {
		t.Fatal("current-focus marker is not on the snapshot's focus area")
	}
}

func TestReadinessRender_AllCompletePostureIsHonest(t *testing.T) {
	html := renderReadinessFixture(t, readinessAllProvenFixture())
	if !strings.Contains(html, "All four steps are complete.") {
		t.Fatal("all-proven snapshot does not state plainly that the steps are complete")
	}
	if strings.Contains(html, "Step 1 of 4") || strings.Contains(html, `aria-current`) {
		t.Fatal("all-proven snapshot invents a current focus")
	}
	if strings.Contains(html, "Known problems in later steps") {
		t.Fatal("all-proven snapshot renders a downstream count with no focus")
	}
	queue := sectionOf(t, html, `<section class="readiness-queue"`, `</section>`)
	if strings.Contains(queue, "data-concern-id") {
		t.Fatalf("all-proven focus list still lists concerns:\n%s", queue)
	}
	if !strings.Contains(queue, "Nothing needs attention: every check in this snapshot is proven.") {
		t.Fatalf("empty focus list does not state its honest reason:\n%s", queue)
	}
	// Every proven fact stays reachable in completed checks.
	completed := sectionOf(t, html, `<section class="readiness-completed"`, "</main>")
	if got := strings.Count(completed, `data-concern-id="`); got != 4 {
		t.Fatalf("completed checks list %d proven concerns, want 4", got)
	}
}

func TestReadinessRender_FocusListTopThreeAndDisclosure(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	queue := sectionOf(t, html, `<section class="readiness-queue"`, `</section>`)

	// Exactly the first three priorities are outside the disclosure, in
	// the snapshot's exact order, ranked 1..3.
	top := sectionOf(t, queue, `<ol class="readiness-queue-list">`, `</ol>`)
	wantTop := []string{
		"shape/question/q-alpha",
		"success/blocker/obligation-quality/coverage",
		"review/action",
	}
	prev := -1
	for i, id := range wantTop {
		idx := strings.Index(top, `data-concern-id="`+id+`"`)
		if idx < 0 {
			t.Fatalf("top-three list is missing %q:\n%s", id, top)
		}
		if idx < prev {
			t.Fatalf("top-three concern %q is out of snapshot order", id)
		}
		prev = idx
		rank := []string{"1", "2", "3"}[i]
		if !strings.Contains(top, `<span class="readiness-rank">`+rank+`</span>`) {
			t.Fatalf("top-three list is missing rank %s", rank)
		}
	}
	if got := strings.Count(top, `data-concern-id="`); got != 3 {
		t.Fatalf("top list shows %d priorities, want exactly 3", got)
	}

	// The remainder sits in an inline disclosure with the exact
	// remaining-count label and the exact open label.
	more := sectionOf(t, queue, `<details class="readiness-more">`, `</details>`)
	if !strings.Contains(more, `<span class="readiness-more-closed">1 more item</span>`) {
		t.Fatalf("disclosure control is missing the exact remaining count:\n%s", more)
	}
	if !strings.Contains(more, `<span class="readiness-more-open">Show fewer</span>`) {
		t.Fatalf("disclosure control is missing the exact open label:\n%s", more)
	}
	if !strings.Contains(more, `data-concern-id="review/blocker/gov-signoff"`) {
		t.Fatalf("disclosure does not carry the remaining concern:\n%s", more)
	}
	if !strings.Contains(more, `<span class="readiness-rank">4</span>`) {
		t.Fatalf("remainder does not continue the original ranking:\n%s", more)
	}
}

func TestReadinessRender_FocusListFewerThanFour(t *testing.T) {
	// A CONFORMING three-priority snapshot, not a render-only slice: the
	// governance signoff is proven here (no destination), so exactly the
	// three remaining unresolved concerns form the attention list and the
	// area states, destinations, and validation all agree.
	snap := readinessFixture()
	signoff := readinessConcernSignoff()
	signoff.State = readinesspilot.StateProven
	signoff.Destination = readinesspilot.Destination{CLI: []string{}}
	snap.AllConcerns[5] = signoff
	snap.Attention = []readinesspilot.Concern{
		readinessConcernQuestion(),
		readinessConcernCoverage(),
		readinessConcernAction(),
	}
	if err := snap.Validate(); err != nil {
		t.Fatalf("fewer-than-four fixture violates the readiness contract: %v", err)
	}
	html := renderReadinessFixture(t, snap)
	queue := sectionOf(t, html, `<section class="readiness-queue"`, `</section>`)
	if strings.Contains(queue, "readiness-more") || strings.Contains(queue, "more item") {
		t.Fatalf("three priorities still render a misleading remaining-count control:\n%s", queue)
	}
	if got := strings.Count(queue, `data-concern-id="`); got != 3 {
		t.Fatalf("focus list shows %d priorities, want all 3", got)
	}
}

func TestReadinessRender_DownstreamViolatedCount(t *testing.T) {
	// Fixture: focus is the first area; both violated concerns (success,
	// review) sit in later areas → exactly 2.
	html := renderReadinessFixture(t, readinessFixture())
	if !strings.Contains(html, `Known problems in later steps: 2`) {
		t.Fatal("page does not disclose the exact downstream violated count")
	}

	// The counting rule itself, on synthetic snapshots distinguishing
	// current, earlier, downstream, violated, and unproven rows.
	areas := []readinesspilot.Area{
		{ID: readinesspilot.AreaShape}, {ID: readinesspilot.AreaSuccess},
		{ID: readinesspilot.AreaContext}, {ID: readinesspilot.AreaReview},
	}
	concern := func(area readinesspilot.AreaID, state readinesspilot.State) readinesspilot.Concern {
		return readinesspilot.Concern{Area: area, State: state}
	}
	tests := []struct {
		name  string
		focus readinesspilot.AreaID
		rows  []readinesspilot.Concern
		want  int
	}{
		{"current-area violation is not downstream", readinesspilot.AreaContext,
			[]readinesspilot.Concern{concern(readinesspilot.AreaContext, readinesspilot.StateViolated)}, 0},
		{"earlier-area violation is not downstream", readinesspilot.AreaContext,
			[]readinesspilot.Concern{concern(readinesspilot.AreaSuccess, readinesspilot.StateViolated)}, 0},
		{"later violation counts", readinesspilot.AreaContext,
			[]readinesspilot.Concern{concern(readinesspilot.AreaReview, readinesspilot.StateViolated)}, 1},
		{"later unproven does not count", readinesspilot.AreaContext,
			[]readinesspilot.Concern{concern(readinesspilot.AreaReview, readinesspilot.StateUnproven)}, 0},
		{"later proven does not count", readinesspilot.AreaContext,
			[]readinesspilot.Concern{concern(readinesspilot.AreaReview, readinesspilot.StateProven)}, 0},
		{"no focus means zero", "",
			[]readinesspilot.Concern{concern(readinesspilot.AreaReview, readinesspilot.StateViolated)}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := readinesspilot.Snapshot{Areas: areas, CurrentFocus: tt.focus, AllConcerns: tt.rows}
			if got := readinessDownstreamViolated(snap); got != tt.want {
				t.Fatalf("readinessDownstreamViolated = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestReadinessRender_PlainStateLabelsWithFormalDetails(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	// Every state chip pairs the formal modifier class with the plain
	// label text — the mapping is exact and total.
	for formal, plain := range map[string]string{
		"proven":                "Ready",
		"violated-with-witness": "Needs attention",
		"unproven":              "Not enough evidence yet",
	} {
		chip := `<span class="readiness-state readiness-state--` + formal + `">` + plain + `</span>`
		if !strings.Contains(html, chip) {
			t.Fatalf("page is missing exact chip %q", chip)
		}
		if strings.Count(html, `readiness-state--`+formal+`"`) != strings.Count(html, chip) {
			t.Fatalf("some %q chip does not carry plain label %q", formal, plain)
		}
	}
	// The formal words stay reachable in technical details.
	for _, formal := range []string{"proven", "violated-with-witness", "unproven"} {
		if !strings.Contains(html, `<dd><code>`+formal+`</code></dd>`) {
			t.Fatalf("technical details never state formal state %q", formal)
		}
	}
}

func TestReadinessRender_TechnicalDetailsComplete(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	card := sectionOf(t, html, `data-concern-id="success/blocker/obligation-quality/coverage"`, `</article>`)
	tech := sectionOf(t, card, `<details class="readiness-tech">`, `</details>`)

	for _, want := range []string{
		`<summary>Technical details</summary>`,
		`<dt>State</dt><dd><code>violated-with-witness</code></dd>`,
		`<dt>Concern</dt><dd><code>success/blocker/obligation-quality/coverage</code></dd>`,
		`<dt>Area</dt><dd><code>show-success</code></dd>`,
		`<dt>Blocking</dt><dd><code>true</code></dd>`,
		`<dt>Timing</dt><dd><code>current</code></dd>`,
		`<dt>Work class</dt><dd><code>mechanical</code></dd>`,
		`coverage gate output names the red step`,
		`gate run 41 is red`,
		`<dt>Destination</dt>`,
	} {
		if !strings.Contains(tech, want) {
			t.Fatalf("technical details are missing %q:\n%s", want, tech)
		}
	}
	// The destination is exact token data, never a joined shell string.
	if !strings.Contains(tech, `<code>verdi</code>`) || strings.Contains(html, "verdi gate run --target spec/pilot") {
		t.Fatalf("technical destination is not the exact token vector:\n%s", tech)
	}

	// A concern without a source work class renders no work-class row —
	// omitted, never guessed.
	question := sectionOf(t, html, `data-concern-id="shape/question/q-alpha"`, `</article>`)
	if strings.Contains(question, "Work class") {
		t.Fatalf("concern without a source work class renders one:\n%s", question)
	}
	// A proven concern's details carry its facts but no destination.
	completed := sectionOf(t, html, `<section class="readiness-completed"`, "</main>")
	proven := sectionOf(t, completed, `data-concern-id="shape/problem"`, `</article>`)
	if !strings.Contains(proven, `<dd><code>proven</code></dd>`) {
		t.Fatalf("proven concern's details lost its formal state:\n%s", proven)
	}
	if strings.Contains(proven, "Destination") || strings.Contains(proven, "readiness-board-link") || strings.Contains(proven, "readiness-cli-token") {
		t.Fatalf("proven concern renders a destination:\n%s", proven)
	}
}

func TestReadinessRender_SummariesArePrimaryCopy(t *testing.T) {
	snap := readinessFixture()
	html := renderReadinessFixture(t, snap)
	for _, concern := range snap.AllConcerns {
		if !strings.Contains(html, `<p class="readiness-summary">`+concern.Summary+`</p>`) {
			t.Fatalf("concern %q does not use its source summary as primary copy", concern.ID)
		}
	}
}

func TestReadinessRender_LosslessDisjointInventoryAndAnchors(t *testing.T) {
	snap := readinessFixture()
	html := renderReadinessFixture(t, snap)

	// Each concern appears exactly once across the two inventories.
	if got := strings.Count(html, `data-concern-id="`); got != len(snap.AllConcerns) {
		t.Fatalf("page lists %d concern rows, want exactly %d (no omission, no duplication)", got, len(snap.AllConcerns))
	}
	queue := sectionOf(t, html, `<section class="readiness-queue"`, `</section>`)
	for _, concern := range snap.Attention {
		if !strings.Contains(queue, `data-concern-id="`+concern.ID+`"`) {
			t.Fatalf("unresolved concern %q is not reachable in the focus list", concern.ID)
		}
	}
	completed := sectionOf(t, html, `<section class="readiness-completed"`, "</main>")
	prev := -1
	for _, concern := range snap.AllConcerns {
		if concern.State != readinesspilot.StateProven {
			continue
		}
		idx := strings.Index(completed, `data-concern-id="`+concern.ID+`"`)
		if idx < 0 {
			t.Fatalf("proven concern %q is not reachable in completed checks", concern.ID)
		}
		if idx < prev {
			t.Fatalf("completed check %q is out of the snapshot's order", concern.ID)
		}
		prev = idx
	}

	// Every area keeps exactly one fragment anchor, on its first row.
	for _, area := range []string{"shape-proposal", "show-success", "check-context", "request-review"} {
		if got := strings.Count(html, `id="area-`+area+`"`); got != 1 {
			t.Fatalf("area %q has %d fragment anchors, want exactly 1", area, got)
		}
	}
}

func TestReadinessRender_DestinationActionsUsable(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	question := sectionOf(t, html, `data-concern-id="shape/question/q-alpha"`, `</article>`)
	link := sectionOf(t, question, `class="readiness-board-link"`, `</a>`)
	for _, want := range []string{`href="/board/spec/pilot"`, `target="_blank"`, `rel="noopener"`} {
		if !strings.Contains(link, want) {
			t.Fatalf("board link is missing %q:\n%s", want, link)
		}
	}
	if strings.Contains(question, "readiness-cli-token") {
		t.Fatalf("board-destination concern also renders CLI tokens:\n%s", question)
	}

	coverage := sectionOf(t, html, `data-concern-id="success/blocker/obligation-quality/coverage"`, `</article>`)
	if strings.Contains(coverage, "readiness-board-link") {
		t.Fatalf("CLI-destination concern also renders a board link:\n%s", coverage)
	}
	tokens := []string{"verdi", "gate", "run", "--target", "spec/pilot"}
	action := sectionOf(t, coverage, `class="readiness-dest readiness-cli"`, `</p>`)
	prev := -1
	for _, token := range tokens {
		marker := `<code class="readiness-cli-token">` + token + `</code>`
		idx := strings.Index(action, marker)
		if idx < 0 {
			t.Fatalf("CLI action is missing token element %q:\n%s", token, action)
		}
		if idx < prev {
			t.Fatalf("CLI token %q is out of vector order", token)
		}
		prev = idx
	}
	if !strings.Contains(action, `tabindex="0"`) {
		t.Fatalf("CLI fallback is not keyboard-reachable:\n%s", action)
	}
}

func TestReadinessRender_StaleNoticeNamesHead(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	notice := sectionOf(t, html, `class="readiness-stale"`, `</aside>`)
	if !strings.Contains(notice, readinessFixtureHead) {
		t.Fatalf("stale notice does not name the exact HEAD:\n%s", notice)
	}
	if !strings.Contains(notice, "restart") || !strings.Contains(notice, "verdi serve") {
		t.Fatalf("stale notice does not tell the author to restart verdi serve:\n%s", notice)
	}
	if !strings.Contains(notice, `tabindex="0"`) {
		t.Fatalf("stale notice is not keyboard-reachable:\n%s", notice)
	}
}

func TestReadinessRender_NoMutationSurface(t *testing.T) {
	for _, snap := range []readinesspilot.Snapshot{readinessFixture(), readinessAllProvenFixture()} {
		html := renderReadinessFixture(t, snap)
		for _, forbidden := range []string{
			"<form", "<button", "method=", "fetch(", "XMLHttpRequest",
			"WebSocket", "EventSource", "http-equiv=\"refresh\"", "contenteditable",
		} {
			if strings.Contains(html, forbidden) {
				t.Fatalf("cockpit page carries mutation/network surface %q", forbidden)
			}
		}
	}
}

func TestReadinessRender_EscapesUntrustedText(t *testing.T) {
	snap := readinessFixture()
	snap.TargetTitle = `<script>alert(0)</script>`
	snap.AllConcerns[2].Summary = `<script>alert(1)</script>`
	snap.Attention[1].Summary = `<script>alert(1)</script>`
	snap.AllConcerns[2].Witnesses = []string{`"><img src=x onerror=alert(2)>`}
	snap.Attention[1].Witnesses = []string{`"><img src=x onerror=alert(2)>`}
	html := renderReadinessFixture(t, snap)
	if strings.Contains(html, "<script>alert(0)") {
		t.Fatal("target title is not HTML-escaped")
	}
	if strings.Contains(html, "<script>alert(1)") {
		t.Fatal("summary text is not HTML-escaped")
	}
	if strings.Contains(html, "<img src=x") {
		t.Fatal("witness text is not HTML-escaped")
	}
}

func TestReadinessRender_KeyboardLandmarksAndScript(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	for _, want := range []string{
		`<nav class="readiness-rail" aria-label="Readiness rail">`,
		`aria-label="Focus next"`,
		`aria-label="Completed checks"`,
		`href="#area-shape-proposal"`,
		`id="area-shape-proposal"`,
		`<details class="readiness-more">`,
		`<summary class="readiness-more-summary">`,
		`<details class="readiness-tech">`,
		`<details class="readiness-tech readiness-target-tech">`,
		`<p class="readiness-dest readiness-cli" data-readiness-cli="1" tabindex="0" aria-label="CLI fallback">`,
		`<script src="/assets/readiness.js" defer></script>`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("page is missing keyboard/instrumentation hook %q", want)
		}
	}
}

func TestReadinessRoute_GetHappy(t *testing.T) {
	snap := readinessFixture()
	h := NewHandlerWith(t.TempDir(), Deps{Readiness: &snap})
	req := httptest.NewRequest(http.MethodGet, "/readiness", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content type = %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), "readiness-rail") {
		t.Fatalf("page body does not carry the rail: %s", rec.Body.String())
	}
}

func TestReadinessRoute_MissingSnapshot503(t *testing.T) {
	h := NewHandler(t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/readiness", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when no snapshot was injected", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "snapshot") || !strings.Contains(body, "verdi serve") {
		t.Fatalf("503 page does not honestly disclose the missing snapshot: %s", body)
	}
}

func TestReadinessRoute_WrongMethod405(t *testing.T) {
	snap := readinessFixture()
	h := NewHandlerWith(t.TempDir(), Deps{Readiness: &snap})
	for _, path := range []string{"/readiness", "/assets/readiness.js"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			req := httptest.NewRequest(method, path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s %s status = %d, want 405", method, path, rec.Code)
			}
		}
	}
}

func TestReadinessAsset_JSServed(t *testing.T) {
	h := NewHandler(t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/assets/readiness.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/javascript") {
		t.Fatalf("content type = %q, want application/javascript", ct)
	}
}

func TestReadinessAsset_JSVocabularyClosed(t *testing.T) {
	h := NewHandler(t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/assets/readiness.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	js := rec.Body.String()

	for _, want := range []string{
		`"readiness-opened"`,
		`"area-inspected"`,
		`"concern-inspected"`,
		`"board-link-followed"`,
		`"cli-fallback-copied"`,
		`"stale-notice-inspected"`,
		`"verdi:readiness-pilot"`,
		"__verdiReadinessPilotEvents",
		"200",
		"sequence",
		"area_id",
		"concern_id",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("readiness.js is missing %q", want)
		}
	}
}

func TestReadinessAsset_JSNoNetworkNoPersistence(t *testing.T) {
	h := NewHandler(t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/assets/readiness.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	js := rec.Body.String()

	for _, forbidden := range []string{
		"fetch(", "XMLHttpRequest", "WebSocket", "EventSource", "sendBeacon",
		"localStorage", "sessionStorage", "indexedDB", "document.cookie",
		"location.reload", "setInterval", "setTimeout", "innerHTML",
	} {
		if strings.Contains(js, forbidden) {
			t.Fatalf("readiness.js carries forbidden capability %q", forbidden)
		}
	}
}

func TestReadinessAsset_JSMiddleClickInstrumented(t *testing.T) {
	h := NewHandler(t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/assets/readiness.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	js := rec.Body.String()

	for _, want := range []string{`"auxclick"`, "button !== 1", "recordBoardLink"} {
		if !strings.Contains(js, want) {
			t.Fatalf("readiness.js is missing middle-button instrumentation %q", want)
		}
	}
	if got := strings.Count(js, `record("board-link-followed"`); got != 1 {
		t.Fatalf("board-link-followed is recorded from %d sites, want exactly 1 shared recorder", got)
	}
}

func TestReadinessStyle_NarrowAnchorsAndWrapping(t *testing.T) {
	css, err := dex.StyleCSS()
	if err != nil {
		t.Fatalf("dex.StyleCSS: %v", err)
	}
	// Scope to the cockpit's own block: other components of the shared
	// stylesheet legitimately use truncation.
	s := sectionOf(t, string(css), "the readiness pilot cockpit", "Syntax-highlighting palettes")
	if !strings.Contains(s, "scroll-margin-top") {
		t.Fatal("cockpit block has no scroll offset for its fragment targets")
	}
	if !strings.Contains(s, "overflow-wrap") {
		t.Fatal("cockpit block has no wrapping rule for long values")
	}
	for _, forbidden := range []string{"text-overflow", "overflow: hidden", "overflow-x: hidden"} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("cockpit block truncates content (%q) instead of wrapping it", forbidden)
		}
	}
}

func TestReadinessStyle_CockpitRules(t *testing.T) {
	css, err := dex.StyleCSS()
	if err != nil {
		t.Fatalf("dex.StyleCSS: %v", err)
	}
	s := string(css)
	for _, want := range []string{
		".readiness-rail", ".readiness-queue", ".readiness-completed",
		".readiness-orient", ".readiness-more", ".readiness-tech",
		".readiness-stale", ".readiness-cli-token",
		"prefers-reduced-motion",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("stylesheet is missing cockpit rule %q", want)
		}
	}
}
