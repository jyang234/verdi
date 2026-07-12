package workbench

// The legibility layer (owner UAT: the board must read, at a glance,
// like the murder board it is): labeled zone bands over the columns the
// zoned algorithm files cards into, an empty wall that invites instead
// of voiding, mode identity readable from the page chrome, a yarn key
// naming only the relationship types actually on the wall, and the
// four-move guide — 05 §Workbench "The four-concept minimum path":
// everything beyond the minimum is discoverable, never front-loaded.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/boardio"
)

// The projection carries the spec's class identity (02 §Kind registry:
// class + story tracker ref + spike flag) so both presentations — the
// HTML case file and get_board's JSON — can say which kind of wall this
// is. Additive fields only; the JSON keys are wire contract (get_board
// re-marshals this struct).
func TestBoardProjection_CarriesSpecClass(t *testing.T) {
	cases := []struct {
		name     string
		fm       artifact.SpecFrontmatter
		class    string
		storyRef string
		spike    bool
	}{
		{"feature", artifact.SpecFrontmatter{Class: artifact.ClassFeature}, "feature", "", false},
		{"story", artifact.SpecFrontmatter{Class: artifact.ClassStory, Story: "jira:LOAN-7"}, "story", "jira:LOAN-7", false},
		{"spike", artifact.SpecFrontmatter{Class: artifact.ClassStory, Story: "jira:LOAN-9", Spike: true}, "story", "jira:LOAN-9", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := buildProjection("s", &tc.fm, nil, nil, nil, modeAuthoring)
			if err != nil {
				t.Fatalf("buildProjection: %v", err)
			}
			if p.Class != tc.class || p.StoryRef != tc.storyRef || p.Spike != tc.spike {
				t.Fatalf("projection = (%q, %q, %v), want (%q, %q, %v)",
					p.Class, p.StoryRef, p.Spike, tc.class, tc.storyRef, tc.spike)
			}
		})
	}

	// The wire keys (get_board marshals the projection verbatim): class
	// always present; story_ref and spike omitted when zero.
	spike, err := buildProjection("s", &artifact.SpecFrontmatter{Class: artifact.ClassStory, Story: "jira:LOAN-9", Spike: true}, nil, nil, nil, modeAuthoring)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	raw, err := json.Marshal(spike)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"class":"story"`, `"story_ref":"jira:LOAN-9"`, `"spike":true`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("projection JSON missing %s\n%s", want, raw)
		}
	}
	feature, err := buildProjection("s", &artifact.SpecFrontmatter{Class: artifact.ClassFeature}, nil, nil, nil, modeAuthoring)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	raw, err = json.Marshal(feature)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, banned := range []string{`"story_ref"`, `"spike"`} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("feature projection JSON carries zero-valued %s\n%s", banned, raw)
		}
	}
}

// Authoring labels every zone — empty bands included, as invitations
// ("decisions land here") — while review/read-only label only what the
// record actually holds.
func TestBoardLegibility_ZoneLabels(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	body := getBoard(t, h, boardFixtureName).Body.String()

	for _, want := range []string{
		`data-testid="zone-label-acceptance-criterion"`,
		`data-testid="zone-label-constraint"`,
		`data-testid="zone-label-decision"`,
		`data-testid="zone-label-open-question"`,
		`data-testid="zone-label-reference"`,
		">acceptance criteria<", ">constraints<", ">decisions<",
		">open questions<", ">references<",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("authoring board missing zone label %q", want)
		}
	}
	// The label bands sit over their own columns (boardlayout.ZoneColumns).
	if !strings.Contains(body, `data-testid="zone-label-acceptance-criterion" style="left:40px;width:200px"`) {
		t.Error("AC zone label not aligned with its column band")
	}
	if !strings.Contains(body, `data-testid="zone-label-constraint" style="left:268px;width:200px"`) {
		t.Error("constraint zone label not aligned with its column band")
	}
	// The fixture has no open questions: in authoring the band still
	// renders, marked as the empty invitation it is.
	if !strings.Contains(body, `zone-label--empty" data-testid="zone-label-open-question"`) {
		t.Error("empty open-question band not marked zone-label--empty in authoring")
	}

	// Read-only and review: only occupied zones are labeled — a sealed
	// record shows what it has, it does not invite.
	proj := &BoardProjection{
		Spec: boardFixtureName, Mode: modeReadOnly,
		Cards: []cardView{{ID: "ac-1", Kind: "acceptance-criterion", Text: "x"}},
	}
	for _, mode := range []boardModeKind{modeReadOnly, modeReview} {
		proj.Mode = mode
		frozen := renderBoardRegion(proj, &boardGitState{})
		if !strings.Contains(frozen, `data-testid="zone-label-acceptance-criterion"`) {
			t.Errorf("%s board missing its occupied zone's label", mode)
		}
		for _, banned := range []string{
			`data-testid="zone-label-decision"`,
			`data-testid="zone-label-reference"`,
			`zone-label--empty`,
		} {
			if strings.Contains(frozen, banned) {
				t.Errorf("%s board labels an unoccupied zone: %s", mode, banned)
			}
		}
	}
}

