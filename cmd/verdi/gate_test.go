package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

const gateFakeFrozenCommit = "0000000000000000000000000000000000000a"

// gateSpecMD renders .verdi/specs/active/stale-decline/spec.md at the given
// status — accepted-pending-build carries a (syntactically valid but
// otherwise meaningless, gate never resolves it) Frozen stamp, since
// SpecFrontmatter.Validate requires one at that status.
func gateSpecMD(status string) string {
	frozen := ""
	if status == "accepted-pending-build" {
		frozen = fmt.Sprintf("\nfrozen: { at: 2024-01-01, commit: %s }", gateFakeFrozenCommit)
	}
	return fmt.Sprintf(`---
id: spec/stale-decline
kind: spec
class: feature
title: "Stale decline"
status: %s
owners: [platform-team]
story: jira:LOAN-1482
impacts: [loansvc]
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }%s
---
# body
`, status, frozen)
}

// buildGateRepo builds a one-layer fixturegit repo whose default branch
// ("main", fixturegit's own init default) carries the spec at mainStatus,
// then checks out feature/stale-decline at the same commit — the build
// branch `verdi feature start` would have cut, per internal/storyresolve.
// ResolveBuildSpec's convention.
func buildGateRepo(t *testing.T, mainStatus string) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                         "schema: verdi.layout/v1\nforge: gitlab\n",
				".verdi/specs/active/stale-decline/spec.md": gateSpecMD(mainStatus),
			},
			Message: "scaffold + spec at " + mainStatus,
		},
	})
	checkoutBranch(t, repo.Dir, "feature/stale-decline")
	return repo
}

func checkoutBranch(t *testing.T, dir, name string) {
	t.Helper()
	cmd := exec.Command("git", "checkout", "-b", name)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b %s: %v\n%s", name, err, out)
	}
}

