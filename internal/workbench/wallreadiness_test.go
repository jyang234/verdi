package workbench

import (
	"context"
	stdhtml "html"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/gitx/readcensus"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/wtmanager"
)

// TestReadinessWallWords_KeepTheWallTriad (SI-368 (14); spec-documents
// ac-12): the Readiness tab speaks the wall's triad — the very words the
// wall shell's chips speak — and never the page's, and the page keeps its
// own (SI-339 (10)). The tab's count words read as a count of items, the
// verb agreeing with one item or more (SI-368 (24)(h)); the page's are
// unchanged.
func TestReadinessWallWords_KeepTheWallTriad(t *testing.T) {
	wall, page := readinessWallWords(), readinessPageWords()
	for _, tc := range []struct {
		state          readinesspilot.State
		wallLabel      string
		pageLabel      string
		wallCountWord  string
		wallCountMany  string
		pageCountWord  string
		shellPlainWord string
	}{
		{readinesspilot.StateProven, "Ready", "Proven", "ready", "ready", "proven", asdPlainState(asdStateProven)},
		{readinesspilot.StateViolated, "Needs attention", "Violated", "needs attention", "need attention", "violated", asdPlainState(asdStateViolated)},
		{readinesspilot.StateUnproven, "Not enough evidence yet", "Not enough evidence yet", "without enough evidence yet", "without enough evidence yet", "not enough evidence yet", asdPlainState(asdStateUnproven)},
	} {
		t.Run(string(tc.state), func(t *testing.T) {
			if got := wall.label(tc.state); got != tc.wallLabel || got != tc.shellPlainWord {
				t.Errorf("the tab's %s chip = %q, want the wall's %q (the shell speaks %q)", tc.state, got, tc.wallLabel, tc.shellPlainWord)
			}
			if got := page.label(tc.state); got != tc.pageLabel {
				t.Errorf("the page's %s chip = %q, want %q", tc.state, got, tc.pageLabel)
			}
			if got := wall.count(tc.state, 1); got != tc.wallCountWord {
				t.Errorf("the tab's %s count word for one item = %q, want %q", tc.state, got, tc.wallCountWord)
			}
			if got := wall.count(tc.state, 2); got != tc.wallCountMany {
				t.Errorf("the tab's %s count word for two items = %q, want %q", tc.state, got, tc.wallCountMany)
			}
			for _, n := range []int{1, 2} {
				if got := page.count(tc.state, n); got != tc.pageCountWord {
					t.Errorf("the page's %s count word for %d = %q, want %q", tc.state, n, got, tc.pageCountWord)
				}
			}
		})
	}
}

// tabConcern is one concern of the tab fixtures: an unresolved row with
// guidance, or a proven one without.
func tabConcern(id string, area readinesspilot.AreaID, state readinesspilot.State, object string) readinesspilot.Concern {
	c := readinesspilot.Concern{
		ID: id, Area: area, State: state, Blocking: true, Timing: readinesspilot.TimingCurrent,
		Summary: "fact of " + id, Object: object, Witnesses: []string{"witness of " + id},
		Destination: readinesspilot.Destination{CLI: []string{}},
	}
	if state != readinesspilot.StateProven {
		c.Guidance = "guidance for " + id
		c.Destination = readinesspilot.Destination{BoardPath: "/board/spec/" + marksWallName}
	}
	return c
}

