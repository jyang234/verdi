package workbench

// The board Revise action's handler tests (spec/uat-round-1 ac-11, board
// half; PLAN.md I-129 option (a)): the sealed accepted feature wall's
// third live affordance, invoking the SAME supersede operation `verdi
// design start --supersedes` runs — ValidateSuccessorName, Resolve,
// Compose, then the no-checkout branch cut — minus the checkout switch.
// Fixtures reuse the scoping-accepted wall stub-instantiate's and create's
// own tests established.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// TestBoardSpec_Revise_Happy: a successor name lands as exactly one
// scaffold commit on a fresh design/<new> branch carrying the composed
// successor (every predecessor object `carried`, a whole-spec supersedes
// link, class feature) — with the serving checkout's HEAD, branch, and
// working tree untouched — and the response names the successor's own
// per-branch board address.
func TestBoardSpec_Revise_Happy(t *testing.T) {
	repo := newScopingAcceptedFixture(t)
	root := repo.Dir
	h := NewHandler(root)
	ctx := context.Background()

	beforeBranch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		t.Fatal(err)
	}

	rec := postBoardAPI(t, h, scopingAcceptedName, "revise", `{"name":"scoping-accepted-v2"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("revise = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Dirty    bool   `json:"dirty"`
		Branch   string `json:"branch"`
		BoardURL string `json:"boardUrl"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v\n%s", err, rec.Body.String())
	}
	if resp.Branch != "design/scoping-accepted-v2" {
		t.Errorf("branch = %q, want design/scoping-accepted-v2", resp.Branch)
	}
	if want := "/b/design%2Fscoping-accepted-v2/board/spec/scoping-accepted-v2"; resp.BoardURL != want {
		t.Errorf("boardUrl = %q, want %q", resp.BoardURL, want)
	}

	// Serving checkout untouched.
	afterBranch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if afterBranch != beforeBranch {
		t.Fatalf("current branch moved from %q to %q", beforeBranch, afterBranch)
	}
	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head != repo.Head {
		t.Fatalf("HEAD moved to %s, want unchanged %s", head, repo.Head)
	}
	dirty, err := gitx.StatusDirty(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Fatal("revise left the serving working tree dirty")
	}

	// The new branch: forked from the prior HEAD, one commit, the composed
	// successor at the successor's own store path.
	parent, err := gitx.RevParse(ctx, root, "design/scoping-accepted-v2^")
	if err != nil {
		t.Fatalf("design/scoping-accepted-v2 missing or rootless: %v", err)
	}
	if parent != repo.Head {
		t.Fatalf("new branch's parent = %s, want %s", parent, repo.Head)
	}
	blob, err := gitx.Show(ctx, root, "design/scoping-accepted-v2", ".verdi/specs/active/scoping-accepted-v2/spec.md")
	if err != nil {
		t.Fatalf("Show successor spec: %v", err)
	}
	fm, _, err := artifact.SplitFrontmatter(blob)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if spec.ID != "spec/scoping-accepted-v2" {
		t.Errorf("ID = %q, want spec/scoping-accepted-v2", spec.ID)
	}
	if spec.Class != artifact.ClassFeature {
		t.Errorf("Class = %q, want feature", spec.Class)
	}
	if spec.Status != "" {
		t.Errorf("Status = %q, want none (a draft on its design branch carries no status field)", spec.Status)
	}
	var supersedes int
	for _, l := range spec.Links {
		if l.Type == artifact.LinkSupersedes && l.Ref == "spec/"+scopingAcceptedName {
			supersedes++
		}
	}
	if supersedes != 1 {
		t.Errorf("links = %+v, want exactly one supersedes edge to spec/%s", spec.Links, scopingAcceptedName)
	}
	if spec.Supersession == nil {
		t.Fatalf("successor carries no supersession: block")
	}
	if got := spec.Supersession.Carried; len(got) != 2 || got[0] != "ac-1" || got[1] != "oq-1" {
		t.Errorf("Supersession.Carried = %v, want [ac-1 oq-1] (every predecessor object, verbatim)", got)
	}
	if len(spec.Stubs) != 2 {
		t.Errorf("Stubs = %+v, want the predecessor's two stubs carried verbatim", spec.Stubs)
	}
}

// composeBreakingAcceptedSpec decodes cleanly and projects as an accepted
// predecessor (class feature, exact bytes on the default branch), but
// breaks supersede.Compose: its title is a multi-line double-quoted scalar
// whose continuation line sits at column 0 and reads exactly like a
// top-level key — the one shape the line-level frontmatter splitter
// discloses it cannot see through (W3-B's own composeBreakingPredecessor,
// cmd/verdi). A REAL predecessor that reaches Compose and fails there.
const composeBreakingAcceptedSpec = `---
id: spec/dedent
kind: spec
class: feature
title: "Dedent fixture
status: this continuation line is inside the title scalar"
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a", evidence: [attestation], anchor: "#ac-1" }
---
# Dedent fixture

Body.
`

