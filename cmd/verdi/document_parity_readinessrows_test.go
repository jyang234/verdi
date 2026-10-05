package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/readinessload"
	"github.com/jyang234/verdi/internal/workbench"
)

// readinessRowsFeatureSpec declares two criteria and one non-spike stub
// that lists only ac-1, so ac-2 has no stub: the readiness derivation's
// success/coverage/ac-2 row (SI-338 (4)).
const readinessRowsFeatureSpec = `---
id: spec/keyring
kind: spec
title: "Keyring"
owners: [platform-team]
class: feature
problem: { text: "Keys go missing.", anchor: problem }
outcome: { text: "Every key is accounted for.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A key is checked out to one holder.", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "A missing key is reported within a day.", evidence: [attestation], anchor: ac-2 }
stubs:
  - { slug: key-checkout, acceptance_criteria: [ac-1] }
---
# Keyring

## Problem

Keys go missing.

## Outcome

Every key is accounted for.

## ac-1

Checked out.

## ac-2

Reported.
`

// readinessRowsStorySpec declares no acceptance criteria — legal for the
// story class — so the derivation reads success/criteria violated with a
// witness (SI-338 (5)), where it used to fail every surface operationally.
const readinessRowsStorySpec = `---
id: spec/bare-story
kind: spec
title: "Bare story"
owners: [platform-team]
class: story
story: jira:KEY-1
problem: { text: "The story has no criteria.", anchor: problem }
outcome: { text: "The story is reviewable.", anchor: outcome }
links:
  - { type: implements, ref: "spec/keyring#ac-1" }
---
# Bare story

## Problem

The story has no criteria.

## Outcome

The story is reviewable.
`

// TestDocumentParity_CoverageAndCriteriaRowsOnFourSurfaces pins the two
// new readiness rows on every surface that renders the snapshot's
// Readiness section (spec/readiness-recovery-v2 ac-4; SI-338): the CLI's
// own derivation, MCP get_document, and the board's Document tab — three
// independently constructed loaders — render byte-identical Markdown that
// carries the row, the readiness page renders the same row from its own
// per-request derivation, and the same ref at the same HEAD derives
// identical bytes twice (ac-2).
func TestDocumentParity_CoverageAndCriteriaRowsOnFourSurfaces(t *testing.T) {
	t.Parallel()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Message: "adopt a store with a feature and a story without criteria",
		Files: map[string]string{
			".verdi/verdi.yaml":                      supersedeManifestYAML,
			".verdi/specs/active/keyring/spec.md":    readinessRowsFeatureSpec,
			".verdi/specs/active/bare-story/spec.md": readinessRowsStorySpec,
		},
	}})
	pinFixtureDefaultBranch(t, repo.Dir)
	env := []string{"CI_DEFAULT_BRANCH=main"}
	bin := buildVerdiBinary(t)
	loader := func() readinessload.Loader {
		return readinessload.Loader{Root: repo.Dir, Opts: readinessload.Options{BoardHref: workbench.BranchBoardHref}}
	}

	for _, tc := range []struct {
		name   string
		row    string
		absent string
	}{
		{name: "keyring", row: "success/coverage/ac-2", absent: "success/coverage/ac-1"},
		{name: "bare-story", row: "success/criteria"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref := "spec/" + tc.name
			cli, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", ref)
			if code != 0 {
				t.Fatalf("cli exit %d: %s", code, stderr)
			}
			again, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", ref)
			if code != 0 || again != cli {
				t.Fatalf("the same ref at the same HEAD rendered different bytes (exit %d, %s)", code, stderr)
			}
			mcp := mcpDocumentMarkdown(t, &mcpserve.Backend{Root: repo.Dir, ReadinessLoader: loader()}, ref)
			board := boardDocumentMarkdown(t, workbench.NewHandlerWith(repo.Dir, workbench.Deps{ReadinessLoader: loader()}), tc.name)
			if cli != mcp {
				t.Errorf("CLI and MCP differ:\n--- cli ---\n%s\n--- mcp ---\n%s", cli, mcp)
			}
			if cli != board {
				t.Errorf("CLI and the Document tab differ:\n--- cli ---\n%s\n--- board ---\n%s", cli, board)
			}
			anchor := `<a id="` + tc.row + `"></a>`
			for surface, doc := range map[string]string{"cli": cli, "mcp": mcp, "board": board} {
				if strings.Contains(doc, "Readiness was not supplied for this render.") {
					t.Fatalf("%s rendered no readiness:\n%s", surface, doc)
				}
				if !strings.Contains(doc, anchor) {
					t.Fatalf("%s's Readiness section lacks the %s row:\n%s", surface, tc.row, doc)
				}
				if tc.absent != "" && strings.Contains(doc, `<a id="`+tc.absent+`"></a>`) {
					t.Fatalf("%s carries %s, whose criterion a stub lists:\n%s", surface, tc.absent, doc)
				}
			}

			rec := httptest.NewRecorder()
			workbench.NewHandlerWith(repo.Dir, workbench.Deps{ReadinessLoader: loader()}).
				ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readiness?spec="+tc.name, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("readiness page: %d %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `data-concern-id="`+tc.row+`"`) {
				t.Fatalf("the readiness page lacks the %s row:\n%s", tc.row, rec.Body.String())
			}
			if tc.absent != "" && strings.Contains(rec.Body.String(), `data-concern-id="`+tc.absent+`"`) {
				t.Fatalf("the readiness page carries %s, whose criterion a stub lists:\n%s", tc.absent, rec.Body.String())
			}
		})
	}
}
