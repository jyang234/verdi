package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// indexDatesServedWant is what the dated store's served index must say
// under VERDI_NOW = 2024-06-15T12:00:00Z — every value a pinned literal,
// never re-derived from the fixture's own tables. quiet "" means the
// data-quiet carrier must be absent (only drafts-in-progress entries carry
// it).
var indexDatesServedWant = []struct {
	name, group, lastChange, quiet, why string
}{
	{"dated-landed", "active-components", "2024-04-26T12:00:00+00:00", "", "its landing commit (50 days), not main's later tip (20 days)"},
	{"dated-edited", "active-components", "2024-05-21T12:00:00+00:00", "", "the landed in-place edit (25 days), not its first landing (45 days)"},
	{"dated-desk-component", "drafts-in-progress", "2024-05-16T12:00:00+00:00", "true", "a status: draft component on the desk, 30 days old"},
	{"dated-draft-13", "drafts-in-progress", "2024-06-02T12:00:00+00:00", "false", "13 days: not quiet"},
	{"dated-draft-14", "drafts-in-progress", "2024-06-01T12:00:00+00:00", "false", "exactly 14 days: not quiet (the boundary is exclusive)"},
	{"dated-draft-15", "drafts-in-progress", "2024-05-31T12:00:00+00:00", "true", "15 days: quiet"},
}

var (
	servedEntryTagRe = regexp.MustCompile(`<li class="dir-entry[^"]*" data-testid="dir-entry-([a-z0-9-]+)"[^>]*>`)
	servedGroupRe    = regexp.MustCompile(`data-testid="dir-group-([a-z-]+)"`)
	servedAttrRe     = regexp.MustCompile(`([a-z-]+)="([^"]*)"`)
)

// servedDirEntry is one directory entry as the served page renders it: its
// group (the dir-group section it sits in) and its open tag's attributes.
type servedDirEntry struct {
	group string
	attrs map[string]string
}

// parseServedDirectory reads every directory entry off a served home page.
func parseServedDirectory(t *testing.T, page string) map[string]servedDirEntry {
	t.Helper()
	out := map[string]servedDirEntry{}
	for _, loc := range servedEntryTagRe.FindAllStringSubmatchIndex(page, -1) {
		tag := page[loc[0]:loc[1]]
		name := page[loc[2]:loc[3]]
		groups := servedGroupRe.FindAllStringSubmatch(page[:loc[0]], -1)
		if len(groups) == 0 {
			t.Fatalf("directory entry %s sits in no dir-group section", name)
		}
		attrs := map[string]string{}
		for _, m := range servedAttrRe.FindAllStringSubmatch(tag, -1) {
			attrs[m[1]] = m[2]
		}
		if _, dup := out[name]; dup {
			t.Fatalf("directory entry %s rendered twice", name)
		}
		out[name] = servedDirEntry{group: groups[len(groups)-1][1], attrs: attrs}
	}
	return out
}

// startIndexDatesThroughControl starts the dated store through the
// harness's own control mux — the registered GET /index-dates-fixture
// route, exactly as a Playwright file reaches it — and returns the fixture
// and its served base URL.
func startIndexDatesThroughControl(t *testing.T) (*indexDatesFixture, string) {
	t.Helper()
	neutralizeCIEnvForTest(t)
	ctrl := newControlServer(t.TempDir(), testModuleRoot, "")
	t.Cleanup(ctrl.indexDates.stop)
	srv := httptest.NewServer(ctrl.handler())
	t.Cleanup(srv.Close)
	status, body := httpGetBody(t, srv.URL+"/index-dates-fixture")
	if status != http.StatusOK {
		t.Fatalf("GET /index-dates-fixture = %d: %s", status, body)
	}
	url := strings.TrimSpace(body)
	if !strings.HasPrefix(url, "http://127.0.0.1:") || !strings.HasSuffix(url, "/") {
		t.Fatalf("fixture URL = %q, want a loopback base URL", url)
	}
	return ctrl.indexDates, url
}

