package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// waiveFixtureStorySpecMD is a story spec declaring two ACs, mirroring
// attestFixtureStorySpecMD's own shape (story-ref RefSlugs to
// "jira-waive-1", multiple owners so "copied verbatim, plural" is
// actually exercised).
const waiveFixtureStorySpecMD = `---
id: spec/waive-fixture-story
kind: spec
class: story
title: "Waive fixture story"
status: accepted-pending-build
owners: [platform-team, oncall-lead]
story: jira:WAIVE-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/waive-fixture-feature#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the fixture behavior holds", evidence: [behavioral] }
  - { id: ac-2, text: "a second fixture behavior holds", evidence: [static] }
frozen: { at: 2026-07-16, commit: e606a109dbc28ea08cc86265c4fa2dd026f8373a }
---
# Waive fixture story
## Problem
p
## Outcome
o
`

const waiveFixtureFeatureSpecMD = `---
id: spec/waive-fixture-feature
kind: spec
class: feature
title: "Waive fixture feature"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the feature outcome holds", evidence: [behavioral] }
frozen: { at: 2026-07-16, commit: e606a109dbc28ea08cc86265c4fa2dd026f8373a }
---
# Waive fixture feature
## Problem
p
## Outcome
o
`

// buildWaiveFixtureRepo builds a real, local, hermetic fixturegit repo
// (co-1) carrying the story+feature fixtures above, mirroring
// buildAttestFixtureRepo's own convention exactly.
func buildWaiveFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: github\n",
			".verdi/specs/active/waive-fixture-story/spec.md":   waiveFixtureStorySpecMD,
			".verdi/specs/active/waive-fixture-feature/spec.md": waiveFixtureFeatureSpecMD,
		},
		Message: "waive fixture: story + feature",
	}})
}

const waiveFixtureStorySlug = "jira-waive-1" // store.RefSlug("jira:WAIVE-1")

func readWaiverFile(t *testing.T, root, storySlug, acID string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".verdi", "waivers", storySlug, acID+".md"))
	if err != nil {
		t.Fatalf("reading waiver file: %v", err)
	}
	return string(data)
}

var waiveTestNow = time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)

// --- parseWaiveArgs ---

func TestParseWaiveArgs_Happy(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantStory     string
		wantAC        string
		wantRationale string
		wantExpires   string
		wantReaffirm  bool
	}{
		{
			name: "positionals then flags", args: []string{"task/retry-worker", "ac-1", "--rationale", "hotfix", "--expires", "2026-08-01"},
			wantStory: "task/retry-worker", wantAC: "ac-1", wantRationale: "hotfix", wantExpires: "2026-08-01",
		},
		{
			name: "equals-form flags", args: []string{"task/retry-worker", "ac-1", "--rationale=hotfix", "--expires=2026-08-01"},
			wantStory: "task/retry-worker", wantAC: "ac-1", wantRationale: "hotfix", wantExpires: "2026-08-01",
		},
		{
			name: "flags interleaved before positionals", args: []string{"--rationale", "hotfix", "task/retry-worker", "ac-1"},
			wantStory: "task/retry-worker", wantAC: "ac-1", wantRationale: "hotfix",
		},
		{
			name: "reaffirm flag", args: []string{"task/retry-worker", "ac-1", "--reaffirm", "--rationale", "still flaking"},
			wantStory: "task/retry-worker", wantAC: "ac-1", wantRationale: "still flaking", wantReaffirm: true,
		},
		{
			name: "no expires at all", args: []string{"task/retry-worker", "ac-1", "--rationale", "hotfix"},
			wantStory: "task/retry-worker", wantAC: "ac-1", wantRationale: "hotfix",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			story, ac, rationale, expires, reaffirm, err := parseWaiveArgs(tt.args)
			if err != nil {
				t.Fatalf("parseWaiveArgs(%v): %v", tt.args, err)
			}
			if story != tt.wantStory || ac != tt.wantAC || rationale != tt.wantRationale || expires != tt.wantExpires || reaffirm != tt.wantReaffirm {
				t.Errorf("parseWaiveArgs(%v) = (%q,%q,%q,%q,%v), want (%q,%q,%q,%q,%v)",
					tt.args, story, ac, rationale, expires, reaffirm,
					tt.wantStory, tt.wantAC, tt.wantRationale, tt.wantExpires, tt.wantReaffirm)
			}
		})
	}
}

