package workbench

import (
	"context"
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/objsupersede"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
)

// Lane L5 fix pass 2 on the board (the board review's I-2, M-2..M-6 and
// the docs review's minor (b)): reserved heights so every §6 line shows
// at rest and covers no card; declared anchors on archived targets; the
// enrichment's error paths; and the per-branch Document tab's links.

// gitIn runs git in dir with a fixed identity for a test fixture.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid",
		"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid",
		"GIT_AUTHOR_DATE=2024-03-01T00:00:00Z", "GIT_COMMITTER_DATE=2024-03-01T00:00:00Z")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestCardHeights(t *testing.T) {
	plainRef := refCardView{Ref: obsFeatureObject}
	if got := refCardHeightPx(plainRef); got != 72 {
		t.Errorf("a reference card without an object = %v, want the 72 px footprint", got)
	}
	plainCard := cardView{ID: "dc-1", Kind: "decision"}
	if got := cardHeightPx(plainCard); got != 140 {
		t.Errorf("a card without views = %v, want the 140 px footprint", got)
	}
	withObject := refCardView{Ref: obsFeatureObject, Object: &refObjectView{Text: obsFeatureText, Supersession: supersessionView{
		State: "superseded",
		Lines: []supersessionLineView{
			{Kind: "governed", Text: obsGovernedF},
			{Kind: "since", Text: obsSinceF, Trailing: []supersessionLinkView{{Ref: obsConflictF, Href: "/a/" + obsConflictF}}},
		},
	}}}
	two := refCardHeightPx(withObject)
	if two <= 72 {
		t.Fatalf("a reference card with two lines = %v, want taller than its footprint", two)
	}
	withObject.Object.Supersession.Lines = append(withObject.Object.Supersession.Lines, supersessionLineView{Kind: "carry", Text: "carried by spec/successor-v3"})
	if three := refCardHeightPx(withObject); three <= two {
		t.Errorf("a third line did not grow the card: %v then %v", two, three)
	}
	// A longer line wraps and grows the estimate; an empty text adds nothing.
	long := withObject
	long.Object = &refObjectView{Text: "", Supersession: supersessionView{Lines: []supersessionLineView{{Kind: "unproven", Text: strings.Repeat("x", 200)}}}}
	short := withObject
	short.Object = &refObjectView{Text: "", Supersession: supersessionView{Lines: []supersessionLineView{{Kind: "unproven", Text: "x"}}}}
	if refCardHeightPx(long) <= refCardHeightPx(short) {
		t.Errorf("a 200-character line did not wrap taller than a 1-character line")
	}
	decision := cardView{ID: "dc-1", Kind: "decision", Supersessions: []supersessionView{{State: "in-force", Lines: []supersessionLineView{{Kind: "edge", Text: "supersedes " + obsFeatureObject}}}}}
	if got := cardHeightPx(decision); got <= 140 {
		t.Errorf("a decision card with a view = %v, want taller than its footprint", got)
	}
}

// TestLoadProjection_ReservesHeights: the layout reserves each grown
// card's height, so the reference cards on the accepted board never
// overlap and the second sits below the first's full height.
func TestLoadProjection_ReservesHeights(t *testing.T) {
	neutralizeCIEnv(t)
	repo := scenario.Build(t, "accepted")
	p := obsBoard(t, repo.Dir, "successor")
	feature := obsRefCard(t, p, obsFeatureObject)
	story := obsRefCard(t, p, obsStoryObject)
	if feature.Object == nil || story.Object == nil {
		t.Fatal("both reference cards must carry objects on the accepted board")
	}
	first, second := feature, story
	if second.Y < first.Y {
		first, second = second, first
	}
	if second.Y < first.Y+refCardHeightPx(*first) {
		t.Errorf("the second reference card (y=%v) sits inside the first's reserved height (y=%v, h=%v)", second.Y, first.Y, refCardHeightPx(*first))
	}
	// The decision cards carrying views are reserved likewise.
	var dc1, dc2 *cardView
	for i := range p.Cards {
		switch p.Cards[i].ID {
		case "dc-1":
			dc1 = &p.Cards[i]
		case "dc-2":
			dc2 = &p.Cards[i]
		}
	}
	if dc1 == nil || dc2 == nil || len(dc1.Supersessions) == 0 {
		t.Fatal("decision cards missing or without views")
	}
	if dc2.Y < dc1.Y+cardHeightPx(*dc1) {
		t.Errorf("dc-2 (y=%v) sits inside dc-1's reserved height (y=%v, h=%v)", dc2.Y, dc1.Y, cardHeightPx(*dc1))
	}
}