// tabSnapshot is a readiness snapshot of the marks wall naming a concern
// of every target kind (SI-368 (16)): the problem (proven) and the
// outcome, an open question and a coverage row naming object cards, the
// digit-led stub's unreconciled row, the criteria row, and a row that
// names nothing on the wall (a CLI destination).
func tabSnapshot(head string) readinesspilot.Snapshot {
	problem := tabConcern("shape/problem", readinesspilot.AreaShape, readinesspilot.StateProven, "")
	outcome := tabConcern("shape/outcome", readinesspilot.AreaShape, readinesspilot.StateViolated, "")
	question := tabConcern("shape/question/oq-1", readinesspilot.AreaShape, readinesspilot.StateUnproven, "oq-1")
	criteria := tabConcern("success/criteria", readinesspilot.AreaSuccess, readinesspilot.StateProven, "")
	coverage := tabConcern("success/coverage/ac-2", readinesspilot.AreaSuccess, readinesspilot.StateUnproven, "ac-2")
	stub := tabConcern("review/blocker/stub-unreconciled/s-2fa-x", readinesspilot.AreaReview, readinesspilot.StateViolated, "")
	action := tabConcern("review/action", readinesspilot.AreaReview, readinesspilot.StateUnproven, "")
	action.Destination = readinesspilot.Destination{CLI: []string{"verdi", "journey", "--target", "spec/" + marksWallName}}
	return readinesspilot.Snapshot{
		TargetRef: "spec/" + marksWallName, TargetTitle: "Marks wall", TargetClass: "feature",
		Branch: "design/" + marksWallName, Head: head, BoardPath: "/board/spec/" + marksWallName,
		RequestDigest: "sha256:" + strings.Repeat("cd", 32),
		Areas: []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateViolated},
			{ID: readinesspilot.AreaSuccess, Label: "Define success", State: readinesspilot.StateUnproven},
			{ID: readinesspilot.AreaContext, Label: "Check constraints", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaReview, Label: "Get approval", State: readinesspilot.StateViolated},
		},
		CurrentFocus: readinesspilot.AreaShape,
		Attention:    []readinesspilot.Concern{outcome, question, coverage, stub, action},
		AllConcerns:  []readinesspilot.Concern{problem, outcome, question, criteria, coverage, stub, action},
		StaleNotice:  "Derived at HEAD " + head + " for this request.",
	}
}

// tabSnapshotTargets is every tabSnapshot concern's wall target on the
// marks wall; review/action names nothing.
func tabSnapshotTargets() map[string]readinessTarget {
	return map[string]readinessTarget{
		"shape/problem":                            {Kind: readinessTargetStrip, Value: "problem"},
		"shape/outcome":                            {Kind: readinessTargetStrip, Value: "outcome"},
		"shape/question/oq-1":                      {Kind: readinessTargetObject, Value: "oq-1"},
		"success/criteria":                         {Kind: readinessTargetSlot, Value: string(boardlayout.ZoneAC)},
		"success/coverage/ac-2":                    {Kind: readinessTargetObject, Value: "ac-2"},
		"review/blocker/stub-unreconciled/s-2fa-x": {Kind: readinessTargetStub, Value: "2fa-x"},
	}
}

// TestReadinessTargets is SI-368 (16)'s table: an object concern selects
// its card when the card is on the wall; a stub concern selects the stub
// card journey's slug rule maps onto it (the marks' rule, SI-360 (1)),
// and none when two stubs share it (SI-362 (3)); the problem, outcome
// and criteria rows select their strip half or the criteria slot; every
// other concern selects nothing.
func TestReadinessTargets(t *testing.T) {
	objects := map[string]bool{"ac-1": true, "ac-2": true, "oq-1": true}
	for _, tc := range []struct {
		name    string
		concern readinesspilot.Concern
		stubs   []string
		want    *readinessTarget
	}{
		{"an object on the wall", readinesspilot.Concern{ID: "success/coverage/ac-2", Object: "ac-2"}, nil, &readinessTarget{readinessTargetObject, "ac-2"}},
		{"an object the wall does not show", readinesspilot.Concern{ID: "success/coverage/ac-9", Object: "ac-9"}, nil, nil},
		{"a stub by journey's slug rule", readinesspilot.Concern{ID: "review/blocker/stub-unreconciled/s-2fa-x"}, []string{"2fa-x", "plain"}, &readinessTarget{readinessTargetStub, "2fa-x"}},
		{"a plain stub", readinesspilot.Concern{ID: "review/blocker/stub-unreconciled/plain"}, []string{"2fa-x", "plain"}, &readinessTarget{readinessTargetStub, "plain"}},
		{"two stubs sharing one concern", readinesspilot.Concern{ID: "review/blocker/stub-unreconciled/s-2fa-x"}, []string{"2fa-x", "s-2fa-x"}, nil},
		{"a stub concern no card carries", readinesspilot.Concern{ID: "review/blocker/stub-unreconciled/gone"}, []string{"plain"}, nil},
		{"the problem", readinesspilot.Concern{ID: "shape/problem"}, nil, &readinessTarget{readinessTargetStrip, "problem"}},
		{"the outcome", readinesspilot.Concern{ID: "shape/outcome"}, nil, &readinessTarget{readinessTargetStrip, "outcome"}},
		{"the criteria", readinesspilot.Concern{ID: "success/criteria"}, nil, &readinessTarget{readinessTargetSlot, string(boardlayout.ZoneAC)}},
		{"a row naming nothing", readinesspilot.Concern{ID: "review/action"}, nil, nil},
		{"a policy row", readinesspilot.Concern{ID: "context/verdict"}, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snap := readinesspilot.Snapshot{AllConcerns: []readinesspilot.Concern{tc.concern}}
			got, ok := readinessTargets(snap, objects, tc.stubs, map[string]bool{"problem": true, "outcome": true})[tc.concern.ID]
			switch {
			case tc.want == nil && ok:
				t.Fatalf("target = %+v, want none", got)
			case tc.want != nil && (!ok || got != *tc.want):
				t.Fatalf("target = %+v (%v), want %+v", got, ok, *tc.want)
			}
		})
	}
}

