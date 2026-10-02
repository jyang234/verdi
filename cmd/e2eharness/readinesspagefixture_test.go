package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/workbench"
)

// TestReadinessPageSnapshots_Derived pins every readiness-page fixture to
// the real derivation: each is readinesspilot.Derive's own output over its
// input, valid under the snapshot contract, and in the posture the page's
// tests rely on — the current step, the human-review rows of the
// solo-author fixture, and the three states of the three-states fixture.
func TestReadinessPageSnapshots_Derived(t *testing.T) {
	snaps, err := readinessPageSnapshots(absModuleRoot(t))
	if err != nil {
		t.Fatalf("readinessPageSnapshots: %v", err)
	}
	want := map[string]readinesspilot.AreaID{
		readinessPageStep2:       readinesspilot.AreaSuccess,
		readinessPageStep3:       readinesspilot.AreaContext,
		readinessPageStep4:       readinesspilot.AreaReview,
		readinessPageSoloAuthor:  readinesspilot.AreaReview,
		readinessPageThreeStates: readinesspilot.AreaShape,
	}
	if len(snaps) != len(want) {
		t.Fatalf("fixture snapshots = %d, want %d", len(snaps), len(want))
	}
	for name, focus := range want {
		snap, ok := snaps["spec/"+name]
		if !ok {
			t.Fatalf("no fixture snapshot for spec/%s", name)
		}
		if err := snap.Validate(); err != nil {
			t.Fatalf("spec/%s violates the readiness contract: %v", name, err)
		}
		if snap.CurrentFocus != focus {
			t.Errorf("spec/%s current focus = %q, want %q", name, snap.CurrentFocus, focus)
		}
		if snap.TargetRef != "spec/"+name || snap.BoardPath != workbench.BranchBoardHref("design/"+name, name) {
			t.Errorf("spec/%s identity = %q board %q", name, snap.TargetRef, snap.BoardPath)
		}
		for _, c := range snap.AllConcerns {
			if (c.State == readinesspilot.StateProven) == (c.Guidance != "") {
				t.Errorf("spec/%s concern %q state %q guidance %q: guidance must be present exactly when unresolved", name, c.ID, c.State, c.Guidance)
			}
		}
		// A step before Get approval has concerns waiting on it in a later
		// step, so the page's waiting disclosure has something to expand.
		later := 0
		for _, c := range snap.Attention {
			if c.Area != focus {
				later++
			}
		}
		if (name == readinessPageStep2 || name == readinessPageStep3) && later == 0 {
			t.Errorf("spec/%s has no concern in a later step; the page's waiting disclosure would be empty", name)
		}
	}

	byID := func(name, id string) readinesspilot.Concern {
		t.Helper()
		for _, c := range snaps["spec/"+name].AllConcerns {
			if c.ID == id {
				return c
			}
		}
		t.Fatalf("spec/%s has no concern %q", name, id)
		return readinesspilot.Concern{}
	}

	// Step 2 carries the coverage row (no stub lists ac-3) and the
	// violated current success blocker that makes it the current step.
	if c := byID(readinessPageStep2, "success/coverage/ac-3"); c.State != readinesspilot.StateUnproven || c.Object != "ac-3" {
		t.Errorf("step 2 coverage row = %+v", c)
	}
	if c := byID(readinessPageStep2, "success/blocker/obligation-quality/ac-2/runtime"); c.State != readinesspilot.StateViolated || !c.Blocking {
		t.Errorf("step 2 success blocker = %+v", c)
	}

	// The solo-author fixture: the author is the only principal, so the
	// broader-role duty (countersign on close) stays visible, human
	// review, and unsatisfied; the author's own vouch is judgmental work.
	for _, id := range []string{
		"review/role/close/attestation/countersign",
		"review/blocker/obligation-countersign-unproven/close/attestation/countersign",
		"review/blocker/principal-resolution-unproven/merge",
	} {
		c := byID(readinessPageSoloAuthor, id)
		if !c.HumanReview() || c.State == readinesspilot.StateProven {
			t.Errorf("solo-author %q = %+v, want unsatisfied human review", id, c)
		}
	}
	if c := byID(readinessPageSoloAuthor, "review/blocker/obligation-author-vouch-unproven/merge/attestation/author-vouch"); c.HumanReview() {
		t.Errorf("solo-author author vouch reads as human review: %+v", c)
	}
	if c := byID(readinessPageSoloAuthor, "context/disclosure/solo-principal-collapse"); c.State != readinesspilot.StateUnproven {
		t.Errorf("solo-author collapse disclosure = %+v, want disclosed unproven", c)
	}

	// The three-states fixture holds a proven, a violated-with-witness, and
	// a disclosed-unproven item.
	if c := byID(readinessPageThreeStates, "shape/problem"); c.State != readinesspilot.StateProven {
		t.Errorf("three-states shape/problem = %q", c.State)
	}
	if c := byID(readinessPageThreeStates, "shape/outcome"); c.State != readinesspilot.StateViolated || len(c.Witnesses) == 0 {
		t.Errorf("three-states shape/outcome = %+v", c)
	}
	if c := byID(readinessPageThreeStates, "shape/question/oq-1"); c.State != readinesspilot.StateUnproven || len(c.Witnesses) == 0 {
		t.Errorf("three-states shape/question/oq-1 = %+v", c)
	}
}

