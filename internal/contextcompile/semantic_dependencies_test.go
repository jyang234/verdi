package contextcompile

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// boundTargetMultiBytes declares two acceptance criteria whose evidence
// kinds enumerate to three obligation pairs, deliberately out of the
// canonical Ref sort order (ac-2's kinds precede ac-1's kind in the
// frontmatter, and within ac-2 "runtime" precedes "static" — the opposite
// of the alphabetical order the resolver must return).
const boundTargetMultiBytes = `---
id: spec/example-story
kind: spec
title: Example story
owners: [platform-team]
class: story
story: jira:EX-1
problem: {text: "The example is missing.", anchor: problem}
outcome: {text: "The example exists.", anchor: outcome}
acceptance_criteria:
  - {id: ac-2, text: "second ac", evidence: [runtime, static], anchor: ac-2}
  - {id: ac-1, text: "first ac", evidence: [behavioral], anchor: ac-1}
links:
  - {type: implements, ref: spec/example-feature#ac-1}
---
# Example story

## Problem

Missing.

## Outcome

Exists.

## AC-1

First.

## AC-2

Second.
`

// boundTargetSingleBytes declares exactly one acceptance criterion with one
// evidence kind: the minimal fixture for single-pair test cases.
const boundTargetSingleBytes = `---
id: spec/example-story
kind: spec
title: Example story
owners: [platform-team]
class: story
story: jira:EX-1
problem: {text: "The example is missing.", anchor: problem}
outcome: {text: "The example exists.", anchor: outcome}
acceptance_criteria:
  - {id: ac-1, text: "first ac", evidence: [behavioral], anchor: ac-1}
links:
  - {type: implements, ref: spec/example-feature#ac-1}
---
# Example story

## Problem

Missing.

## Outcome

Exists.

## AC-1

First.
`

func decodeBoundTargetFixture(t *testing.T, data string) *artifact.SpecFrontmatter {
	t.Helper()
	fmBytes, _, err := artifact.SplitFrontmatter([]byte(data))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	return spec
}

func boundTargetMulti(t *testing.T) ResolvedSpec {
	t.Helper()
	return ResolvedSpec{
		Ref:           "spec/example-story",
		Path:          ".verdi/specs/active/example-story/spec.md",
		Blob:          strings.Repeat("d", 40),
		Commit:        strings.Repeat("e", 40),
		ContentDigest: rawContentDigest([]byte(boundTargetMultiBytes)),
		Content:       []byte(boundTargetMultiBytes),
		Spec:          decodeBoundTargetFixture(t, boundTargetMultiBytes),
		State:         specstate.AcceptedPendingBuild,
	}
}

func boundTargetSingle(t *testing.T) ResolvedSpec {
	t.Helper()
	return ResolvedSpec{
		Ref:           "spec/example-story",
		Path:          ".verdi/specs/active/example-story/spec.md",
		Blob:          strings.Repeat("d", 40),
		Commit:        strings.Repeat("e", 40),
		ContentDigest: rawContentDigest([]byte(boundTargetSingleBytes)),
		Content:       []byte(boundTargetSingleBytes),
		Spec:          decodeBoundTargetFixture(t, boundTargetSingleBytes),
		State:         specstate.AcceptedPendingBuild,
	}
}

// obligationBytes builds a strictly-decodable obligation document: id/for_kind
// declared by the caller, exactly one verifies edge to verifiesRef.
func obligationBytes(id, forKind, verifiesRef string) []byte {
	return []byte("---\n" +
		"id: " + id + "\n" +
		"kind: obligation\n" +
		"title: \"Bound obligation\"\n" +
		"owners: [platform-team]\n" +
		"for_kind: " + forKind + "\n" +
		"links:\n" +
		"  - { type: verifies, ref: \"" + verifiesRef + "\" }\n" +
		"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n" +
		"---\n" +
		"Bound obligation body.\n")
}

func singlePairEntry(path, object string) []gitx.TreeEntry {
	return []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: object, Path: path}}
}

func TestResolveBoundObligations_Happy(t *testing.T) {
	root, head := "/repo", strings.Repeat("a", 40)
	target := boundTargetMulti(t)

	pathAC1Behavioral := store.ObligationPath("", "example-story", "ac-1", "behavioral")
	pathAC2Runtime := store.ObligationPath("", "example-story", "ac-2", "runtime")
	pathAC2Static := store.ObligationPath("", "example-story", "ac-2", "static")

	contentAC1Behavioral := obligationBytes("obligation/example-story--ac-1--behavioral", "behavioral", "spec/example-story")
	contentAC2Runtime := obligationBytes("obligation/example-story--ac-2--runtime", "runtime", "spec/example-story")
	contentAC2Static := obligationBytes("obligation/example-story--ac-2--static", "static", "spec/example-story")

	// Tree entries are listed in a scrambled (non-canonical-Ref-sorted)
	// order to prove the resolver, not the fake, produces canonical order.
	entries := []gitx.TreeEntry{
		{Mode: "100644", Type: "blob", Object: strings.Repeat("3", 40), Path: pathAC2Static},
		{Mode: "100644", Type: "blob", Object: strings.Repeat("1", 40), Path: pathAC1Behavioral},
		{Mode: "100644", Type: "blob", Object: strings.Repeat("2", 40), Path: pathAC2Runtime},
	}
	contentByPath := map[string][]byte{
		pathAC1Behavioral: contentAC1Behavioral,
		pathAC2Runtime:    contentAC2Runtime,
		pathAC2Static:     contentAC2Static,
	}

	var treeCalls, showCalls int
	git := authorityGit{
		tree: func(_ context.Context, gotRoot, gotHead string) ([]gitx.TreeEntry, error) {
			treeCalls++
			if gotRoot != root || gotHead != head {
				t.Fatalf("LsTreeEntries(%q, %q), want (%q, %q)", gotRoot, gotHead, root, head)
			}
			return append([]gitx.TreeEntry(nil), entries...), nil
		},
		show: func(_ context.Context, gotRoot, gotHead, gotPath string) ([]byte, error) {
			showCalls++
			if gotRoot != root || gotHead != head {
				t.Fatalf("Show(%q, %q, %q), want root=%q head=%q", gotRoot, gotHead, gotPath, root, head)
			}
			data, ok := contentByPath[gotPath]
			if !ok {
				t.Fatalf("Show called for unexpected path %q", gotPath)
			}
			return append([]byte(nil), data...), nil
		},
	}

	got, err := ResolveBoundObligations(context.Background(), git, root, head, target)
	if err != nil {
		t.Fatalf("ResolveBoundObligations: %v", err)
	}
	if treeCalls != 1 {
		t.Fatalf("LsTreeEntries called %d times, want exactly 1 (reused across pairs)", treeCalls)
	}
	if showCalls != 3 {
		t.Fatalf("Show called %d times, want 3 (one per present pair)", showCalls)
	}
	if got == nil {
		t.Fatal("ResolveBoundObligations returned a nil slice, want non-nil")
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}

	wantRefs := []string{
		"obligation/example-story--ac-1--behavioral",
		"obligation/example-story--ac-2--runtime",
		"obligation/example-story--ac-2--static",
	}
	for i, want := range wantRefs {
		if got[i].Ref != want {
			t.Fatalf("got[%d].Ref = %q, want %q (canonical Ref-sorted order)", i, got[i].Ref, want)
		}
	}
	if got[0].Path != pathAC1Behavioral || got[0].AC != "ac-1" || got[0].Kind != artifact.EvidenceBehavioral || got[0].TargetRef != target.Ref {
		t.Fatalf("got[0] = %+v", got[0])
	}
	if got[0].ContentDigest != rawContentDigest(contentAC1Behavioral) {
		t.Fatalf("got[0].ContentDigest = %q, want %q", got[0].ContentDigest, rawContentDigest(contentAC1Behavioral))
	}
	if !bytes.Equal(got[0].Content, contentAC1Behavioral) {
		t.Fatalf("got[0].Content = %q, want %q", got[0].Content, contentAC1Behavioral)
	}
	if got[1].Path != pathAC2Runtime || got[1].AC != "ac-2" || got[1].Kind != artifact.EvidenceRuntime {
		t.Fatalf("got[1] = %+v", got[1])
	}
	if got[2].Path != pathAC2Static || got[2].AC != "ac-2" || got[2].Kind != artifact.EvidenceStatic {
		t.Fatalf("got[2] = %+v", got[2])
	}
}