// An empty or sparse board is an invitation in authoring, a plain
// statement of record everywhere else — never a bare void. Reference
// cards don't count as pinned facts: the leanest valid story spec
// already hangs its implements thread, and its wall still invites.
func TestBoardLegibility_EmptyWall(t *testing.T) {
	empty := &BoardProjection{Spec: "fresh", Mode: modeAuthoring}
	body := renderBoardRegion(empty, &boardGitState{Branch: "design/fresh"})
	if !strings.Contains(body, `data-testid="board-empty"`) {
		t.Fatal("empty authoring board renders no empty-wall state")
	}
	for _, want := range []string{"Nothing pinned yet", "Add sticky", "graduate"} {
		if !strings.Contains(body, want) {
			t.Errorf("empty-wall invitation missing %q", want)
		}
	}

	// A refcard-only wall (story spec + implements thread, no facts)
	// still invites.
	refOnly := &BoardProjection{
		Spec: "fresh", Mode: modeAuthoring,
		RefCards: []refCardView{{Ref: "spec/f#ac-1", X: 40, Y: 40}},
		Edges:    []edgeView{{Type: "implements", From: "spec", To: "spec/f#ac-1", Layer: "spec"}},
	}
	if !strings.Contains(renderBoardRegion(refOnly, &boardGitState{}), `data-testid="board-empty"`) {
		t.Error("a wall holding only reference cards lost its invitation")
	}

	for _, mode := range []boardModeKind{modeReadOnly, modeReview} {
		empty.Mode = mode
		frozen := renderBoardRegion(empty, &boardGitState{})
		if !strings.Contains(frozen, `data-testid="board-empty"`) {
			t.Errorf("empty %s board renders no empty state", mode)
		}
		if !strings.Contains(frozen, "Nothing is declared on this spec") {
			t.Errorf("empty %s board missing the record statement", mode)
		}
		if strings.Contains(frozen, "Add sticky") {
			t.Errorf("empty %s board invites editing", mode)
		}
	}

	// A board with any element is not empty.
	occupied := &BoardProjection{
		Spec: "fresh", Mode: modeAuthoring,
		Stickies: []scratchStickyView{{ID: "a-1", Type: "comment", Body: "b"}},
	}
	if strings.Contains(renderBoardRegion(occupied, &boardGitState{}), `data-testid="board-empty"`) {
		t.Error("a board holding a sticky still renders the empty-wall state")
	}
}

// The yarn key names exactly the edge types present, in the canonical
// order, and disappears with the last thread — a legend of the wall,
// not a vocabulary lesson (05 §Workbench: the fuller vocabulary is
// "discoverable when needed, never front-loaded").
func TestBoardLegibility_YarnKey(t *testing.T) {
	proj := &BoardProjection{
		Spec: "s", Mode: modeAuthoring,
		Cards: []cardView{{ID: "dc-1", Kind: "decision", Text: "x"}},
		Edges: []edgeView{
			{Type: "relates", From: "dc-1", To: "adr/a", Layer: "annotation", AnnotationID: "a-1"},
			{Type: "exempts", From: "dc-1", To: "adr/a", Layer: "spec"},
		},
	}
	body := renderBoardRegion(proj, &boardGitState{})
	if !strings.Contains(body, `data-testid="yarn-key"`) {
		t.Fatal("board with edges renders no yarn key")
	}
	exempts := strings.Index(body, `data-edge-type="exempts">`)
	relates := strings.Index(body, `data-edge-type="relates">`)
	if exempts < 0 || relates < 0 {
		t.Fatalf("yarn key missing present types (exempts@%d relates@%d)", exempts, relates)
	}
	if exempts > relates {
		t.Error("yarn key not in canonical order (exempts should precede relates)")
	}
	for _, absent := range []string{
		`data-edge-type="implements">`, `data-edge-type="supersedes">`,
		`data-edge-type="resolves">`, `data-edge-type="depends-on">`,
	} {
		if strings.Contains(body, absent) {
			t.Errorf("yarn key lists a type not on the wall: %s", absent)
		}
	}

	proj.Edges = nil
	if strings.Contains(renderBoardRegion(proj, &boardGitState{}), `data-testid="yarn-key"`) {
		t.Error("yarn key renders on a wall with no threads")
	}
}

