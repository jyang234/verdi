package workbench

// spec/wall-changes: unit coverage for the pure classifier
// (classifyWallChanges) — the static obligations' direct unit under test
// (ac-1--static, ac-2--static) — and for the git-backed loader through the
// wallGitReader port (every git-failure path, and the status mapping). The
// real git integration (computeWallChanges over fixturegit repositories)
// is proven separately by the behavioral tests in
// wallchanges_integration_test.go (ac-1/ac-2/ac-3--behavioral, co-2).

import (
	"context"
	"encoding/json"
	"errors"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/gitx"
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

// The SI-298 working-tree states: each changes the spec file in a way the
// semantic snapshot does not type.
var (
	// wallChangesTitleOnly changes only the title.
	wallChangesTitleOnly = strings.Replace(wallChangesHeadSpec, "title: Wall fixture\n", "title: Wall fixture renamed\n", 1)
	// wallChangesOwnersOnly changes only the owners.
	wallChangesOwnersOnly = strings.Replace(wallChangesHeadSpec, "owners: [platform-team]\n", "owners: [platform-team, design-team]\n", 1)
	// wallChangesImpactsAdded adds an impacts list.
	wallChangesImpactsAdded = strings.Replace(wallChangesHeadSpec, "owners: [platform-team]\n", "owners: [platform-team]\nimpacts: [loansvc]\n", 1)
	// wallChangesQuotingOnly re-quotes the title: every decoded value is
	// identical, only the bytes differ.
	wallChangesQuotingOnly = strings.Replace(wallChangesHeadSpec, "title: Wall fixture\n", "title: \"Wall fixture\"\n", 1)
	// wallChangesTitleAndTyped changes the title beside a typed ac-1 edit.
	wallChangesTitleAndTyped = strings.Replace(wallChangesTypedOnly, "title: Wall fixture\n", "title: Wall fixture renamed\n", 1)
)

const wallTestSpecPath = ".verdi/specs/active/wall-fixture/spec.md"

// wallTestLayoutPath is derived exactly as production derives it.
var wallTestLayoutPath = filepath.ToSlash(boardlayout.FilePath(path.Dir(wallTestSpecPath)))

const wallTestOtherSpec = ".verdi/specs/active/other-spec/spec.md"

// wallIn builds classifier inputs over a committed HEAD spec. The spec is
// reported by git exactly when its bytes differ, unless status overrides.
func wallIn(working string, changed ...wallChangedPath) wallChangesInputs {
	return wallChangesInputs{
		SpecPath: wallTestSpecPath, LayoutPath: wallTestLayoutPath,
		HeadExists: true, HeadSpec: []byte(wallChangesHeadSpec), WorkingSpec: []byte(working),
		SpecStatus:   wallSpecStatus{Reported: working != wallChangesHeadSpec},
		ChangedPaths: changed,
	}
}

// withStatus replaces in's spec status.
func withStatus(in wallChangesInputs, status wallSpecStatus) wallChangesInputs {
	in.SpecStatus = status
	return in
}

// withHead replaces in's HEAD revision (exists == false means none),
// keeping wallIn's invariant: git reports the spec exactly when HEAD's
// bytes differ from the working tree's (a missing HEAD revision sets no
// status; those rows state it with withStatus).
func withHead(in wallChangesInputs, exists bool, head string) wallChangesInputs {
	in.HeadExists = exists
	in.HeadSpec = nil
	in.SpecStatus = wallSpecStatus{}
	if exists {
		in.HeadSpec = []byte(head)
		in.SpecStatus.Reported = head != string(in.WorkingSpec)
	}
	return in
}

// entry is a classifier-test unclassified entry.
func entry(p string, reason wallChangeReason) wallUnclassifiedChange {
	return wallUnclassifiedChange{Path: p, Reason: reason}
}

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
// its reason (naming which side) when either side cannot be read or
// diffed — still listing every other change (dc-2).
func TestWallChanges_Classify(t *testing.T) {
	notes := wallChangedPath{Path: "notes.txt", Untracked: true}
	cases := []struct {
		name string
		in   wallChangesInputs
		// wantTyped lists the typed targets; nil means unreadable, where
		// Typed must be nil (its reason instead of a typed list).
		wantTyped        []string
		wantUnclassified []wallUnclassifiedChange
		// wantUnreadable is a substring naming the unreadable side.
		wantUnreadable string
	}{
		{
			name:             "clean tree reports nothing",
			in:               wallIn(wallChangesHeadSpec),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{},
		},
		{
			name:             "typed-only edit",
			in:               wallIn(wallChangesTypedOnly),
			wantTyped:        []string{"ac-1"},
			wantUnclassified: []wallUnclassifiedChange{},
		},
		{
			name:             "prose-only edit",
			in:               wallIn(wallChangesProseOnly),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestSpecPath, wallReasonProse)},
		},
		{
			name:             "layout-only edit",
			in:               wallIn(wallChangesHeadSpec, wallChangedPath{Path: wallTestLayoutPath}),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestLayoutPath, wallReasonLayout)},
		},
		{
			name:             "an untracked layout is still layout",
			in:               wallIn(wallChangesHeadSpec, wallChangedPath{Path: wallTestLayoutPath, Untracked: true}),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestLayoutPath, wallReasonLayout)},
		},
		{
			name:             "another staged path",
			in:               wallIn(wallChangesHeadSpec, wallChangedPath{Path: wallTestOtherSpec}),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestOtherSpec, wallReasonStagedPath)},
		},
		{
			name:             "an untracked file",
			in:               wallIn(wallChangesHeadSpec, notes),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry("notes.txt", wallReasonUntracked)},
		},
		{
			name:      "mixed tree",
			in:        wallIn(wallChangesMixed, wallChangedPath{Path: wallTestLayoutPath}, wallChangedPath{Path: wallTestOtherSpec}, notes),
			wantTyped: []string{"ac-1"},
			wantUnclassified: []wallUnclassifiedChange{
				entry(wallTestOtherSpec, wallReasonStagedPath),
				entry(wallTestLayoutPath, wallReasonLayout),
				entry(wallTestSpecPath, wallReasonProse),
				entry("notes.txt", wallReasonUntracked),
			},
		},
		{
			name:             "a path git names twice for one reason is listed once",
			in:               wallIn(wallChangesHeadSpec, wallChangedPath{Path: wallTestOtherSpec}, wallChangedPath{Path: wallTestOtherSpec}),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestOtherSpec, wallReasonStagedPath)},
		},
		{
			name:             "SI-298: a title-only edit",
			in:               wallIn(wallChangesTitleOnly),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		},
		{
			name:             "SI-298: an owners-only edit",
			in:               wallIn(wallChangesOwnersOnly),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		},
		{
			name:             "SI-298: impacts added",
			in:               wallIn(wallChangesImpactsAdded),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		},
		{
			name:             "SI-298: a quoting-only edit",
			in:               wallIn(wallChangesQuotingOnly),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		},
		{
			name:             "SI-298: a mode-only change",
			in:               withStatus(wallIn(wallChangesHeadSpec), wallSpecStatus{Reported: true, ModeChanged: true}),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		},
		{
			name:             "SI-298: a mode change beside a typed edit",
			in:               withStatus(wallIn(wallChangesTypedOnly), wallSpecStatus{Reported: true, ModeChanged: true}),
			wantTyped:        []string{"ac-1"},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		},
		{
			name:             "SI-298: a typed edit beside a diverged index lists both",
			in:               withStatus(wallIn(wallChangesTypedOnly), wallSpecStatus{Reported: true, IndexDiverged: true}),
			wantTyped:        []string{"ac-1"},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		},
		{
			name:             "SI-298: a byte difference git does not report (an end-of-line filter) lists nothing",
			in:               withStatus(wallIn(strings.ReplaceAll(wallChangesHeadSpec, "\n", "\r\n")), wallSpecStatus{}),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{},
		},
		{
			name:             "SI-298: a typed edit git does not report lists nothing, and other changes are still listed",
			in:               withStatus(wallIn(wallChangesTypedOnly, wallChangedPath{Path: "notes.txt", Untracked: true}), wallSpecStatus{}),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry("notes.txt", wallReasonUntracked)},
		},
		{
			name:             "SI-298: a spec staged then reverted",
			in:               withStatus(wallIn(wallChangesHeadSpec), wallSpecStatus{Reported: true, IndexDiverged: true}),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		},
		{
			name:             "SI-298: git reports the spec while its bytes match HEAD",
			in:               withStatus(wallIn(wallChangesHeadSpec), wallSpecStatus{Reported: true}),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		},
		{
			name:             "SI-298: a title edit beside a typed edit lists both",
			in:               wallIn(wallChangesTitleAndTyped),
			wantTyped:        []string{"ac-1"},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		},
		{
			name:             "SI-298: a title edit beside a prose edit lists both",
			in:               wallIn(strings.Replace(wallChangesProseOnly, "title: Wall fixture\n", "title: Wall fixture renamed\n", 1)),
			wantTyped:        []string{},
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestSpecPath, wallReasonProse), entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		},
		{
			name:           "unreadable: an undecodable HEAD spec names HEAD and still lists other changes",
			in:             withHead(wallIn(wallChangesHeadSpec, notes), true, wallChangesUndecodable),
			wantUnreadable: "HEAD's spec.md could not be read",
			wantUnclassified: []wallUnclassifiedChange{
				entry(wallTestSpecPath, wallReasonUnrecognizedSpec),
				entry("notes.txt", wallReasonUntracked),
			},
		},
		{
			name:           "unreadable: an undecodable working-tree spec names the working tree and still lists other changes",
			in:             wallIn(wallChangesUndecodable, notes),
			wantUnreadable: "the working tree's spec.md could not be read",
			wantUnclassified: []wallUnclassifiedChange{
				entry(wallTestSpecPath, wallReasonUnrecognizedSpec),
				entry("notes.txt", wallReasonUntracked),
			},
		},
		{
			name:           "unreadable: a missing spec, untracked, still lists other changes",
			in:             withStatus(withHead(wallIn(wallChangesHeadSpec, notes), false, ""), wallSpecStatus{Reported: true, Untracked: true}),
			wantUnreadable: "has no revision at HEAD",
			wantUnclassified: []wallUnclassifiedChange{
				entry(wallTestSpecPath, wallReasonUntracked),
				entry("notes.txt", wallReasonUntracked),
			},
		},
		{
			name:             "unreadable: a missing spec, staged as new",
			in:               withStatus(withHead(wallIn(wallChangesHeadSpec), false, ""), wallSpecStatus{Reported: true}),
			wantUnreadable:   "has no revision at HEAD",
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		},
		{
			name:             "unreadable: a missing spec git does not report (ignored)",
			in:               withStatus(withHead(wallIn(wallChangesHeadSpec), false, ""), wallSpecStatus{}),
			wantUnreadable:   "has no revision at HEAD",
			wantUnclassified: []wallUnclassifiedChange{},
		},
		{
			name: "unreadable: an unborn HEAD",
			in: func() wallChangesInputs {
				in := withStatus(withHead(wallIn(wallChangesHeadSpec), false, ""), wallSpecStatus{Reported: true, Untracked: true})
				in.HeadUnborn = true
				return in
			}(),
			wantUnreadable:   "HEAD does not name a commit yet",
			wantUnclassified: []wallUnclassifiedChange{entry(wallTestSpecPath, wallReasonUntracked)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := classifyWallChanges(tc.in)
			if err != nil {
				t.Fatalf("classifyWallChanges: %v", err)
			}
			if tc.wantUnreadable == "" {
				if got.UnreadableReason != "" {
					t.Fatalf("UnreadableReason = %q, want a readable comparison", got.UnreadableReason)
				}
				if got.Typed == nil {
					t.Fatal("Typed = nil, want a list (possibly empty) for a readable comparison")
				}
				if targets := typedTargets(got); !reflect.DeepEqual(targets, tc.wantTyped) {
					t.Fatalf("typed targets = %v, want %v", targets, tc.wantTyped)
				}
			} else {
				if !strings.Contains(got.UnreadableReason, tc.wantUnreadable) {
					t.Fatalf("UnreadableReason = %q, want it to name %q", got.UnreadableReason, tc.wantUnreadable)
				}
				if got.Typed != nil {
					t.Fatalf("Typed = %+v, want nil: an unreadable comparison carries its reason instead (dc-2)", got.Typed)
				}
			}
			if !reflect.DeepEqual(got.Unclassified, tc.wantUnclassified) {
				t.Fatalf("Unclassified = %+v\nwant           %+v", got.Unclassified, tc.wantUnclassified)
			}
		})
	}
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
		want wallUnclassifiedChange
	}{
		{name: "prose-only", in: wallIn(wallChangesProseOnly), want: entry(wallTestSpecPath, wallReasonProse)},
		{name: "layout-only", in: wallIn(wallChangesHeadSpec, wallChangedPath{Path: wallTestLayoutPath}), want: entry(wallTestLayoutPath, wallReasonLayout)},
		{name: "another-staged-path", in: wallIn(wallChangesHeadSpec, wallChangedPath{Path: wallTestOtherSpec}), want: entry(wallTestOtherSpec, wallReasonStagedPath)},
		{name: "untracked-file", in: wallIn(wallChangesHeadSpec, wallChangedPath{Path: "notes.txt", Untracked: true}), want: entry("notes.txt", wallReasonUntracked)},
		{name: "title-only", in: wallIn(wallChangesTitleOnly), want: entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		{name: "quoting-only", in: wallIn(wallChangesQuotingOnly), want: entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		{name: "mode-only", in: withStatus(wallIn(wallChangesHeadSpec), wallSpecStatus{Reported: true, ModeChanged: true}), want: entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
		{name: "staged-then-reverted", in: withStatus(wallIn(wallChangesHeadSpec), wallSpecStatus{Reported: true, IndexDiverged: true}), want: entry(wallTestSpecPath, wallReasonUnrecognizedSpec)},
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
			if e := findUnclassified(got.Unclassified, tc.want.Path); e == nil || *e != tc.want {
				t.Fatalf("Unclassified = %+v, want an entry %+v", got.Unclassified, tc.want)
			}
		})
	}
}

