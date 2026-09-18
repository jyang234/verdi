package workbench

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/readinesspilot"
)

// The Document tab (spec/spec-documents, wave 2 task 5): a reading of a
// spec's objects rendered through the shared loader (internal/specdocload)
// beside the board, with a conditional /snapshot refresh, a Markdown
// download, and a copy control. Never authority (co-2); GET-only.

const documentWallName = "lockbox"

// documentWallSpec is one accepted feature spec on main — the sealed
// wall whose working-tree bytes ARE the accepted bytes, so its document
// renders as the accepted reading (no proposed stamp).
const documentWallSpec = `---
id: spec/lockbox
kind: spec
title: "Lockbox"
owners: [platform-team]
class: feature
problem: { text: "Keys are shared.", anchor: problem }
outcome: { text: "Each key has one holder.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A key opens one box.", evidence: [behavioral, attestation], anchor: ac-1 }
stubs:
  - { slug: key-holder, acceptance_criteria: [ac-1] }
---
# Lockbox

## Problem

Keys are shared today.

## Outcome

One holder.

## ac-1

Proven by opening.
`

// newAcceptedWallFixture builds a store with one accepted feature spec on
// main and returns the workbench handler over it, the repo, and the spec
// name. CI_DEFAULT_BRANCH=main makes the default branch provable without
// a remote (the same shape internal/specdocload's own fixtures use).
func newAcceptedWallFixture(t *testing.T) (http.Handler, *fixturegit.Repo, string) {
	t.Helper()
	return newAcceptedWallFixtureWithReadiness(t, nil)
}

func newAcceptedWallFixtureWithReadiness(t *testing.T, snap *readinesspilot.Snapshot) (http.Handler, *fixturegit.Repo, string) {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Message: "adopt store with one accepted spec",
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.config/v1\nforge: none\n",
			".verdi/specs/active/" + documentWallName + "/spec.md": documentWallSpec,
		},
	}})
	return NewHandlerWith(repo.Dir, Deps{Readiness: snap}), repo, documentWallName
}

// decodeStrictJSON decodes body into v with unknown fields and trailing
// data both refused (CLAUDE.md: strict decode everywhere).
func decodeStrictJSON(t *testing.T, body string, v any) error {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("trailing data after the JSON document: %v", err)
	}
	return nil
}

type httpResult struct {
	code                                 int
	body, etag, contentType, disposition string
}

func getStatus(t *testing.T, h http.Handler, path string) httpResult {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return httpResult{code: rec.Code, body: rec.Body.String(), etag: rec.Header().Get("ETag"), contentType: rec.Header().Get("Content-Type"), disposition: rec.Header().Get("Content-Disposition")}
}

