package disclosureview

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/disclosure"
)

// storySpecMD is a minimal, decodable new-class (story) spec — the shape
// VL-017's disclosed-unproven notice applies to when the mutable zone is
// absent (a bare clone), which is exactly the disclosure this package's
// happy path must enumerate.
const storySpecMD = `---
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

// manifestYAML mirrors internal/lint's own test manifest: a configured
// jira scheme so VL-005 has nothing to say about the fixture's story ref.
const manifestYAML = `schema: verdi.layout/v1
forge: gitlab
providers:
  jira:
    base_url: https://example.atlassian.net
    rollup_field: customfield_00000
services:
  discovery: flowmap
`

// buildFixtureStore writes a minimal store (no git history, no mutable
// zone — the bare-clone shape) whose lint run yields exactly one
// disclosure-severity finding: VL-017's disclosed-unproven notice for the
// new-class spec.
func buildFixtureStore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	specDir := filepath.Join(root, ".verdi", "specs", "active", "panel-fixture")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(storySpecMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "verdi.yaml"), []byte(manifestYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte(".verdi/specs/*/*/board.json          gitlab-generated\n.verdi/specs/*/*/rollup.json         gitlab-generated\n.verdi/specs/*/*/deviation-report.md gitlab-generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// listTree returns every path under root, for the never-persisted check.
func listTree(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.Walk(root, func(p string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	return paths
}

func TestCurrent_EnumeratesLintDisclosures(t *testing.T) {
	root := buildFixtureStore(t)
	before := listTree(t, root)

	items, err := Current(context.Background(), root)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}

	var vl017 []disclosure.Disclosure
	for _, d := range items {
		if d.Source == "lint:VL-017" {
			vl017 = append(vl017, d)
		}
	}
	if len(vl017) != 1 {
		t.Fatalf("want exactly 1 lint:VL-017 disclosure, got %d (all items: %+v)", len(vl017), items)
	}
	d := vl017[0]
	if d.Scope != ".verdi/specs/active/panel-fixture/spec.md" {
		t.Errorf("Scope = %q, want the fixture spec path", d.Scope)
	}
	if d.Severity != disclosure.SeverityDisclosedUnproven {
		t.Errorf("Severity = %q, want %q", d.Severity, disclosure.SeverityDisclosedUnproven)
	}
	if d.ID != d.Source+"/"+d.Scope {
		t.Errorf("ID = %q, want the seam's content-derived id %q", d.ID, d.Source+"/"+d.Scope)
	}
	if !strings.Contains(d.Text, "disclosed-unproven") {
		t.Errorf("Text = %q, want VL-017's own message", d.Text)
	}

	// Never persisted (ac-1): enumerating writes nothing anywhere under
	// the store.
	after := listTree(t, root)
	if len(before) != len(after) {
		t.Fatalf("Current wrote into the store: before %d paths, after %d\nafter: %v", len(before), len(after), after)
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("Current changed the store tree: %q vs %q", before[i], after[i])
		}
	}
}

// TestCurrent_FreshPerCall proves the enumeration reflects the checkout's
// CURRENT state (ac-1: computed fresh, not a log): once the mutable zone
// exists, VL-017's bare-clone disclosure disappears from the very next
// call on the same root.
func TestCurrent_FreshPerCall(t *testing.T) {
	root := buildFixtureStore(t)

	items, err := Current(context.Background(), root)
	if err != nil {
		t.Fatalf("Current (bare): %v", err)
	}
	if len(items) == 0 {
		t.Fatal("bare-clone fixture yielded no disclosures; VL-017 should have fired")
	}

	if err := os.MkdirAll(filepath.Join(root, ".verdi", "data", "mutable"), 0o755); err != nil {
		t.Fatal(err)
	}
	items, err = Current(context.Background(), root)
	if err != nil {
		t.Fatalf("Current (mutable present): %v", err)
	}
	for _, d := range items {
		if d.Source == "lint:VL-017" {
			t.Fatalf("stale enumeration: lint:VL-017 still present after the mutable zone appeared: %+v", d)
		}
	}
}

func TestCurrent_AppendsExtrasSorted(t *testing.T) {
	root := buildFixtureStore(t)
	extra := disclosure.New("mcp:review-feed", "", "forge configured but unreachable")

	items, err := Current(context.Background(), root, extra)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	found := false
	for _, d := range items {
		if d == extra {
			found = true
		}
	}
	if !found {
		t.Fatalf("extra disclosure not enumerated: %+v", items)
	}
	if !sort.SliceIsSorted(items, func(i, j int) bool {
		if items[i].ID != items[j].ID {
			return items[i].ID < items[j].ID
		}
		return items[i].Text < items[j].Text
	}) {
		t.Fatalf("enumeration not deterministically sorted: %+v", items)
	}
}

func TestCurrent_MissingRootErrors(t *testing.T) {
	_, err := Current(context.Background(), filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("Current on a nonexistent root: want an operational error, got nil")
	}
}

func TestHTML_ItemCarriesSeamFields(t *testing.T) {
	d := disclosure.New("lint:VL-017", ".verdi/specs/active/x/spec.md", "the check is disclosed-unproven")
	got := string(HTML([]disclosure.Disclosure{d}, "computed fresh"))

	for name, want := range map[string]string{
		"stable id":      `data-disclosure-id="lint:VL-017/.verdi/specs/active/x/spec.md"`,
		"severity badge": `<span class="disclosure-severity">disclosed-unproven</span>`,
		"source":         `<code class="disclosure-source">lint:VL-017</code>`,
		"scope":          `<code class="disclosure-scope">.verdi/specs/active/x/spec.md</code>`,
		"text":           `<p class="disclosure-text">the check is disclosed-unproven</p>`,
		"compute note":   `computed fresh`,
		"count":          `data-count="1"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered view missing %s: want substring %q in:\n%s", name, want, got)
		}
	}
	if strings.Contains(got, "disclosures-empty") {
		t.Error("non-empty enumeration must not render the empty state")
	}
}