func TestResolveBoundObligations_AbsentPathIsEmptyNonNil(t *testing.T) {
	root, head := "/repo", strings.Repeat("a", 40)
	target := boundTargetSingle(t)

	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			return nil, nil // no obligation on the exact HEAD tree at all
		},
		show: func(context.Context, string, string, string) ([]byte, error) {
			panic("unexpected Show call for an absent obligation path")
		},
	}

	got, err := ResolveBoundObligations(context.Background(), git, root, head, target)
	if err != nil {
		t.Fatalf("ResolveBoundObligations: %v", err)
	}
	if got == nil {
		t.Fatal("ResolveBoundObligations returned a nil slice, want an explicit empty non-nil slice")
	}
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
}

func TestResolveBoundObligations_MalformedFrontmatter(t *testing.T) {
	root, head := "/repo", strings.Repeat("a", 40)
	target := boundTargetSingle(t)
	path := store.ObligationPath("", "example-story", "ac-1", "behavioral")

	// unknown top-level field trips strict decode's KnownFields(true).
	content := []byte("---\n" +
		"id: obligation/example-story--ac-1--behavioral\n" +
		"kind: obligation\n" +
		"title: \"Bound obligation\"\n" +
		"owners: [platform-team]\n" +
		"for_kind: behavioral\n" +
		"unexpected: true\n" +
		"links:\n" +
		"  - { type: verifies, ref: \"spec/example-story\" }\n" +
		"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n" +
		"---\n" +
		"Body.\n")

	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			return singlePairEntry(path, strings.Repeat("1", 40)), nil
		},
		show: func(context.Context, string, string, string) ([]byte, error) {
			return append([]byte(nil), content...), nil
		},
	}

	got, err := ResolveBoundObligations(context.Background(), git, root, head, target)
	if err == nil {
		t.Fatal("ResolveBoundObligations: want operational error, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("malformed frontmatter classified as refusal: %T %v", err, err)
	}
	if got != nil {
		t.Fatalf("ResolveBoundObligations result = %#v, want nil on error", got)
	}
}

func TestResolveBoundObligations_WrongIDPathPair(t *testing.T) {
	root, head := "/repo", strings.Repeat("a", 40)
	target := boundTargetSingle(t)
	path := store.ObligationPath("", "example-story", "ac-1", "behavioral")
	// for_kind matches the pair's kind, but the id names a different AC.
	content := obligationBytes("obligation/example-story--ac-2--behavioral", "behavioral", "spec/example-story")

	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			return singlePairEntry(path, strings.Repeat("1", 40)), nil
		},
		show: func(context.Context, string, string, string) ([]byte, error) {
			return append([]byte(nil), content...), nil
		},
	}

	got, gotErr := ResolveBoundObligations(context.Background(), git, root, head, target)
	err := mustOperationalError(t, got, gotErr)
	if !strings.Contains(err.Error(), "id") {
		t.Fatalf("error = %q, want it to name the id offense", err.Error())
	}
}

func TestResolveBoundObligations_WrongForKind(t *testing.T) {
	root, head := "/repo", strings.Repeat("a", 40)
	target := boundTargetSingle(t)
	path := store.ObligationPath("", "example-story", "ac-1", "behavioral")
	// id/for_kind are internally consistent (runtime), but resolved against
	// the ac-1/behavioral pair.
	content := obligationBytes("obligation/example-story--ac-1--runtime", "runtime", "spec/example-story")

	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			return singlePairEntry(path, strings.Repeat("1", 40)), nil
		},
		show: func(context.Context, string, string, string) ([]byte, error) {
			return append([]byte(nil), content...), nil
		},
	}

	got, gotErr := ResolveBoundObligations(context.Background(), git, root, head, target)
	err := mustOperationalError(t, got, gotErr)
	if !strings.Contains(err.Error(), "for_kind") {
		t.Fatalf("error = %q, want it to name the for_kind offense", err.Error())
	}
}

func TestResolveBoundObligations_WrongVerifiesTarget(t *testing.T) {
	root, head := "/repo", strings.Repeat("a", 40)
	target := boundTargetSingle(t)
	path := store.ObligationPath("", "example-story", "ac-1", "behavioral")
	content := obligationBytes("obligation/example-story--ac-1--behavioral", "behavioral", "spec/other-story")

	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			return singlePairEntry(path, strings.Repeat("1", 40)), nil
		},
		show: func(context.Context, string, string, string) ([]byte, error) {
			return append([]byte(nil), content...), nil
		},
	}

	got, gotErr := ResolveBoundObligations(context.Background(), git, root, head, target)
	err := mustOperationalError(t, got, gotErr)
	if !strings.Contains(err.Error(), "verifies") {
		t.Fatalf("error = %q, want it to name the verifies offense", err.Error())
	}
}

func TestResolveBoundObligations_WrongVerifiesCount(t *testing.T) {
	root, head := "/repo", strings.Repeat("a", 40)
	target := boundTargetSingle(t)
	path := store.ObligationPath("", "example-story", "ac-1", "behavioral")
	content := []byte("---\n" +
		"id: obligation/example-story--ac-1--behavioral\n" +
		"kind: obligation\n" +
		"title: \"Bound obligation\"\n" +
		"owners: [platform-team]\n" +
		"for_kind: behavioral\n" +
		"links:\n" +
		"  - { type: verifies, ref: \"spec/example-story\" }\n" +
		"  - { type: verifies, ref: \"spec/other-story\" }\n" +
		"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n" +
		"---\n" +
		"Body.\n")

	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			return singlePairEntry(path, strings.Repeat("1", 40)), nil
		},
		show: func(context.Context, string, string, string) ([]byte, error) {
			return append([]byte(nil), content...), nil
		},
	}

	got, gotErr := ResolveBoundObligations(context.Background(), git, root, head, target)
	_ = mustOperationalError(t, got, gotErr)
}

