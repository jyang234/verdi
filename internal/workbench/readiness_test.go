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

// readinessFixture is the canonical mixed-state cockpit fixture: one
// proven, one violated-with-witness, and unproven concerns spread over
// the four areas, exercising both destination kinds and both timings.
func readinessFixture() readinesspilot.Snapshot {
	return readinesspilot.Snapshot{
		TargetRef:     "spec/pilot",
		TargetClass:   "story",
		Branch:        "design/pilot",
		Head:          readinessFixtureHead,
		RequestDigest: "sha256:" + strings.Repeat("ab", 32),
		Areas: []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "Shape proposal", State: readinesspilot.StateUnproven},
			{ID: readinesspilot.AreaSuccess, Label: "Show success", State: readinesspilot.StateViolated},
			{ID: readinesspilot.AreaContext, Label: "Check context", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaReview, Label: "Request review", State: readinesspilot.StateUnproven},
		},
		CurrentFocus: readinesspilot.AreaShape,
		Attention: []readinesspilot.Concern{
			readinessConcernCoverage(),
			readinessConcernQuestion(),
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

// readinessAllProvenFixture is the fully proven variant: an empty
// attention queue and no current-focus area.
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
		TargetClass:   "story",
		Branch:        "design/pilot",
		Head:          readinessFixtureHead,
		RequestDigest: "sha256:" + strings.Repeat("ab", 32),
		Areas: []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "Shape proposal", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaSuccess, Label: "Show success", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaContext, Label: "Check context", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaReview, Label: "Request review", State: readinesspilot.StateProven},
		},
		CurrentFocus: "",
		Attention:    []readinesspilot.Concern{},
		AllConcerns:  []readinesspilot.Concern{problem, contributor, verdict, action},
		StaleNotice:  "Startup snapshot at " + readinessFixtureHead + "; restart verdi serve after an edit.",
	}
}

// TestReadinessFixture_ContractValid pins both fixtures to the committed
// Task 1 contract: a fixture Validate() rejects would make every
// assertion below untrustworthy.
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
// the next top-level section marker, so assertions can scope themselves
// to one region of the page.
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

func TestReadinessRender_RailOrderAndStates(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	rail := sectionOf(t, html, `<nav class="readiness-rail"`, `</nav>`)

	prev := -1
	for _, area := range []string{"shape-proposal", "show-success", "check-context", "request-review"} {
		idx := strings.Index(rail, `data-area-id="`+area+`"`)
		if idx < 0 {
			t.Fatalf("rail is missing station %q:\n%s", area, rail)
		}
		if idx < prev {
			t.Fatalf("rail station %q is out of the fixed four-area order", area)
		}
		prev = idx
	}
	for _, label := range []string{"Shape proposal", "Show success", "Check context", "Request review"} {
		if !strings.Contains(rail, label) {
			t.Fatalf("rail is missing area label %q", label)
		}
	}
	// Exact per-area state words, matched inside each station's slice.
	stations := map[string]string{
		"shape-proposal": "unproven", "show-success": "violated-with-witness",
		"check-context": "proven", "request-review": "unproven",
	}
	for area, state := range stations {
		station := sectionOf(t, rail, `data-area-id="`+area+`"`, `</li>`)
		if !strings.Contains(station, `>`+state+`<`) {
			t.Fatalf("station %q does not carry exact state label %q:\n%s", area, state, station)
		}
	}
}

func TestReadinessRender_CurrentFocusMarker(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	rail := sectionOf(t, html, `<nav class="readiness-rail"`, `</nav>`)

	if got := strings.Count(rail, "current focus"); got != 1 {
		t.Fatalf("rail carries %d current-focus markers, want exactly 1", got)
	}
	focusStation := sectionOf(t, rail, `data-area-id="shape-proposal"`, `</li>`)
	if !strings.Contains(focusStation, "current focus") {
		t.Fatalf("current-focus marker is not on the snapshot's focus area:\n%s", focusStation)
	}
	if !strings.Contains(focusStation, `aria-current`) {
		t.Fatalf("focus station is missing aria-current:\n%s", focusStation)
	}
}

func TestReadinessRender_AttentionQueueOrderAndPrimacy(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	queue := sectionOf(t, html, `<section class="readiness-queue"`, `</section>`)

	want := []string{
		"success/blocker/obligation-quality/coverage",
		"shape/question/q-alpha",
		"review/action",
		"review/blocker/gov-signoff",
	}
	prev := -1
	for _, id := range want {
		idx := strings.Index(queue, `data-concern-id="`+id+`"`)
		if idx < 0 {
			t.Fatalf("attention queue is missing concern %q:\n%s", id, queue)
		}
		if idx < prev {
			t.Fatalf("attention queue concern %q is out of snapshot order", id)
		}
		prev = idx
	}
	if got := strings.Count(queue, `data-concern-id="`); got != len(want) {
		t.Fatalf("attention queue lists %d concerns, want exactly %d", got, len(want))
	}
	// The queue renders BEFORE the complete section and as an ordered
	// list — its position is the deterministic comparator's order.
	if !strings.Contains(queue, "<ol") {
		t.Fatalf("attention queue is not an ordered list:\n%s", queue)
	}
	if strings.Index(html, `<section class="readiness-queue"`) > strings.Index(html, `<section class="readiness-all"`) {
		t.Fatal("attention queue does not precede the complete concerns section")
	}
}

