package workbench

import (
	"encoding/json"
	stdhtml "html"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designprovenance"
)

// The Commit and push fragment (spec/wall-strip-and-drawer-v2 ac-3; ledger
// SI-368 (8)): the uncommitted indicator, the "n changes" count, and the
// three-state changes body, rendered from the wall-changes summary the
// snapshot already carries.

// commitChange is a typed change for the fragment's tables.
func commitChange(target string, kind designprovenance.ChangeKind) designprovenance.Change {
	return designprovenance.Change{Target: target, Change: kind}
}

// commitGit is a git state carrying a changes summary.
func commitGit(dirty bool, changes *wallChanges) *boardGitState {
	return &boardGitState{Branch: "design/wall", Dirty: dirty, Changes: changes}
}

// TestWallUncommitted_Derive: the fragment's facts for each state of the
// summary — typed, unclassified, mixed, unreadable, clean — and for the
// two edges: git reporting a change the summary lists none of, and no
// summary at all.
func TestWallUncommitted_Derive(t *testing.T) {
	typed := []designprovenance.Change{commitChange("ac-1", designprovenance.ChangeReplaced)}
	notes := []wallUnclassifiedChange{entry("notes.txt", wallReasonUntracked)}
	for _, tc := range []struct {
		name      string
		git       *boardGitState
		wantSet   bool
		wantState string
		wantCount string
	}{
		{name: "typed only", git: commitGit(true, &wallChanges{Typed: typed, Unclassified: []wallUnclassifiedChange{}}), wantSet: true, wantState: "typed", wantCount: "1 change"},
		{name: "unclassified only", git: commitGit(true, &wallChanges{Typed: []designprovenance.Change{}, Unclassified: notes}), wantSet: true, wantState: "unclassified", wantCount: "1 change"},
		{name: "mixed", git: commitGit(true, &wallChanges{Typed: typed, Unclassified: notes}), wantSet: true, wantState: "mixed", wantCount: "2 changes"},
		{name: "unreadable, listing a change", git: commitGit(true, &wallChanges{Unclassified: notes, UnreadableReason: "HEAD's spec.md could not be read"}), wantSet: true, wantState: "unreadable", wantCount: "unreadable"},
		{name: "unreadable over a clean tree", git: commitGit(false, &wallChanges{Unclassified: []wallUnclassifiedChange{}, UnreadableReason: "never committed"}), wantSet: false, wantState: "unreadable", wantCount: "unreadable"},
		{name: "clean", git: commitGit(false, &wallChanges{Typed: []designprovenance.Change{}, Unclassified: []wallUnclassifiedChange{}}), wantSet: false, wantState: "none", wantCount: "0 changes"},
		{name: "git reports a change the summary does not list", git: commitGit(true, &wallChanges{Typed: []designprovenance.Change{}, Unclassified: []wallUnclassifiedChange{}}), wantSet: true, wantState: "none", wantCount: "0 changes"},
		{name: "no summary computed", git: commitGit(true, nil), wantSet: true, wantState: "unreadable", wantCount: "unreadable"},
		{name: "typed beside an unreadable reason is not counted", git: commitGit(false, &wallChanges{Typed: typed, Unclassified: []wallUnclassifiedChange{}, UnreadableReason: "x"}), wantSet: false, wantState: "unreadable", wantCount: "unreadable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := deriveWallUncommitted(tc.git)
			if u.Set != tc.wantSet {
				t.Errorf("Set = %v, want %v", u.Set, tc.wantSet)
			}
			if got := u.state(); got != tc.wantState {
				t.Errorf("state = %q, want %q", got, tc.wantState)
			}
			if got := u.countLabel(); got != tc.wantCount {
				t.Errorf("count = %q, want %q", got, tc.wantCount)
			}
		})
	}
}

