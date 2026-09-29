package workbench

// spec/wall-changes: unit coverage for the pure classifier
// (classifyWallChanges) — the static obligations' direct unit under test
// (ac-1--static, ac-2--static). The git-backed integration
// (loadWallChangesInputs/computeWallChanges) is proven separately, over
// fixturegit repositories, by the behavioral tests in
// wallchanges_integration_test.go (ac-1/ac-2/ac-3--behavioral, co-2).

import (
	"testing"

	"github.com/jyang234/verdi/internal/draftmutation"
)

const wallChangesHeadSpec = `---
id: spec/wall-fixture
kind: spec
class: feature
title: Wall fixture
owners: [platform-team]
problem: { text: "problem text", anchor: "#problem" }
outcome: { text: "outcome text", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "original ac-1 text", evidence: [attestation], anchor: "#ac-1" }
---
# Wall fixture

## Problem

Original problem prose.

## Outcome

Original outcome prose.

## ac-1

Original ac-1 prose.
`

// wallChangesTypedOnly changes only the ac-1 frontmatter field: every body
// section stays byte-identical to wallChangesHeadSpec.
const wallChangesTypedOnly = `---
id: spec/wall-fixture
kind: spec
class: feature
title: Wall fixture
owners: [platform-team]
problem: { text: "problem text", anchor: "#problem" }
outcome: { text: "outcome text", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "edited ac-1 text", evidence: [attestation], anchor: "#ac-1" }
---
# Wall fixture

## Problem

Original problem prose.

## Outcome

Original outcome prose.

## ac-1

Original ac-1 prose.
`

// wallChangesProseOnly changes only body prose: the frontmatter is
// byte-identical to wallChangesHeadSpec, so the semantic diff recognizes
// nothing.
const wallChangesProseOnly = `---
id: spec/wall-fixture
kind: spec
class: feature
title: Wall fixture
owners: [platform-team]
problem: { text: "problem text", anchor: "#problem" }
outcome: { text: "outcome text", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "original ac-1 text", evidence: [attestation], anchor: "#ac-1" }
---
# Wall fixture

## Problem

Original problem prose.

## Outcome

Original outcome prose.

## ac-1

Edited ac-1 prose.
`

// wallChangesMixed changes both the ac-1 frontmatter field and body prose.
const wallChangesMixed = `---
id: spec/wall-fixture
kind: spec
class: feature
title: Wall fixture
owners: [platform-team]
problem: { text: "problem text", anchor: "#problem" }
outcome: { text: "outcome text", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "edited ac-1 text", evidence: [attestation], anchor: "#ac-1" }
---
# Wall fixture

## Problem

Original problem prose.

## Outcome

Original outcome prose.

## ac-1

Edited ac-1 prose.
`

const wallChangesUndecodable = "not a spec document at all, no frontmatter delimiters here"

const (
	wallTestSpecPath   = ".verdi/specs/active/wall-fixture/spec.md"
	wallTestLayoutPath = ".verdi/specs/active/wall-fixture/layout.json"
)

// findUnclassified returns the entry for path, or nil.
func findUnclassified(got []wallUnclassifiedChange, path string) *wallUnclassifiedChange {
	for i := range got {
		if got[i].Path == path {
			return &got[i]
		}
	}
	return nil
}

