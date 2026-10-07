package workbench

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/boardlayout"
)

// The wall canvas's markup for spec/wall-canvas-v2 ac-3, ac-4 and ac-5
// (lane F2b; ledger SI-350 (5), (10), (15)). The in-chip and in-card
// buttons move to the contextual toolbar: a thread chip becomes the button
// that selects its thread and carries the actions the server deems legal
// for it as data attributes the toolbar projects; a sticky carries its own.
// The add-in-place slots sit at the foot of each object column where the
// domain is live. The pin toolbox loses its fixed tab (the toolbar's button
// controls the tray), and the status row hosts the toolbar.

// toolbarProjection is a wall with one editable spec edge (dc-1 exempts the
// ADR), one document-level edge (the spec implements a feature AC), one
// scratch thread between two cards (graduatable), one scratch thread that
// touches a sticky (an attribution thread: never graduates), a story
// proto-sticky and a comment sticky.
func toolbarProjection(mode boardModeKind, refusal string) *BoardProjection {
	return &BoardProjection{
		Spec: "s", Mode: mode, Class: "feature", DomainRefusal: refusal,
		Cards: []cardView{
			{ID: "ac-1", Kind: "acceptance-criterion", Text: "a", X: 40, Y: 40},
			{ID: "dc-1", Kind: "decision", Text: "d", X: 496, Y: 40},
		},
		RefCards: []refCardView{{Ref: "adr/0001-outbox-events", X: 1180, Y: 40}},
		Edges: []edgeView{
			{Type: "exempts", From: "dc-1", To: "adr/0001-outbox-events", Layer: "spec"},
			{Type: "implements", From: "spec", To: "spec/parent#ac-3", Layer: "spec"},
			{Type: "relates", From: "ac-1", To: "dc-1", Layer: "annotation", AnnotationID: "a-01J8Z0K3BBBBBBBBBBBBBBBBBB"},
			{Type: "relates", From: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", To: "ac-1", Layer: "annotation", AnnotationID: "a-01J8Z0K3CCCCCCCCCCCCCCCCCC"},
		},
		Stickies: []scratchStickyView{
			{ID: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", Type: "story", Body: "s", X: 952, Y: 40},
			{ID: "a-01J8Z0K3DDDDDDDDDDDDDDDDDD", Type: "comment", Body: "c", X: 1408, Y: 40},
		},
	}
}

// chipsOf slices every yarn chip element out of a rendered region.
func chipsOf(body string) []string {
	var out []string
	rest := body
	for {
		at := strings.Index(rest, `<div class="yarn-chip`)
		if at < 0 {
			return out
		}
		rest = rest[at:]
		end := strings.Index(rest, `</div>`)
		if end < 0 {
			return out
		}
		out = append(out, rest[:end+len(`</div>`)])
		rest = rest[end+len(`</div>`):]
	}
}

// stickiesOf slices every scratch sticky element out of a rendered region
// (through its footer, the sticky's last child).
func stickiesOf(body string) []string {
	var out []string
	rest := body
	for {
		at := strings.Index(rest, `<div class="sticky sticky--`)
		if at < 0 {
			return out
		}
		rest = rest[at:]
		end := strings.Index(rest, `</span></div>`)
		if end < 0 {
			return out
		}
		out = append(out, rest[:end+len(`</span></div>`)])
		rest = rest[end+len(`</span></div>`):]
	}
}

func chipFor(t *testing.T, chips []string, from, to string) string {
	t.Helper()
	for _, c := range chips {
		if strings.Contains(c, `data-from="`+from+`" data-to="`+to+`"`) {
			return c
		}
	}
	t.Fatalf("no chip %s → %s among %d chips", from, to, len(chips))
	return ""
}

// TestWallToolbar_ChipsAreSelectButtonsCarryingTheirLegalActions: every
// chip is the button that selects its thread (role, focusable), holds no
// inner button, and carries exactly the actions legal for it in the mode:
// retype and edge removal on an editable spec edge with the domain live,
// graduation on a card-to-card scratch thread with the domain live, thread
// deletion on every scratch thread in authoring, and nothing anywhere else
// (SI-350 (5); the owner UAT round 6 affordance rules, unchanged).
func TestWallToolbar_ChipsAreSelectButtonsCarryingTheirLegalActions(t *testing.T) {
	for _, tc := range []struct {
		name          string
		mode          boardModeKind
		refusal       string
		editRetype    bool // dc-1 exempts: retype + remove edge
		threadGrad    bool // ac-1 relates dc-1: graduate
		threadDelete  bool // every scratch thread: delete thread
		anyCanAtAll   bool
		editDeleteVal string
	}{
		{name: "authoring, domain live", mode: modeAuthoring, editRetype: true, threadGrad: true, threadDelete: true, anyCanAtAll: true},
		{name: "authoring under a domain refusal", mode: modeAuthoring, refusal: "wrong branch", threadDelete: true, anyCanAtAll: true},
		{name: "review", mode: modeReview},
		{name: "read-only", mode: modeReadOnly},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := renderBoardRegion(toolbarProjection(tc.mode, tc.refusal), &boardGitState{}, testASDView())
			chips := chipsOf(body)
			if len(chips) != 4 {
				t.Fatalf("sliced %d chips, want 4", len(chips))
			}
			// Each chip carries its own stable key, the thread's identity,
			// which the region swap's focus restore reads (Wave 6 §5.1);
			// no two chips share one.
			keys := map[string]bool{}
			for _, chip := range chips {
				if !strings.Contains(chip, ` role="button" tabindex="0" data-edge-type="`) {
					t.Errorf("a chip is not the focusable button that selects its thread:\n%s", chip)
				}
				_, after, found := strings.Cut(chip, ` data-testid="yarn-chip-`)
				if !found {
					t.Errorf("a chip carries no stable yarn-chip- key:\n%s", chip)
				} else if key, _, _ := strings.Cut(after, `"`); keys[key] {
					t.Errorf("two chips share the key %q:\n%s", key, chip)
				} else {
					keys[key] = true
				}
				for _, forbidden := range []string{"<button", "data-retype", "data-delete=", "data-graduate=", "graduate-btn", "delete-btn"} {
					if strings.Contains(chip, forbidden) {
						t.Errorf("a chip still carries %s (the action moved to the toolbar):\n%s", forbidden, chip)
					}
				}
				if !tc.anyCanAtAll && strings.Contains(chip, "data-can-") {
					t.Errorf("%s: a chip offers an action:\n%s", tc.mode, chip)
				}
			}
			edit := chipFor(t, chips, "dc-1", "adr/0001-outbox-events")
			if !strings.Contains(edit, ` data-testid="yarn-chip-spec-exempts-dc-1-adr/0001-outbox-events"`) {
				t.Errorf("the spec edge's key is not its layer, type and endpoints:\n%s", edit)
			}
			if got := strings.Contains(edit, `data-can-retype="true"`); got != tc.editRetype {
				t.Errorf("editable spec edge retype offered = %v, want %v:\n%s", got, tc.editRetype, edit)
			}
			if got := strings.Contains(edit, `data-can-delete="edge"`); got != tc.editRetype {
				t.Errorf("editable spec edge removal offered = %v, want %v:\n%s", got, tc.editRetype, edit)
			}
			if !strings.Contains(edit, `<span class="yarn-chip-type">exempts</span>`) {
				t.Errorf("the chip's type label is not a plain span:\n%s", edit)
			}
			doc := chipFor(t, chips, "spec", "spec/parent#ac-3")
			if strings.Contains(doc, "data-can-") {
				t.Errorf("the document-level chip offers an action the board cannot perform:\n%s", doc)
			}
			if !strings.Contains(doc, `<span class="yarn-chip-doc">this spec</span>`) {
				t.Errorf("the document-level chip lost its off-board label:\n%s", doc)
			}
			thread := chipFor(t, chips, "ac-1", "dc-1")
			if !strings.Contains(thread, ` data-testid="yarn-chip-annotation-relates-ac-1-dc-1-a-01J8Z0K3BBBBBBBBBBBBBBBBBB"`) {
				t.Errorf("the scratch thread's key does not end in its annotation id:\n%s", thread)
			}
			if got := strings.Contains(thread, `data-can-graduate="thread"`); got != tc.threadGrad {
				t.Errorf("card-to-card scratch thread graduation offered = %v, want %v:\n%s", got, tc.threadGrad, thread)
			}
			if got := strings.Contains(thread, `data-can-delete="thread"`); got != tc.threadDelete {
				t.Errorf("scratch thread deletion offered = %v, want %v:\n%s", got, tc.threadDelete, thread)
			}
			attribution := chipFor(t, chips, "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", "ac-1")
			if strings.Contains(attribution, "data-can-graduate") {
				t.Errorf("an attribution thread (a sticky at one end) offers graduation:\n%s", attribution)
			}
			if got := strings.Contains(attribution, `data-can-delete="thread"`); got != tc.threadDelete {
				t.Errorf("attribution thread deletion offered = %v, want %v:\n%s", got, tc.threadDelete, attribution)
			}
		})
	}
}

