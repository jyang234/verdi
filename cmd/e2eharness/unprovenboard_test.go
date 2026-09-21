package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The module root relative to this package directory — the same literal
// provision_vocab_test.go hands newVocabFixture, so the committed fixture
// under cmd/e2eharness/testdata/ and the binary build resolve exactly as
// they do in the running harness (main.go's os.Getwd at the module root).
const testModuleRoot = "../.."

// neutralizeCIEnvForTest mirrors main.go's process-level neutralizeCIEnv
// for one test: "no CI_DEFAULT_BRANCH" is half the fixture's premise, so a
// runner exporting one must not dissolve the case under test. The serve
// subprocess inherits the neutralized values.
func neutralizeCIEnvForTest(t *testing.T) {
	t.Helper()
	for _, key := range ciEnvVars {
		t.Setenv(key, "")
	}
}

// readOnlyPanelOf slices the rendered read-only rail panel out of a page,
// or returns "" when the page carries none.
func readOnlyPanelOf(page string) string {
	start := strings.Index(page, `data-testid="readonly-panel"`)
	if start < 0 {
		return ""
	}
	end := strings.Index(page[start:], "</section>")
	if end < 0 {
		return ""
	}
	return page[start : start+end]
}

// startUnprovenFixture starts the fixture (building the binary, spawning
// its serve) and returns its URL, reaping the subprocess at test end.
func startUnprovenFixture(t *testing.T) (*unprovenBoardFixture, string) {
	t.Helper()
	f := newUnprovenBoardFixture(testModuleRoot)
	t.Cleanup(f.stop)
	req := httptest.NewRequest(http.MethodGet, "/unproven-board-fixture", nil)
	rec := httptest.NewRecorder()
	f.handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	url := strings.TrimSpace(rec.Body.String())
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("url = %q, want a loopback URL", url)
	}
	return f, url
}

func httpGetBody(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", url, err)
	}
	return resp.StatusCode, string(body)
}

func httpPostJSON(t *testing.T, url string, payload any) (int, string) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", url, err)
	}
	return resp.StatusCode, string(body)
}

