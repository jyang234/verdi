package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/boardio"
	"github.com/OWNER/verdi/internal/fixturegit"
	"github.com/OWNER/verdi/internal/gitx"
)

// The board-fixture spec: a draft feature on a design branch — the v1
// board's authoring case. dc-1 carries a declared exempts edge (projects
// as spec-layer yarn with an external reference card); dc-2 is plain
// (fresh yarn draws from it).
const boardFixtureSpec = `---
id: spec/refi-test
kind: spec
class: feature
title: "Refi test flow"
status: draft
owners: [platform-team]
problem: { text: "declined applicants act on stale decline reasons", anchor: "#problem" }
outcome: { text: "declined applicants see a current decline flow", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a declined applicant sees the current reason", evidence: [attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a decline reverses within one day", evidence: [behavioral, attestation], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "notices never name internal model scores", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "excuse this flow from the outbox rule", anchor: "#dc-1",
      links: [ { type: exempts, ref: adr/0001-outbox-events, note: "async by design" } ] }
  - { id: dc-2, text: "reuse the notification channel", anchor: "#dc-2" }
---
# Refi test flow

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.

## ac-2

Prose.

## co-1

Prose.

## dc-1

Prose.

## dc-2

Prose.
`

const boardFixtureLayout = `{
  "schema": "verdi.boardlayout/v1",
  "positions": { "ac-1": { "x": 40, "y": 60 } }
}
`

const boardFixtureName = "refi-test"

// newBoardFixture builds a fixture repo with the draft spec, checked out
// on a design branch (authoring mode's branch state).
func newBoardFixture(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + boardFixtureName + "/spec.md":     boardFixtureSpec,
			".verdi/specs/active/" + boardFixtureName + "/layout.json": boardFixtureLayout,
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed board fixture",
	}})
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "design/"+boardFixtureName); err != nil {
		t.Fatalf("checkout design branch: %v", err)
	}
	return repo.Dir
}

func getBoard(t *testing.T, h http.Handler, name string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/"+name, nil))
	return rec
}

func postBoardAPI(t *testing.T, h http.Handler, name, action, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/board/spec/"+name+"/api/"+action, strings.NewReader(body))
	h.ServeHTTP(rec, req)
	return rec
}

func TestBoardSpecPage_Authoring(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	rec := getBoard(t, h, boardFixtureName)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET board = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`data-board-mode="authoring"`,
		`data-testid="placard-problem"`, "stale decline reasons",
		`data-testid="placard-outcome"`, "current decline flow",
		`data-testid="card-ac-1"`, `data-object-kind="acceptance-criterion"`,
		`data-testid="card-co-1"`, `data-object-kind="constraint"`,
		`data-testid="card-dc-2"`, `data-object-kind="decision"`,
		`data-edge-type="exempts" data-from="dc-1" data-to="adr/0001-outbox-events" data-layer="spec"`,
		`data-testid="ref-card-adr-0001-outbox-events"`,
		`data-testid="yarn-handle-dc-2"`,
		`data-testid="uncommitted-indicator" hidden`,
		`Commit &amp; push`,
		`Add sticky`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("board page missing %q", want)
		}
	}
	// The stored layout position passes through verbatim.
	if !strings.Contains(body, `data-testid="card-ac-1" data-id="ac-1" data-object-kind="acceptance-criterion" style="left:40px;top:60px"`) {
		t.Error("stored ac-1 position not rendered verbatim")
	}
}

func TestBoardSpecPage_Deterministic(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	first := getBoard(t, h, boardFixtureName)
	second := getBoard(t, h, boardFixtureName)
	if first.Body.String() != second.Body.String() {
		t.Fatal("two renders of the same inputs differ")
	}
}

func TestBoardSpecPage_NotFound(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	for _, name := range []string{"no-such-spec", "Bad_Name!"} {
		rec := getBoard(t, h, name)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET board %q = %d, want 404", name, rec.Code)
		}
	}
}

