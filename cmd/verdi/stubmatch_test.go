package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/fixturegit"
)

// stubMatchFeatureSpecMD is a minimal accepted feature carrying one stub
// (slug: stale-decline, ac-1) — the R4-I-12 fast-path target every
// TestComputeStubMatch case below matches or deliberately misses against.
const stubMatchFeatureSpecMD = `---
id: spec/loan-mgmt
kind: spec
title: "Loan management"
owners: [platform-team]
class: feature
status: accepted-pending-build
problem: { text: "x", anchor: problem }
outcome: { text: "y", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static, attestation] }
stubs:
  - { slug: stale-decline, acceptance_criteria: [ac-1] }
frozen: { at: 2024-01-01, commit: 0000000000000000000000000000000000000a }
---
# Loan management

## Problem
x
## Outcome
y
`

func buildStubMatchRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                     phase7ManifestYAML,
				".verdi/specs/active/loan-mgmt/spec.md": stubMatchFeatureSpecMD,
			},
			Message: "init store with a stubbed feature",
		},
	})
}

// draftStory renders a class: story spec at status: draft (never written
// to disk by these table cases — computeStubMatch only reads the FEATURE
// from disk; the story itself is passed in directly).
func draftStory(title string, links []artifact.Link, decisions []artifact.Decision) *artifact.SpecFrontmatter {
	return &artifact.SpecFrontmatter{
		Base: artifact.Base{
			ID:     "spec/stale-decline-story",
			Kind:   artifact.KindSpec,
			Title:  title,
			Owners: []string{"platform-team"},
			Links:  links,
		},
		Class:     artifact.ClassStory,
		Status:    "draft",
		Story:     "jira:LOAN-1482",
		Problem:   &artifact.Attribute{Text: "x", Anchor: "problem"},
		Outcome:   &artifact.Attribute{Text: "y", Anchor: "outcome"},
		Decisions: decisions,
		AcceptanceCriteria: []artifact.AcceptanceCriterion{
			{ID: "ac-1", Text: "static obligation holds", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}},
		},
	}
}

func implementsAC1() []artifact.Link {
	return []artifact.Link{{Type: artifact.LinkImplements, Ref: "spec/loan-mgmt#ac-1"}}
}

// TestComputeStubMatch covers R4-I-12's four-condition test, one negative
// case per condition plus the happy path.
func TestComputeStubMatch(t *testing.T) {
	repo := buildStubMatchRepo(t)

	cases := []struct {
		name        string
		story       *artifact.SpecFrontmatter
		wantMatched bool
		wantReason  string // substring, only checked when wantMatched is false
	}{
		{
			name:        "happy: implements-set and RefSlug(title) both match",
			story:       draftStory("Stale Decline", implementsAC1(), nil),
			wantMatched: true,
		},
		{
			name:        "implements-set does not match any stub",
			story:       draftStory("Stale Decline", []artifact.Link{{Type: artifact.LinkImplements, Ref: "spec/loan-mgmt#ac-2"}}, nil),
			wantMatched: false,
			wantReason:  "does not equal",
		},
		{
			name:        "RefSlug(title) does not match the matched stub's slug",
			story:       draftStory("A Totally Different Title", implementsAC1(), nil),
			wantMatched: false,
			wantReason:  "RefSlug(title)",
		},
		{
			name: "top-level supersedes edge disqualifies",
			story: draftStory("Stale Decline", append(implementsAC1(),
				artifact.Link{Type: artifact.LinkSupersedes, Ref: "spec/stale-decline-story-v1"}), nil),
			wantMatched: false,
			wantReason:  "supersedes",
		},
		{
			name: "exempts edge on a decision disqualifies",
			story: draftStory("Stale Decline", implementsAC1(), []artifact.Decision{
				{ID: "dc-1", Text: "x", Anchor: "problem", Links: []artifact.Link{{Type: artifact.LinkExempts, Ref: "adr/some-decision"}}},
			}),
			wantMatched: false,
			wantReason:  "supersedes/exempts",
		},
		{
			name:        "no implements edges at all (spike-shaped)",
			story:       draftStory("Stale Decline", nil, nil),
			wantMatched: false,
			wantReason:  "no implements edges",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			matched, reason := computeStubMatch(repo.Dir, tc.story)
			if matched != tc.wantMatched {
				t.Fatalf("computeStubMatch() matched = %v, want %v (reason=%q)", matched, tc.wantMatched, reason)
			}
			if !tc.wantMatched && !contains(reason, tc.wantReason) {
				t.Fatalf("reason = %q, want it to contain %q", reason, tc.wantReason)
			}
		})
	}
}

// TestComputeStubMatch_UndispositionedJudgedFinding proves condition (d):
// a decision-conflict-report.md with an undispositioned judged finding
// disqualifies the match; a fully-dispositioned one (or its absence
// entirely) does not.
func TestComputeStubMatch_UndispositionedJudgedFinding(t *testing.T) {
	repo := buildStubMatchRepo(t)
	story := draftStory("Stale Decline", implementsAC1(), nil)

	t.Run("absent report: vacuously satisfied", func(t *testing.T) {
		matched, reason := computeStubMatch(repo.Dir, story)
		if !matched {
			t.Fatalf("computeStubMatch() = false (%s), want true (no report at all)", reason)
		}
	})

	dir := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline-story")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(dir, "decision-conflict-report.md")

	t.Run("undispositioned judged finding disqualifies", func(t *testing.T) {
		content := fmt.Sprintf(`---
schema: verdi.deviation/v1
covers: %s
findings:
  - { id: dc-conflict-1, kind: judged, text: "possible conflict with ADR-3" }
digest: sha256:%s
---
# Decision-conflict report
`, "0000000000000000000000000000000000000b", repeatHex(64))
		if err := os.WriteFile(reportPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		matched, reason := computeStubMatch(repo.Dir, story)
		if matched {
			t.Fatal("computeStubMatch() = true, want false (undispositioned judged finding)")
		}
		if !contains(reason, "undispositioned") {
			t.Fatalf("reason = %q, want it to name the undispositioned finding", reason)
		}
	})

	t.Run("dispositioned judged finding satisfies", func(t *testing.T) {
		content := fmt.Sprintf(`---
schema: verdi.deviation/v1
covers: %s
findings:
  - { id: dc-conflict-1, kind: judged, text: "possible conflict with ADR-3", disposition: accepted-deviation, note: "reviewed, fine" }
digest: sha256:%s
---
# Decision-conflict report
`, "0000000000000000000000000000000000000b", repeatHex(64))
		if err := os.WriteFile(reportPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		matched, reason := computeStubMatch(repo.Dir, story)
		if !matched {
			t.Fatalf("computeStubMatch() = false (%s), want true (fully dispositioned)", reason)
		}
	})
}

func repeatHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '0'
	}
	return string(b)
}