// TestHTML_ConsistentAcrossProducers: items from different producers render
// with the identical field structure (ac-2's one consistent rendering).
func TestHTML_ConsistentAcrossProducers(t *testing.T) {
	items := []disclosure.Disclosure{
		disclosure.New("lint:VL-017", "a/path.md", "t1"),
		disclosure.New("mcp:review-feed", "", "t2"),
	}
	got := string(HTML(items, ""))
	if n := strings.Count(got, `<li class="disclosure-item"`); n != 2 {
		t.Fatalf("want 2 identically-classed items, got %d:\n%s", n, got)
	}
	if n := strings.Count(got, `<span class="disclosure-severity">disclosed-unproven</span>`); n != 2 {
		t.Fatalf("want the same severity rendering on both items, got %d:\n%s", n, got)
	}
	// A scopeless disclosure simply omits the scope element — no dangling
	// empty code tag.
	if strings.Contains(got, `<code class="disclosure-scope"></code>`) {
		t.Error("scopeless item rendered an empty scope element")
	}
}

func TestHTML_EscapesUntrustedText(t *testing.T) {
	d := disclosure.New("lint:VL-017", "p", `<script>alert("x")</script>`)
	got := string(HTML([]disclosure.Disclosure{d}, ""))
	if strings.Contains(got, `<script>alert`) {
		t.Fatalf("producer-authored text must be HTML-escaped (data, never instructions):\n%s", got)
	}
}

// TestHTML_EmptyState: an empty enumeration is an explicit positive claim
// (the story's ac-1: never a blank page — silence is never a pass).
func TestHTML_EmptyState(t *testing.T) {
	got := string(HTML(nil, "computed at build"))
	for name, want := range map[string]string{
		"empty container": `class="disclosures-empty"`,
		"positive claim":  "No current disclosures.",
		"count zero":      `data-count="0"`,
		"compute note":    "computed at build",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("empty state missing %s: want substring %q in:\n%s", name, want, got)
		}
	}
	if strings.Contains(got, "disclosure-item") {
		t.Error("empty enumeration must not render items")
	}
}