// TestWallUncommitted_ZeroOperationsNeverClearTheIndicator is ac-3's
// zero-operation rule on the fragment: for every state wall-changes' own
// zero-operation table covers (B4, TestWallChanges_ZeroOperationsStayDirty),
// a comparison with no recognized operation leaves the indicator set while
// the remaining change is listed — whatever git status's dirty bit says,
// so the fragment never relies on it alone.
func TestWallUncommitted_ZeroOperationsNeverClearTheIndicator(t *testing.T) {
	inputs := map[string]wallChangesInputs{
		"prose-only":           wallIn(wallChangesProseOnly),
		"layout-only":          wallIn(wallChangesHeadSpec, wallChangedPath{Path: wallTestLayoutPath}),
		"another-staged-path":  wallIn(wallChangesHeadSpec, wallChangedPath{Path: wallTestOtherSpec}),
		"untracked-file":       wallIn(wallChangesHeadSpec, wallChangedPath{Path: "notes.txt", Untracked: true}),
		"title-only":           wallIn(wallChangesTitleOnly),
		"quoting-only":         wallIn(wallChangesQuotingOnly),
		"mode-only":            withStatus(wallIn(wallChangesHeadSpec), wallSpecStatus{Reported: true, ModeChanged: true}),
		"staged-then-reverted": withStatus(wallIn(wallChangesHeadSpec), wallSpecStatus{Reported: true, IndexDiverged: true}),
	}
	for name, in := range inputs {
		for _, dirty := range []bool{false, true} {
			changes, err := classifyWallChanges(in)
			if err != nil {
				t.Fatalf("%s: classifyWallChanges: %v", name, err)
			}
			if len(changes.Typed) != 0 || len(changes.Unclassified) == 0 {
				t.Fatalf("%s: the fixture is not a zero-operation comparison with a remaining change: %+v", name, changes)
			}
			fragment := renderWallUncommitted(deriveWallUncommitted(commitGit(dirty, changes)))
			if strings.Contains(fragment, `data-testid="uncommitted-indicator" hidden`) {
				t.Errorf("%s (dirty=%v): a zero-operation comparison cleared the indicator while a change remains:\n%s", name, dirty, fragment)
			}
			if !strings.Contains(fragment, `data-changes="unclassified"`) || !strings.Contains(fragment, `data-testid="wall-commit-unclassified"`) {
				t.Errorf("%s (dirty=%v): the remaining change is not listed as unclassified:\n%s", name, dirty, fragment)
			}
		}
	}
}

// commitCountRe matches a numeric change count.
var commitCountRe = regexp.MustCompile(`\d+ changes?\b`)

// TestWallUncommitted_UnreadableIsNeverACount: an unreadable comparison
// reads "unreadable" where the count stands — never a number, and never
// "0 changes" — with or without other changes listed, and its reason is
// disclosed.
func TestWallUncommitted_UnreadableIsNeverACount(t *testing.T) {
	for name, changes := range map[string]*wallChanges{
		"nothing else listed":   {Unclassified: []wallUnclassifiedChange{}, UnreadableReason: "the working tree's spec.md could not be read: bad"},
		"other changes listed":  {Unclassified: []wallUnclassifiedChange{entry("a.txt", wallReasonUntracked), entry("b.txt", wallReasonStagedPath)}, UnreadableReason: "HEAD does not name a commit yet"},
		"no summary was loaded": nil,
	} {
		for _, dirty := range []bool{false, true} {
			fragment := renderWallUncommitted(deriveWallUncommitted(commitGit(dirty, changes)))
			if !strings.Contains(fragment, `data-testid="wall-commit-count" title="unreadable">unreadable</summary>`) {
				t.Errorf("%s (dirty=%v): the count does not read unreadable:\n%s", name, dirty, fragment)
			}
			if m := commitCountRe.FindString(fragment); m != "" {
				t.Errorf("%s (dirty=%v): an unreadable comparison renders the count %q:\n%s", name, dirty, m, fragment)
			}
			if !strings.Contains(fragment, `data-testid="wall-commit-unreadable"`) {
				t.Errorf("%s (dirty=%v): the unreadable comparison is not disclosed:\n%s", name, dirty, fragment)
			}
			if changes != nil && !strings.Contains(fragment, stdhtml.EscapeString(changes.UnreadableReason)) {
				t.Errorf("%s (dirty=%v): the disclosure lacks its reason %q:\n%s", name, dirty, changes.UnreadableReason, fragment)
			}
		}
	}
}

