package workbench

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
	"github.com/jyang234/verdi/internal/specdocload"
)

// The board's closed-spec object supersession surface (design §6, §8;
// SI-278 and the controller's rulings on the L5 stop report): a closed
// spec's reference card renders the object's original text and the
// objsupersede views' lines computed from DEFAULT-BRANCH records, only
// where the views report lines; the successor's own decision card renders
// its decision views from the board's OWN tree, so a design branch reads
// "proposed". get_board's JSON gains only additive, omitted-when-empty
// fields. Every line is the views' text verbatim (specdoc's one
// conversion); the board adds markup and link targets only.

const (
	obsFeatureObject = "spec/closed-feature#dc-1"
	obsStoryObject   = "spec/closed-story#ac-1"
	obsFeatureText   = "the governed records are listed newest first"
	obsStoryText     = "the record list renders every governed record"
	obsGovernedF     = "governed spec/closed-feature's completed work (closed 2024-01-10)"
	obsSinceF        = "superseded since 2024-02-15 by spec/successor#dc-1"
	obsSinceS        = "superseded since 2024-02-15 by spec/successor#dc-2"
	obsConflictF     = "conflict/successor-closed-feature"
	obsConflictS     = "conflict/successor-closed-story"
)

// obsBoard loads the board projection of spec name over root exactly as
// get_board does (no feed, no forge).
func obsBoard(t *testing.T, root, name string) *BoardProjection {
	t.Helper()
	proj, _, err := LoadProjection(context.Background(), root, name, nil, "", nil)
	if err != nil {
		t.Fatalf("LoadProjection(%s): %v", name, err)
	}
	return proj
}

func obsRefCard(t *testing.T, p *BoardProjection, ref string) *refCardView {
	t.Helper()
	for i := range p.RefCards {
		if p.RefCards[i].Ref == ref {
			return &p.RefCards[i]
		}
	}
	t.Fatalf("no reference card for %s on %s: %+v", ref, p.Spec, p.RefCards)
	return nil
}

func obsCard(t *testing.T, p *BoardProjection, id string) *cardView {
	t.Helper()
	for i := range p.Cards {
		if p.Cards[i].ID == id {
			return &p.Cards[i]
		}
	}
	t.Fatalf("no card %s on %s", id, p.Spec)
	return nil
}

func TestObjectSupersession_AfterAcceptance(t *testing.T) {
	neutralizeCIEnv(t)
	repo := scenario.Build(t, "accepted")
	p := obsBoard(t, repo.Dir, "successor")

	rc := obsRefCard(t, p, obsFeatureObject)
	if rc.Object == nil {
		t.Fatalf("the closed decision's reference card carries no object: %+v", rc)
	}
	if rc.Object.Text != obsFeatureText {
		t.Errorf("Object.Text = %q, want the original text %q", rc.Object.Text, obsFeatureText)
	}
	s := rc.Object.Supersession
	if s.State != "superseded" || s.By != "spec/successor#dc-1" || s.Conflict != obsConflictF || s.Since != "2024-02-15" || s.Closed != "2024-01-10" || s.Carry != "" {
		t.Errorf("structured fields = %+v", s)
	}
	if len(s.Lines) != 2 || s.Lines[0].Kind != "governed" || s.Lines[0].Text != obsGovernedF || s.Lines[1].Kind != "since" || s.Lines[1].Text != obsSinceF {
		t.Fatalf("lines = %+v", s.Lines)
	}
	// Links: S's decision is a card on this very board; the conflict has
	// its corpus page on the serving checkout.
	if want := []supersessionLinkView{{Ref: "spec/successor#dc-1", Href: "#obj-dc-1"}}; !equalLinks(s.Lines[1].Links, want) {
		t.Errorf("since links = %+v, want %+v", s.Lines[1].Links, want)
	}
	if want := []supersessionLinkView{{Ref: obsConflictF, Href: "/a/" + obsConflictF}}; !equalLinks(s.Lines[1].Trailing, want) {
		t.Errorf("since trailing = %+v, want %+v", s.Lines[1].Trailing, want)
	}
	if len(s.Lines[0].Links) != 0 {
		t.Errorf("the governed line names this closed spec only; got links %+v", s.Lines[0].Links)
	}
	story := obsRefCard(t, p, obsStoryObject)
	if story.Object == nil || story.Object.Text != obsStoryText || story.Object.Supersession.Lines[1].Text != obsSinceS {
		t.Errorf("closed-story#ac-1 reference card = %+v", story.Object)
	}

	// The successor's decision cards carry the decision view, in force,
	// linking to the archived object's corpus page (the board route 404s
	// on the archive zone; ADJ-39).
	dc1 := obsCard(t, p, "dc-1")
	if len(dc1.Supersessions) != 1 {
		t.Fatalf("dc-1 supersessions = %+v, want one", dc1.Supersessions)
	}
	d := dc1.Supersessions[0]
	if d.State != "in-force" || d.Object != obsFeatureObject || d.Edge != obsFeatureObject || d.Establisher != "spec/successor" || d.Since != "2024-02-15" || d.Conflict != obsConflictF || d.Carried {
		t.Errorf("dc-1 decision view = %+v", d)
	}
	if len(d.Lines) != 1 || d.Lines[0].Kind != "edge" || d.Lines[0].Text != "supersedes "+obsFeatureObject {
		t.Errorf("dc-1 lines = %+v", d.Lines)
	}
	if want := []supersessionLinkView{{Ref: obsFeatureObject, Href: "/a/spec/closed-feature#dc-1"}}; !equalLinks(d.Lines[0].Links, want) {
		t.Errorf("dc-1 edge links = %+v, want %+v", d.Lines[0].Links, want)
	}
	if obsCard(t, p, "dc-2").Supersessions[0].Object != obsStoryObject {
		t.Errorf("dc-2 supersession = %+v", obsCard(t, p, "dc-2").Supersessions)
	}
	if ac := obsCard(t, p, "ac-1"); ac.Supersessions != nil {
		t.Errorf("ac-1 gained supersessions: %+v", ac.Supersessions)
	}
}

