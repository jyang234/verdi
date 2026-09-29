// TestIndex_DisclosuresCount (spec/index-coverage ac-3--behavioral) proves
// the served index carries the disclosures count, computed once per
// render, from the SAME enumeration the disclosures page shows
// (internal/disclosureview's Current/Count) — over fixture stores with and
// without disclosures. The carrier is a non-visible machine-readable
// data-disclosures-count attribute on the home page's existing "Disclosures"
// pointer paragraph (the visible Disclosures toggle itself is a later
// lane's top-bar work; see this lane's report).
package workbench

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// indexDisclosureManifestYAML mirrors internal/disclosureview's own test
// manifest (a configured jira scheme so VL-005 has nothing to say about
// the fixture's story ref).
const indexDisclosureManifestYAML = `schema: verdi.layout/v1
forge: gitlab
providers:
  jira:
    base_url: https://example.atlassian.net
    rollup_field: customfield_00000
services:
  discovery: flowmap
`

// indexDisclosureSpecMD is a minimal, decodable new-class (story) spec —
// the shape VL-017's disclosed-unproven notice applies to when the
// mutable zone is absent (a bare clone).
const indexDisclosureSpecMD = `---
id: spec/index-panel-fixture
kind: spec
title: "Index Panel Fixture"
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
# Index Panel Fixture

## Problem

p

## Outcome

o

## ac-1

a
`

// buildIndexDisclosureFixture writes a minimal store (no mutable zone —
// the bare-clone shape) whose lint run yields exactly one
// disclosure-severity finding (VL-017), for the "with a disclosure" case.
func buildIndexDisclosureFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	specDir := filepath.Join(root, ".verdi", "specs", "active", "index-panel-fixture")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(indexDisclosureSpecMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "verdi.yaml"), []byte(indexDisclosureManifestYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// buildIndexDisclosureFreeFixture is buildIndexDisclosureFixture with its
// mutable zone present, so VL-017 no longer fires (mirroring
// internal/disclosureview's TestCurrent_FreshPerCall) — the "with none"
// case.
func buildIndexDisclosureFreeFixture(t *testing.T) string {
	t.Helper()
	root := buildIndexDisclosureFixture(t)
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "data", "mutable"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestIndex_DisclosuresCount(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  int
	}{
		{"a store with a disclosure", buildIndexDisclosureFixture, 1},
		{"a store with none", buildIndexDisclosureFreeFixture, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.setup(t)
			code, body := getHome(t, root, HomeDeps{Index: cannedIndex(nil, nil)})
			if code != http.StatusOK {
				t.Fatalf("GET / = %d, want 200\n%s", code, body)
			}
			want := fmt.Sprintf(`data-disclosures-count="%d"`, tt.want)
			if !strings.Contains(body, want) {
				t.Errorf("index body missing %q:\n%s", want, body)
			}
		})
	}
}
