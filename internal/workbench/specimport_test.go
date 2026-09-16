package workbench

// Task 4 UI handler tests for the mechanical spec importer's browser
// adapter (docs/superpowers/specs/2026-09-14-spec-import-contract.md,
// "Errors and browser behavior"; the Task 4 UI preflight's transport
// binding). Every case drives the FULL workbench handler over a real,
// hermetic store: a minimal manifest-only checkout on main with a bare
// local origin whose HEAD names main — a SYNTHETIC default-branch proof
// (the same convention cmd/e2eharness's provisioners use), never CI or an
// owner approval — no adopted assistance policy, no model override, no
// forge/tracker/provider configuration of any kind.
//
// The adapter consumes the accepted Task 3 service through the production
// NewService wiring and the existing browser-human actor; the transport
// binding under test is: both POST bodies are the exact strict Request
// JSON, apply additionally carries the returned digest in
// X-Verdi-Import-Preview, and every failure maps onto the contract's own
// closed codes with the HTTP statuses the contract names.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specimport"
	"github.com/jyang234/verdi/internal/store"
)

// specImportFixtureDir is the accepted Task 1 fixture tree: the browser
// adapter reuses its pinned bytes rather than copying them (the F13 inputs
// are pinned to an exact SHA-256 the reference profile binds).
const specImportFixtureDir = "../specimport/testdata"

func readSpecImportFixture(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(specImportFixtureDir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", rel, err)
	}
	return data
}

// gitRun runs one git command against dir with a deterministic identity,
// failing the test on error — the fixture's own plumbing seam, never a
// production code path.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=verdi-test", "GIT_AUTHOR_EMAIL=test@verdi.invalid", "GIT_AUTHOR_DATE=1704067200 +0000",
		"GIT_COMMITTER_NAME=verdi-test", "GIT_COMMITTER_EMAIL=test@verdi.invalid", "GIT_COMMITTER_DATE=1704067200 +0000",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newSpecImportStore builds the hermetic import store described in the
// file comment and returns its root. The checkout sits on a clean main.
func newSpecImportStore(t *testing.T) string {
	t.Helper()
	neutralizeCIEnv(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed store",
	}})
	origin := filepath.Join(t.TempDir(), "origin.git")
	gitRun(t, "", "init", "--bare", "--quiet", "--initial-branch=main", origin)
	gitRun(t, repo.Dir, "remote", "add", "origin", origin)
	gitRun(t, repo.Dir, "push", "--quiet", "--set-upstream", "origin", "main")
	gitRun(t, repo.Dir, "remote", "set-head", "origin", "main")
	return repo.Dir
}

// importSource builds one Source JSON object from raw bytes — base64 of
// the exact bytes, the label a display name that is never a path.
func importSource(id, label string, data []byte) map[string]any {
	return map[string]any{"id": id, "label": label, "data": data}
}

// labeledRequest is the positive labeled-Markdown request over the
// accepted positive-basic.md fixture.
func labeledRequest(t *testing.T, slug string) map[string]any {
	t.Helper()
	return map[string]any{
		"schema":  specimport.RequestSchema,
		"target":  map[string]any{"slug": slug, "class": "feature", "title": "Widget Import"},
		"format":  specimport.FormatMarkdownV1,
		"primary": "widget",
		"sources": []any{importSource("widget", "positive-basic.md", readSpecImportFixture(t, "markdown/positive-basic.md"))},
		// Both dispositions are explicit and false until the user acts.
		"defer_statements": false,
		"retain_unmapped":  false,
	}
}

func mustJSONBody(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// importClient wraps one httptest server with browser-shaped requests: a
// same-origin Origin header and Sec-Fetch-Site: same-origin by default
// (exactly what a modern browser sends for the page's own fetch), which
// individual cases override to prove the cross-origin refusal.
type importClient struct {
	t   *testing.T
	srv *httptest.Server
}

func newImportClient(t *testing.T, h http.Handler) *importClient {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &importClient{t: t, srv: srv}
}

func (c *importClient) do(method, path string, body []byte, headers map[string]string) (int, http.Header, []byte) {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.srv.URL+path, reader)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Origin", c.srv.URL)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		if v == "" {
			req.Header.Del(k)
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := c.srv.Client().Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, out
}

func (c *importClient) get(path string) (int, string) {
	status, _, body := c.do(http.MethodGet, path, nil, nil)
	return status, string(body)
}

// importFailure is the adapter's JSON failure body: the contract's closed
// code plus a safe, human-readable message.
type importFailure struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

func (c *importClient) preview(request any) (int, specimport.PreviewResult, importFailure) {
	c.t.Helper()
	status, _, body := c.do(http.MethodPost, "/design/import/preview", mustJSONBody(c.t, request), nil)
	var result specimport.PreviewResult
	var failure importFailure
	if status == http.StatusOK {
		if err := json.Unmarshal(body, &result); err != nil {
			c.t.Fatalf("decoding preview: %v\n%s", err, body)
		}
	} else {
		_ = json.Unmarshal(body, &failure)
	}
	return status, result, failure
}