func TestObjectSupersession_Scenarios(t *testing.T) {
	neutralizeCIEnv(t)
	type want struct {
		ref    string // a reference card's ref, or a card id
		object bool   // a reference card: whether Object is set
		lines  string // the lines' texts, " | "-joined
		links  string // the last line's links, "ref->href" ", "-joined
	}
	tests := []struct {
		name, store, board string
		wants              []want
	}{
		{"carried through two revisions: S1's date kept, carried by the head", "chain", "successor-v3", []want{
			{obsFeatureObject, true, obsGovernedF + " | " + obsSinceF + " | carried by spec/successor-v3", ""},
			{"dc-2", false, "supersedes " + obsStoryObject + " | carries the replacement established by spec/successor (conflict/successor-closed-story, since 2024-02-15)",
				"spec/successor->/board/spec/successor, conflict/successor-closed-story->/a/conflict/successor-closed-story"},
		}},
		{"a revision drops the edge: the establishing successor's board (the fixture's object_board)", "chain-drop", "successor", []want{
			{obsFeatureObject, true, obsGovernedF + " | " + obsSinceF + " | no longer carried by the current revision (spec/successor-v2)", "spec/successor-v2->/board/spec/successor-v2"},
			{"dc-1", false, "supersedes " + obsFeatureObject, obsFeatureObject + "->/a/spec/closed-feature#dc-1"},
		}},
		{"not yet accepted: the design board reads proposed, the object untouched", "proposed", "successor", []want{
			{obsFeatureObject, false, "", ""},
			{obsStoryObject, false, "", ""},
			{"dc-1", false, "proposed — supersedes " + obsFeatureObject + " when spec/successor is accepted", obsFeatureObject + "->/a/spec/closed-feature#dc-1"},
			{"dc-2", false, "proposed — supersedes " + obsStoryObject + " when spec/successor is accepted", obsStoryObject + "->/a/spec/closed-story#ac-1"},
		}},
		{"not established on the design branch: the reason, never a supersession", "no-conflict", "successor", []want{
			{obsFeatureObject, false, "", ""},
			{"dc-1", false, "supersession not established: no conflict challenges " + obsFeatureObject, ""},
			{"dc-2", false, "proposed — supersedes " + obsStoryObject + " when spec/successor is accepted", obsStoryObject + "->/a/spec/closed-story#ac-1"},
		}},
		{"not in force after acceptance: reasons on the revision's decisions; the story's object still superseded from default-branch records", "chain-not-in-force", "successor-v2", []want{
			{obsFeatureObject, false, "", ""},
			{"spec/closed-feature#ac-1", false, "", ""},
			{obsStoryObject, true, "governed spec/closed-story's completed work (closed 2024-01-10) | " + obsSinceS, "spec/successor#dc-2->/board/spec/successor#obj-dc-2"},
			{"dc-1", false, "supersession not established: spec/successor's supersession was not in force at its acceptance: as of commit 092a88215473, no conflict challenges spec/closed-feature#ac-1", ""},
			{"dc-3", false, "supersession not established: no conflict challenges spec/closed-feature#ac-1", ""},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := scenario.Build(t, tc.store)
			p := obsBoard(t, repo.Dir, tc.board)
			for _, w := range tc.wants {
				var s *supersessionView
				if strings.HasPrefix(w.ref, "spec/") {
					rc := obsRefCard(t, p, w.ref)
					if (rc.Object != nil) != w.object {
						t.Errorf("%s: Object set = %v, want %v (%+v)", w.ref, rc.Object != nil, w.object, rc.Object)
						continue
					}
					if rc.Object == nil {
						continue
					}
					s = &rc.Object.Supersession
				} else {
					c := obsCard(t, p, w.ref)
					if w.lines == "" {
						if c.Supersessions != nil {
							t.Errorf("%s: supersessions = %+v, want none", w.ref, c.Supersessions)
						}
						continue
					}
					if len(c.Supersessions) != 1 {
						t.Errorf("%s: %d supersessions, want 1: %+v", w.ref, len(c.Supersessions), c.Supersessions)
						continue
					}
					s = &c.Supersessions[0]
				}
				var texts []string
				for _, l := range s.Lines {
					texts = append(texts, l.Text)
				}
				if got := strings.Join(texts, " | "); got != w.lines {
					t.Errorf("%s lines:\n got %q\nwant %q", w.ref, got, w.lines)
				}
				var links []string
				for _, l := range s.Lines[len(s.Lines)-1].Links {
					links = append(links, l.Ref+"->"+l.Href)
				}
				if got := strings.Join(links, ", "); got != w.links {
					t.Errorf("%s last line links:\n got %q\nwant %q", w.ref, got, w.links)
				}
			}
		})
	}
}