func TestBoardDocument_PageSnapshotAndDownload(t *testing.T) {
	h, _, name := newAcceptedWallFixture(t)
	page := getStatus(t, h, "/board/spec/"+name+"/document")
	if page.code != http.StatusOK || !strings.Contains(page.body, `id="document-region"`) || !strings.Contains(page.body, "not authority") || !strings.Contains(page.body, `data-testid="document-copy"`) || !strings.Contains(page.body, `data-testid="document-download"`) {
		t.Fatalf("page: %d\n%s", page.code, page.body)
	}
	if strings.Contains(page.body, "Proposed, not accepted") {
		t.Fatalf("the sealed wall's document is the accepted reading")
	}
	// The page is usable before JS: the kind switch and the download are
	// plain links, and the back link to the board is derived from the
	// request path (no raw ref).
	for _, want := range []string{
		`href="/board/spec/` + name + `" data-testid="document-tab-board"`,
		`href="/board/spec/` + name + `/document?kind=plan" data-testid="document-kind-plan"`,
		`href="/board/spec/` + name + `/document?format=md&amp;kind=spec"`,
		`data-snapshot-href="/board/spec/` + name + `/document/snapshot?kind=spec"`,
		`<script src="/assets/specdocument.js"></script>`,
	} {
		if !strings.Contains(page.body, want) {
			t.Errorf("page lacks %q\n%s", want, page.body)
		}
	}

	snap := getStatus(t, h, "/board/spec/"+name+"/document/snapshot")
	if snap.code != http.StatusOK || snap.etag == "" || !strings.Contains(snap.body, `"revision":"`) || !strings.Contains(snap.body, `"markdown":"# `) {
		t.Fatalf("snapshot: %d etag %q\n%s", snap.code, snap.etag, snap.body)
	}
	if !strings.HasPrefix(snap.contentType, "application/json") {
		t.Fatalf("snapshot content type %q", snap.contentType)
	}
	// The page embeds the same revision token the snapshot answers, so
	// the first conditional poll compares against a genuine token.
	if !strings.Contains(page.body, `data-revision="`+strings.Trim(snap.etag, `"`)+`"`) {
		t.Fatalf("page revision differs from snapshot etag %s\n%s", snap.etag, page.body)
	}
	req := httptest.NewRequest(http.MethodGet, "/board/spec/"+name+"/document/snapshot", nil)
	req.Header.Set("If-None-Match", snap.etag)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("unchanged token must 304, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("304 must carry no body, got %q", rec.Body.String())
	}
	// A different kind is a different revision: the plan's snapshot never
	// answers 304 to the spec's token.
	req = httptest.NewRequest(http.MethodGet, "/board/spec/"+name+"/document/snapshot?kind=plan", nil)
	req.Header.Set("If-None-Match", snap.etag)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("plan snapshot against the spec token: %d", rec.Code)
	}

	dl := getStatus(t, h, "/board/spec/"+name+"/document?format=md&kind=tasks")
	if dl.code != http.StatusOK || !strings.HasPrefix(dl.contentType, "text/markdown") || !strings.Contains(dl.disposition, `attachment; filename="`+name+`-tasks.md"`) || !strings.HasPrefix(dl.body, "# ") || strings.Contains(dl.body, "## Problem") {
		t.Fatalf("download: %d %q %q\n%s", dl.code, dl.contentType, dl.disposition, dl.body)
	}
	if !strings.Contains(dl.body, "not authority") {
		t.Fatalf("the download carries the not-authority stamp:\n%s", dl.body)
	}
	// The hidden Markdown source: the HTML parser drops exactly one
	// newline after <pre>, so the template emits one deliberately and
	// the served Markdown itself never begins with one.
	if strings.HasPrefix(dl.body, "\n") {
		t.Fatalf("served Markdown must not start with a newline:\n%q", dl.body[:16])
	}
	if !strings.Contains(page.body, `class="document-source" hidden>`+"\n# ") {
		t.Fatalf("the page must emit one deliberate newline after the <pre> tag\n%s", page.body)
	}
}

// TestDocumentLoadStatus (F2): only the loader's own not-found phrase for
// THIS spec (or the board's sentinel) is a 404; every other failure —
// including one that merely contains "not found", such as a missing git
// executable — stays operational.
func TestDocumentLoadStatus(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		want int
	}{
		{"lockbox", errors.New("specdocload: spec/lockbox not found in either zone of the working tree (tried a and b)"), http.StatusNotFound},
		{"lockbox", errors.New("specdocload: spec/lockbox not found at abc123 in either zone"), http.StatusNotFound},
		{"lockbox", fmt.Errorf("workbench: spec %q not found: %w", "Bad Name", ErrBoardNotFound), http.StatusNotFound},
		{"lockbox", errors.New(`specdocload: resolving HEAD: exec: "git": executable file not found in $PATH`), http.StatusInternalServerError},
		{"lockbox", errors.New("specdocload: spec/other not found in either zone of the working tree"), http.StatusInternalServerError},
		{"lockbox", errors.New("specdocload: reading spec.md: permission denied"), http.StatusInternalServerError},
	} {
		if got := documentLoadStatus(c.name, c.err); got != c.want {
			t.Errorf("documentLoadStatus(%q, %v) = %d, want %d", c.name, c.err, got, c.want)
		}
	}
}