// TestWallToolbar_StickiesCarryTheirLegalActions: a sticky's Graduate and
// × leave the card for the toolbar (SI-350 (5)); the sticky says what it
// offers: graduation into a stub (proto) or an object (any other type)
// with the domain live, deletion in authoring, nothing elsewhere. Its yarn
// handle and footer are untouched.
func TestWallToolbar_StickiesCarryTheirLegalActions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mode     boardModeKind
		refusal  string
		graduate bool
		del      bool
	}{
		{name: "authoring, domain live", mode: modeAuthoring, graduate: true, del: true},
		{name: "authoring under a domain refusal", mode: modeAuthoring, refusal: "wrong branch", del: true},
		{name: "review", mode: modeReview},
		{name: "read-only", mode: modeReadOnly},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := renderBoardRegion(toolbarProjection(tc.mode, tc.refusal), &boardGitState{}, testASDView())
			stickies := stickiesOf(body)
			if len(stickies) != 2 {
				t.Fatalf("sliced %d stickies, want 2", len(stickies))
			}
			for _, s := range stickies {
				for _, forbidden := range []string{"graduate-btn", "delete-btn", "data-graduate=", "data-delete="} {
					if strings.Contains(s, forbidden) {
						t.Errorf("a sticky still carries %s (the action moved to the toolbar):\n%s", forbidden, s)
					}
				}
				if got := strings.Contains(s, `data-can-delete="sticky"`); got != tc.del {
					t.Errorf("sticky deletion offered = %v, want %v:\n%s", got, tc.del, s)
				}
				proto := strings.Contains(s, `data-annotation-type="story"`)
				want := ""
				if tc.graduate && proto {
					want = `data-can-graduate="stub"`
				} else if tc.graduate {
					want = `data-can-graduate="sticky"`
				}
				if want != "" && !strings.Contains(s, want) {
					t.Errorf("sticky lacks %s:\n%s", want, s)
				}
				if want == "" && strings.Contains(s, "data-can-graduate") {
					t.Errorf("sticky offers graduation where the kernel refuses it:\n%s", s)
				}
				if !strings.Contains(s, `<span class="sticky-foot">`) {
					t.Errorf("sticky lost its footer:\n%s", s)
				}
			}
			// The proto-sticky's attribution handle rides authoring, as before.
			if got := strings.Contains(body, `class="yarn-handle yarn-handle--proto"`); got != (tc.mode == modeAuthoring) {
				t.Errorf("proto-sticky handle rendered = %v, want %v", got, tc.mode == modeAuthoring)
			}
		})
	}
}

