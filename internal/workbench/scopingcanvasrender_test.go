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
	"os"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/boardlayout"
)

// scopingRenderProjection builds the projection for the shared scoping
// fixture spec (two ACs, two OQs, one plain stub, two spike stubs) in
// the given mode.
func scopingRenderProjection(t *testing.T, mode boardModeKind) *BoardProjection {
	t.Helper()
	fm := mustDecodeSpecForTest(t, scopingProjectionFixtureSpec)
	p, err := buildProjection("scoping-fixture", fm, nil, nil, nil, nil, mode)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	return p
}

// Stub views fall back to a computed lane position in the stubs band
// absent any stored one — this fixture's projection carries no stored
// positions at all, so every stub takes its lane default (round 5.5 dc-6
// amendment: a stub CAN carry a stored `stub:<slug>` position and win
// verbatim, covered separately in projection_test.go and boardspecapi's
// position-action tests; this test only exercises the no-stored-position
// path).
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
// spec register in the stubs band — slug tab, story/spike marking —
// visually distinct from object cards. AMENDED (scoping yarn, owner
// directive): the card's AC/OQ chip list retired — the scoping yarn is
// the attribution's representation now (asserted below in
// TestScopingCanvas_ScopingYarnChips), so this test now proves the chips
// are GONE where it used to prove they rendered.
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
	} {
		if !strings.Contains(body, want) {
			t.Errorf("stub cards missing %q", want)
		}
	}
	// The chip list is retired: the attribution lives on the wall as
	// scoping yarn, not on the card as text.
	for _, gone := range []string{"stub-links", "stub-link-chip"} {
		if strings.Contains(body, gone) {
			t.Errorf("retired stub chip markup %q still renders", gone)
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

// scopingChipsOf slices every scoping-layer yarn chip's full element out
// of a rendered board region (the chip div opens with its class and
// closes before any nested div — chips contain only spans/buttons).
func scopingChipsOf(t *testing.T, body string) []string {
	t.Helper()
	var chips []string
	rest := body
	for {
		start := strings.Index(rest, `<div class="yarn-chip yarn-chip--scoping"`)
		if start < 0 {
			break
		}
		rest = rest[start:]
		end := strings.Index(rest, `</div>`)
		if end < 0 {
			t.Fatal("unterminated yarn chip markup")
		}
		chips = append(chips, rest[:end])
		rest = rest[end:]
	}
	return chips
}

// The scoping yarn (owner directive, verbatim: "the yarn should be used
// to consistently represent the UI element. Story stubs are associated
// with acceptance criteria and spikes are associated with open
// questions."): each scoping edge renders one yarn chip under
// data-layer="scoping" — the projection of the stubs block, in the yarn
// system's own contract (data-edge-type / data-from / data-to), with the
// stub's "stub:<slug>" key as its From so the thread ties to the stub
// card's paper.
func TestScopingCanvas_ScopingYarnChips(t *testing.T) {
	for _, mode := range []boardModeKind{modeAuthoring, modeReview, modeReadOnly} {
		p := scopingRenderProjection(t, mode)
		body := renderBoardRegion(p, &boardGitState{})
		for _, want := range []string{
			`<div class="yarn-chip yarn-chip--scoping" data-edge-type="covers" data-from="stub:plain-one" data-to="ac-1" data-layer="scoping">`,
			`<div class="yarn-chip yarn-chip--scoping" data-edge-type="resolves" data-from="stub:spike-one" data-to="oq-1" data-layer="scoping">`,
			`<div class="yarn-chip yarn-chip--scoping" data-edge-type="resolves" data-from="stub:spike-two" data-to="oq-1" data-layer="scoping">`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: scoping yarn chip missing: %q", mode, want)
			}
		}
	}
}

// A scoping edge is a PROJECTION of the stubs block, not a document
// link: it carries no graduate/delete/retype affordance in ANY mode —
// there is no spec edge behind it to edit, and the closed five-type
// vocabulary is untouched. This is the merge-blocking assertion the
// owner directive names: no scoping edge ever renders an edit
// affordance.
func TestScopingCanvas_ScopingEdgesCarryNoAffordances(t *testing.T) {
	for _, mode := range []boardModeKind{modeAuthoring, modeReview, modeReadOnly} {
		p := scopingRenderProjection(t, mode)
		body := renderBoardRegion(p, &boardGitState{})
		chips := scopingChipsOf(t, body)
		if len(chips) != 3 {
			t.Fatalf("%s: found %d scoping chips, want 3", mode, len(chips))
		}
		for _, chip := range chips {
			if strings.Contains(chip, "<button") {
				t.Errorf("%s: a scoping chip renders an affordance:\n%s", mode, chip)
			}
			for _, forbidden := range []string{"data-retype", "data-graduate", "data-delete"} {
				if strings.Contains(chip, forbidden) {
					t.Errorf("%s: a scoping chip carries %s:\n%s", mode, forbidden, chip)
				}
			}
		}
	}
}

// The yarn key gains the scoping entries — listed only when present,
// each with its one-line planning-tense meaning, distinguishable from
// the spec layer's committed types by data-layer (a wall can carry a
// spec-layer resolves AND a scoping resolves at once; the key must name
// both without collapsing them).
func TestScopingCanvas_YarnKeyScopingEntries(t *testing.T) {
	p := scopingRenderProjection(t, modeReadOnly)
	body := renderBoardRegion(p, &boardGitState{})
	for _, want := range []string{
		`<li data-layer="scoping" data-edge-type="covers">`,
		`<li data-layer="scoping" data-edge-type="resolves">`,
		"a planned story will deliver it",
		"a planned spike will answer it",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("yarn key missing scoping entry %q", want)
		}
	}
	// Only when present: a wall with covers-only lists no scoping
	// resolves entry.
	coversOnly := &BoardProjection{
		Spec: "s", Mode: modeReadOnly,
		Edges: []edgeView{{Type: "covers", From: "stub:x", To: "ac-1", Layer: "scoping"}},
	}
	coversBody := renderBoardRegion(coversOnly, &boardGitState{})
	if !strings.Contains(coversBody, `<li data-layer="scoping" data-edge-type="covers">`) {
		t.Error("covers-only wall lost its covers key entry")
	}
	if strings.Contains(coversBody, `<li data-layer="scoping" data-edge-type="resolves">`) {
		t.Error("covers-only wall lists a scoping resolves entry it does not carry")
	}

	// Coexistence: spec resolves and scoping resolves are two entries.
	both := &BoardProjection{
		Spec: "s", Mode: modeReadOnly,
		Edges: []edgeView{
			{Type: "resolves", From: "spec", To: "spec/other", Layer: "spec"},
			{Type: "resolves", From: "stub:x", To: "oq-1", Layer: "scoping"},
		},
	}
	bothBody := renderBoardRegion(both, &boardGitState{})
	if !strings.Contains(bothBody, `<li data-layer="spec" data-edge-type="resolves">`) ||
		!strings.Contains(bothBody, `<li data-layer="scoping" data-edge-type="resolves">`) {
		t.Error("spec resolves and scoping resolves collapsed into one key entry")
	}
	// Canonical order: the committed types precede the planning types,
	// scratch stays last.
	specIdx := strings.Index(bothBody, `<li data-layer="spec" data-edge-type="resolves">`)
	scopingIdx := strings.Index(bothBody, `<li data-layer="scoping" data-edge-type="resolves">`)
	if specIdx > scopingIdx {
		t.Error("yarn key lists planning threads before the committed record's")
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
	}, nil, nil, nil, nil, modeAuthoring)
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
	feature, err := buildProjection("f", &artifact.SpecFrontmatter{Class: artifact.ClassFeature}, nil, nil, nil, nil, modeAuthoring)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	body := renderBoardRegion(feature, &boardGitState{})
	if !strings.Contains(body, `zone-label--empty" data-testid="zone-label-stub"`) {
		t.Error("feature authoring wall missing the empty stubs-band invitation")
	}

	story, err := buildProjection("s", &artifact.SpecFrontmatter{Class: artifact.ClassStory}, nil, nil, nil, nil, modeAuthoring)
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
	p, err := buildProjection("scoping-fixture", fm, nil, nil, annotations, nil, modeAuthoring)
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
	p, err := buildProjection("scoping-fixture", fm, nil, nil, annotations, nil, modeAuthoring)
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
	p, err := buildProjection("scoping-fixture", fm, nil, nil, annotations, nil, modeAuthoring)
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

