package storyresolve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testFeatureSpec = `---
id: spec/matrix-helper-test
kind: spec
class: feature
title: "Matrix helper test spec"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1482
acceptance_criteria:
  - { id: ac-1, text: "t", evidence: [static] }
frozen: { at: 2026-05-14, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# body
`

const testStorySpec = `---
id: spec/matrix-helper-story
kind: spec
class: story
title: "Matrix helper story spec"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
story: jira:LOAN-1490
links:
  - { type: implements, ref: "spec/matrix-helper-test#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "t", evidence: [static] }
frozen: { at: 2026-05-14, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# body
`

const testComponentSpec = `---
id: spec/matrix-helper-component
kind: spec
class: component
title: "Matrix helper component spec"
status: active
owners: [platform-team]
---
# body
`

func writeActiveSpec(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing spec.md: %v", err)
	}
}

// TestResolve_Happy covers the two accepted input shapes (I-30): a spec
// ref, and a scheme-prefixed story ref matched against a feature spec's
// story: field.
func TestResolve_Happy(t *testing.T) {
	root := t.TempDir()
	writeActiveSpec(t, root, "matrix-helper-test", testFeatureSpec)

	t.Run("spec ref", func(t *testing.T) {
		spec, err := Resolve(root, "spec/matrix-helper-test")
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if spec.ID != "spec/matrix-helper-test" {
			t.Fatalf("ID = %q, want spec/matrix-helper-test", spec.ID)
		}
	})

	t.Run("scheme-prefixed story ref", func(t *testing.T) {
		spec, err := Resolve(root, "jira:LOAN-1482")
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if spec.ID != "spec/matrix-helper-test" {
			t.Fatalf("ID = %q, want spec/matrix-helper-test", spec.ID)
		}
	})

	// A round-four `class: story` spec ref resolves too (I-1): it is
	// story-grade and folds at the story level; only component specs are
	// rejected. Regression against the old feature-only gate that made a
	// round-four story unresolvable by matrix.
	t.Run("round-four story spec ref", func(t *testing.T) {
		storyRoot := t.TempDir()
		writeActiveSpec(t, storyRoot, "matrix-helper-story", testStorySpec)
		spec, err := Resolve(storyRoot, "spec/matrix-helper-story")
		if err != nil {
			t.Fatalf("Resolve(story spec ref): %v", err)
		}
		if spec.Class != "story" {
			t.Fatalf("Class = %q, want story", spec.Class)
		}
	})
}

// TestResolve_Negative covers, in I-30's strict regime: a bare tracker
// key (no scheme, not a spec ref) rejected with a message naming both
// accepted forms; a well-formed but unknown story ref that no spec claims;
// an ambiguous story ref two feature specs both claim; a spec ref naming a
// component spec (no story, no ACs); and no specs/active directory at all.
func TestResolve_Negative(t *testing.T) {
	t.Run("bare tracker key is rejected naming both forms", func(t *testing.T) {
		root := t.TempDir()
		writeActiveSpec(t, root, "matrix-helper-test", testFeatureSpec)
		_, err := Resolve(root, "STORY-1482")
		if err == nil {
			t.Fatal("Resolve(bare key STORY-1482): want error, got nil")
		}
		msg := err.Error()
		if !strings.Contains(msg, "jira:LOAN-1482") || !strings.Contains(msg, "spec/") {
			t.Fatalf("error %q must name both accepted forms (a scheme-prefixed story ref and a spec ref)", msg)
		}
	})

	t.Run("unknown story ref", func(t *testing.T) {
		root := t.TempDir()
		writeActiveSpec(t, root, "matrix-helper-test", testFeatureSpec)
		_, err := Resolve(root, "jira:NOPE-1")
		if err == nil {
			t.Fatal("Resolve(unknown story ref): want error, got nil")
		}
		if !strings.Contains(err.Error(), "jira:NOPE-1") {
			t.Fatalf("error %q should name the unmatched story ref", err.Error())
		}
	})

	t.Run("ambiguous story ref", func(t *testing.T) {
		root := t.TempDir()
		writeActiveSpec(t, root, "matrix-helper-test", testFeatureSpec)
		other := `---
id: spec/matrix-helper-test-2
kind: spec
class: feature
title: "Second spec, same story ref by construction"
status: draft
owners: [platform-team]
story: jira:LOAN-1482
acceptance_criteria:
  - { id: ac-1, text: "t", evidence: [static] }
---
# body
`
		writeActiveSpec(t, root, "matrix-helper-test-2", other)
		if _, err := Resolve(root, "jira:LOAN-1482"); err == nil {
			t.Fatal("Resolve(ambiguous story ref): want error, got nil")
		}
	})

	t.Run("component spec ref", func(t *testing.T) {
		root := t.TempDir()
		writeActiveSpec(t, root, "matrix-helper-component", testComponentSpec)
		if _, err := Resolve(root, "spec/matrix-helper-component"); err == nil {
			t.Fatal("Resolve(component spec ref): want error, got nil")
		}
	})

	t.Run("no specs/active directory at all", func(t *testing.T) {
		if _, err := Resolve(t.TempDir(), "jira:LOAN-1482"); err == nil {
			t.Fatal("Resolve(no specs/active dir): want error, got nil")
		}
	})
}

// TestResolveBuildSpec_Happy proves the feature/<name> branch convention
// `verdi feature start` cuts (cmd/verdi/feature.go) resolves back to the
// same spec with no argument at all — the inference `verdi align`/`verdi
// gate` rely on (PLAN.md Phase 8, 05 §CLI).
func TestResolveBuildSpec_Happy(t *testing.T) {
	root := t.TempDir()
	writeActiveSpec(t, root, "matrix-helper-test", testFeatureSpec)

	spec, err := ResolveBuildSpec(root, "feature/matrix-helper-test")
	if err != nil {
		t.Fatalf("ResolveBuildSpec: %v", err)
	}
	if spec.ID != "spec/matrix-helper-test" {
		t.Fatalf("ID = %q, want spec/matrix-helper-test", spec.ID)
	}
}

func TestResolveBuildSpec_Negative(t *testing.T) {
	root := t.TempDir()
	writeActiveSpec(t, root, "matrix-helper-test", testFeatureSpec)
	writeActiveSpec(t, root, "matrix-helper-component", testComponentSpec)

	cases := map[string]string{
		"not a build branch at all": "main",
		"design branch, not build":  "design/matrix-helper-test",
		"detached HEAD (empty)":     "",
		"unknown spec name":         "feature/does-not-exist",
		"component spec":            "feature/matrix-helper-component",
	}
	for name, branch := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ResolveBuildSpec(root, branch); err == nil {
				t.Fatalf("ResolveBuildSpec(%q): want error, got nil", branch)
			}
		})
	}
}
