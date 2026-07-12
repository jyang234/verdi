package workbench

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/disclosure"
)

// disclosuresFixtureStore writes a minimal bare store (no mutable zone)
// with one new-class spec, so the lint engine's VL-017 disclosed-unproven
// notice is a live, real disclosure for the enumeration to find — the
// same fixture shape internal/disclosureview's own tests pin.
func disclosuresFixtureStore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	specDir := filepath.Join(root, ".verdi", "specs", "active", "panel-fixture")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := `---
id: spec/panel-fixture
kind: spec
title: "Panel Fixture"
owners: [platform-team]
class: story
status: draft
story: jira:FIX-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a", evidence: [behavioral], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
---
# Panel Fixture

## Problem

p

## Outcome

o

## Ac 1

a
`
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "schema: verdi.layout/v1\nforge: gitlab\nproviders:\n  jira:\n    base_url: https://example.atlassian.net\n    rollup_field: customfield_00000\nservices:\n  discovery: flowmap\n"
	if err := os.WriteFile(filepath.Join(root, ".verdi", "verdi.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDisclosuresHandler_RendersCheckoutDisclosures(t *testing.T) {
	root := disclosuresFixtureStore(t)
	h := NewHandlerWith(root, Deps{
		Disclosures: []disclosure.Disclosure{
			disclosure.New("mcp:review-feed", "", `forge "gitlab" is configured (verdi.yaml) but no credentials are available to reach it; review state cannot be shown`),
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/disclosures", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	// The live lint disclosure (VL-017 on the bare fixture store) and the
	// process-context extra both enumerate through the one view.
	for name, want := range map[string]string{
		"lint disclosure source":  `<code class="disclosure-source">lint:VL-017</code>`,
		"lint disclosure scope":   `.verdi/specs/active/panel-fixture/spec.md`,
		"process-context extra":   `<code class="disclosure-source">mcp:review-feed</code>`,
		"severity badge":          `<span class="disclosure-severity">disclosed-unproven</span>`,
		"stable id (ac-2)":        `data-disclosure-id="lint:VL-017/.verdi/specs/active/panel-fixture/spec.md"`,
		"page title":              "Disclosures",
		"freshness note":          "never persisted",
		"shared view (ac-3 seam)": `<section class="disclosures-view"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("disclosures page missing %s: want substring %q", name, want)
		}
	}
}

// TestDisclosuresHandler_FreshPerRender proves ac-1's "computed fresh per
// render": the same handler, asked twice, reflects a checkout-state change
// (the mutable zone appearing) with no restart and no cache.
func TestDisclosuresHandler_FreshPerRender(t *testing.T) {
	root := disclosuresFixtureStore(t)
	h := NewHandler(root)

	get := func() string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/disclosures", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}

	if body := get(); !strings.Contains(body, "lint:VL-017") {
		t.Fatalf("first render: want the bare-clone VL-017 disclosure present:\n%s", body)
	}
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "data", "mutable"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := get()
	if strings.Contains(body, "lint:VL-017") {
		t.Fatalf("second render is stale: VL-017 still shown after the mutable zone appeared:\n%s", body)
	}
	// With no extras and no lint disclosures left, the empty state is an
	// explicit positive claim, never a blank page.
	if !strings.Contains(body, "No current disclosures.") {
		t.Fatalf("empty enumeration must render the positive empty-state claim:\n%s", body)
	}
}

// TestIndexLinksDisclosures: the landing page points at the view — the
// operator's surface must be discoverable, not tribal knowledge.
func TestIndexLinksDisclosures(t *testing.T) {
	h := NewHandler(disclosuresFixtureStore(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `href="/disclosures"`) {
		t.Fatal("home page does not link the disclosures view")
	}
}

func TestDisclosuresHandler_MethodNotAllowed(t *testing.T) {
	h := NewHandler(disclosuresFixtureStore(t))
	req := httptest.NewRequest(http.MethodPost, "/disclosures", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 for POST /disclosures", rec.Code)
	}
}

func TestDisclosuresHandler_OperationalErrorIsAnErrorPage(t *testing.T) {
	// A root whose .verdi cannot be walked is an operational failure: the
	// page must say so (500), never render a vacuous empty view (that
	// would be a silent pass — constitution 2).
	root := t.TempDir() // no .verdi at all
	h := NewHandler(root)
	req := httptest.NewRequest(http.MethodGet, "/disclosures", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 for an unenumerable store; body: %s", rec.Code, rec.Body.String())
	}
}
