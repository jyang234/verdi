package workbench

import (
	"context"
	"errors"
	stdhtml "html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/dex"
	"github.com/jyang234/verdi/internal/journey"
	"github.com/jyang234/verdi/internal/readinesspilot"
)

const readinessFixtureHead = "4f1c9d2ab7e0"

// readinessFixture is the canonical mixed-state fixture on the
// readiness-page-v2 contract: a target title, class, branch and wall
// address, the four plain area labels, and the current-focus-first
// attention order. Define the work is the current step (an unproven open
// question); Define success is violated (a current coverage blocker);
// Check constraints is proven; Get approval is unproven (a safe-action
// row) and carries a violated eventual governance blocker, human-review
// work by SI-338 (3).
func readinessFixture() readinesspilot.Snapshot {
	return readinesspilot.Snapshot{
		TargetRef:     "spec/pilot",
		TargetTitle:   "Pilot decline flow",
		TargetClass:   "story",
		Branch:        "design/pilot",
		Head:          readinessFixtureHead,
		BoardPath:     BranchBoardHref("design/pilot", "pilot"),
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
		StaleNotice: "Derived at HEAD " + readinessFixtureHead + " for this request.",
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
		Guidance:  readinesspilot.Guidance(readinesspilot.GuidanceQuestion, readinesspilot.GuidanceFacts{Object: "q-alpha"}),
		Object:    "q-alpha",
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
		Guidance:  "Run the coverage gate again and clear the red step it names",
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
		Guidance:  "Establish the facts the witnesses name, so verdi journey can offer a safe review action.",
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
		Guidance:  "A governance principal signs off before this design can be approved",
		Witnesses: []string{"principal profile names a governance signoff"},
		Destination: readinesspilot.Destination{
			CLI: []string{"verdi", "journey", "--target", "spec/pilot"},
		},
	}
}

// readinessConcernRole is a principal-role requirement: human review by
// SI-338 (3), with no work class (it is not a journey blocker).
func readinessConcernRole() readinesspilot.Concern {
	return readinesspilot.Concern{
		ID: "review/role/close/attestation/countersign", Area: readinesspilot.AreaReview,
		State: readinesspilot.StateUnproven, Blocking: false, Timing: readinesspilot.TimingCurrent,
		Summary:   "A principal must provide attestation/countersign before close",
		Guidance:  "Before close, a principal entitled to give attestation/countersign must provide it; verdi journey shows the requirement.",
		Witnesses: []string{"no principal other than the author can countersign close"},
		Destination: readinesspilot.Destination{
			CLI: []string{"verdi", "journey", "--target", "spec/pilot"},
		},
	}
}

// readinessWithRoleFixture adds the role row to the mixed fixture: the
// attention order keeps the focus area first, then blocking before
// non-blocking, current before eventual.
func readinessWithRoleFixture() readinesspilot.Snapshot {
	snap := readinessFixture()
	snap.AllConcerns = append(snap.AllConcerns, readinessConcernRole())
	snap.Attention = []readinesspilot.Concern{
		readinessConcernQuestion(),
		readinessConcernCoverage(),
		readinessConcernAction(),
		readinessConcernRole(),
		readinessConcernSignoff(),
	}
	return snap
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
		BoardPath:     BranchBoardHref("design/pilot", "pilot"),
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
		StaleNotice:  "Derived at HEAD " + readinessFixtureHead + " for this request.",
	}
}

// readinessLastStepFixture is a conforming snapshot whose every unresolved
// concern sits in the current step (Get approval): the earlier steps are
// proven, so nothing waits and the later-steps disclosure has nothing to
// hold.
func readinessLastStepFixture() readinesspilot.Snapshot {
	snap := readinessFixture()
	question := readinessConcernQuestion()
	question.State = readinesspilot.StateProven
	question.Guidance = ""
	question.Destination = readinesspilot.Destination{CLI: []string{}}
	coverage := readinessConcernCoverage()
	coverage.State = readinesspilot.StateProven
	coverage.Guidance = ""
	coverage.Destination = readinesspilot.Destination{CLI: []string{}}
	snap.Areas[0].State = readinesspilot.StateProven
	snap.Areas[1].State = readinesspilot.StateProven
	snap.CurrentFocus = readinesspilot.AreaReview
	snap.AllConcerns = []readinesspilot.Concern{
		readinessConcernProblem(), question, coverage, readinessConcernVerdict(),
		readinessConcernAction(), readinessConcernSignoff(),
	}
	snap.Attention = []readinesspilot.Concern{readinessConcernAction(), readinessConcernSignoff()}
	return snap
}

// TestReadinessFixture_ContractValid pins every fixture to the readiness
// contract: a fixture Validate() rejects would make every assertion below
// untrustworthy.
func TestReadinessFixture_ContractValid(t *testing.T) {
	for name, snap := range map[string]readinesspilot.Snapshot{
		"mixed":      readinessFixture(),
		"with role":  readinessWithRoleFixture(),
		"all proven": readinessAllProvenFixture(),
		"last step":  readinessLastStepFixture(),
	} {
		if err := snap.Validate(); err != nil {
			t.Fatalf("%s fixture violates the readiness contract: %v", name, err)
		}
	}
}