// TestWallChanges_Classify is spec/wall-changes ac-1's static obligation:
// for every pair of HEAD/working-tree inputs, the classifier returns
// recognized operations as typed changes, every other difference as an
// unclassified change with its reason, and an unreadable comparison with
// its reason when either side cannot be read or diffed.
func TestWallChanges_Classify(t *testing.T) {
	t.Run("typed-only edit", func(t *testing.T) {
		got, err := classifyWallChanges(wallChangesInputs{
			SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
			HeadExists: true, HeadSpec: []byte(wallChangesHeadSpec), WorkingSpec: []byte(wallChangesTypedOnly),
		})
		if err != nil {
			t.Fatalf("classifyWallChanges: %v", err)
		}
		if got.UnreadableReason != "" {
			t.Fatalf("UnreadableReason = %q, want empty", got.UnreadableReason)
		}
		if len(got.Typed) != 1 || got.Typed[0].Target != "ac-1" {
			t.Fatalf("Typed = %+v, want exactly one change touching ac-1", got.Typed)
		}
		if len(got.Unclassified) != 0 {
			t.Fatalf("Unclassified = %+v, want none (body untouched)", got.Unclassified)
		}
	})

	t.Run("prose-only edit", func(t *testing.T) {
		got, err := classifyWallChanges(wallChangesInputs{
			SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
			HeadExists: true, HeadSpec: []byte(wallChangesHeadSpec), WorkingSpec: []byte(wallChangesProseOnly),
		})
		if err != nil {
			t.Fatalf("classifyWallChanges: %v", err)
		}
		if len(got.Typed) != 0 {
			t.Fatalf("Typed = %+v, want none (frontmatter untouched)", got.Typed)
		}
		entry := findUnclassified(got.Unclassified, wallTestSpecPath)
		if entry == nil || entry.Reason != wallReasonProse {
			t.Fatalf("Unclassified = %+v, want %s carrying reason %q", got.Unclassified, wallTestSpecPath, wallReasonProse)
		}
	})

	t.Run("layout-only edit", func(t *testing.T) {
		got, err := classifyWallChanges(wallChangesInputs{
			SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
			HeadExists: true, HeadSpec: []byte(wallChangesHeadSpec), WorkingSpec: []byte(wallChangesHeadSpec),
			ChangedPaths: []wallChangedPath{{Path: wallTestLayoutPath}},
		})
		if err != nil {
			t.Fatalf("classifyWallChanges: %v", err)
		}
		if len(got.Typed) != 0 {
			t.Fatalf("Typed = %+v, want none", got.Typed)
		}
		entry := findUnclassified(got.Unclassified, wallTestLayoutPath)
		if entry == nil || entry.Reason != wallReasonLayout {
			t.Fatalf("Unclassified = %+v, want %s carrying reason %q", got.Unclassified, wallTestLayoutPath, wallReasonLayout)
		}
	})

	t.Run("another staged path", func(t *testing.T) {
		got, err := classifyWallChanges(wallChangesInputs{
			SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
			HeadExists: true, HeadSpec: []byte(wallChangesHeadSpec), WorkingSpec: []byte(wallChangesHeadSpec),
			ChangedPaths: []wallChangedPath{{Path: ".verdi/specs/active/other-spec/spec.md", Untracked: false}},
		})
		if err != nil {
			t.Fatalf("classifyWallChanges: %v", err)
		}
		entry := findUnclassified(got.Unclassified, ".verdi/specs/active/other-spec/spec.md")
		if entry == nil || entry.Reason != wallReasonStagedPath {
			t.Fatalf("Unclassified = %+v, want the other path carrying reason %q", got.Unclassified, wallReasonStagedPath)
		}
	})

	t.Run("an untracked file", func(t *testing.T) {
		got, err := classifyWallChanges(wallChangesInputs{
			SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
			HeadExists: true, HeadSpec: []byte(wallChangesHeadSpec), WorkingSpec: []byte(wallChangesHeadSpec),
			ChangedPaths: []wallChangedPath{{Path: "notes.txt", Untracked: true}},
		})
		if err != nil {
			t.Fatalf("classifyWallChanges: %v", err)
		}
		entry := findUnclassified(got.Unclassified, "notes.txt")
		if entry == nil || entry.Reason != wallReasonUntracked {
			t.Fatalf("Unclassified = %+v, want notes.txt carrying reason %q", got.Unclassified, wallReasonUntracked)
		}
	})

	t.Run("mixed tree", func(t *testing.T) {
		got, err := classifyWallChanges(wallChangesInputs{
			SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
			HeadExists: true, HeadSpec: []byte(wallChangesHeadSpec), WorkingSpec: []byte(wallChangesMixed),
			ChangedPaths: []wallChangedPath{
				{Path: wallTestLayoutPath},
				{Path: ".verdi/specs/active/other-spec/spec.md"},
				{Path: "notes.txt", Untracked: true},
			},
		})
		if err != nil {
			t.Fatalf("classifyWallChanges: %v", err)
		}
		if len(got.Typed) != 1 || got.Typed[0].Target != "ac-1" {
			t.Fatalf("Typed = %+v, want exactly one change touching ac-1", got.Typed)
		}
		if got.UnreadableReason != "" {
			t.Fatalf("UnreadableReason = %q, want empty (a readable comparison carries both lists, dc-2)", got.UnreadableReason)
		}
		wantReasons := map[string]wallChangeReason{
			wallTestSpecPath:                         wallReasonProse,
			wallTestLayoutPath:                       wallReasonLayout,
			".verdi/specs/active/other-spec/spec.md": wallReasonStagedPath,
			"notes.txt":                              wallReasonUntracked,
		}
		if len(got.Unclassified) != len(wantReasons) {
			t.Fatalf("Unclassified = %+v, want %d entries", got.Unclassified, len(wantReasons))
		}
		for path, reason := range wantReasons {
			entry := findUnclassified(got.Unclassified, path)
			if entry == nil || entry.Reason != reason {
				t.Errorf("path %s: got %+v, want reason %q", path, entry, reason)
			}
		}
	})

	t.Run("undecodable HEAD spec", func(t *testing.T) {
		got, err := classifyWallChanges(wallChangesInputs{
			SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
			HeadExists: true, HeadSpec: []byte(wallChangesUndecodable), WorkingSpec: []byte(wallChangesHeadSpec),
		})
		if err != nil {
			t.Fatalf("classifyWallChanges: %v", err)
		}
		if got.UnreadableReason == "" {
			t.Fatal("UnreadableReason = \"\", want a disclosed reason naming HEAD")
		}
		if len(got.Typed) != 0 {
			t.Fatalf("Typed = %+v, want none when unreadable", got.Typed)
		}
	})

	t.Run("undecodable working-tree spec", func(t *testing.T) {
		got, err := classifyWallChanges(wallChangesInputs{
			SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
			HeadExists: true, HeadSpec: []byte(wallChangesHeadSpec), WorkingSpec: []byte(wallChangesUndecodable),
		})
		if err != nil {
			t.Fatalf("classifyWallChanges: %v", err)
		}
		if got.UnreadableReason == "" {
			t.Fatal("UnreadableReason = \"\", want a disclosed reason naming the working tree")
		}
		if len(got.Typed) != 0 {
			t.Fatalf("Typed = %+v, want none when unreadable", got.Typed)
		}
	})

	t.Run("a missing spec", func(t *testing.T) {
		got, err := classifyWallChanges(wallChangesInputs{
			SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
			HeadExists: false, WorkingSpec: []byte(wallChangesHeadSpec),
		})
		if err != nil {
			t.Fatalf("classifyWallChanges: %v", err)
		}
		if got.UnreadableReason == "" {
			t.Fatal("UnreadableReason = \"\", want a disclosed reason: HEAD has no revision of this spec")
		}
		if len(got.Typed) != 0 {
			t.Fatalf("Typed = %+v, want none: never reported as no changes (ac-1)", got.Typed)
		}
	})

	t.Run("clean tree reports nothing", func(t *testing.T) {
		got, err := classifyWallChanges(wallChangesInputs{
			SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
			HeadExists: true, HeadSpec: []byte(wallChangesHeadSpec), WorkingSpec: []byte(wallChangesHeadSpec),
		})
		if err != nil {
			t.Fatalf("classifyWallChanges: %v", err)
		}
		if got.UnreadableReason != "" || len(got.Typed) != 0 || len(got.Unclassified) != 0 {
			t.Fatalf("classifyWallChanges(clean) = %+v, want an entirely empty summary", got)
		}
	})
}