// TestReadinessPageSnapshots_MissingSources: a module root without the
// journey and policy-conflict fixtures is an operational error, never an
// empty fixture set.
func TestReadinessPageSnapshots_MissingSources(t *testing.T) {
	if snaps, err := readinessPageSnapshots(t.TempDir()); err == nil {
		t.Fatalf("readinessPageSnapshots(empty root) = %d snapshots, want an error", len(snaps))
	}
}

// TestReadinessPageFixture_Handler_Happy proves the isolated server: GET
// returns a loopback URL whose /readiness?spec=<name> page is the REAL
// workbench render of each fixture snapshot, and an unknown spec is the
// route's own 503 disclosure.
func TestReadinessPageFixture_Handler_Happy(t *testing.T) {
	f := newReadinessPageFixture(absModuleRoot(t))
	rec := httptest.NewRecorder()
	f.handler(rec, httptest.NewRequest(http.MethodGet, "/readiness-page-fixture", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	url := strings.TrimSpace(rec.Body.String())
	if !strings.HasPrefix(url, "http://127.0.0.1:") || !strings.HasSuffix(url, "/") {
		t.Fatalf("url = %q, want a loopback base URL", url)
	}
	get := func(path string) (int, string) {
		t.Helper()
		resp, err := http.Get(url + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, string(body)
	}
	for name, id := range map[string]string{
		readinessPageStep2:       "success/coverage/ac-3",
		readinessPageStep3:       "context/verdict",
		readinessPageStep4:       "review/blocker/forge-facts-unavailable/close",
		readinessPageSoloAuthor:  "review/role/close/attestation/countersign",
		readinessPageThreeStates: "shape/outcome",
	} {
		code, page := get("readiness?spec=" + name)
		if code != http.StatusOK {
			t.Fatalf("spec %s status = %d, want 200: %s", name, code, page)
		}
		if !strings.Contains(page, `data-concern-id="`+id+`"`) {
			t.Fatalf("spec %s page lacks concern %q", name, id)
		}
	}
	if code, _ := get("readiness?spec=readiness-page-absent"); code != http.StatusServiceUnavailable {
		t.Fatalf("unknown fixture spec status = %d, want 503", code)
	}
	if code, _ := get("readiness"); code != http.StatusServiceUnavailable {
		t.Fatalf("no ?spec= status = %d, want the no-spec 503 (the fixture names no default)", code)
	}
}

// TestReadinessPageFixture_Handler_Reuse proves start-once reuse.
func TestReadinessPageFixture_Handler_Reuse(t *testing.T) {
	f := newReadinessPageFixture(absModuleRoot(t))
	urls := make([]string, 2)
	for i := range urls {
		rec := httptest.NewRecorder()
		f.handler(rec, httptest.NewRequest(http.MethodGet, "/readiness-page-fixture", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("call %d status = %d, want 200", i, rec.Code)
		}
		urls[i] = strings.TrimSpace(rec.Body.String())
	}
	if urls[0] != urls[1] {
		t.Fatalf("fixture URL changed across calls: %q then %q", urls[0], urls[1])
	}
}

// TestReadinessPageFixture_Handler_Negative: wrong methods are 405, and a
// module root without the source fixtures is a 500 that names the reason
// and caches nothing.
func TestReadinessPageFixture_Handler_Negative(t *testing.T) {
	f := newReadinessPageFixture(absModuleRoot(t))
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		f.handler(rec, httptest.NewRequest(method, "/readiness-page-fixture", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s status = %d, want 405", method, rec.Code)
		}
	}
	broken := newReadinessPageFixture(t.TempDir())
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		broken.handler(rec, httptest.NewRequest(http.MethodGet, "/readiness-page-fixture", nil))
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "readiness-page fixture") {
			t.Fatalf("call %d over an empty module root = %d %q, want a named 500", i, rec.Code, rec.Body.String())
		}
	}
}

// TestReadinessPageFixture_ControlWiring proves the control server routes
// the endpoint.
func TestReadinessPageFixture_ControlWiring(t *testing.T) {
	ctrl := newControlServer(t.TempDir(), absModuleRoot(t), "")
	srv := httptest.NewServer(ctrl.handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/readiness-page-fixture")
	if err != nil {
		t.Fatalf("GET control endpoint: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(strings.TrimSpace(string(body)), "http://127.0.0.1:") {
		t.Fatalf("control endpoint = %d %q, want a loopback URL", resp.StatusCode, body)
	}
}