// The two committed fixtures that already declare stubs — the corpus's
// accepted-pending-build (also driven end-to-end by e2e/tests/31) and
// this repo's own live-store disclosure-legibility — render their stub
// cards for free; assert them verbatim so the committed record and the
// wall never drift apart silently.
func TestScopingCanvas_CommittedFixturesRenderStubCards(t *testing.T) {
	cases := []struct {
		name, path string
		slugs      []string
	}{
		{
			name: "accepted-pending-build (examples/showcase)",
			path: "../../examples/showcase/.verdi/specs/active/accepted-pending-build/spec.md",
			slugs: []string{
				"borrower-update-api", "borrower-update-ui", "borrower-update-audit-log",
			},
		},
		{
			name:  "disclosure-legibility (the live store)",
			path:  "../../.verdi/specs/active/disclosure-legibility/spec.md",
			slugs: []string{"disclosure-seam", "disclosures-panel"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			fm := mustDecodeSpecForTest(t, string(raw))
			p, err := buildProjection("x", fm, nil, nil, nil, nil, modeReadOnly)
			if err != nil {
				t.Fatalf("buildProjection: %v", err)
			}
			body := renderBoardRegion(p, &boardGitState{})
			for _, slug := range tc.slugs {
				if !strings.Contains(body, `data-testid="stub-card-`+slug+`"`) {
					t.Errorf("declared stub %q has no card on the wall", slug)
				}
			}
			if !strings.Contains(body, `data-testid="zone-label-stub"`) {
				t.Error("stubs band not labeled on a wall that files stubs")
			}
		})
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
