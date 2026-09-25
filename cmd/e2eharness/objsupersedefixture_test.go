package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
)

// fakeObjSupersedeStart is a substitutable starter: it counts calls and
// answers with a canned set of serves or an error, the same shape as the
// other subprocess fixtures' fake starters.
type fakeObjSupersedeStart struct {
	calls  int
	serves map[string]*objSupersedeServe
	err    error
}

func (s *fakeObjSupersedeStart) start(context.Context) (map[string]*objSupersedeServe, error) {
	s.calls++
	return s.serves, s.err
}

func getObjSupersedeFixture(t *testing.T, f *objSupersedeFixture, method string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/objsupersede-fixture", nil)
	rec := httptest.NewRecorder()
	f.handler(rec, req)
	return rec
}

func canonicalObjSupersedeServes() map[string]*objSupersedeServe {
	return map[string]*objSupersedeServe{
		"accepted": {info: objSupersedeStoreInfo{Scenario: "accepted", URL: "http://127.0.0.1:41001/", Checkout: "main"}},
	}
}

// TestObjSupersedeFixture_Handler_LazyStartOnce pins the handler's
// contract against a fake starter: nothing starts until the first GET,
// the body is the started stores' JSON, and every later GET returns the
// same info without starting again.
func TestObjSupersedeFixture_Handler_LazyStartOnce(t *testing.T) {
	fake := &fakeObjSupersedeStart{serves: canonicalObjSupersedeServes()}
	f := newObjSupersedeFixture(testModuleRoot)
	f.start = fake.start
	if fake.calls != 0 {
		t.Fatalf("constructing the fixture started it (%d calls)", fake.calls)
	}

	first := getObjSupersedeFixture(t, f, http.MethodGet)
	if first.Code != http.StatusOK {
		t.Fatalf("first GET status = %d, want 200; body=%s", first.Code, first.Body.String())
	}
	if ct := first.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var info objSupersedeFixtureInfo
	if err := json.Unmarshal(first.Body.Bytes(), &info); err != nil {
		t.Fatalf("decoding body: %v: %s", err, first.Body.String())
	}
	if len(info.Stores) != 1 || info.Stores["accepted"].URL != "http://127.0.0.1:41001/" {
		t.Fatalf("info = %+v, want one store named accepted with the canned URL", info)
	}

	second := getObjSupersedeFixture(t, f, http.MethodGet)
	if second.Code != http.StatusOK || second.Body.String() != first.Body.String() {
		t.Fatalf("second GET = %d %q, want 200 and the same body %q", second.Code, second.Body.String(), first.Body.String())
	}
	if fake.calls != 1 {
		t.Fatalf("starter ran %d times across two GETs, want exactly once", fake.calls)
	}
}

// TestObjSupersedeFixture_Handler_Negative_StartFails: a failing start is
// a disclosed 500 naming the cause, caches nothing (the next GET
// retries), and a starter that returns no stores is refused the same way.
func TestObjSupersedeFixture_Handler_Negative_StartFails(t *testing.T) {
	fake := &fakeObjSupersedeStart{err: errors.New("materializing objsupersede scenario \"accepted\": boom")}
	f := newObjSupersedeFixture(testModuleRoot)
	f.start = fake.start

	rec := getObjSupersedeFixture(t, f, http.MethodGet)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("body = %q, want the start error disclosed", rec.Body.String())
	}
	rec = getObjSupersedeFixture(t, f, http.MethodGet)
	if rec.Code != http.StatusInternalServerError || fake.calls != 2 {
		t.Fatalf("retry: status = %d, starter calls = %d; want 500 and a second attempt", rec.Code, fake.calls)
	}

	fake.err, fake.serves = nil, nil
	rec = getObjSupersedeFixture(t, f, http.MethodGet)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "no stores") {
		t.Fatalf("nil serves: status = %d body = %q, want 500 naming the missing stores", rec.Code, rec.Body.String())
	}
	f.stop() // never started: safe
}