func TestBoardSupersessionLink(t *testing.T) {
	neutralizeCIEnv(t)
	repo := scenario.Build(t, "accepted")
	ix := mustIndex(t, repo.Dir)
	views, err := specdocload.WorkTreeViews(context.Background(), repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, ref, own, fixedBranch, want string
	}{
		{"own decision: the card on this board", "spec/successor#dc-1", "successor", "", "#obj-dc-1"},
		{"own spec, whole: no link to itself", "spec/successor", "successor", "", ""},
		{"an active spec's object: its board", "spec/successor#dc-1", "other", "", "/board/spec/successor#obj-dc-1"},
		{"an active spec, whole: its board", "spec/successor", "other", "", "/board/spec/successor"},
		{"an archived spec's object: its corpus page and anchor (ADJ-39)", "spec/closed-feature#dc-1", "successor", "", "/a/spec/closed-feature#dc-1"},
		{"a conflict: its corpus page", "conflict/successor-closed-feature", "successor", "", "/a/conflict/successor-closed-feature"},
		{"a spec the index lacks", "spec/nowhere#dc-1", "successor", "", ""},
		{"a conflict the index lacks", "conflict/nowhere", "successor", "", ""},
		{"a pinned ref is never linked", "spec/successor@0123abc#dc-1", "other", "", ""},
		{"not a ref", "nonsense", "successor", "", ""},
		{"per-branch board: own decision stays on the page", "spec/successor#dc-1", "successor", "design/successor", "#obj-dc-1"},
		{"per-branch board: another active spec's board keeps the branch prefix (ADJ-70)", "spec/successor#dc-1", "other", "design/x", "/b/design%2Fx/board/spec/successor#obj-dc-1"},
		{"per-branch board: no corpus page is provably served for an archived spec", "spec/closed-feature#dc-1", "successor", "design/x", ""},
		{"per-branch board: nor for a conflict", "conflict/successor-closed-feature", "successor", "design/x", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := boardSupersessionLink(tc.ref, tc.own, ix, tc.fixedBranch, views); got != tc.want {
				t.Errorf("boardSupersessionLink(%q, own %q, branch %q) = %q, want %q", tc.ref, tc.own, tc.fixedBranch, got, tc.want)
			}
		})
	}
}