func (c *importClient) apply(request any, digest string, headers map[string]string) (int, specimport.Result, importFailure) {
	c.t.Helper()
	h := map[string]string{"X-Verdi-Import-Preview": digest}
	for k, v := range headers {
		h[k] = v
	}
	status, _, body := c.do(http.MethodPost, "/design/import/apply", mustJSONBody(c.t, request), h)
	var result specimport.Result
	var failure importFailure
	if status == http.StatusOK {
		if err := json.Unmarshal(body, &result); err != nil {
			c.t.Fatalf("decoding apply result: %v\n%s", err, body)
		}
	} else {
		_ = json.Unmarshal(body, &failure)
	}
	return status, result, failure
}

func findingCounts(findings []specimport.Finding) map[string]int {
	counts := map[string]int{}
	for _, f := range findings {
		counts[f.Code]++
	}
	return counts
}

func fieldByTarget(fields []specimport.Field, target string) (specimport.Field, bool) {
	for _, f := range fields {
		if f.Target == target {
			return f, true
		}
	}
	return specimport.Field{}, false
}

var hexDigestRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// TestSpecImport_HomeAndPageDiscoverable proves the import page is
// reachable from home BEFORE any statement is requested, that the page
// carries the labeled file/format/target/disposition/confirmation
// controls, and that every route holds its method guard.
func TestSpecImport_HomeAndPageDiscoverable(t *testing.T) {
	root := newSpecImportStore(t)
	c := newImportClient(t, NewHandlerWith(root, Deps{}))

	status, home := c.get("/")
	if status != http.StatusOK {
		t.Fatalf("GET / = %d", status)
	}
	link := strings.Index(home, `data-testid="home-import-link"`)
	if link < 0 {
		t.Fatalf("home page has no import link: %s", home)
	}
	if !strings.Contains(home[link-200:link+200], `href="/design/import"`) {
		t.Fatalf("home import link does not address /design/import: %s", home[link-200:link+200])
	}
	glance := strings.Index(home, `data-testid="home-glance"`)
	if glance >= 0 && link > glance {
		t.Fatalf("the import link must be discoverable before the glance/directory sections (link at %d, glance at %d)", link, glance)
	}

	status, page := c.get("/design/import")
	if status != http.StatusOK {
		t.Fatalf("GET /design/import = %d: %s", status, page)
	}
	for _, want := range []string{
		`id="import-form"`, `id="import-files"`, `type="file"`, `multiple`,
		`id="import-format"`, `value="markdown-v1"`, `value="native"`, `value="f13-reference-v1"`, `value="manual-v1"`,
		`id="import-slug"`, `id="import-class"`, `id="import-title"`, `id="import-story"`,
		`id="import-retain"`, `id="import-defer"`,
		`id="import-mapping-list"`, `id="import-add-mapping"`, `id="import-link-list"`, `id="import-add-link"`,
		`id="import-preview-btn"`, `id="import-confirm"`, `id="import-apply-btn"`, `id="import-next-action"`, `id="import-retry-btn"`,
		`id="import-findings"`, `id="import-fields"`, `id="import-coverage"`,
		`src="/assets/specimport.js"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("import page missing %q", want)
		}
	}
	// Labeled controls: every input the user drives carries a label.
	for _, id := range []string{"import-files", "import-format", "import-slug", "import-class", "import-title", "import-story"} {
		if !strings.Contains(page, `for="`+id+`"`) {
			t.Errorf("import page control %s has no label", id)
		}
	}
	// No source command, URL fetch, archive or recursion affordance.
	for _, banned := range []string{`webkitdirectory`, `type="url"`, `.zip`, `accept="`} {
		if strings.Contains(page, banned) {
			t.Errorf("import page must not offer %q", banned)
		}
	}

	status, _, _ = c.do(http.MethodPost, "/design/import", []byte("{}"), nil)
	if status != http.StatusMethodNotAllowed {
		t.Errorf("POST /design/import = %d, want 405", status)
	}
	for _, path := range []string{"/design/import/preview", "/design/import/apply"} {
		status, _ = c.get(path)
		if status != http.StatusMethodNotAllowed {
			t.Errorf("GET %s = %d, want 405", path, status)
		}
	}
	status, _, _ = c.do(http.MethodPost, "/design/import/record?branch=main&spec=x", []byte("{}"), nil)
	if status != http.StatusMethodNotAllowed {
		t.Errorf("POST /design/import/record = %d, want 405", status)
	}

	status, _, body := c.do(http.MethodGet, "/assets/specimport.js", nil, nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte("/design/import/preview")) {
		t.Errorf("GET /assets/specimport.js = %d (%d bytes)", status, len(body))
	}
	if len(body) > 96*1024 {
		t.Errorf("specimport.js is %d bytes, over the 96 KiB structural ceiling", len(body))
	}
}