// TestBoardDocument_DownloadMatchesSnapshotMarkdown is the tab's own
// parity witness: the bytes the download hands over are exactly the
// snapshot's markdown field (one render, one loader).
func TestBoardDocument_DownloadMatchesSnapshotMarkdown(t *testing.T) {
	h, _, name := newAcceptedWallFixture(t)
	snap := getStatus(t, h, "/board/spec/"+name+"/document/snapshot?kind=plan")
	if snap.code != http.StatusOK {
		t.Fatalf("snapshot: %d\n%s", snap.code, snap.body)
	}
	var decoded struct {
		Revision string `json:"revision"`
		HTML     string `json:"html"`
		Markdown string `json:"markdown"`
		Kind     string `json:"kind"`
		Ref      string `json:"ref"`
		Proposed bool   `json:"proposed"`
	}
	if err := decodeStrictJSON(t, snap.body, &decoded); err != nil {
		t.Fatalf("snapshot decode: %v\n%s", err, snap.body)
	}
	if decoded.Kind != "plan" || decoded.Ref != "spec/"+name || decoded.Proposed {
		t.Fatalf("snapshot facts: %+v", decoded)
	}
	dl := getStatus(t, h, "/board/spec/"+name+"/document?format=md&kind=plan")
	if dl.body != decoded.Markdown {
		t.Fatalf("download differs from snapshot markdown:\n--- download\n%s\n--- snapshot\n%s", dl.body, decoded.Markdown)
	}
	if strings.Trim(dl.etag, `"`) != decoded.Revision {
		t.Fatalf("download etag %q vs revision %q", dl.etag, decoded.Revision)
	}
}

func TestBoardDocument_ReadinessGatedByTarget(t *testing.T) {
	snap := readinesspilot.Snapshot{TargetRef: "spec/other", Areas: []readinesspilot.Area{{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateProven}}, CurrentFocus: readinesspilot.AreaShape}
	h, _, name := newAcceptedWallFixtureWithReadiness(t, &snap)
	page := getStatus(t, h, "/board/spec/"+name+"/document")
	if !strings.Contains(page.body, "Readiness was not supplied for this render.") || strings.Contains(page.body, "Define the work") || strings.Contains(page.body, "readiness snapshot for") {
		t.Fatalf("a snapshot for another spec must not render here:\n%s", page.body)
	}
	snap.TargetRef = "spec/" + name
	h, _, name = newAcceptedWallFixtureWithReadiness(t, &snap)
	page = getStatus(t, h, "/board/spec/"+name+"/document")
	if !strings.Contains(page.body, "readiness snapshot for") || !strings.Contains(page.body, "Define the work") {
		t.Fatalf("matching snapshot must render:\n%s", page.body)
	}
	// The branch mount carries the same snapshot (branchboard.go passes
	// deps.Readiness to every per-branch instance; a branch checked out at
	// the serving root dispatches into the serving instance).
	page = getStatus(t, h, "/b/main/board/spec/"+name+"/document")
	if page.code != http.StatusOK || !strings.Contains(page.body, "readiness snapshot for") || !strings.Contains(page.body, "Define the work") {
		t.Fatalf("branch mount must render the matching snapshot: %d\n%s", page.code, page.body)
	}
}

func TestBoardDocument_Refusals(t *testing.T) {
	h, _, name := newAcceptedWallFixture(t)
	for _, c := range []struct{ path, want string }{
		{"/board/spec/" + name + "/document?kind=chapter", "unknown document kind"},
		{"/board/spec/" + name + "/document?format=pdf", "format"},
		{"/board/spec/nope/document", "not found"},
		{"/board/spec/Not%20A%20Name/document", "not found"},
		{"/board/spec/" + name + "/document/snapshot?kind=chapter", "unknown document kind"},
		{"/board/spec/nope/document/snapshot", "not found"},
		{"/board/spec/" + name + "/document/snapshot?format=pdf", "format"},
	} {
		res := getStatus(t, h, c.path)
		if res.code < 400 || res.code >= 500 || !strings.Contains(res.body, c.want) {
			t.Errorf("%s: %d %q", c.path, res.code, res.body)
			continue
		}
		// The HTML route fails as the board does — renderError's page —
		// while /snapshot keeps JSON errors for its fetch client.
		if strings.Contains(c.path, "/snapshot") {
			if !strings.HasPrefix(res.contentType, "application/json") || !strings.Contains(res.body, `"error":`) {
				t.Errorf("%s: snapshot refusal must be JSON: %q %q", c.path, res.contentType, res.body)
			}
		} else if !strings.HasPrefix(res.contentType, "text/html") || !strings.Contains(res.body, `class="error-page"`) {
			t.Errorf("%s: page refusal must be the HTML error page: %q %q", c.path, res.contentType, res.body)
		}
	}
	// ?format=md on /snapshot is accepted and ignored: the projection is
	// always JSON (documentFormatFromQuery's contract).
	if res := getStatus(t, h, "/board/spec/"+name+"/document/snapshot?format=md"); res.code != http.StatusOK || !strings.HasPrefix(res.contentType, "application/json") {
		t.Errorf("snapshot?format=md: %d %q", res.code, res.contentType)
	}
	for _, path := range []string{"/board/spec/" + name + "/document", "/board/spec/" + name + "/document/snapshot"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s must be refused, got %d", path, rec.Code)
		}
	}
}