// TestUnprovenBoardFixture_Handler_Happy is MVP release amendment R2's
// browser-fixture witness through the SHIPPED binary: the handler builds
// and starts a real `verdi serve` over a real no-remote store (its spec
// the committed testdata fixture), and the board it serves is read-only
// with its lifecycle stamped UNPROVEN — the honest witness and the local
// remedy disclosed, never the sealed record, never an editing affordance.
// The mutation boundary is then proven against the wired design bridge:
// both tiers refuse with the read-only 403 and its diagnostic, and the
// store is byte-for-byte untouched afterwards.
func TestUnprovenBoardFixture_Handler_Happy(t *testing.T) {
	neutralizeCIEnvForTest(t)
	f, url := startUnprovenFixture(t)
	boardURL := url + "board/spec/" + unprovenBoardSpecName

	status, page := httpGetBody(t, boardURL)
	if status != http.StatusOK {
		t.Fatalf("GET board status = %d, want 200", status)
	}
	for _, want := range []string{
		`data-board-mode="readonly"`,
		`data-readonly-reason="unproven"`,
		"read-only · lifecycle unproven",
		"displayed bytes: unproven",
		`data-testid="readonly-panel"`,
		"cannot claim acceptance or sealing",
		"default branch could not be resolved",
		"git remote set-head origin",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("isolated unproven board missing %q", want)
		}
	}
	for _, banned := range []string{
		"read-only · sealed record",
		"This spec is accepted",
		`sealed-panel`,
		`id="add-sticky-btn"`,
		`id="commit-push-btn"`,
		`data-testid="asd-forms"`,
		"set CI_DEFAULT_BRANCH",
	} {
		if strings.Contains(page, banned) {
			t.Errorf("isolated unproven board must not contain %q", banned)
		}
	}
	// The panel's own remedy is the local one — the configured default
	// branch fetched and origin/HEAD pointed at it — never a CI-environment
	// workaround (adopted R0 forbids one for local adoption).
	panel := readOnlyPanelOf(page)
	if !strings.Contains(panel, "git remote set-head origin") {
		t.Errorf("read-only panel missing the origin/HEAD remedy: %q", panel)
	}
	if strings.Contains(panel, "CI_DEFAULT_BRANCH") {
		t.Errorf("read-only panel offers a CI-environment workaround: %q", panel)
	}

	// The no-mutation witness: the store before...
	specPath := filepath.Join(f.root, ".verdi", "specs", "active", unprovenBoardSpecName, "spec.md")
	before, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	porcelainBefore, err := gitOutput(context.Background(), f.root, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}

	// ...a typed write through the wired bridge: the adapter's read-only
	// gate answers 403 with its diagnostic (never the unwired 500)...
	status, snap := httpGetBody(t, boardURL+"/snapshot")
	if status != http.StatusOK {
		t.Fatalf("GET snapshot status = %d, want 200: %s", status, snap)
	}
	var snapshot struct {
		BaseDigest  string `json:"base_digest"`
		BaseSpecB64 string `json:"base_spec_b64"`
	}
	if err := json.Unmarshal([]byte(snap), &snapshot); err != nil {
		t.Fatalf("decoding snapshot: %v\n%s", err, snap)
	}
	status, body := httpPostJSON(t, boardURL+"/api/mutate_draft", map[string]any{
		"request": map[string]any{
			"schema":        "verdi.draftmutation/v1",
			"spec":          "spec/" + unprovenBoardSpecName,
			"base_digest":   snapshot.BaseDigest,
			"base_spec_b64": snapshot.BaseSpecB64,
			// Syntactically valid placeholders: the read-only gate refuses
			// before any identity check.
			"expected":   map[string]string{"checkout": "/never-reached", "branch": "main", "head": strings.Repeat("0", 40)},
			"operations": []map[string]string{{"op": "set-problem", "text": "x", "anchor": "#problem"}},
		},
	})
	if status != http.StatusForbidden {
		t.Errorf("mutate_draft on the unproven wall = %d, want 403; body=%s", status, body)
	}
	if !strings.Contains(body, "readonly mode") {
		t.Errorf("mutate_draft refusal lacks its read-only diagnostic: %s", body)
	}
	// ...the scratch tier refuses the same way...
	status, body = httpPostJSON(t, boardURL+"/api/sticky", map[string]string{"text": "unproven wall", "type": "comment"})
	if status != http.StatusForbidden {
		t.Errorf("sticky on the unproven wall = %d, want 403; body=%s", status, body)
	}
	if !strings.Contains(body, "readonly mode") {
		t.Errorf("sticky refusal lacks its read-only diagnostic: %s", body)
	}
	// ...and nothing changed.
	after, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("refused writes changed the spec bytes")
	}
	porcelainAfter, err := gitOutput(context.Background(), f.root, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if porcelainAfter != porcelainBefore {
		t.Errorf("refused writes changed the store's status: before %q, after %q", porcelainBefore, porcelainAfter)
	}
}