// The four-move guide is authoring's quiet teacher: present (collapsed
// by markup — no open attribute) in authoring, absent from the mirror
// and the sealed record.
func TestBoardLegibility_Guide(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-testid="board-guide"`) {
		t.Fatal("authoring board renders no guide")
	}
	if strings.Contains(body, `<details class="board-guide" data-testid="board-guide" open`) {
		t.Error("the guide front-loads itself (open by default)")
	}
	for _, want := range []string{"case file", "acceptance criteria", "yarn", "Commit"} {
		if !strings.Contains(body, want) {
			t.Errorf("guide missing the four-move vocabulary %q", want)
		}
	}

	proj := &BoardProjection{Spec: "s", Mode: modeReadOnly, Cards: []cardView{{ID: "ac-1", Kind: "acceptance-criterion", Text: "x"}}}
	for _, mode := range []boardModeKind{modeReadOnly, modeReview} {
		proj.Mode = mode
		if strings.Contains(renderBoardRegion(proj, &boardGitState{}), `data-testid="board-guide"`) {
			t.Errorf("%s board renders the authoring guide", mode)
		}
	}
}

// Mode identity is page-level chrome, not just a data attribute: the
// body carries a mode class and the stamp names the room's state.
func TestBoardLegibility_ModeChrome(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	body := getBoard(t, h, boardFixtureName).Body.String()
	for _, want := range []string{
		`class="board-page boardv2-page mode-authoring"`,
		`board-mode-tag--authoring`, "live wall",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("authoring page chrome missing %q", want)
		}
	}

	for mode, want := range map[boardModeKind]string{
		modeReview:   "mirror of the MR",
		modeReadOnly: "sealed record",
	} {
		page, err := renderBoardSpecPage(&BoardProjection{Spec: "s", Title: "S", Mode: mode}, &boardGitState{})
		if err != nil {
			t.Fatalf("rendering %s page: %v", mode, err)
		}
		if !strings.Contains(string(page), want) {
			t.Errorf("%s page stamp missing %q", mode, want)
		}
		if !strings.Contains(string(page), "mode-"+string(mode)) {
			t.Errorf("%s page body missing its mode class", mode)
		}
	}
}

// The case-file header and the document's own yarn: placards join into
// one problem→outcome lockup, and a document-level chip says whose edge
// it is ("this spec"), since the document is not a card.
func TestBoardLegibility_CaseFileAndDocChip(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	body := getBoard(t, h, boardFixtureName).Body.String()
	for _, want := range []string{`board-placards case-file`, `case-arrow`, `placard--problem`, `placard--outcome`} {
		if !strings.Contains(body, want) {
			t.Errorf("case-file header missing %q", want)
		}
	}

	// A spec carrying neither attribute (grandfathered v0 artifacts) gets
	// no folder header at all — never an empty tab.
	bare := &BoardProjection{Spec: "s", Mode: modeReadOnly, Cards: []cardView{{ID: "ac-1", Kind: "acceptance-criterion", Text: "x"}}}
	if strings.Contains(renderBoardRegion(bare, &boardGitState{}), "case-tab") {
		t.Error("a spec with no problem/outcome still renders the case-file tab")
	}

	proj := &BoardProjection{
		Spec: "s", Mode: modeReadOnly,
		Edges:    []edgeView{{Type: "implements", From: "spec", To: "adr/a", Layer: "spec"}},
		RefCards: []refCardView{{Ref: "adr/a", X: 40, Y: 40}},
	}
	frozen := renderBoardRegion(proj, &boardGitState{})
	if !strings.Contains(frozen, `yarn-chip--doc`) {
		t.Error("document-level chip not marked yarn-chip--doc")
	}
	if !strings.Contains(frozen, `<span class="yarn-chip-doc">this spec</span>`) {
		t.Error("document-level chip missing its 'this spec' prefix")
	}
	// A card-sourced chip carries no document prefix.
	proj.Edges = []edgeView{{Type: "exempts", From: "dc-1", To: "adr/a", Layer: "spec"}}
	proj.Cards = []cardView{{ID: "dc-1", Kind: "decision", Text: "x"}}
	if strings.Contains(renderBoardRegion(proj, &boardGitState{}), "yarn-chip--doc") {
		t.Error("card-sourced chip wrongly marked as the document's")
	}
}

// The case file wears its class (owner directive): a small stamp in the
// case-file lockup — "feature" on feature walls, "story · <tracker-ref>"
// on story walls (ref omitted when absent), "spike · <tracker-ref>" on
// spikes — in every mode; the sealed record needs it just as much. A
// projection with no class (grandfathered callers) gets no stamp, and a
// spec with no case-file header (no problem/outcome) has nowhere to
// wear one.
func TestBoardLegibility_CaseClassTag(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `<span class="case-class-tag case-class-tag--feature" data-testid="case-class-tag">feature</span>`) {
		t.Error("feature wall's case file wears no feature stamp")
	}

	cases := []struct {
		name string
		proj BoardProjection
		want string
	}{
		{"story with tracker ref", BoardProjection{Class: "story", StoryRef: "jira:LOAN-7"},
			`<span class="case-class-tag case-class-tag--story" data-testid="case-class-tag">story · <span class="case-class-ref">jira:LOAN-7</span></span>`},
		{"story without tracker ref", BoardProjection{Class: "story"},
			`<span class="case-class-tag case-class-tag--story" data-testid="case-class-tag">story</span>`},
		{"spike", BoardProjection{Class: "story", StoryRef: "jira:LOAN-9", Spike: true},
			`<span class="case-class-tag case-class-tag--spike" data-testid="case-class-tag">spike · <span class="case-class-ref">jira:LOAN-9</span></span>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Every mode: the mirror and the sealed record wear the stamp too.
			for _, mode := range []boardModeKind{modeAuthoring, modeReview, modeReadOnly} {
				proj := tc.proj
				proj.Spec, proj.Mode = "s", mode
				proj.Problem, proj.Outcome = "p", "o"
				if got := renderBoardRegion(&proj, &boardGitState{}); !strings.Contains(got, tc.want) {
					t.Errorf("%s board missing class stamp %s", mode, tc.want)
				}
			}
		})
	}

	// No class → no stamp (never an empty tag).
	bare := &BoardProjection{Spec: "s", Mode: modeReadOnly, Problem: "p", Outcome: "o"}
	if strings.Contains(renderBoardRegion(bare, &boardGitState{}), "case-class-tag") {
		t.Error("a projection with no class still renders a class stamp")
	}
}