func renderReadinessFixture(t *testing.T, snap readinesspilot.Snapshot) string {
	t.Helper()
	out, err := renderReadiness(t.Context(), "", nil, snap)
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

// concernRow is one concern's whole article, by its id: from the article's
// opening tag (its class and id attributes included) to its close.
func concernRow(t *testing.T, html, id string) string {
	t.Helper()
	at := strings.Index(html, `data-concern-id="`+id+`"`)
	if at < 0 {
		t.Fatalf("page does not contain concern %q", id)
	}
	start := strings.LastIndex(html[:at], `<article `)
	if start < 0 {
		t.Fatalf("concern %q is not inside an article", id)
	}
	end := strings.Index(html[at:], `</article>`)
	if end < 0 {
		t.Fatalf("concern %q's article never closes", id)
	}
	return html[start : at+end]
}

// TestReadinessRender_WhereYouAre is ac-1's header: the eyebrow, the exact
// target title, the class chip beside it, the spec ref and the branch,
// then the current step and the purpose, all before the target's technical
// metadata; the derivation stamp stays inside the block.
func TestReadinessRender_WhereYouAre(t *testing.T) {
	snap := readinessFixture()
	html := renderReadinessFixture(t, snap)

	title := strings.Index(html, `<h2 class="readiness-title">Pilot decline flow</h2>`)
	chip := strings.Index(html, `<span class="readiness-class-chip readiness-class-chip--story" data-testid="readiness-class-chip" data-class="story">story</span>`)
	refs := strings.Index(html, `<code class="readiness-ref" data-testid="readiness-target-ref">spec/pilot</code>`)
	branch := strings.Index(html, `<code class="readiness-ref" data-testid="readiness-branch">design/pilot</code>`)
	step := strings.Index(html, `<p class="readiness-step">Step 1 of 4 — Define the work</p>`)
	purpose := strings.Index(html, `This page derives readiness for the current design work on every request.`)
	target := strings.Index(html, `readiness-target-tech`)
	for name, idx := range map[string]int{"title": title, "chip": chip, "refs": refs, "branch": branch, "step": step, "purpose": purpose, "target": target} {
		if idx < 0 {
			t.Fatalf("page is missing its %s:\n%s", name, html)
		}
	}
	if title >= chip || chip >= refs || refs >= branch || branch >= step || step >= purpose || purpose >= target {
		t.Fatalf("where-you-are order is wrong: title=%d chip=%d refs=%d branch=%d step=%d purpose=%d target=%d", title, chip, refs, branch, step, purpose, target)
	}
	if !strings.Contains(html, `<p class="readiness-eyebrow">Where you are</p>`) {
		t.Fatal("page is missing the Where you are eyebrow")
	}
	if strings.Contains(html, `metadata-card`) {
		t.Fatal("page still renders the shell's leading metadata card")
	}

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

	orient := sectionOf(t, html, `<section class="readiness-orient"`, `</section>`)
	if strings.Contains(sectionOf(t, orient, `<h2 class="readiness-title">`, `</h2>`), "spec/pilot") {
		t.Fatalf("orientation title uses the technical ref:\n%s", orient)
	}
	if !strings.Contains(orient, `class="readiness-stale"`) {
		t.Fatalf("stale notice left the orientation block:\n%s", orient)
	}
}

// TestReadinessRender_StepperStatesLinesAndFocus is ac-1's stepper: four
// stations in the fixed order, each with its plain state, its formal id as
// secondary text (dc-1), and one line of count by state and reason from
// the step order (SI-339 (8)); the current step alone is aria-current.
func TestReadinessRender_StepperStatesLinesAndFocus(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	rail := sectionOf(t, html, `<nav class="readiness-rail"`, `</nav>`)

	type station struct{ area, label, plain, formal, reason, line string }
	want := []station{
		{"shape-proposal", "Define the work", "Not enough evidence yet", "unproven", "current-focus", "1 not enough evidence yet, 1 proven — current focus"},
		{"show-success", "Define success", "Violated", "violated-with-witness", "waits", "1 violated — waits on Define the work"},
		{"check-context", "Check constraints", "Proven", "proven", "complete", "1 proven — complete"},
		{"request-review", "Get approval", "Not enough evidence yet", "unproven", "waits", "1 violated, 1 not enough evidence yet — waits on Define the work"},
	}
	prev := -1
	for i, st := range want {
		idx := strings.Index(rail, `data-area-id="`+st.area+`"`)
		if idx < 0 {
			t.Fatalf("stepper is missing station %q", st.area)
		}
		if idx < prev {
			t.Fatalf("station %q is out of the fixed order", st.area)
		}
		prev = idx
		block := sectionOf(t, rail, `data-area-id="`+st.area+`"`, `</li>`)
		for _, piece := range []string{
			`data-state="` + st.formal + `"`,
			`data-reason="` + st.reason + `"`,
			`<span class="readiness-station-label">` + st.label + `</span>`,
			`<span class="readiness-station-id">` + st.area + `</span>`,
			`>` + st.plain + `<`,
			`href="#area-` + st.area + `"`,
			`<span class="readiness-station-step">step</span>`,
			`<span class="readiness-station-num">` + []string{"1", "2", "3", "4"}[i] + `</span>`,
			`<span class="readiness-station-line">` + st.line + `</span>`,
		} {
			if !strings.Contains(block, piece) {
				t.Fatalf("station %q is missing %q:\n%s", st.area, piece, block)
			}
		}
	}
	if got := strings.Count(rail, `aria-current="step"`); got != 1 {
		t.Fatalf("stepper carries %d aria-current markers, want exactly 1", got)
	}
	if !strings.Contains(sectionOf(t, rail, `data-area-id="shape-proposal"`, `</li>`), `aria-current="step"`) {
		t.Fatal("current-focus marker is not on the snapshot's focus area")
	}
}

// TestReadinessRender_StepLineRule pins the stepper line's projection on
// synthetic snapshots: counts by state with zero counts omitted, and the
// reason from the step order — a proven step is complete wherever it
// sits, the current step is the current focus, and an unresolved later
// step waits on the current step's label.
func TestReadinessRender_StepLineRule(t *testing.T) {
	areas := func(states ...readinesspilot.State) []readinesspilot.Area {
		out := []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "Define the work"}, {ID: readinesspilot.AreaSuccess, Label: "Define success"},
			{ID: readinesspilot.AreaContext, Label: "Check constraints"}, {ID: readinesspilot.AreaReview, Label: "Get approval"},
		}
		for i := range states {
			out[i].State = states[i]
		}
		return out
	}
	concern := func(area readinesspilot.AreaID, state readinesspilot.State) readinesspilot.Concern {
		return readinesspilot.Concern{Area: area, State: state}
	}
	tests := []struct {
		name   string
		snap   readinesspilot.Snapshot
		area   int
		reason string
		line   string
	}{
		{"the current step", readinesspilot.Snapshot{
			Areas: areas(readinesspilot.StateUnproven, readinesspilot.StateProven, readinesspilot.StateProven, readinesspilot.StateProven), CurrentFocus: readinesspilot.AreaShape,
			AllConcerns: []readinesspilot.Concern{concern(readinesspilot.AreaShape, readinesspilot.StateUnproven), concern(readinesspilot.AreaShape, readinesspilot.StateUnproven), concern(readinesspilot.AreaShape, readinesspilot.StateViolated)},
		}, 0, "current-focus", "1 violated, 2 not enough evidence yet — current focus"},
		{"a later unresolved step waits on the current one", readinesspilot.Snapshot{
			Areas: areas(readinesspilot.StateUnproven, readinesspilot.StateProven, readinesspilot.StateViolated, readinesspilot.StateProven), CurrentFocus: readinesspilot.AreaShape,
			AllConcerns: []readinesspilot.Concern{concern(readinesspilot.AreaContext, readinesspilot.StateViolated), concern(readinesspilot.AreaContext, readinesspilot.StateProven)},
		}, 2, "waits", "1 violated, 1 proven — waits on Define the work"},
		{"a proven step after the current one is complete", readinesspilot.Snapshot{
			Areas: areas(readinesspilot.StateUnproven, readinesspilot.StateProven, readinesspilot.StateProven, readinesspilot.StateProven), CurrentFocus: readinesspilot.AreaShape,
			AllConcerns: []readinesspilot.Concern{concern(readinesspilot.AreaReview, readinesspilot.StateProven), concern(readinesspilot.AreaReview, readinesspilot.StateProven)},
		}, 3, "complete", "2 proven — complete"},
		{"every step complete with no focus", readinesspilot.Snapshot{
			Areas: areas(readinesspilot.StateProven, readinesspilot.StateProven, readinesspilot.StateProven, readinesspilot.StateProven), CurrentFocus: "",
			AllConcerns: []readinesspilot.Concern{concern(readinesspilot.AreaSuccess, readinesspilot.StateProven)},
		}, 1, "complete", "1 proven — complete"},
		{"a step with no concern says so rather than counting nothing", readinesspilot.Snapshot{
			Areas: areas(readinesspilot.StateUnproven, readinesspilot.StateUnproven, readinesspilot.StateProven, readinesspilot.StateProven), CurrentFocus: readinesspilot.AreaShape,
			AllConcerns: []readinesspilot.Concern{},
		}, 1, "waits", "no concerns — waits on Define the work"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, kind := readinessStepReason(tt.snap, tt.snap.Areas[tt.area])
			if kind != tt.reason {
				t.Fatalf("readinessStepReason kind = %q, want %q", kind, tt.reason)
			}
			if got := readinessStepLine(readinessPageWords(), readinessStepCounts(tt.snap, tt.snap.Areas[tt.area].ID), reason); got != tt.line {
				t.Fatalf("readinessStepLine = %q, want %q", got, tt.line)
			}
		})
	}
}