// TestObjSupersedeFixture_ZeroValue_DefaultsToRealStart: a struct literal
// with no starter never nil-panics — ensureStarted defaults to the real
// sequence, which here discloses its own build/materialize failure as a
// 500 (the module root carries no corpus), and stop stays safe.
func TestObjSupersedeFixture_ZeroValue_DefaultsToRealStart(t *testing.T) {
	f := &objSupersedeFixture{moduleRoot: t.TempDir()}
	t.Cleanup(f.stop)
	rec := getObjSupersedeFixture(t, f, http.MethodGet)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	if f.start == nil {
		t.Fatal("ensureStarted left start unset")
	}
	var zero objSupersedeFixture
	zero.stop() // never started, no starter: safe
}

// TestObjSupersedeFixture_Handler_Negative_WrongMethod: a non-GET request
// is refused before any start.
func TestObjSupersedeFixture_Handler_Negative_WrongMethod(t *testing.T) {
	fake := &fakeObjSupersedeStart{serves: canonicalObjSupersedeServes()}
	f := newObjSupersedeFixture(testModuleRoot)
	f.start = fake.start
	rec := getObjSupersedeFixture(t, f, http.MethodPost)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if fake.calls != 0 {
		t.Fatalf("a refused POST started the fixture (%d calls)", fake.calls)
	}
}

// TestObjSupersedeFixture_Stop pins stop's contract on fake serves: it
// cancels every store's context, waits for each exit, and is idempotent.
func TestObjSupersedeFixture_Stop(t *testing.T) {
	cancelled := map[string]int{}
	doneA, doneB := make(chan error, 1), make(chan error, 1)
	serves := map[string]*objSupersedeServe{
		"accepted": {info: objSupersedeStoreInfo{Scenario: "accepted"}, cancel: func() { cancelled["accepted"]++; doneA <- nil }, done: doneA},
		"chain":    {info: objSupersedeStoreInfo{Scenario: "chain"}, cancel: func() { cancelled["chain"]++; doneB <- nil }, done: doneB},
	}
	fake := &fakeObjSupersedeStart{serves: serves}
	f := newObjSupersedeFixture(testModuleRoot)
	f.start = fake.start
	if rec := getObjSupersedeFixture(t, f, http.MethodGet); rec.Code != http.StatusOK {
		t.Fatalf("GET = %d, want 200", rec.Code)
	}

	f.stop()
	if cancelled["accepted"] != 1 || cancelled["chain"] != 1 {
		t.Fatalf("cancelled = %+v, want each store cancelled exactly once", cancelled)
	}
	f.stop()
	if cancelled["accepted"] != 1 || cancelled["chain"] != 1 {
		t.Fatalf("second stop cancelled again: %+v, want idempotent", cancelled)
	}
}

// TestControlServer_WiresObjSupersedeFixture proves the endpoint is
// mounted on the control server's own mux (no subprocess: the method
// guard answers first).
func TestControlServer_WiresObjSupersedeFixture(t *testing.T) {
	c := newControlServer(t.TempDir(), testModuleRoot, "")
	t.Cleanup(c.objSupersede.stop)
	req := httptest.NewRequest(http.MethodPost, "/objsupersede-fixture", nil)
	rec := httptest.NewRecorder()
	c.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /objsupersede-fixture = %d, want 405 (route mounted)", rec.Code)
	}
}