const composeBreakingAcceptedName = "dedent"

// TestBoardSpec_Revise_ComposeFailureWritesNoRef is the ordering witness:
// Compose runs BEFORE any ref is written, so a predecessor that composes
// badly refuses (400, the composition error named) and leaves no
// refs/heads/design/<new> behind — the CLI's F3 preparation-boundary
// property, proven on the board's path.
func TestBoardSpec_Revise_ComposeFailureWritesNoRef(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + composeBreakingAcceptedName + "/spec.md": composeBreakingAcceptedSpec,
			".verdi/.gitignore": "data/\n",
			".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
		},
		Message: "seed compose-breaking accepted fixture",
	}})
	setDefaultBranchSymref(t, repo.Dir)
	h := NewHandler(repo.Dir)
	ctx := context.Background()

	rec := postBoardAPI(t, h, composeBreakingAcceptedName, "revise", `{"name":"dedent-v2"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("revise(compose-breaking predecessor) = %d, want 400\n%s", rec.Code, rec.Body.String())
	}
	// The refusal is Compose's own — proof the predecessor passed the wall
	// guard and Resolve and failed exactly at composition.
	if !strings.Contains(rec.Body.String(), "composed successor failed self-validation") {
		t.Fatalf("refusal is not Compose's:\n%s", rec.Body.String())
	}
	if _, err := gitx.RevParse(ctx, repo.Dir, "refs/heads/design/dedent-v2"); err == nil {
		t.Fatal("refs/heads/design/dedent-v2 exists after a composition failure: a ref was written before Compose ran")
	}
	head, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head != repo.Head {
		t.Fatalf("HEAD moved to %s, want unchanged %s", head, repo.Head)
	}
}

// TestBoardSpec_Revise_Refusals: every operator-input refusal is a 400
// with a legible message and writes no ref; the transport refusals keep
// the handler's own statuses.
func TestBoardSpec_Revise_Refusals(t *testing.T) {
	ctx := context.Background()
	noRef := func(t *testing.T, root, branch string) {
		t.Helper()
		if _, err := gitx.RevParse(ctx, root, "refs/heads/"+branch); err == nil {
			t.Fatalf("refs/heads/%s exists after a refusal", branch)
		}
	}

	t.Run("empty name", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		h := NewHandler(repo.Dir)
		rec := postBoardAPI(t, h, scopingAcceptedName, "revise", `{}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("revise(empty name) = %d, want 400\n%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "kebab-case name") {
			t.Errorf("refusal does not ask for a name:\n%s", rec.Body.String())
		}
	})

	t.Run("invalid name", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		h := NewHandler(repo.Dir)
		rec := postBoardAPI(t, h, scopingAcceptedName, "revise", `{"name":"Not Kebab"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("revise(invalid name) = %d, want 400\n%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "Not Kebab") || !strings.Contains(rec.Body.String(), "kebab-case") {
			t.Errorf("refusal does not name the bad name and the rule:\n%s", rec.Body.String())
		}
		noRef(t, repo.Dir, "design/Not Kebab")
	})

	t.Run("successor directory exists", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		if err := os.MkdirAll(filepath.Join(repo.Dir, ".verdi", "specs", "active", "taken"), 0o755); err != nil {
			t.Fatal(err)
		}
		h := NewHandler(repo.Dir)
		rec := postBoardAPI(t, h, scopingAcceptedName, "revise", `{"name":"taken"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("revise(existing dir) = %d, want 400\n%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "spec taken already exists under specs/active/") {
			t.Errorf("refusal does not name the collision:\n%s", rec.Body.String())
		}
		noRef(t, repo.Dir, "design/taken")
	})

	t.Run("archived successor name exists", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		if err := os.MkdirAll(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "retired"), 0o755); err != nil {
			t.Fatal(err)
		}
		h := NewHandler(repo.Dir)
		rec := postBoardAPI(t, h, scopingAcceptedName, "revise", `{"name":"retired"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("revise(archived name) = %d, want 400\n%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "specs/archive/") {
			t.Errorf("refusal does not name the archive collision:\n%s", rec.Body.String())
		}
		noRef(t, repo.Dir, "design/retired")
	})

	t.Run("branch already exists", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		if err := gitx.UpdateRef(ctx, repo.Dir, "refs/heads/design/scoping-accepted-v2", repo.Head); err != nil {
			t.Fatalf("pre-creating the branch: %v", err)
		}
		h := NewHandler(repo.Dir)
		rec := postBoardAPI(t, h, scopingAcceptedName, "revise", `{"name":"scoping-accepted-v2"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("revise(branch exists) = %d, want 400\n%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "branch design/scoping-accepted-v2 already exists") {
			t.Errorf("branch-exists refusal not in plain language:\n%s", rec.Body.String())
		}
		// The pre-existing ref is untouched — no rewrite, no second commit.
		got, err := gitx.RevParse(ctx, repo.Dir, "refs/heads/design/scoping-accepted-v2")
		if err != nil {
			t.Fatal(err)
		}
		if got != repo.Head {
			t.Fatalf("existing branch moved to %s, want %s", got, repo.Head)
		}
	})

	t.Run("wrong status (draft feature wall)", func(t *testing.T) {
		root := newScopingWallFixture(t)
		h := NewHandler(root)
		rec := postBoardAPI(t, h, scopingWallName, "revise", `{"name":"scoping-wall-v2"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("revise(draft wall) = %d, want 400\n%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "revise is only available on") {
			t.Errorf("draft wall refusal is not the sealed-wall guard's:\n%s", rec.Body.String())
		}
		noRef(t, root, "design/scoping-wall-v2")
	})

	t.Run("wrong class (story wall)", func(t *testing.T) {
		root := newStoryWallFixture(t)
		h := NewHandler(root)
		rec := postBoardAPI(t, h, storyWallName, "revise", `{"name":"scoping-story-wall-v2"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("revise(story wall) = %d, want 400\n%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "revise is only available on") {
			t.Errorf("story wall refusal is not the sealed-wall guard's:\n%s", rec.Body.String())
		}
		noRef(t, root, "design/scoping-story-wall-v2")
	})

	t.Run("unknown JSON field", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		h := NewHandler(repo.Dir)
		rec := postBoardAPI(t, h, scopingAcceptedName, "revise", `{"name":"scoping-accepted-v2","bogus":true}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("revise(unknown field) = %d, want 400\n%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "malformed request") {
			t.Errorf("unknown field not refused at the strict-decode seam:\n%s", rec.Body.String())
		}
		noRef(t, repo.Dir, "design/scoping-accepted-v2")
	})

	t.Run("GET method", func(t *testing.T) {
		repo := newScopingAcceptedFixture(t)
		h := NewHandler(repo.Dir)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/"+scopingAcceptedName+"/api/revise", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("GET revise = %d, want 405", rec.Code)
		}
	})
}

// TestReviseSuccessorDefault pins the dialog's prefilled successor name:
// <pred>-v2, or the next -v<n> when the predecessor already carries one.
func TestReviseSuccessorDefault(t *testing.T) {
	cases := []struct{ pred, want string }{
		{"escrow-autopay", "escrow-autopay-v2"},
		{"escrow-autopay-v2", "escrow-autopay-v3"},
		{"rate-lock-v9", "rate-lock-v10"},
		{"x-v0", "x-v1"},
		// Not a version suffix: no digits, digits without the -v, a bare
		// v-prefixed segment that is the whole name, or an inner -v<n>.
		{"x-v", "x-v-v2"},
		{"x-2", "x-2-v2"},
		{"v2", "v2-v2"},
		{"x-v2-final", "x-v2-final-v2"},
		{"", "-v2"},
	}
	for _, tc := range cases {
		if got := reviseSuccessorDefault(tc.pred); got != tc.want {
			t.Errorf("reviseSuccessorDefault(%q) = %q, want %q", tc.pred, got, tc.want)
		}
	}
}

// TestBoardSpec_ReviseAffordance_Rendered: the sealed accepted feature
// wall renders the Revise button and its dialog (prefilled successor
// name, explanatory line, error slot, OK/cancel, success link); a draft
// wall, a story wall, and a superseded wall render none of it — the
// action's guard, mirrored at render so the rail never offers what the
// server would refuse.
func TestBoardSpec_ReviseAffordance_Rendered(t *testing.T) {
	repo := newScopingAcceptedFixture(t)
	h := NewHandler(repo.Dir)
	rec := getBoard(t, h, scopingAcceptedName)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET board = %d", rec.Code)
	}
	body := rec.Body.String()
	wants := []string{
		`data-testid="revise-spec-btn"`,
		`id="revise-dialog"`,
		`data-testid="revise-name"`,
		`value="scoping-accepted-v2"`,
		`data-testid="revise-error"`,
		`role="alert"`,
		`data-testid="revise-ok"`,
		`data-testid="revise-cancel"`,
		`data-testid="revise-success-link"`,
		// The explanatory line: verbatim carry, the supersedes link, a
		// draft on its own design branch, this checkout unmoved.
		"carried",
		"supersedes",
		"design/scoping-accepted-v2",
		"never moves",
	}
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("sealed wall page lacks %q", w)
		}
	}

	absent := map[string]string{
		"draft feature wall": newScopingWallFixture(t),
		"story wall":         newStoryWallFixture(t),
	}
	legacySuperseded := strings.Replace(scopingAcceptedSpec, "status: accepted-pending-build\n", "status: superseded\n", 1)
	superseded := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + scopingAcceptedName + "/spec.md": legacySuperseded,
			".verdi/.gitignore": "data/\n",
			".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
		},
		Message: "seed superseded fixture",
	}})
	setDefaultBranchSymref(t, superseded.Dir)
	absent["superseded feature wall"] = superseded.Dir
	names := map[string]string{
		"draft feature wall":      scopingWallName,
		"story wall":              storyWallName,
		"superseded feature wall": scopingAcceptedName,
	}
	for label, root := range absent {
		rec := getBoard(t, NewHandler(root), names[label])
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: GET board = %d", label, rec.Code)
		}
		if label == "superseded feature wall" && !strings.Contains(rec.Body.String(), `badge-superseded`) {
			t.Fatalf("%s fixture did not project as superseded", label)
		}
		for _, w := range []string{`data-testid="revise-spec-btn"`, `id="revise-dialog"`} {
			if strings.Contains(rec.Body.String(), w) {
				t.Errorf("%s renders %s; revise is sealed-accepted-feature-wall only", label, w)
			}
		}
	}
}

// TestBoardRender_ReviseVocabulary: every spoken class and state word in
// the affordance and dialog resolves through the model display chain
// (L-M13a(6)); the identity layer (testids, ids, the branch name) stays
// bare.
func TestBoardRender_ReviseVocabulary(t *testing.T) {
	proj := &BoardProjection{
		Spec:   "vocab-probe",
		Title:  "Vocab probe",
		Mode:   modeReadOnly,
		Status: "accepted-pending-build",
		Class:  "feature",
	}
	proj.applyModelVocabulary(vocabTestModel())
	page, err := renderBoardSpecPage(proj, &boardGitState{}, testASDView())
	if err != nil {
		t.Fatalf("renderBoardSpecPage: %v", err)
	}
	body := string(page)
	if !strings.Contains(body, `data-testid="revise-spec-btn">`) || !strings.Contains(body, "Revise this Initiative") {
		t.Errorf("revise affordance does not speak the renamed class word:\n%s", body)
	}
	start := strings.Index(body, `id="revise-dialog"`)
	if start < 0 {
		t.Fatalf("revise dialog not rendered")
	}
	end := strings.Index(body[start:], `</div>
`)
	if end < 0 {
		t.Fatalf("revise dialog not terminated")
	}
	dialog := body[start : start+end]
	// Attribute values (ids, testids, the design/ branch) are identity and
	// legitimately bare; the visible prose between tags must not be.
	visible := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(dialog, " ")
	if regexp.MustCompile(`\bfeature\b|\bstory\b`).MatchString(visible) {
		t.Errorf("revise dialog prose speaks a bare class word:\n%s", visible)
	}
	if !strings.Contains(visible, "Initiative") {
		t.Errorf("revise dialog prose does not speak the renamed class word:\n%s", visible)
	}
}

// TestBoardRender_ReviseAbsentInReviewMode: the revise dialog follows the
// SAME decision the Revise panel does — renderBoardRegion renders the
// panel only in its read-only room, so an accepted feature wall mirrored
// under review (modeReview) emits neither the panel nor a dead hidden
// dialog; the read-only room emits both.
func TestBoardRender_ReviseAbsentInReviewMode(t *testing.T) {
	render := func(mode boardModeKind) string {
		proj := &BoardProjection{
			Spec:   "review-probe",
			Title:  "Review probe",
			Mode:   mode,
			Status: "accepted-pending-build",
			Class:  "feature",
		}
		page, err := renderBoardSpecPage(proj, &boardGitState{}, testASDView())
		if err != nil {
			t.Fatalf("renderBoardSpecPage(%s): %v", mode, err)
		}
		return string(page)
	}
	review := render(modeReview)
	for _, w := range []string{`id="revise-dialog"`, `data-testid="revise-spec-btn"`} {
		if strings.Contains(review, w) {
			t.Errorf("review-mode accepted wall emits %s; the revise dialog and panel must share one gate", w)
		}
	}
	readOnly := render(modeReadOnly)
	for _, w := range []string{`id="revise-dialog"`, `data-testid="revise-spec-btn"`} {
		if !strings.Contains(readOnly, w) {
			t.Errorf("read-only accepted wall lacks %s", w)
		}
	}
}