// TestReadinessRender_OrderingSentence is ac-1's plain explanation of why
// the steps run in their order, between the stepper and Focus next.
func TestReadinessRender_OrderingSentence(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	order := strings.Index(html, `<p class="readiness-order" data-testid="readiness-order">The four steps run in this order: define the work, define success, check constraints, then get approval. Later steps report their known problems and wait on the current step; the current step's items are what move this design forward now.</p>`)
	rail := strings.Index(html, `</nav>`)
	focus := strings.Index(html, `id="readiness-focus"`)
	if order < 0 {
		t.Fatalf("page is missing the ordering sentence:\n%s", html)
	}
	if rail >= order || order >= focus {
		t.Fatalf("ordering sentence is out of place: rail end=%d order=%d focus=%d", rail, order, focus)
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
		t.Fatal("all-proven snapshot renders a later-steps section with no current step")
	}
	if got := strings.Count(html, `data-reason="complete"`); got != 4 {
		t.Fatalf("all-proven stepper marks %d stations complete, want 4", got)
	}
	queue := sectionOf(t, html, `<section class="readiness-queue"`, `</section>`)
	if strings.Contains(queue, "data-concern-id") || strings.Contains(queue, "readiness-more") {
		t.Fatalf("all-proven focus list still lists concerns:\n%s", queue)
	}
	if !strings.Contains(queue, "Nothing needs attention: every check in this snapshot is proven.") {
		t.Fatalf("empty focus list does not state its honest reason:\n%s", queue)
	}
	completed := sectionOf(t, html, `<section class="readiness-completed"`, "</main>")
	if got := strings.Count(completed, `data-concern-id="`); got != 4 {
		t.Fatalf("completed checks list %d proven concerns, want 4", got)
	}
}

// TestReadinessRender_FocusNowVisibleLaterDisclosed is ac-2 and SI-339
// (3)-(4): every concern of the current step is in the visible list,
// marked with its step and "now"; the later steps' concerns sit behind one
// inline disclosure, each marked "later" with what it waits on, ranked on
// from the visible list.
func TestReadinessRender_FocusNowVisibleLaterDisclosed(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	queue := sectionOf(t, html, `<section class="readiness-queue"`, `</section>`)
	if !strings.Contains(queue, `<h2 class="readiness-heading">Focus next<span class="readiness-count"> · 4</span></h2>`) {
		t.Fatalf("focus heading does not count the whole attention list:\n%s", queue)
	}

	now := sectionOf(t, queue, `<ol class="readiness-queue-list">`, `</ol>`)
	if got := strings.Count(now, `data-concern-id="`); got != 1 {
		t.Fatalf("visible list shows %d concerns, want exactly the current step's 1", got)
	}
	question := concernRow(t, now, "shape/question/q-alpha")
	for _, want := range []string{
		`data-timing="now"`,
		`<span class="readiness-rank">1</span>`,
		`<p class="readiness-stage">Define the work <span class="readiness-when readiness-when--now">now</span></p>`,
	} {
		if !strings.Contains(question, want) {
			t.Fatalf("current-step row is missing %q:\n%s", want, question)
		}
	}

	// The disclosure runs to the section's end: each row's own technical
	// details nest a </details> of their own inside it.
	more := sectionOf(t, queue, `<details class="readiness-more" data-testid="readiness-later">`, "")
	if strings.Index(queue, `<details class="readiness-more"`) < strings.Index(queue, `</ol>`) {
		t.Fatal("the later-steps disclosure precedes the visible list")
	}
	for _, want := range []string{
		`<summary class="readiness-more-summary"><span class="readiness-more-closed">3 more, waiting on Define the work</span><span class="readiness-more-open">Show fewer</span></summary>`,
		`<ol class="readiness-queue-list readiness-queue-rest" start="2">`,
	} {
		if !strings.Contains(more, want) {
			t.Fatalf("later-steps disclosure is missing %q:\n%s", want, more)
		}
	}
	prev := -1
	for i, id := range []string{"success/blocker/obligation-quality/coverage", "review/action", "review/blocker/gov-signoff"} {
		row := concernRow(t, more, id)
		idx := strings.Index(more, `data-concern-id="`+id+`"`)
		if idx < prev {
			t.Fatalf("later concern %q is out of the attention order", id)
		}
		prev = idx
		for _, want := range []string{
			`data-timing="later"`,
			`<span class="readiness-rank">` + []string{"2", "3", "4"}[i] + `</span>`,
			`<span class="readiness-when readiness-when--later">later — waits on Define the work</span>`,
		} {
			if !strings.Contains(row, want) {
				t.Fatalf("later row %q is missing %q:\n%s", id, want, row)
			}
		}
	}
	if got := strings.Count(more, `data-concern-id="`); got != 3 {
		t.Fatalf("disclosure holds %d concerns, want 3", got)
	}
}

// TestReadinessRender_FocusAllInCurrentStepHasNoDisclosure: when nothing
// waits on the current step, no disclosure control is rendered — a control
// with nothing behind it would mislead.
func TestReadinessRender_FocusAllInCurrentStepHasNoDisclosure(t *testing.T) {
	html := renderReadinessFixture(t, readinessLastStepFixture())
	queue := sectionOf(t, html, `<section class="readiness-queue"`, `</section>`)
	if strings.Contains(queue, "readiness-more") || strings.Contains(queue, "waiting on") {
		t.Fatalf("a focus list with nothing waiting renders a disclosure control:\n%s", queue)
	}
	if got := strings.Count(queue, `data-timing="now"`); got != 2 {
		t.Fatalf("focus list marks %d rows now, want all 2", got)
	}
	if got := strings.Count(html, `data-reason="complete"`); got != 3 {
		t.Fatalf("stepper marks %d earlier steps complete, want 3", got)
	}
}