func TestReadinessRender_WorkClassLabels(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	queue := sectionOf(t, html, `<section class="readiness-queue"`, `</section>`)

	coverage := sectionOf(t, queue, `data-concern-id="success/blocker/obligation-quality/coverage"`, `</article>`)
	if !strings.Contains(coverage, `>mechanical<`) {
		t.Fatalf("journey-derived concern is missing its source-provided work-class label:\n%s", coverage)
	}
	signoff := sectionOf(t, queue, `data-concern-id="review/blocker/gov-signoff"`, `</article>`)
	if !strings.Contains(signoff, `>governance<`) {
		t.Fatalf("eventual concern is missing its source-provided work-class label:\n%s", signoff)
	}
	// A concern whose source supplies no work class renders NO work-class
	// chip at all — omitted, never guessed.
	question := sectionOf(t, queue, `data-concern-id="shape/question/q-alpha"`, `</article>`)
	if strings.Contains(question, "readiness-workclass") {
		t.Fatalf("concern without a source work class renders a work-class chip:\n%s", question)
	}
}

func TestReadinessRender_AllConcernsComplete(t *testing.T) {
	snap := readinessFixture()
	html := renderReadinessFixture(t, snap)
	all := sectionOf(t, html, `<section class="readiness-all"`, "</main>")

	for _, concern := range snap.AllConcerns {
		if !strings.Contains(all, `data-concern-id="`+concern.ID+`"`) {
			t.Fatalf("complete section is missing concern %q", concern.ID)
		}
	}
	if got := strings.Count(all, `data-concern-id="`); got != len(snap.AllConcerns) {
		t.Fatalf("complete section lists %d concerns, want all %d", got, len(snap.AllConcerns))
	}
	// Proven facts stay visible: the proven concerns render with their
	// exact state word, never hidden or collapsed away.
	for _, id := range []string{"shape/problem", "context/verdict"} {
		row := sectionOf(t, all, `data-concern-id="`+id+`"`, `</article>`)
		if !strings.Contains(row, `>proven<`) {
			t.Fatalf("proven concern %q does not carry its exact state label:\n%s", id, row)
		}
	}
	// No collapsed remainder: no disclosure widget and no bare hidden
	// attribute (aria-hidden on a decorative glyph is fine — it hides
	// nothing from sighted readers).
	if strings.Contains(all, "<details") || strings.Contains(all, " hidden>") || strings.Contains(all, ` hidden `) {
		t.Fatalf("complete section hides content behind a disclosure widget:\n%s", all)
	}
}

func TestReadinessRender_StateLabelsNeverColorAlone(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	// Every state chip carries the exact state word as TEXT — the three
	// exact labels all appear, and each chip class is accompanied by its
	// word so color is never the only signal.
	for _, state := range []string{"proven", "violated-with-witness", "unproven"} {
		if !strings.Contains(html, `>`+state+`<`) {
			t.Fatalf("page never renders exact state label %q as text", state)
		}
		if strings.Count(html, `readiness-state--`+state) != strings.Count(html, `readiness-state readiness-state--`+state) {
			t.Fatalf("state chip class %q appears outside a labelled chip", state)
		}
	}
}

func TestReadinessRender_WitnessesExact(t *testing.T) {
	snap := readinessFixture()
	html := renderReadinessFixture(t, snap)
	for _, concern := range snap.AllConcerns {
		for _, witness := range concern.Witnesses {
			if !strings.Contains(html, witness) {
				t.Fatalf("witness %q of %q is not rendered verbatim", witness, concern.ID)
			}
		}
	}
}

func TestReadinessRender_DestinationExclusivity(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	queue := sectionOf(t, html, `<section class="readiness-queue"`, `</section>`)

	question := sectionOf(t, queue, `data-concern-id="shape/question/q-alpha"`, `</article>`)
	if !strings.Contains(question, `class="readiness-board-link"`) {
		t.Fatalf("board-destination concern has no board link:\n%s", question)
	}
	if strings.Contains(question, "readiness-cli-token") {
		t.Fatalf("board-destination concern also renders CLI tokens:\n%s", question)
	}

	coverage := sectionOf(t, queue, `data-concern-id="success/blocker/obligation-quality/coverage"`, `</article>`)
	if strings.Contains(coverage, "readiness-board-link") {
		t.Fatalf("CLI-destination concern also renders a board link:\n%s", coverage)
	}

	all := sectionOf(t, html, `<section class="readiness-all"`, "</main>")
	proven := sectionOf(t, all, `data-concern-id="shape/problem"`, `</article>`)
	if strings.Contains(proven, "readiness-board-link") || strings.Contains(proven, "readiness-cli-token") {
		t.Fatalf("proven concern renders a destination:\n%s", proven)
	}
}