// TestSpecImport_LabeledMarkdown_PreviewCorrectApplyRecord is the
// end-to-end handler journey: a labeled Markdown source previews with its
// truthful blocking findings; explicit evidence selection, retained
// acknowledgement and one user-edited mapping make it ready; apply through
// the browser-human actor publishes exactly the contract's result while the
// process's already-held writer lock is REUSED (never released, never a
// second lock); the ordinary board carries the adjacent source-record link;
// and the record view verifies the committed bytes.
func TestSpecImport_LabeledMarkdown_PreviewCorrectApplyRecord(t *testing.T) {
	root := newSpecImportStore(t)
	c := newImportClient(t, NewHandlerWith(root, Deps{}))
	ctx := context.Background()

	// A running `verdi serve` holds the checkout's writer lock for its
	// lifetime; the handler runs inside that process and Apply must reuse
	// the held lock through draftmutation's registry-proven lease.
	lockPath := store.WriterLockPath(root)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}
	lockFile, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("acquiring the writer lock: %v", err)
	}
	t.Cleanup(func() { _ = filelock.Release(lockFile, lockPath) })
	lockBefore, err := os.Stat(lockPath)
	if err != nil {
		t.Fatal(err)
	}

	slug := "widget-import"
	request := labeledRequest(t, slug)
	primary := readSpecImportFixture(t, "markdown/positive-basic.md")

	// Preview 1: nothing retained, no evidence — ready:false with the exact
	// blocking findings, still 200 (a completed preview, never fabricated
	// success).
	status, first, failure := c.preview(request)
	if status != http.StatusOK {
		t.Fatalf("preview 1 = %d %+v", status, failure)
	}
	if first.Ready {
		t.Fatalf("preview 1 ready = true, want false: %+v", first.Findings)
	}
	counts := findingCounts(first.Findings)
	if counts[specimport.FindingUnresolvedCoverage] != 1 || counts[specimport.FindingMissingEvidence] != 3 {
		t.Fatalf("preview 1 findings = %+v", first.Findings)
	}
	problem, ok := fieldByTarget(first.Fields, "problem")
	if !ok || problem.Origin != specimport.OriginCopiedSource || problem.Text != "Operators currently retype every requirement by hand.\nThis wastes their afternoon." {
		t.Fatalf("problem field = %+v", problem)
	}
	if len(first.Coverage) != 1 || first.Coverage[0].TotalBytes != len(primary) ||
		first.Coverage[0].TotalBytes != first.Coverage[0].MappedBytes+first.Coverage[0].RetainedBytes+first.Coverage[0].UnresolvedBytes ||
		first.Coverage[0].UnresolvedBytes == 0 {
		t.Fatalf("preview 1 coverage = %+v", first.Coverage)
	}
	if !hexDigestRe.MatchString(first.Digest) {
		t.Fatalf("digest = %q", first.Digest)
	}

	// Preview 2: explicit acknowledgement, evidence selection on every
	// criterion, and ONE user-edited source-backed mapping over ac-2's own
	// automatic span.
	ac2, ok := fieldByTarget(first.Fields, "ac-2")
	if !ok || len(ac2.Spans) != 1 {
		t.Fatalf("ac-2 field = %+v", ac2)
	}
	// Evidence kinds are the user's selection; the accepted candidate
	// lint (VL-006) still requires attestation among a feature criterion's
	// kinds, which the preview reports truthfully as invalid-candidate.
	request["retain_unmapped"] = true
	request["mappings"] = []any{
		map[string]any{"target": "ac-1", "evidence": []string{"static", "attestation"}},
		map[string]any{"target": "ac-2", "source_id": ac2.Spans[0].SourceID, "start": ac2.Spans[0].Start, "end": ac2.Spans[0].End,
			"transform": ac2.Spans[0].Transform, "text": "The importer preserves exact wording (edited by the reviewer).", "evidence": []string{"attestation"}},
		map[string]any{"target": "ac-3", "evidence": []string{"attestation"}},
	}
	status, ready, failure := c.preview(request)
	if status != http.StatusOK || !ready.Ready {
		t.Fatalf("preview 2 = %d ready=%v findings=%+v failure=%+v", status, ready.Ready, ready.Findings, failure)
	}
	edited, _ := fieldByTarget(ready.Fields, "ac-2")
	if edited.Origin != specimport.OriginUserEditedSrc || !strings.Contains(edited.Text, "(edited by the reviewer)") || len(edited.Spans) != 1 {
		t.Fatalf("edited ac-2 = %+v", edited)
	}
	one, _ := fieldByTarget(ready.Fields, "ac-1")
	if strings.Join(one.Evidence, ",") != "static,attestation" {
		t.Fatalf("ac-1 evidence = %v", one.Evidence)
	}
	if ready.Coverage[0].UnresolvedBytes != 0 || ready.Coverage[0].RetainedBytes == 0 {
		t.Fatalf("preview 2 coverage = %+v", ready.Coverage)
	}

	// The apply transport binding: the digest rides X-Verdi-Import-Preview.
	if status, _, failure := c.apply(request, "", nil); status != http.StatusBadRequest || failure.Code != "invalid-request" {
		t.Fatalf("apply without digest = %d %+v, want 400 invalid-request", status, failure)
	}
	if status, _, failure := c.apply(request, "not-a-digest", nil); status != http.StatusBadRequest || failure.Code != "invalid-request" {
		t.Fatalf("apply with malformed digest = %d %+v, want 400 invalid-request", status, failure)
	}
	if status, _, failure := c.apply(request, strings.Repeat("a", 64), nil); status != http.StatusConflict || failure.Code != "stale-preview" {
		t.Fatalf("apply with a stale digest = %d %+v, want 409 stale-preview", status, failure)
	}

	status, created, failure := c.apply(request, ready.Digest, nil)
	if status != http.StatusOK {
		t.Fatalf("apply = %d %+v", status, failure)
	}
	if created.Schema != specimport.ResultSchema || created.Status != specimport.StatusCreated ||
		created.Branch != "design/"+slug || created.SpecRef != "spec/"+slug || created.PreviewDigest != ready.Digest ||
		created.BoardPath != "/b/design%2F"+slug+"/board/spec/"+slug || created.StatementsDeferred {
		t.Fatalf("created = %+v", created)
	}

	// The held lock was reused: same open file, same pid, never recreated.
	lockAfter, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("writer lock is gone after apply: %v", err)
	}
	if !os.SameFile(lockBefore, lockAfter) {
		t.Fatal("writer lock file identity changed across apply — it was released and recreated instead of reused")
	}
	// The caller's checkout/index are untouched and clean; HEAD stays main.
	if branch, _ := gitx.CurrentBranch(ctx, root); branch != "main" {
		t.Fatalf("serving checkout moved to %q", branch)
	}
	if porcelain := gitRun(t, root, "status", "--porcelain"); porcelain != "" {
		t.Fatalf("checkout dirty after apply: %q", porcelain)
	}
	if exists, _ := gitx.HasLocalBranch(ctx, root, created.Branch); !exists {
		t.Fatalf("branch %s was not published", created.Branch)
	}

	// Identical retry: the same commit reported already-created, never a
	// second branch or a renamed target.
	status, again, failure := c.apply(request, ready.Digest, nil)
	if status != http.StatusOK || again.Status != specimport.StatusAlreadyCreated || again.Commit != created.Commit || again.SpecRef != created.SpecRef {
		t.Fatalf("retry = %d %+v %+v", status, again, failure)
	}

	// The ordinary branch board renders in authoring mode and carries the
	// adjacent source-record link beside the review panels — explaining
	// the import origin separately from ASD history and acceptance.
	status, board := c.get(created.BoardPath)
	if status != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", created.BoardPath, status, board)
	}
	if !strings.Contains(board, `data-board-mode="authoring"`) {
		t.Fatalf("imported board is not authoring: %s", board)
	}
	origin := strings.Index(board, `data-testid="asd-import-origin"`)
	if origin < 0 {
		t.Fatalf("imported board has no source-record affordance")
	}
	review := strings.Index(board, `data-testid="asd-review"`)
	if review < 0 || origin < review || origin-review > 2000 {
		t.Fatalf("source-record affordance is not adjacent to the semantic review panel (review at %d, origin at %d)", review, origin)
	}
	recordHref := "/design/import/record?branch=design%2F" + slug + "&amp;spec=" + slug
	panel := board[origin : origin+1500]
	if !strings.Contains(panel, recordHref) {
		t.Fatalf("source-record affordance lacks the record href %s: %s", recordHref, panel)
	}
	for _, want := range []string{"not verified here", "not an ASD provenance entry", "classify the creation as unclassified", "not evidence of acceptance"} {
		if !strings.Contains(panel, want) {
			t.Errorf("source-record affordance must explain the import origin separately from ASD history and acceptance (missing %q): %s", want, panel)
		}
	}

	// The record view: committed bytes verified, current spec matching.
	status, record := c.get("/design/import/record?branch=design%2F" + slug + "&spec=" + slug)
	if status != http.StatusOK {
		t.Fatalf("GET record = %d: %s", status, record)
	}
	for _, want := range []string{
		`data-testid="import-record"`, `data-current-spec-matches="true"`, created.Commit, ready.Digest,
		"(edited by the reviewer)", "unauthenticated", "not-applicable", "positive-basic.md",
	} {
		if !strings.Contains(record, want) {
			t.Errorf("record view missing %q", want)
		}
	}
	if strings.Contains(record, `data-testid="record-current-spec-changed"`) {
		t.Errorf("record view claims a changed current spec for an untouched import")
	}

	// Malformed record queries are distinct from unavailable proof.
	for _, query := range []string{"", "?branch=design%2F" + slug, "?spec=" + slug, "?branch=-x&spec=" + slug, "?branch=design%2F" + slug + "&spec=Bad%20Slug", "?branch=a%2F..%2Fb&spec=" + slug} {
		status, body := c.get("/design/import/record" + query)
		if status != http.StatusBadRequest || !strings.Contains(body, `data-testid="import-record-invalid"`) {
			t.Errorf("record %q = %d, want 400 invalid page: %s", query, status, body)
		}
	}
	status, missing := c.get("/design/import/record?branch=design%2Fno-such-import&spec=no-such-import")
	if status != http.StatusConflict || !strings.Contains(missing, `data-testid="import-record-unavailable"`) || !strings.Contains(missing, "provenance-mismatch") {
		t.Errorf("record for an absent import = %d, want 409 unavailable page: %s", status, missing)
	}
	if strings.Contains(missing, "gitx:") || strings.Contains(missing, "fatal:") || strings.Contains(missing, "exit status") {
		t.Errorf("record unavailable page leaks raw git command detail: %s", missing)
	}
}