// TestReadinessTargets_StripHalfOnTheWall (SI-368 (16), (28)(a); F3BR-2):
// the problem and outcome rows target their half of the case-file strip
// only where the wall renders that half, and are plain rows otherwise, so
// a click never shuts the drawer onto nothing. shape/problem is
// unresolved exactly when the spec lacks a problem, the very case whose
// half is gone. Each case is checked against the strip the region itself
// renders for that spec: a half is targeted exactly when it is drawn.
func TestReadinessTargets_StripHalfOnTheWall(t *testing.T) {
	stated := func(text, anchor string) *artifact.Attribute {
		return &artifact.Attribute{Text: text, Anchor: anchor}
	}
	for _, tc := range []struct {
		name             string
		problem, outcome *artifact.Attribute
		want             map[string]bool // the halves the problem and outcome rows target
	}{
		{"both stated", stated("p", "#problem"), stated("o", "#outcome"), map[string]bool{"problem": true, "outcome": true}},
		{"a spec that lacks a problem", nil, stated("o", "#outcome"), map[string]bool{"outcome": true}},
		{"a spec that lacks an outcome", stated("p", "#problem"), nil, map[string]bool{"problem": true}},
		{"an empty problem statement", stated("", "#problem"), stated("o", "#outcome"), map[string]bool{"outcome": true}},
		{"neither, so no strip", nil, nil, map[string]bool{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fm := &artifact.SpecFrontmatter{Class: artifact.ClassFeature, Problem: tc.problem, Outcome: tc.outcome}
			snap := readinesspilot.Snapshot{AllConcerns: []readinesspilot.Concern{{ID: "shape/problem"}, {ID: "shape/outcome"}}}
			targets := readinessTargets(snap, nil, nil, caseStripHalves(fm))

			p, err := buildProjectionFM("strip-halves", fm, nil, nil, nil, nil, modeAuthoring)
			if err != nil {
				t.Fatalf("buildProjection: %v", err)
			}
			var strip strings.Builder
			if p.Problem != "" || p.Outcome != "" { // the region's hasCaseFile
				writeCaseStrip(&strip, p, &asdView{}, true)
			}
			for _, half := range []string{"problem", "outcome"} {
				got, ok := targets["shape/"+half]
				switch {
				case tc.want[half] && (!ok || got != readinessTarget{readinessTargetStrip, half}):
					t.Errorf("shape/%s targets %+v (%v), want the strip's %s half", half, got, ok, half)
				case !tc.want[half] && ok:
					t.Errorf("shape/%s targets %+v, want a plain row: the wall draws no %s half", half, got, half)
				}
				if drawn := strings.Contains(strip.String(), `data-testid="placard-`+half+`"`); drawn != ok {
					t.Errorf("the %s half is drawn=%v but targeted=%v", half, drawn, ok)
				}
			}
		})
	}
}

// chipLabels is every plain state chip's label in html, in order.
func chipLabels(html string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`<span class="readiness-state readiness-state--[a-z-]+">([^<]*)</span>`).FindAllStringSubmatch(html, -1) {
		out = append(out, m[1])
	}
	return out
}