// wallCoverageBase exercises every field the semantic snapshot covers.
const wallCoverageBase = `---
id: spec/wall-coverage
kind: spec
class: feature
title: Coverage
owners: [platform-team]
links: [ { type: depends-on, ref: spec/base } ]
problem: { text: "old problem", anchor: "#problem" }
outcome: { text: "old outcome", anchor: "#outcome" }
context: [spec/base@abcdef0]
acceptance_criteria:
  - { id: ac-1, text: "first", evidence: [static], anchor: "#ac-1" }
constraints:
  - { id: co-1, text: "bounded", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "reuse base", anchor: "#dc-1", links: [ { type: depends-on, ref: spec/base } ] }
open_questions:
  - { id: oq-1, text: "which signal?", anchor: "#oq-1" }
stubs:
  - { slug: first-story, acceptance_criteria: [ac-1] }
---
# Coverage
`

// TestWallChanges_SnapshotCoverage pins outsideSemanticSnapshot to the
// semantic snapshot in both directions: an edit to every field it clears
// yields a typed operation and leaves the remainder equal (so clearing it
// hides nothing the diff does not type), and an edit to a field it keeps
// yields no typed operation and changes the remainder (so it is listed).
func TestWallChanges_SnapshotCoverage(t *testing.T) {
	cases := []struct {
		name, old, new string
		covered        bool
	}{
		{name: "problem", old: `"old problem"`, new: `"new problem"`, covered: true},
		{name: "outcome", old: `"old outcome"`, new: `"new outcome"`, covered: true},
		{name: "acceptance criteria", old: `text: "first"`, new: `text: "first, revised"`, covered: true},
		{name: "constraints", old: `"bounded"`, new: `"tightly bounded"`, covered: true},
		{name: "decisions", old: `"reuse base"`, new: `"reuse the base"`, covered: true},
		{name: "a decision's links", old: `"#dc-1", links: [ { type: depends-on, ref: spec/base } ]`, new: `"#dc-1", links: [ { type: depends-on, ref: spec/other } ]`, covered: true},
		{name: "open questions", old: `"which signal?"`, new: `"which signal, exactly?"`, covered: true},
		{name: "stubs", old: "  - { slug: first-story, acceptance_criteria: [ac-1] }\n", new: "  - { slug: first-story, acceptance_criteria: [ac-1] }\n  - { slug: second-story, acceptance_criteria: [ac-1] }\n", covered: true},
		{name: "context", old: "context: [spec/base@abcdef0]", new: "context: [spec/base@abcdef0, adr/choice@abcdef1]", covered: true},
		{name: "links", old: "links: [ { type: depends-on, ref: spec/base } ]\n", new: "links: [ { type: depends-on, ref: spec/base }, { type: depends-on, ref: spec/other } ]\n", covered: true},
		{name: "title", old: "title: Coverage", new: "title: Coverage renamed"},
		{name: "owners", old: "owners: [platform-team]", new: "owners: [platform-team, design-team]"},
		{name: "impacts", old: "owners: [platform-team]\n", new: "owners: [platform-team]\nimpacts: [loansvc]\n"},
	}
	head := []byte(wallCoverageBase)
	headRev, err := readSpecRevision(head)
	if err != nil {
		t.Fatalf("coverage base does not decode: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Count(wallCoverageBase, tc.old) != 1 {
				t.Fatalf("edit anchor %q is not unique in the base", tc.old)
			}
			working := []byte(strings.Replace(wallCoverageBase, tc.old, tc.new, 1))
			workingRev, err := readSpecRevision(working)
			if err != nil {
				t.Fatalf("edited spec does not decode: %v", err)
			}
			typed, err := semanticDiffChanges(head, working)
			if err != nil {
				t.Fatalf("semanticDiffChanges: %v", err)
			}
			remainderEqual := reflect.DeepEqual(outsideSemanticSnapshot(headRev.spec), outsideSemanticSnapshot(workingRev.spec))
			if tc.covered && (len(typed) == 0 || !remainderEqual) {
				t.Fatalf("covered field: typed = %+v, remainder equal = %v; want a typed operation and an equal remainder", typed, remainderEqual)
			}
			if !tc.covered && (len(typed) != 0 || remainderEqual) {
				t.Fatalf("uncovered field: typed = %+v, remainder equal = %v; want no typed operation and a changed remainder", typed, remainderEqual)
			}
		})
	}
}

