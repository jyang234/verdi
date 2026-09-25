// SI-267 (owner directive 2026-09-25: "parallelize it like merge-gate"):
// verify.yml, the push-side gate that also produces verdi's self-hosted
// evidence, runs merge-gate.yml's parallel gate jobs unchanged, plus one final
// job keyed `verify` that builds the binary, runs `verdi sync --produce`, and
// uploads the "verdi-evidence" artifact. That job needs every gate job and has
// no `if:`, so GitHub's default success() skips it unless every gate job of
// the same run succeeded. The evidence's honesty basis becomes "the same
// workflow run, at the same commit, after every gate job of that run
// succeeded" instead of "the same job, after `make verify` exited 0".
//
// This file is the compensating control SI-267 names. verifyWorkflowViolations
// fails a verify.yml whose:
//
//   - top-level keys are anything but name/on/jobs. A workflow-level `env:`
//     such as MAKEFLAGS=-i would change what every gate job runs while each
//     job stayed identical to merge-gate.yml's;
//   - jobs are anything but merge-gate.yml's gate jobs plus `verify`;
//   - gate jobs differ from merge-gate.yml's in any key or value. They are
//     compared as decoded YAML trees, not through workflowJob's typed fields,
//     so an `env:`, `if:`, `timeout-minutes:`, `with:` entry, or any key the
//     typed decoder does not model counts. YAML comments and mapping key
//     order do not, since neither changes what GitHub runs;
//   - `verify` job declares any key but needs/runs-on/steps, needs anything
//     but exactly the gate jobs, or runs anything but, in order: checkout,
//     setup-go, the verdict call over every gate job's result, the binary
//     build, `verdi sync --produce`, and the upload. Each step passes the
//     step-level whitelist net (stepKeyProblem).
//
// The verdict call is defense in depth and a legible in-log witness of the
// results the evidence binds. It needs no canary, unlike merge-gate.yml's
// aggregator: `verify` cannot run over a failed need, so a verdict script
// broken into passing could not pass it over one.
//
// TestVerifyWorkflowRunsMergeGateJobsThenProducesEvidence applies the check to
// verify.yml. close_workflow_test.go applies it to the workflow
// close-evidence.yml calls. What remains a review obligation: verify.yml's
// `on:` block, which differs from merge-gate.yml's on purpose (a
// path-filtered push trigger with the close/** carve-out, plus
// workflow_call). close_workflow_test.go pins its own parts of it.
package specalign