func TestBoardSpec_EditText(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, boardFixtureName, "edit-text", `{"id":"ac-1","text":"a declined applicant sees the current reason [edited]"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("edit-text = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if !resp.Dirty {
		t.Error("edit-text response dirty = false, want true (the spec working tree changed)")
	}

	specPath := filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `text: "a declined applicant sees the current reason [edited]"`) {
		t.Error("spec document does not carry the edit")
	}
	// The projection re-renders the edit.
	if !strings.Contains(getBoard(t, h, boardFixtureName).Body.String(), "[edited]") {
		t.Error("board does not re-render the edited text")
	}
}

func TestBoardSpec_EditText_Negative(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	tests := []struct {
		name, body string
		wantCode   int
	}{
		{"unknown object", `{"id":"ac-99","text":"x"}`, http.StatusBadRequest},
		{"unknown field fails closed", `{"id":"ac-1","text":"x","bogus":1}`, http.StatusBadRequest},
		{"empty text", `{"id":"ac-1","text":""}`, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := postBoardAPI(t, h, boardFixtureName, "edit-text", tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("= %d, want %d\n%s", rec.Code, tc.wantCode, rec.Body.String())
			}
		})
	}
}

func TestBoardSpec_WritesRefusedOutsideAuthoring(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	// On the default branch the draft is not authorable (05 §Workbench:
	// modes keyed by branch state).
	if err := gitx.Checkout(context.Background(), root, "main"); err != nil {
		t.Fatal(err)
	}
	rec := postBoardAPI(t, h, boardFixtureName, "edit-text", `{"id":"ac-1","text":"x"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("edit-text off design branch = %d, want 403", rec.Code)
	}
	if !strings.Contains(getBoard(t, h, boardFixtureName).Body.String(), `data-board-mode="readonly"`) {
		t.Error("board off design branch is not read-only")
	}
}

func TestBoardSpec_Edge(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	// Illegal pair: an AC source offers no typed edge.
	rec := postBoardAPI(t, h, boardFixtureName, "edge", `{"from":"ac-1","to":"ac-2","type":"implements"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("illegal edge = %d, want 400", rec.Code)
	}
	// Unknown type fails closed even on a legal pair.
	rec = postBoardAPI(t, h, boardFixtureName, "edge", `{"from":"dc-2","to":"adr/0001-outbox-events","type":"blesses"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown edge type = %d, want 400", rec.Code)
	}

	// Legal: decision → ADR supersedes (the first yarn on dc-2 — the S7
	// absent-links splice case, exercised through the full HTTP path).
	rec = postBoardAPI(t, h, boardFixtureName, "edge", `{"from":"dc-2","to":"adr/0001-outbox-events","type":"supersedes","note":"drawn on the board"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("legal edge = %d\n%s", rec.Code, rec.Body.String())
	}
	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-edge-type="supersedes" data-from="dc-2" data-to="adr/0001-outbox-events" data-layer="spec"`) {
		t.Error("new edge does not project as spec-layer yarn")
	}
}

func TestBoardSpec_StickyLifecycle(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, boardFixtureName, "sticky", `{"text":"open question: partial refunds?"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("sticky = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Dirty {
		t.Error("a sticky dirtied the spec working tree (mutable zone must not)")
	}

	annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(annotations) != 1 || annotations[0].Type != artifact.AnnotationQuestion {
		t.Fatalf("annotations = %+v, want one question", annotations)
	}
	stickyID := annotations[0].ID

	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-testid="sticky-`+stickyID+`" data-id="`+stickyID+`" data-annotation-type="question"`) {
		t.Error("sticky does not render")
	}

	// Graduation: the sticky becomes a declared open-question object via
	// an ordinary edit; the record flips to graduated.
	rec = postBoardAPI(t, h, boardFixtureName, "sticky-graduate", `{"id":"`+stickyID+`","kind":"open-question"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("sticky-graduate = %d\n%s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Dirty {
		t.Error("graduation did not dirty the spec working tree (it IS a spec edit)")
	}

	specData, _ := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if !strings.Contains(string(specData), `{ id: oq-1, text: "open question: partial refunds?", anchor: "#oq-1" }`) {
		t.Error("spec does not carry the graduated open question")
	}
	if !strings.Contains(string(specData), "\n## oq-1\n") {
		t.Error("spec body has no heading for the graduated object's anchor")
	}

	body = getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-testid="card-oq-1" data-id="oq-1" data-object-kind="open-question"`) {
		t.Error("graduated object does not render as a card")
	}
	if strings.Contains(body, `data-testid="sticky-`+stickyID+`"`) {
		t.Error("graduated sticky still renders")
	}
}

func TestBoardSpec_StickyGraduate_Negative(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	rec := postBoardAPI(t, h, boardFixtureName, "sticky-graduate", `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","kind":"open-question"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("graduating a missing sticky = %d, want 400", rec.Code)
	}
	rec = postBoardAPI(t, h, boardFixtureName, "sticky-graduate", `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","kind":"story"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("graduating to an unknown kind = %d, want 400", rec.Code)
	}
}