// TestObjectSupersession_Render: the ref card and the decision card carry
// the lines as markup — the same inline markup the docs site writes
// (specdoc.SupersessionLineMarkup) — and a projection without views
// renders byte-identically to before.
func TestObjectSupersession_Render(t *testing.T) {
	base := &BoardProjection{
		Spec: "successor", Title: "Successor", Mode: modeReadOnly, Class: "feature", Status: "accepted-pending-build",
		Cards:    []cardView{{ID: "dc-1", Kind: "decision", Text: "grouped by owner"}},
		RefCards: []refCardView{{Ref: obsFeatureObject}},
	}
	plain, err := renderBoardSpecPage(base, &boardGitState{}, testASDView())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "objsupersede") || strings.Contains(string(plain), "refcard--object") {
		t.Fatalf("a projection without views rendered supersession markup:\n%s", plain)
	}

	with := *base
	with.Cards = []cardView{{ID: "dc-1", Kind: "decision", Text: "grouped by owner", Supersessions: []supersessionView{{
		State: "in-force", Object: obsFeatureObject, Edge: obsFeatureObject,
		Lines: []supersessionLineView{{Kind: "edge", Text: "supersedes " + obsFeatureObject, Links: []supersessionLinkView{{Ref: obsFeatureObject, Href: "/a/spec/closed-feature#dc-1"}}}},
	}}}}
	with.RefCards = []refCardView{{Ref: obsFeatureObject, Object: &refObjectView{Text: obsFeatureText, Supersession: supersessionView{
		State: "superseded",
		Lines: []supersessionLineView{
			{Kind: "governed", Text: obsGovernedF},
			{Kind: "since", Text: obsSinceF, Links: []supersessionLinkView{{Ref: "spec/successor#dc-1", Href: "#obj-dc-1"}}, Trailing: []supersessionLinkView{{Ref: obsConflictF, Href: "/a/" + obsConflictF}}},
		},
	}}}}
	out, err := renderBoardSpecPage(&with, &boardGitState{}, testASDView())
	if err != nil {
		t.Fatal(err)
	}
	page := string(out)
	for _, want := range []string{
		`class="refcard refcard--object" data-testid="ref-card-spec-closed-feature#dc-1"`,
		`<p class="card-text refcard-object-text" data-testid="refcard-object-text" title="` + obsFeatureText + `">` + obsFeatureText + `</p>`,
		`<ul class="objsupersede-lines" data-testid="refcard-supersession" data-state="superseded">`,
		`<li><span class="objsupersede objsupersede--superseded" data-testid="objsupersede-spec-closed-feature-dc-1-governed" data-state="superseded">` + obsGovernedF + `</span></li>`,
		`<li><span class="objsupersede objsupersede--superseded" data-testid="objsupersede-spec-closed-feature-dc-1-since" data-state="superseded">superseded since 2024-02-15 by <a href="#obj-dc-1">spec/successor#dc-1</a></span> <a class="objsupersede-conflict" data-testid="objsupersede-spec-closed-feature-dc-1-conflict" href="/a/conflict/successor-closed-feature">conflict/successor-closed-feature</a></li>`,
		`<ul class="objsupersede-lines" data-testid="card-supersession-dc-1">`,
		`<li><span class="objsupersede objsupersede--in-force" data-testid="objsupersede-dc-1-spec-closed-feature-dc-1-edge" data-state="in-force">supersedes <a href="/a/spec/closed-feature#dc-1">spec/closed-feature#dc-1</a></span></li>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q:\n%s", want, page)
		}
	}
	// The fragment (the post-mutation and poll re-render) carries the same markup.
	if frag := renderBoardRegion(&with, &boardGitState{}, testASDView()); !strings.Contains(frag, `data-testid="refcard-supersession"`) || !strings.Contains(frag, `data-testid="card-supersession-dc-1"`) {
		t.Errorf("fragment lacks the supersession markup")
	}
}

// TestObjectSupersession_WireOmitsEmpty: get_board marshals cardView and
// refCardView directly, so the new fields must vanish from the wire when
// unset and appear, additively, when set.
func TestObjectSupersession_WireOmitsEmpty(t *testing.T) {
	plain, err := json.Marshal(BoardProjection{Cards: []cardView{{ID: "dc-1", Kind: "decision"}}, RefCards: []refCardView{{Ref: obsFeatureObject}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"supersessions"`, `"object"`} {
		if strings.Contains(string(plain), key) {
			t.Errorf("unset field %s reached the wire: %s", key, plain)
		}
	}
	with, err := json.Marshal(BoardProjection{
		Cards:    []cardView{{ID: "dc-1", Kind: "decision", Supersessions: []supersessionView{{State: "proposed", Object: obsFeatureObject, Edge: obsFeatureObject, Lines: []supersessionLineView{{Kind: "edge", Text: "x"}}}}}},
		RefCards: []refCardView{{Ref: obsFeatureObject, Object: &refObjectView{Text: obsFeatureText, Supersession: supersessionView{State: "superseded", By: "spec/s#dc-1", Conflict: "conflict/c", Since: "2024-02-15", Closed: "2024-01-10", Lines: []supersessionLineView{{Kind: "governed", Text: "g"}}}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"supersessions":[{"state":"proposed","object":"spec/closed-feature#dc-1","edge":"spec/closed-feature#dc-1","lines":[{"kind":"edge","text":"x"}]}]`,
		`"object":{"text":"the governed records are listed newest first","supersession":{"state":"superseded","by":"spec/s#dc-1","conflict":"conflict/c","since":"2024-02-15","closed":"2024-01-10","lines":[{"kind":"governed","text":"g"}]}}`,
	} {
		if !strings.Contains(string(with), want) {
			t.Errorf("wire lacks %s:\n%s", want, with)
		}
	}
}

// mustIndex builds the corpus index over root, as loadBoard does per request.
func mustIndex(t *testing.T, root string) *index.Index {
	t.Helper()
	ix, err := index.Build(root)
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func equalLinks(got, want []supersessionLinkView) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