// A new sticky lands at the BOTTOM of its type's lane (owner directive):
// questions queue beneath the open-questions column they may graduate
// into, decisions-needed beneath decisions, comments and agent tasks in
// the scratch lane past the references. Deterministic given the board.
func TestBoardLegibility_StickyLanding(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	post := func(body string) {
		t.Helper()
		rec := postBoardAPI(t, h, boardFixtureName, "sticky", body)
		if rec.Code != http.StatusOK {
			t.Fatalf("sticky = %d\n%s", rec.Code, rec.Body.String())
		}
	}
	stickyAt := func(text string) (x, y float64) {
		t.Helper()
		annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
		if err != nil {
			t.Fatalf("reading annotations: %v", err)
		}
		for _, a := range annotations {
			if a.Body == text && a.Board != nil {
				return a.Board.X, a.Board.Y
			}
		}
		t.Fatalf("no positioned annotation with body %q", text)
		return 0, 0
	}

	// The fixture's open-question lane (x=724) is empty: first slot.
	post(`{"type":"question","text":"q-lane"}`)
	if x, y := stickyAt("q-lane"); x != 724 || y != 40 {
		t.Errorf("question landed at (%v,%v), want empty open-question lane slot (724,40)", x, y)
	}

	// The decisions lane holds dc-1 (zoned 496,40) and dc-2 (496,216):
	// a decision-needed sticky appends below dc-2's footprint.
	post(`{"type":"decision-needed","text":"d-lane"}`)
	if x, y := stickyAt("d-lane"); x != 496 || y != 380 {
		t.Errorf("decision-needed landed at (%v,%v), want below dc-2 (496,380)", x, y)
	}

	// Comments file into the scratch lane; a second one appends below
	// the first (sticky footprint estimate + gap).
	post(`{"type":"comment","text":"c-one"}`)
	if x, y := stickyAt("c-one"); x != 1180 || y != 40 {
		t.Errorf("comment landed at (%v,%v), want empty scratch lane slot (1180,40)", x, y)
	}
	post(`{"type":"agent-task","text":"c-two"}`)
	if x, y := stickyAt("c-two"); x != 1180 || y != 214 {
		t.Errorf("agent-task landed at (%v,%v), want appended below c-one (1180,214)", x, y)
	}

	// The scratch lane's zone label materializes with its occupant.
	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-testid="zone-label-scratch"`) {
		t.Error("occupied scratch lane has no zone label")
	}
}
