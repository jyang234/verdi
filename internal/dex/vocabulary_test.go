// spec/vocabulary-surfaces ac-2, the dex surface: built pages render the
// resolved model's display names — the page status badge, the listing
// chips, the Class metadata row, and the story-page ladder badges — with
// the bare id kept in every badge-<id> CSS class and testid, and
// byte-identical output when the store carries no model.yaml (proven by
// this package's whole pre-existing suite, which builds exactly such
// stores and stays untouched).
package dex

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/model"
)

const vocabManifestYAML = `schema: verdi.layout/v1
forge: gitlab
providers:
  jira:
    base_url: https://example.atlassian.net
    rollup_field: customfield_00000
services:
  discovery: flowmap
`

const vocabProbeSpecMD = `---
id: spec/vocab-probe
kind: spec
title: "Vocab probe"
owners: [platform-team]
class: feature
status: accepted-pending-build
story: jira:LOAN-9001
problem: { text: "renames leak per surface", anchor: problem }
outcome: { text: "renames land everywhere at once", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "renamed labels render", evidence: [behavioral] }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# Vocab probe

## Problem

Renames leak per surface.

## Outcome

Renames land everywhere at once.
`

// TestBuildV2_VocabularyRenames builds a store carrying model-schema's
// vocab-rename.yaml (reused verbatim as .verdi/model.yaml) and proves the
// renamed labels reach the built pages: the raw state id never renders as
// visible badge text anywhere in the site, while the renamed state and
// class words do — and every badge-<id> CSS class keeps the bare id.
func TestBuildV2_VocabularyRenames(t *testing.T) {
	modelYAML, err := os.ReadFile(filepath.Join("..", "model", "testdata", "vocab-rename.yaml"))
	if err != nil {
		t.Fatalf("reading vocab-rename.yaml: %v", err)
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                       vocabManifestYAML,
				".verdi/model.yaml":                       string(modelYAML),
				".verdi/specs/active/vocab-probe/spec.md": vocabProbeSpecMD,
			},
			Message: "init vocab-rename store",
		},
	})

	outDir := t.TempDir()
	if err := Build(context.Background(), Options{Root: repo.Dir, OutDir: outDir}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	var sawRenamedState, sawRenamedClass bool
	err = filepath.WalkDir(outDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		html := string(data)
		if strings.Contains(html, `>accepted-pending-build<`) {
			t.Errorf("%s renders the bare state id as visible text; want the renamed label everywhere", path)
		}
		if strings.Contains(html, "Ready to build") {
			sawRenamedState = true
			if !strings.Contains(html, `badge-accepted-pending-build`) {
				t.Errorf("%s renders the renamed state without keeping badge-accepted-pending-build as the CSS id", path)
			}
		}
		if strings.Contains(html, ">Initiative<") {
			sawRenamedClass = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking built site: %v", err)
	}
	if !sawRenamedState {
		t.Fatal("no built page renders the renamed state label \"Ready to build\"")
	}
	if !sawRenamedClass {
		t.Fatal("no built page renders the renamed class label \"Initiative\" (the Class metadata row)")
	}
}

// TestLadderBadgeViews_ModelVocabulary is the ladder-chip unit case (one
// case per surface, failing independently): the story-page ladder badges
// resolve their visible words through the model's state-display lookup
// with id fallback, while the badge id stays for CSS/testid addressing.
func TestLadderBadgeViews_ModelVocabulary(t *testing.T) {
	m := &model.Model{
		Schema:     "verdi.model/v1",
		Vocabulary: model.Vocabulary{States: map[string]string{"spec-stale": "Drifted"}},
	}
	views := ladderBadgeViews(m, []string{"spec-stale", "pending-supersession"})
	if len(views) != 2 {
		t.Fatalf("ladderBadgeViews returned %d views, want 2", len(views))
	}
	if views[0].ID != "spec-stale" || views[0].Label != "Drifted" {
		t.Fatalf("views[0] = %+v, want ID spec-stale with renamed label Drifted", views[0])
	}
	if views[1].ID != "pending-supersession" || views[1].Label != "pending-supersession" {
		t.Fatalf("views[1] = %+v, want the id fallback", views[1])
	}

	var nilModel *model.Model
	views = ladderBadgeViews(nilModel, []string{"spec-stale"})
	if views[0].Label != "spec-stale" {
		t.Fatalf("nil-model label = %q, want the bare id", views[0].Label)
	}
}
