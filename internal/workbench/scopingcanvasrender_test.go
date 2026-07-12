package workbench

// Render tests for the scoping-canvas wall surface (spec/scoping-canvas
// ac-3/ac-4/ac-5/ac-6, dc-5/dc-6): stub cards in the kind-locked stubs
// band, AC coverage chips, the OQ multi-claim smell, proto-sticky
// affordances, and the sealed wall's one live affordance (instantiate).
// All of it is presentation over the projection seams — deterministic
// markup, no LLM, no clock.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/boardio"
	"github.com/OWNER/verdi/internal/boardlayout"
)

// scopingRenderProjection builds the projection for the shared scoping
// fixture spec (two ACs, two OQs, one plain stub, two spike stubs) in
// the given mode.
func scopingRenderProjection(t *testing.T, mode boardModeKind) *BoardProjection {
	t.Helper()
	fm := mustDecodeSpecForTest(t, scopingProjectionFixtureSpec)
	p, err := buildProjection("scoping-fixture", fm, nil, nil, nil, mode)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	return p
}

// Stub views take computed-only positions in the stubs band, exactly
// like reference cards (dc-6; VL-018: stubs are not objects, so nothing
// is ever stored for them).
func TestScopingCanvas_StubViewPositions(t *testing.T) {
	p := scopingRenderProjection(t, modeReadOnly)
	if len(p.StubViews) != 3 {
		t.Fatalf("len(StubViews) = %d, want 3", len(p.StubViews))
	}
	var stubCol boardlayout.ZoneColumn
	for _, c := range boardlayout.ZoneColumns() {
		if c.Kind == boardlayout.ZoneStub {
			stubCol = c
		}
	}
	if stubCol.Kind != boardlayout.ZoneStub {
		t.Fatal("no stub column in ZoneColumns")
	}
	seen := map[[2]float64]bool{}
	for _, sv := range p.StubViews {
		if sv.X != float64(stubCol.X) {
			t.Errorf("stub %s at x=%v, want the stubs band (%d)", sv.Slug, sv.X, stubCol.X)
		}
		at := [2]float64{sv.X, sv.Y}
		if seen[at] {
			t.Errorf("two stubs stacked at %v", at)
		}
		seen[at] = true
	}
	// Deterministic: same inputs, same layout.
	again := scopingRenderProjection(t, modeReadOnly)
	for i := range p.StubViews {
		if p.StubViews[i].X != again.StubViews[i].X || p.StubViews[i].Y != again.StubViews[i].Y {
			t.Errorf("stub %s position not deterministic", p.StubViews[i].Slug)
		}
	}
}

// Declared stubs render as first-class scoping cards (ac-3): typeset
// spec register in the stubs band — slug tab, story/spike marking, and
// legible AC/OQ attributions — visually distinct from object cards.
func TestScopingCanvas_StubCardsRender(t *testing.T) {
	p := scopingRenderProjection(t, modeReadOnly)
	body := renderBoardRegion(p, &boardGitState{})

	for _, want := range []string{
		`data-testid="stub-card-plain-one"`,
		`data-stub="plain-one"`,
		`class="stubcard"`,
		`class="stubcard stubcard--spike"`,
		`data-testid="stub-card-spike-one"`,
		`data-spike="true"`,
		`<span class="stub-tab">plain-one</span>`,
		`<span class="stub-tab">spike-one</span>`,
		">story stub<",
		">spike stub<",
		// The typeset title: the slug set in the record's serif.
		`>Plain One</`,
		// Attributions: chips naming the covered ACs / resolved OQs.
		`data-testid="stub-links-plain-one"`,
		`stub-link-chip--ac">ac-1<`,
		`stub-link-chip--oq">oq-1<`,
		">covers<",
		">resolves<",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("stub cards missing %q", want)
		}
	}
	// Positions are inline like every card's.
	if !strings.Contains(body, `data-testid="stub-card-plain-one" data-stub="plain-one" style="left:952px;top:40px"`) {
		t.Error("plain-one not rendered at the stubs band's first slot")
	}
	// The stubs band wears its tape label (occupied, so in every mode).
	if !strings.Contains(body, `data-testid="zone-label-stub"`) || !strings.Contains(body, ">stubs<") {
		t.Error("stubs band label missing")
	}
}