// TestSemanticDiffChanges is semanticDiffChanges' table: it returns
// exactly internal/draftmutation's own Diff (dc-1: reused, never
// re-derived) and surfaces an undecodable side as an error.
func TestSemanticDiffChanges(t *testing.T) {
	cases := []struct {
		name          string
		before, after string
		wantErr       bool
	}{
		{name: "a typed edit", before: wallChangesHeadSpec, after: wallChangesTypedOnly},
		{name: "a title edit types nothing", before: wallChangesHeadSpec, after: wallChangesTitleOnly},
		{name: "identical specs", before: wallChangesHeadSpec, after: wallChangesHeadSpec},
		{name: "an undecodable before", before: wallChangesUndecodable, after: wallChangesHeadSpec, wantErr: true},
		{name: "an undecodable after", before: wallChangesHeadSpec, after: wallChangesUndecodable, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := semanticDiffChanges([]byte(tc.before), []byte(tc.after))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("semanticDiffChanges = %+v, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("semanticDiffChanges: %v", err)
			}
			want, _, err := draftmutation.Diff([]byte(tc.before), []byte(tc.after))
			if err != nil {
				t.Fatalf("draftmutation.Diff: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("semanticDiffChanges = %+v, want draftmutation.Diff's %+v", got, want)
			}
		})
	}
}