// slotProjection: object cards in three columns (two in the AC column),
// an empty open-question column, and a sticky parked in the decision
// column below its card.
func slotProjection(mode boardModeKind, refusal string) *BoardProjection {
	return &BoardProjection{
		Spec: "s", Mode: mode, Class: "story", DomainRefusal: refusal,
		Cards: []cardView{
			{ID: "ac-1", Kind: "acceptance-criterion", Text: "a", X: 40, Y: 40},
			{ID: "ac-2", Kind: "acceptance-criterion", Text: "b", X: 40, Y: 216},
			{ID: "co-1", Kind: "constraint", Text: "c", X: 268, Y: 40},
			{ID: "dc-1", Kind: "decision", Text: "d", X: 496, Y: 40},
		},
		Stickies: []scratchStickyView{{ID: "a-01J8Z0K3DDDDDDDDDDDDDDDDDD", Type: "comment", Body: "c", X: 520, Y: 300}},
	}
}

// TestWallToolbar_AddSlotsAtEachObjectColumnsFoot (ac-5; SI-350 (15)): with
// the domain live, one slot per object column sits below the lowest paper
// whose footprint overlaps the column band, one row gap down (the layout's
// row pitch from the paper's top: last card + 176), at the first row when
// the column is empty; each names its typed operation and prefix so the
// toolbar asset declares with the server's next id; the slot never renders
// where a typed write would be refused.
func TestWallToolbar_AddSlotsAtEachObjectColumnsFoot(t *testing.T) {
	body := renderBoardRegion(slotProjection(modeAuthoring, ""), &boardGitState{}, testASDView())
	for _, want := range []string{
		// AC column: ac-2 (top 216) is the lowest card → 216 + 176. The
		// resting label is the kind's short noun: a button named
		// "acceptance …" would read as a control claiming governance
		// authority (50-design-workbench pins that no browser control does).
		`<div class="wall-slot" data-testid="slot-ac" data-slot-kind="acceptance-criterion" data-slot-op="add-ac" data-slot-prefix="ac" style="left:40px;top:392px"><button type="button" class="wall-slot-open" data-testid="slot-open-ac">+ criterion</button></div>`,
		// Constraint column: co-1 alone → 40 + 176.
		`<div class="wall-slot" data-testid="slot-co" data-slot-kind="constraint" data-slot-op="add-constraint" data-slot-prefix="co" style="left:268px;top:216px"><button type="button" class="wall-slot-open" data-testid="slot-open-co">+ constraint</button></div>`,
		// Decision column: the sticky at (520, 300) overlaps the band and
		// is the lowest paper (its working estimate is 150 tall) → 300 +
		// 150 + 36.
		`<div class="wall-slot" data-testid="slot-dc" data-slot-kind="decision" data-slot-op="add-decision" data-slot-prefix="dc" style="left:496px;top:486px"><button type="button" class="wall-slot-open" data-testid="slot-open-dc">+ decision</button></div>`,
		// Open-question column: empty → the first row.
		`<div class="wall-slot" data-testid="slot-oq" data-slot-kind="open-question" data-slot-op="add-question" data-slot-prefix="oq" style="left:724px;top:40px"><button type="button" class="wall-slot-open" data-testid="slot-open-oq">+ question</button></div>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("slot markup missing:\n%s", want)
		}
	}
	if strings.Count(body, `class="wall-slot"`) != 4 {
		t.Errorf("rendered %d slots, want 4 (one per object column)", strings.Count(body, `class="wall-slot"`))
	}
	// Slots sit inside the canvas (before its close), after the chips, so
	// they are papers of the scroll area, never of the status row.
	canvas := sliceBetween(body, `<div id="board-canvas"`, `<div class="wall-status-row"`)
	if strings.Count(canvas, `class="wall-slot"`) != 4 {
		t.Error("the slots are not all inside the canvas")
	}
	// Every slot's foot is inside the canvas's working height.
	if min := canvasMinHeight(slotProjection(modeAuthoring, "")); min < 486+44 {
		t.Errorf("canvas min-height %v leaves no room for the lowest slot (486+44)", min)
	}
	// Never where a typed write is refused.
	for _, tc := range []struct {
		name    string
		mode    boardModeKind
		refusal string
	}{
		{"authoring under a domain refusal", modeAuthoring, "wrong branch"},
		{"review", modeReview, ""},
		{"read-only", modeReadOnly, ""},
	} {
		if frozen := renderBoardRegion(slotProjection(tc.mode, tc.refusal), &boardGitState{}, testASDView()); strings.Contains(frozen, "wall-slot") {
			t.Errorf("%s: the wall renders an add slot", tc.name)
		}
	}
	// The next ids the slot declares with stay the canvas's.
	if !strings.Contains(body, ` data-next-id-ac="ac-1" data-next-id-co="co-1" data-next-id-dc="dc-1" data-next-id-oq="oq-1"`) {
		t.Error("the canvas lost its data-next-id-* attributes")
	}
}

// TestWallToolbar_SlotOnlyWhereItFits: a slot whose foot would fall past
// the canvas's working height is not rendered (ac-5 "only where it fits"):
// the canvas height is a pure function of the papers, so the check is
// exercised through the one input that can push a slot past it — the
// object column's lowest paper is what sizes the canvas, and the slot is
// one row below its top, so the guard holds by construction here; the
// test pins the arithmetic so a later change to either side is caught.
func TestWallToolbar_SlotOnlyWhereItFits(t *testing.T) {
	p := slotProjection(modeAuthoring, "")
	min := canvasMinHeight(p)
	for _, s := range addSlotsFor(p) {
		if s.Y+addSlotHeight > min {
			t.Errorf("slot %s at %v (foot %v) rendered past the canvas's %v", s.Prefix, s.Y, s.Y+addSlotHeight, min)
		}
	}
	if len(addSlotsFor(p)) != 4 {
		t.Fatalf("addSlotsFor = %d slots, want 4", len(addSlotsFor(p)))
	}
	// A column whose lowest paper is a reference card: the slot clears the
	// shorter footprint by the same gap.
	p.RefCards = []refCardView{{Ref: "adr/x", X: 724, Y: 40}}
	for _, s := range addSlotsFor(p) {
		if s.Prefix == "oq" && s.Y != 40+boardlayout.RefCardHeight+36 {
			t.Errorf("open-question slot at %v, want below the reference card at %v", s.Y, 40+boardlayout.RefCardHeight+36)
		}
	}
}

// TestWallToolbar_RowHostsTheToolbarAndTheTabIsRetired: the status row
// carries the toolbar's host in every mode (the asset fills it; the mode
// decides what it offers), the pin toolbox keeps its tray but loses its
// fixed tab (the toolbar's button controls the tray, SI-350 (10)), and
// the page loads the toolbar asset after the selection asset it acts on.
func TestWallToolbar_RowHostsTheToolbarAndTheTabIsRetired(t *testing.T) {
	for _, mode := range []boardModeKind{modeAuthoring, modeReview, modeReadOnly} {
		region := renderBoardRegion(toolbarProjection(mode, ""), &boardGitState{}, testASDView())
		const row = `</div><div class="wall-status-row" data-testid="wall-status-row"><div class="wall-toolbar" data-testid="wall-toolbar" role="toolbar" aria-label="Wall actions"></div><div class="wall-minimap" data-testid="wall-minimap" aria-hidden="true"></div></div></div><aside class="board-side">`
		if !strings.Contains(region, row) {
			t.Errorf("%s: the status row does not host the toolbar after the canvas, before the rail", mode)
		}
	}
	root := newBoardFixture(t)
	h := NewHandler(root)
	body := getBoard(t, h, boardFixtureName).Body.String()
	for _, want := range []string{`id="pin-tray" role="dialog" aria-label="Pin an artifact"`, `data-testid="pin-toolbox"`, `id="pin-search"`} {
		if !strings.Contains(body, want) {
			t.Errorf("authoring page lacks %s", want)
		}
	}
	if strings.Contains(body, "pin-toolbox-tab") {
		t.Error("the pin toolbox still renders its fixed tab (the toolbar's button controls the tray)")
	}
	scripts := []string{`<script src="/assets/boardspec.js"></script>`, `<script src="/assets/boardspecasd.js"></script>`, `<script src="/assets/wallselect.js"></script>`, `<script src="/assets/walltoolbar.js"></script>`}
	last := -1
	for _, s := range scripts {
		at := strings.Index(body, s)
		if at < 0 {
			t.Errorf("page lacks %s", s)
			continue
		}
		if at < last {
			t.Errorf("%s loads before the script it follows", s)
		}
		last = at
	}
}

// TestWallToolbarAsset_ServedWithinBudget: the toolbar asset ships as its
// own JavaScript file (co-1: new behaviour in new assets of at most 64 KiB;
// boardspec.js does not grow), embedded, non-empty, within the generic
// ceiling, and served at /assets/walltoolbar.js, GET only.
func TestWallToolbarAsset_ServedWithinBudget(t *testing.T) {
	data, err := embeddedAssets.ReadFile("assets/walltoolbar.js")
	if err != nil {
		t.Fatalf("walltoolbar.js is not embedded: %v", err)
	}
	if err := assetBudgetError("walltoolbar.js", len(data)); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(t.TempDir())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/walltoolbar.js", nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/javascript") {
		t.Fatalf("GET /assets/walltoolbar.js = %d (%s), want 200 JavaScript", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != string(data) {
		t.Fatal("/assets/walltoolbar.js does not serve the embedded assets/walltoolbar.js bytes")
	}
	post := httptest.NewRecorder()
	h.ServeHTTP(post, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/assets/walltoolbar.js", nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /assets/walltoolbar.js = %d, want 405", post.Code)
	}
}