import (
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// verifyEvidenceJob is verify.yml's evidence job. Its key is GITHUB_JOB there,
// which feeds the records' provenance.job_name (SI-229), so it stays `verify`.
const verifyEvidenceJob = "verify"

// verifyWorkflowTopKeys is verify.yml's whole top-level key set.
var verifyWorkflowTopKeys = []string{"jobs", "name", "on"}

// evidenceJobKeys is the evidence job's whole key set. An `if:` (always()
// above all) would let it produce evidence over a failed gate job,
// `continue-on-error:` would report a failed production green, and `name:`,
// `strategy:`, or `uses:` change what runs or what the job reports as.
var evidenceJobKeys = []string{"needs", "runs-on", "steps"}

// The evidence job's production command and upload: the artifact name
// close's `verdi sync` fetches by, and the directory `verdi sync --produce`
// writes the bundle to.
const (
	evidenceProduceRun   = "./.build/verdi sync --produce"
	evidenceArtifactName = "verdi-evidence"
	evidenceArtifactPath = ".verdi/data/derived/"
)

// evidenceJobSteps is the evidence job's whole step list, in SI-267's order:
// a full-history checkout, Go 1.25 (the gate jobs' pinned inputs), the
// verdict call over every gate job's result, the binary build the static
// job's self-lint also uses, `verdi sync --produce`, and the upload. Step
// names are free; stepKeyProblem allows `name:` and nothing else beside them.
func evidenceJobSteps(gates []string) []workflowStep {
	// The golangci-lint pin feeds only the cache step's key, which is not
	// read here.
	setup := pinnedSetupActions("")
	return []workflowStep{
		{Uses: "actions/checkout@v4", With: setup["actions/checkout@v4"]},
		{Uses: "actions/setup-go@v5", With: setup["actions/setup-go@v5"]},
		{Run: verdictRunFor(gates)},
		{Run: mergeGatePostVerifyCommands[0]},
		{Run: evidenceProduceRun},
		{Uses: "actions/upload-artifact@v4", With: map[string]string{
			"name":              evidenceArtifactName,
			"path":              evidenceArtifactPath,
			"if-no-files-found": "error",
		}},
	}
}

// describeStep renders what a step runs, for failure messages.
func describeStep(step workflowStep) string {
	if step.Uses != "" {
		return fmt.Sprintf("uses %s with %v", step.Uses, step.With)
	}
	return fmt.Sprintf("run %q", strings.TrimSpace(step.Run))
}

// treeDiff is one place where two decoded YAML trees differ: the path to it
// and each side's value there, rendered by renderTreeValue.
type treeDiff struct {
	Path      string
	Got, Want string
}

// renderTreeValue renders a decoded YAML value for a treeDiff. present is
// false when the key does not exist on that side, which is not the same as an
// explicit null.
func renderTreeValue(v interface{}, present bool) string {
	if !present {
		return "nothing"
	}
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return strconv.Quote(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

// treeDiffs returns every place where got and want, two decoded YAML trees,
// differ, in path order. Mappings are compared key by key over the union of
// their keys, so a key on either side alone is a difference; sequences are
// compared item by item, with a length difference reported at the sequence's
// own path; scalars must be equal in type as well as value, so `0` and "0"
// differ. Mapping key order is not part of a decoded tree, and so it never
// differs.
func treeDiffs(path string, got, want interface{}) []treeDiff {
	gotMap, gotIsMap := asMap(got)
	wantMap, wantIsMap := asMap(want)
	if gotIsMap && wantIsMap {
		var out []treeDiff
		keys := sortedKeys(gotMap)
		for k := range wantMap {
			if _, ok := gotMap[k]; !ok {
				keys = append(keys, k)
			}
		}
		slices.Sort(keys)
		for _, k := range keys {
			g, gotHas := gotMap[k]
			w, wantHas := wantMap[k]
			sub := path + "." + k
			if gotHas && wantHas {
				out = append(out, treeDiffs(sub, g, w)...)
				continue
			}
			out = append(out, treeDiff{Path: sub, Got: renderTreeValue(g, gotHas), Want: renderTreeValue(w, wantHas)})
		}
		return out
	}
	gotSeq, gotIsSeq := asSlice(got)
	wantSeq, wantIsSeq := asSlice(want)
	if gotIsSeq && wantIsSeq {
		var out []treeDiff
		if len(gotSeq) != len(wantSeq) {
			out = append(out, treeDiff{Path: path, Got: fmt.Sprintf("%d items", len(gotSeq)), Want: fmt.Sprintf("%d items", len(wantSeq))})
		}
		for i := range min(len(gotSeq), len(wantSeq)) {
			out = append(out, treeDiffs(fmt.Sprintf("%s[%d]", path, i), gotSeq[i], wantSeq[i])...)
		}
		return out
	}
	if reflect.DeepEqual(got, want) {
		return nil
	}
	return []treeDiff{{Path: path, Got: renderTreeValue(got, true), Want: renderTreeValue(want, true)}}
}

// gateJobDiffs compares verify.yml's jobs with merge-gate.yml's gate jobs. It
// reports each gate job verify.yml lacks, each place a gate job's decoded tree
// differs between the two files, and each verify.yml job that is neither a
// gate job nor the evidence job.
func gateJobDiffs(verifyJobs, mergeGateJobs map[string]interface{}, gates []string) []string {
	if len(gates) == 0 {
		return []string{"merge-gate.yml declares no gate jobs, so there is nothing for verify.yml's gate jobs to equal"}
	}
	var out []string
	for _, g := range gates {
		got, ok := verifyJobs[g]
		if !ok {
			out = append(out, fmt.Sprintf("verify.yml has no job %q; it must run every merge-gate.yml gate job %v before the evidence job", g, gates))
			continue
		}
		for _, d := range treeDiffs("jobs."+g, got, mergeGateJobs[g]) {
			out = append(out, fmt.Sprintf("verify.yml's gate job %q must equal merge-gate.yml's in every key and value; at %s verify.yml has %s, merge-gate.yml has %s", g, d.Path, d.Got, d.Want))
		}
	}
	for _, k := range sortedKeys(verifyJobs) {
		if k != verifyEvidenceJob && !slices.Contains(gates, k) {
			out = append(out, fmt.Sprintf("verify.yml job %q is neither one of merge-gate.yml's gate jobs %v nor the evidence job %q", k, gates, verifyEvidenceJob))
		}
	}
	return out
}

// evidenceJobViolations checks verify.yml's evidence job against gates: its
// key set, its runner, its `needs:`, and its whole step list.
func evidenceJobViolations(job workflowJob, gates []string) []string {
	var out []string
	if !slices.Equal(job.Keys, evidenceJobKeys) {
		out = append(out, fmt.Sprintf("verify.yml job %q must declare exactly the keys %v, got %v (extra: %v): an `if:` such as always() would let it produce evidence over a failed gate job, and `continue-on-error:` would report a failed production green", verifyEvidenceJob, evidenceJobKeys, job.Keys, keysOutside(job.Keys, evidenceJobKeys)))
	}
	if job.RunsOn != "ubuntu-latest" {
		out = append(out, fmt.Sprintf("verify.yml job %q runs-on %q, want ubuntu-latest", verifyEvidenceJob, job.RunsOn))
	}
	if needs := slices.Sorted(slices.Values(job.Needs)); !slices.Equal(needs, gates) {
		out = append(out, fmt.Sprintf("verify.yml job %q needs %v, want exactly the gate jobs %v: evidence produced while a gate job outside `needs:` failed would bind a failed gate", verifyEvidenceJob, needs, gates))
	}
	want := evidenceJobSteps(gates)
	if len(job.Steps) != len(want) {
		described := make([]string, len(want))
		for i, w := range want {
			described[i] = describeStep(w)
		}
		out = append(out, fmt.Sprintf("verify.yml job %q runs %d steps, want exactly these %d in order: %s", verifyEvidenceJob, len(job.Steps), len(want), strings.Join(described, "; ")))
	}
	for i, step := range job.Steps {
		if problem := stepKeyProblem(step); problem != "" {
			out = append(out, fmt.Sprintf("verify.yml job %q step %d %s", verifyEvidenceJob, i, problem))
		}
		if i >= len(want) {
			continue
		}
		if w := want[i]; step.Uses != w.Uses || !maps.Equal(step.With, w.With) || strings.TrimSpace(step.Run) != w.Run {
			out = append(out, fmt.Sprintf("verify.yml job %q step %d must be: %s; got: %s", verifyEvidenceJob, i, describeStep(w), describeStep(step)))
		}
	}
	return out
}

// verifyWorkflowViolations returns every way the workflow source verifyRaw
// departs from SI-267's shape for verify.yml, measured against merge-gate.yml's
// source mergeGateRaw (this file's package comment lists the checks). An
// error means a source did not decode at all.
func verifyWorkflowViolations(verifyRaw, mergeGateRaw []byte) ([]string, error) {
	verifyTop, err := decodeWorkflowTree(verifyRaw)
	if err != nil {
		return nil, fmt.Errorf("verify.yml: %w", err)
	}
	mergeGateTop, err := decodeWorkflowTree(mergeGateRaw)
	if err != nil {
		return nil, fmt.Errorf("merge-gate.yml: %w", err)
	}

	var out []string
	if keys := sortedKeys(verifyTop); !slices.Equal(keys, verifyWorkflowTopKeys) {
		out = append(out, fmt.Sprintf("verify.yml must declare exactly the top-level keys %v, got %v (extra: %v): a workflow-level `env:` (MAKEFLAGS=-i makes make ignore failed recipes), `defaults:`, `concurrency:`, or `permissions:` changes what every gate job runs while each stays identical to merge-gate.yml's", verifyWorkflowTopKeys, keys, keysOutside(keys, verifyWorkflowTopKeys)))
	}
	verifyJobs, _ := asMap(verifyTop["jobs"])
	mergeGateJobs, _ := asMap(mergeGateTop["jobs"])
	gates := gateJobKeys(decodeJobs(mergeGateTop["jobs"]))
	out = append(out, gateJobDiffs(verifyJobs, mergeGateJobs, gates)...)

	evidence, ok := verifyJobs[verifyEvidenceJob]
	if !ok {
		return append(out, fmt.Sprintf("verify.yml has no %q job: the evidence job's key is GITHUB_JOB there, which feeds provenance.job_name (SI-229)", verifyEvidenceJob)), nil
	}
	return append(out, evidenceJobViolations(decodeJob(evidence), gates)...), nil
}

// verifyWorkflowFileViolations applies verifyWorkflowViolations to the
// workflow file at path, measured against this checkout's merge-gate.yml.
func verifyWorkflowFileViolations(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	mergeGate, err := os.ReadFile(mergeGatePath(verdiRepoRoot))
	if err != nil {
		t.Fatalf("reading merge-gate.yml: %v", err)
	}
	violations, err := verifyWorkflowViolations(raw, mergeGate)
	if err != nil {
		t.Fatalf("checking %s: %v", path, err)
	}
	return violations
}

// TestVerifyWorkflowRunsMergeGateJobsThenProducesEvidence proves verify.yml
// has SI-267's shape: merge-gate.yml's gate jobs, identical in every key and
// value, and one evidence job that needs all of them, carries no `if:` or
// `continue-on-error:`, and runs exactly checkout, setup-go, the verdict call,
// the build, `verdi sync --produce`, and the upload, in that order.
func TestVerifyWorkflowRunsMergeGateJobsThenProducesEvidence(t *testing.T) {
	for _, v := range verifyWorkflowFileViolations(t, workflowPath(verdiRepoRoot, "verify.yml")) {
		t.Error(v)
	}
}

// The fixture pair below is a two-gate-job merge-gate.yml and the verify.yml
// SI-267 requires beside it. The gate jobs are one shared text, so the pair
// starts identical, and they carry each value kind the real gate jobs do: an
// action step with `with:`, a named block-scalar `run:`, and plain `run:`s.
const fixtureGateJobs = `  static:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - name: Install a tool
        run: |
          test -x tool || install tool
          echo installed
      - run: make build
      - run: make lint
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - run: make e2e
`

const fixtureMergeGate = `name: merge-gate
on:
  pull_request: {}
jobs:
` + fixtureGateJobs + `  merge-gate:
    needs: [static, e2e]
    if: always()
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: scripts/merge-gate-verdict.sh e2e=${{ needs.e2e.result }} static=${{ needs.static.result }}
`

const fixtureVerify = `name: verify
on:
  push:
    paths:
      - "**.go"
  workflow_call: {}
jobs:
` + fixtureGateJobs + `  verify:
    needs: [static, e2e]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
      - name: Require every gate job to succeed
        run: scripts/merge-gate-verdict.sh e2e=${{ needs.e2e.result }} static=${{ needs.static.result }}
      - run: go build -o .build/verdi ./cmd/verdi
      - name: Produce the evidence bundle
        run: ./.build/verdi sync --produce
      - uses: actions/upload-artifact@v4
        with:
          name: verdi-evidence
          path: .verdi/data/derived/
          if-no-files-found: error
`

// TestVerifyWorkflowViolations is verifyWorkflowViolations' happy and negative
// paths, over mutated in-memory copies of the fixture pair. Each negative case
// names a substring one of its violations must contain.
func TestVerifyWorkflowViolations(t *testing.T) {
	verify := func(from, to string) string {
		return mutateSource(t, "the verify.yml fixture", fixtureVerify, from, to)
	}
	mergeGate := func(from, to string) string {
		return mutateSource(t, "the merge-gate.yml fixture", fixtureMergeGate, from, to)
	}
	const (
		e2eJob          = "  e2e:\n    runs-on: ubuntu-latest\n"
		e2eJobWhole     = e2eJob + "    steps:\n      - uses: actions/checkout@v4\n        with:\n          fetch-depth: 0\n      - run: make e2e\n"
		evidenceNeeds   = "    needs: [static, e2e]\n"
		verdictStep     = "      - name: Require every gate job to succeed\n        run: scripts/merge-gate-verdict.sh e2e=${{ needs.e2e.result }} static=${{ needs.static.result }}\n"
		produceStep     = "      - name: Produce the evidence bundle\n        run: ./.build/verdi sync --produce\n"
		uploadStep      = "      - uses: actions/upload-artifact@v4\n        with:\n          name: verdi-evidence\n          path: .verdi/data/derived/\n          if-no-files-found: error\n"
		evidenceSetupGo = "fetch-depth: 0\n      - uses: actions/setup-go@v5\n"
	)
	cases := []struct {
		name      string
		verify    string
		mergeGate string // "" means fixtureMergeGate
		want      string // a substring of one violation; "" wants none
	}{
		{name: "the fixture pair as written", verify: fixtureVerify},
		{name: "a YAML comment only verify.yml carries", verify: verify("  static:\n", "  static:\n    # only verify.yml says this\n")},
		{name: "a gate job's keys in another order", verify: verify(e2eJobWhole, "  e2e:\n    steps:\n      - uses: actions/checkout@v4\n        with:\n          fetch-depth: 0\n      - run: make e2e\n    runs-on: ubuntu-latest\n")},

		{name: "an extra key on a gate job", verify: verify(e2eJob, e2eJob+"    timeout-minutes: 5\n"), want: "at jobs.e2e.timeout-minutes verify.yml has 5, merge-gate.yml has nothing"},
		{name: "an if: on a gate job", verify: verify(e2eJob, e2eJob+"    if: always()\n"), want: "at jobs.e2e.if"},
		{name: "a continue-on-error: on a gate job", verify: verify(e2eJob, e2eJob+"    continue-on-error: true\n"), want: "at jobs.e2e.continue-on-error"},
		{name: "an env: on a gate step", verify: verify("      - run: make build\n", "      - run: make build\n        env:\n          MAKEFLAGS: -i\n"), want: "at jobs.static.steps[2].env"},
		{name: "an added with: entry", verify: verify("fetch-depth: 0\n      - run: make e2e", "fetch-depth: 0\n          ref: main\n      - run: make e2e"), want: "at jobs.e2e.steps[0].with.ref"},
		{name: "a with: value of another type", verify: verify("fetch-depth: 0\n      - run: make e2e", "fetch-depth: \"0\"\n      - run: make e2e"), want: `at jobs.e2e.steps[0].with.fetch-depth verify.yml has "0", merge-gate.yml has 0`},
		{name: "a changed step run:", verify: verify("      - run: make lint\n", "      - run: make lint || true\n"), want: `at jobs.static.steps[3].run verify.yml has "make lint || true", merge-gate.yml has "make lint"`},
		{name: "a changed block-scalar run:", verify: verify("          echo installed\n", "          echo skipped\n"), want: "at jobs.static.steps[1].run"},
		{name: "a dropped step", verify: verify("      - run: make lint\n", ""), want: "at jobs.static.steps verify.yml has 3 items, merge-gate.yml has 4 items"},
		{name: "a reordered step", verify: verify("      - run: make build\n      - run: make lint\n", "      - run: make lint\n      - run: make build\n"), want: `at jobs.static.steps[2].run verify.yml has "make lint", merge-gate.yml has "make build"`},
		{name: "a missing gate job", verify: verify(e2eJobWhole, ""), want: `verify.yml has no job "e2e"`},
		{name: "an extra job in verify.yml", verify: verify("jobs:\n", "jobs:\n  lint-again:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make lint\n"), want: `verify.yml job "lint-again" is neither`},
		{name: "a gate job changed on merge-gate.yml's side", verify: fixtureVerify, mergeGate: mergeGate(e2eJob, e2eJob+"    timeout-minutes: 5\n"), want: "at jobs.e2e.timeout-minutes verify.yml has nothing, merge-gate.yml has 5"},
		{name: "a gate job merge-gate.yml added", verify: fixtureVerify, mergeGate: mergeGate("jobs:\n", "jobs:\n  vet:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make vet\n"), want: `verify.yml has no job "vet"`},
		{name: "a workflow-level env:", verify: verify("jobs:\n", "env:\n  MAKEFLAGS: -i\njobs:\n"), want: "top-level keys [jobs name on], got [env jobs name on]"},

		{name: "no evidence job", verify: verify("  verify:\n", "  produce:\n"), want: `verify.yml has no "verify" job`},
		{name: "if: always() on the evidence job", verify: verify(evidenceNeeds, evidenceNeeds+"    if: always()\n"), want: "(extra: [if])"},
		{name: "continue-on-error: on the evidence job", verify: verify(evidenceNeeds, evidenceNeeds+"    continue-on-error: true\n"), want: "(extra: [continue-on-error])"},
		{name: "a name: on the evidence job", verify: verify(evidenceNeeds, evidenceNeeds+"    name: Evidence\n"), want: "(extra: [name])"},
		{name: "the evidence job on another runner", verify: verify(evidenceNeeds+"    runs-on: ubuntu-latest\n", evidenceNeeds+"    runs-on: macos-latest\n"), want: `runs-on "macos-latest"`},
		{name: "the evidence job needs one gate job", verify: verify(evidenceNeeds, "    needs: [static]\n"), want: "needs [static], want exactly the gate jobs [e2e static]"},
		{name: "the evidence job needs no gate job", verify: verify(evidenceNeeds, ""), want: "needs [], want exactly the gate jobs [e2e static]"},
		{name: "a shallow evidence checkout", verify: verify(evidenceSetupGo, "{}\n      - uses: actions/setup-go@v5\n"), want: "step 0 must be: uses actions/checkout@v4 with map[fetch-depth:0]; got: uses actions/checkout@v4 with map[]"},
		{name: "the verdict omits a gate job", verify: verify("run: scripts/merge-gate-verdict.sh e2e=${{ needs.e2e.result }} static=", "run: scripts/merge-gate-verdict.sh static="), want: "step 2 must be: run \"scripts/merge-gate-verdict.sh e2e=${{ needs.e2e.result }} static=${{ needs.static.result }}\"; got: run \"scripts/merge-gate-verdict.sh static="},
		{name: "no verdict step", verify: verify(verdictStep, ""), want: "runs 5 steps, want exactly these 6"},
		{name: "production before the verdict", verify: verify(verdictStep+"      - run: go build -o .build/verdi ./cmd/verdi\n"+produceStep, "      - run: go build -o .build/verdi ./cmd/verdi\n"+produceStep+verdictStep), want: "step 2 must be: run \"scripts/merge-gate-verdict.sh"},
		{name: "the upload before production", verify: verify(produceStep+uploadStep, uploadStep+produceStep), want: "step 4 must be: run \"./.build/verdi sync --produce\"; got: uses actions/upload-artifact@v4"},
		{name: "production twice", verify: verify(produceStep, produceStep+produceStep), want: "runs 7 steps, want exactly these 6"},
		{name: "production over its own failure", verify: verify("run: ./.build/verdi sync --produce\n", "run: ./.build/verdi sync --produce || true\n"), want: "step 4 must be: run \"./.build/verdi sync --produce\"; got: run \"./.build/verdi sync --produce || true\""},
		{name: "the upload names another artifact", verify: verify("name: verdi-evidence\n", "name: other-evidence\n"), want: "step 5 must be: uses actions/upload-artifact@v4"},
		{name: "the upload takes another path", verify: verify("path: .verdi/data/derived/\n", "path: .verdi/data/\n"), want: "step 5 must be: uses actions/upload-artifact@v4"},
		{name: "an if: on an evidence step", verify: verify("      - uses: actions/upload-artifact@v4\n", "      - uses: actions/upload-artifact@v4\n        if: always()\n"), want: "step 5 (uses \"actions/upload-artifact@v4\"): key(s) [if] are not whitelisted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mergeGateSource := tc.mergeGate
			if mergeGateSource == "" {
				mergeGateSource = fixtureMergeGate
			}
			violations, err := verifyWorkflowViolations([]byte(tc.verify), []byte(mergeGateSource))
			if err != nil {
				t.Fatalf("verifyWorkflowViolations: %v", err)
			}
			if tc.want == "" {
				if len(violations) != 0 {
					t.Fatalf("violations = %q, want none", violations)
				}
				return
			}
			if !slices.ContainsFunc(violations, func(v string) bool { return strings.Contains(v, tc.want) }) {
				t.Fatalf("violations = %q, want one containing %q", violations, tc.want)
			}
		})
	}
}

// TestVerifyWorkflowViolationsRefusesUndecodableSources is the operational
// negative path: a source that is not a YAML mapping is an error, never an
// empty (passing) violation list.
func TestVerifyWorkflowViolationsRefusesUndecodableSources(t *testing.T) {
	cases := []struct {
		name              string
		verify, mergeGate string
		want              string
	}{
		{"verify.yml is not YAML", "jobs: [", fixtureMergeGate, "verify.yml"},
		{"verify.yml is a sequence", "- a\n- b\n", fixtureMergeGate, "verify.yml: top-level document is not a mapping"},
		{"merge-gate.yml is not YAML", fixtureVerify, "jobs: [", "merge-gate.yml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			violations, err := verifyWorkflowViolations([]byte(tc.verify), []byte(tc.mergeGate))
			if err == nil {
				t.Fatalf("verifyWorkflowViolations = %q, nil error; want an error", violations)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}

// TestTreeDiffs is treeDiffs' happy and negative paths over hand-built trees
// of the shapes the loose decoder produces.
func TestTreeDiffs(t *testing.T) {
	cases := []struct {
		name      string
		got, want interface{}
		wantDiffs []treeDiff
	}{
		{name: "equal scalars", got: "make e2e", want: "make e2e"},
		{name: "equal trees", got: map[string]interface{}{"steps": []interface{}{map[string]interface{}{"run": "make e2e"}}}, want: map[string]interface{}{"steps": []interface{}{map[string]interface{}{"run": "make e2e"}}}},
		{name: "a string key map equals the same interface key map", got: map[interface{}]interface{}{"a": 1}, want: map[string]interface{}{"a": 1}},
		{name: "different scalars", got: "make lint", want: "make e2e", wantDiffs: []treeDiff{{"x", `"make lint"`, `"make e2e"`}}},
		{name: "same text, different type", got: "0", want: 0, wantDiffs: []treeDiff{{"x", `"0"`, "0"}}},
		{name: "null against a value", got: nil, want: "x", wantDiffs: []treeDiff{{"x", "null", `"x"`}}},
		{name: "a key only got has", got: map[string]interface{}{"a": 1, "b": nil}, want: map[string]interface{}{"a": 1}, wantDiffs: []treeDiff{{"x.b", "null", "nothing"}}},
		{name: "a key only want has", got: map[string]interface{}{}, want: map[string]interface{}{"if": "always()"}, wantDiffs: []treeDiff{{"x.if", "nothing", `"always()"`}}},
		{name: "a nested difference", got: map[string]interface{}{"steps": []interface{}{map[string]interface{}{"run": "a"}}}, want: map[string]interface{}{"steps": []interface{}{map[string]interface{}{"run": "b"}}}, wantDiffs: []treeDiff{{"x.steps[0].run", `"a"`, `"b"`}}},
		{name: "sequences of different length", got: []interface{}{"a"}, want: []interface{}{"a", "b"}, wantDiffs: []treeDiff{{"x", "1 items", "2 items"}}},
		{name: "a mapping against a scalar", got: map[string]interface{}{"a": 1}, want: "a", wantDiffs: []treeDiff{{"x", "map[a:1]", `"a"`}}},
		{name: "a sequence against a mapping", got: []interface{}{"a"}, want: map[string]interface{}{"a": 1}, wantDiffs: []treeDiff{{"x", "[a]", "map[a:1]"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := treeDiffs("x", tc.got, tc.want); !slices.Equal(got, tc.wantDiffs) {
				t.Errorf("treeDiffs = %+v, want %+v", got, tc.wantDiffs)
			}
		})
	}
}