// TestWallUncommitted_RenderStructure: the fragment's structural hooks —
// its root with the state, the indicator (hidden only when clear), the
// count, each typed change with its target and badge, each unclassified
// change with its path and reason, the clean line — every value escaped.
func TestWallUncommitted_RenderStructure(t *testing.T) {
	mixed := renderWallUncommitted(deriveWallUncommitted(commitGit(true, &wallChanges{
		Typed: []designprovenance.Change{
			commitChange("ac-3", designprovenance.ChangeAdded),
			commitChange("link/depends-on/spec/base", designprovenance.ChangeRelationshipAdded),
			commitChange("problem", designprovenance.ChangeReplaced),
		},
		Unclassified: []wallUnclassifiedChange{
			entry(".verdi/specs/active/wall/spec.md", wallReasonProse),
			entry(`docs/<odd> & "name".md`, wallReasonUntracked),
		},
	})))
	for _, want := range []string{
		`<div class="wall-commit" data-testid="wall-commit" data-changes="mixed">`,
		`<span class="uncommitted" data-testid="uncommitted-indicator">uncommitted changes</span>`,
		`<summary class="wall-commit-count" data-testid="wall-commit-count" title="5 changes">5<span class="wall-commit-count-word"> changes</span></summary>`,
		`<section class="wall-commit-typed" data-testid="wall-commit-typed">`,
		`<li data-target="ac-3" data-change="added"><span class="wall-commit-target">ac-3</span> <span class="wall-commit-badge" data-badge="added">added</span></li>`,
		`<li data-target="link/depends-on/spec/base" data-change="relationship-added"><span class="wall-commit-target">link/depends-on/spec/base</span> <span class="wall-commit-badge" data-badge="added">added</span></li>`,
		`<li data-target="problem" data-change="replaced"><span class="wall-commit-target">problem</span> <span class="wall-commit-badge" data-badge="edited">edited</span></li>`,
		`<section class="wall-commit-unclassified" data-testid="wall-commit-unclassified">`,
		`<li data-path=".verdi/specs/active/wall/spec.md" data-reason="prose-or-body-text"><span class="wall-commit-path">.verdi/specs/active/wall/spec.md</span> <span class="wall-commit-reason">prose or body text</span></li>`,
		`data-path="docs/&lt;odd&gt; &amp; &#34;name&#34;.md"`,
	} {
		if !strings.Contains(mixed, want) {
			t.Errorf("mixed fragment lacks %s:\n%s", want, mixed)
		}
	}
	if strings.Contains(mixed, `<odd>`) {
		t.Errorf("a path reaches the markup unescaped:\n%s", mixed)
	}
	for _, absent := range []string{`wall-commit-unreadable`, `wall-commit-none`, `wall-commit-unlisted`} {
		if strings.Contains(mixed, absent) {
			t.Errorf("mixed fragment carries %s:\n%s", absent, mixed)
		}
	}

	clean := renderWallUncommitted(deriveWallUncommitted(commitGit(false, &wallChanges{Typed: []designprovenance.Change{}, Unclassified: []wallUnclassifiedChange{}})))
	for _, want := range []string{`data-changes="none"`, `data-testid="uncommitted-indicator" hidden>`, `title="0 changes">0<span class="wall-commit-count-word"> changes</span></summary>`, `data-testid="wall-commit-none"`} {
		if !strings.Contains(clean, want) {
			t.Errorf("clean fragment lacks %s:\n%s", want, clean)
		}
	}
	for _, absent := range []string{`wall-commit-typed`, `wall-commit-unclassified`, `wall-commit-unreadable`} {
		if strings.Contains(clean, absent) {
			t.Errorf("clean fragment carries %s:\n%s", absent, clean)
		}
	}

	unlisted := renderWallUncommitted(deriveWallUncommitted(commitGit(true, &wallChanges{Typed: []designprovenance.Change{}, Unclassified: []wallUnclassifiedChange{}})))
	if !strings.Contains(unlisted, `data-testid="wall-commit-unlisted"`) || strings.Contains(unlisted, `wall-commit-none`) || strings.Contains(unlisted, `uncommitted-indicator" hidden`) {
		t.Errorf("a change git reports but the summary does not list must keep the indicator set and say so, never claim a clean tree:\n%s", unlisted)
	}

	unreadable := renderWallUncommitted(deriveWallUncommitted(commitGit(true, &wallChanges{
		Typed:            []designprovenance.Change{commitChange("ac-1", designprovenance.ChangeReplaced)},
		Unclassified:     []wallUnclassifiedChange{entry("x.md", wallReasonUnrecognizedSpec)},
		UnreadableReason: `HEAD's spec.md could not be read: <bad>`,
	})))
	for _, want := range []string{
		`data-changes="unreadable"`,
		`<p class="wall-commit-unreadable" data-testid="wall-commit-unreadable" role="status">The comparison with HEAD is unreadable: HEAD&#39;s spec.md could not be read: &lt;bad&gt;.</p>`,
		`data-reason="unrecognized-spec-change"`,
	} {
		if !strings.Contains(unreadable, want) {
			t.Errorf("unreadable fragment lacks %s:\n%s", want, unreadable)
		}
	}
	if strings.Contains(unreadable, `wall-commit-typed`) {
		t.Errorf("an unreadable comparison lists typed changes (wall-changes dc-2: its reason instead of a typed list):\n%s", unreadable)
	}
}