// TestBoardSupersessionLink_DeclaredAnchor (the board review's M-3): an
// archived object's corpus link uses the object's DECLARED anchor — the
// same fragment every document consumer links — never the raw id, and no
// link at all when the records do not declare the object.
func TestBoardSupersessionLink_DeclaredAnchor(t *testing.T) {
	neutralizeCIEnv(t)
	repo := scenario.Build(t, "accepted")
	ix := mustIndex(t, repo.Dir)
	fake := &specdocload.Views{Records: &objsupersede.Records{Specs: map[string]*objsupersede.Spec{
		"closed-feature": {Name: "closed-feature", FM: &artifact.SpecFrontmatter{Decisions: []artifact.Decision{{ID: "dc-1", Anchor: "#decision-one"}}}},
	}}}
	if got := boardSupersessionLink("spec/closed-feature#dc-1", "successor", ix, "", fake); got != "/a/spec/closed-feature#decision-one" {
		t.Errorf("declared anchor: got %q", got)
	}
	if got := boardSupersessionLink("spec/closed-feature#dc-9", "successor", ix, "", fake); got != "" {
		t.Errorf("an undeclared object linked %q", got)
	}
	real, err := specdocload.WorkTreeViews(context.Background(), repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := boardSupersessionLink("spec/closed-feature#dc-1", "successor", ix, "", real); got != "/a/spec/closed-feature#dc-1" {
		t.Errorf("the fixture's anchor equals its id: got %q", got)
	}
}

// TestLoadProjection_SupersessionErrors (the board review's M-5): when the
// views cannot be computed the board load fails — never a board that
// renders a superseded object as untouched — and recovers once the
// store does.
func TestLoadProjection_SupersessionErrors(t *testing.T) {
	neutralizeCIEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, "accepted")
	blob, err := gitx.RevParse(ctx, repo.Dir, "HEAD:.verdi/verdi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo.Dir, "update-ref", "refs/remotes/origin/main", blob)
	// The board load fails (the state resolver meets the bad ref first),
	// and the enrichment itself names the views' failure.
	if _, _, err := LoadProjection(ctx, repo.Dir, "successor", nil, "", nil); err == nil {
		t.Fatal("LoadProjection succeeded over a default branch that is not a commit")
	}
	if _, err := computeObjectSupersession(ctx, "successor", &artifact.SpecFrontmatter{}, mustIndex(t, repo.Dir), repo.Dir, ""); err == nil || !strings.Contains(err.Error(), "closed-spec object supersession views") {
		t.Fatalf("computeObjectSupersession over a default branch that is not a commit: err = %v, want the views' error", err)
	}
	gitIn(t, repo.Dir, "update-ref", "refs/remotes/origin/main", "main")
	if _, _, err := LoadProjection(ctx, repo.Dir, "successor", nil, "", nil); err != nil {
		t.Fatalf("after the remedy: %v", err)
	}
}

// TestLoadDocument_PerBranchLinksNoCorpusPage (the docs review's minor
// (b)): a per-branch Document tab serves the branch's tree while the
// corpus route serves the serving checkout's, so it links no corpus page
// — the board cards' posture — while the serving checkout's tab does.
func TestLoadDocument_PerBranchLinksNoCorpusPage(t *testing.T) {
	neutralizeCIEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, "proposed") // checkout design/successor
	serving := &boardSpecServer{root: repo.Dir}
	snap, err := serving.loadDocument(ctx, "successor", specdoc.KindSpec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(snap.Markdown, `<a href="/a/spec/closed-feature#dc-1">spec/closed-feature#dc-1</a>`) {
		t.Fatalf("the serving checkout's Document tab lacks the corpus link:\n%s", snap.Markdown)
	}
	branch := &boardSpecServer{root: repo.Dir, fixedBranch: "design/successor"}
	snap, err = branch.loadDocument(ctx, "successor", specdoc.KindSpec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(snap.Markdown, `data-testid="objsupersede-dc-1-spec-closed-feature-dc-1-edge"`) {
		t.Fatalf("the per-branch Document tab lacks the decision view:\n%s", snap.Markdown)
	}
	if strings.Contains(snap.Markdown, `href="/a/`) {
		t.Fatalf("the per-branch Document tab links a corpus page it cannot serve:\n%s", snap.Markdown)
	}
}

// TestWrappedLines (the board closure's C-4): the estimate simulates word
// wrap — the reviewer's witness, tokens of about 15 characters, needs one
// line each at 24 characters per line, where a character count sees two
// per line — and breaks an unbreakable token anywhere.
func TestWrappedLines(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       float64
	}{
		{"empty", "", 0},
		{"one short token", "x", 1},
		{"fits one line", "supersedes spec/t#dc-1", 1},
		{"three 15-character tokens: one per line", "aaaaaaaaaaaaaaa bbbbbbbbbbbbbbb ccccccccccccccc", 3},
		{"the reviewer's shape: 2 per line by count, 1 per line by wrap", strings.Repeat("abcdefghijklmno ", 6), 6},
		{"an unbreakable 50-character token breaks across three lines", strings.Repeat("x", 50), 3},
		{"a long token after a short one", "ab " + strings.Repeat("y", 30), 3},
		{"exactly 24 characters fill one line", strings.Repeat("z", 24), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := wrappedLines(tc.text, 24); got != tc.want {
				t.Errorf("wrappedLines(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
	// A card estimate is never below the character-count estimate.
	long := strings.Repeat("abcdefghijklmno ", 6)
	if wrappedLines(long, 24) < math.Ceil(float64(len(strings.TrimSpace(long)))/24) {
		t.Errorf("word wrap estimated fewer lines than a character count")
	}
}

// TestCardHeights_BadgesAndFullText: a decision card with lines and wall
// badges reserves the badge rows (the docs closure's B-1); a reference
// card reserves its whole original text (minor d), never a clamp.
func TestCardHeights_BadgesAndFullText(t *testing.T) {
	lines := []supersessionView{{State: "not-established", Lines: []supersessionLineView{{Kind: "not-established", Text: "supersession not established: the object spec/closed-feature#co-1 is not an acceptance criterion or a decision"}}}}
	plain := cardView{ID: "dc-1", Kind: "decision", Supersessions: lines}
	badged := cardView{ID: "dc-1", Kind: "decision", Supersessions: lines, Badges: []badgeView{{Source: "lint:VL-026", Label: "VL-026"}}}
	if cardHeightPx(badged) <= cardHeightPx(plain) {
		t.Errorf("a badge row added no height: %v vs %v", cardHeightPx(badged), cardHeightPx(plain))
	}
	three := badged
	three.Badges = append(three.Badges, badgeView{Label: "b"}, badgeView{Label: "c"})
	if cardHeightPx(three) <= cardHeightPx(badged) {
		t.Errorf("a second badge row added no height: %v vs %v", cardHeightPx(three), cardHeightPx(badged))
	}
	// Badges alone (no lines) leave the footprint: they stay pinned at the foot.
	if got := cardHeightPx(cardView{ID: "ac-1", Kind: "acceptance-criterion", Badges: badged.Badges}); got != 140 {
		t.Errorf("badges without lines changed the footprint: %v", got)
	}
	short := refCardView{Ref: obsFeatureObject, Object: &refObjectView{Text: "short", Supersession: supersessionView{State: "unproven", Lines: []supersessionLineView{{Kind: "unproven", Text: "supersession unproven: w"}}}}}
	long := short
	long.Object = &refObjectView{Text: strings.Repeat("a long original criterion text ", 8), Supersession: short.Object.Supersession}
	if refCardHeightPx(long)-refCardHeightPx(short) < 8*heightTextLinePx {
		t.Errorf("the reference card clamps the original text in its estimate: %v vs %v", refCardHeightPx(long), refCardHeightPx(short))
	}
}

// TestLaneLanding_ClearsReservedHeights (the board closure's C-3): a new
// sticky in the decision lane and a new pin in the reference lane land
// below the cards' reserved heights, never on a card's lines.
func TestLaneLanding_ClearsReservedHeights(t *testing.T) {
	neutralizeCIEnv(t)
	repo := scenario.Build(t, "accepted")
	p := obsBoard(t, repo.Dir, "successor")
	var lowest *cardView
	for i := range p.Cards {
		c := &p.Cards[i]
		if c.Kind == "decision" && (lowest == nil || c.Y+cardHeightPx(*c) > lowest.Y+cardHeightPx(*lowest)) {
			lowest = c
		}
	}
	if lowest == nil || len(lowest.Supersessions) == 0 {
		t.Fatal("no decision card with lines on the accepted board")
	}
	_, y := stickyLanePosition(p, artifact.AnnotationDecisionNeeded)
	if y < lowest.Y+cardHeightPx(*lowest) {
		t.Errorf("a decision-needed sticky lands at y=%v, inside %s's reserved height (y=%v, h=%v)", y, lowest.ID, lowest.Y, cardHeightPx(*lowest))
	}
	if y < lowest.Y+boardlayout.CardHeight+1 {
		t.Errorf("the landing used the bare footprint: y=%v", y)
	}
	var lowestRef *refCardView
	for i := range p.RefCards {
		rc := &p.RefCards[i]
		if lowestRef == nil || rc.Y+refCardHeightPx(*rc) > lowestRef.Y+refCardHeightPx(*lowestRef) {
			lowestRef = rc
		}
	}
	if lowestRef == nil || lowestRef.Object == nil {
		t.Fatal("no reference card with an object on the accepted board")
	}
	_, py := laneBottomPosition(p, referenceLane())
	if py < lowestRef.Y+refCardHeightPx(*lowestRef) {
		t.Errorf("a pin lands at y=%v, inside %s's reserved height (y=%v, h=%v)", py, lowestRef.Ref, lowestRef.Y, refCardHeightPx(*lowestRef))
	}
}

// TestLoadSealed_DisclosesMissingLines (the board closure's C-5): the
// sealed render of a remote-only branch computes no views and says so in
// its disclosure — never silence.
func TestLoadSealed_DisclosesMissingLines(t *testing.T) {
	neutralizeCIEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, "proposed") // design/successor exists locally; make it remote-only
	gitIn(t, repo.Dir, "checkout", "-q", "main")
	gitIn(t, repo.Dir, "update-ref", "refs/remotes/origin/design/successor", "design/successor")
	gitIn(t, repo.Dir, "branch", "-q", "-D", "design/successor")
	bb := newBranchBoards(repo.Dir, Deps{}, &boardSpecServer{root: repo.Dir})
	proj, _, err := bb.loadSealed(ctx, "design/successor", "origin/design/successor", "successor")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range proj.Notices {
		if strings.Contains(n, "closed-spec object supersession lines (design §6) are not computed on this sealed render") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the sealed board's notices do not disclose the missing lines: %q", proj.Notices)
	}
	for _, c := range proj.Cards {
		if len(c.Supersessions) != 0 {
			t.Fatalf("the sealed board carries views it says it does not compute: %+v", c.Supersessions)
		}
	}
}
