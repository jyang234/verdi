package workbench

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

func TestCorpusHandler_Happy(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	req := httptest.NewRequest(http.MethodGet, "/a/spec/stale-decline", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Stale decline handling (fixture)") {
		t.Errorf("missing title, got: %s", body)
	}
	if !strings.Contains(body, "platform-team") {
		t.Errorf("missing owners in frontmatter card, got: %s", body)
	}
	if !strings.Contains(body, "jira:LOAN-1482") {
		t.Errorf("missing story in frontmatter card, got: %s", body)
	}
	if !strings.Contains(body, "routed through the outbox pattern") {
		t.Errorf("missing rendered body, got: %s", body)
	}
	if !strings.Contains(body, `class="dispositions-table"`) {
		t.Errorf("missing I-5 dispositions table, got: %s", body)
	}
	if !strings.Contains(body, "incorporated") || !strings.Contains(body, "contradicted") || !strings.Contains(body, "open-question") {
		t.Errorf("dispositions table missing expected values, got: %s", body)
	}
	// Links panel: this spec's own `implements: adr/0002-outbox-events` link.
	if !strings.Contains(body, "implements") || !strings.Contains(body, "adr/0002-outbox-events") {
		t.Errorf("missing outgoing link, got: %s", body)
	}
}

// TestCorpusHandler_Backlink proves the backlinks half of the
// links/backlinks panel: adr/0002-outbox-events is `implements`-linked BY
// spec/stale-decline, so its own corpus page must show the computed
// inverse (implemented-by).
func TestCorpusHandler_Backlink(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	req := httptest.NewRequest(http.MethodGet, "/a/adr/0002-outbox-events", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "implemented-by") {
		t.Errorf("missing computed backlink, got: %s", body)
	}
	if !strings.Contains(body, "spec/stale-decline") {
		t.Errorf("backlink does not name the source spec, got: %s", body)
	}
}

// TestCorpusHandler_ServesArchivedSpec pins the corpus route's zone-agnostic
// contract: GET /a/spec/<name> serves a spec resolving under specs/archive/
// exactly as it serves an active one, because index.Build's walk indexes
// both zones (artifact.ClassifyPath) and this handler reads the indexed
// entry's own path. This is the servable-surface guarantee ADJ-39
// (2026-07-16) relies on — the archived-match family card links here rather
// than to the board route, which serves the active zone alone and 404s. A
// regression that stranded the archive zone from the corpus route would
// silently re-break every archived family link; this test fails first.
func TestCorpusHandler_ServesArchivedSpec(t *testing.T) {
	const archivedSpec = `---
id: spec/corpus-archived-fixture
kind: spec
class: story
title: "Corpus archived fixture"
status: closed
owners: [platform-team]
story: jira:CORP-1
problem: { text: "an archived spec still needs a legible read surface", anchor: "#problem" }
outcome: { text: "the corpus page serves it from the archive zone", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/corpus-archived-parent#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "served from archive", evidence: [static], anchor: "#ac-1" }
frozen: { at: 2024-01-01, commit: cccccccccccccccccccccccccccccccccccccccc }
---
# Corpus archived fixture

## Problem

## Outcome

## ac-1

Archived, yet readable on the corpus page.
`
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/archive/corpus-archived-fixture/spec.md": archivedSpec,
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed an archived spec",
	}})
	h := NewHandler(repo.Dir)

	req := httptest.NewRequest(http.MethodGet, "/a/spec/corpus-archived-fixture", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /a/spec/corpus-archived-fixture = %d, want 200 (corpus route is zone-agnostic)\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Corpus archived fixture") {
		t.Errorf("archived spec title not rendered, got: %s", body)
	}
	if !strings.Contains(body, "closed") {
		t.Errorf("archived spec status not rendered, got: %s", body)
	}
}

func TestCorpusHandler_Negative(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	t.Run("unknown artifact 404s", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/a/spec/does-not-exist", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("wrong method rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/a/spec/stale-decline", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", rec.Code)
		}
	})

	t.Run("bad store root", func(t *testing.T) {
		h := NewHandler(t.TempDir())
		req := httptest.NewRequest(http.MethodGet, "/a/spec/anything", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code < 400 {
			t.Fatalf("status = %d, want a 4xx/5xx for a root with no .verdi/ at all", rec.Code)
		}
	})
}
