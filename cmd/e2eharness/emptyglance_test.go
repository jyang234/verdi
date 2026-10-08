package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestEmptyGlanceFixture_Handler_Happy is spec/index-v2 ac-1's empty
// state (the kept home-status-glance ac-3/co-1 property, parent dc-12)
// proven directly through the REAL pipeline (Controller adjudication
// ADJ-40, 2026-07-16): the handler starts a genuinely separate, hermetic
// workbench instance backed by a REAL minimal store on disk — git init +
// .verdi/verdi.yaml, ZERO specs — computed by the real
// refindex.ComputeIndex, not a canned index. An empty store carries no
// entries, so all FOUR columns render empty at once (heading, zero count,
// the explicit empty state), the strongest witness of ac-1 through the
// true pipe.
func TestEmptyGlanceFixture_Handler_Happy(t *testing.T) {
	f := newEmptyGlanceFixture()

	req := httptest.NewRequest(http.MethodGet, "/empty-glance-fixture", nil)
	rec := httptest.NewRecorder()
	f.handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	url := strings.TrimSpace(rec.Body.String())
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("url = %q, want a loopback URL", url)
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	page := string(body)

	if !strings.Contains(page, `class="home-directory"`) {
		t.Fatalf("isolated render missing the directory's columns entirely; got: %s", page)
	}
	// The real empty store has zero specs, so nothing is ever badged or
	// linked: not a single card may render (the populated-column contrast
	// is proven by the shared-store e2e, not here).
	if strings.Contains(page, `data-testid="dir-entry-`) {
		t.Fatalf("empty store rendered a card, want none through the real pipeline; got: %s", page)
	}
	// All four columns render their heading, zero count, and explicit
	// empty state — proven through refindex.ComputeIndex's real
	// default-branch walk, not a canned index (co-1; ADJ-40).
	for _, group := range []string{"drafts-in-progress", "accepted-pending-build", "active-components", "terminal"} {
		start := strings.Index(page, `data-testid="dir-group-`+group+`"`)
		if start < 0 {
			t.Fatalf("isolated render missing the %s column heading; got: %s", group, page)
		}
		end := strings.Index(page[start:], "</section>")
		if end < 0 {
			t.Fatalf("could not find the %s column's closing tag; got: %s", group, page)
		}
		column := page[start : start+end]
		if !strings.Contains(column, `<span class="count">0</span>`) {
			t.Fatalf("%s column missing its zero count; got: %s", group, column)
		}
		if !strings.Contains(column, `<p class="empty dir-empty">`) {
			t.Fatalf("%s column missing its explicit empty state; got: %s", group, column)
		}
	}
}

// TestEmptyGlanceFixture_Handler_Idempotent proves repeated calls return
// the SAME URL rather than starting a second listener each time.
func TestEmptyGlanceFixture_Handler_Idempotent(t *testing.T) {
	f := newEmptyGlanceFixture()

	get := func() string {
		req := httptest.NewRequest(http.MethodGet, "/empty-glance-fixture", nil)
		rec := httptest.NewRecorder()
		f.handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		return strings.TrimSpace(rec.Body.String())
	}

	first := get()
	second := get()
	if first != second {
		t.Fatalf("url changed across calls: %q then %q, want the same instance reused", first, second)
	}
}

// TestEmptyGlanceFixture_Handler_Negative_WrongMethod is the endpoint's
// negative path: a non-GET request is refused, never silently accepted.
func TestEmptyGlanceFixture_Handler_Negative_WrongMethod(t *testing.T) {
	f := newEmptyGlanceFixture()
	req := httptest.NewRequest(http.MethodPost, "/empty-glance-fixture", nil)
	rec := httptest.NewRecorder()
	f.handler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// TestControlServer_WiresEmptyGlanceFixture proves the endpoint is
// actually mounted on the control server's own mux, not merely present as
// an unwired method.
func TestControlServer_WiresEmptyGlanceFixture(t *testing.T) {
	c := newControlServer(t.TempDir(), t.TempDir(), "")
	req := httptest.NewRequest(http.MethodGet, "/empty-glance-fixture", nil)
	rec := httptest.NewRecorder()
	c.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "http://127.0.0.1:") {
		t.Fatalf("body = %q, want a loopback URL", rec.Body.String())
	}
}
