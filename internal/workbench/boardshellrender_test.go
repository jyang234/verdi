package workbench

import (
	"strings"
	"testing"
)

// The wall's concern cards (spec/spec-documents ac-12, R-W4-7): the
// guidance sentence is the primary line when the row carries one, the
// fact stays visible as a secondary line, and timing lives in the
// technical disclosure (plus data-timing on the article) rather than
// inline on the stage line. Proven rows carry no timing at all.

func TestWriteASDConcern_GuidanceIsPrimaryFactIsSecondary(t *testing.T) {
	asd := &asdView{Shell: asdShell{CurrentFocus: asdAreaSuccess}}
	var b strings.Builder
	writeASDConcern(&b, asdConcern{ID: "success/ac", Area: asdAreaSuccess, State: asdStateViolated, Blocking: true, Summary: "No acceptance criteria are declared.", Guidance: "Declare what must be true when this lands.", Witnesses: []string{"w"}}, asd, 1)
	html := b.String()
	primary := `<p class="readiness-summary" data-testid="asd-guidance-success/ac">Declare what must be true when this lands.</p>`
	fact := `<p class="asd-fact" data-testid="asd-fact-success/ac">No acceptance criteria are declared.</p>`
	if !strings.Contains(html, primary) || !strings.Contains(html, fact) || strings.Index(html, primary) > strings.Index(html, fact) {
		t.Fatalf("guidance must lead and the fact follow:\n%s", html)
	}
	if strings.Contains(html, "asd-timing") || !strings.Contains(html, `data-timing="now"`) || !strings.Contains(html, `<dt>Timing</dt><dd><code>now</code></dd>`) {
		t.Fatalf("timing must live in the disclosure and the data attribute:\n%s", html)
	}
	for _, want := range []string{`<dt>Concern</dt><dd><code>success/ac</code></dd>`, `<dt>Blocking</dt><dd><code>true</code></dd>`, `<span class="readiness-state readiness-state--violated-with-witness">Needs attention</span>`, `<p class="readiness-stage">Define success</p>`} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q:\n%s", want, html)
		}
	}
}

func TestWriteASDConcern_NoGuidanceShowsSummaryAsPrimaryAndLaterTiming(t *testing.T) {
	asd := &asdView{Shell: asdShell{CurrentFocus: asdAreaShape}}
	var b strings.Builder
	writeASDConcern(&b, asdConcern{ID: "review/x", Area: asdAreaReview, State: asdStateUnproven, Summary: "Human review has not accepted this proposal yet."}, asd, 2)
	html := b.String()
	if !strings.Contains(html, `<p class="readiness-summary" data-testid="asd-summary-review/x">Human review has not accepted this proposal yet.</p>`) || strings.Contains(html, "asd-fact") {
		t.Fatalf("summary must be the primary line when there is no guidance:\n%s", html)
	}
	if !strings.Contains(html, `data-timing="later"`) || !strings.Contains(html, `<dt>Timing</dt><dd><code>later — waits on Define the work</code></dd>`) {
		t.Fatalf("later timing:\n%s", html)
	}
	// A proven row carries no timing at all.
	b.Reset()
	writeASDConcern(&b, asdConcern{ID: "shape/problem", Area: asdAreaShape, State: asdStateProven, Summary: "The problem statement is present."}, asd, 0)
	if strings.Contains(b.String(), "Timing") || strings.Contains(b.String(), "data-timing") {
		t.Fatalf("proven rows carry no timing:\n%s", b.String())
	}
}
