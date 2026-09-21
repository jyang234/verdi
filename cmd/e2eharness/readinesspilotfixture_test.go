package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// TestSharedServeEnv pins the pure env composition both serves share: the
// ambient slice passes through untouched and in order, the three injection
// seams are appended last in their fixed order, and — unlike
// hermeticServeEnv — NOTHING is stripped: an ambient value for one of the
// seams stays in the slice, where exec's last-wins rule lets the appended
// fixture value override it.
func TestSharedServeEnv(t *testing.T) {
	store := sharedStore{
		storeRoot:            "/scratch/store",
		feedPath:             "/scratch/review-feed.json",
		verificationPath:     "/scratch/verification.json",
		readinessRequestPath: "/scratch/store/context-request.json",
	}
	ambient := []string{"PATH=/usr/bin", "VERDI_REVIEW_FEED=/ambient/feed.json", "HOME=/home/e2e"}
	ambientBefore := append([]string{}, ambient...)

	got := sharedServeEnv(ambient, store, "http://127.0.0.1:4177/openmrs")

	want := []string{
		"PATH=/usr/bin",
		"VERDI_REVIEW_FEED=/ambient/feed.json",
		"HOME=/home/e2e",
		"VERDI_REVIEW_FEED=/scratch/review-feed.json",
		"VERDI_OPENMR_FEED=http://127.0.0.1:4177/openmrs",
		"VERDI_DIAGRAM_VERIFICATION=/scratch/verification.json",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sharedServeEnv = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(ambient, ambientBefore) {
		t.Fatalf("ambient slice mutated: %q", ambient)
	}
	// Negative: an empty ambient still yields exactly the three seams.
	if got := sharedServeEnv(nil, store, "http://127.0.0.1:4177/openmrs"); !reflect.DeepEqual(got, want[3:]) {
		t.Fatalf("sharedServeEnv(nil) = %q, want %q", got, want[3:])
	}
}

// fakeReadinessPilotStart is a substitutable starter: it counts calls and
// answers with a canned serve or error.
type fakeReadinessPilotStart struct {
	calls int
	serve *readinessPilotServe
	err   error
}

func (s *fakeReadinessPilotStart) start(context.Context) (*readinessPilotServe, error) {
	s.calls++
	return s.serve, s.err
}

func getReadinessPilotFixture(t *testing.T, f *readinessPilotFixture, method string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/readiness-pilot-fixture", nil)
	rec := httptest.NewRecorder()
	f.handler(rec, req)
	return rec
}

// TestReadinessPilotFixture_Handler_LazyStartOnce pins the handler's
// contract against a fake starter: nothing starts until the first GET, the
// body is the started serve's base URL, and every later GET returns the
// same URL without starting again.
func TestReadinessPilotFixture_Handler_LazyStartOnce(t *testing.T) {
	fake := &fakeReadinessPilotStart{serve: &readinessPilotServe{url: "http://127.0.0.1:41999/"}}
	f := newReadinessPilotFixture(testModuleRoot, "http://127.0.0.1:4177/openmrs")
	f.start = fake.start
	if fake.calls != 0 {
		t.Fatalf("constructing the fixture started it (%d calls)", fake.calls)
	}

	first := getReadinessPilotFixture(t, f, http.MethodGet)
	if first.Code != http.StatusOK {
		t.Fatalf("first GET status = %d, want 200; body=%s", first.Code, first.Body.String())
	}
	if got := first.Body.String(); got != "http://127.0.0.1:41999/" {
		t.Fatalf("first GET body = %q, want the serve's base URL", got)
	}
	if ct := first.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/plain; charset=utf-8", ct)
	}
	second := getReadinessPilotFixture(t, f, http.MethodGet)
	if second.Code != http.StatusOK || second.Body.String() != first.Body.String() {
		t.Fatalf("second GET = %d %q, want 200 and the same URL %q", second.Code, second.Body.String(), first.Body.String())
	}
	if fake.calls != 1 {
		t.Fatalf("starter ran %d times across two GETs, want exactly once", fake.calls)
	}
}