// commitOnBranchDetached commits mutate's edit of relPath on branch through
// a detached temporary worktree — an ordinary descendant commit, never the
// importer's own path — leaving the serving checkout untouched.
func commitOnBranchDetached(t *testing.T, root, branch, relPath string, mutate func([]byte) []byte, message string) {
	t.Helper()
	ctx := context.Background()
	tip := gitRun(t, root, "rev-parse", "refs/heads/"+branch)
	wt := filepath.Join(t.TempDir(), "wt")
	if err := gitx.WorktreeAddDetached(ctx, root, wt, tip); err != nil {
		t.Fatal(err)
	}
	defer func() { gitRun(t, root, "worktree", "remove", "--force", wt) }()
	path := filepath.Join(wt, filepath.FromSlash(relPath))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, mutate(data), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wt, "add", "-A")
	gitRun(t, wt, "commit", "--quiet", "--no-verify", "-m", message)
	next := gitRun(t, wt, "rev-parse", "HEAD")
	gitRun(t, root, "update-ref", "refs/heads/"+branch, next, tip)
}

// importLabeled runs the ready labeled journey for slug and returns the
// created result.
func importLabeled(t *testing.T, c *importClient, slug string) specimport.Result {
	t.Helper()
	request := labeledRequest(t, slug)
	request["retain_unmapped"] = true
	request["mappings"] = []any{
		map[string]any{"target": "ac-1", "evidence": []string{"attestation"}},
		map[string]any{"target": "ac-2", "evidence": []string{"attestation"}},
		map[string]any{"target": "ac-3", "evidence": []string{"attestation"}},
	}
	status, preview, failure := c.preview(request)
	if status != http.StatusOK || !preview.Ready {
		t.Fatalf("preview = %d ready=%v %+v %+v", status, preview.Ready, preview.Findings, failure)
	}
	status, created, failure := c.apply(request, preview.Digest, nil)
	if status != http.StatusOK {
		t.Fatalf("apply = %d %+v", status, failure)
	}
	return created
}