// requireWords asserts every chip in html speaks one of words' labels
// and none of other's that words does not share.
func requireWords(t *testing.T, label, html string, words, other readinessWords) {
	t.Helper()
	allowed := map[string]bool{}
	for _, s := range []readinesspilot.State{readinesspilot.StateProven, readinesspilot.StateViolated, readinesspilot.StateUnproven} {
		allowed[words.label(s)] = true
	}
	chips := chipLabels(html)
	if len(chips) == 0 {
		t.Fatalf("%s renders no state chip", label)
	}
	for _, chip := range chips {
		if !allowed[chip] {
			t.Errorf("%s renders the chip %q, outside its words", label, chip)
		}
	}
	for _, s := range []readinesspilot.State{readinesspilot.StateProven, readinesspilot.StateViolated} {
		if w := other.label(s); !allowed[w] && strings.Contains(html, ">"+w+"<") {
			t.Errorf("%s speaks the other surface's %q", label, w)
		}
	}
}

// TestRenderReadinessTab (SI-368 (1), (14), (16)): the tab is the page's
// body in the wall's words, scoped as the tab, with every concern exactly
// once carrying its wall target, the derivation stamp, and no new-tab
// board link or wall-address line; the page renders the same snapshot in
// its own words with no target.
func TestRenderReadinessTab(t *testing.T) {
	snap := tabSnapshot(readinessFixtureHead)
	targets := tabSnapshotTargets()
	tab := renderReadinessTab(nil, snap, targets)
	if !strings.HasPrefix(tab, `<div class="readiness-tab" data-testid="readiness-tab">`) {
		t.Fatalf("the tab's body is not scoped as the tab:\n%.200s", tab)
	}
	requireWords(t, "the tab", tab, readinessWallWords(), readinessPageWords())
	if !strings.Contains(tab, `<span class="readiness-station-line">1 needs attention, 1 without enough evidence yet, 1 ready — current focus</span>`) {
		t.Errorf("the tab's stepper does not count in the wall's words")
	}
	if line := readinessStepLine(readinessWallWords(), readinessStepCount{violated: 2, unproven: 3, proven: 2}, "current focus"); line != "2 need attention, 3 without enough evidence yet, 2 ready — current focus" {
		t.Errorf("the tab's stepper counts two items as %q, want them read as items", line)
	}
	if !strings.Contains(tab, stdhtml.EscapeString(snap.StaleNotice)) || !strings.Contains(tab, `data-readiness-stale="1"`) {
		t.Error("the tab lacks the per-request derivation stamp")
	}
	if !strings.Contains(tab, `<p class="readiness-purpose">This tab derives readiness for this wall on every request.</p>`) {
		t.Error("the tab's purpose line is not the tab's")
	}
	for _, never := range []string{`readiness-board-link`, `target="_blank"`, `readiness-wall-absent`, `readiness-standalone`, `This page derives`} {
		if strings.Contains(tab, never) {
			t.Errorf("the tab carries %s", never)
		}
	}
	if !strings.Contains(tab, `<code class="readiness-cli-token">journey</code>`) {
		t.Error("the tab dropped a CLI destination, which stays the row's one usable action")
	}
	for _, c := range snap.AllConcerns {
		if n := strings.Count(tab, `data-concern-id="`+c.ID+`"`); n != 1 {
			t.Fatalf("concern %s renders %d times in the tab, want once", c.ID, n)
		}
		row := concernRow(t, tab, c.ID)
		open := row[:strings.Index(row, ">")]
		want := ` data-target-kind="none"`
		if target, ok := targets[c.ID]; ok {
			want = ` data-target-kind="` + target.Kind + `" data-target="` + target.Value + `"`
		}
		if !strings.HasSuffix(open, want) {
			t.Errorf("concern %s's row opens %q, want its target %q", c.ID, open, want)
		}
	}

	// Guidance first (ac-5; parent dc-8; spec-documents ac-12): inside the
	// tab each row's first line is its primary line — the guidance, or the
	// fact of a proven row — with the step label and its timing mark after
	// it, and the fact, timing and blocking flag in the technical
	// disclosure. A row with a wall target carries one button naming it
	// (SI-368 (16)): the card's id, the stub's slug, the strip half, or the
	// slot's object kind; a row without one carries none.
	chips := map[string]string{
		"shape/problem":                            "problem",
		"shape/outcome":                            "outcome",
		"shape/question/oq-1":                      "oq-1",
		"success/criteria":                         "+ criterion",
		"success/coverage/ac-2":                    "ac-2",
		"review/blocker/stub-unreconciled/s-2fa-x": "2fa-x",
	}
	for _, c := range snap.AllConcerns {
		row := concernRow(t, tab, c.ID)
		primary := c.Guidance
		if primary == "" {
			primary = c.Summary
		}
		copyAt := strings.Index(row, `<div class="readiness-copy">`)
		if copyAt < 0 {
			t.Fatalf("concern %s has no copy:\n%s", c.ID, row)
		}
		first := row[copyAt+len(`<div class="readiness-copy">`):]
		if !strings.HasPrefix(first, `<div class="readiness-primary"><p class="readiness-summary`) || !strings.Contains(first[:strings.Index(first, `</p>`)+4], `>`+primary+`</p>`) {
			t.Errorf("concern %s's first line is not its primary line %q:\n%.300s", c.ID, primary, first)
		}
		if stage := strings.Index(first, `<p class="readiness-stage">`); stage < strings.Index(first, `</div>`) {
			t.Errorf("concern %s's step label precedes its primary line", c.ID)
		}
		tech := row[strings.Index(row, `<details class="readiness-tech">`):]
		for _, fact := range []string{`<dt>Fact</dt>`, `<dt>Timing</dt>`, `<dt>Blocking</dt>`} {
			if !strings.Contains(tech, fact) {
				t.Errorf("concern %s's technical disclosure lacks %s", c.ID, fact)
			}
		}
		label, targeted := chips[c.ID]
		named := label
		if c.ID == "success/criteria" {
			named = "the criterion slot"
		}
		chip := `<button type="button" class="readiness-target" data-testid="readiness-target" aria-label="Find ` + named + ` on the wall">` + label + `</button>`
		if got := strings.Count(row, `class="readiness-target"`); targeted && (got != 1 || !strings.Contains(row, chip)) {
			t.Errorf("concern %s lacks its target button %s:\n%s", c.ID, chip, row)
		} else if !targeted && got != 0 {
			t.Errorf("concern %s names no wall target yet carries a target button", c.ID)
		}
	}

	var page strings.Builder
	writeReadinessBody(&page, nil, snap, readinessPageSurface())
	requireWords(t, "the page", page.String(), readinessPageWords(), readinessWallWords())
	if strings.Contains(page.String(), `class="readiness-primary"`) || strings.Contains(page.String(), `class="readiness-target"`) {
		t.Error("the page's rows carry the tab's primary-first shape or a wall target")
	}
	if strings.Contains(page.String(), `data-target-kind`) || !strings.Contains(page.String(), `readiness-board-link`) {
		t.Error("the page's body carries wall targets, or lost its board link")
	}
}