// TestWallUncommittedFragment_AuthoringWallsOnly: the fragment exists
// exactly where Commit and push does, on the authoring wall; review and
// read-only walls carry none.
func TestWallUncommittedFragment_AuthoringWallsOnly(t *testing.T) {
	git := commitGit(true, &wallChanges{Typed: []designprovenance.Change{}, Unclassified: []wallUnclassifiedChange{entry("n.txt", wallReasonUntracked)}})
	for _, tc := range []struct {
		mode boardModeKind
		want bool
	}{
		{modeAuthoring, true}, {modeReview, false}, {modeReadOnly, false},
	} {
		got := wallUncommittedFragment(&BoardProjection{Mode: tc.mode}, git)
		if (got != "") != tc.want {
			t.Errorf("%s: fragment = %q, want present=%v", tc.mode, got, tc.want)
		}
		if tc.want && got != renderWallUncommitted(deriveWallUncommitted(git)) {
			t.Errorf("%s: fragment is not the renderer's output over the wall's git state", tc.mode)
		}
	}
}

// TestSnapshotRevision_CoversTheUncommittedFragment: the revision hashes
// the fragment, so a client adopting a revision has the fragment it
// covers (SI-362 (1)); a snapshot without one keeps the token it had.
func TestSnapshotRevision_CoversTheUncommittedFragment(t *testing.T) {
	snapWith := func(fragment string) *asdSnapshot {
		return &asdSnapshot{HTML: "<main>same</main>", Posture: "<section>same</section>", BaseDigest: "d", Git: &boardGitState{}, Uncommitted: fragment}
	}
	base := snapshotRevision(snapWith(`<div data-changes="none"></div>`))
	if again := snapshotRevision(snapWith(`<div data-changes="none"></div>`)); again != base {
		t.Fatalf("revision is not deterministic: %s vs %s", base, again)
	}
	if moved := snapshotRevision(snapWith(`<div data-changes="typed"></div>`)); moved == base {
		t.Error("a different fragment over identical HTML and git leaves the revision unchanged: the token does not hash the fragment")
	}
	if none := snapshotRevision(snapWith("")); none == base {
		t.Error("a snapshot with no fragment shares the token of one with a fragment")
	}
}

