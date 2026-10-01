package main

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// playwrightBuildStorySpecMD is a landed story whose two acceptance criteria
// each expect behavioral evidence.
const playwrightBuildStorySpecMD = `---
id: spec/pw-story
kind: spec
class: story
title: "Playwright-proven story"
owners: [platform-team]
story: jira:PW-1
problem: { text: "x", anchor: problem }
outcome: { text: "y", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "the page renders before script runs", evidence: [behavioral] }
  - { id: ac-2, text: "the board's keyboard order holds", evidence: [behavioral] }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
---
# body
`

// playwrightQualityBlock is an elaborated quality block naming ref, a
// Playwright test, as its producer, authoritative for the verify job.
func playwrightQualityBlock(ref string) string {
	return "quality:\n  state: elaborated\n  claim: claim\n  falsifier: falsifier\n  scope: scope\n" +
		"  producer: { kind: test, ref: \"" + ref + "\" }\n" +
		"  authoritative_source: { kind: ci-job, ref: \"verify\" }\n" +
		"  freshness:\n    invalidated_by: [code]\n    rule: rerun\n"
}

// TestBuildStart_PlaywrightProducersPassObligationQuality proves design §7
// item 6 by driving the BUILT binary: `verdi build start` over a fixture store
// whose story's behavioral obligations name Playwright producers (one with a
// ":" in its title path) passes its obligation-quality precondition and
// creates the build branch, while the same story with one obligation still
// unresolved design debt is refused naming it.
func TestBuildStart_PlaywrightProducersPassObligationQuality(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	const (
		ref1 = "playwright:e2e/tests/00-home.spec.ts:home renders: before script runs"
		ref2 = "playwright:e2e/tests/27-board-legibility.spec.ts:board legibility › keyboard order"
	)
	cases := []struct {
		name       string
		ac2Quality string
		wantCode   int
		wantOut    string
		wantErr    string
	}{
		{name: "every obligation names a Playwright producer", ac2Quality: playwrightQualityBlock(ref2), wantCode: 0, wantOut: "build start: created branch feature/pw-story from spec/pw-story"},
		{name: "one obligation is still design debt", ac2Quality: "quality:\n  state: unresolved-design-debt\n", wantCode: 1, wantErr: "obligation quality unresolved: ac-2/behavioral: unresolved-design-debt"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			repo := fixturegit.Build(t, []fixturegit.Layer{{
				Files: map[string]string{
					".verdi/verdi.yaml":                               phase7ManifestYAML,
					".verdi/specs/active/pw-story/spec.md":            playwrightBuildStorySpecMD,
					".verdi/obligations/pw-story/ac-1--behavioral.md": buildQualityObligationDocument("pw-story", "ac-1", artifact.EvidenceBehavioral, playwrightQualityBlock(ref1)),
					".verdi/obligations/pw-story/ac-2--behavioral.md": buildQualityObligationDocument("pw-story", "ac-2", artifact.EvidenceBehavioral, c.ac2Quality),
				},
				Message: "a landed story whose behavioral obligations name Playwright tests",
			}})
			pinFixtureDefaultBranch(t, repo.Dir)

			code, stdout, stderr := runVerdi(t, bin, repo.Dir, "build", "start", "spec/pw-story")
			if code != c.wantCode {
				t.Fatalf("build start = %d, want %d; stdout=%s stderr=%s", code, c.wantCode, stdout, stderr)
			}
			if c.wantOut != "" && !strings.Contains(stdout, c.wantOut) {
				t.Errorf("stdout = %q, want %q", stdout, c.wantOut)
			}
			if c.wantErr != "" && !strings.Contains(stderr, c.wantErr) {
				t.Errorf("stderr = %q, want %q", stderr, c.wantErr)
			}
			if c.wantCode == 0 && strings.Contains(stderr, "obligation quality") {
				t.Errorf("stderr = %q, want no obligation-quality finding", stderr)
			}
		})
	}
}