// tabGet GETs path under ctx and returns the recorder.
func tabGet(t *testing.T, ctx context.Context, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil))
	return rec
}

// unavailableTab is the tab's whole body when its readiness is
// unavailable for reason.
func unavailableTab(reason string) string {
	return `<div class="readiness-tab" data-testid="readiness-tab" data-readiness-unavailable="1"><p class="readiness-tab-unavailable" data-testid="readiness-unavailable" role="status">Readiness is unavailable: ` + stdhtml.EscapeString(strings.TrimSuffix(reason, ".")) + `.</p></div>`
}

// TestReadinessTab_RouteServesOneLoad (SI-368 (1); co-1): on a wall
// served from the serving root, GET /board/spec/{name}/readiness renders
// the tab from exactly one readiness load of the wall's spec — the
// loader's snapshot, with the wall's targets — and the same route
// answers beneath /b/ for the serving root's own branch. A load that
// fails, or that names another spec, is the tab's unavailable body with
// its reason; a spec the wall does not carry is a 404, and a write is a
// 405, each before any load.
func TestReadinessTab_RouteServesOneLoad(t *testing.T) {
	root := newMarksWallFixture(t)
	head := gitOut(t, root, "rev-parse", "HEAD")
	snap := tabSnapshot(head)
	loader := &swapLoader{snap: snap}
	h := NewHandlerWith(root, Deps{Design: readinessGapCapsBridge(), ReadinessLoader: loader})
	path := "/board/spec/" + marksWallName + "/readiness"

	rec := tabGet(t, t.Context(), h, path)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("GET %s = %d %s\n%s", path, rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	if want := []string{"spec/" + marksWallName}; !reflect.DeepEqual(loader.loads, want) {
		t.Fatalf("the tab loaded %q, want exactly one load of %q", loader.loads, want)
	}
	cfg, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := rec.Body.String(), renderReadinessTab(cfg.Model, snap, tabSnapshotTargets()); got != want {
		t.Fatalf("the tab is not the loader's snapshot with the wall's targets:\n got: %.400s\nwant: %.400s", got, want)
	}

	serving := "/b/" + strings.ReplaceAll("design/"+marksWallName, "/", "%2F") + path
	if rec := tabGet(t, t.Context(), h, serving); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `data-concern-id="shape/problem"`) {
		t.Fatalf("GET %s = %d, want the serving wall's tab", serving, rec.Code)
	}
	if len(loader.loads) != 2 {
		t.Fatalf("the serving root's own branch at /b/ loaded %d times in all, want one more load", len(loader.loads))
	}

	other := snap
	other.TargetRef = "spec/some-other"
	for _, tc := range []struct {
		name   string
		snap   readinesspilot.Snapshot
		err    error
		reason string
	}{
		{"a failed load", readinesspilot.Snapshot{}, errBoom, "the readiness load failed: " + errBoom.Error()},
		{"a snapshot of another spec", other, nil, "the readiness snapshot describes spec/some-other, and this wall shows spec/" + marksWallName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loader.set(tc.snap, tc.err)
			before := len(loader.loads)
			rec := tabGet(t, t.Context(), h, path)
			if rec.Code != http.StatusOK || rec.Body.String() != unavailableTab(tc.reason) {
				t.Fatalf("GET %s = %d\n%s\nwant the unavailable body naming %q", path, rec.Code, rec.Body.String(), tc.reason)
			}
			if len(loader.loads) != before+1 {
				t.Fatalf("the tab loaded %d times, want once", len(loader.loads)-before)
			}
		})
	}

	before := len(loader.loads)
	if rec := tabGet(t, t.Context(), h, "/board/spec/no-such-wall/readiness"); rec.Code != http.StatusNotFound {
		t.Errorf("an unknown spec's tab = %d, want 404", rec.Code)
	}
	post := httptest.NewRecorder()
	h.ServeHTTP(post, httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST %s = %d, want 405", path, post.Code)
	}
	if len(loader.loads) != before {
		t.Fatalf("a refused request loaded readiness %d times", len(loader.loads)-before)
	}
}