// writeGateReport writes deviation-report.md directly to the working tree
// (never git-committed — condition 3 reads it via plain os.ReadFile, same
// as a real `verdi align` run before its own commit, if any) with the
// given covers sha and raw findings YAML block.
func writeGateReport(t *testing.T, root, covers, findingsYAML string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", "stale-decline")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := fmt.Sprintf(`---
schema: verdi.deviation/v1
covers: %s
findings:
%s
digest: sha256:%s
---
# Alignment report
`, covers, findingsYAML, strings.Repeat("0", 64))
	if err := os.WriteFile(filepath.Join(dir, "deviation-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing deviation-report.md: %v", err)
	}
}

const dispositionedFindingYAML = `  - { id: f-1, kind: computed, text: "boundary holds", disposition: fixed }
`

const undispositionedFindingYAML = `  - { id: f-1, kind: computed, text: "boundary holds" }
`

const dispositionedAbsenceFindingYAML = `  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no align.judge_cmd configured", disposition: accepted-deviation, note: "CI LLM plumbing not wired up yet" }
`

const undispositionedAbsenceFindingYAML = `  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no align.judge_cmd configured" }
`

// writeGateViolatingRecord writes a source:ci fail record bound to ac-1
// under the derived tree, keyed by store.RefSlug(spec.ID) and the exact
// commit named (its own ancestor, per gitx.IsAncestor's doc) — the shape
// evidence.LoadRecords/Fold need to fold ac-1 to violated.
func writeGateViolatingRecord(t *testing.T, root, headCommit string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "data", "derived", store.RefSlug("spec/stale-decline"), headCommit)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	record := fmt.Sprintf(`[{"schema":"verdi.evidence/v1","evidence_for":["ac-1"],"kind":"static","verdict":"fail","witness":"w","provenance":{"source":"ci","pipeline":"1","job":"1","commit":"%s"},"digest":"sha256:%s"}]`, headCommit, strings.Repeat("0", 64))
	if err := os.WriteFile(filepath.Join(dir, "verdicts.json"), []byte(record), 0o644); err != nil {
		t.Fatalf("writing verdicts.json: %v", err)
	}
}

func mustResolveBuildSpec(t *testing.T, root string) *artifact.SpecFrontmatter {
	t.Helper()
	spec, err := storyresolve.ResolveBuildSpec(root, "feature/stale-decline")
	if err != nil {
		t.Fatalf("ResolveBuildSpec: %v", err)
	}
	return spec
}

// TestGate_Condition1_FailsAlone proves condition 1 (spec status on the
// default branch) fails independently while 2 and 3 hold.
func TestGate_Condition1_FailsAlone(t *testing.T) {
	repo := buildGateRepo(t, "draft")
	spec := mustResolveBuildSpec(t, repo.Dir)
	writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)

	var stdout, stderr bytes.Buffer
	got := runGate(context.Background(), repo.Dir, spec, repo.Head, "main", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runGate = %d, want 1; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	assertConditionFails(t, stdout.String(), 1)
	assertConditionPasses(t, stdout.String(), 2)
	assertConditionPasses(t, stdout.String(), 3)
}

// TestGate_Condition2_FailsAlone proves condition 2 (no AC violated at
// head) fails independently while 1 and 3 hold.
func TestGate_Condition2_FailsAlone(t *testing.T) {
	repo := buildGateRepo(t, "accepted-pending-build")
	spec := mustResolveBuildSpec(t, repo.Dir)
	writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
	writeGateViolatingRecord(t, repo.Dir, repo.Head)

	var stdout, stderr bytes.Buffer
	got := runGate(context.Background(), repo.Dir, spec, repo.Head, "main", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runGate = %d, want 1; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	assertConditionPasses(t, stdout.String(), 1)
	assertConditionFails(t, stdout.String(), 2)
	assertConditionPasses(t, stdout.String(), 3)
}

// TestGate_Condition3_FailsAlone covers every condition-3 failure mode
// (no report, stale covers, an undispositioned finding) independently,
// while 1 and 2 hold in each case.
func TestGate_Condition3_FailsAlone(t *testing.T) {
	cases := map[string]func(t *testing.T, root, head string){
		"no report at all": func(t *testing.T, root, head string) {},
		"stale covers": func(t *testing.T, root, head string) {
			writeGateReport(t, root, "0000000000000000000000000000000000000b", dispositionedFindingYAML)
		},
		"undispositioned finding": func(t *testing.T, root, head string) {
			writeGateReport(t, root, head, undispositionedFindingYAML)
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			repo := buildGateRepo(t, "accepted-pending-build")
			spec := mustResolveBuildSpec(t, repo.Dir)
			setup(t, repo.Dir, repo.Head)

			var stdout, stderr bytes.Buffer
			got := runGate(context.Background(), repo.Dir, spec, repo.Head, "main", &stdout, &stderr)
			if got != 1 {
				t.Fatalf("runGate = %d, want 1; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
			}
			assertConditionPasses(t, stdout.String(), 1)
			assertConditionPasses(t, stdout.String(), 2)
			assertConditionFails(t, stdout.String(), 3)
		})
	}
}

// TestGate_AbsenceFindingMustBeDispositioned proves the no-judge path's
// synthetic finding counts toward condition 3 like any other finding: gate
// fails while it is undispositioned and passes once it carries
// accepted-deviation + a note (I-9's ratified reading).
func TestGate_AbsenceFindingMustBeDispositioned(t *testing.T) {
	t.Run("undispositioned absence finding fails the gate", func(t *testing.T) {
		repo := buildGateRepo(t, "accepted-pending-build")
		spec := mustResolveBuildSpec(t, repo.Dir)
		writeGateReport(t, repo.Dir, repo.Head, undispositionedAbsenceFindingYAML)

		var stdout, stderr bytes.Buffer
		got := runGate(context.Background(), repo.Dir, spec, repo.Head, "main", &stdout, &stderr)
		if got != 1 {
			t.Fatalf("runGate = %d, want 1; stdout=%s", got, stdout.String())
		}
		assertConditionFails(t, stdout.String(), 3)
		if !strings.Contains(stdout.String(), "judged-coverage-absent") {
			t.Fatalf("stdout = %q, want it to name judged-coverage-absent", stdout.String())
		}
	})

	t.Run("accepted-deviation absence finding satisfies the gate", func(t *testing.T) {
		repo := buildGateRepo(t, "accepted-pending-build")
		spec := mustResolveBuildSpec(t, repo.Dir)
		writeGateReport(t, repo.Dir, repo.Head, dispositionedAbsenceFindingYAML)

		var stdout, stderr bytes.Buffer
		got := runGate(context.Background(), repo.Dir, spec, repo.Head, "main", &stdout, &stderr)
		if got != 0 {
			t.Fatalf("runGate = %d, want 0; stdout=%s", got, stdout.String())
		}
	})
}

// TestGate_AllHold proves all three conditions passing exits 0.
func TestGate_AllHold(t *testing.T) {
	repo := buildGateRepo(t, "accepted-pending-build")
	spec := mustResolveBuildSpec(t, repo.Dir)
	writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)

	var stdout, stderr bytes.Buffer
	got := runGate(context.Background(), repo.Dir, spec, repo.Head, "main", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runGate = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	assertConditionPasses(t, stdout.String(), 1)
	assertConditionPasses(t, stdout.String(), 2)
	assertConditionPasses(t, stdout.String(), 3)
	if !strings.Contains(stdout.String(), "gate: PASS") {
		t.Fatalf("stdout = %q, want a final gate: PASS line", stdout.String())
	}
}

// TestGate_UnknownDefaultBranch_FailsClosed proves condition 1 fails
// closed (never silently passes) when the default branch cannot be
// determined at all — I-14's "otherwise, can't prove it".
func TestGate_UnknownDefaultBranch_FailsClosed(t *testing.T) {
	repo := buildGateRepo(t, "accepted-pending-build")
	spec := mustResolveBuildSpec(t, repo.Dir)
	writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)

	var stdout, stderr bytes.Buffer
	got := runGate(context.Background(), repo.Dir, spec, repo.Head, "", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runGate(unknown default branch) = %d, want 1", got)
	}
	assertConditionFails(t, stdout.String(), 1)
}

// TestCmdGate_EntryPoint drives the real verdi gate entry point
// (dispatch.go's route, current-branch/CI-env inference included), proving
// the full wiring — not just runGate's testable core — works end to end.
func TestCmdGate_EntryPoint(t *testing.T) {
	repo := buildGateRepo(t, "accepted-pending-build")
	writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
	t.Chdir(repo.Dir)
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	var stderr bytes.Buffer
	got := run([]string{"gate"}, &stderr)
	if got != 0 {
		t.Fatalf("run([gate]) = %d, want 0; stderr=%s", got, stderr.String())
	}
}

// gateSpikeSpecMD renders a spike story spec (class story, spike: true, no
// acceptance criteria, one resolves edge) at accepted-pending-build.
func gateSpikeSpecMD() string {
	return fmt.Sprintf(`---
id: spec/enum-spike
kind: spec
class: story
title: "Enumeration spike"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1490
spike: true
problem: { text: "which enumeration approach is right", anchor: "#problem" }
outcome: { text: "a recommendation recorded", anchor: "#outcome" }
links:
  - { type: resolves, ref: "spec/some-feature#oq-1" }
frozen: { at: 2024-01-01, commit: %s }
---
# body
`, gateFakeFrozenCommit)
}

// TestGate_SpikeBranch_EvidenceExempt is D-6's regression: a spike build
// branch has zero acceptance criteria, which used to hard-error the
// condition-2 fold (exit 2, the gate inoperable). It must now DISCLOSE the
// evidence exemption (03 §Ceremony pricing) — never a silent pass — while
// conditions 1/3/4 still decide the verdict. With all three of those
// holding, gate exits 0.
func TestGate_SpikeBranch_EvidenceExempt(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                      "schema: verdi.layout/v1\nforge: gitlab\n",
				".verdi/specs/active/enum-spike/spec.md": gateSpikeSpecMD(),
			},
			Message: "scaffold + spike spec",
		},
	})
	checkoutBranch(t, repo.Dir, "feature/enum-spike")

	spec, err := storyresolve.ResolveBuildSpec(repo.Dir, "feature/enum-spike")
	if err != nil {
		t.Fatalf("ResolveBuildSpec: %v", err)
	}

	// A fresh, fully-dispositioned alignment report for condition 3.
	dir := filepath.Join(repo.Dir, ".verdi", "specs", "active", "enum-spike")
	report := fmt.Sprintf(`---
schema: verdi.deviation/v1
covers: %s
findings:
%s
digest: sha256:%s
---
# Alignment report
`, repo.Head, dispositionedFindingYAML, strings.Repeat("0", 64))
	if err := os.WriteFile(filepath.Join(dir, "deviation-report.md"), []byte(report), 0o644); err != nil {
		t.Fatalf("writing deviation-report.md: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := runGate(context.Background(), repo.Dir, spec, repo.Head, "main", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runGate(spike) = %d, want 0 (evidence-exempt, not inoperable); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	out := stdout.String()
	// Condition 2 is disclosed, not passed or failed.
	if strings.Contains(out, "[PASS] 2.") || strings.Contains(out, "[FAIL] 2.") {
		t.Fatalf("condition 2 must be disclosed for a spike, not pass/fail:\n%s", out)
	}
	if !strings.Contains(out, "disclosed-unproven [gate:spike-evidence-exempt]") {
		t.Fatalf("stdout missing the spike evidence-exempt disclosure line:\n%s", out)
	}
	assertConditionPasses(t, out, 1)
	assertConditionPasses(t, out, 3)
	assertConditionPasses(t, out, 4)
	if !strings.Contains(out, "gate: PASS") {
		t.Fatalf("stdout = %q, want gate: PASS", out)
	}
}

func assertConditionFails(t *testing.T, stdout string, n int) {
	t.Helper()
	if !strings.Contains(stdout, fmt.Sprintf("[FAIL] %d.", n)) {
		t.Fatalf("stdout = %q, want condition %d to FAIL", stdout, n)
	}
}

func assertConditionPasses(t *testing.T, stdout string, n int) {
	t.Helper()
	if !strings.Contains(stdout, fmt.Sprintf("[PASS] %d.", n)) {
		t.Fatalf("stdout = %q, want condition %d to PASS", stdout, n)
	}
}