// TestHarness_IndexDatesAndClock is spec/index-data ac-3's behavioral
// obligation, end to end: the harness provisions its dated store through
// its real provisioning path, starts the shipped binary's `verdi serve`
// over it with VERDI_NOW fixed (SI-296), and the SERVED index — the page
// itself, the one source — carries each entry's pinned last-change date
// and its quiet mark decided against that instant, with the fixed clock
// disclosed. Every fixture entry has a date, and nothing depends on the
// real date: a wall-clock read anywhere on this path flips the 13- and
// 14-day drafts to quiet.
func TestHarness_IndexDatesAndClock(t *testing.T) {
	f, url := startIndexDatesThroughControl(t)

	status, page := httpGetBody(t, url)
	if status != http.StatusOK {
		t.Fatalf("GET %s = %d", url, status)
	}
	served := parseServedDirectory(t, page)

	var gotNames, wantNames []string
	for name := range served {
		gotNames = append(gotNames, name)
	}
	for _, w := range indexDatesServedWant {
		wantNames = append(wantNames, w.name)
	}
	sort.Strings(gotNames)
	sort.Strings(wantNames)
	if strings.Join(gotNames, ",") != strings.Join(wantNames, ",") {
		t.Fatalf("served directory entries = %v, want exactly %v", gotNames, wantNames)
	}

	for _, w := range indexDatesServedWant {
		e := served[w.name]
		if e.group != w.group {
			t.Errorf("%s sits in group %q, want %q", w.name, e.group, w.group)
		}
		if got := e.attrs["data-last-change"]; got != w.lastChange {
			t.Errorf("%s data-last-change = %q, want %q (%s)", w.name, got, w.lastChange, w.why)
		}
		if reason, ok := e.attrs["data-date-unproven"]; ok {
			t.Errorf("%s carries data-date-unproven=%q, want every fixture entry dated", w.name, reason)
		}
		quiet, hasQuiet := e.attrs["data-quiet"]
		switch {
		case w.quiet == "" && hasQuiet:
			t.Errorf("%s carries data-quiet=%q, want none outside drafts-in-progress", w.name, quiet)
		case w.quiet != "" && quiet != w.quiet:
			t.Errorf("%s data-quiet = %q (present %v), want %q under VERDI_NOW (%s)", w.name, quiet, hasQuiet, w.quiet, w.why)
		}
	}

	status, disclosures := httpGetBody(t, url+"disclosures")
	if status != http.StatusOK {
		t.Fatalf("GET %sdisclosures = %d", url, status)
	}
	if !strings.Contains(disclosures, `data-disclosure-id="serve:fixed-clock"`) || !strings.Contains(disclosures, "2024-06-15T12:00:00Z") {
		t.Errorf("the served disclosures page does not disclose the fixed clock at 2024-06-15T12:00:00Z:\n%s", disclosures)
	}

	var nows []string
	for _, kv := range f.env {
		if strings.HasPrefix(kv, verdiNowEnvVar+"=") {
			nows = append(nows, kv)
		}
	}
	if len(nows) != 1 || nows[0] != "VERDI_NOW=2024-06-15T12:00:00Z" {
		t.Errorf("the dated serve's VERDI_NOW entries = %q, want exactly [VERDI_NOW=2024-06-15T12:00:00Z]", nows)
	}
}

// TestIndexDatesFixture_ChildEnvIsHermetic: an ambient VERDI_NOW (or any
// other serve injection seam, or a CI identity variable) never reaches the
// dated serve — its one VERDI_NOW is the fixture's own — and the other
// hermetic fixtures strip VERDI_NOW entirely.
func TestIndexDatesFixture_ChildEnvIsHermetic(t *testing.T) {
	t.Setenv(verdiNowEnvVar, "1999-01-01T00:00:00Z")
	t.Setenv("VERDI_OPENMR_FEED", "http://127.0.0.1:9/never-fetched")
	for _, kv := range hermeticServeEnv(os.Environ()) {
		if strings.HasPrefix(kv, verdiNowEnvVar+"=") || strings.HasPrefix(kv, "VERDI_OPENMR_FEED=") {
			t.Errorf("hermeticServeEnv kept %q", kv)
		}
	}
	env := indexDatesServeEnv(os.Environ())
	var nows []string
	for _, kv := range env {
		if strings.HasPrefix(kv, verdiNowEnvVar+"=") {
			nows = append(nows, kv)
		}
		if strings.HasPrefix(kv, "VERDI_OPENMR_FEED=") {
			t.Errorf("indexDatesServeEnv kept the ambient %q", kv)
		}
	}
	if len(nows) != 1 || nows[0] != "VERDI_NOW=2024-06-15T12:00:00Z" {
		t.Errorf("indexDatesServeEnv VERDI_NOW entries = %q, want exactly the fixture's own", nows)
	}
}

