package workbench

// spec/illustrative-class on the workbench's own surfaces: the corpus
// artifact page and the board reference peek both route diagram bodies
// through internal/render's shared seam, so a non-proposal diagram
// inherits the illustrative badged figure and a class: proposal diagram
// is never painted with it (ac-2's negative case) — proven here against
// the same handlers the browser drives, with no second render path.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// diagramTierFixtureIllustrative is an incumbent diagram — no class, so
// illustrative BY CLASS (spec/illustrative-class dc-2).
const diagramTierFixtureIllustrative = `---
id: diagram/refi-flow
kind: diagram
title: "Refi flow sketch (fixture)"
status: active
owners: [platform-team]
---
graph TD
  refi --> decline
`

// diagramTierFixtureProposal is a class: proposal diagram — the tier is
// the extractor's to compute, never the illustrative badge (ac-2).
const diagramTierFixtureProposal = `---
id: diagram/refi-flow-future
kind: diagram
class: proposal
title: "Refi flow future state (fixture)"
status: proposed
owners: [platform-team]
---
graph TD
  refi --> audit
`

func newDiagramTierFixture(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + boardFixtureName + "/spec.md": boardFixtureSpec,
			".verdi/diagrams/refi-flow.mermaid":                    diagramTierFixtureIllustrative,
			".verdi/diagrams/refi-flow-future.mermaid":             diagramTierFixtureProposal,
			".verdi/.gitignore":                                    "data/\n",
		},
		Message: "seed diagram tier fixture",
	}})
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "design/"+boardFixtureName); err != nil {
		t.Fatalf("checkout design branch: %v", err)
	}
	return repo.Dir
}

const (
	illustrativeMarker = `data-diagram-tier="illustrative"`
	illustrativeChip   = "illustrative · not deterministically verifiable"
)

func TestCorpusPage_DiagramTier(t *testing.T) {
	root := newDiagramTierFixture(t)
	h := NewHandler(root)
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	t.Run("a non-proposal diagram page wears the illustrative badge", func(t *testing.T) {
		rec := get("/a/diagram/refi-flow")
		if rec.Code != http.StatusOK {
			t.Fatalf("corpus page = %d\n%s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		for _, want := range []string{illustrativeMarker, illustrativeChip, `<pre class="mermaid">`} {
			if !strings.Contains(body, want) {
				t.Errorf("corpus diagram page missing %q\n%s", want, body)
			}
		}
	})

	t.Run("NEGATIVE: a proposal diagram page is never painted illustrative", func(t *testing.T) {
		rec := get("/a/diagram/refi-flow-future")
		if rec.Code != http.StatusOK {
			t.Fatalf("corpus page = %d\n%s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if strings.Contains(body, "illustrative") {
			t.Errorf("proposal page painted with the illustrative badge\n%s", body)
		}
		if !strings.Contains(body, `data-diagram-tier="full"`) {
			t.Errorf("proposal page missing the extractor-computed tier marker\n%s", body)
		}
	})
}

func TestBoardPeek_DiagramTier(t *testing.T) {
	root := newDiagramTierFixture(t)
	h := NewHandler(root)
	peek := func(ref string) string {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/"+boardFixtureName+"/peek?ref="+ref, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("peek %s = %d\n%s", ref, rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}

	t.Run("peeking a non-proposal diagram carries the illustrative badge", func(t *testing.T) {
		body := peek("diagram/refi-flow")
		for _, want := range []string{illustrativeMarker, illustrativeChip, `<pre class="mermaid">`} {
			if !strings.Contains(body, want) {
				t.Errorf("diagram peek missing %q\n%s", want, body)
			}
		}
	})

	t.Run("NEGATIVE: peeking a proposal diagram never paints it illustrative", func(t *testing.T) {
		body := peek("diagram/refi-flow-future")
		if strings.Contains(body, "illustrative") {
			t.Errorf("proposal peek painted with the illustrative badge\n%s", body)
		}
		if !strings.Contains(body, `data-diagram-tier="full"`) {
			t.Errorf("proposal peek missing the extractor-computed tier marker\n%s", body)
		}
	})
}
