package main

import (
	"os"
	"path/filepath"
	"testing"
)

const matrixTestFeatureSpec = `---
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

const matrixTestComponentSpec = `---
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

// TestResolveSpec_Happy covers both accepted input shapes: a direct spec
// ref, and a story key matched via storyKeyMatches.
func TestResolveSpec_Happy(t *testing.T) {
	root := t.TempDir()
	writeActiveSpec(t, root, "matrix-helper-test", matrixTestFeatureSpec)

	t.Run("spec ref", func(t *testing.T) {
		spec, err := resolveSpec(root, "spec/matrix-helper-test")
		if err != nil {
			t.Fatalf("resolveSpec: %v", err)
		}
		if spec.ID != "spec/matrix-helper-test" {
			t.Fatalf("ID = %q, want spec/matrix-helper-test", spec.ID)
		}
	})

	t.Run("story key", func(t *testing.T) {
		spec, err := resolveSpec(root, "STORY-1482")
		if err != nil {
			t.Fatalf("resolveSpec: %v", err)
		}
		if spec.ID != "spec/matrix-helper-test" {
			t.Fatalf("ID = %q, want spec/matrix-helper-test", spec.ID)
		}
	})
}

// TestResolveSpec_Negative covers: no match, an ambiguous match (two
// specs whose story keys both match), and a spec ref naming a component
// spec (no story, no ACs — matrix cannot fold it).
func TestResolveSpec_Negative(t *testing.T) {
	t.Run("no match", func(t *testing.T) {
		root := t.TempDir()
		writeActiveSpec(t, root, "matrix-helper-test", matrixTestFeatureSpec)
		if _, err := resolveSpec(root, "STORY-9999"); err == nil {
			t.Fatal("resolveSpec(no matching story): want error, got nil")
		}
	})

	t.Run("ambiguous match", func(t *testing.T) {
		root := t.TempDir()
		writeActiveSpec(t, root, "matrix-helper-test", matrixTestFeatureSpec)
		other := `---
id: spec/matrix-helper-test-2
kind: spec
class: feature
title: "Second spec, same story key by construction"
status: draft
owners: [platform-team]
story: jira:LOAN-1482
acceptance_criteria:
  - { id: ac-1, text: "t", evidence: [static] }
---
# body
`
		writeActiveSpec(t, root, "matrix-helper-test-2", other)
		if _, err := resolveSpec(root, "STORY-1482"); err == nil {
			t.Fatal("resolveSpec(ambiguous story match): want error, got nil")
		}
	})

	t.Run("component spec ref", func(t *testing.T) {
		root := t.TempDir()
		writeActiveSpec(t, root, "matrix-helper-component", matrixTestComponentSpec)
		if _, err := resolveSpec(root, "spec/matrix-helper-component"); err == nil {
			t.Fatal("resolveSpec(component spec ref): want error, got nil")
		}
	})

	t.Run("no specs/active directory at all", func(t *testing.T) {
		if _, err := resolveSpec(t.TempDir(), "STORY-1482"); err == nil {
			t.Fatal("resolveSpec(no specs/active dir): want error, got nil")
		}
	})
}

// TestStoryKeyMatches_Happy proves the trailing-digit-run comparison
// bridges a generic story key and a scheme-prefixed tracker ref that
// share a numeric suffix.
func TestStoryKeyMatches_Happy(t *testing.T) {
	if !storyKeyMatches("STORY-1482", "jira:LOAN-1482") {
		t.Fatal("storyKeyMatches(STORY-1482, jira:LOAN-1482) = false, want true (both end in 1482)")
	}
	if !storyKeyMatches("story-1482", "jira:LOAN-1482") {
		t.Fatal("storyKeyMatches is not case-sensitive-broken: lowercase arg must still match")
	}
}

// TestStoryKeyMatches_Negative covers a numeric mismatch and the
// digit-less case (neither side should match on an empty suffix).
func TestStoryKeyMatches_Negative(t *testing.T) {
	if storyKeyMatches("STORY-9999", "jira:LOAN-1482") {
		t.Fatal("storyKeyMatches(STORY-9999, jira:LOAN-1482) = true, want false (different numbers)")
	}
	if storyKeyMatches("spec/no-digits", "adr/also-no-digits") {
		t.Fatal("storyKeyMatches with no digits on either side = true, want false (empty suffixes must never match)")
	}
}

// TestResolveStorySlug_Happy proves it prefers the argument's own slug
// when a waivers/ or attestations/ directory exists there, and falls back
// to the spec's story-field slug when only that one exists.
func TestResolveStorySlug_Happy(t *testing.T) {
	t.Run("argument's own slug exists on disk", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".verdi", "waivers", "story-1482"), 0o755); err != nil {
			t.Fatal(err)
		}
		got := resolveStorySlug(root, "STORY-1482", "jira:LOAN-1482")
		if got != "story-1482" {
			t.Fatalf("resolveStorySlug = %q, want story-1482", got)
		}
	})

	t.Run("falls back to the spec's story-field slug", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".verdi", "attestations", "jira-loan-1482"), 0o755); err != nil {
			t.Fatal(err)
		}
		got := resolveStorySlug(root, "STORY-1482", "jira:LOAN-1482")
		if got != "jira-loan-1482" {
			t.Fatalf("resolveStorySlug = %q, want jira-loan-1482 (fallback)", got)
		}
	})
}

// TestResolveStorySlug_Negative proves that when neither candidate
// directory exists, resolveStorySlug still returns a deterministic slug
// (the argument's own) rather than an error — a story with no
// waivers/attestations at all is the ordinary case.
func TestResolveStorySlug_Negative(t *testing.T) {
	got := resolveStorySlug(t.TempDir(), "STORY-1482", "jira:LOAN-1482")
	if got != "story-1482" {
		t.Fatalf("resolveStorySlug (neither exists) = %q, want the argument's own slug story-1482", got)
	}
}