// TestBoardDocument_BranchMountDerivesLinksFromRequestPath: the same
// route table serves /b/{branch}/...; every link on the page is derived
// from the request path (escaped, so the branch's %2F survives), never
// from a raw ref — a branch already checked out at the serving root
// dispatches into the serving instance, which is what this fixture
// exercises without cutting a worktree.
func TestBoardDocument_BranchMountDerivesLinksFromRequestPath(t *testing.T) {
	h, _, name := newAcceptedWallFixture(t)
	base := "/b/main/board/spec/" + name
	page := getStatus(t, h, base+"/document?kind=tasks")
	if page.code != http.StatusOK {
		t.Fatalf("branch-mount page: %d\n%s", page.code, page.body)
	}
	for _, want := range []string{
		`href="` + base + `" data-testid="document-tab-board"`,
		`href="` + base + `/document?kind=spec" data-testid="document-kind-spec"`,
		`data-snapshot-href="` + base + `/document/snapshot?kind=tasks"`,
		`href="` + base + `/document?format=md&amp;kind=tasks" download="` + name + `-tasks.md"`,
	} {
		if !strings.Contains(page.body, want) {
			t.Errorf("branch-mount page lacks %q\n%s", want, page.body)
		}
	}
	snap := getStatus(t, h, base+"/document/snapshot?kind=tasks")
	if snap.code != http.StatusOK || snap.etag == "" {
		t.Fatalf("branch-mount snapshot: %d\n%s", snap.code, snap.body)
	}
}

// TestBoardPage_LinksToDocumentTab: the board page's nav carries the
// Document link, derived from the request path so the branch mount's
// escaped branch segment survives; a projection without a request path
// (the sealed remote-only render, whose document route refuses for want
// of a working tree) renders no link rather than a dead one.
func TestBoardPage_LinksToDocumentTab(t *testing.T) {
	root := newClaimWallFixture(t)
	h := NewHandler(root)
	rec := getBoard(t, h, claimWallName)
	if rec.Code != http.StatusOK {
		t.Fatalf("board page: %d\n%s", rec.Code, rec.Body.String())
	}
	want := `<a href="/board/spec/` + claimWallName + `/document" data-testid="board-tab-document">Document</a>`
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("board nav lacks %q\n%s", want, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/b/design%2F"+claimWallName+"/board/spec/"+claimWallName, nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("branch board page: %d\n%s", rec.Code, rec.Body.String())
	}
	want = `<a href="/b/design%2F` + claimWallName + `/board/spec/` + claimWallName + `/document" data-testid="board-tab-document">Document</a>`
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("branch board nav lacks %q\n%s", want, rec.Body.String())
	}
	page, err := renderBoardSpecPage(&BoardProjection{Spec: "s", Title: "S", Mode: modeReadOnly}, &boardGitState{}, testASDView())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(page), "board-tab-document") {
		t.Fatalf("a projection without a request path must render no Document link:\n%s", page)
	}
}

func TestBoardDocumentAssetBudget(t *testing.T) {
	const ceiling = 64 * 1024
	data, err := embeddedAssets.ReadFile("assets/specdocument.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > ceiling || len(data) == 0 {
		t.Fatalf("assets/specdocument.js is %d bytes; want 1..%d", len(data), ceiling)
	}
	rec := httptest.NewRecorder()
	NewHandler(t.TempDir()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/specdocument.js", nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/javascript") {
		t.Fatalf("asset route: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}