func TestResolveBoundObligations_FragmentVerifiesRef(t *testing.T) {
	root, head := "/repo", strings.Repeat("a", 40)
	target := boundTargetSingle(t)
	path := store.ObligationPath("", "example-story", "ac-1", "behavioral")
	content := obligationBytes("obligation/example-story--ac-1--behavioral", "behavioral", "spec/example-story#ac-1")

	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			return singlePairEntry(path, strings.Repeat("1", 40)), nil
		},
		show: func(context.Context, string, string, string) ([]byte, error) {
			return append([]byte(nil), content...), nil
		},
	}

	got, gotErr := ResolveBoundObligations(context.Background(), git, root, head, target)
	_ = mustOperationalError(t, got, gotErr)
}

func TestResolveBoundObligations_NonRegularEntry(t *testing.T) {
	root, head := "/repo", strings.Repeat("a", 40)
	target := boundTargetSingle(t)
	path := store.ObligationPath("", "example-story", "ac-1", "behavioral")

	tests := []struct {
		name  string
		entry gitx.TreeEntry
	}{
		{name: "symlink", entry: gitx.TreeEntry{Mode: "120000", Type: "blob", Object: strings.Repeat("1", 40), Path: path}},
		{name: "gitlink", entry: gitx.TreeEntry{Mode: "160000", Type: "commit", Object: strings.Repeat("1", 40), Path: path}},
		{name: "tree", entry: gitx.TreeEntry{Mode: "040000", Type: "tree", Object: strings.Repeat("1", 40), Path: path}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := authorityGit{
				tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
					return []gitx.TreeEntry{tt.entry}, nil
				},
				show: func(context.Context, string, string, string) ([]byte, error) {
					panic("unexpected Show call for a non-regular candidate")
				},
			}
			got, gotErr := ResolveBoundObligations(context.Background(), git, root, head, target)
			_ = mustOperationalError(t, got, gotErr)
		})
	}
}

func TestResolveBoundObligations_PortErrorsPropagateAsOperational(t *testing.T) {
	root, head := "/repo", strings.Repeat("a", 40)
	target := boundTargetSingle(t)
	path := store.ObligationPath("", "example-story", "ac-1", "behavioral")

	t.Run("tree error", func(t *testing.T) {
		want := errors.New("git ls-tree unavailable")
		git := authorityGit{
			tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) { return nil, want },
			show: func(context.Context, string, string, string) ([]byte, error) {
				panic("unexpected Show call after LsTreeEntries failure")
			},
		}
		_, err := ResolveBoundObligations(context.Background(), git, root, head, target)
		if !errors.Is(err, want) {
			t.Fatalf("ResolveBoundObligations error = %v, want wrapping %v", err, want)
		}
		if IsRefusal(err) {
			t.Fatalf("git error classified as refusal: %T %v", err, err)
		}
	})

	t.Run("show error", func(t *testing.T) {
		want := errors.New("git show unavailable")
		git := authorityGit{
			tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
				return singlePairEntry(path, strings.Repeat("1", 40)), nil
			},
			show: func(context.Context, string, string, string) ([]byte, error) { return nil, want },
		}
		_, err := ResolveBoundObligations(context.Background(), git, root, head, target)
		if !errors.Is(err, want) {
			t.Fatalf("ResolveBoundObligations error = %v, want wrapping %v", err, want)
		}
		if IsRefusal(err) {
			t.Fatalf("git error classified as refusal: %T %v", err, err)
		}
	})
}

func TestResolveBoundObligations_MalformedTarget(t *testing.T) {
	root, head := "/repo", strings.Repeat("a", 40)
	panicking := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			panic("unexpected LsTreeEntries call for a malformed target")
		},
		show: func(context.Context, string, string, string) ([]byte, error) {
			panic("unexpected Show call for a malformed target")
		},
	}

	tests := []struct {
		name   string
		target ResolvedSpec
	}{
		{name: "empty ref", target: ResolvedSpec{}},
		{name: "wrong ref grammar", target: ResolvedSpec{Ref: "story"}},
		{name: "fragment ref", target: ResolvedSpec{Ref: "spec/example-story#ac-1"}},
		{name: "nil decoded spec", target: ResolvedSpec{Ref: "spec/example-story"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveBoundObligations(context.Background(), panicking, root, head, tt.target)
			if err == nil {
				t.Fatal("ResolveBoundObligations: want operational error, got nil")
			}
			if IsRefusal(err) {
				t.Fatalf("malformed target classified as refusal: %T %v", err, err)
			}
			if got != nil {
				t.Fatalf("ResolveBoundObligations result = %#v, want nil on error", got)
			}
		})
	}
}