// TestReadinessTab_FixedWallsLoadNothing (SI-368 (1); SI-360 (3); SI-364
// (3)): where the wall's marks are fixed for the server instance — a /b/
// wall whose branch is not the serving root's, or a server with no loader
// wired — and on a remote-only branch's sealed render, the tab answers
// that one reason without loading readiness, and a spec the wall does not
// carry is still a 404.
func TestReadinessTab_FixedWallsLoadNothing(t *testing.T) {
	t.Run("a /b/ branch wall", func(t *testing.T) {
		root := newMarksWallFixture(t)
		loader := &swapLoader{}
		h := NewHandlerWith(root, Deps{Design: readinessGapCapsBridge(), ReadinessLoader: loader})
		if _, err := wtmanager.EnsureWorktree(context.Background(), root, "design/marks-other"); err != nil {
			t.Fatalf("cutting the /b/ wall's worktree: %v", err)
		}
		mount := "/b/design%2Fmarks-other/board/spec/"
		acc := originAccepted(t, root)
		census := &readcensus.Census{}
		rec := tabGet(t, gitx.WithObserver(context.Background(), census), h, mount+marksWallName+"/readiness")
		if rec.Code != http.StatusOK || rec.Body.String() != unavailableTab(marksBranchWall("design/marks-other")) {
			t.Fatalf("the /b/ wall's tab = %d\n%s", rec.Code, rec.Body.String())
		}
		if b := census.Budget(acc); len(b.Writes) != 0 {
			t.Fatalf("the /b/ wall's tab wrote:\n  %s", strings.Join(b.Writes, "\n  "))
		}
		if rec := tabGet(t, t.Context(), h, mount+"no-such-wall/readiness"); rec.Code != http.StatusNotFound {
			t.Errorf("an unknown spec's /b/ tab = %d, want 404", rec.Code)
		}
		if len(loader.loads) != 0 {
			t.Fatalf("the /b/ wall's tab loaded readiness for %q", loader.loads)
		}
	})
	t.Run("unwired", func(t *testing.T) {
		root := newMarksWallFixture(t)
		h := NewHandlerWith(root, Deps{Design: readinessGapCapsBridge()})
		rec := tabGet(t, t.Context(), h, "/board/spec/"+marksWallName+"/readiness")
		if rec.Code != http.StatusOK || rec.Body.String() != unavailableTab(marksUnwired) {
			t.Fatalf("the unwired tab = %d\n%s", rec.Code, rec.Body.String())
		}
	})
	t.Run("sealed", func(t *testing.T) {
		h := NewHandler(newBranchBoardFixture(t))
		mount := "/b/design%2Fremote-only/board/spec/"
		rec := bGet(t, h, mount+"remote-spec/readiness")
		if rec.Code != http.StatusOK || rec.Body.String() != unavailableTab(marksSealed("origin/design/remote-only")) {
			t.Fatalf("the sealed tab = %d\n%s", rec.Code, rec.Body.String())
		}
		if rec := bGet(t, h, mount+"no-such-spec/readiness"); rec.Code != http.StatusNotFound {
			t.Errorf("an unknown spec's sealed tab = %d, want 404", rec.Code)
		}
	})
}