// TestReadinessPilotFixture_Handler_Negative_StartFails: a failing start
// is a disclosed 500 naming the cause, caches nothing (the next GET
// retries), and a starter that returns no serve is refused the same way.
func TestReadinessPilotFixture_Handler_Negative_StartFails(t *testing.T) {
	fake := &fakeReadinessPilotStart{err: errors.New("provisioning the readiness-pilot fixture store: boom")}
	f := newReadinessPilotFixture(testModuleRoot, "")
	f.start = fake.start

	rec := getReadinessPilotFixture(t, f, http.MethodGet)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("body = %q, want the start error disclosed", rec.Body.String())
	}
	rec = getReadinessPilotFixture(t, f, http.MethodGet)
	if rec.Code != http.StatusInternalServerError || fake.calls != 2 {
		t.Fatalf("retry: status = %d, starter calls = %d; want 500 and a second attempt", rec.Code, fake.calls)
	}

	fake.err, fake.serve = nil, nil
	rec = getReadinessPilotFixture(t, f, http.MethodGet)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "no serve") {
		t.Fatalf("nil serve: status = %d body = %q, want 500 naming the missing serve", rec.Code, rec.Body.String())
	}
	f.stop() // never started: safe
}

// TestReadinessPilotFixture_ZeroValue_DefaultsToRealStart: a struct
// literal with no starter never nil-panics — ensureStarted defaults to the
// real sequence, which here discloses its provisioning failure (the
// module root carries no corpus) as a 500, and stop stays safe.
func TestReadinessPilotFixture_ZeroValue_DefaultsToRealStart(t *testing.T) {
	f := &readinessPilotFixture{moduleRoot: t.TempDir()}
	t.Cleanup(f.stop)
	rec := getReadinessPilotFixture(t, f, http.MethodGet)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "provisioning the readiness-pilot fixture store") {
		t.Fatalf("body = %q, want the real starter's provisioning failure named", rec.Body.String())
	}
	if f.start == nil {
		t.Fatal("ensureStarted left start unset")
	}
	var zero readinessPilotFixture
	zero.stop() // never started, no starter: safe
}

// TestReadinessPilotFixture_Handler_Negative_WrongMethod: a non-GET
// request is refused before any start.
func TestReadinessPilotFixture_Handler_Negative_WrongMethod(t *testing.T) {
	fake := &fakeReadinessPilotStart{serve: &readinessPilotServe{url: "http://127.0.0.1:41999/"}}
	f := newReadinessPilotFixture(testModuleRoot, "")
	f.start = fake.start
	rec := getReadinessPilotFixture(t, f, http.MethodPost)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if fake.calls != 0 {
		t.Fatalf("a refused POST started the fixture (%d calls)", fake.calls)
	}
}

// TestReadinessPilotFixture_Stop pins stop's contract on a fake serve:
// it cancels the child's context, waits for its exit, and is idempotent.
func TestReadinessPilotFixture_Stop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	cancelled := 0
	serve := &readinessPilotServe{
		url:    "http://127.0.0.1:41999/",
		cancel: func() { cancelled++; cancel(); done <- nil },
		done:   done,
	}
	f := newReadinessPilotFixture(testModuleRoot, "")
	f.start = (&fakeReadinessPilotStart{serve: serve}).start
	if rec := getReadinessPilotFixture(t, f, http.MethodGet); rec.Code != http.StatusOK {
		t.Fatalf("GET = %d, want 200", rec.Code)
	}

	f.stop()
	if cancelled != 1 {
		t.Fatalf("stop cancelled %d times, want 1", cancelled)
	}
	if ctx.Err() == nil {
		t.Fatal("stop did not cancel the child's context")
	}
	f.stop()
	if cancelled != 1 {
		t.Fatalf("second stop cancelled again (%d), want idempotent", cancelled)
	}
}