// TestSpecImport_RecordView_CurrentSpecChangedIsDistinctFromCorruption
// proves the record view stays truthful after an ordinary later edit of
// the branch (the original import remains verifiable; the current spec is
// disclosed as changed, never as corrupted provenance) and that a tampered
// record renders as unavailable proof — a different surface from the
// changed-current-spec disclosure, never a pass.
func TestSpecImport_RecordView_CurrentSpecChangedIsDistinctFromCorruption(t *testing.T) {
	root := newSpecImportStore(t)
	c := newImportClient(t, NewHandlerWith(root, Deps{}))
	slug := "record-honesty"
	created := importLabeled(t, c, slug)
	recordPath := "/design/import/record?branch=design%2F" + slug + "&spec=" + slug

	commitOnBranchDetached(t, root, created.Branch, store.ActiveSpecRelPath(slug), func(data []byte) []byte {
		return append(data, []byte("\nEdited after import through an ordinary supported change.\n")...)
	}, "ordinary later edit")

	status, page := c.get(recordPath)
	if status != http.StatusOK {
		t.Fatalf("record after edit = %d: %s", status, page)
	}
	for _, want := range []string{`data-current-spec-matches="false"`, `data-testid="record-current-spec-changed"`, "current-spec-changed", created.Commit, "ORIGINAL imported revision", "do not corrupt"} {
		if !strings.Contains(page, want) {
			t.Errorf("record after an ordinary edit missing %q", want)
		}
	}
	if strings.Contains(page, `data-testid="import-record-unavailable"`) {
		t.Errorf("an ordinary later edit must not read as unavailable/corrupt proof")
	}

	// Corruption: a truncated record.json committed out of band.
	commitOnBranchDetached(t, root, created.Branch, store.ImportRecordRelPath(slug, created.PreviewDigest), func(data []byte) []byte {
		return data[:10]
	}, "tamper with the import record")
	status, page = c.get(recordPath)
	if status != http.StatusConflict {
		t.Fatalf("record over a truncated record.json = %d, want 409: %s", status, page)
	}
	if !strings.Contains(page, `data-testid="import-record-unavailable"`) || !strings.Contains(page, "provenance-mismatch") {
		t.Errorf("truncated record must render as unavailable proof: %s", page)
	}
	if strings.Contains(page, `data-testid="record-current-spec-changed"`) || strings.Contains(page, `data-current-spec-matches=`) {
		t.Errorf("a corrupt record must not render the changed-current-spec disclosure or any match claim")
	}
}