// TestReadinessRender_KnownProblemsLinkLaterViolatedRows is ac-1 and
// SI-339 (7): the later steps' violated concerns only, each a link to that
// concern's one row — never a second row carrying the concern id.
func TestReadinessRender_KnownProblemsLinkLaterViolatedRows(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	known := sectionOf(t, html, `<section class="readiness-known"`, `</section>`)
	if !strings.Contains(known, `<h2 class="readiness-heading readiness-heading--violated">Known problems in later steps<span class="readiness-count"> · 2</span></h2>`) {
		t.Fatalf("known-problems heading does not count the later violated rows:\n%s", known)
	}
	// An entry is a link to the row and nothing of the row itself: no
	// concern row, no disclosure, no guidance, no destination (review
	// finding F4P-2).
	for _, forbidden := range []string{`data-concern-id="`, `<details`, `readiness-guidance`, `readiness-dest`, `readiness-board-link`, `readiness-tech`} {
		if strings.Contains(known, forbidden) {
			t.Fatalf("known-problems section carries %q:\n%s", forbidden, known)
		}
	}
	prev := -1
	for _, want := range []struct{ id, label, summary string }{
		{"success/blocker/obligation-quality/coverage", "Define success", readinessConcernCoverage().Summary},
		{"review/blocker/gov-signoff", "Get approval", readinessConcernSignoff().Summary},
	} {
		idx := strings.Index(known, `data-known-concern="`+want.id+`"`)
		if idx < 0 {
			t.Fatalf("known-problems list is missing %q:\n%s", want.id, known)
		}
		if idx < prev {
			t.Fatalf("known problem %q is out of the attention order", want.id)
		}
		prev = idx
		// The entry's exact content: its step label, its fact, its chip.
		entry := `<li><a class="readiness-known-link" href="#concern-` + want.id + `" data-known-concern="` + want.id + `">` +
			`<span class="readiness-stage">` + want.label + `</span>` +
			`<span class="readiness-known-text">` + stdhtml.EscapeString(want.summary) + `</span>` +
			`<span class="readiness-state readiness-state--violated-with-witness">Violated</span></a></li>`
		if !strings.Contains(known, entry) {
			t.Fatalf("known problem %q is not exactly its label, fact and chip:\nwant %s\nin   %s", want.id, entry, known)
		}
		if got := strings.Count(html, `id="concern-`+want.id+`"`); got != 1 {
			t.Fatalf("concern %q has %d row anchors, want exactly 1", want.id, got)
		}
	}
	if got := strings.Count(known, `data-known-concern="`); got != 2 {
		t.Fatalf("known-problems list has %d entries, want 2", got)
	}

	// A current step with no later violated row says so plainly.
	empty := renderReadinessFixture(t, readinessLastStepFixture())
	if !strings.Contains(sectionOf(t, empty, `<section class="readiness-known"`, `</section>`), `<p class="readiness-known-empty">None: no later step reports a violated check.</p>`) {
		t.Fatalf("empty known-problems section does not state its honest reason:\n%s", empty)
	}

	// The listing rule itself, on synthetic snapshots distinguishing
	// current, earlier, later, violated, and unproven rows.
	areas := []readinesspilot.Area{
		{ID: readinesspilot.AreaShape}, {ID: readinesspilot.AreaSuccess},
		{ID: readinesspilot.AreaContext}, {ID: readinesspilot.AreaReview},
	}
	concern := func(area readinesspilot.AreaID, state readinesspilot.State) readinesspilot.Concern {
		return readinesspilot.Concern{ID: string(area) + "/x", Area: area, State: state}
	}
	tests := []struct {
		name  string
		focus readinesspilot.AreaID
		rows  []readinesspilot.Concern
		want  int
	}{
		{"current-area violation is not a later problem", readinesspilot.AreaContext,
			[]readinesspilot.Concern{concern(readinesspilot.AreaContext, readinesspilot.StateViolated)}, 0},
		{"earlier-area violation is not a later problem", readinesspilot.AreaContext,
			[]readinesspilot.Concern{concern(readinesspilot.AreaSuccess, readinesspilot.StateViolated)}, 0},
		{"later violation is listed", readinesspilot.AreaContext,
			[]readinesspilot.Concern{concern(readinesspilot.AreaReview, readinesspilot.StateViolated)}, 1},
		{"later unproven is not a known problem", readinesspilot.AreaContext,
			[]readinesspilot.Concern{concern(readinesspilot.AreaReview, readinesspilot.StateUnproven)}, 0},
		{"no focus means nothing", "",
			[]readinesspilot.Concern{concern(readinesspilot.AreaReview, readinesspilot.StateViolated)}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := readinesspilot.Snapshot{Areas: areas, CurrentFocus: tt.focus, Attention: tt.rows}
			if got := len(readinessKnownProblems(snap)); got != tt.want {
				t.Fatalf("readinessKnownProblems lists %d rows, want %d", got, tt.want)
			}
		})
	}
}