// objSupersedeExpected pins, per store, the facts objSupersedeStores'
// startAll must report — independent of the production maps above, so a
// typo in either one is caught.
var objSupersedeExpected = map[string]objSupersedeStoreInfo{
	"accepted": {
		Scenario: "accepted", Checkout: "main", MainBranch: "main",
		Successor: "spec/successor", SupersededDecision: objSupersedeClosedDecision, SupersededCriterion: objSupersedeClosedCriterion,
	},
	"chain": {
		Scenario: "chain", Checkout: "main", MainBranch: "main",
		Successor: "spec/successor-v3", SupersededDecision: objSupersedeClosedDecision, SupersededCriterion: objSupersedeClosedCriterion,
	},
	"chain-drop": {
		Scenario: "chain-drop", Checkout: "main", MainBranch: "main",
		Successor: "spec/successor-v2", SupersededDecision: objSupersedeClosedDecision, SupersededCriterion: objSupersedeClosedCriterion,
	},
	"proposed": {
		Scenario: "proposed", Checkout: "design/successor", MainBranch: "main", DesignBranch: "design/successor",
		Successor: "spec/successor", SupersededDecision: objSupersedeClosedDecision, SupersededCriterion: objSupersedeClosedCriterion,
	},
	"no-conflict": {
		Scenario: "no-conflict", Checkout: "design/successor", MainBranch: "main", DesignBranch: "design/successor",
		Successor: "spec/successor", SupersededDecision: objSupersedeClosedDecision, SupersededCriterion: objSupersedeClosedCriterion,
	},
	"chain-not-in-force": {
		Scenario: "chain-not-in-force", Checkout: "design/successor-v2", MainBranch: "main", DesignBranch: "design/successor-v2",
		Successor: "spec/successor-v2", SupersededDecision: objSupersedeClosedDecision, SupersededCriterion: objSupersedeClosedCriterion,
	},
}

// branchBoardPath mirrors internal/workbench's BranchBoardHref: the
// branch rides one path segment with its slashes percent-encoded.
func branchBoardPath(branch, slug string) string {
	return "b/" + url.PathEscape(branch) + "/board/spec/" + slug
}

// TestObjSupersedeFixture_Handler_Happy is the real witness through the
// SHIPPED binary: the handler materializes all six scenario stores
// (scenario.Materialize) and starts a real `verdi serve` subprocess over
// each, and every store's reported facts and board routes are reachable —
// the checked-out branch's own board, and (for proposed/no-conflict) the
// default branch's board showing nothing yet for the not-yet-accepted
// successor, and (for accepted/chain/chain-drop) the original proposing
// design branch's board still reachable through the per-branch route
// after its merge.
func TestObjSupersedeFixture_Handler_Happy(t *testing.T) {
	neutralizeCIEnvForTest(t)
	f := newObjSupersedeFixture(absModuleRoot(t))
	t.Cleanup(f.stop)

	rec := getObjSupersedeFixture(t, f, http.MethodGet)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var info objSupersedeFixtureInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatalf("decoding body: %v: %s", err, rec.Body.String())
	}
	if len(info.Stores) != len(objSupersedeStores) {
		t.Fatalf("got %d stores, want %d: %+v", len(info.Stores), len(objSupersedeStores), info.Stores)
	}
	for _, name := range objSupersedeStores {
		got, ok := info.Stores[name]
		if !ok {
			t.Fatalf("missing store %q in %+v", name, info.Stores)
		}
		want := objSupersedeExpected[name]
		want.URL = got.URL // URL is dynamic (ephemeral port); compared separately below
		if !reflect.DeepEqual(got, want) {
			t.Errorf("store %q = %+v, want %+v", name, got, want)
		}
		if !strings.HasPrefix(got.URL, "http://127.0.0.1:") {
			t.Errorf("store %q URL = %q, want a loopback URL", name, got.URL)
		}
	}

	// Each store's own checked-out branch renders its successor's board.
	titles := map[string]string{
		"spec/successor":    "Successor",
		"spec/successor-v2": "Successor v2",
		"spec/successor-v3": "Successor v3",
	}
	for _, name := range objSupersedeStores {
		s := info.Stores[name]
		slug := strings.TrimPrefix(s.Successor, "spec/")
		status, page := httpGetBody(t, s.URL+"board/spec/"+slug)
		if status != http.StatusOK {
			t.Errorf("store %q: GET board/spec/%s = %d, want 200", name, slug, status)
			continue
		}
		if want := titles[s.Successor]; want != "" && !strings.Contains(page, want) {
			t.Errorf("store %q: board/spec/%s missing title %q", name, slug, want)
		}
	}

	// proposed and no-conflict: the default branch shows nothing yet for
	// the not-yet-accepted successor (design §8's "not yet accepted" case).
	for _, name := range []string{"proposed", "no-conflict"} {
		s := info.Stores[name]
		status, _ := httpGetBody(t, s.URL+branchBoardPath(s.MainBranch, "successor"))
		if status == http.StatusOK {
			t.Errorf("store %q: /b/%s/board/spec/successor = 200, want the default branch to show nothing pre-acceptance", name, s.MainBranch)
		}
	}

	// accepted, chain, and chain-drop: the original proposing design
	// branch (design/successor) still resolves and its board is still
	// reachable through the per-branch route after the merge — the same
	// mechanism proposed/no-conflict rely on for their default-branch view,
	// proven here on the branch that actually merged.
	for _, name := range []string{"accepted", "chain", "chain-drop"} {
		s := info.Stores[name]
		status, page := httpGetBody(t, s.URL+branchBoardPath("design/successor", "successor"))
		if status != http.StatusOK {
			t.Errorf("store %q: /b/design%%2Fsuccessor/board/spec/successor = %d, want 200", name, status)
			continue
		}
		if !strings.Contains(page, "Successor") {
			t.Errorf("store %q: per-branch board missing the spec title", name)
		}
	}
}