func TestResolveBoundObligations_NilGitPort(t *testing.T) {
	target := boundTargetSingle(t)
	_, err := ResolveBoundObligations(context.Background(), nil, "/repo", strings.Repeat("a", 40), target)
	if err == nil {
		t.Fatal("ResolveBoundObligations: want operational error for a nil git port, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("nil port classified as refusal: %T %v", err, err)
	}
}

// TestResolveBoundObligations_FreshBytes proves both that BoundObligation.Content
// is a fresh copy of git's returned buffer (never an alias) and that each
// call returns bytes independent of any previously returned result.
func TestResolveBoundObligations_FreshBytes(t *testing.T) {
	root, head := "/repo", strings.Repeat("a", 40)
	target := boundTargetSingle(t)
	path := store.ObligationPath("", "example-story", "ac-1", "behavioral")
	content := obligationBytes("obligation/example-story--ac-1--behavioral", "behavioral", "spec/example-story")
	buf := append([]byte(nil), content...)

	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			return singlePairEntry(path, strings.Repeat("1", 40)), nil
		},
		show: func(context.Context, string, string, string) ([]byte, error) {
			return buf, nil // deliberately not copied: aliasing would be observable
		},
	}

	got, err := ResolveBoundObligations(context.Background(), git, root, head, target)
	if err != nil {
		t.Fatalf("ResolveBoundObligations: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	original := append([]byte(nil), got[0].Content...)

	buf[0] ^= 0xFF
	if !bytes.Equal(got[0].Content, original) {
		t.Fatalf("mutating git's returned buffer leaked into BoundObligation.Content: got %q, want %q", got[0].Content, original)
	}
	buf[0] ^= 0xFF // restore

	got[0].Content[0] ^= 0xFF
	got2, err := ResolveBoundObligations(context.Background(), git, root, head, target)
	if err != nil {
		t.Fatalf("ResolveBoundObligations (second call): %v", err)
	}
	if !bytes.Equal(got2[0].Content, original) {
		t.Fatalf("second resolve = %q, want fresh unaffected bytes %q", got2[0].Content, original)
	}
}

// mustOperationalError asserts err is non-nil, not a refusal, and that
// result is nil, returning err for the caller's own message assertions.
func mustOperationalError(t *testing.T, result []BoundObligation, err error) error {
	t.Helper()
	if err == nil {
		t.Fatal("ResolveBoundObligations: want operational error, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("classified as refusal, want operational: %T %v", err, err)
	}
	if result != nil {
		t.Fatalf("ResolveBoundObligations result = %#v, want nil on error", result)
	}
	return err
}

// ============================================================================
// SI-92: exact pinned declared-context artifact and fragment resolution
// ============================================================================
//
// --- fixture builders --------------------------------------------------
//
// SI-92's declared-context resolver must strict-decode every closed-
// registry kind and check its decoded unpinned whole id against the ref's
// (kind, name) — these builders produce one minimal, individually valid
// document per kind, parameterized only by the bytes that must vary
// per-test (title/body filler distinguishes otherwise-identical commits
// for the two-commit divergent-bytes proof).

func adrBytes(name, filler string) []byte {
	return []byte("---\n" +
		"id: adr/" + name + "\n" +
		"kind: adr\n" +
		"title: \"ADR " + filler + "\"\n" +
		"owners: [platform-team]\n" +
		"status: proposed\n" +
		"---\n" +
		"Body " + filler + ".\n")
}

func diagramBytes(name, filler string) []byte {
	return []byte("---\n" +
		"id: diagram/" + name + "\n" +
		"kind: diagram\n" +
		"title: \"Diagram " + filler + "\"\n" +
		"owners: [platform-team]\n" +
		"status: active\n" +
		"---\n" +
		"Body " + filler + ".\n")
}

func attestationBytesDC(name, filler string) []byte {
	return []byte("---\n" +
		"id: attestation/" + name + "\n" +
		"kind: attestation\n" +
		"title: \"Attestation " + filler + "\"\n" +
		"owners: [platform-team]\n" +
		"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n" +
		"---\n" +
		"Body " + filler + ".\n")
}

func waiverBytesDC(name, filler string) []byte {
	return []byte("---\n" +
		"id: waiver/" + name + "\n" +
		"kind: waiver\n" +
		"title: \"Waiver " + filler + "\"\n" +
		"owners: [platform-team]\n" +
		"status: active\n" +
		"reason: \"Temporary.\"\n" +
		"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n" +
		"---\n" +
		"Body " + filler + ".\n")
}

func conflictBytesDC(name, filler string) []byte {
	return []byte("---\n" +
		"id: conflict/" + name + "\n" +
		"kind: conflict\n" +
		"title: \"Conflict " + filler + "\"\n" +
		"owners: [platform-team]\n" +
		"status: open\n" +
		"links:\n" +
		"  - { type: challenges, ref: \"adr/example-choice\" }\n" +
		"---\n" +
		"Body " + filler + ".\n")
}

func reaffirmationBytesDC(name, filler string) []byte {
	return []byte("---\n" +
		"id: reaffirmation/" + name + "\n" +
		"kind: reaffirmation\n" +
		"title: \"Reaffirmation " + filler + "\"\n" +
		"owners: [platform-team]\n" +
		"object: \"spec/example-feature@" + strings.Repeat("a", 40) + "#ac-1\"\n" +
		"hash: { old: \"sha256:" + strings.Repeat("1", 64) + "\", new: \"sha256:" + strings.Repeat("2", 64) + "\" }\n" +
		"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n" +
		"---\n" +
		"Body " + filler + ".\n")
}

func obligationBytesDC(name, filler string) []byte {
	return []byte("---\n" +
		"id: obligation/" + name + "\n" +
		"kind: obligation\n" +
		"title: \"Obligation " + filler + "\"\n" +
		"owners: [platform-team]\n" +
		"for_kind: behavioral\n" +
		"links:\n" +
		"  - { type: verifies, ref: \"spec/example-story\" }\n" +
		"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n" +
		"---\n" +
		"Body " + filler + ".\n")
}

// featureSpecBytesDC builds a class:feature fixture declaring contextRefs
// in its context: list, with acceptance criteria ac-1 and ac-2 declared (so
// a spec fragment ref can resolve #ac-1 and prove #ac-99/#problem/#outcome
// do not).
func featureSpecBytesDC(name string, contextRefs []string) []byte {
	ctxBlock := ""
	if len(contextRefs) > 0 {
		ctxBlock = "context:\n"
		for _, r := range contextRefs {
			ctxBlock += "  - \"" + r + "\"\n"
		}
	}
	return []byte("---\n" +
		"id: spec/" + name + "\n" +
		"kind: spec\n" +
		"class: feature\n" +
		"title: \"Feature " + name + "\"\n" +
		"owners: [platform-team]\n" +
		"problem: {text: \"Problem.\", anchor: problem}\n" +
		"outcome: {text: \"Outcome.\", anchor: outcome}\n" +
		"acceptance_criteria:\n" +
		"  - {id: ac-1, text: \"first\", evidence: [static], anchor: ac-1}\n" +
		"  - {id: ac-2, text: \"second\", evidence: [static], anchor: ac-2}\n" +
		ctxBlock +
		"---\n" +
		"# Feature " + name + "\n")
}

func decodeFeatureFixtureDC(t *testing.T, data []byte) *artifact.SpecFrontmatter {
	t.Helper()
	fmBytes, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	return spec
}

// featureTargetDC builds a ResolvedSpec for a class:feature target
// declaring contextRefs.
func featureTargetDC(t *testing.T, name string, contextRefs []string) ResolvedSpec {
	t.Helper()
	data := featureSpecBytesDC(name, contextRefs)
	return ResolvedSpec{
		Ref:           "spec/" + name,
		Path:          ".verdi/specs/active/" + name + "/spec.md",
		Blob:          strings.Repeat("d", 40),
		Commit:        strings.Repeat("e", 40),
		ContentDigest: rawContentDigest(data),
		Content:       data,
		Spec:          decodeFeatureFixtureDC(t, data),
	}
}

// storyTargetDC builds a bare class:story ResolvedSpec (story class carries
// no context: field of its own — SI-91/SI-92 use the union of governing
// parent features instead).
func storyTargetDC(name string) ResolvedSpec {
	return ResolvedSpec{
		Ref: "spec/" + name,
		Spec: &artifact.SpecFrontmatter{
			Base:  artifact.Base{ID: "spec/" + name, Kind: artifact.KindSpec, Title: "t", Owners: []string{"x"}},
			Class: artifact.ClassStory,
		},
	}
}

// multiCommitGit is a GitReader double keyed by ref (commit or HEAD), each
// carrying its own tree entries and path->content map, so a test can prove
// the resolver reads exactly the pinned commit it was told to and never
// conflates two commits' bytes. WorktreeChangedPaths fails the test
// immediately — declared-context resolution must never read the worktree.
type multiCommitGit struct {
	t            *testing.T
	entriesByRef map[string][]gitx.TreeEntry
	contentByRef map[string]map[string][]byte
	treeCalls    map[string]int
	showCalls    map[string]int
}

func newMultiCommitGit(t *testing.T) *multiCommitGit {
	return &multiCommitGit{
		t:            t,
		entriesByRef: map[string][]gitx.TreeEntry{},
		contentByRef: map[string]map[string][]byte{},
		treeCalls:    map[string]int{},
		showCalls:    map[string]int{},
	}
}

func (g *multiCommitGit) addEntry(ref string, e gitx.TreeEntry, content []byte) {
	g.entriesByRef[ref] = append(g.entriesByRef[ref], e)
	if g.contentByRef[ref] == nil {
		g.contentByRef[ref] = map[string][]byte{}
	}
	g.contentByRef[ref][e.Path] = content
}

func (g *multiCommitGit) Show(_ context.Context, _ string, ref, path string) ([]byte, error) {
	g.showCalls[ref]++
	c, ok := g.contentByRef[ref][path]
	if !ok {
		g.t.Fatalf("Show called for unexpected (ref=%q, path=%q)", ref, path)
	}
	return append([]byte(nil), c...), nil
}

func (g *multiCommitGit) LsTreeEntries(_ context.Context, _ string, ref string) ([]gitx.TreeEntry, error) {
	g.treeCalls[ref]++
	return append([]gitx.TreeEntry(nil), g.entriesByRef[ref]...), nil
}

func (g *multiCommitGit) WorktreeChangedPaths(context.Context, string) ([]string, error) {
	g.t.Fatal("unexpected WorktreeChangedPaths call — declared-context resolution must never read the worktree")
	return nil, nil
}

func regularEntryDC(path, object string) gitx.TreeEntry {
	return gitx.TreeEntry{Mode: "100644", Type: "blob", Object: object, Path: path}
}

// addArtifactDC registers a decodable non-spec artifact at (ref/commit,
// kind, name) in git, returning the pinned ref string.
func addArtifactDC(g *multiCommitGit, commit string, kind artifact.Kind, name string, content []byte) string {
	path, err := store.NonSpecArtifactPath(kind, name)
	if err != nil {
		g.t.Fatalf("NonSpecArtifactPath(%q, %q): %v", kind, name, err)
	}
	g.addEntry(commit, regularEntryDC(path, strings.Repeat("1", 40)), content)
	return string(kind) + "/" + name + "@" + commit
}

func addSpecDC(g *multiCommitGit, commit, zone, name string, content []byte) string {
	path := store.SpecRelPath(zone, name)
	g.addEntry(commit, regularEntryDC(path, strings.Repeat("2", 40)), content)
	return "spec/" + name + "@" + commit
}

const dcHeadCommit = "ffffffffffffffffffffffffffffffffffffff"
const dcRoot = "/repo"

// --- happy path: one of every non-spec kind, plus a whole and a fragment
// spec ref ------------------------------------------------------------

func TestResolveDeclaredContext_EveryNonSpecKindAndWholeSpec(t *testing.T) {
	commit := strings.Repeat("a", 40)
	g := newMultiCommitGit(t)

	adrRef := addArtifactDC(g, commit, artifact.KindADR, "example-choice", adrBytes("example-choice", "v1"))
	diagRef := addArtifactDC(g, commit, artifact.KindDiagram, "example-flow", diagramBytes("example-flow", "v1"))
	attRef := addArtifactDC(g, commit, artifact.KindAttestation, "example-story--ac-1", attestationBytesDC("example-story--ac-1", "v1"))
	waiverRef := addArtifactDC(g, commit, artifact.KindWaiver, "example-story--ac-1", waiverBytesDC("example-story--ac-1", "v1"))
	conflictRef := addArtifactDC(g, commit, artifact.KindConflict, "example-challenge", conflictBytesDC("example-challenge", "v1"))
	reaffRef := addArtifactDC(g, commit, artifact.KindReaffirmation, "example-story--ac-1", reaffirmationBytesDC("example-story--ac-1", "v1"))
	obligationRef := addArtifactDC(g, commit, artifact.KindObligation, "example-story--ac-1--behavioral", obligationBytesDC("example-story--ac-1--behavioral", "v1"))
	otherFeatureBytes := featureSpecBytesDC("other-feature", nil)
	specRef := addSpecDC(g, commit, store.ZoneActive, "other-feature", otherFeatureBytes)

	contextRefs := []string{adrRef, diagRef, attRef, waiverRef, conflictRef, reaffRef, obligationRef, specRef}
	target := featureTargetDC(t, "example-feature", contextRefs)

	got, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
	if err != nil {
		t.Fatalf("ResolveDeclaredContext: %v", err)
	}
	if len(got.Items) != len(contextRefs) {
		t.Fatalf("len(Items) = %d, want %d", len(got.Items), len(contextRefs))
	}
	byLogical := map[string]DeclaredContextItem{}
	for _, item := range got.Items {
		byLogical[item.LogicalRef] = item
	}
	cases := []struct {
		logical string
		kind    artifact.Kind
		name    string
		content []byte
	}{
		{"adr/example-choice", artifact.KindADR, "example-choice", adrBytes("example-choice", "v1")},
		{"diagram/example-flow", artifact.KindDiagram, "example-flow", diagramBytes("example-flow", "v1")},
		{"attestation/example-story--ac-1", artifact.KindAttestation, "example-story--ac-1", attestationBytesDC("example-story--ac-1", "v1")},
		{"waiver/example-story--ac-1", artifact.KindWaiver, "example-story--ac-1", waiverBytesDC("example-story--ac-1", "v1")},
		{"conflict/example-challenge", artifact.KindConflict, "example-challenge", conflictBytesDC("example-challenge", "v1")},
		{"reaffirmation/example-story--ac-1", artifact.KindReaffirmation, "example-story--ac-1", reaffirmationBytesDC("example-story--ac-1", "v1")},
		{"obligation/example-story--ac-1--behavioral", artifact.KindObligation, "example-story--ac-1--behavioral", obligationBytesDC("example-story--ac-1--behavioral", "v1")},
		{"spec/other-feature", artifact.KindSpec, "other-feature", otherFeatureBytes},
	}
	for _, c := range cases {
		item, ok := byLogical[c.logical]
		if !ok {
			t.Fatalf("missing item for logical ref %q", c.logical)
		}
		if item.Kind != c.kind || item.Name != c.name {
			t.Fatalf("item %q: kind=%q name=%q, want kind=%q name=%q", c.logical, item.Kind, item.Name, c.kind, c.name)
		}
		if !bytes.Equal(item.Content, c.content) {
			t.Fatalf("item %q: Content = %q, want %q", c.logical, item.Content, c.content)
		}
		if item.ContentDigest != rawContentDigest(c.content) {
			t.Fatalf("item %q: ContentDigest = %q, want %q", c.logical, item.ContentDigest, rawContentDigest(c.content))
		}
		if !strings.HasPrefix(item.ContentDigest, "sha256:") || len(item.ContentDigest) != len("sha256:")+64 {
			t.Fatalf("item %q: ContentDigest %q is not sha256:+64hex form", c.logical, item.ContentDigest)
		}
	}
	// Lift map: path -> logical ref, one entry per item, none pinned/fragmented.
	if len(got.Lift) != len(contextRefs) {
		t.Fatalf("len(Lift) = %d, want %d", len(got.Lift), len(contextRefs))
	}
	for path, logical := range got.Lift {
		item, ok := byLogical[logical]
		if !ok || item.Path != path {
			t.Fatalf("Lift[%q] = %q does not match a resolved item's own Path", path, logical)
		}
		if strings.ContainsAny(logical, "@#") {
			t.Fatalf("Lift value %q must be the unpinned, unfragmented logical ref (SI-92: ref:<kind>/<name>)", logical)
		}
	}
}

// --- two-commit divergent-bytes proof: never worktree, never a different
// commit's bytes -------------------------------------------------------

func TestResolveDeclaredContext_ExactPinnedCommitBytes(t *testing.T) {
	commit1 := strings.Repeat("a", 40)
	commit2 := strings.Repeat("b", 40)
	g := newMultiCommitGit(t)

	ref1 := addArtifactDC(g, commit1, artifact.KindADR, "example-choice", adrBytes("example-choice", "v1"))
	addArtifactDC(g, commit2, artifact.KindADR, "example-choice", adrBytes("example-choice", "v2"))

	t.Run("pinned to commit1", func(t *testing.T) {
		target := featureTargetDC(t, "example-feature", []string{ref1})
		got, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
		if err != nil {
			t.Fatalf("ResolveDeclaredContext: %v", err)
		}
		if len(got.Items) != 1 || !bytes.Equal(got.Items[0].Content, adrBytes("example-choice", "v1")) {
			t.Fatalf("got = %+v, want v1 bytes", got.Items)
		}
		if g.showCalls[commit1] == 0 {
			t.Fatal("Show was never called with commit1")
		}
		if g.showCalls[commit2] != 0 || g.treeCalls[commit2] != 0 {
			t.Fatalf("commit2 was touched while resolving a ref pinned to commit1: show=%d tree=%d", g.showCalls[commit2], g.treeCalls[commit2])
		}
		if g.showCalls[dcHeadCommit] != 0 || g.treeCalls[dcHeadCommit] != 0 {
			t.Fatalf("HEAD was touched while resolving a feature target with no parents: show=%d tree=%d", g.showCalls[dcHeadCommit], g.treeCalls[dcHeadCommit])
		}
	})

	t.Run("pinned to commit2", func(t *testing.T) {
		g2 := newMultiCommitGit(t)
		ref2 := addArtifactDC(g2, commit1, artifact.KindADR, "example-choice", adrBytes("example-choice", "v1"))
		_ = ref2
		ref2b := addArtifactDC(g2, commit2, artifact.KindADR, "example-choice", adrBytes("example-choice", "v2"))
		target := featureTargetDC(t, "example-feature", []string{ref2b})
		got, err := ResolveDeclaredContext(context.Background(), g2, dcRoot, dcHeadCommit, target, nil)
		if err != nil {
			t.Fatalf("ResolveDeclaredContext: %v", err)
		}
		if len(got.Items) != 1 || !bytes.Equal(got.Items[0].Content, adrBytes("example-choice", "v2")) {
			t.Fatalf("got = %+v, want v2 bytes", got.Items)
		}
	})
}

// --- spec fragments: DeclaredObjectIDs only, #problem/#outcome never
// resolve, non-spec fragment is invalid authority -----------------------

func TestResolveDeclaredContext_SpecFragmentResolvesViaDeclaredObjectIDs(t *testing.T) {
	commit := strings.Repeat("a", 40)
	g := newMultiCommitGit(t)
	parentBytes := featureSpecBytesDC("other-feature", nil)
	specRef := addSpecDC(g, commit, store.ZoneActive, "other-feature", parentBytes)

	target := featureTargetDC(t, "example-feature", []string{specRef + "#ac-1"})
	got, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
	if err != nil {
		t.Fatalf("ResolveDeclaredContext: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("len(Items) = %d, want 1", len(got.Items))
	}
	item := got.Items[0]
	if item.LogicalRef != "spec/other-feature" {
		t.Fatalf("LogicalRef = %q, want the whole spec (fragment dropped)", item.LogicalRef)
	}
	if !bytes.Equal(item.Content, parentBytes) {
		t.Fatalf("Content = %q, want the WHOLE file bytes, not a fragment projection", item.Content)
	}
	if item.Ref != specRef+"#ac-1" {
		t.Fatalf("Ref = %q, want the caller's complete pinned+fragment ref preserved as identity", item.Ref)
	}
}

func TestResolveDeclaredContext_ProblemOutcomeFragmentsDoNotResolve(t *testing.T) {
	// "#problem"/"#outcome" are not even legal fragment-object-id shapes
	// (artifact's objectIDRe requires a "<prefix>-<slug>" shape with a
	// dash — "problem"/"outcome" have none), so a real feature spec can
	// never declare them in its context: list in the first place (decode
	// itself rejects it via ParsePinnedRef). Context is therefore
	// constructed directly here, bypassing the YAML round trip, to prove
	// ResolveDeclaredContext itself — not just spec decode — also refuses
	// them (SI-92: "#problem and #outcome do not resolve"). "#ac-99" is a
	// grammatically legal but undeclared object id, reaching the
	// DeclaredObjectIDs check this resolver performs.
	for _, frag := range []string{"#problem", "#outcome", "#ac-99"} {
		t.Run(frag, func(t *testing.T) {
			commit := strings.Repeat("a", 40)
			g := newMultiCommitGit(t)
			specRef := addSpecDC(g, commit, store.ZoneActive, "other-feature", featureSpecBytesDC("other-feature", nil))
			target := featureTargetDC(t, "example-feature", nil)
			target.Spec.Context = []string{specRef + frag}
			got, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
			if err == nil {
				t.Fatalf("ResolveDeclaredContext: want invalid-authority error, got %+v", got)
			}
			if IsRefusal(err) {
				t.Fatalf("classified as refusal, want operational invalid-authority: %T %v", err, err)
			}
		})
	}
}

func TestResolveDeclaredContext_FragmentOnNonSpecArtifactIsInvalidAuthority(t *testing.T) {
	commit := strings.Repeat("a", 40)
	g := newMultiCommitGit(t)
	adrRef := addArtifactDC(g, commit, artifact.KindADR, "example-choice", adrBytes("example-choice", "v1"))
	target := featureTargetDC(t, "example-feature", []string{adrRef + "#whatever-1"})
	_, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
	if err == nil {
		t.Fatal("ResolveDeclaredContext: want error for a fragment on a non-spec artifact, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("classified as refusal, want operational: %T %v", err, err)
	}
}

// --- dedup / duplicate-candidate ---------------------------------------

func TestResolveDeclaredContext_IdenticalExactRefsDedupe(t *testing.T) {
	commit := strings.Repeat("a", 40)
	g := newMultiCommitGit(t)
	ref := addArtifactDC(g, commit, artifact.KindADR, "example-choice", adrBytes("example-choice", "v1"))
	target := featureTargetDC(t, "example-feature", nil)
	target.Spec.Context = []string{ref, ref} // identical exact ref declared twice

	got, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
	if err != nil {
		t.Fatalf("ResolveDeclaredContext: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("len(Items) = %d, want 1 (identical exact refs dedupe)", len(got.Items))
	}
}

func TestResolveDeclaredContext_DistinctRefsSameLogicalIDIsDuplicateCandidate(t *testing.T) {
	commit1 := strings.Repeat("a", 40)
	commit2 := strings.Repeat("b", 40)
	g := newMultiCommitGit(t)
	ref1 := addArtifactDC(g, commit1, artifact.KindADR, "example-choice", adrBytes("example-choice", "v1"))
	ref2 := addArtifactDC(g, commit2, artifact.KindADR, "example-choice", adrBytes("example-choice", "v2"))
	target := featureTargetDC(t, "example-feature", nil)
	target.Spec.Context = []string{ref1, ref2} // same kind/name, different commits

	_, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
	if err == nil {
		t.Fatal("ResolveDeclaredContext: want duplicate-candidate error, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("classified as refusal, want operational: %T %v", err, err)
	}
}

// --- archived/superseded pins remain included ---------------------------

func TestResolveDeclaredContext_ArchivedSpecPinRemainsIncluded(t *testing.T) {
	commit := strings.Repeat("a", 40)
	g := newMultiCommitGit(t)
	archivedBytes := featureSpecBytesDC("archived-feature", nil)
	specRef := addSpecDC(g, commit, store.ZoneArchive, "archived-feature", archivedBytes)

	target := featureTargetDC(t, "example-feature", []string{specRef})
	got, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
	if err != nil {
		t.Fatalf("ResolveDeclaredContext: %v", err)
	}
	if len(got.Items) != 1 || !bytes.Equal(got.Items[0].Content, archivedBytes) {
		t.Fatalf("archived spec pin was not resolved: %+v", got.Items)
	}
}

// --- multi-parent story/spike union, TOCTOU re-verification -------------

func TestResolveDeclaredContext_MultiParentUnionSortedDeduplicated(t *testing.T) {
	commit := strings.Repeat("a", 40)
	g := newMultiCommitGit(t)
	adrRef := addArtifactDC(g, commit, artifact.KindADR, "shared-choice", adrBytes("shared-choice", "v1"))
	diagRef := addArtifactDC(g, commit, artifact.KindDiagram, "only-in-b", diagramBytes("only-in-b", "v1"))

	featureABytes := featureSpecBytesDC("feature-a", []string{adrRef})
	featureBBytes := featureSpecBytesDC("feature-b", []string{adrRef, diagRef}) // adrRef duplicated across parents
	g.addEntry(dcHeadCommit, regularEntryDC(".verdi/specs/active/feature-a/spec.md", strings.Repeat("3", 40)), featureABytes)
	g.addEntry(dcHeadCommit, regularEntryDC(".verdi/specs/active/feature-b/spec.md", strings.Repeat("4", 40)), featureBBytes)

	parents := []FeatureFragment{
		{Feature: FragmentFeature{Ref: "spec/feature-a", Path: ".verdi/specs/active/feature-a/spec.md", SourceDigest: rawContentDigest(featureABytes)}},
		{Feature: FragmentFeature{Ref: "spec/feature-b", Path: ".verdi/specs/active/feature-b/spec.md", SourceDigest: rawContentDigest(featureBBytes)}},
	}
	target := storyTargetDC("example-story")

	got, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, parents)
	if err != nil {
		t.Fatalf("ResolveDeclaredContext: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2 (adrRef unioned once, diagRef once)", len(got.Items))
	}
	if g.showCalls[dcHeadCommit] != 2 {
		t.Fatalf("Show(head, ...) called %d times, want exactly 2 (one per parent feature path)", g.showCalls[dcHeadCommit])
	}
}

func TestResolveDeclaredContext_ParentTOCTOUDigestMismatchFails(t *testing.T) {
	g := newMultiCommitGit(t)
	featureABytes := featureSpecBytesDC("feature-a", nil)
	g.addEntry(dcHeadCommit, regularEntryDC(".verdi/specs/active/feature-a/spec.md", strings.Repeat("3", 40)), featureABytes)

	parents := []FeatureFragment{
		{Feature: FragmentFeature{Ref: "spec/feature-a", Path: ".verdi/specs/active/feature-a/spec.md", SourceDigest: "sha256:" + strings.Repeat("0", 64)}},
	}
	target := storyTargetDC("example-story")
	_, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, parents)
	if err == nil {
		t.Fatal("ResolveDeclaredContext: want TOCTOU digest-mismatch error, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("classified as refusal, want operational: %T %v", err, err)
	}
}

func TestResolveDeclaredContext_ParentTOCTOURefClassMismatchFails(t *testing.T) {
	g := newMultiCommitGit(t)
	// spec.ID disagrees with the claimed FragmentFeature.Ref.
	wrongBytes := featureSpecBytesDC("actually-different-feature", nil)
	g.addEntry(dcHeadCommit, regularEntryDC(".verdi/specs/active/feature-a/spec.md", strings.Repeat("3", 40)), wrongBytes)

	parents := []FeatureFragment{
		{Feature: FragmentFeature{Ref: "spec/feature-a", Path: ".verdi/specs/active/feature-a/spec.md", SourceDigest: rawContentDigest(wrongBytes)}},
	}
	target := storyTargetDC("example-story")
	_, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, parents)
	if err == nil {
		t.Fatal("ResolveDeclaredContext: want ref-mismatch error, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("classified as refusal, want operational: %T %v", err, err)
	}
}

// --- class dispatch guards ------------------------------------------

func TestResolveDeclaredContext_FeatureTargetRejectsParents(t *testing.T) {
	g := newMultiCommitGit(t)
	target := featureTargetDC(t, "example-feature", nil)
	parents := []FeatureFragment{{Feature: FragmentFeature{Ref: "spec/feature-a", Path: ".verdi/specs/active/feature-a/spec.md", SourceDigest: "sha256:" + strings.Repeat("0", 64)}}}
	_, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, parents)
	if err == nil {
		t.Fatal("ResolveDeclaredContext: want error, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("classified as refusal, want operational: %T %v", err, err)
	}
}

func TestResolveDeclaredContext_StoryTargetRequiresParents(t *testing.T) {
	g := newMultiCommitGit(t)
	target := storyTargetDC("example-story")
	_, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
	if err == nil {
		t.Fatal("ResolveDeclaredContext: want error, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("classified as refusal, want operational: %T %v", err, err)
	}
}

func TestResolveDeclaredContext_UnsupportedClassFails(t *testing.T) {
	g := newMultiCommitGit(t)
	target := ResolvedSpec{
		Ref: "spec/example-component",
		Spec: &artifact.SpecFrontmatter{
			Base:  artifact.Base{ID: "spec/example-component", Kind: artifact.KindSpec, Title: "t", Owners: []string{"x"}},
			Class: artifact.ClassComponent,
		},
	}
	_, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
	if err == nil {
		t.Fatal("ResolveDeclaredContext: want error, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("classified as refusal, want operational: %T %v", err, err)
	}
}

// --- negative matrix -----------------------------------------------------

func TestResolveDeclaredContext_MissingPathAtPinnedCommit(t *testing.T) {
	commit := strings.Repeat("a", 40)
	g := newMultiCommitGit(t)
	// Nothing registered at commit at all.
	target := featureTargetDC(t, "example-feature", []string{"adr/example-choice@" + commit})
	_, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
	if err == nil {
		t.Fatal("ResolveDeclaredContext: want error for a missing pinned path, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("classified as refusal, want operational: %T %v", err, err)
	}
}

func TestResolveDeclaredContext_NonRegularEntry(t *testing.T) {
	commit := strings.Repeat("a", 40)
	path, err := store.NonSpecArtifactPath(artifact.KindADR, "example-choice")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		entry gitx.TreeEntry
	}{
		{"symlink", gitx.TreeEntry{Mode: "120000", Type: "blob", Object: strings.Repeat("1", 40), Path: path}},
		{"gitlink", gitx.TreeEntry{Mode: "160000", Type: "commit", Object: strings.Repeat("1", 40), Path: path}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newMultiCommitGit(t)
			g.addEntry(commit, tt.entry, adrBytes("example-choice", "v1"))
			target := featureTargetDC(t, "example-feature", []string{"adr/example-choice@" + commit})
			_, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
			if err == nil {
				t.Fatal("ResolveDeclaredContext: want error for a non-regular entry, got nil")
			}
			if IsRefusal(err) {
				t.Fatalf("classified as refusal, want operational: %T %v", err, err)
			}
		})
	}
}

func TestResolveDeclaredContext_AmbiguousSpecZones(t *testing.T) {
	commit := strings.Repeat("a", 40)
	g := newMultiCommitGit(t)
	content := featureSpecBytesDC("ambiguous-feature", nil)
	g.addEntry(commit, regularEntryDC(store.ActiveSpecRelPath("ambiguous-feature"), strings.Repeat("1", 40)), content)
	g.addEntry(commit, regularEntryDC(store.SpecRelPath(store.ZoneArchive, "ambiguous-feature"), strings.Repeat("2", 40)), content)

	target := featureTargetDC(t, "example-feature", []string{"spec/ambiguous-feature@" + commit})
	_, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
	if err == nil {
		t.Fatal("ResolveDeclaredContext: want error for ambiguous active+archive zones, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("classified as refusal, want operational: %T %v", err, err)
	}
}

func TestResolveDeclaredContext_IdentityMismatch(t *testing.T) {
	commit := strings.Repeat("a", 40)
	g := newMultiCommitGit(t)
	// File at the adr/wrong-name path declares id adr/actually-different.
	path, err := store.NonSpecArtifactPath(artifact.KindADR, "wrong-name")
	if err != nil {
		t.Fatal(err)
	}
	g.addEntry(commit, regularEntryDC(path, strings.Repeat("1", 40)), adrBytes("actually-different", "v1"))
	target := featureTargetDC(t, "example-feature", []string{"adr/wrong-name@" + commit})
	_, err = ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
	if err == nil {
		t.Fatal("ResolveDeclaredContext: want identity-mismatch error, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("classified as refusal, want operational: %T %v", err, err)
	}
}

func TestResolveDeclaredContext_MalformedFrontmatter(t *testing.T) {
	commit := strings.Repeat("a", 40)
	g := newMultiCommitGit(t)
	path, err := store.NonSpecArtifactPath(artifact.KindADR, "example-choice")
	if err != nil {
		t.Fatal(err)
	}
	malformed := []byte("---\n" +
		"id: adr/example-choice\n" +
		"kind: adr\n" +
		"title: \"ADR\"\n" +
		"owners: [platform-team]\n" +
		"status: proposed\n" +
		"unexpected: true\n" +
		"---\n" +
		"Body.\n")
	g.addEntry(commit, regularEntryDC(path, strings.Repeat("1", 40)), malformed)
	target := featureTargetDC(t, "example-feature", []string{"adr/example-choice@" + commit})
	_, err = ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
	if err == nil {
		t.Fatal("ResolveDeclaredContext: want decode error, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("classified as refusal, want operational: %T %v", err, err)
	}
}

func TestResolveDeclaredContext_InvalidUTF8AndNUL(t *testing.T) {
	commit := strings.Repeat("a", 40)
	path, err := store.NonSpecArtifactPath(artifact.KindADR, "example-choice")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		content []byte
	}{
		{"invalid utf-8", append(adrBytes("example-choice", "v1"), 0xff, 0xfe)},
		{"embedded NUL", append(adrBytes("example-choice", "v1"), 0x00)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newMultiCommitGit(t)
			g.addEntry(commit, regularEntryDC(path, strings.Repeat("1", 40)), tt.content)
			target := featureTargetDC(t, "example-feature", []string{"adr/example-choice@" + commit})
			_, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
			if err == nil {
				t.Fatal("ResolveDeclaredContext: want invalid-authority error, got nil")
			}
			if IsRefusal(err) {
				t.Fatalf("classified as refusal, want operational: %T %v", err, err)
			}
		})
	}
}

func TestResolveDeclaredContext_ShortOrMalformedCommitHash(t *testing.T) {
	g := newMultiCommitGit(t)
	tests := []string{
		"adr/example-choice@abcdef",   // 6 hex, too short
		"adr/example-choice@abcdefgz", // non-hex character
		"adr/example-choice",          // unpinned entirely
	}
	for _, ref := range tests {
		t.Run(ref, func(t *testing.T) {
			target := featureTargetDC(t, "example-feature", nil)
			target.Spec.Context = []string{ref}
			_, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil)
			if err == nil {
				t.Fatalf("ResolveDeclaredContext(%q): want error, got nil", ref)
			}
			if IsRefusal(err) {
				t.Fatalf("classified as refusal, want operational: %T %v", err, err)
			}
		})
	}
}

func TestResolveDeclaredContext_NilGitPort(t *testing.T) {
	target := featureTargetDC(t, "example-feature", nil)
	_, err := ResolveDeclaredContext(context.Background(), nil, dcRoot, dcHeadCommit, target, nil)
	if err == nil {
		t.Fatal("ResolveDeclaredContext: want error for a nil git port, got nil")
	}
	if IsRefusal(err) {
		t.Fatalf("classified as refusal, want operational: %T %v", err, err)
	}
}

func TestResolveDeclaredContext_NeverReadsWorktree(t *testing.T) {
	// Every test above already uses a git double that Fatals on
	// WorktreeChangedPaths; this test only makes the guarantee explicit for
	// the happy path with a nonempty context list.
	commit := strings.Repeat("a", 40)
	g := newMultiCommitGit(t)
	ref := addArtifactDC(g, commit, artifact.KindADR, "example-choice", adrBytes("example-choice", "v1"))
	target := featureTargetDC(t, "example-feature", []string{ref})
	if _, err := ResolveDeclaredContext(context.Background(), g, dcRoot, dcHeadCommit, target, nil); err != nil {
		t.Fatalf("ResolveDeclaredContext: %v", err)
	}
	// No explicit call needed: WorktreeChangedPaths would have failed the
	// test immediately via g.t.Fatal if the resolver had ever called it.
}