// TestWallChanges_WireShape pins the summary's JSON: keys, list-vs-absent,
// and the reason literals the Commit and push popover consumes.
func TestWallChanges_WireShape(t *testing.T) {
	change := designprovenance.Change{Target: "ac-1", Change: designprovenance.ChangeReplaced, BeforeDigest: "sha256:a", AfterDigest: "sha256:b"}
	cases := []struct {
		name string
		in   wallChanges
		want string
	}{
		{name: "a readable empty summary carries empty lists", in: wallChanges{Typed: []designprovenance.Change{}, Unclassified: []wallUnclassifiedChange{}}, want: `{"typed":[],"unclassified":[]}`},
		{name: "a readable summary with nil lists still carries lists", in: wallChanges{}, want: `{"typed":[],"unclassified":[]}`},
		{
			name: "a readable summary carries typed and unclassified side by side",
			in:   wallChanges{Typed: []designprovenance.Change{change}, Unclassified: []wallUnclassifiedChange{entry("a", wallReasonProse), entry("b", wallReasonLayout), entry("c", wallReasonStagedPath), entry("d", wallReasonUntracked), entry("e", wallReasonUnrecognizedSpec)}},
			want: `{"typed":[{"target":"ac-1","change":"replaced","before_digest":"sha256:a","after_digest":"sha256:b"}],"unclassified":[{"path":"a","reason":"prose-or-body-text"},{"path":"b","reason":"layout"},{"path":"c","reason":"another-staged-path"},{"path":"d","reason":"untracked-file"},{"path":"e","reason":"unrecognized-spec-change"}]}`,
		},
		{
			name: "an unreadable summary carries its reason instead of a typed list",
			in:   wallChanges{Typed: []designprovenance.Change{change}, UnreadableReason: "HEAD's spec.md could not be read"},
			want: `{"unclassified":[],"unreadableReason":"HEAD's spec.md could not be read"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(&tc.in)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("wire = %s\nwant   %s", got, tc.want)
			}
		})
	}
}

// fakeWallGit is a wallGitReader whose every answer is canned, so each git
// failure path of loadWallChangesInputs can be driven on its own.
type fakeWallGit struct {
	loc          gitx.Location
	locErr       error
	exists       bool
	existsErr    error
	resolves     bool
	resolvesErr  error
	show         []byte
	showErr      error
	tracked      []gitx.TrackedChange
	trackedErr   error
	untracked    []string
	untrackedErr error
}

func (f fakeWallGit) Locate(context.Context, string) (gitx.Location, error) { return f.loc, f.locErr }
func (f fakeWallGit) PathExistsAt(context.Context, string, string, string) (bool, error) {
	return f.exists, f.existsErr
}
func (f fakeWallGit) CommitExists(context.Context, string, string) (bool, error) {
	return f.resolves, f.resolvesErr
}
func (f fakeWallGit) Show(context.Context, string, string, string) ([]byte, error) {
	return f.show, f.showErr
}
func (f fakeWallGit) TrackedChanges(context.Context, string) ([]gitx.TrackedChange, error) {
	return f.tracked, f.trackedErr
}
func (f fakeWallGit) UntrackedPaths(context.Context, string) ([]string, error) {
	return f.untracked, f.untrackedErr
}

// TestWallChanges_LoadInputsGitFailures drives every git-failure path of
// loadWallChangesInputs: each is an operational error naming its step —
// except a HEAD that names no commit, which is the disclosed
// missing-at-HEAD state, never an error.
func TestWallChanges_LoadInputsGitFailures(t *testing.T) {
	boom := errors.New("git failed")
	ok := fakeWallGit{loc: gitx.Location{TopLevel: "/repo"}, exists: true, resolves: true, show: []byte(wallChangesHeadSpec)}
	cases := []struct {
		name       string
		git        func() fakeWallGit
		wantErr    string
		wantUnborn bool
	}{
		{name: "locating the repository fails", git: func() fakeWallGit { f := ok; f.locErr = boom; return f }, wantErr: "locating the repository"},
		{name: "ls-tree fails while HEAD resolves", git: func() fakeWallGit { f := ok; f.existsErr = boom; return f }, wantErr: "checking HEAD for"},
		{name: "ls-tree fails and the HEAD check fails too", git: func() fakeWallGit {
			f := ok
			f.existsErr, f.resolvesErr = boom, errors.New("rev-parse failed")
			return f
		}, wantErr: "rev-parse failed"},
		{name: "ls-tree fails because HEAD names no commit", git: func() fakeWallGit { f := ok; f.existsErr, f.resolves = boom, false; return f }, wantUnborn: true},
		{name: "reading HEAD's spec fails", git: func() fakeWallGit { f := ok; f.showErr = boom; return f }, wantErr: "reading HEAD's"},
		{name: "listing changed paths fails", git: func() fakeWallGit { f := ok; f.trackedErr = boom; return f }, wantErr: "listing changed paths"},
		{name: "listing untracked paths fails", git: func() fakeWallGit { f := ok; f.untrackedErr = boom; return f }, wantErr: "listing untracked paths"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in, err := loadWallChangesInputs(context.Background(), tc.git(), "/repo", "wall-fixture", []byte(wallChangesHeadSpec))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want one naming %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("loadWallChangesInputs: %v", err)
			}
			if in.HeadUnborn != tc.wantUnborn || in.HeadExists {
				t.Fatalf("HeadUnborn = %v HeadExists = %v, want unborn %v and no HEAD revision", in.HeadUnborn, in.HeadExists, tc.wantUnborn)
			}
		})
	}
}

// TestWallChanges_LoadInputsMapsStatus is loadWallChangesInputs' mapping
// table: every path resolved against the top level with the store's
// prefix, the spec's own status facts, and every other path — both sides
// of a rename included — listed once.
func TestWallChanges_LoadInputsMapsStatus(t *testing.T) {
	const prefix = "store/"
	spec := prefix + wallTestSpecPath
	base := fakeWallGit{loc: gitx.Location{TopLevel: "/repo", Prefix: prefix}, exists: true, resolves: true, show: []byte(wallChangesHeadSpec)}
	cases := []struct {
		name        string
		tracked     []gitx.TrackedChange
		untracked   []string
		wantStatus  wallSpecStatus
		wantChanged []wallChangedPath
	}{
		{name: "a clean tree", wantStatus: wallSpecStatus{}},
		{
			name:       "an unstaged spec edit",
			tracked:    []gitx.TrackedChange{{Path: spec, Index: '.', Worktree: 'M', ModeHead: "100644", ModeWorktree: "100644"}},
			wantStatus: wallSpecStatus{Reported: true},
		},
		{
			name:       "a spec staged then edited again diverges the index",
			tracked:    []gitx.TrackedChange{{Path: spec, Index: 'M', Worktree: 'M', ModeHead: "100644", ModeWorktree: "100644"}},
			wantStatus: wallSpecStatus{Reported: true, IndexDiverged: true},
		},
		{
			name:       "an unmerged spec diverges the index",
			tracked:    []gitx.TrackedChange{{Path: spec, Index: 'U', Worktree: 'U', Unmerged: true, ModeWorktree: "100644"}},
			wantStatus: wallSpecStatus{Reported: true, IndexDiverged: true},
		},
		{
			name:       "a spec mode change",
			tracked:    []gitx.TrackedChange{{Path: spec, Index: '.', Worktree: 'M', ModeHead: "100644", ModeWorktree: "100755"}},
			wantStatus: wallSpecStatus{Reported: true, ModeChanged: true},
		},
		{
			name:        "the spec renamed away is reported, never listed as another path",
			tracked:     []gitx.TrackedChange{{Path: prefix + "moved.md", RenamedFrom: spec, Index: 'R', Worktree: '.', ModeHead: "100644", ModeWorktree: "100644"}},
			wantStatus:  wallSpecStatus{Reported: true},
			wantChanged: []wallChangedPath{{Path: prefix + "moved.md"}},
		},
		{
			name:       "an untracked spec",
			untracked:  []string{spec},
			wantStatus: wallSpecStatus{Reported: true, Untracked: true},
		},
		{
			name:        "other paths: an edit, both sides of a rename, and untracked files",
			tracked:     []gitx.TrackedChange{{Path: "outside.md", Index: '.', Worktree: 'M'}, {Path: prefix + "new.md", RenamedFrom: prefix + "old.md", Index: 'R', Worktree: '.'}},
			untracked:   []string{"top.txt", prefix + "notes.txt"},
			wantChanged: []wallChangedPath{{Path: "outside.md"}, {Path: prefix + "new.md"}, {Path: prefix + "old.md"}, {Path: "top.txt", Untracked: true}, {Path: prefix + "notes.txt", Untracked: true}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			git := base
			git.tracked, git.untracked = tc.tracked, tc.untracked
			in, err := loadWallChangesInputs(context.Background(), git, "/repo/store", "wall-fixture", []byte(wallChangesHeadSpec))
			if err != nil {
				t.Fatalf("loadWallChangesInputs: %v", err)
			}
			if in.SpecPath != spec || in.LayoutPath != prefix+wallTestLayoutPath {
				t.Fatalf("SpecPath = %q LayoutPath = %q, want both under the store prefix %q", in.SpecPath, in.LayoutPath, prefix)
			}
			if !in.HeadExists || string(in.HeadSpec) != wallChangesHeadSpec {
				t.Fatalf("HEAD revision not carried: exists = %v", in.HeadExists)
			}
			if in.SpecStatus != tc.wantStatus {
				t.Fatalf("SpecStatus = %+v, want %+v", in.SpecStatus, tc.wantStatus)
			}
			if !reflect.DeepEqual(in.ChangedPaths, tc.wantChanged) {
				t.Fatalf("ChangedPaths = %+v, want %+v", in.ChangedPaths, tc.wantChanged)
			}
		})
	}
}