// TestProvisionIndexDatesStore pins the provisioned git facts the served
// dates rest on — main's tip is none of the default-branch entries'
// landing commits, the in-place edit is its spec's last landing, and each
// draft's tip carries its own age — through git itself, not refindex.
func TestProvisionIndexDatesStore(t *testing.T) {
	ctx := context.Background()
	root, err := provisionIndexDatesStore(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("provisionIndexDatesStore: %v", err)
	}
	committed := func(args ...string) string {
		t.Helper()
		out, err := gitOutput(ctx, root, append([]string{"log", "-1", "--format=%cI"}, args...)...)
		if err != nil {
			t.Fatalf("git log %v: %v", args, err)
		}
		return strings.Replace(out, "Z", "+00:00", 1)
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"main's tip: an unrelated later commit", []string{"main"}, "2024-05-26T12:00:00+00:00"},
		{"origin/HEAD resolves to main's tip", []string{"origin/HEAD"}, "2024-05-26T12:00:00+00:00"},
		{"dated-landed's last landing", []string{"main", "--", ".verdi/specs/active/dated-landed/spec.md"}, "2024-04-26T12:00:00+00:00"},
		{"dated-edited's in-place edit", []string{"main", "--", ".verdi/specs/active/dated-edited/spec.md"}, "2024-05-21T12:00:00+00:00"},
		{"dated-desk-component's landing", []string{"main", "--", ".verdi/specs/active/dated-desk-component/spec.md"}, "2024-05-16T12:00:00+00:00"},
		{"design/dated-draft-13's tip", []string{"design/dated-draft-13"}, "2024-06-02T12:00:00+00:00"},
		{"design/dated-draft-14's tip", []string{"design/dated-draft-14"}, "2024-06-01T12:00:00+00:00"},
		{"design/dated-draft-15's tip", []string{"design/dated-draft-15"}, "2024-05-31T12:00:00+00:00"},
	} {
		if got := committed(tc.args...); got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, got, tc.want)
		}
	}
	if branch, err := gitOutput(ctx, root, "rev-parse", "--abbrev-ref", "HEAD"); err != nil || branch != "main" {
		t.Errorf("checked-out branch = %q (%v), want main", branch, err)
	}
}

// TestProvisionIndexDatesStore_CancelledContext: provisioning honours ctx
// (main.go's interrupt path) and refuses with an error.
func TestProvisionIndexDatesStore_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provisionIndexDatesStore(ctx, t.TempDir()); err == nil {
		t.Fatal("provisionIndexDatesStore with a cancelled context: want an error, got nil")
	}
}

// TestIndexDatesFixture_Handler_MethodNotAllowed is the route's negative
// path: only GET starts or reports the fixture.
func TestIndexDatesFixture_Handler_MethodNotAllowed(t *testing.T) {
	f := newIndexDatesFixture(testModuleRoot)
	t.Cleanup(f.stop)
	rec := httptest.NewRecorder()
	f.handler(rec, httptest.NewRequest(http.MethodPost, "/index-dates-fixture", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /index-dates-fixture = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if f.url != "" {
		t.Fatalf("a refused request started the fixture at %s", f.url)
	}
}

// TestIndexDatesFixture_StopReapsAndRemovesStore: stop() ends the serve and
// removes the fixture's temporary store (B1-R5: no leaked stores), and is
// idempotent — safe when never started, safe twice.
func TestIndexDatesFixture_StopReapsAndRemovesStore(t *testing.T) {
	never := newIndexDatesFixture(testModuleRoot)
	never.stop()
	never.stop()

	f, url := startIndexDatesThroughControl(t)
	tmp := f.tmp
	if tmp == "" {
		t.Fatal("a started fixture recorded no temporary directory")
	}
	f.stop()
	f.stop()
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("fixture temp dir %s still exists after stop (stat err %v)", tmp, err)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	if resp, err := client.Get(url + "healthz"); err == nil {
		_ = resp.Body.Close()
		t.Errorf("the dated serve still answers at %s after stop", url)
	}
}
