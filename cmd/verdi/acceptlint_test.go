// Tests for D6-23 (round6-divergences.md): `verdi accept` must refuse to
// freeze a quartet the store's own linter rejects, rather than flipping
// status and writing the frozen stamp over a spec the store already knows
// is broken (the round-6 witness: a dangling layout.json positions key
// sailed through accept and was only caught by CI's spec-gate, after push).
// Kept in its own file per this package's one-file-per-topic convention
// (accept.go's own doc comment; supersedepredecessor_test.go mirrors
// supersede.go the same way) since acceptlint.go is itself a new, focused
// file rather than more weight added to accept.go directly.
package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/lint"
)

// quartetDraftSpecMD is a minimal, otherwise-lint-clean draft feature spec
// (mirrors internal/lint's own vl018CleanSpec fixture, reused here rather
// than re-invented, since that fixture is already proven clean against the
// real engine by lint's own test suite) — the sibling layout.json is what
// each test below varies. ac-1 declares attestation (L-M14 remedy 1,
// internal/lint/vl006.go's checkFeatureACAttestation) — this fixture is
// status: draft/unfrozen, so it is not otherwise grandfathered.
const quartetDraftSpecMD = `---
id: spec/quartet-lint
kind: spec
class: feature
title: "Quartet lint gate"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static, attestation], anchor: "#ac-1" }
constraints:
  - { id: co-1, text: "placeholder constraint", anchor: "#co-1" }
---
# Quartet lint gate

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.

## CO-1

Placeholder constraint.
`

// quartetDanglingLayoutJSON carries a positions key ("no-such-ac") that
// resolves to no declared object id or stub on quartetDraftSpecMD — the
// exact VL-018 shape D6-23 witnessed.
const quartetDanglingLayoutJSON = `{
  "schema": "verdi.boardlayout/v1",
  "positions": {
    "ac-1": { "x": 40, "y": 20 },
    "no-such-ac": { "x": 990, "y": 40 }
  }
}
`

// quartetCleanLayoutJSON is quartetDanglingLayoutJSON's positive
// complement: every positions key resolves.
const quartetCleanLayoutJSON = `{
  "schema": "verdi.boardlayout/v1",
  "positions": {
    "ac-1": { "x": 40, "y": 20 }
  }
}
`

// buildQuartetLintRepo builds a one-layer fixturegit repo carrying
// quartetDraftSpecMD plus an optional layout.json (layoutJSON == "" omits
// it entirely — VL-018 never gates on absence).
func buildQuartetLintRepo(t *testing.T, layoutJSON string) *fixturegit.Repo {
	t.Helper()
	files := map[string]string{
		".verdi/verdi.yaml":                        phase7ManifestYAML,
		".gitattributes":                           phase7GitAttributes,
		".verdi/specs/active/quartet-lint/spec.md": quartetDraftSpecMD,
	}
	if layoutJSON != "" {
		files[".verdi/specs/active/quartet-lint/layout.json"] = layoutJSON
	}
	return fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "init store with quartet-lint draft spec"}})
}

// TestRunAccept_RefusesDanglingLayoutKey is D6-23's core reproduction: a
// dangling layout.json positions key (VL-018) must refuse accept — exit 1,
// naming the violation verbatim — leaving the spec and HEAD untouched.
func TestRunAccept_RefusesDanglingLayoutKey(t *testing.T) {
	repo := buildQuartetLintRepo(t, quartetDanglingLayoutJSON)
	ctx := context.Background()

	beforeHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	_, rawBefore := readSpec(t, repo.Dir, "quartet-lint")

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "spec/quartet-lint", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runAccept(dangling layout key) = %d, want 1 (verdict refusal); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !contains(stderr.String(), "VL-018") {
		t.Fatalf("stderr = %q, want it to name VL-018 verbatim", stderr.String())
	}
	if !contains(stderr.String(), "no-such-ac") {
		t.Fatalf("stderr = %q, want it to name the dangling key %q verbatim", stderr.String(), "no-such-ac")
	}

	// Refused: the spec is byte-identical (no status flip, no frozen
	// stamp), and no commit was created.
	_, rawAfter := readSpec(t, repo.Dir, "quartet-lint")
	if !bytes.Equal(rawBefore, rawAfter) {
		t.Fatalf("a refused accept must not touch the spec:\n--- before ---\n%s\n--- after ---\n%s", rawBefore, rawAfter)
	}
	afterHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if afterHead != beforeHead {
		t.Fatal("a refused accept must not create a commit")
	}
}

// TestRunAccept_AcceptsCleanLayoutJSON is the positive complement: a
// present layout.json whose every key resolves does not block acceptance —
// "a clean spec must accept exactly as today".
func TestRunAccept_AcceptsCleanLayoutJSON(t *testing.T) {
	repo := buildQuartetLintRepo(t, quartetCleanLayoutJSON)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "spec/quartet-lint", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAccept(clean layout.json) = %d, want 0; stderr=%s", got, stderr.String())
	}
	spec, _ := readSpec(t, repo.Dir, "quartet-lint")
	if spec.Status != "accepted-pending-build" {
		t.Fatalf("spec.Status = %q, want accepted-pending-build", spec.Status)
	}
}

// TestRunAccept_AcceptsWithNoLayoutJSON proves the same "accepts exactly as
// today" baseline when there is no layout.json sidecar at all (VL-018 never
// gates on absence, mirrored here at the accept level too).
func TestRunAccept_AcceptsWithNoLayoutJSON(t *testing.T) {
	repo := buildQuartetLintRepo(t, "")
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "spec/quartet-lint", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAccept(no layout.json) = %d, want 0; stderr=%s", got, stderr.String())
	}
}

// TestRunAccept_DisclosureOnlyFindingsDoNotRefuse proves a spec whose only
// in-scope finding is a SeverityDisclosure (VL-017's standing "mutable zone
// absent" notice on a bare clone) still accepts — disclosures are never a
// verdict failure, mirroring `verdi lint`'s own severity semantics exactly
// (lint.go: "a run whose only findings are disclosures still exits 0").
// The precondition is proved directly against the real engine, rather than
// merely asserted in prose, so this test stays honest if the design
// scaffold's shape ever changes.
func TestRunAccept_DisclosureOnlyFindingsDoNotRefuse(t *testing.T) {
	repo, _ := scaffoldAndDesign(t)
	ctx := context.Background()

	lctx := lint.BuildContext(ctx, repo.Dir)
	findings, err := lint.NewEngine().Run(ctx, repo.Dir, lctx, lint.Options{})
	if err != nil {
		t.Fatalf("lint.Run: %v", err)
	}
	prefix := ".verdi/specs/active/stale-decline"
	var inScope []lint.Finding
	for _, f := range findings {
		if f.Path == prefix || len(f.Path) > len(prefix) && f.Path[:len(prefix)+1] == prefix+"/" {
			inScope = append(inScope, f)
		}
	}
	if len(inScope) == 0 {
		t.Fatal("test setup: expected at least one in-scope finding (the standing VL-017 disclosure) before accept even runs")
	}
	for _, f := range inScope {
		if f.Severity != lint.SeverityDisclosure {
			t.Fatalf("test setup: in-scope finding %s is not a disclosure (severity=%d) — fixture no longer matches this test's premise", f.String(), f.Severity)
		}
	}

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "spec/stale-decline", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAccept with only disclosure findings in scope = %d, want 0; stderr=%s", got, stderr.String())
	}
}

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
