package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/specstate"
)

// specStateExactPaymentsMD is a minimal, valid, STATUSLESS active feature
// spec — Task 4's compatibility grammar, landed via fixturegit so its exact
// bytes are reachable from the default branch.
const specStateExactPaymentsMD = `---
id: spec/payments
kind: spec
class: feature
title: "Payments"
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
---
# body
`

func buildSpecStateRepo(t *testing.T, files map[string]string) *fixturegit.Repo {
	t.Helper()
	base := map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: gitlab\n"}
	for k, v := range files {
		base[k] = v
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: base, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	t.Chdir(repo.Dir)
	return repo
}

// TestCmdSpecState_ExactAcceptedPendingBuild proves the read-only surface
// over the shared internal/specstate.Projector: a statusless exact
// default-branch spec resolves accepted-pending-build/exact, changes
// neither HEAD nor `git status --porcelain`, and emits exactly one
// canonical JSON line naming a full Git baseline identity.
func TestCmdSpecState_ExactAcceptedPendingBuild(t *testing.T) {
	repo := buildSpecStateRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": specStateExactPaymentsMD})

	headBefore := currentHead(t, repo.Dir)
	statusBefore := porcelainStatus(t, repo.Dir)

	var stdout, stderr bytes.Buffer
	got := cmdSpecState([]string{"spec/payments"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdSpecState = %d, want 0; stderr=%s", got, stderr.String())
	}

	if headBefore != currentHead(t, repo.Dir) {
		t.Fatalf("HEAD changed: before=%s after=%s", headBefore, currentHead(t, repo.Dir))
	}
	if statusBefore != porcelainStatus(t, repo.Dir) {
		t.Fatalf("git status --porcelain changed: before=%q after=%q", statusBefore, porcelainStatus(t, repo.Dir))
	}

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout = %q, want exactly one line", stdout.String())
	}

	var got2 specstate.Result
	if err := json.Unmarshal([]byte(lines[0]), &got2); err != nil {
		t.Fatalf("decoding stdout as JSON: %v\nstdout=%s", err, stdout.String())
	}
	if got2.State != "accepted-pending-build" || got2.Relation != "exact" || got2.Baseline.Path != ".verdi/specs/active/payments/spec.md" {
		t.Fatalf("spec state = %+v", got2)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(got2.Baseline.Blob) || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(got2.Baseline.LandingCommit) {
		t.Fatalf("baseline is not full Git identity: %+v", got2.Baseline)
	}
}

// TestCmdSpecState_CanonicalEncoding_SortedKeys proves the emitted JSON is
// this store's own canonical encoding seam (internal/canonjson) — sorted
// object keys, no hand-rolled JSON — rather than Go's default map/struct
// marshal order.
func TestCmdSpecState_CanonicalEncoding_SortedKeys(t *testing.T) {
	buildSpecStateRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": specStateExactPaymentsMD})

	var stdout, stderr bytes.Buffer
	got := cmdSpecState([]string{"spec/payments"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdSpecState = %d, want 0; stderr=%s", got, stderr.String())
	}
	line := strings.TrimRight(stdout.String(), "\n")
	// canonjson.Marshal sorts object keys: baseline < disclosures < relation < state.
	wantKeyOrder := []string{`"baseline"`, `"disclosures"`, `"relation"`, `"state"`}
	lastIdx := -1
	for _, k := range wantKeyOrder {
		idx := strings.Index(line, k)
		if idx == -1 {
			t.Fatalf("stdout line %q missing key %s", line, k)
		}
		if idx < lastIdx {
			t.Fatalf("stdout line %q keys are not sorted (canonjson seam not used)", line)
		}
		lastIdx = idx
	}
	if !strings.Contains(line, `"baseline":{`) {
		t.Fatalf("stdout = %q, want an object-valued baseline for an accepted spec", line)
	}
}

// specStateProposedMD is a valid feature spec that is NEVER landed on the
// default branch in this test — RelationNew/Proposed, so its Baseline must
// be null.
const specStateProposedMD = `---
id: spec/unmerged
kind: spec
class: feature
title: "Unmerged"
owners: [platform-team]
status: draft
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
---
# body
`

// TestCmdSpecState_Proposed_BaselineNull proves a proposed spec (never
// landed) emits baseline:null, exits 0, and mutates nothing.
func TestCmdSpecState_Proposed_BaselineNull(t *testing.T) {
	repo := buildSpecStateRepo(t, nil)
	writeTestFile(t, repo.Dir+"/.verdi/specs/active/unmerged/spec.md", []byte(specStateProposedMD))

	var stdout, stderr bytes.Buffer
	got := cmdSpecState([]string{"spec/unmerged"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdSpecState(proposed) = %d, want 0; stderr=%s", got, stderr.String())
	}
	line := strings.TrimRight(stdout.String(), "\n")
	if !strings.Contains(line, `"baseline":null`) {
		t.Fatalf("stdout = %q, want baseline:null for a proposed spec", line)
	}
	var got2 specstate.Result
	if err := json.Unmarshal([]byte(line), &got2); err != nil {
		t.Fatalf("decoding stdout as JSON: %v", err)
	}
	if got2.State != specstate.Proposed {
		t.Fatalf("state = %q, want proposed", got2.State)
	}
}

// TestCmdSpecState_Diverged_PartialBaseline is fix-round-1 finding 6's
// proof: a candidate whose local bytes DIVERGE from the default branch's
// own landed content at the same path is Proposed/diverged with a PARTIAL
// baseline — Path and Blob (the default branch's own object id) are
// populated, but Commit is always "" (no landing commit is ever computed
// for a diverged candidate — see this file's own doc comment). This is
// the honest witness of divergence, not a value this verb forgot to fill
// in, so it is never normalized away.
func TestCmdSpecState_Diverged_PartialBaseline(t *testing.T) {
	repo := buildSpecStateRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": specStateExactPaymentsMD})

	// Diverge the working-tree copy from what actually landed — never
	// re-committed.
	divergedPath := repo.Dir + "/.verdi/specs/active/payments/spec.md"
	diverged := append([]byte(specStateExactPaymentsMD), []byte("<!-- local, uncommitted edit -->\n")...)
	if err := os.WriteFile(divergedPath, diverged, 0o644); err != nil {
		t.Fatalf("diverging payments spec.md: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdSpecState([]string{"spec/payments"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdSpecState(diverged) = %d, want 0; stderr=%s", got, stderr.String())
	}
	line := strings.TrimRight(stdout.String(), "\n")

	var got2 specstate.Result
	if err := json.Unmarshal([]byte(line), &got2); err != nil {
		t.Fatalf("decoding stdout as JSON: %v\nstdout=%s", err, stdout.String())
	}
	if got2.State != specstate.Proposed || got2.Relation != specstate.RelationDiverged {
		t.Fatalf("spec state = %+v, want Proposed/diverged", got2)
	}
	if got2.Baseline == nil {
		t.Fatalf("Baseline = nil, want a partial baseline (Path/Blob populated, Commit empty)")
	}
	if got2.Baseline.Path != ".verdi/specs/active/payments/spec.md" {
		t.Fatalf("Baseline.Path = %q, want the candidate's own path", got2.Baseline.Path)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(got2.Baseline.Blob) {
		t.Fatalf("Baseline.Blob = %q, want a full git object id (the default branch's own landed blob)", got2.Baseline.Blob)
	}
	if got2.Baseline.LandingCommit != "" {
		t.Fatalf("Baseline.LandingCommit = %q, want empty — a diverged candidate never has a computed landing commit", got2.Baseline.LandingCommit)
	}
	if !strings.Contains(line, `"commit":""`) {
		t.Fatalf("stdout = %q, want the wire-form baseline to carry an explicit empty commit field, never omit or null it", line)
	}
}

// TestCmdSpecState_Unproven_ExitsCleanWithDisclosure proves an unproven
// ancestry (no default branch resolvable) still exits 0 (a KNOWN state —
// "unproven" is itself a proven verdict about provability, not a failure
// to read the ref or the store) and carries a non-empty disclosure.
func TestCmdSpecState_Unproven_ExitsCleanWithDisclosure(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                    "schema: verdi.layout/v1\nforge: gitlab\n",
			".verdi/specs/active/payments/spec.md": specStateExactPaymentsMD,
		},
		Message: "scaffold, no default branch resolvable",
	}})
	t.Setenv("CI_DEFAULT_BRANCH", "")
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := cmdSpecState([]string{"spec/payments"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdSpecState(unproven ancestry) = %d, want 0; stderr=%s", got, stderr.String())
	}
	line := strings.TrimRight(stdout.String(), "\n")
	var got2 specstate.Result
	if err := json.Unmarshal([]byte(line), &got2); err != nil {
		t.Fatalf("decoding stdout as JSON: %v", err)
	}
	if got2.State != specstate.Unproven {
		t.Fatalf("state = %q, want unproven", got2.State)
	}
	if len(got2.Disclosures) == 0 {
		t.Fatalf("disclosures = %v, want at least one", got2.Disclosures)
	}
}

// TestCmdSpecState_Negative covers argument, read, and ref-resolution
// failures — all exit 2 (operational), never 0 or 1: `spec state` makes no
// lifecycle claim of its own, so it has no verdict to fail.
func TestCmdSpecState_Negative(t *testing.T) {
	repo := buildSpecStateRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": specStateExactPaymentsMD})

	cases := []struct {
		name string
		args []string
	}{
		{"no arguments", nil},
		{"two arguments", []string{"spec/payments", "extra"}},
		{"malformed ref (no kind/name separator)", []string{"payments"}},
		{"wrong kind", []string{"attestation/payments"}},
		{"unknown spec", []string{"spec/does-not-exist"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(repo.Dir)
			var stdout, stderr bytes.Buffer
			got := cmdSpecState(tc.args, &stdout, &stderr)
			if got != 2 {
				t.Fatalf("cmdSpecState(%v) = %d, want 2; stdout=%s stderr=%s", tc.args, got, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty on an operational error", stdout.String())
			}
		})
	}
}

// TestRunSpecVerb_Usage pins spec's own subcommand-shape check.
func TestRunSpecVerb_Usage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := runSpecVerb(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("runSpecVerb(no args) = %d, want 2", got)
	}
	stdout.Reset()
	stderr.Reset()
	if got := runSpecVerb([]string{"bogus"}, &stdout, &stderr); got != 2 {
		t.Fatalf("runSpecVerb(bogus subcommand) = %d, want 2", got)
	}
}

// TestRun_SpecStateDispatchesToRealVerb proves dispatch.go routes
// "spec state" to the real implementation.
func TestRun_SpecStateDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	got := run([]string{"spec", "state", "spec/x"}, &stderr)
	if got != 2 {
		t.Fatalf("run([spec state ...]) outside a store = %d, want 2 (operational)", got)
	}
	if contains(stderr.String(), "usage") || contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}

func currentHead(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func porcelainStatus(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status --porcelain: %v", err)
	}
	return string(out)
}
