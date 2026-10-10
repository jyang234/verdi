package workbench

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/readinesspilot"
)

// The wall's concern cards (spec/spec-documents ac-12, R-W4-7), which the
// record drawer's Readiness tab renders since the wall shell retired
// (SI-368 (24)(a), (32) T2): the guidance sentence is the primary line
// when the row carries one, otherwise the fact; the fact, concern id,
// timing and blocking flag are filed in the technical disclosure, where
// every row carries its Timing; an unresolved row's step label carries
// its "now" or "later" mark and the article its data-timing, and a
// proven row carries neither.

// concernCardSnapshot is a snapshot whose current step is Define the
// work: one violated row there with its guidance, one unresolved row in
// Get approval waiting on it, and one proven row.
func concernCardSnapshot() readinesspilot.Snapshot {
	current := readinesspilot.Concern{
		ID: "shape/question/oq-1", Area: readinesspilot.AreaShape, State: readinesspilot.StateViolated, Blocking: true,
		Timing: readinesspilot.TimingCurrent, Summary: "Declared open question remains unresolved",
		Guidance:  "Resolve it on the wall: edit or remove oq-1, or graduate a decision that answers it.",
		Witnesses: []string{"oq-1"}, Destination: readinesspilot.Destination{BoardPath: "/board/spec/x"},
	}
	later := readinesspilot.Concern{
		ID: "review/action", Area: readinesspilot.AreaReview, State: readinesspilot.StateUnproven, Blocking: true,
		Timing: readinesspilot.TimingCurrent, Summary: "Lifecycle and safe-action posture can advance review",
		Guidance:  "Establish the facts the witnesses name, so verdi journey can offer a safe review action.",
		Witnesses: []string{"safe review action is unavailable"}, Destination: readinesspilot.Destination{CLI: []string{"verdi", "journey"}},
	}
	proven := readinesspilot.Concern{
		ID: "shape/problem", Area: readinesspilot.AreaShape, State: readinesspilot.StateProven, Blocking: true,
		Timing: readinesspilot.TimingCurrent, Summary: "Problem statement is present", Witnesses: []string{},
		Destination: readinesspilot.Destination{CLI: []string{}},
	}
	return readinesspilot.Snapshot{
		TargetRef: "spec/x", TargetTitle: "X", TargetClass: "feature", Branch: "design/x", Head: readinessFixtureHead,
		Areas: []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateViolated},
			{ID: readinesspilot.AreaSuccess, Label: "Define success", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaContext, Label: "Check constraints", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaReview, Label: "Get approval", State: readinesspilot.StateUnproven},
		},
		CurrentFocus: readinesspilot.AreaShape,
		Attention:    []readinesspilot.Concern{current, later},
		AllConcerns:  []readinesspilot.Concern{proven, current, later},
		StaleNotice:  "Derived at HEAD " + readinessFixtureHead + " for this request.",
	}
}

func TestWriteASDConcern_GuidanceIsPrimaryFactIsSecondary(t *testing.T) {
	tab := renderReadinessTab(nil, concernCardSnapshot(), nil)
	row := concernRow(t, tab, "shape/question/oq-1")
	primary := `<div class="readiness-copy"><div class="readiness-primary"><p class="readiness-summary readiness-guidance">Resolve it on the wall: edit or remove oq-1, or graduate a decision that answers it.</p></div>`
	fact := `<dt>Fact</dt><dd class="readiness-fact">Declared open question remains unresolved</dd>`
	if !strings.Contains(row, primary) || !strings.Contains(row, fact) || strings.Index(row, primary) > strings.Index(row, fact) {
		t.Fatalf("guidance must lead and the fact follow, filed:\n%s", row)
	}
	if !strings.Contains(row, `data-timing="now"`) || !strings.Contains(row, `<dt>Timing</dt><dd><code>current</code></dd>`) ||
		!strings.Contains(row, `<p class="readiness-stage">Define the work <span class="readiness-when readiness-when--now">now</span></p>`) {
		t.Fatalf("timing must ride the disclosure, the step label and the data attribute:\n%s", row)
	}
	for _, want := range []string{
		`<dt>Concern</dt><dd><code>shape/question/oq-1</code></dd>`,
		`<dt>Blocking</dt><dd><code>true</code></dd>`,
		`<span class="readiness-state readiness-state--violated-with-witness">Needs attention</span>`,
	} {
		if !strings.Contains(row, want) {
			t.Fatalf("missing %q:\n%s", want, row)
		}
	}
}

func TestWriteASDConcern_NoGuidanceShowsSummaryAsPrimaryAndLaterTiming(t *testing.T) {
	tab := renderReadinessTab(nil, concernCardSnapshot(), nil)
	// A row without guidance — a proven one, since every unresolved row
	// carries its guidance (SI-338 (1)) — leads with its fact.
	proven := concernRow(t, tab, "shape/problem")
	if !strings.Contains(proven, `<div class="readiness-primary"><p class="readiness-summary">Problem statement is present</p></div>`) || strings.Contains(proven, "readiness-guidance") {
		t.Fatalf("the fact must be the primary line when there is no guidance:\n%s", proven)
	}
	// A proven row is marked neither now nor later; its Timing is filed in
	// the disclosure like every row's (SI-368 (24)(a), (32) T2).
	if strings.Contains(proven, "data-timing") || strings.Contains(proven, "readiness-when") || !strings.Contains(proven, `<dt>Timing</dt><dd><code>current</code></dd>`) {
		t.Fatalf("a proven row carries no timing mark, and its Timing is filed:\n%s", proven)
	}
	later := concernRow(t, tab, "review/action")
	if !strings.Contains(later, `data-timing="later"`) || !strings.Contains(later, `<span class="readiness-when readiness-when--later">later — waits on Define the work</span>`) {
		t.Fatalf("later timing:\n%s", later)
	}
}
