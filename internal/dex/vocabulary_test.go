// spec/vocabulary-surfaces ac-2, the dex surface: built pages render the
// resolved model's display names on the LIFECYCLE-STATE surfaces — the
// page status badge, the listing chips, and the Class metadata row — with
// the bare id kept in every badge-<id> CSS class and testid, and byte-
// identical output when the store carries no model.yaml (proven by this
// package's whole pre-existing suite, which builds exactly such stores and
// stays untouched). The story-page ladder badges are the ONE exception:
// they are case-file FLAGS (spec-stale, pending-supersession), not
// lifecycle states, so their labels are FIXED and never vocabulary-
// resolved (finding judged-ladder-flags-share-state-namespace — see
// TestLadderBadgeViews_FlagLabelsNotVocabularyAddressable and
// internal/dex/ladder.go).
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

// TestBuildV2_ClassWordProse is the vocabulary-prose closure's dex case
// (closure finding 2's dex half, featurelens.go's "no implementing
// story" among the sites): over a model whose vocabulary renames story
// to "Workstream" (the committed vocab-rename.yaml's own classes block,
// reused verbatim — never string-surgered, which would duplicate its
// story key now that the fixture renames story itself), every
// class-word PROSE site in the built site speaks the renamed word — the
// feature lens' heading, mapping column, and empty marker; the chrome
// nav's by-story label; the home hub entry; the by-story axis title;
// the metadata card's Story row LABEL — while the identity layer (the
// /by-story/ URL and output paths, the tracker ref VALUE) provably
// keeps bare ids. The fixture's rename rides vocabulary.classes ON TOP
// of its Class.Display "Story", proving the chain's first rung wins on
// prose sites too (the full three-rung chain is model_test.go's
// TestDisplayClass_ThreeLevelChain).
func TestBuildV2_ClassWordProse(t *testing.T) {
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
			Message: "init class-word-prose store",
		},
	})

	outDir := t.TempDir()
	if err := Build(context.Background(), Options{Root: repo.Dir, OutDir: outDir}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	// The feature lens on the probe's permalink page.
	probe, err := os.ReadFile(filepath.Join(outDir, "a", "spec", "vocab-probe", "index.html"))
	if err != nil {
		t.Fatalf("reading probe page: %v", err)
	}
	page := string(probe)
	for _, want := range []string{
		"<h2>Workstreams</h2>",
		"<th>Implementing Workstreams</th>",
		`<span class="empty">no implementing Workstream</span>`,
		`<dt>Workstream</dt><dd>jira:LOAN-9001</dd>`, // the Story row: label renamed, tracker ref untouched
		`<a href="/by-story/">by Workstream</a>`,     // chrome nav: label renamed, URL identity
	} {
		if !strings.Contains(page, want) {
			t.Errorf("probe page missing renamed prose %q", want)
		}
	}
	for _, gone := range []string{"<h2>Stories</h2>", "Implementing stories", "no implementing story", ">by story<", "<dt>Story</dt>"} {
		if strings.Contains(page, gone) {
			t.Errorf("probe page still renders bare class-word prose %q", gone)
		}
	}

	// The home hub entry and the by-story axis hub, at their UNRENAMED
	// output paths (URL identity).
	home, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatalf("reading home page: %v", err)
	}
	if !strings.Contains(string(home), `<a href="/by-story/">By Workstream</a>`) {
		t.Error("home hub entry not renamed (or its /by-story/ href moved)")
	}
	hub, err := os.ReadFile(filepath.Join(outDir, "by-story", "index.html"))
	if err != nil {
		t.Fatalf("reading by-story hub (the output PATH must stay /by-story/): %v", err)
	}
	if !strings.Contains(string(hub), "<h1>By Workstream</h1>") {
		t.Error("by-story axis title not renamed")
	}
}

// TestLadderBadgeViews_FlagLabelsNotVocabularyAddressable is the ladder-
// chip unit negative case (one per surface, failing independently): the
// story-page ladder flags are case-file taxonomy, not lifecycle states, so
// their visible labels are FIXED — a vocabulary entry keyed `spec-stale`
// under `states:` does NOT rename the flag (finding
// judged-ladder-flags-share-state-namespace). The bare id stays for
// CSS/testid addressing.
func TestLadderBadgeViews_FlagLabelsNotVocabularyAddressable(t *testing.T) {
	// Adversarial model: it TRIES to rename spec-stale via states:.
	m := &model.Model{
		Schema:     "verdi.model/v1",
		Vocabulary: model.Vocabulary{States: map[string]string{"spec-stale": "Drifted"}},
	}
	views := ladderBadgeViews(m, []string{"spec-stale", "pending-supersession"})
	if len(views) != 2 {
		t.Fatalf("ladderBadgeViews returned %d views, want 2", len(views))
	}
	if views[0].ID != "spec-stale" || views[0].Label != "spec-stale" {
		t.Fatalf("views[0] = %+v, want ID+Label both the FIXED id spec-stale — a states entry keyed spec-stale must not rename the flag (judged-ladder-flags-share-state-namespace)", views[0])
	}
	if views[1].ID != "pending-supersession" || views[1].Label != "pending-supersession" {
		t.Fatalf("views[1] = %+v, want ID+Label both the fixed id pending-supersession", views[1])
	}

	// Nil model: identical fixed labels — the flag path never consults a model.
	var nilModel *model.Model
	views = ladderBadgeViews(nilModel, []string{"spec-stale"})
	if views[0].Label != "spec-stale" {
		t.Fatalf("nil-model label = %q, want the fixed id", views[0].Label)
	}
}