// TestSpecImport_F13Profile_AbsentLabelsSeparateFromEvidenceAndDeferral
// drives the pinned F13 inputs through the reference profile: the two
// absent statement labels and the eight unset evidence declarations are
// separate findings; explicit pair deferral replaces the statement findings
// with nonblocking TODO disclosures; evidence selection completes the
// import, whose created AND already-created results keep the deferral
// disclosures.
func TestSpecImport_F13Profile_AbsentLabelsSeparateFromEvidenceAndDeferral(t *testing.T) {
	root := newSpecImportStore(t)
	c := newImportClient(t, NewHandlerWith(root, Deps{}))

	var sources []any
	for _, name := range []string{"primary-f13", "transition-core", "review-journal", "review-validation", "stage-plan-c346c005"} {
		sources = append(sources, importSource(name, name+".md", readSpecImportFixture(t, "f13/"+name+".md")))
	}
	request := map[string]any{
		"schema":           specimport.RequestSchema,
		"target":           map[string]any{"slug": "gatekeeper-flight", "class": "feature", "title": "Bounded gatekeeper state machine"},
		"format":           specimport.FormatF13Reference,
		"primary":          "primary-f13",
		"sources":          sources,
		"defer_statements": false,
		"retain_unmapped":  true,
	}

	status, preview, failure := c.preview(request)
	if status != http.StatusOK || preview.Ready {
		t.Fatalf("f13 preview = %d ready=%v %+v", status, preview.Ready, failure)
	}
	// The two absent statement labels and the eight unset evidence
	// declarations are separate blocking findings (the candidate lint adds
	// its own blocking splice finding until evidence exists; it is reported
	// beside them, never folded in).
	counts := findingCounts(preview.Findings)
	if counts[specimport.FindingMissingStatement] != 2 || counts[specimport.FindingMissingEvidence] != 8 {
		t.Fatalf("f13 findings = %+v", preview.Findings)
	}
	for _, f := range preview.Findings {
		if (f.Code == specimport.FindingMissingStatement || f.Code == specimport.FindingMissingEvidence) && !f.Blocking {
			t.Fatalf("f13 finding must block: %+v", f)
		}
	}
	if len(preview.Fields) != 8 || len(preview.Sources) != 5 || len(preview.Coverage) != 5 {
		t.Fatalf("f13 shape: %d fields, %d sources, %d coverage", len(preview.Fields), len(preview.Sources), len(preview.Coverage))
	}
	for _, cov := range preview.Coverage {
		if cov.SourceID == "primary-f13" {
			if cov.TotalBytes != 2409 || cov.MappedBytes == 0 || cov.UnresolvedBytes != 0 {
				t.Fatalf("primary coverage = %+v", cov)
			}
		} else if cov.RetainedBytes != cov.TotalBytes {
			t.Fatalf("support %s must be one whole retained unit: %+v", cov.SourceID, cov)
		}
	}

	request["defer_statements"] = true
	status, deferred, _ := c.preview(request)
	if status != http.StatusOK || deferred.Ready {
		t.Fatalf("deferred preview = %d ready=%v", status, deferred.Ready)
	}
	counts = findingCounts(deferred.Findings)
	if counts[specimport.FindingMissingStatement] != 0 || counts[specimport.FindingStatementsDeferred] != 2 || counts[specimport.FindingMissingEvidence] != 8 {
		t.Fatalf("deferred findings = %+v", deferred.Findings)
	}
	problem, _ := fieldByTarget(deferred.Fields, "problem")
	if problem.Origin != specimport.OriginGeneratedDeferral || problem.Text != designscaffold.DefaultProblem {
		t.Fatalf("deferred problem = %+v", problem)
	}

	var mappings []any
	for i := 1; i <= 8; i++ {
		mappings = append(mappings, map[string]any{"target": "ac-" + string(rune('0'+i)), "evidence": []string{"attestation"}})
	}
	request["mappings"] = mappings
	status, ready, failure := c.preview(request)
	if status != http.StatusOK || !ready.Ready {
		t.Fatalf("evidenced deferred preview = %d ready=%v %+v %+v", status, ready.Ready, ready.Findings, failure)
	}
	status, created, failure := c.apply(request, ready.Digest, nil)
	if status != http.StatusOK || !created.StatementsDeferred || findingCounts(created.Disclosures)[specimport.FindingStatementsDeferred] != 2 {
		t.Fatalf("f13 apply = %d %+v %+v", status, created, failure)
	}
	status, again, _ := c.apply(request, ready.Digest, nil)
	if status != http.StatusOK || again.Status != specimport.StatusAlreadyCreated || !again.StatementsDeferred || findingCounts(again.Disclosures)[specimport.FindingStatementsDeferred] != 2 {
		t.Fatalf("f13 retry = %d %+v", status, again)
	}
}

// TestSpecImport_AmbiguousHeadingIsReportedAndCorrectable proves a
// duplicated statement label is an explicit ambiguous-field finding and
// that an explicit mapping resolves it without the importer choosing.
func TestSpecImport_AmbiguousHeadingIsReportedAndCorrectable(t *testing.T) {
	root := newSpecImportStore(t)
	c := newImportClient(t, NewHandlerWith(root, Deps{}))
	request := map[string]any{
		"schema":           specimport.RequestSchema,
		"target":           map[string]any{"slug": "ambiguous-widget", "class": "feature", "title": "Ambiguous widget"},
		"format":           specimport.FormatMarkdownV1,
		"primary":          "dup",
		"sources":          []any{importSource("dup", "duplicate-label.md", readSpecImportFixture(t, "markdown/duplicate-label.md"))},
		"defer_statements": false,
		"retain_unmapped":  true,
	}
	status, preview, _ := c.preview(request)
	if status != http.StatusOK || preview.Ready {
		t.Fatalf("ambiguous preview = %d ready=%v", status, preview.Ready)
	}
	var ambiguous *specimport.Finding
	for i := range preview.Findings {
		if preview.Findings[i].Code == specimport.FindingAmbiguousField && preview.Findings[i].Target == "problem" {
			ambiguous = &preview.Findings[i]
		}
	}
	if ambiguous == nil || !ambiguous.Blocking {
		t.Fatalf("no blocking ambiguous-field finding for problem: %+v", preview.Findings)
	}
	if _, ok := fieldByTarget(preview.Fields, "problem"); ok {
		t.Fatalf("an ambiguous statement must not be silently selected: %+v", preview.Fields)
	}

	// The explicit mapping supplies the value; the source-structure gap
	// stays disclosed (nonblocking) rather than silently vanishing.
	request["mappings"] = []any{map[string]any{"target": "problem", "text": "Operators currently retype every requirement by hand."}}
	status, corrected, _ := c.preview(request)
	if status != http.StatusOK {
		t.Fatalf("corrected preview = %d", status)
	}
	for _, f := range corrected.Findings {
		if f.Code == specimport.FindingAmbiguousField && f.Blocking {
			t.Fatalf("explicit mapping did not resolve the blocking ambiguity: %+v", corrected.Findings)
		}
	}
	problem, ok := fieldByTarget(corrected.Fields, "problem")
	if !ok || problem.Origin != specimport.OriginUserAdded {
		t.Fatalf("corrected problem = %+v", problem)
	}
}