// TestReadinessTab_OneProjectionAndTheMarksTargets (Wave 6 §5.3; SI-368
// (1), (16)): through the production loader, the tab's route is one
// application projection — one read session, one accepted-HEAD
// resolution, at most one accepted-tree enumeration, no write — and each
// card the composed poll's marks name is the target of that concern's
// row in the tab.
func TestReadinessTab_OneProjectionAndTheMarksTargets(t *testing.T) {
	root := newMarksWallFixture(t)
	s := refreshServer(root)
	h := NewHandlerWith(root, Deps{Design: s.design, ReadinessLoader: s.readinessLoader})
	path := "/board/spec/" + marksWallName
	acc := originAccepted(t, root)

	var tab string
	for _, round := range []string{"cold", "warm"} {
		census := &readcensus.Census{}
		rec := tabGet(t, gitx.WithObserver(context.Background(), census), h, path+"/readiness")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s GET tab = %d\n%s", round, rec.Code, rec.Body.String())
		}
		checkRefreshBudget(t, "the tab's route, "+round, census.Budget(acc))
		tab = rec.Body.String()
	}
	if strings.Contains(tab, `data-readiness-unavailable`) {
		t.Fatalf("the tab is unavailable over the production loader:\n%s", tab)
	}
	_, poll := pollWall(t, context.Background(), h, path, "")
	if poll.Marks == nil || poll.Marks.Unavailable != "" || len(poll.Marks.Objects) == 0 || len(poll.Marks.Stubs) == 0 {
		t.Fatalf("the poll's marks = %+v, want object and stub marks", poll.Marks)
	}
	for card, marks := range poll.Marks.Objects {
		for _, m := range marks {
			if open := concernRow(t, tab, m.Concern); !strings.Contains(open, `data-target-kind="object" data-target="`+card+`"`) {
				t.Errorf("the mark on card %s names %s, whose tab row does not target it:\n%.300s", card, m.Concern, open)
			}
		}
	}
	for slug, m := range poll.Marks.Stubs {
		if open := concernRow(t, tab, m.Concern); !strings.Contains(open, `data-target-kind="stub" data-target="`+slug+`"`) {
			t.Errorf("the mark on stub %s names %s, whose tab row does not target it:\n%.300s", slug, m.Concern, open)
		}
	}
}

