// Tests for D6-23's quartet-scoping helpers (round6-divergences.md):
// quartetPathPrefixes/inQuartetScope, exercised directly as pure functions.
//
// D6-23 originally wired lintQuartetOrRefuse (acceptlint.go) into accept's
// own ritual, refusing to freeze a quartet the store's own linter
// rejected. Task 7 (docs/superpowers/specs/2026-08-01-merge-signals-spec-
// acceptance-design.md) retired that ritual entirely — `verdi accept` no
// longer lints, flips status, or writes a frozen stamp at all — and
// lintQuartetOrRefuse itself is DELETED (this repo's policy: unused code
// is deleted, not left unwired; golangci-lint's `unused` would flag it
// regardless). Only acceptlint.go's pure quartet-scoping predicates
// survive: they carry no dependency on accept's retired mutation, and
// they are the reusable building block the design's rollout step 8 (the
// pre-merge required check that "verifies ... acceptance criteria,
// obligations, stubs, and required sidecars") would reach for when that
// gate is built — out of this build's scope. These tests cover exactly
// the surviving predicates.
package main

import (
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestQuartetPathPrefixes and TestInQuartetScope table-drive the pure
// scoping helpers directly (CLAUDE.md: every function gets happy- and
// negative-path table-driven unit tests).
func TestQuartetPathPrefixes(t *testing.T) {
	cases := []struct {
		name string
		ref  artifact.Ref
		spec *artifact.SpecFrontmatter
		want []string
	}{
		{
			name: "feature with no story ref: spec directory only",
			ref:  artifact.Ref{Kind: artifact.KindSpec, Name: "no-story-feature"},
			spec: &artifact.SpecFrontmatter{},
			want: []string{".verdi/specs/active/no-story-feature"},
		},
		{
			name: "story with a tracker ref: spec directory plus its attestations dir",
			ref:  artifact.Ref{Kind: artifact.KindSpec, Name: "stale-decline"},
			spec: &artifact.SpecFrontmatter{Story: "jira:LOAN-1482"},
			want: []string{".verdi/specs/active/stale-decline", ".verdi/attestations/jira-loan-1482"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := quartetPathPrefixes(tc.ref, tc.spec)
			if len(got) != len(tc.want) {
				t.Fatalf("quartetPathPrefixes = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("quartetPathPrefixes = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestInQuartetScope(t *testing.T) {
	prefixes := []string{".verdi/specs/active/stale-decline", ".verdi/attestations/jira-loan-1482"}
	cases := []struct {
		name string
		path string
		want bool
	}{
		{"exact directory match", ".verdi/specs/active/stale-decline", true},
		{"file nested under the spec directory", ".verdi/specs/active/stale-decline/layout.json", true},
		{"file nested under the attestations directory", ".verdi/attestations/jira-loan-1482/ac-1.md", true},
		{"a sibling spec directory sharing a name prefix must not match", ".verdi/specs/active/stale-decline-2/spec.md", false},
		{"an unrelated repo-wide path", ".gitattributes", false},
		{"an unrelated spec directory", ".verdi/specs/active/other-spec/spec.md", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := inQuartetScope(tc.path, prefixes); got != tc.want {
				t.Fatalf("inQuartetScope(%q, %v) = %t, want %t", tc.path, prefixes, got, tc.want)
			}
		})
	}
}