// Every AC card on a feature wall wears its computed coverage chip
// (ac-4): calm when covered, quietly insistent when not; open questions
// wear the multi-claim smell only when >1 spike claims them (ac-5: an
// observation, never a rule).
func TestScopingCanvas_CoverageChipsAndSmell(t *testing.T) {
	p := scopingRenderProjection(t, modeReadOnly)
	body := renderBoardRegion(p, &boardGitState{})

	for _, want := range []string{
		`data-testid="coverage-ac-1" data-coverage="1"`,
		`>covered by 1 stub<`,
		`data-testid="coverage-ac-2" data-coverage="0"`,
		`>no stub<`,
		`coverage-chip--covered`,
		`coverage-chip--none`,
		`data-testid="oq-claims-oq-1" data-claims="2"`,
		`>claimed by 2 spikes<`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("coverage chips missing %q", want)
		}
	}
	// oq-2 is unclaimed — no badge at all (0 or 1 claims are the norm,
	// not an observation worth a badge).
	if strings.Contains(body, `data-testid="oq-claims-oq-2"`) {
		t.Error("unclaimed oq-2 wears a claims badge")
	}

	// A story wall never wears coverage chips: coverage is the feature's
	// scoping surface, not the story's.
	story, err := buildProjection("s", &artifact.SpecFrontmatter{
		Class: artifact.ClassStory,
		AcceptanceCriteria: []artifact.AcceptanceCriterion{
			{ID: "ac-1", Text: "story ac"},
		},
	}, nil, nil, nil, modeAuthoring)
	if err != nil {
		t.Fatalf("buildProjection(story): %v", err)
	}
	storyBody := renderBoardRegion(story, &boardGitState{})
	if strings.Contains(storyBody, "coverage-chip") {
		t.Error("a story wall wears coverage chips")
	}
}

// The stubs band label is class-aware: a feature-class authoring wall
// gets the empty invitation; a story wall never files stubs and gets no
// band label at all (dc-6's band is the feature's scoping surface).
func TestScopingCanvas_StubZoneLabelClassAware(t *testing.T) {
	feature, err := buildProjection("f", &artifact.SpecFrontmatter{Class: artifact.ClassFeature}, nil, nil, nil, modeAuthoring)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	body := renderBoardRegion(feature, &boardGitState{})
	if !strings.Contains(body, `zone-label--empty" data-testid="zone-label-stub"`) {
		t.Error("feature authoring wall missing the empty stubs-band invitation")
	}

	story, err := buildProjection("s", &artifact.SpecFrontmatter{Class: artifact.ClassStory}, nil, nil, nil, modeAuthoring)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	storyBody := renderBoardRegion(story, &boardGitState{})
	if strings.Contains(storyBody, `data-testid="zone-label-stub"`) {
		t.Error("story wall labels a stubs band it can never file into")
	}
}