func TestParseWaiveArgs_Negative(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no positionals at all", []string{"--rationale", "x"}},
		{"only one positional", []string{"task/retry-worker", "--rationale", "x"}},
		{"three positionals", []string{"task/retry-worker", "ac-1", "extra", "--rationale", "x"}},
		{"rationale given twice", []string{"task/retry-worker", "ac-1", "--rationale", "a", "--rationale", "b"}},
		{"rationale flag missing a value", []string{"task/retry-worker", "ac-1", "--rationale"}},
		{"expires flag missing a value", []string{"task/retry-worker", "ac-1", "--expires"}},
		{"unknown flag", []string{"task/retry-worker", "ac-1", "--bogus"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, _, _, _, err := parseWaiveArgs(tt.args); err == nil {
				t.Fatalf("parseWaiveArgs(%v): want error, got nil", tt.args)
			}
		})
	}
}

// --- runWaive: create path ---

// TestRunWaive_Create_Happy proves ac-1's core create path: a
// well-formed invocation writes a decodable, active waiver at the
// convention path, copying owners verbatim and stamping frozen at HEAD.
func TestRunWaive_Create_Happy(t *testing.T) {
	repo := buildWaiveFixtureRepo(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runWaive(ctx, repo.Dir, "jira:WAIVE-1", "ac-1", false, "hotfix for PSP outage; tracked in PAY-1519", "2026-08-01", nil, waiveTestNow, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runWaive = %d, want 0; stderr=%s", got, stderr.String())
	}

	content := readWaiverFile(t, repo.Dir, waiveFixtureStorySlug, "ac-1")
	fm, body, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v\ncontent:\n%s", err, content)
	}
	w, err := artifact.DecodeWaiver(fm)
	if err != nil {
		t.Fatalf("DecodeWaiver: %v\ncontent:\n%s", err, content)
	}
	if w.ID != "waiver/jira-waive-1--ac-1" {
		t.Errorf("ID = %q", w.ID)
	}
	if w.Status != "active" {
		t.Errorf("Status = %q, want active", w.Status)
	}
	if w.Reason != "hotfix for PSP outage; tracked in PAY-1519" {
		t.Errorf("Reason = %q", w.Reason)
	}
	if w.Expiry != "2026-08-01" {
		t.Errorf("Expiry = %q, want 2026-08-01", w.Expiry)
	}
	if len(w.Owners) != 2 || w.Owners[0] != "platform-team" || w.Owners[1] != "oncall-lead" {
		t.Errorf("Owners = %v, want the story's own owners verbatim", w.Owners)
	}
	if w.Frozen == nil || w.Frozen.Commit != repo.Head {
		t.Errorf("Frozen = %+v, want commit == repo HEAD (%s)", w.Frozen, repo.Head)
	}
	if !bytes.Contains(body, []byte("waived")) {
		t.Errorf("body missing a waived log entry:\n%s", body)
	}
	if !contains(stdout.String(), "ac-1") || !contains(stdout.String(), "2026-08-01") {
		t.Errorf("stdout = %q, want it to surface the AC and the configured expiry", stdout.String())
	}
}