func TestBoardSpec_RelatesLifecycle(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, boardFixtureName, "relates", `{"from":"dc-2","to":"adr/0001-outbox-events"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("relates = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Dirty {
		t.Error("a relates thread dirtied the spec working tree")
	}

	annotations, _ := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	if len(annotations) != 1 || annotations[0].Type != artifact.AnnotationRelates {
		t.Fatalf("annotations = %+v, want one relates", annotations)
	}
	threadID := annotations[0].ID

	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-edge-type="relates" data-from="dc-2" data-to="adr/0001-outbox-events" data-layer="annotation" data-annotation-id="`+threadID+`"`) {
		t.Error("relates thread does not render as annotation-layer yarn")
	}

	// Graduation to a typed edge via the picker's pair.
	rec = postBoardAPI(t, h, boardFixtureName, "relates-graduate", `{"id":"`+threadID+`","type":"exempts","note":"confirmed on the board"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("relates-graduate = %d\n%s", rec.Code, rec.Body.String())
	}
	body = getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-edge-type="exempts" data-from="dc-2" data-to="adr/0001-outbox-events" data-layer="spec"`) {
		t.Error("graduated thread does not project as spec-layer yarn")
	}
	if strings.Contains(body, `data-annotation-id="`+threadID+`"`) {
		t.Error("graduated thread still renders as annotation yarn")
	}

	// Illegal graduation is refused server-side.
	rec = postBoardAPI(t, h, boardFixtureName, "relates", `{"from":"ac-1","to":"ac-2"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("second relates = %d", rec.Code)
	}
	annotations, _ = boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	var acThread string
	for _, a := range annotations {
		if a.ID != threadID {
			acThread = a.ID
		}
	}
	rec = postBoardAPI(t, h, boardFixtureName, "relates-graduate", `{"id":"`+acThread+`","type":"implements"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("illegal relates-graduate = %d, want 400", rec.Code)
	}
}