// Proto-stickies (dc-5): on a feature-class authoring wall a story or
// spike sticky carries the yarn affordance (its attribution thread) and
// a Graduate that routes to stub-graduate — not the object graduate menu.
func TestScopingCanvas_ProtoStickyAffordances(t *testing.T) {
	const stickyID = "a-01J8Z0K3AAAAAAAAAAAAAAAAAA"
	fm := mustDecodeSpecForTest(t, scopingProjectionFixtureSpec)
	annotations := []*artifact.Annotation{
		{
			ID: stickyID, TS: "2026-07-10T14:02:11Z", Author: "j",
			Type: artifact.AnnotationStory, Body: "borrower bulk update", Status: artifact.AnnotationOpen,
			Board: &artifact.BoardAnchor{Story: "scoping-fixture", X: 960, Y: 600},
		},
		{
			ID: "a-01J8Z0K3CCCCCCCCCCCCCCCCCC", TS: "2026-07-10T14:02:12Z", Author: "j",
			Type: artifact.AnnotationComment, Body: "plain comment", Status: artifact.AnnotationOpen,
			Board: &artifact.BoardAnchor{Story: "scoping-fixture", X: 10, Y: 20},
		},
	}
	p, err := buildProjection("scoping-fixture", fm, nil, annotations, nil, modeAuthoring)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	body := renderBoardRegion(p, &boardGitState{})

	for _, want := range []string{
		`sticky--story`,
		`data-testid="yarn-handle-` + stickyID + `"`,
		`data-graduate="stub"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("proto-sticky affordances missing %q", want)
		}
	}
	// The plain comment keeps the generic graduate menu and gets NO yarn
	// handle: attribution yarn is the proto-sticky's meaning, not a
	// general sticky feature.
	if strings.Contains(body, `data-testid="yarn-handle-a-01J8Z0K3CCCCCCCCCCCCCCCCCC"`) {
		t.Error("a plain comment sticky grew a yarn handle")
	}
	if !strings.Contains(body, `data-graduate="sticky"`) {
		t.Error("the plain sticky lost its object graduate menu")
	}
}

// An attribution thread (a relates whose endpoint is a live sticky)
// carries no picker-graduate affordance: its meaning is the endpoint
// pair (dc-5), and stub-graduate on the sticky consumes it. It keeps
// its × (threads still die).
func TestScopingCanvas_AttributionThreadHasNoPickerGraduate(t *testing.T) {
	const stickyID = "a-01J8Z0K3AAAAAAAAAAAAAAAAAA"
	const threadID = "a-01J8Z0K4BBBBBBBBBBBBBBBBBB"
	fm := mustDecodeSpecForTest(t, scopingProjectionFixtureSpec)
	annotations := []*artifact.Annotation{
		{
			ID: stickyID, TS: "2026-07-10T14:02:11Z", Author: "j",
			Type: artifact.AnnotationStory, Body: "bulk update", Status: artifact.AnnotationOpen,
			Board: &artifact.BoardAnchor{Story: "scoping-fixture", X: 960, Y: 600},
		},
		{
			ID: threadID, TS: "2026-07-10T14:03:00Z", Author: "j",
			Type: artifact.AnnotationRelates, Body: "relates: sticky ~ ac-1", Status: artifact.AnnotationOpen,
			Target:  &artifact.Target{Ref: stickyID},
			TargetB: &artifact.Target{Ref: "spec/scoping-fixture@7f3c2a1", Selector: artifact.Selector{Heading: "ac-1"}},
		},
	}
	p, err := buildProjection("scoping-fixture", fm, nil, annotations, nil, modeAuthoring)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	body := renderBoardRegion(p, &boardGitState{})
	chipStart := strings.Index(body, `data-annotation-id="`+threadID+`"`)
	if chipStart < 0 {
		t.Fatal("attribution thread chip not rendered")
	}
	chip := body[chipStart:]
	if end := strings.Index(chip, "</div>"); end >= 0 {
		chip = chip[:end]
	}
	if strings.Contains(chip, `data-graduate="thread"`) {
		t.Error("attribution thread offers the picker graduate")
	}
	if !strings.Contains(chip, `data-delete="thread"`) {
		t.Error("attribution thread lost its delete affordance")
	}
}

// An attribution thread's sticky endpoint is the STICKY's paper — it
// must never mint a reference card for the annotation id (a live sticky
// is on this board already; a second card would split the endpoint and
// steal the thread's tie).
func TestScopingCanvas_StickyEndpointMintsNoRefCard(t *testing.T) {
	const stickyID = "a-01J8Z0K3AAAAAAAAAAAAAAAAAA"
	fm := mustDecodeSpecForTest(t, scopingProjectionFixtureSpec)
	annotations := []*artifact.Annotation{
		{
			ID: stickyID, TS: "2026-07-10T14:02:11Z", Author: "j",
			Type: artifact.AnnotationSpike, Body: "probe", Status: artifact.AnnotationOpen,
			Board: &artifact.BoardAnchor{Story: "scoping-fixture", X: 960, Y: 600},
		},
		{
			ID: "a-01J8Z0K4BBBBBBBBBBBBBBBBBB", TS: "2026-07-10T14:03:00Z", Author: "j",
			Type: artifact.AnnotationRelates, Body: "relates: sticky ~ oq-1", Status: artifact.AnnotationOpen,
			Target:  &artifact.Target{Ref: stickyID},
			TargetB: &artifact.Target{Ref: "spec/scoping-fixture@7f3c2a1", Selector: artifact.Selector{Heading: "oq-1"}},
		},
	}
	p, err := buildProjection("scoping-fixture", fm, nil, annotations, nil, modeAuthoring)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	for _, rc := range p.RefCards {
		if artifact.IsAnnotationID(rc.Ref) {
			t.Errorf("annotation id %s minted a reference card", rc.Ref)
		}
	}
	// The thread itself still projects, sticky id as its endpoint.
	found := false
	for _, e := range p.Edges {
		if e.Type == "relates" && (e.From == stickyID || e.To == stickyID) {
			found = true
		}
	}
	if !found {
		t.Error("attribution thread lost from the projection")
	}
}

// The embedded client payload carries the wall's class, so the sticky
// draft's type control can offer story/spike ONLY where the server would
// accept them (feature-class walls) — the client mirrors the server's
// gate instead of discovering it by refusal.
func TestScopingCanvas_PayloadCarriesClass(t *testing.T) {
	p := scopingRenderProjection(t, modeAuthoring)
	page, err := renderBoardSpecPage(p, &boardGitState{Branch: "design/x"})
	if err != nil {
		t.Fatalf("renderBoardSpecPage: %v", err)
	}
	if !strings.Contains(string(page), `"class":"feature"`) {
		t.Error("client payload missing the wall's class")
	}
}

// A sealed accepted-pending-build feature wall carries the ONE live
// affordance a sealed record permits (ac-6): Instantiate story on each
// stub card — consequence-labeled client-side before firing — plus the
// confirm dialog chrome it needs even though the wall is read-only.
func TestScopingCanvas_InstantiateAffordance(t *testing.T) {
	p := scopingRenderProjection(t, modeReadOnly)
	if p.Status != "accepted-pending-build" {
		t.Fatalf("fixture status = %q", p.Status)
	}
	body := renderBoardRegion(p, &boardGitState{})
	for _, want := range []string{
		`data-testid="instantiate-plain-one"`,
		`data-instantiate="plain-one"`,
		`>Instantiate story<`,
		`data-testid="instantiate-spike-one"`,
		`>Instantiate spike<`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sealed wall missing instantiate affordance %q", want)
		}
	}

	page, err := renderBoardSpecPage(p, &boardGitState{Branch: "main"})
	if err != nil {
		t.Fatalf("renderBoardSpecPage: %v", err)
	}
	for _, want := range []string{`id="edge-confirm"`, `id="modal-backdrop"`} {
		if !strings.Contains(string(page), want) {
			t.Errorf("sealed instantiate wall missing dialog chrome %q", want)
		}
	}

	// A DRAFT feature wall (authoring) shows no instantiate: only an
	// accepted-pending-build record may cut story branches (the owner's
	// rule: implementations build accepted specs only).
	draft := scopingRenderProjection(t, modeAuthoring)
	draft.Status = "draft"
	draftBody := renderBoardRegion(draft, &boardGitState{})
	if strings.Contains(draftBody, "data-instantiate") {
		t.Error("a draft wall offers instantiate")
	}
}

// A story or spike sticky parks handwritten at the BOTTOM of the stubs
// band (dc-6: "its parking spot a claim about where the stub will land")
// — the same landing policy every sticky type already follows.
func TestScopingCanvas_ProtoStickyLandsInStubBand(t *testing.T) {
	root := newScopingWallFixture(t)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, scopingWallName, "sticky", `{"type":"story","text":"lane check"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("sticky = %d\n%s", rec.Code, rec.Body.String())
	}
	annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	if err != nil {
		t.Fatalf("reading annotations: %v", err)
	}
	var found *artifact.Annotation
	for _, a := range annotations {
		if a.Body == "lane check" {
			found = a
		}
	}
	if found == nil || found.Board == nil {
		t.Fatal("story sticky not written with a board anchor")
	}
	var stubCol boardlayout.ZoneColumn
	for _, c := range boardlayout.ZoneColumns() {
		if c.Kind == boardlayout.ZoneStub {
			stubCol = c
		}
	}
	if found.Board.X != float64(stubCol.X) || found.Board.Y != boardlayout.ZoneOriginY {
		t.Errorf("story sticky landed at (%v,%v), want the empty stubs band's first slot (%d,%d)",
			found.Board.X, found.Board.Y, stubCol.X, boardlayout.ZoneOriginY)
	}
}

// StubView's JSON gains x/y additively — get_board re-marshals this
// struct, so the wire shape is asserted here once.
func TestScopingCanvas_StubViewJSONAdditive(t *testing.T) {
	p := scopingRenderProjection(t, modeReadOnly)
	raw, err := json.Marshal(p.StubViews[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"slug":"plain-one"`, `"x":952`, `"y":40`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("StubView JSON missing %s\n%s", want, raw)
		}
	}
}