// TestReadinessTab_CarriesThePolicyGuide (SI-368 (3), (24)(f); ac-6's
// guide home): the Readiness tab carries the policy setup guide, chosen
// by policyGuideFor from the capabilities consultation alone, under a
// capabilities label beside the readiness — the not-adopted variant for
// draftmutation's own not-adopted refusal, the no-design-assistance
// variant for every other policy-forbidden refusal, on a wall whose
// readiness is fixed as much as on one whose readiness loads, its test
// ids kept; and no guide when capabilities were derived, when another
// failure stands, or when no design service is wired.
func TestReadinessTab_CarriesThePolicyGuide(t *testing.T) {
	forbidden := func(detail string) *scriptedCapsBridge {
		return &scriptedCapsBridge{script: func(int) (DesignReadOutcome, *DesignCapabilitiesView) {
			return DesignReadOutcome{Failure: &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: detail}}, nil
		}}
	}
	operational := &scriptedCapsBridge{script: func(int) (DesignReadOutcome, *DesignCapabilitiesView) {
		return DesignReadOutcome{Failure: &DesignFailure{Classification: "operational", Code: "io-failure", Detail: "boom"}}, nil
	}}
	const label = `<section class="readiness-tab-capabilities" data-testid="readiness-tab-capabilities" aria-label="Capabilities">`
	for _, tc := range []struct {
		name   string
		bridge DesignBridge
		kind   policyGuideKind
	}{
		{"not adopted", forbidden(policyNotAdoptedDetail), policyGuideNotAdopted},
		{"no design assistance", forbidden("effective policy carries no design_assistance payload"), policyGuideNoDesignAssistance},
		{"capabilities derived", readinessGapCapsBridge(), policyGuideNone},
		{"another failure", operational, policyGuideNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := newMarksWallFixture(t)
			snap := tabSnapshot(gitOut(t, root, "rev-parse", "HEAD"))
			h := NewHandlerWith(root, Deps{Design: tc.bridge, ReadinessLoader: &swapLoader{snap: snap}})
			cfg, err := store.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			body := renderReadinessTab(cfg.Model, snap, tabSnapshotTargets())
			if _, err := wtmanager.EnsureWorktree(context.Background(), root, "design/marks-other"); err != nil {
				t.Fatalf("cutting the /b/ wall's worktree: %v", err)
			}
			for path, want := range map[string]string{
				"/board/spec/" + marksWallName + "/readiness":                        body,
				"/b/design%2Fmarks-other/board/spec/" + marksWallName + "/readiness": unavailableTab(marksBranchWall("design/marks-other")),
			} {
				rec := tabGet(t, t.Context(), h, path)
				got := rec.Body.String()
				if rec.Code != http.StatusOK || !strings.HasPrefix(got, want) {
					t.Fatalf("GET %s = %d, want the tab's body first:\n%.400s", path, rec.Code, got)
				}
				guide := strings.TrimPrefix(got, want)
				if tc.kind == policyGuideNone {
					if guide != "" {
						t.Errorf("GET %s carries a guide without a policy-forbidden refusal:\n%s", path, guide)
					}
					continue
				}
				if !strings.HasPrefix(guide, label) || !strings.HasSuffix(guide, `</section></section>`) {
					t.Errorf("GET %s: the guide is not labelled as capabilities beside the readiness:\n%.300s", path, guide)
				}
				for _, id := range []string{`data-testid="asd-policy-guide"`, `data-policy-guide="` + string(tc.kind) + `"`, `data-testid="asd-policy-guide-report">policy-forbidden: `} {
					if !strings.Contains(guide, id) {
						t.Errorf("GET %s: the guide lacks %s", path, id)
					}
				}
			}
		})
	}
	t.Run("unwired", func(t *testing.T) {
		root := newMarksWallFixture(t)
		rec := tabGet(t, t.Context(), NewHandlerWith(root, Deps{}), "/board/spec/"+marksWallName+"/readiness")
		if rec.Code != http.StatusOK || rec.Body.String() != unavailableTab(marksUnwired) {
			t.Fatalf("the unwired tab = %d\n%s", rec.Code, rec.Body.String())
		}
	})
}