// TestSpecImport_TransportRefusals pins the adapter's closed transport
// behavior: strict body grammar, the raw 12 MiB envelope cap, the same-
// origin protection, and the contract's status mapping for dirty context,
// unresolved previews and target collisions.
func TestSpecImport_TransportRefusals(t *testing.T) {
	root := newSpecImportStore(t)
	c := newImportClient(t, NewHandlerWith(root, Deps{}))

	post := func(path string, body []byte, headers map[string]string) (int, importFailure) {
		t.Helper()
		status, _, raw := c.do(http.MethodPost, path, body, headers)
		var failure importFailure
		_ = json.Unmarshal(raw, &failure)
		return status, failure
	}
	valid := mustJSONBody(t, labeledRequest(t, "transport"))

	for name, body := range map[string][]byte{
		"unknown field": []byte(`{"schema":"verdi.spec-import-request/v1","bogus":1}`),
		"null":          []byte(`null`),
		"array":         []byte(`[]`),
		"trailing":      append(append([]byte{}, valid...), []byte(`{}`)...),
		"empty":         []byte(``),
		"scalar":        []byte(`"x"`),
	} {
		status, failure := post("/design/import/preview", body, nil)
		if status != http.StatusBadRequest || failure.Code != "invalid-request" || failure.Error == "" {
			t.Errorf("%s body = %d %+v, want 400 invalid-request", name, status, failure)
		}
	}
	bad := labeledRequest(t, "transport")
	bad["format"] = "docx"
	if status, failure := post("/design/import/preview", mustJSONBody(t, bad), nil); status != http.StatusBadRequest || failure.Code != "unsupported-format" {
		t.Errorf("unsupported format = %d %+v", status, failure)
	}

	oversize := make([]byte, specimport.MaxEnvelopeBytes+1)
	for i := range oversize {
		oversize[i] = ' '
	}
	copy(oversize, valid)
	for _, path := range []string{"/design/import/preview", "/design/import/apply"} {
		status, failure := post(path, oversize, map[string]string{"X-Verdi-Import-Preview": strings.Repeat("a", 64)})
		if status != http.StatusRequestEntityTooLarge || failure.Code != "invalid-request" {
			t.Errorf("oversize %s = %d %+v, want 413 invalid-request", path, status, failure)
		}
	}

	// Cross-origin protection: Sec-Fetch-Site says cross-site, or Origin
	// disagrees with Host without Sec-Fetch-Site — both refused before any
	// decode; a plain same-origin browser fetch and a header-less
	// non-browser client both pass.
	for name, headers := range map[string]map[string]string{
		"cross-site":             {"Sec-Fetch-Site": "cross-site"},
		"same-site":              {"Sec-Fetch-Site": "same-site"},
		"foreign origin, no sfs": {"Sec-Fetch-Site": "", "Origin": "http://evil.invalid"},
	} {
		for _, path := range []string{"/design/import/preview", "/design/import/apply"} {
			h := map[string]string{"X-Verdi-Import-Preview": strings.Repeat("a", 64)}
			for k, v := range headers {
				h[k] = v
			}
			status, _ := post(path, valid, h)
			if status != http.StatusForbidden {
				t.Errorf("%s %s = %d, want 403", name, path, status)
			}
		}
	}
	if status, _ := post("/design/import/preview", valid, map[string]string{"Sec-Fetch-Site": "", "Origin": ""}); status != http.StatusOK {
		t.Errorf("header-less same-origin preview = %d, want 200", status)
	}
	if status, _ := post("/design/import/preview", valid, map[string]string{"Sec-Fetch-Site": "none"}); status != http.StatusOK {
		t.Errorf("Sec-Fetch-Site: none preview = %d, want 200", status)
	}

	// Unresolved: applying a ready:false preview's exact digest is a
	// completed refusal, never a creation.
	status, notReady, _ := c.preview(labeledRequest(t, "transport"))
	if status != http.StatusOK || notReady.Ready {
		t.Fatalf("not-ready preview = %d ready=%v", status, notReady.Ready)
	}
	if status, _, failure := c.apply(labeledRequest(t, "transport"), notReady.Digest, nil); status != http.StatusConflict || failure.Code != "unresolved" {
		t.Errorf("apply of an unresolved preview = %d %+v, want 409 unresolved", status, failure)
	}
	if exists, _ := gitx.HasLocalBranch(context.Background(), root, "design/transport"); exists {
		t.Fatal("an unresolved apply created a branch")
	}

	// Target collision: a different request against an already-created
	// slug refuses as target-exists, never a silently different name.
	created := importLabeled(t, c, "collide")
	other := labeledRequest(t, "collide")
	other["target"].(map[string]any)["title"] = "A different title over the same slug"
	other["retain_unmapped"] = true
	other["mappings"] = []any{
		map[string]any{"target": "ac-1", "evidence": []string{"attestation"}},
		map[string]any{"target": "ac-2", "evidence": []string{"attestation"}},
		map[string]any{"target": "ac-3", "evidence": []string{"attestation"}},
	}
	status, otherPreview, _ := c.preview(other)
	if status != http.StatusOK || !otherPreview.Ready {
		t.Fatalf("colliding preview = %d ready=%v", status, otherPreview.Ready)
	}
	if status, result, failure := c.apply(other, otherPreview.Digest, nil); status != http.StatusConflict || failure.Code != "target-exists" || result.Status != "" {
		t.Errorf("colliding apply = %d %+v %+v, want 409 target-exists", status, result, failure)
	}
	if tip := gitRun(t, root, "rev-parse", "refs/heads/"+created.Branch); tip != created.Commit {
		t.Errorf("collision moved the existing branch to %s", tip)
	}

	// Dirty context: an untracked corpus input refuses preview with the
	// contract's 409 and correction guidance, naming the path but never a
	// raw git command.
	dirtyPath := filepath.Join(root, ".verdi", "specs", "active", "stray", "spec.md")
	if err := os.MkdirAll(filepath.Dir(dirtyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dirtyPath, []byte("stray\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, _, failure := c.preview(labeledRequest(t, "transport"))
	if status != http.StatusConflict || failure.Code != "dirty-context" || !strings.Contains(failure.Error, "stray") {
		t.Errorf("dirty preview = %d %+v, want 409 dirty-context naming the path", status, failure)
	}
	if strings.Contains(failure.Error, "gitx:") || strings.Contains(failure.Error, "exit status") {
		t.Errorf("dirty-context message leaks raw git detail: %s", failure.Error)
	}
}

// TestSpecImport_EscapesSourceAndErrorText proves no source byte, label or
// error detail is ever emitted as markup: the record view and the failure
// pages escape everything they render.
func TestSpecImport_EscapesSourceAndErrorText(t *testing.T) {
	root := newSpecImportStore(t)
	c := newImportClient(t, NewHandlerWith(root, Deps{}))
	xss, err := os.ReadFile(filepath.Join("testdata", "specimport", "xss.md"))
	if err != nil {
		t.Fatal(err)
	}
	slug := "escaped-import"
	request := map[string]any{
		"schema":  specimport.RequestSchema,
		"target":  map[string]any{"slug": slug, "class": "feature", "title": "<b>Escaped</b> title"},
		"format":  specimport.FormatMarkdownV1,
		"primary": "xss",
		"sources": []any{importSource("xss", `<img src=x onerror="window.__xss=3">.md`, xss)},
		"mappings": []any{
			map[string]any{"target": "ac-1", "evidence": []string{"attestation"}},
			map[string]any{"target": "ac-2", "evidence": []string{"attestation"}},
		},
		"defer_statements": false,
		"retain_unmapped":  true,
	}
	status, preview, failure := c.preview(request)
	if status != http.StatusOK || !preview.Ready {
		t.Fatalf("xss preview = %d ready=%v %+v %+v", status, preview.Ready, preview.Findings, failure)
	}
	status, _, failure = c.apply(request, preview.Digest, nil)
	if status != http.StatusOK {
		t.Fatalf("xss apply = %d %+v", status, failure)
	}
	status, page := c.get("/design/import/record?branch=design%2F" + slug + "&spec=" + slug)
	if status != http.StatusOK {
		t.Fatalf("record = %d", status)
	}
	for _, raw := range []string{"<script>", "<img src=x", "<b>never</b>", "<em>literal</em>", "<b>Escaped</b>"} {
		if strings.Contains(page, raw) {
			t.Errorf("record view emits raw source markup %q", raw)
		}
	}
	if !strings.Contains(page, "&lt;script&gt;") || !strings.Contains(page, "&lt;img src=x") {
		t.Errorf("record view does not show the escaped source text")
	}

	status, page = c.get("/design/import/record?branch=design%2F%3Cb%3Eevil%3C%2Fb%3E&spec=" + slug)
	if status != http.StatusConflict || strings.Contains(page, "<b>evil</b>") || !strings.Contains(page, "&lt;b&gt;evil&lt;/b&gt;") {
		t.Errorf("unavailable page must escape the branch name: %d %s", status, page)
	}
}

// TestSpecImport_BoardWithoutImportRecordHasNoSourceRecordLink pins the
// affordance's gate: an ordinary authoring board that was never imported
// renders no source-record link at all.
func TestSpecImport_BoardWithoutImportRecordHasNoSourceRecordLink(t *testing.T) {
	root := newSpecImportStore(t)
	commitFilesOnBranch(t, root, "design/plain-widget", map[string]string{
		".verdi/specs/active/plain-widget/spec.md": "---\n" +
			"id: spec/plain-widget\nkind: spec\nclass: feature\ntitle: \"Plain widget\"\nstatus: draft\nowners: [platform-team]\n" +
			"problem: { text: \"plain\", anchor: \"#problem\" }\noutcome: { text: \"plain\", anchor: \"#outcome\" }\n" +
			"acceptance_criteria:\n  - { id: ac-1, text: \"plain\", evidence: [attestation], anchor: \"#ac-1\" }\n---\n# Plain widget\n\n## Problem\n\n## Outcome\n\n## ac-1\n\nProse.\n",
	})
	c := newImportClient(t, NewHandlerWith(root, Deps{}))
	status, board := c.get("/board/spec/plain-widget")
	if status != http.StatusOK {
		t.Fatalf("GET board = %d", status)
	}
	if strings.Contains(board, `data-testid="asd-import-origin"`) {
		t.Fatal("a never-imported board must not carry a source-record link")
	}
}