// TestWallSnapshot_CarriesTheUncommittedFragment is the served half, over
// every state wall-changes' own served table covers (B4's real git
// fixtures): the snapshot of an authoring wall carries the fragment
// rendered from its own git state, with the state, the indicator and the
// count each served case's summary implies; a wall in another mode
// carries none.
func TestWallSnapshot_CarriesTheUncommittedFragment(t *testing.T) {
	for _, tc := range wallServedCases() {
		if tc.wantCode != 0 && tc.wantCode != http.StatusOK {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			top, root := buildWallFixture(t, tc.prefix, tc.headSpec, tc.extraDraft)
			h := NewHandler(root)
			tc.edit(t, top, root)

			got := fetchWallSnapshot(t, h, wallFixtureName)
			if got.rec.Code != http.StatusOK {
				t.Fatalf("GET snapshot = %d\n%s", got.rec.Code, got.rec.Body.String())
			}
			fragment := got.snap.Uncommitted
			if !strings.Contains(got.snap.HTML, `data-board-mode="authoring"`) {
				if fragment != "" {
					t.Fatalf("a wall outside authoring carries the fragment:\n%s", fragment)
				}
				return
			}
			if fragment == "" || fragment != renderWallUncommitted(deriveWallUncommitted(got.snap.Git)) {
				t.Fatalf("snapshot fragment is not the renderer's output over its own git state:\n%s", fragment)
			}
			typed, uncl := len(tc.wantTyped), len(tc.wantUnclassified)
			state, count := "none", ""
			switch {
			case tc.wantUnreadable != "":
				state, count = "unreadable", "unreadable"
			case typed > 0 && uncl > 0:
				state = "mixed"
			case typed > 0:
				state = "typed"
			case uncl > 0:
				state = "unclassified"
			}
			if count == "" {
				count = asdCountLabel(typed+uncl, "change", "changes")
			}
			if !strings.Contains(fragment, `data-changes="`+state+`"`) {
				t.Errorf("fragment state is not %q:\n%s", state, fragment)
			}
			if !strings.Contains(fragment, `data-testid="wall-commit-count" title="`+count+`">`) {
				t.Errorf("fragment count is not %q:\n%s", count, fragment)
			}
			set := tc.wantDirty || typed > 0 || uncl > 0
			if hidden := strings.Contains(fragment, `uncommitted-indicator" hidden`); hidden == set {
				t.Errorf("indicator hidden=%v, want set=%v (dirty=%v, %d typed, %d unclassified):\n%s", hidden, set, tc.wantDirty, typed, uncl, fragment)
			}
		})
	}
}

// TestMutationResponse_CarriesTheUncommittedFragment: a mutation's fresh
// projection carries the fragment from the same snapshot as its revision —
// the one /snapshot serves for the same state — so a client adopting the
// revision also has the changes the write just made.
func TestMutationResponse_CarriesTheUncommittedFragment(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)
	rec, _ := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "a declined applicant sees the current reason, today", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("mutate_draft = %d\n%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Projection *mutationProjection `json:"projection"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Projection == nil {
		t.Fatalf("decoding the mutation response: %v\n%s", err, rec.Body.String())
	}
	snapRec := httptest.NewRecorder()
	h.ServeHTTP(snapRec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/board/spec/"+boardFixtureName+"/snapshot", nil))
	var snap asdSnapshot
	if err := artifact.DecodeStrictJSON(snapRec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("strict-decoding the snapshot: %v", err)
	}
	if out.Projection.Revision != snap.Revision {
		t.Fatalf("mutation revision %s, snapshot revision %s: the fixture must hold the same state", out.Projection.Revision, snap.Revision)
	}
	if out.Projection.Uncommitted == "" || out.Projection.Uncommitted != snap.Uncommitted {
		t.Fatalf("mutation fragment =\n%q\n/snapshot fragment =\n%q", out.Projection.Uncommitted, snap.Uncommitted)
	}
	if !strings.Contains(out.Projection.Uncommitted, `<li data-target="ac-1" data-change="replaced">`) {
		t.Errorf("the mutation's fragment does not list the typed change it made:\n%s", out.Projection.Uncommitted)
	}
}