// TestReadinessRender_PlainStateLabelsWithFormalDetails is ac-3's triad
// (SI-339 (10)): every chip pairs its formal modifier class with the plain
// label, and the formal words stay reachable in technical details.
func TestReadinessRender_PlainStateLabelsWithFormalDetails(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	for formal, plain := range map[string]string{
		"proven":                "Proven",
		"violated-with-witness": "Violated",
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
	for _, retired := range []string{">Ready<", ">Needs attention<"} {
		if strings.Contains(html, retired) {
			t.Fatalf("page still speaks the retired label %q", retired)
		}
	}
	for _, formal := range []string{"proven", "violated-with-witness", "unproven"} {
		if !strings.Contains(html, `<dd><code>`+formal+`</code></dd>`) {
			t.Fatalf("technical details never state formal state %q", formal)
		}
	}
	// A violated row's witness is in its own technical details.
	violated := concernRow(t, html, "success/blocker/obligation-quality/coverage")
	if !strings.Contains(violated, `<li><code>gate run 41 is red</code></li>`) {
		t.Fatalf("violated row does not carry its witness:\n%s", violated)
	}
}

// TestReadinessRender_TechnicalDetailsComplete: the disclosure holds the
// fact (the summary), the formal state, the concern and area ids, the
// blocking flag, the snapshot's own timing (current or eventual,
// unchanged), the work class when present, the witnesses, and the
// destination data.
func TestReadinessRender_TechnicalDetailsComplete(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	card := concernRow(t, html, "success/blocker/obligation-quality/coverage")
	tech := sectionOf(t, card, `<details class="readiness-tech">`, `</details>`)

	for _, want := range []string{
		`<summary>Technical details</summary>`,
		`<dt>Fact</dt><dd class="readiness-fact">Coverage gate must be green</dd>`,
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
	if !strings.Contains(tech, `<code>verdi</code>`) || strings.Contains(html, "verdi gate run --target spec/pilot") {
		t.Fatalf("technical destination is not the exact token vector:\n%s", tech)
	}
	// The snapshot's eventual timing is kept as it is, beside the row's
	// own "later" mark.
	signoff := concernRow(t, html, "review/blocker/gov-signoff")
	if !strings.Contains(signoff, `<dt>Timing</dt><dd><code>eventual</code></dd>`) {
		t.Fatalf("eventual row's timing is not the snapshot's own:\n%s", signoff)
	}

	question := concernRow(t, html, "shape/question/q-alpha")
	if strings.Contains(question, "Work class") {
		t.Fatalf("concern without a source work class renders one:\n%s", question)
	}
	completed := sectionOf(t, html, `<section class="readiness-completed"`, "</main>")
	proven := concernRow(t, completed, "shape/problem")
	if !strings.Contains(proven, `<dd><code>proven</code></dd>`) {
		t.Fatalf("proven concern's details lost its formal state:\n%s", proven)
	}
	if strings.Contains(proven, "Destination") || strings.Contains(proven, "readiness-board-link") || strings.Contains(proven, "readiness-cli-token") {
		t.Fatalf("proven concern renders a destination:\n%s", proven)
	}
}

// readinessLeadIn matches everything a row's copy block renders before
// its one primary line: the step label (with its now/later mark when it
// has one) and then, optionally, the plain human-review label — nothing
// else, in that order. Group 1 is the human-review label when present.
var readinessLeadIn = regexp.MustCompile(`^<div class="readiness-copy"><p class="readiness-stage">[^<]*(?:<span class="readiness-when readiness-when--(?:now|later)">[^<]*</span>)?</p>(<p class="readiness-human-review" data-testid="readiness-human-review">.*?</p>)?$`)

// TestReadinessRender_GuidanceIsPrimaryCopy is ac-2 and SI-339 (2): an
// unresolved concern's primary line is its guidance, its fact filed in the
// disclosure; a proven concern, which carries no guidance, leads with its
// fact. The primary line is the copy block's first line after the step
// label — and after the human-review label exactly when HumanReview()
// holds (review finding F4P-1): no fact line, chip, or anything else
// comes before it, and there is exactly one primary line.
func TestReadinessRender_GuidanceIsPrimaryCopy(t *testing.T) {
	snap := readinessWithRoleFixture()
	html := renderReadinessFixture(t, snap)
	for _, concern := range snap.AllConcerns {
		row := concernRow(t, html, concern.ID)
		primary := `<p class="readiness-summary">` + stdhtml.EscapeString(concern.Summary) + `</p>`
		if concern.State == readinesspilot.StateProven {
			if strings.Contains(row, "readiness-guidance") {
				t.Fatalf("proven concern %q renders a guidance line:\n%s", concern.ID, row)
			}
		} else {
			primary = `<p class="readiness-summary readiness-guidance">` + stdhtml.EscapeString(concern.Guidance) + `</p>`
			if !strings.Contains(row, `<dt>Fact</dt><dd class="readiness-fact">`+stdhtml.EscapeString(concern.Summary)+`</dd>`) {
				t.Fatalf("concern %q does not file its fact in the disclosure:\n%s", concern.ID, row)
			}
		}
		at := strings.Index(row, primary)
		if at < 0 {
			t.Fatalf("concern %q does not render its primary line %q:\n%s", concern.ID, primary, row)
		}
		if got := strings.Count(row, `<p class="readiness-summary`); got != 1 {
			t.Fatalf("concern %q renders %d primary lines, want exactly 1:\n%s", concern.ID, got, row)
		}
		copyAt := strings.Index(row, `<div class="readiness-copy">`)
		if copyAt < 0 || copyAt > at {
			t.Fatalf("concern %q's primary line is outside its copy block:\n%s", concern.ID, row)
		}
		lead := readinessLeadIn.FindStringSubmatch(row[copyAt:at])
		if lead == nil {
			t.Fatalf("concern %q renders something other than the step label (and the human-review label) before its primary line:\n%s", concern.ID, row[copyAt:at])
		}
		if (lead[1] != "") != concern.HumanReview() {
			t.Fatalf("concern %q (human review %v) leads with human-review label %q:\n%s", concern.ID, concern.HumanReview(), lead[1], row)
		}
		if strings.Index(row, `<span class="readiness-state`) < at || strings.Index(row, `<details class="readiness-tech">`) < at {
			t.Fatalf("concern %q's chip or disclosure precedes its primary line:\n%s", concern.ID, row)
		}
	}
}

// TestReadinessRender_HumanReviewLabeledPlainly is ac-3 and SI-339 (5):
// a concern whose HumanReview() is true is labeled "Human review", with
// the formal obligation — its id and, when it has one, its work class —
// as secondary text; a judgmental or mechanical row carries no label.
func TestReadinessRender_HumanReviewLabeledPlainly(t *testing.T) {
	html := renderReadinessFixture(t, readinessWithRoleFixture())
	signoff := concernRow(t, html, "review/blocker/gov-signoff")
	if !strings.Contains(signoff, `<p class="readiness-human-review" data-testid="readiness-human-review">Human review<span class="readiness-human-review-formal"> · <code>review/blocker/gov-signoff</code> · <code>governance</code></span></p>`) {
		t.Fatalf("governance row is not labeled human review with its formal obligation:\n%s", signoff)
	}
	if !strings.Contains(signoff, `readiness-concern--violated-with-witness readiness-concern--human-review"`) {
		t.Fatalf("governance row's article lacks the human-review modifier after its state:\n%s", signoff)
	}
	role := concernRow(t, html, "review/role/close/attestation/countersign")
	if !strings.Contains(role, `<p class="readiness-human-review" data-testid="readiness-human-review">Human review<span class="readiness-human-review-formal"> · <code>review/role/close/attestation/countersign</code></span></p>`) {
		t.Fatalf("role row is not labeled human review with its id alone:\n%s", role)
	}
	if !strings.Contains(role, `<p class="readiness-summary readiness-guidance">`+stdhtml.EscapeString(readinessConcernRole().Guidance)+`</p>`) {
		t.Fatalf("role row does not carry its derived text verbatim:\n%s", role)
	}
	for _, id := range []string{"shape/question/q-alpha", "success/blocker/obligation-quality/coverage", "review/action"} {
		if row := concernRow(t, html, id); strings.Contains(row, "readiness-human-review") {
			t.Fatalf("non-human-review concern %q is labeled human review:\n%s", id, row)
		}
	}
	if got := strings.Count(html, `data-testid="readiness-human-review"`); got != 2 {
		t.Fatalf("page labels %d rows human review, want exactly 2", got)
	}
}

func TestReadinessRender_LosslessDisjointInventoryAndAnchors(t *testing.T) {
	snap := readinessWithRoleFixture()
	html := renderReadinessFixture(t, snap)

	if got := strings.Count(html, `data-concern-id="`); got != len(snap.AllConcerns) {
		t.Fatalf("page lists %d concern rows, want exactly %d (no omission, no duplication)", got, len(snap.AllConcerns))
	}
	for _, concern := range snap.AllConcerns {
		if got := strings.Count(html, `id="concern-`+concern.ID+`"`); got != 1 {
			t.Fatalf("concern %q has %d row anchors, want exactly 1", concern.ID, got)
		}
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
	if !strings.Contains(completed, `<h2 class="readiness-heading readiness-heading--proven">Completed checks<span class="readiness-count"> · 2</span></h2>`) {
		t.Fatalf("completed heading does not count the proven rows:\n%s", completed)
	}
	// Every area keeps exactly one fragment anchor, on its first row's item.
	for _, area := range []string{"shape-proposal", "show-success", "check-context", "request-review"} {
		if got := strings.Count(html, `id="area-`+area+`"`); got != 1 {
			t.Fatalf("area %q has %d fragment anchors, want exactly 1", area, got)
		}
	}
}

func TestReadinessRender_DestinationActionsUsable(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	question := concernRow(t, html, "shape/question/q-alpha")
	link := sectionOf(t, question, `class="readiness-board-link"`, `</a>`)
	for _, want := range []string{`href="/board/spec/pilot"`, `target="_blank"`, `rel="noopener"`} {
		if !strings.Contains(link, want) {
			t.Fatalf("board link is missing %q:\n%s", want, link)
		}
	}
	if strings.Contains(question, "readiness-cli-token") {
		t.Fatalf("board-destination concern also renders CLI tokens:\n%s", question)
	}

	coverage := concernRow(t, html, "success/blocker/obligation-quality/coverage")
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

// TestReadinessRender_OpenTheWallInTheBar is SI-339 (9): "Open the wall →"
// rides the bar's controls slot with its own class — never the
// per-concern destination link's — so the page's first board link is still
// the first concern's; a snapshot with no wall address carries no link and
// says so in the body.
func TestReadinessRender_OpenTheWallInTheBar(t *testing.T) {
	snap := readinessFixture()
	html := renderReadinessFixture(t, snap)
	bar := sectionOf(t, html, `<header class="topbar"`, `</header>`)
	want := `<div class="topbar-controls" data-testid="topbar-controls"><a class="btn-primary readiness-wall-link" data-testid="readiness-wall-link" href="` + snap.BoardPath + `">Open the wall<span aria-hidden="true"> →</span></a></div>`
	if !strings.Contains(bar, want) {
		t.Fatalf("bar controls are missing the wall link %q:\n%s", want, bar)
	}
	if strings.Contains(bar, "readiness-board-link") {
		t.Fatalf("the bar's wall link wears the destination link's class:\n%s", bar)
	}
	if first := sectionOf(t, html, `class="readiness-board-link"`, `</a>`); !strings.Contains(first, `href="/board/spec/pilot"`) {
		t.Fatalf("the page's first board link is not the first concern's:\n%s", first)
	}
	if strings.Contains(html, "readiness-wall-absent") {
		t.Fatal("a snapshot with a wall address discloses a missing one")
	}

	snap.BoardPath = ""
	absent := renderReadinessFixture(t, snap)
	if strings.Contains(absent, "readiness-wall-link") {
		t.Fatal("a snapshot with no wall address still renders the wall link")
	}
	if !strings.Contains(sectionOf(t, absent, `<section class="readiness-orient"`, `</section>`), `<p class="readiness-wall-absent">Open the wall: no wall address is known for this request.</p>`) {
		t.Fatalf("a snapshot with no wall address does not disclose it:\n%s", absent)
	}
}

// TestReadinessRender_DerivationStampNamesHead is spec/readiness-recovery
// ac-2 and readiness-page-v2 ac-4: the page's notice is a derivation stamp
// naming the HEAD this request looked at — never a startup notice telling
// the author to restart. The chrome (class names, role, data attribute,
// tabindex) is unchanged so the stale-notice-inspected instrumentation
// keeps working. The stamp-text assertion repeats the fixture's own
// StaleNotice, so it proves pass-through and escaping only; the wording
// oracle is TestLoad_AnyBranchNoRequest (internal/readinessload), which
// pins the sentence where it is produced.
func TestReadinessRender_DerivationStampNamesHead(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	notice := sectionOf(t, html, `class="readiness-stale"`, `</aside>`)
	for _, want := range []string{
		"Derived at HEAD " + readinessFixtureHead + " for this request.",
		`<strong>Derivation stamp.</strong>`,
		`aria-label="Derivation stamp"`,
		`role="note"`,
		`data-readiness-stale="1"`,
		`class="readiness-stale-text"`,
		`tabindex="0"`,
	} {
		if !strings.Contains(notice, want) {
			t.Fatalf("derivation stamp is missing %q:\n%s", want, notice)
		}
	}
	for _, forbidden := range []string{"Startup snapshot", "restart"} {
		if strings.Contains(notice, forbidden) {
			t.Fatalf("derivation stamp still carries the startup notice text %q:\n%s", forbidden, notice)
		}
	}
	if strings.Contains(strings.ToLower(html), "startup snapshot") {
		t.Fatalf("page still describes itself as a startup snapshot (any case):\n%s", html)
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
				t.Fatalf("readiness page carries mutation/network surface %q", forbidden)
			}
		}
	}
}

func TestReadinessRender_EscapesUntrustedText(t *testing.T) {
	snap := readinessFixture()
	snap.TargetTitle = `<script>alert(0)</script>`
	snap.AllConcerns[2].Summary = `<script>alert(1)</script>`
	snap.Attention[1].Summary = `<script>alert(1)</script>`
	snap.AllConcerns[2].Guidance = `<script>alert(3)</script>`
	snap.Attention[1].Guidance = `<script>alert(3)</script>`
	snap.AllConcerns[2].Witnesses = []string{`"><img src=x onerror=alert(2)>`}
	snap.Attention[1].Witnesses = []string{`"><img src=x onerror=alert(2)>`}
	snap.BoardPath = `/b/x" onclick="alert(4)`
	html := renderReadinessFixture(t, snap)
	for _, raw := range []string{"<script>alert(0)", "<script>alert(1)", "<script>alert(3)", "<img src=x", `" onclick="alert(4)`} {
		if strings.Contains(html, raw) {
			t.Fatalf("page carries unescaped text %q", raw)
		}
	}
}

func TestReadinessRender_KeyboardLandmarksAndScript(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	for _, want := range []string{
		`<nav class="readiness-rail" aria-label="Readiness rail">`,
		`<p class="readiness-order" data-testid="readiness-order">`,
		`aria-label="Focus next"`,
		`aria-label="Known problems in later steps"`,
		`aria-label="Completed checks"`,
		`href="#area-shape-proposal"`,
		`id="area-shape-proposal"`,
		`<details class="readiness-more" data-testid="readiness-later">`,
		`<summary class="readiness-more-summary">`,
		`<details class="readiness-tech">`,
		`<details class="readiness-tech readiness-target-tech">`,
		`<p class="readiness-dest readiness-cli" data-readiness-cli="1" tabindex="0" aria-label="CLI fallback">`,
		`<script src="/assets/readiness.js" defer></script>`,
		`<div class="readiness-page readiness-standalone">`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("page is missing keyboard/instrumentation hook %q", want)
		}
	}
}

// TestReadinessRender_FragmentHrefEscapesTheConcernId: a known-problem
// link's fragment percent-encodes what a fragment cannot carry raw — a
// hash in a concern id — while the row's id attribute keeps the id
// verbatim, which the browser's percent-decoded fragment matches.
func TestReadinessRender_FragmentHrefEscapesTheConcernId(t *testing.T) {
	for in, want := range map[string]string{
		"shape/question/q-alpha":                             "shape/question/q-alpha",
		"context/mechanical/action:make-verify#complete":     "context/mechanical/action:make-verify%23complete",
		"review/blocker/conflict-semantic/sha256-9fe503eb5b": "review/blocker/conflict-semantic/sha256-9fe503eb5b",
		"shape/board/question/a b%c":                         "shape/board/question/a%20b%25c",
	} {
		if got := readinessFragment(in); got != want {
			t.Fatalf("readinessFragment(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReadinessRoute_GetHappy(t *testing.T) {
	snap := readinessFixture()
	h := NewHandlerWith(t.TempDir(), Deps{ReadinessLoader: fixedSnapshotLoader{snap: snap}, ReadinessDefaultSpec: snap.TargetRef})
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

// TestReadinessRoute_StoreVocabularyRenamesThePagesWords (F4-data review
// finding F4D-B1, mutant M3): the readiness route carries the store's
// resolved model, and the page's class chip speaks its display word — once
// from a store whose own model.yaml renames the classes, resolved at
// registration, and once from an injected model. A registration that
// dropped the model would render the bare id both times.
func TestReadinessRoute_StoreVocabularyRenamesThePagesWords(t *testing.T) {
	snap := readinessFixture()
	chipOf := func(t *testing.T, h http.Handler) string {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readiness", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
		}
		return sectionOf(t, rec.Body.String(), `<span class="readiness-class-chip`, `</span>`)
	}

	t.Run("the store's own model.yaml, resolved at registration", func(t *testing.T) {
		root := t.TempDir()
		modelYAML, err := os.ReadFile(filepath.Join("..", "model", "testdata", "vocab-rename.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string]string{"verdi.yaml": "schema: verdi.layout/v1\n", "model.yaml": string(modelYAML)} {
			if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".verdi", name), []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		h := NewHandlerWith(root, Deps{ReadinessLoader: fixedSnapshotLoader{snap: snap}, ReadinessDefaultSpec: snap.TargetRef})
		chip := chipOf(t, h)
		if !strings.HasSuffix(chip, `data-class="story">Workstream`) {
			t.Fatalf("class chip does not speak the store's renamed word:\n%s", chip)
		}
	})

	t.Run("an injected model", func(t *testing.T) {
		h := NewHandlerWith(t.TempDir(), Deps{Model: vocabTestModel(), ReadinessLoader: fixedSnapshotLoader{snap: snap}, ReadinessDefaultSpec: snap.TargetRef})
		chip := chipOf(t, h)
		if !strings.HasSuffix(chip, `data-class="story">Change Request`) {
			t.Fatalf("class chip does not speak the injected model's word:\n%s", chip)
		}
	})

	t.Run("no model renders the bare id", func(t *testing.T) {
		h := NewHandlerWith(t.TempDir(), Deps{ReadinessLoader: fixedSnapshotLoader{snap: snap}, ReadinessDefaultSpec: snap.TargetRef})
		chip := chipOf(t, h)
		if !strings.HasSuffix(chip, `data-class="story">story`) {
			t.Fatalf("class chip without a model is not the bare id:\n%s", chip)
		}
	})
}

// TestReadinessPage_PerRequestStampAndSharedFacts is spec/readiness-page-v2
// ac-4's static obligation (obligation/readiness-page-v2--ac-4--static;
// SI-339 (1)), the page's half. One GET /readiness through the production
// wiring — NewHandlerWith, a counting ReadinessLoader behind
// Deps.ReadinessLoader, the one readiness seam — over the mixed fixture
// proves three things: (1) the page carries the per-request derivation
// stamp, the snapshot's own StaleNotice naming the HEAD this request
// derived at; (2) the page contains no startup-snapshot text; (3) the
// concerns the page renders — every data-concern-id, each exactly once,
// with its state, its primary line and its filed fact — equal the
// snapshot the seam returned for that request, and the seam was asked
// exactly once, so there is no second derivation.
//
// DISCLOSED AS UNPROVEN: the drawer's Readiness tab does not exist yet
// (lane F3 builds it after F4, plan order), so the obligation's other
// half — "renders from the same readiness facts value the Readiness tab
// renders" — is not proven here. F3's brief extends this test to render
// the tab from the same value; until then the story cannot close on ac-4.
func TestReadinessPage_PerRequestStampAndSharedFacts(t *testing.T) {
	snap := readinessWithRoleFixture()
	loader := &countingReadinessLoader{snap: snap}
	h := NewHandlerWith(t.TempDir(), Deps{ReadinessLoader: loader, ReadinessDefaultSpec: snap.TargetRef})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readiness", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	html := rec.Body.String()

	// (1) The per-request derivation stamp, the seam's own text.
	notice := sectionOf(t, html, `<aside class="readiness-stale" role="note" tabindex="0" data-readiness-stale="1" aria-label="Derivation stamp">`, `</aside>`)
	if !strings.Contains(notice, stdhtml.EscapeString(snap.StaleNotice)) || !strings.Contains(snap.StaleNotice, snap.Head) {
		t.Fatalf("page does not carry the per-request stamp %q:\n%s", snap.StaleNotice, notice)
	}

	// (2) No startup-snapshot text anywhere on the page.
	lower := strings.ToLower(html)
	for _, forbidden := range []string{"startup snapshot", "restart verdi serve", "restart verdi"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("page carries the design's startup-snapshot copy %q", forbidden)
		}
	}

	// (3) One derivation, and the rendered concerns are its value.
	if loader.calls != 1 {
		t.Fatalf("the seam was asked %d times for one page, want exactly 1 (no second derivation)", loader.calls)
	}
	rendered := regexp.MustCompile(`data-concern-id="([^"]+)"`).FindAllStringSubmatch(html, -1)
	seen := make(map[string]int, len(rendered))
	for _, m := range rendered {
		seen[stdhtml.UnescapeString(m[1])]++
	}
	if len(rendered) != len(snap.AllConcerns) {
		t.Fatalf("page renders %d concern rows, the seam's value has %d", len(rendered), len(snap.AllConcerns))
	}
	for _, concern := range snap.AllConcerns {
		if seen[concern.ID] != 1 {
			t.Fatalf("concern %q rendered %d times, want exactly once", concern.ID, seen[concern.ID])
		}
		row := concernRow(t, html, concern.ID)
		if !strings.Contains(row, `readiness-concern--`+string(concern.State)) {
			t.Fatalf("concern %q rendered with another state than the seam's %q:\n%s", concern.ID, concern.State, row)
		}
		primary := concern.Guidance
		if primary == "" {
			primary = concern.Summary
		}
		if !strings.Contains(row, `">`+stdhtml.EscapeString(primary)+`</p>`) {
			t.Fatalf("concern %q's primary line is not the seam's %q:\n%s", concern.ID, primary, row)
		}
		if !strings.Contains(row, `<dd class="readiness-fact">`+stdhtml.EscapeString(concern.Summary)+`</dd>`) {
			t.Fatalf("concern %q's filed fact is not the seam's %q:\n%s", concern.ID, concern.Summary, row)
		}
	}
	t.Log("disclosed-as-unproven: the Readiness tab's half of ac-4 (the tab renders the same facts value) awaits lane F3; this test proves the page's half only")
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

// TestReadinessRoute_QuerySpecDerivesPerRequest is spec/readiness-recovery
// ac-2/ac-4: the readiness route asks the loader fresh for every request
// (never a startup-frozen snapshot) — a counting fake proves two GETs
// trigger two loads, ?spec=<name> passes exactly "spec/"+name, an unknown
// spec surfaces the loader's own error text as a 503, and a process with
// no loader wired at all keeps the existing 503 disclosure unchanged.
func TestReadinessRoute_QuerySpecDerivesPerRequest(t *testing.T) {
	snap := readinessFixture()
	get := func(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	t.Run("two GETs trigger two loads", func(t *testing.T) {
		loader := &countingReadinessLoader{snap: snap}
		h := NewHandlerWith(t.TempDir(), Deps{ReadinessLoader: loader, ReadinessDefaultSpec: snap.TargetRef})
		first := get(t, h, "/readiness")
		second := get(t, h, "/readiness")
		if first.Code != http.StatusOK || second.Code != http.StatusOK {
			t.Fatalf("status = %d, %d, want 200, 200", first.Code, second.Code)
		}
		if loader.calls != 2 {
			t.Fatalf("loader.calls = %d, want exactly 2 (one per GET)", loader.calls)
		}
	})

	t.Run("?spec=x passes spec/x", func(t *testing.T) {
		loader := &countingReadinessLoader{snap: snap}
		h := NewHandlerWith(t.TempDir(), Deps{ReadinessLoader: loader})
		rec := get(t, h, "/readiness?spec=pilot")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
		}
		if len(loader.refs) != 1 || loader.refs[0] != "spec/pilot" {
			t.Fatalf("loader.refs = %v, want exactly [\"spec/pilot\"]", loader.refs)
		}
	})

	t.Run("unknown spec surfaces the loader's own error as a 503", func(t *testing.T) {
		wantErr := errors.New("readinessload: loading readiness: gathering repository facts: no such spec")
		loader := erroringReadinessLoader{err: wantErr}
		h := NewHandlerWith(t.TempDir(), Deps{ReadinessLoader: loader, ReadinessDefaultSpec: "spec/pilot"})
		rec := get(t, h, "/readiness?spec=nope")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), wantErr.Error()) {
			t.Fatalf("503 body does not carry the loader's own error text:\n%s", rec.Body.String())
		}
	})

	t.Run("a bad spec name is a 400, never reaches the loader", func(t *testing.T) {
		loader := &countingReadinessLoader{snap: snap}
		h := NewHandlerWith(t.TempDir(), Deps{ReadinessLoader: loader})
		rec := get(t, h, "/readiness?spec=Not%20A%20Valid%20Name")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if loader.calls != 0 {
			t.Fatalf("loader.calls = %d, want 0 (a bad name must never reach the loader)", loader.calls)
		}
		_, parseErr := artifact.ParseRef("spec/Not A Valid Name")
		if parseErr == nil {
			t.Fatal("fixture no longer malformed: ParseRef accepted it")
		}
		if !strings.Contains(rec.Body.String(), stdhtml.EscapeString(parseErr.Error())) {
			t.Fatalf("the 400 page must disclose WHY the name was refused (%q):\n%s", parseErr.Error(), rec.Body.String())
		}
	})

	// A pinned or fragment ?spec= parses fine but is not a whole spec ref,
	// which is the route's own shape rule: the loader refuses it one layer
	// down (readinessload: "not an unpinned whole spec ref") and that
	// arrives as a 503 — the wrong code for a malformed query, and a
	// derivation attempt the handler never had to make. The gate owns both.
	for _, tc := range []struct{ name, query, want string }{
		{name: "pinned", query: "pilot@" + strings.Repeat("a", 40), want: "commit pin"},
		{name: "fragment", query: "pilot#ac-1", want: "fragment"},
	} {
		t.Run("a "+tc.name+" spec ref is a 400, never reaches the loader", func(t *testing.T) {
			loader := &countingReadinessLoader{snap: snap}
			h := NewHandlerWith(t.TempDir(), Deps{ReadinessLoader: loader})
			rec := get(t, h, "/readiness?spec="+url.QueryEscape(tc.query))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
			}
			if loader.calls != 0 {
				t.Fatalf("loader.calls = %d, want 0 (a %s ref must never reach the loader)", loader.calls, tc.name)
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("the 400 page must disclose why a %s ref was refused (naming %q):\n%s", tc.name, tc.want, rec.Body.String())
			}
		})
	}

	t.Run("a valid ?spec= reaches the loader in canonical form", func(t *testing.T) {
		loader := &countingReadinessLoader{snap: snap}
		h := NewHandlerWith(t.TempDir(), Deps{ReadinessLoader: loader})
		if rec := get(t, h, "/readiness?spec=pilot"); rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
		}
		parsed, err := artifact.ParseRef("spec/pilot")
		if err != nil {
			t.Fatal(err)
		}
		if len(loader.refs) != 1 || loader.refs[0] != parsed.String() {
			t.Fatalf("loader.refs = %v, want exactly the parsed ref's own canonical form [%q]", loader.refs, parsed.String())
		}
	})

	t.Run("no loader keeps the existing 503 disclosure unchanged", func(t *testing.T) {
		h := NewHandlerWith(t.TempDir(), Deps{ReadinessDefaultSpec: "spec/pilot"})
		rec := get(t, h, "/readiness")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, stdhtml.EscapeString(errReadinessNotWired.Error())) {
			t.Fatalf("503 page does not carry the no-loader disclosure verbatim: %s", body)
		}
		if strings.Contains(body, "no spec was named") {
			t.Fatalf("no-loader 503 page wrongly blames a missing spec name: %s", body)
		}
	})

	// The check order is a contract: loader-nil is tested before ref-empty,
	// so a process with neither a loader nor a named spec reports the
	// not-wired disclosure, never the no-spec one. There is no loader, so
	// there is no call counter to read; the body alone pins the order.
	t.Run("a nil loader with no spec named is the not-wired 503", func(t *testing.T) {
		h := NewHandlerWith(t.TempDir(), Deps{})
		rec := get(t, h, "/readiness")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503 (body: %s)", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, stdhtml.EscapeString(errReadinessNotWired.Error())) {
			t.Fatalf("503 page does not carry the no-loader disclosure verbatim: %s", body)
		}
		if strings.Contains(body, "no spec was named") {
			t.Fatalf("nil loader with no spec named must report not-wired first, not the missing spec: %s", body)
		}
	})

	// co-6: a loader that IS wired but has nothing to derive — no ?spec=
	// and no default — is a different missing fact from "no loader", and
	// the disclosure must say which one it is and how to supply it.
	t.Run("a wired loader with no spec named is a 503 naming the missing spec", func(t *testing.T) {
		loader := &countingReadinessLoader{snap: snap}
		h := NewHandlerWith(t.TempDir(), Deps{ReadinessLoader: loader})
		rec := get(t, h, "/readiness")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503 (body: %s)", rec.Code, rec.Body.String())
		}
		if loader.calls != 0 {
			t.Fatalf("loader.calls = %d, want 0 (nothing was named, so nothing is derived)", loader.calls)
		}
		body := rec.Body.String()
		if !strings.Contains(body, stdhtml.EscapeString(errReadinessNoSpec.Error())) {
			t.Fatalf("503 page does not carry the no-spec disclosure verbatim: %s", body)
		}
		for _, want := range []string{"no spec was named", "?spec=", "--context-request"} {
			if !strings.Contains(body, stdhtml.EscapeString(want)) {
				t.Fatalf("503 page does not tell the author how to name a spec (%q): %s", want, body)
			}
		}
		if strings.Contains(body, "without the readiness pilot wired") {
			t.Fatalf("no-spec 503 page wrongly claims no loader is wired: %s", body)
		}
	})
}