// TestObjSupersedeStores_MaterializeMatchesScenarioBuildAndLintsClean is
// the Contract's SHA-reproduction and lint-clean proof: for every needed
// store, an INDEPENDENT scenario.Materialize into its own directory
// reproduces exactly the root and step SHAs scenario.Build (the Go test
// suite's own helper) produces, and every branch scenario.Materialize
// left in that repository lints clean in-process — the same call
// `verdi lint` itself makes (cmd/verdi/lint.go). The L3c report's re-review
// a found all six of these variants lint-clean; none is an intended
// refusal, so no branch here is exempted.
func TestObjSupersedeStores_MaterializeMatchesScenarioBuildAndLintsClean(t *testing.T) {
	ctx := context.Background()
	fixtureDir := scenario.Dir()
	for _, name := range objSupersedeStores {
		t.Run(name, func(t *testing.T) {
			want := scenario.Build(t, name)

			got, err := scenario.Materialize(ctx, fixtureDir, t.TempDir(), name)
			if err != nil {
				t.Fatalf("Materialize(%q): %v", name, err)
			}
			if !reflect.DeepEqual(got.Base, want.Base) {
				t.Errorf("store %q: Base = %v, want %v", name, got.Base, want.Base)
			}
			if !reflect.DeepEqual(got.Steps, want.Steps) {
				t.Errorf("store %q: Steps = %v, want %v", name, got.Steps, want.Steps)
			}
			if len(got.Base) == 0 {
				t.Fatalf("store %q: no base commits", name)
			}
			t.Logf("store %q: root = %s", name, got.Base[0])

			branches, err := gitOutput(ctx, got.Dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
			if err != nil {
				t.Fatalf("listing branches: %v", err)
			}
			if branches == "" {
				t.Fatalf("store %q: no local branches", name)
			}
			for _, branch := range strings.Split(branches, "\n") {
				branch = strings.TrimSpace(branch)
				if branch == "" {
					continue
				}
				if err := runGit(ctx, got.Dir, nil, "checkout", "-q", branch); err != nil {
					t.Fatalf("checking out %s: %v", branch, err)
				}
				findings, err := lint.NewEngine().Run(ctx, got.Dir, lint.BuildContext(ctx, got.Dir), lint.Options{})
				if err != nil {
					t.Fatalf("store %q branch %s: lint.Run: %v", name, branch, err)
				}
				for _, f := range findings {
					if f.Severity != lint.SeverityDisclosure {
						t.Errorf("store %q branch %s: unexpected lint finding: %s", name, branch, f.String())
					}
				}
			}
		})
	}
}