// TestWallUncommitted_BadgeWords is SI-368 (25)(a): a typed change's
// semantic kind maps onto ac-3's three badge words — added and
// relationship added read "added", replaced reads "edited", removed,
// reordered and relationship removed read "changed" — while the raw kind
// stays in data-change. A kind the provenance record does not declare is
// rendered as its own plain words, never folded into one of the three.
func TestWallUncommitted_BadgeWords(t *testing.T) {
	for _, tc := range []struct {
		kind designprovenance.ChangeKind
		want string
	}{
		{designprovenance.ChangeAdded, "added"},
		{designprovenance.ChangeRelationshipAdded, "added"},
		{designprovenance.ChangeReplaced, "edited"},
		{designprovenance.ChangeRemoved, "changed"},
		{designprovenance.ChangeReordered, "changed"},
		{designprovenance.ChangeRelationshipRemoved, "changed"},
		{designprovenance.ChangeKind("frob-nicated"), "frob nicated"},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			if got := uncommittedBadgeWord(tc.kind); got != tc.want {
				t.Fatalf("uncommittedBadgeWord(%q) = %q, want %q", tc.kind, got, tc.want)
			}
			fragment := renderWallUncommitted(deriveWallUncommitted(commitGit(true, &wallChanges{
				Typed:        []designprovenance.Change{commitChange("ac-1", tc.kind)},
				Unclassified: []wallUnclassifiedChange{},
			})))
			want := `<li data-target="ac-1" data-change="` + string(tc.kind) + `"><span class="wall-commit-target">ac-1</span> <span class="wall-commit-badge" data-badge="` + tc.want + `">` + tc.want + `</span></li>`
			if !strings.Contains(fragment, want) {
				t.Errorf("fragment lacks %s:\n%s", want, fragment)
			}
		})
	}
}

// TestWallUncommitted_UnclassifiedCap is SI-368 (25)(e): the popover lists
// at most uncommittedUnclassifiedCap unclassified entries, then "+n more";
// a list at the cap carries no "+0 more"; and the count still counts every
// change, listed or not, so the cap never understates what is uncommitted.
func TestWallUncommitted_UnclassifiedCap(t *testing.T) {
	entries := func(n int) []wallUnclassifiedChange {
		out := make([]wallUnclassifiedChange, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, entry("notes/"+strconv.Itoa(i)+".md", wallReasonUntracked))
		}
		return out
	}
	for _, tc := range []struct {
		n          int
		wantListed int
		wantMore   string
	}{
		{n: 1, wantListed: 1},
		{n: uncommittedUnclassifiedCap, wantListed: uncommittedUnclassifiedCap},
		{n: uncommittedUnclassifiedCap + 1, wantListed: uncommittedUnclassifiedCap, wantMore: "+1 more"},
		{n: uncommittedUnclassifiedCap + 5, wantListed: uncommittedUnclassifiedCap, wantMore: "+5 more"},
	} {
		t.Run(strconv.Itoa(tc.n), func(t *testing.T) {
			fragment := renderWallUncommitted(deriveWallUncommitted(commitGit(true, &wallChanges{
				Typed:        []designprovenance.Change{commitChange("ac-1", designprovenance.ChangeReplaced)},
				Unclassified: entries(tc.n),
			})))
			if got := strings.Count(fragment, `<li data-path="`); got != tc.wantListed {
				t.Errorf("listed %d unclassified entries, want %d:\n%s", got, tc.wantListed, fragment)
			}
			more := `<li class="wall-commit-more" data-testid="wall-commit-more">` + tc.wantMore + `</li>`
			if tc.wantMore == "" {
				if strings.Contains(fragment, "wall-commit-more") {
					t.Errorf("a list within the cap carries a more line:\n%s", fragment)
				}
			} else if !strings.Contains(fragment, more) {
				t.Errorf("fragment lacks %s:\n%s", more, fragment)
			}
			count := asdCountLabel(tc.n+1, "change", "changes")
			if !strings.Contains(fragment, `data-testid="wall-commit-count" title="`+count+`">`) {
				t.Errorf("the count does not read %q over every change:\n%s", count, fragment)
			}
		})
	}
}