// countingReadinessLoader is a test-only ReadinessLoader recording every
// ref it was asked to Load, in order, and always returning the same fixed
// snapshot.
type countingReadinessLoader struct {
	snap  readinesspilot.Snapshot
	calls int
	refs  []string
}

func (c *countingReadinessLoader) Load(_ context.Context, ref string) (readinesspilot.Snapshot, error) {
	c.calls++
	c.refs = append(c.refs, ref)
	return c.snap, nil
}

func TestReadinessRoute_WrongMethod405(t *testing.T) {
	snap := readinessFixture()
	h := NewHandlerWith(t.TempDir(), Deps{ReadinessLoader: fixedSnapshotLoader{snap: snap}, ReadinessDefaultSpec: snap.TargetRef})
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

// readinessStyleBlocks are the stylesheet's two readiness regions: the
// shared pilot cockpit block, which the wall and the bar reuse, and the
// readiness page's own workbench-only block (spec/readiness-page-v2),
// which hangs off the standalone page's hook alone.
func readinessStyleBlocks(t *testing.T) map[string]string {
	t.Helper()
	css, err := dex.StyleCSS()
	if err != nil {
		t.Fatalf("dex.StyleCSS: %v", err)
	}
	s := string(css)
	return map[string]string{
		"cockpit": sectionOf(t, s, "the readiness pilot cockpit", "Syntax-highlighting palettes"),
		"page":    sectionOf(t, s, "The readiness page in the design's layout", "/* verdi:workbench-only:end */"),
	}
}

func TestReadinessStyle_NarrowAnchorsAndWrapping(t *testing.T) {
	for name, s := range readinessStyleBlocks(t) {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(s, "scroll-margin-top") {
				t.Fatal("block has no scroll offset for its fragment targets")
			}
			if !strings.Contains(s, "overflow-wrap") {
				t.Fatal("block has no wrapping rule for long values")
			}
			for _, forbidden := range []string{"text-overflow", "overflow: hidden", "overflow-x: hidden", "white-space: nowrap"} {
				if strings.Contains(s, forbidden) {
					t.Fatalf("block truncates content (%q) instead of wrapping it", forbidden)
				}
			}
		})
	}
}