// TestControlServer_WiresReadinessPilotFixture proves the endpoint is
// mounted on the control server's own mux (no subprocess: the method
// guard answers first).
func TestControlServer_WiresReadinessPilotFixture(t *testing.T) {
	c := newControlServer(t.TempDir(), testModuleRoot, "http://127.0.0.1:4177/openmrs")
	t.Cleanup(c.readinessPilot.stop)
	req := httptest.NewRequest(http.MethodPost, "/readiness-pilot-fixture", nil)
	rec := httptest.NewRecorder()
	c.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /readiness-pilot-fixture = %d, want 405 (route mounted)", rec.Code)
	}
	if c.readinessPilot.openMRFeedURL != "http://127.0.0.1:4177/openmrs" {
		t.Fatalf("openMRFeedURL = %q, want the control server's /openmrs", c.readinessPilot.openMRFeedURL)
	}
}

var concernIDRe = regexp.MustCompile(`data-concern-id="([^"]+)"`)

// TestReadinessPilotFixture_Handler_Happy is the real witness through the
// SHIPPED binary: the handler provisions the shared-shape store into its
// own scratch, builds and starts a real `verdi serve --context-request`
// under the shared serve's exact environment, and the cockpit it serves
// is the PRISTINE derivation the browser suite's oracles describe — the
// target title, the unclaimed oq-1 first, and no board-question concern
// (the residue earlier browser suites leave in the shared store).
func TestReadinessPilotFixture_Handler_Happy(t *testing.T) {
	neutralizeCIEnvForTest(t)
	f := newReadinessPilotFixture(absModuleRoot(t), "http://127.0.0.1:9/openmrs")
	t.Cleanup(f.stop)

	rec := getReadinessPilotFixture(t, f, http.MethodGet)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	base := strings.TrimSpace(rec.Body.String())
	if !regexp.MustCompile(`^http://127\.0\.0\.1:\d+/$`).MatchString(base) {
		t.Fatalf("body = %q, want a loopback base URL", base)
	}

	// The exact env handed to exec names THIS store's own artifacts and
	// the feed URL the fixture was given — the shared serve's posture.
	f.mu.Lock()
	serve := f.serve
	f.mu.Unlock()
	wantTail := []string{
		"VERDI_REVIEW_FEED=" + serve.store.feedPath,
		"VERDI_OPENMR_FEED=http://127.0.0.1:9/openmrs",
		"VERDI_DIAGRAM_VERIFICATION=" + serve.store.verificationPath,
	}
	if len(serve.env) < 3 || !reflect.DeepEqual(serve.env[len(serve.env)-3:], wantTail) {
		t.Fatalf("child env tail = %q, want %q", serve.env[max(0, len(serve.env)-3):], wantTail)
	}
	if !strings.HasPrefix(serve.store.feedPath, "/") || strings.HasPrefix(serve.store.storeRoot, absModuleRoot(t)) {
		t.Fatalf("fixture store %q / feed %q are not in an isolated scratch", serve.store.storeRoot, serve.store.feedPath)
	}

	status, page := httpGetBody(t, base+"readiness")
	if status != http.StatusOK {
		t.Fatalf("GET readiness status = %d, want 200", status)
	}
	if !strings.Contains(page, "Refinancing decline flow") {
		t.Errorf("isolated cockpit does not name the target title")
	}
	var ids []string
	for _, m := range concernIDRe.FindAllStringSubmatch(page, -1) {
		ids = append(ids, m[1])
	}
	if len(ids) < 3 || ids[0] != "shape/question/oq-1" || ids[1] != "shape/provenance" || ids[2] != "shape/question/oq-2" {
		t.Errorf("focus head = %q, want [shape/question/oq-1 shape/provenance shape/question/oq-2 ...]", ids)
	}
	for _, id := range ids {
		if strings.HasPrefix(id, "shape/board/") || id == "shape/question/oq-3" {
			t.Errorf("pristine derivation carries shared-store residue %q", id)
		}
	}
}
