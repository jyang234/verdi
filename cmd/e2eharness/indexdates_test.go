package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// indexDatesComponentSpec is the fixture's one default-branch entry: an
// "active" component, never eligible for quiet (only drafts-in-progress
// entries are).
const indexDatesComponentSpec = `---
id: spec/index-dates-component
kind: spec
class: component
title: "Index dates component (e2e fixture)"
status: active
owners: [platform-team]
---
# Index dates component
`

// indexDatesDraftSpec is the fixture's one design-branch entry: a draft,
// unconditionally drafts-in-progress and so eligible for quiet.
const indexDatesDraftSpec = `---
id: spec/index-dates-draft
kind: spec
class: component
title: "Index dates draft (e2e fixture)"
status: draft
owners: [platform-team]
---
# Index dates draft
`

const (
	// indexDatesMainCommitDate/indexDatesDraftCommitDate are commitAt's
	// pinned dates (git's "<unix-seconds> <tz-offset>" form) — two KNOWN,
	// DIFFERENT dates, never the wall clock at provisioning time.
	indexDatesMainCommitDate  = "1701388800 +0000" // 2023-12-01T00:00:00Z
	indexDatesDraftCommitDate = "1699920000 +0000" // 2023-11-14T00:00:00Z
	// The dates gitx.CommitDate's normalization renders for the two commits
	// above — this package never touches internal/gitx directly, but proves
	// GET /index-dates reports exactly these through refindex's real seam.
	indexDatesMainWantDate  = "2023-12-01T00:00:00+00:00"
	indexDatesDraftWantDate = "2023-11-14T00:00:00+00:00"
)

// provisionIndexDatesStore builds a REAL, minimal, hermetic store — one
// default-branch component and one design-branch draft, each committed at
// its own known, different date via commitAt (git.go) — mirroring
// provisionEmptyStore's bare-origin setup (emptyglance.go) so
// gitx.DefaultBranch genuinely resolves "main" rather than degrading to the
// no-default-branch path.
func provisionIndexDatesStore(ctx context.Context) (root string, err error) {
	tmp, err := os.MkdirTemp("", "verdi-e2e-index-dates-*")
	if err != nil {
		return "", err
	}
	root = filepath.Join(tmp, "store")
	originDir := filepath.Join(tmp, "origin.git")

	if err := os.MkdirAll(filepath.Join(root, ".verdi", "specs", "active", "index-dates-component"), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "verdi.yaml"), []byte("store:\n  name: index-dates-fixture\n"), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "specs", "active", "index-dates-component", "spec.md"), []byte(indexDatesComponentSpec), 0o644); err != nil {
		return "", err
	}

	if err := runGit(ctx, root, nil, "init", "--quiet", "--initial-branch=main"); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "add", "-A"); err != nil {
		return "", err
	}
	if err := commitAt(ctx, root, indexDatesMainCommitDate, "commit", "--quiet", "--no-verify", "-m", "index-dates fixture: default-branch component"); err != nil {
		return "", err
	}

	// A bare local origin whose HEAD names main (emptyglance.go's own
	// load-bearing reasoning: gitx.DefaultBranch keys off
	// refs/remotes/origin/HEAD, so without this the walk degrades to the
	// no-default-branch path instead of genuinely resolving "main").
	if err := runGit(ctx, "", nil, "init", "--bare", "--quiet", "--initial-branch=main", originDir); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "remote", "add", "origin", originDir); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "push", "--quiet", "--set-upstream", "origin", "main"); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "remote", "set-head", "origin", "main"); err != nil {
		return "", err
	}

	if err := runGit(ctx, root, nil, "checkout", "--quiet", "-b", "design/index-dates-draft"); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "specs", "active", "index-dates-draft"), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "specs", "active", "index-dates-draft", "spec.md"), []byte(indexDatesDraftSpec), 0o644); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "add", "-A"); err != nil {
		return "", err
	}
	if err := commitAt(ctx, root, indexDatesDraftCommitDate, "commit", "--quiet", "--no-verify", "-m", "index-dates fixture: aged design-branch draft"); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "checkout", "--quiet", "main"); err != nil {
		return "", err
	}

	return root, nil
}