func TestReadinessStyle_PageRulesScopedToTheStandaloneHook(t *testing.T) {
	page := readinessStyleBlocks(t)["page"]
	for _, want := range []string{
		".readiness-standalone .readiness-rail-list", ".readiness-standalone .readiness-station-line",
		".readiness-standalone .readiness-class-chip", ".readiness-standalone .readiness-order",
		".readiness-standalone .readiness-columns", ".readiness-standalone .readiness-known-link",
		".readiness-standalone .readiness-human-review", ".readiness-standalone .readiness-when",
		".readiness-standalone .readiness-more-summary", ".topbar-controls .readiness-wall-link",
		"@media (max-width: 720px)", "@media (max-width: 480px)",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("readiness page block is missing rule %q", want)
		}
	}
	// Every selector in the block carries the standalone hook or the bar
	// link's own class: the shared readiness-* base rules stay the wall's.
	for _, line := range strings.Split(page, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "@media") || strings.HasPrefix(trimmed, "}") || !strings.Contains(trimmed, "{") {
			continue
		}
		selectors := strings.TrimSpace(trimmed[:strings.Index(trimmed, "{")])
		for _, sel := range strings.Split(selectors, ",") {
			sel = strings.TrimSpace(sel)
			if sel == "" {
				continue
			}
			if !strings.Contains(sel, ".readiness-standalone") && !strings.Contains(sel, ".readiness-wall-link") {
				t.Fatalf("selector %q is not scoped to the standalone readiness page", sel)
			}
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