func TestReadinessRender_BoardLinkTargetRelAttrs(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	link := sectionOf(t, html, `class="readiness-board-link"`, `</a>`)
	if !strings.Contains(link, `href="/board/spec/pilot"`) {
		t.Fatalf("board link href is not the snapshot's exact board path:\n%s", link)
	}
	if !strings.Contains(link, `target="_blank"`) {
		t.Fatalf("board link is missing target=_blank:\n%s", link)
	}
	if !strings.Contains(link, `rel="noopener"`) {
		t.Fatalf("board link is missing rel=noopener:\n%s", link)
	}
}

func TestReadinessRender_CLITokenVector(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	queue := sectionOf(t, html, `<section class="readiness-queue"`, `</section>`)
	coverage := sectionOf(t, queue, `data-concern-id="success/blocker/obligation-quality/coverage"`, `</article>`)

	tokens := []string{"verdi", "gate", "run", "--target", "spec/pilot"}
	prev := -1
	for _, token := range tokens {
		marker := `<code class="readiness-cli-token">` + token + `</code>`
		idx := strings.Index(coverage, marker)
		if idx < 0 {
			t.Fatalf("CLI destination is missing token element %q:\n%s", token, coverage)
		}
		if idx < prev {
			t.Fatalf("CLI token %q is out of vector order", token)
		}
		prev = idx
	}
	if got := strings.Count(coverage, "readiness-cli-token"); got != len(tokens) {
		t.Fatalf("CLI destination renders %d token elements, want exactly %d", got, len(tokens))
	}
	// Never a shell string: the joined command must not appear as one run
	// of text anywhere on the page.
	if strings.Contains(html, "verdi gate run --target spec/pilot") {
		t.Fatal("CLI destination is rendered as a joined shell string")
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
	snap.AllConcerns[2].Summary = `<script>alert(1)</script>`
	snap.Attention[0].Summary = `<script>alert(1)</script>`
	snap.AllConcerns[2].Witnesses = []string{`"><img src=x onerror=alert(2)>`}
	snap.Attention[0].Witnesses = []string{`"><img src=x onerror=alert(2)>`}
	html := renderReadinessFixture(t, snap)
	if strings.Contains(html, "<script>alert(1)") {
		t.Fatal("summary text is not HTML-escaped")
	}
	if strings.Contains(html, "<img src=x") {
		t.Fatal("witness text is not HTML-escaped")
	}
}

func TestReadinessRender_AllProvenEmptyQueue(t *testing.T) {
	html := renderReadinessFixture(t, readinessAllProvenFixture())
	queue := sectionOf(t, html, `<section class="readiness-queue"`, `</section>`)
	if strings.Contains(queue, `data-concern-id=`) {
		t.Fatalf("all-proven snapshot still lists queue concerns:\n%s", queue)
	}
	if !strings.Contains(queue, "every concern is proven") {
		t.Fatalf("empty queue does not state its honest reason:\n%s", queue)
	}
	if strings.Contains(html, "current focus") {
		t.Fatal("all-proven snapshot renders a current-focus marker")
	}
	// The complete section still shows all proven facts.
	all := sectionOf(t, html, `<section class="readiness-all"`, "</main>")
	if got := strings.Count(all, `data-concern-id="`); got != 4 {
		t.Fatalf("complete section lists %d proven concerns, want 4", got)
	}
}

func TestReadinessRender_KeyboardLandmarksAndScript(t *testing.T) {
	html := renderReadinessFixture(t, readinessFixture())
	for _, want := range []string{
		`<nav class="readiness-rail" aria-label="Readiness rail">`,
		`aria-label="Attention queue"`,
		`aria-label="All concerns"`,
		`href="#area-shape-proposal"`,
		`id="area-shape-proposal"`,
		`id="concern-shape/question/q-alpha"`,
		// The CLI fallback is keyboard-reachable (a Task 4 browser test
		// exposed that a keyboard-only author could not reach it to copy).
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

// TestReadinessAsset_JSMiddleClickInstrumented pins fix-round finding 1:
// middle-button activation of a board link (auxclick, button 1) must
// share the primary click's synchronous board-link recorder, so the
// _blank navigation is never un-instrumented.
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

// TestReadinessStyle_NarrowAnchorsAndWrapping pins fix-round findings 2
// and 3: fragment targets carry a narrow-mode scroll offset so the
// pinned rail never obscures them, and long valid snapshot values wrap
// instead of overflowing or truncating.
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
		".readiness-rail", ".readiness-queue", ".readiness-all",
		".readiness-stale", ".readiness-cli-token",
		"prefers-reduced-motion",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("stylesheet is missing cockpit rule %q", want)
		}
	}
}