// TestUnprovenBoardFixture_ChildEnvIsHermetic: ambient sentinel values in
// every serve injection seam and CI identity variable never reach the
// child — proven on the helper's output, on the exact environment slice
// the started fixture handed to exec, and behaviorally: with an ambient
// CI_DEFAULT_BRANCH=main (which WOULD resolve this store's local main and
// dissolve the unproven case if inherited) the served board still renders
// unproven. No network: the sentinels are stripped, so nothing is fetched
// even though one names a feed URL.
func TestUnprovenBoardFixture_ChildEnvIsHermetic(t *testing.T) {
	sentinels := map[string]string{
		"VERDI_REVIEW_FEED":                   "/never/read/review-feed.json",
		"VERDI_OPENMR_FEED":                   "http://127.0.0.1:9/never-fetched",
		"VERDI_DIAGRAM_VERIFICATION":          "/never/read/verification.json",
		"CI":                                  "true",
		"GITHUB_ACTIONS":                      "true",
		"GITHUB_BASE_REF":                     "main",
		"CI_DEFAULT_BRANCH":                   "main",
		"CI_MERGE_REQUEST_TARGET_BRANCH_NAME": "main",
	}
	for key, value := range sentinels {
		t.Setenv(key, value)
	}
	assertNoSentinel := func(label string, env []string) {
		t.Helper()
		if len(env) == 0 {
			t.Fatalf("%s is empty; the child needs PATH, HOME, and friends", label)
		}
		for _, kv := range env {
			key, value, _ := strings.Cut(kv, "=")
			if want, ok := sentinels[key]; ok {
				t.Errorf("%s carries %s=%q (ambient sentinel %q reached the child)", label, key, value, want)
			}
		}
	}
	// The pure helper, over the real ambient environment.
	assertNoSentinel("hermeticServeEnv(os.Environ())", hermeticServeEnv(os.Environ()))
	if path := os.Getenv("PATH"); path != "" {
		found := false
		for _, kv := range hermeticServeEnv(os.Environ()) {
			if kv == "PATH="+path {
				found = true
			}
		}
		if !found {
			t.Errorf("hermeticServeEnv dropped PATH; the child could not find git")
		}
	}

	// A real start: the slice handed to exec, and the child's behavior.
	f, url := startUnprovenFixture(t)
	assertNoSentinel("started fixture's child env", f.env)
	status, page := httpGetBody(t, url+"board/spec/"+unprovenBoardSpecName)
	if status != http.StatusOK {
		t.Fatalf("GET board status = %d, want 200", status)
	}
	if !strings.Contains(page, `data-readonly-reason="unproven"`) {
		t.Errorf("ambient CI_DEFAULT_BRANCH reached the child: the board no longer renders unproven")
	}
	if strings.Contains(page, "never-fetched") || strings.Contains(page, "never/read") {
		t.Errorf("an ambient feed sentinel surfaced in the served board")
	}
}

// TestUnprovenBoardFixture_Handler_Idempotent proves repeated calls return
// the SAME URL rather than starting a second serve each time.
func TestUnprovenBoardFixture_Handler_Idempotent(t *testing.T) {
	neutralizeCIEnvForTest(t)
	f, first := startUnprovenFixture(t)

	req := httptest.NewRequest(http.MethodGet, "/unproven-board-fixture", nil)
	rec := httptest.NewRecorder()
	f.handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if second := strings.TrimSpace(rec.Body.String()); second != first {
		t.Fatalf("url changed across calls: %q then %q, want the same instance reused", first, second)
	}
}

// TestUnprovenBoardFixture_Handler_Negative_WrongMethod is the endpoint's
// negative path: a non-GET request is refused, never silently accepted.
func TestUnprovenBoardFixture_Handler_Negative_WrongMethod(t *testing.T) {
	f := newUnprovenBoardFixture(testModuleRoot)
	req := httptest.NewRequest(http.MethodPost, "/unproven-board-fixture", nil)
	rec := httptest.NewRecorder()
	f.handler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// TestUnprovenBoardFixture_Handler_Negative_MissingFixture: a module root
// that carries no committed fixture is a disclosed 500 before any build or
// serve, never a silently empty store dressed as the unproven wall.
func TestUnprovenBoardFixture_Handler_Negative_MissingFixture(t *testing.T) {
	neutralizeCIEnvForTest(t)
	f := newUnprovenBoardFixture(t.TempDir())
	t.Cleanup(f.stop)
	req := httptest.NewRequest(http.MethodGet, "/unproven-board-fixture", nil)
	rec := httptest.NewRecorder()
	f.handler(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unproven-board fixture spec") {
		t.Fatalf("body = %q, want the missing-fixture reason named", rec.Body.String())
	}
}

// TestControlServer_WiresUnprovenBoardFixture proves the endpoint is
// actually mounted on the control server's own mux, with the module root
// the running harness passes.
func TestControlServer_WiresUnprovenBoardFixture(t *testing.T) {
	neutralizeCIEnvForTest(t)
	c := newControlServer(t.TempDir(), testModuleRoot, "")
	t.Cleanup(c.unprovenBoard.stop)
	req := httptest.NewRequest(http.MethodGet, "/unproven-board-fixture", nil)
	rec := httptest.NewRecorder()
	c.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "http://127.0.0.1:") {
		t.Fatalf("body = %q, want a loopback URL", rec.Body.String())
	}
}