func TestBoardSpec_Position(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	rec := postBoardAPI(t, h, boardFixtureName, "position", `{"id":"ac-2","x":613,"y":218}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("position = %d\n%s", rec.Code, rec.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "layout.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, `"ac-2":{"x":613,"y":218}`) {
		t.Errorf("layout.json missing the stored position: %s", got)
	}
	if !strings.Contains(got, `"ac-1":{"x":40,"y":60}`) {
		t.Errorf("layout.json lost the pre-existing stored position: %s", got)
	}
	if !strings.Contains(getBoard(t, h, boardFixtureName).Body.String(), `style="left:613px;top:218px"`) {
		t.Error("stored position does not render")
	}

	// A non-object key is refused (VL-018: keys must resolve).
	rec = postBoardAPI(t, h, boardFixtureName, "position", `{"id":"zz-9","x":1,"y":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("position for unknown id = %d, want 400", rec.Code)
	}
}

func TestBoardSpec_GitCommitAndSwitch(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	ctx := context.Background()

	// Dirty the tree through the board's own write path.
	postBoardAPI(t, h, boardFixtureName, "edit-text", `{"id":"co-1","text":"notices never name internal scores [amended]"}`)
	dirty, _ := gitx.StatusDirty(ctx, root)
	if !dirty {
		t.Fatal("fixture not dirty after edit")
	}

	// The guard: a dirty tree refuses to switch.
	rec := postBoardAPI(t, h, boardFixtureName, "git-switch", `{"branch":"main"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("git-switch (dirty) = %d, want 409", rec.Code)
	}

	// A message is required.
	rec = postBoardAPI(t, h, boardFixtureName, "git-commit", `{"message":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("git-commit without message = %d, want 400", rec.Code)
	}

	// Commit clears the indicator's signal. (No origin remote here: the
	// commit is still durable; push engages only when origin exists.)
	rec = postBoardAPI(t, h, boardFixtureName, "git-commit", `{"message":"board: amend co-1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("git-commit = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Dirty {
		t.Error("tree still dirty after the board commit")
	}

	// Clean: the switch works.
	rec = postBoardAPI(t, h, boardFixtureName, "git-switch", `{"branch":"main"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("git-switch (clean) = %d\n%s", rec.Code, rec.Body.String())
	}
	branch, _ := gitx.CurrentBranch(ctx, root)
	if branch != "main" {
		t.Fatalf("branch after switch = %q, want main", branch)
	}
}

// fakeFeed is the CommentFeed test double review mode is built against
// (V1-P6 "Stubs": the real forge port is V1-P7's; the wave close adapts
// it over this interface).
type fakeFeed struct {
	feeds map[string][]MRComment
}

func (f fakeFeed) ListMRComments(_ context.Context, specName string) ([]MRComment, bool, error) {
	comments, ok := f.feeds[specName]
	return comments, ok, nil
}

func TestBoardSpec_ReviewMode(t *testing.T) {
	root := newBoardFixture(t)
	feed := fakeFeed{feeds: map[string][]MRComment{
		boardFixtureName: {
			{ID: "1", Author: "alice", Body: "[vd:ac-2] this outcome AC reads implementation-scoped — reword?"},
			{ID: "2", Author: "bob", Body: "overall direction looks right"},
			{ID: "3", Author: "carol", Body: "[vd:zz-99] does this still apply after the split?", Resolved: true},
		},
	}}
	h := NewHandlerWith(root, Deps{CommentFeed: feed})

	rec := getBoard(t, h, boardFixtureName)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET review board = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-board-mode="review"`) {
		t.Fatal("board with an open MR is not in review mode")
	}
	// The resolvable token anchors to its object's card.
	acCard := body[strings.Index(body, `data-testid="card-ac-2"`):]
	acCard = acCard[:strings.Index(acCard, `data-testid="card-co-1"`)]
	if !strings.Contains(acCard, `data-annotation-type="review" data-anchor="ac-2"`) {
		t.Error("anchored comment does not ride its object's card")
	}
	// Token-free and unresolvable-token comments land in the tray.
	if !strings.Contains(body, `aria-label="Inbox tray"`) {
		t.Fatal("no inbox tray")
	}
	tray := body[strings.Index(body, `aria-label="Inbox tray"`):]
	for _, want := range []string{"overall direction looks right", "[vd:zz-99] does this still apply"} {
		if !strings.Contains(tray, want) {
			t.Errorf("inbox tray missing %q", want)
		}
	}
	// Conservation: the whole feed renders.
	if got := strings.Count(body, `data-annotation-type="review"`); got != 3 {
		t.Errorf("review stickies = %d, want 3 (never dropped)", got)
	}
	// A mirror, not an editing surface.
	for _, absent := range []string{"Commit &amp; push", "Add sticky", "yarn-handle", "graduate-btn"} {
		if strings.Contains(body, absent) {
			t.Errorf("review mode still renders %q", absent)
		}
	}
	// And no write goes through.
	recW := postBoardAPI(t, h, boardFixtureName, "edit-text", `{"id":"ac-1","text":"x"}`)
	if recW.Code != http.StatusForbidden {
		t.Fatalf("write in review mode = %d, want 403", recW.Code)
	}
}

func TestCommentToken(t *testing.T) {
	tests := []struct {
		body, want string
	}{
		{"[vd:ac-2] reword this", "ac-2"},
		{"no token here", ""},
		{"mid-body [vd:ac-2] token does not anchor", ""},
		{"[vd:zz-99] unresolvable is still a token", "zz-99"},
		{"[vd:] empty", ""},
	}
	for _, tc := range tests {
		if got := commentToken(tc.body); got != tc.want {
			t.Errorf("commentToken(%q) = %q, want %q", tc.body, got, tc.want)
		}
	}
}

func TestLegalEdgeTypes(t *testing.T) {
	tests := []struct {
		source, target string
		want           int
	}{
		{"decision", "adr", 2},
		{"decision", "decision", 2},
		{"decision", "spec-fragment", 2},
		{"acceptance-criterion", "acceptance-criterion", 0},
		{"constraint", "adr", 0},
		{"open-question", "decision", 0},
	}
	for _, tc := range tests {
		if got := len(legalEdgeTypes(tc.source, tc.target)); got != tc.want {
			t.Errorf("legalEdgeTypes(%s, %s) = %d types, want %d", tc.source, tc.target, got, tc.want)
		}
	}
}

// erroringFeed is a CommentFeed whose call always fails — the
// configured-but-unreachable forge from the failure side (I-2).
type erroringFeed struct{}

func (erroringFeed) ListMRComments(context.Context, string) ([]MRComment, bool, error) {
	return nil, false, errors.New("forge unreachable: dial tcp 10.0.0.1:443: connect: connection refused")
}

// TestBoard_FeedError_DegradesNotBlocks proves I-2: a feed error on an
// authoring board renders 200 with the board content intact PLUS a
// disclosed notice — never a 500, never a blocked page. The feed is a
// review-mode-only input; authoring must always render (04 §Semantics'
// degradation posture).
func TestBoard_FeedError_DegradesNotBlocks(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandlerWith(root, Deps{CommentFeed: erroringFeed{}})

	rec := getBoard(t, h, boardFixtureName)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET board with a failing feed = %d, want 200 (never block on the feed)\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	// Board content is intact — authoring mode rendered fully.
	if !strings.Contains(body, `data-board-mode="authoring"`) {
		t.Error("authoring board did not render authoring mode on a feed error")
	}
	if !strings.Contains(body, `data-testid="card-ac-1"`) {
		t.Error("authoring board content missing on a feed error (should be fully rendered)")
	}
	// The failure is disclosed, never silent.
	if !strings.Contains(body, `data-testid="board-notice"`) || !strings.Contains(body, "review feed unavailable") {
		t.Errorf("feed error not disclosed as a board notice:\n%s", body)
	}
	// The fragment surface degrades identically (post-mutation re-render).
	frag := httptest.NewRecorder()
	h.ServeHTTP(frag, httptest.NewRequest(http.MethodGet, "/board/spec/"+boardFixtureName+"/fragment", nil))
	if frag.Code != http.StatusOK || !strings.Contains(frag.Body.String(), "review feed unavailable") {
		t.Errorf("fragment on feed error = %d, want 200 with the disclosure notice", frag.Code)
	}
}

// TestBoard_ConfiguredButUnavailable_Disclosed proves I-1(b) state 3: a
// forge configured but with no live feed (Deps.ReviewUnavailable set)
// discloses on the board chrome rather than rendering as silently
// not-under-review.
func TestBoard_ConfiguredButUnavailable_Disclosed(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandlerWith(root, Deps{ReviewUnavailable: `forge "gitlab" is configured but no credentials are available to reach it; review state cannot be shown`})

	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-testid="board-notice"`) || !strings.Contains(body, "review state cannot be shown") {
		t.Errorf("configured-but-unavailable forge not disclosed on the board:\n%s", body)
	}
}

// TestBoard_NoForge_Silent proves I-1(b) state 1: with no feed and no
// ReviewUnavailable, the board says nothing about review — an unconfigured
// integration legitimately stays silent (no review-specific disclosure).
func TestBoard_NoForge_Silent(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandlerWith(root, Deps{})

	body := getBoard(t, h, boardFixtureName).Body.String()
	for _, absent := range []string{"review feed unavailable", "review state cannot be shown"} {
		if strings.Contains(body, absent) {
			t.Errorf("unconfigured forge should be silent, but board contains %q", absent)
		}
	}
}

// TestBoard_DefaultBranchAssumed_Disclosed proves M-4: a repo with no
// origin/HEAD configured (the fixture's state) discloses the assumed "main"
// default rather than keying authoring mode off it silently.
func TestBoard_DefaultBranchAssumed_Disclosed(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	body := getBoard(t, h, boardFixtureName).Body.String()
	if !strings.Contains(body, `data-testid="board-notice"`) || !strings.Contains(body, "default branch could not be resolved") {
		t.Errorf("assumed default branch not disclosed on the board:\n%s", body)
	}
}

// TestBoard_ConcurrentMutations_BothLand proves M-2: two racing board
// mutations against the same spec both land (no lost update). Without the
// per-server writeMu the second read-modify-write of spec.md could
// clobber the first (last writer wins).
func TestBoard_ConcurrentMutations_BothLand(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)

	var wg sync.WaitGroup
	edits := []struct{ id, text string }{
		{"ac-1", "a declined applicant sees the current reason [edit-A]"},
		{"ac-2", "a decline reverses within one day [edit-B]"},
	}
	for _, e := range edits {
		wg.Add(1)
		go func(id, text string) {
			defer wg.Done()
			body := `{"id":"` + id + `","text":"` + text + `"}`
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/board/spec/"+boardFixtureName+"/api/edit-text", strings.NewReader(body))
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("edit-text %s = %d, want 200: %s", id, rec.Code, rec.Body.String())
			}
		}(e.id, e.text)
	}
	wg.Wait()

	specData, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	spec := string(specData)
	for _, e := range edits {
		if !strings.Contains(spec, e.text) {
			t.Errorf("edit %q was lost (last-writer-wins) — both concurrent mutations must land:\n%s", e.text, spec)
		}
	}
}