// newTestControlServer builds a bare controlServer wired ONLY with
// storeRoot and a fresh harnessClock — deliberately bypassing
// newControlServer's other fixtures (vocab, unprovenBoard, specImport,
// readinessPilot, objSupersede), which need a real moduleRoot and spawn
// their own subprocesses on first use. Nothing this test exercises reaches
// them; constructing the struct literal directly keeps this test hermetic
// and fast (co-2: no network, no subprocess).
func newTestControlServer(storeRoot string) *controlServer {
	return &controlServer{storeRoot: storeRoot, clock: &harnessClock{}}
}

func indexDatesTestMux(ctrl *controlServer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/clock", ctrl.clockHandler)
	mux.HandleFunc("/index-dates", ctrl.indexDatesHandler)
	return mux
}

// TestHarness_IndexDatesAndClock is spec/index-data ac-3's behavioral
// obligation: the harness provisions a store whose entries carry known,
// pinned commit dates (never the wall clock at provisioning time), and its
// GET /index-dates control endpoint reports exactly those dates plus a
// quiet verdict that flips as POST /clock's "now" changes — never as a
// function of when this test itself happens to run.
func TestHarness_IndexDatesAndClock(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	root, err := provisionIndexDatesStore(t.Context())
	if err != nil {
		t.Fatalf("provisionIndexDatesStore: %v", err)
	}

	ctrl := newTestControlServer(root)
	srv := httptest.NewServer(indexDatesTestMux(ctrl))
	defer srv.Close()

	setClock := func(t *testing.T, body string) *http.Response {
		t.Helper()
		resp, err := http.Post(srv.URL+"/clock", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST /clock: %v", err)
		}
		return resp
	}
	setClockNow := func(t *testing.T, now time.Time) {
		t.Helper()
		body, err := json.Marshal(clockRequestBody{Now: now.UTC().Format(time.RFC3339)})
		if err != nil {
			t.Fatalf("marshaling clock body: %v", err)
		}
		resp := setClock(t, string(body))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("POST /clock status = %d, want %d", resp.StatusCode, http.StatusNoContent)
		}
	}

	fetchDates := func(t *testing.T) []indexDateEntry {
		t.Helper()
		resp, err := http.Get(srv.URL + "/index-dates")
		if err != nil {
			t.Fatalf("GET /index-dates: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /index-dates status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		var out []indexDateEntry
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decoding /index-dates response: %v", err)
		}
		return out
	}

	byRef := func(t *testing.T, entries []indexDateEntry, ref string) indexDateEntry {
		t.Helper()
		for _, e := range entries {
			if e.Ref == ref {
				return e
			}
		}
		t.Fatalf("no entry for %q among %+v", ref, entries)
		return indexDateEntry{}
	}

	// now = the draft's own commit date + 6 days: not yet quiet.
	setClockNow(t, time.Date(2023, 11, 20, 0, 0, 0, 0, time.UTC))
	entries := fetchDates(t)

	comp := byRef(t, entries, "spec/index-dates-component")
	if comp.Date != indexDatesMainWantDate {
		t.Fatalf("component Date = %q, want %q (its own pinned commit date)", comp.Date, indexDatesMainWantDate)
	}
	if comp.Quiet {
		t.Fatal("component Quiet = true, want false (only drafts-in-progress entries can ever read quiet)")
	}
	if comp.DateDisclosed != "" {
		t.Fatalf("component DateDisclosed = %q, want empty (a real, pinned commit date was read)", comp.DateDisclosed)
	}

	draft := byRef(t, entries, "spec/index-dates-draft")
	if draft.Date != indexDatesDraftWantDate {
		t.Fatalf("draft Date = %q, want %q (its own pinned commit date)", draft.Date, indexDatesDraftWantDate)
	}
	if draft.Quiet {
		t.Fatal("draft Quiet = true at now = commit date + 6 days, want false (the threshold is fourteen days)")
	}

	// Advance the clock to the draft's commit date + 21 days: now quiet —
	// the served verdict tracks the harness clock, never the real wall
	// clock at test-run time (the obligation's own falsifier).
	setClockNow(t, time.Date(2023, 12, 5, 0, 0, 0, 0, time.UTC))
	entries = fetchDates(t)
	draft = byRef(t, entries, "spec/index-dates-draft")
	if !draft.Quiet {
		t.Fatal("draft Quiet = false at now = commit date + 21 days, want true")
	}
	// The component never reads quiet regardless of the clock.
	comp = byRef(t, entries, "spec/index-dates-component")
	if comp.Quiet {
		t.Fatal("component Quiet = true after advancing the clock, want false")
	}

	// Clearing the override (empty JSON body) returns to the real wall
	// clock.
	before := time.Now()
	clearResp := setClock(t, "{}")
	defer clearResp.Body.Close()
	if clearResp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /clock (clear) status = %d, want %d", clearResp.StatusCode, http.StatusNoContent)
	}
	getResp, err := http.Get(srv.URL + "/clock")
	if err != nil {
		t.Fatalf("GET /clock: %v", err)
	}
	defer getResp.Body.Close()
	var clockBody struct {
		Now string `json:"now"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&clockBody); err != nil {
		t.Fatalf("decoding /clock response: %v", err)
	}
	got, err := time.Parse(time.RFC3339, clockBody.Now)
	if err != nil {
		t.Fatalf("parsing /clock now %q: %v", clockBody.Now, err)
	}
	after := time.Now()
	if got.Before(before.Add(-time.Minute)) || got.After(after.Add(time.Minute)) {
		t.Fatalf("cleared clock's now = %v, want close to the real wall clock (between %v and %v)", got, before, after)
	}
}

// TestClockHandler_InvalidJSON_BadRequest is the clock endpoint's negative
// path: a malformed body is refused, never silently ignored.
func TestClockHandler_InvalidJSON_BadRequest(t *testing.T) {
	ctrl := newTestControlServer(t.TempDir())
	srv := httptest.NewServer(indexDatesTestMux(ctrl))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/clock", "application/json", strings.NewReader("not json"))
	if err != nil {
		t.Fatalf("POST /clock: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestClockHandler_InvalidTime_BadRequest is a second negative path: a
// syntactically valid JSON body whose "now" is not RFC3339 is refused.
func TestClockHandler_InvalidTime_BadRequest(t *testing.T) {
	ctrl := newTestControlServer(t.TempDir())
	srv := httptest.NewServer(indexDatesTestMux(ctrl))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/clock", "application/json", strings.NewReader(`{"now":"not-a-time"}`))
	if err != nil {
		t.Fatalf("POST /clock: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestClockHandler_MethodNotAllowed proves an unsupported method is
// refused rather than silently handled.
func TestClockHandler_MethodNotAllowed(t *testing.T) {
	ctrl := newTestControlServer(t.TempDir())
	srv := httptest.NewServer(indexDatesTestMux(ctrl))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/clock", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /clock: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

// TestIndexDatesHandler_MethodNotAllowed is /index-dates's own method
// guard.
func TestIndexDatesHandler_MethodNotAllowed(t *testing.T) {
	ctrl := newTestControlServer(t.TempDir())
	srv := httptest.NewServer(indexDatesTestMux(ctrl))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/index-dates", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /index-dates: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

// TestIndexDatesHandler_ComputeIndexError_InternalServerError is
// /index-dates's negative path over a broken store: ComputeIndex's error
// surfaces as 500, never a fabricated empty index.
func TestIndexDatesHandler_ComputeIndexError_InternalServerError(t *testing.T) {
	ctrl := newTestControlServer(filepath.Join(t.TempDir(), "does-not-exist"))
	srv := httptest.NewServer(indexDatesTestMux(ctrl))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/index-dates")
	if err != nil {
		t.Fatalf("GET /index-dates: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}