// TestRunWaive_Create_NoExpiry proves an absent --expires still creates
// cleanly and discloses "no --expires given" rather than a fabricated date.
func TestRunWaive_Create_NoExpiry(t *testing.T) {
	repo := buildWaiveFixtureRepo(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runWaive(ctx, repo.Dir, "jira:WAIVE-1", "ac-2", false, "no proof yet", "", nil, waiveTestNow, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runWaive = %d, want 0; stderr=%s", got, stderr.String())
	}
	if !contains(stdout.String(), "no --expires given") {
		t.Errorf("stdout = %q, want the no-expiry disclosure", stdout.String())
	}
	content := readWaiverFile(t, repo.Dir, waiveFixtureStorySlug, "ac-2")
	if contains(content, "expiry:") {
		t.Errorf("content = %q, want no expiry: field at all", content)
	}
}

// TestRunWaive_Create_RefusesWhenAlreadyExists proves the create path is
// create-only: a second plain invocation over an existing waiver refuses
// (exit 1) naming --reaffirm, and leaves the existing file byte-for-byte
// untouched.
func TestRunWaive_Create_RefusesWhenAlreadyExists(t *testing.T) {
	repo := buildWaiveFixtureRepo(t)
	ctx := context.Background()

	var stdout1, stderr1 bytes.Buffer
	if got := runWaive(ctx, repo.Dir, "jira:WAIVE-1", "ac-1", false, "first", "", nil, waiveTestNow, &stdout1, &stderr1); got != 0 {
		t.Fatalf("first runWaive = %d, want 0; stderr=%s", got, stderr1.String())
	}
	before := readWaiverFile(t, repo.Dir, waiveFixtureStorySlug, "ac-1")

	var stdout2, stderr2 bytes.Buffer
	got := runWaive(ctx, repo.Dir, "jira:WAIVE-1", "ac-1", false, "second", "", nil, waiveTestNow, &stdout2, &stderr2)
	if got != 1 {
		t.Fatalf("second runWaive = %d, want 1 (verdict refusal)", got)
	}
	if !contains(stderr2.String(), "--reaffirm") {
		t.Errorf("stderr = %q, want it to name --reaffirm", stderr2.String())
	}
	after := readWaiverFile(t, repo.Dir, waiveFixtureStorySlug, "ac-1")
	if before != after {
		t.Errorf("existing waiver was modified by a refused create call:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// --- runWaive: --reaffirm path ---

// TestRunWaive_Reaffirm_RefusesWhenNoneExists proves --reaffirm requires a
// prior waiver: refuses (exit 1) naming the plain create form, writes
// nothing.
func TestRunWaive_Reaffirm_RefusesWhenNoneExists(t *testing.T) {
	repo := buildWaiveFixtureRepo(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runWaive(ctx, repo.Dir, "jira:WAIVE-1", "ac-1", true, "reaffirming", "", nil, waiveTestNow, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runWaive(--reaffirm, none exists) = %d, want 1", got)
	}
	if !contains(stderr.String(), "verdi waive") {
		t.Errorf("stderr = %q, want it to name the plain create form", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "waivers", waiveFixtureStorySlug, "ac-1.md")); err == nil {
		t.Fatal("a refused --reaffirm must not create a file")
	}
}

// TestRunWaive_Reaffirm_RoundTrips is ac-2's core proof: a reaffirm
// rewrites the same file with a fresh reason/expiry/frozen stamp, and the
// body's log carries BOTH the original "waived" entry (verbatim) and
// exactly one new "reaffirmed" entry after it.
func TestRunWaive_Reaffirm_RoundTrips(t *testing.T) {
	repo := buildWaiveFixtureRepo(t)
	ctx := context.Background()

	var stdout1, stderr1 bytes.Buffer
	if got := runWaive(ctx, repo.Dir, "jira:WAIVE-1", "ac-1", false, "hotfix one", "2026-08-01", nil, waiveTestNow, &stdout1, &stderr1); got != 0 {
		t.Fatalf("create runWaive = %d, want 0; stderr=%s", got, stderr1.String())
	}
	firstContent := readWaiverFile(t, repo.Dir, waiveFixtureStorySlug, "ac-1")

	laterNow := time.Date(2026, 7, 25, 9, 0, 0, 0, time.UTC)
	var stdout2, stderr2 bytes.Buffer
	got := runWaive(ctx, repo.Dir, "jira:WAIVE-1", "ac-1", true, "still flaking; PAY-1519 not fixed", "2026-08-15", nil, laterNow, &stdout2, &stderr2)
	if got != 0 {
		t.Fatalf("reaffirm runWaive = %d, want 0; stderr=%s", got, stderr2.String())
	}

	content := readWaiverFile(t, repo.Dir, waiveFixtureStorySlug, "ac-1")
	fm, body, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	w, err := artifact.DecodeWaiver(fm)
	if err != nil {
		t.Fatalf("DecodeWaiver: %v", err)
	}
	if w.Reason != "still flaking; PAY-1519 not fixed" {
		t.Errorf("Reason = %q, want the fresh rationale", w.Reason)
	}
	if w.Expiry != "2026-08-15" {
		t.Errorf("Expiry = %q, want the fresh expiry", w.Expiry)
	}
	if w.Status != "active" {
		t.Errorf("Status = %q, want active", w.Status)
	}
	if !bytes.Contains(body, []byte("hotfix one")) {
		t.Errorf("body lost the prior rationale/log entry:\n%s", body)
	}
	if !bytes.Contains(body, []byte("still flaking; PAY-1519 not fixed")) {
		t.Errorf("body missing the new rationale/log entry:\n%s", body)
	}
	if bytes.Count(body, []byte("<!-- verdi:waiver-reaffirmation-log -->")) != 1 {
		t.Errorf("body must carry exactly one log marker after reaffirm:\n%s", body)
	}
	if content == firstContent {
		t.Error("reaffirm produced byte-identical content to the original create — nothing actually changed")
	}
	if !contains(stdout2.String(), "reaffirmed") {
		t.Errorf("stdout = %q, want it to say reaffirmed", stdout2.String())
	}
}

// TestRunWaive_Reaffirm_DisclosesLapsedPriorExpiry proves reaffirming a
// waiver whose recorded expiry has already passed (by the reaffirm
// invocation's own `now`) discloses that lapse plainly on stdout.
func TestRunWaive_Reaffirm_DisclosesLapsedPriorExpiry(t *testing.T) {
	repo := buildWaiveFixtureRepo(t)
	ctx := context.Background()

	var stdout1, stderr1 bytes.Buffer
	if got := runWaive(ctx, repo.Dir, "jira:WAIVE-1", "ac-1", false, "hotfix", "2026-08-01", nil, waiveTestNow, &stdout1, &stderr1); got != 0 {
		t.Fatalf("create runWaive = %d, want 0; stderr=%s", got, stderr1.String())
	}

	lapsedNow := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC) // well after 2026-08-01
	var stdout2, stderr2 bytes.Buffer
	got := runWaive(ctx, repo.Dir, "jira:WAIVE-1", "ac-1", true, "still needed", "2026-10-01", nil, lapsedNow, &stdout2, &stderr2)
	if got != 0 {
		t.Fatalf("reaffirm runWaive = %d, want 0; stderr=%s", got, stderr2.String())
	}
	if !contains(stdout2.String(), "2026-08-01") || !contains(stdout2.String(), "lapsed") {
		t.Errorf("stdout = %q, want it to disclose the lapsed prior expiry (2026-08-01)", stdout2.String())
	}
}

// --- runWaive: shared refusal/validation paths ---

// TestRunWaive_RefusesUnknownStoryRef mirrors TestRunAttest_
// RefusesUnknownStoryRef: an unresolvable story ref is a verdict refusal
// (exit 1), never operational — classifyPair's own shared behavior,
// reused rather than duplicated.
func TestRunWaive_RefusesUnknownStoryRef(t *testing.T) {
	repo := buildWaiveFixtureRepo(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runWaive(ctx, repo.Dir, "jira:NO-SUCH-STORY", "ac-1", false, "x", "", nil, waiveTestNow, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runWaive(unknown story) = %d, want 1", got)
	}
}

// TestRunWaive_RefusesUndeclaredAC mirrors TestRunAttest_
// RefusesUndeclaredAC.
func TestRunWaive_RefusesUndeclaredAC(t *testing.T) {
	repo := buildWaiveFixtureRepo(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runWaive(ctx, repo.Dir, "jira:WAIVE-1", "ac-99", false, "x", "", nil, waiveTestNow, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runWaive(undeclared AC) = %d, want 1", got)
	}
}

// --- cmdWaive: usage-level validation (exit 2) ---

func TestCmdWaive_UsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no args", nil},
		{"missing rationale", []string{"task/retry-worker", "ac-1"}},
		{"malformed expires", []string{"task/retry-worker", "ac-1", "--rationale", "x", "--expires", "not-a-date"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := cmdWaive(tt.args, &stdout, &stderr)
			if got != 2 {
				t.Fatalf("cmdWaive(%v) = %d, want 2; stderr=%s", tt.args, got, stderr.String())
			}
		})
	}
}

// TestRun_WaiveDispatchesToRealVerb mirrors TestRun_ObligationDispatchesToRealVerb's
// exact pattern.
func TestRun_WaiveDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	got := run([]string{"waive", "spec/x", "ac-1", "--rationale", "x"}, &stderr)
	if got != 2 {
		t.Fatalf("run([waive ...]) outside a store = %d, want 2 (operational)", got)
	}
	if contains(stderr.String(), "usage") || contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}

// TestWaiveE2E_FullLifecycle is spec/verb-surfaces' own built-binary
// lifecycle proof (ac-1/ac-2/ac-3, mirroring obligationseam_e2e_test.go's
// convention: the REAL compiled verdi binary as a real OS process against
// a real, local fixturegit repository, never a package-internal call
// standing in for it): waive creates the record and it is present on
// disk; verdi matrix's fold reads it as waived; --expires surfaces on
// waive's own stdout; --reaffirm round-trips (a fresh commit-worthy
// rewrite, the prior log entry preserved, one new entry appended); and
// verdi audit counts the waiver in its own dedicated section.
func TestWaiveE2E_FullLifecycle(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildWaiveFixtureRepo(t)

	// Step 1: create, with --expires.
	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil,
		"waive", "jira:WAIVE-1", "ac-1",
		"--rationale", "hotfix for PSP outage; tracked in PAY-1519",
		"--expires", "2026-08-01")
	if code != 0 {
		t.Fatalf("verdi waive (create) exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	// --expires surfaces expiry on waive's own stdout.
	if !contains(stdout, "2026-08-01") {
		t.Fatalf("verdi waive stdout = %q, want it to surface the configured expiry 2026-08-01", stdout)
	}

	// Record present on disk, decodable.
	waiverPath := filepath.Join(repo.Dir, ".verdi", "waivers", waiveFixtureStorySlug, "ac-1.md")
	data, err := os.ReadFile(waiverPath)
	if err != nil {
		t.Fatalf("reading waiver record at %s: %v", waiverPath, err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v\n%s", err, data)
	}
	if _, err := artifact.DecodeWaiver(fm); err != nil {
		t.Fatalf("waiver record does not decode: %v\n%s", err, data)
	}

	// Step 2: verdi matrix's fold reads the AC as waived.
	matrixOut, matrixErr, matrixCode := runVerdiBinary(t, bin, repo.Dir, nil, "matrix", "spec/waive-fixture-story")
	if matrixCode != 0 {
		t.Fatalf("verdi matrix exit = %d, want 0\nstdout: %s\nstderr: %s", matrixCode, matrixOut, matrixErr)
	}
	if !contains(matrixOut, "waived") {
		t.Fatalf("verdi matrix stdout = %q, want the ac-1 row to read waived", matrixOut)
	}

	// Step 3: --reaffirm round-trips.
	reaffirmOut, reaffirmErr, reaffirmCode := runVerdiBinary(t, bin, repo.Dir, nil,
		"waive", "jira:WAIVE-1", "ac-1", "--reaffirm",
		"--rationale", "still flaking on the CI runner; PAY-1519 not yet fixed",
		"--expires", "2026-08-15")
	if reaffirmCode != 0 {
		t.Fatalf("verdi waive --reaffirm exit = %d, want 0\nstdout: %s\nstderr: %s", reaffirmCode, reaffirmOut, reaffirmErr)
	}
	if !contains(reaffirmOut, "reaffirmed") {
		t.Fatalf("verdi waive --reaffirm stdout = %q, want it to say reaffirmed", reaffirmOut)
	}
	reaffirmedData, err := os.ReadFile(waiverPath)
	if err != nil {
		t.Fatalf("reading reaffirmed waiver record: %v", err)
	}
	if !bytes.Contains(reaffirmedData, []byte("hotfix for PSP outage")) {
		t.Fatalf("reaffirmed record lost the original log entry:\n%s", reaffirmedData)
	}
	if !bytes.Contains(reaffirmedData, []byte("still flaking on the CI runner")) {
		t.Fatalf("reaffirmed record missing the new log entry:\n%s", reaffirmedData)
	}

	// Step 4: verdi audit counts the (still active) waiver in its own
	// section, distinct from the deviations/exemptions sections.
	auditOut, auditErr, auditCode := runVerdiBinary(t, bin, repo.Dir, nil, "audit")
	if auditCode != 0 {
		// One active waiver is well under the default threshold (3) — a
		// clean run is the correct, deterministic outcome here.
		t.Fatalf("verdi audit exit = %d, want 0 (1 active waiver is under the default threshold)\nstdout: %s\nstderr: %s", auditCode, auditOut, auditErr)
	}
	if !contains(auditOut, "Waiver audit") {
		t.Fatalf("verdi audit stdout = %q, want a \"Waiver audit\" section", auditOut)
	}
	if !contains(auditOut, "spec/waive-fixture-story") || !contains(auditOut, "active waivers: 1") {
		t.Fatalf("verdi audit stdout = %q, want it to count exactly 1 active waiver for spec/waive-fixture-story", auditOut)
	}
}