// TestWallChanges_ZeroOperationsStayDirty is ac-2's static obligation: when
// the semantic diff recognizes no operation but any other change remains,
// the classifier keeps listing that change as unclassified rather than
// silently reporting an all-clean (empty) result — the shape
// boardGitState.Dirty (git status's own independent answer, unaffected by
// this classifier) must never contradict.
func TestWallChanges_ZeroOperationsStayDirty(t *testing.T) {
	cases := []struct {
		name string
		in   wallChangesInputs
		path string
	}{
		{
			name: "prose-only",
			in: wallChangesInputs{
				SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
				HeadExists: true, HeadSpec: []byte(wallChangesHeadSpec), WorkingSpec: []byte(wallChangesProseOnly),
			},
			path: wallTestSpecPath,
		},
		{
			name: "layout-only",
			in: wallChangesInputs{
				SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
				HeadExists: true, HeadSpec: []byte(wallChangesHeadSpec), WorkingSpec: []byte(wallChangesHeadSpec),
				ChangedPaths: []wallChangedPath{{Path: wallTestLayoutPath}},
			},
			path: wallTestLayoutPath,
		},
		{
			name: "another-staged-path",
			in: wallChangesInputs{
				SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
				HeadExists: true, HeadSpec: []byte(wallChangesHeadSpec), WorkingSpec: []byte(wallChangesHeadSpec),
				ChangedPaths: []wallChangedPath{{Path: ".verdi/specs/active/other-spec/spec.md"}},
			},
			path: ".verdi/specs/active/other-spec/spec.md",
		},
		{
			name: "untracked-file",
			in: wallChangesInputs{
				SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
				HeadExists: true, HeadSpec: []byte(wallChangesHeadSpec), WorkingSpec: []byte(wallChangesHeadSpec),
				ChangedPaths: []wallChangedPath{{Path: "notes.txt", Untracked: true}},
			},
			path: "notes.txt",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := classifyWallChanges(tc.in)
			if err != nil {
				t.Fatalf("classifyWallChanges: %v", err)
			}
			if len(got.Typed) != 0 {
				t.Fatalf("Typed = %+v, want zero recognized operations", got.Typed)
			}
			if len(got.Unclassified) == 0 {
				t.Fatal("Unclassified is empty: a remaining change with zero recognized operations yielded a clean result (ac-2 violation)")
			}
			if findUnclassified(got.Unclassified, tc.path) == nil {
				t.Fatalf("Unclassified = %+v, want an entry for %s", got.Unclassified, tc.path)
			}
		})
	}
}

// TestWallChanges_DiffSpecBytesReused proves diffSpecBytes' typed output is
// exactly draftmutation.Diff's own — dc-1: reused, never re-derived.
func TestWallChanges_DiffSpecBytesReused(t *testing.T) {
	want, _, err := draftmutation.Diff([]byte(wallChangesHeadSpec), []byte(wallChangesTypedOnly))
	if err != nil {
		t.Fatalf("draftmutation.Diff: %v", err)
	}
	got, prose, reason, err := diffSpecBytes([]byte(wallChangesHeadSpec), []byte(wallChangesTypedOnly))
	if err != nil || reason != "" {
		t.Fatalf("diffSpecBytes: got=%v prose=%v reason=%q err=%v", got, prose, reason, err)
	}
	if len(got) != len(want) {
		t.Fatalf("diffSpecBytes typed = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("diffSpecBytes typed[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
