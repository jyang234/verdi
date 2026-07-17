package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// attestFixtureStorySpecMD is a story spec declaring two ACs — the exact
// shape spec/attest-helper's own tests target: a story-ref that RefSlugs
// to "jira-attest-1", a declared ac-1/ac-2, and multiple owners (to prove
// owners are copied verbatim, plural, never a single hardcoded value).
const attestFixtureStorySpecMD = `---
id: spec/attest-fixture-story
kind: spec
class: story
title: "Attest fixture story"
status: accepted-pending-build
owners: [platform-team, qa-lead]
story: jira:ATTEST-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/attest-fixture-feature#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the fixture behavior holds", evidence: [attestation] }
  - { id: ac-2, text: "a second fixture behavior holds", evidence: [static, attestation] }
frozen: { at: 2026-07-16, commit: e606a109dbc28ea08cc86265c4fa2dd026f8373a }
---
# Attest fixture story
## Problem
p
## Outcome
o
`

// attestFixtureFeatureSpecMD is a class: feature spec — used both as the
// story's own implements target and directly as a wrong-class refusal
// target (dc-5's scope boundary: attest targets STORY attestations only).
const attestFixtureFeatureSpecMD = `---
id: spec/attest-fixture-feature
kind: spec
class: feature
title: "Attest fixture feature"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the feature outcome holds", evidence: [attestation] }
frozen: { at: 2026-07-16, commit: e606a109dbc28ea08cc86265c4fa2dd026f8373a }
---
# Attest fixture feature
## Problem
p
## Outcome
o
`

// buildAttestFixtureRepo builds a fixturegit repo carrying the story and
// feature fixtures above — a real, local, hermetic git repository (co-1),
// mirroring cmd/verdi/design_test.go's and cmd/verdi/close_test.go's own
// harness exactly: no subprocess exec of the built binary, no network.
func buildAttestFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: github\n",
			".verdi/specs/active/attest-fixture-story/spec.md":   attestFixtureStorySpecMD,
			".verdi/specs/active/attest-fixture-feature/spec.md": attestFixtureFeatureSpecMD,
		},
		Message: "attest fixture: story + feature",
	}})
}

// readAttestationFile reads back the attestation file at the exact fold
// path (evidence's own attestations/<storySlug>/<acID>.md convention),
// failing the test if it is missing.
func readAttestationFile(t *testing.T, root, storySlug, acID string) string {
	t.Helper()
	path := filepath.Join(root, ".verdi", "attestations", storySlug, acID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading attestation at the fold path %s: %v", path, err)
	}
	return string(data)
}

// snapshotTree captures every regular file's path -> content under root,
// for a byte-for-byte "working tree unchanged" comparison after a refused
// invocation (co-2).
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshotting tree at %s: %v", root, err)
	}
	return out
}

// assertTreeUnchanged fails the test if root's working tree differs from
// before — co-2's "a refused invocation leaves the working tree
// byte-for-byte as it found it".
func assertTreeUnchanged(t *testing.T, root string, before map[string]string) {
	t.Helper()
	after := snapshotTree(t, root)
	if len(before) != len(after) {
		t.Fatalf("working tree file count changed: before=%d after=%d (want byte-for-byte unchanged, co-2)", len(before), len(after))
	}
	for path, want := range before {
		got, ok := after[path]
		if !ok {
			t.Fatalf("file %s disappeared from the working tree (want unchanged, co-2)", path)
		}
		if got != want {
			t.Fatalf("file %s changed content (want byte-for-byte unchanged, co-2)", path)
		}
	}
}

// TestRunAttest_Happy is AC-1's own behavioral register: driving the
// verb's testable core against a fixturegit-backed store, asserting the
// file lands at the exact slugged path the fold reads (derived through the
// real store.RefSlug, not a hand-typed literal), with the AC-1 frontmatter
// fields and the unauthored marker leading the body — and never any
// claim-shaped body prose.
func TestRunAttest_Happy(t *testing.T) {
	repo := buildAttestFixtureRepo(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runAttest(ctx, repo.Dir, "jira:ATTEST-1", "ac-1", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAttest = %d, want 0; stderr=%s", got, stderr.String())
	}

	wantSlug := "jira-attest-1" // store.RefSlug("jira:ATTEST-1")
	content := readAttestationFile(t, repo.Dir, wantSlug, "ac-1")

	fm, body, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v\ncontent:\n%s", err, content)
	}
	decoded, err := artifact.DecodeAttestation(fm)
	if err != nil {
		t.Fatalf("DecodeAttestation: %v\ncontent:\n%s", err, content)
	}

	if decoded.ID != "attestation/jira-attest-1--ac-1" {
		t.Errorf("id = %q, want attestation/jira-attest-1--ac-1", decoded.ID)
	}
	if decoded.Kind != artifact.KindAttestation {
		t.Errorf("kind = %q, want attestation", decoded.Kind)
	}
	if decoded.Schema != "verdi.attestation/v1" {
		t.Errorf("schema = %q, want verdi.attestation/v1", decoded.Schema)
	}
	if len(decoded.Owners) != 2 || decoded.Owners[0] != "platform-team" || decoded.Owners[1] != "qa-lead" {
		t.Errorf("owners = %v, want the story spec's own owners verbatim [platform-team qa-lead]", decoded.Owners)
	}
	if len(decoded.Links) != 1 || decoded.Links[0].Type != artifact.LinkVerifies || decoded.Links[0].Ref != "spec/attest-fixture-story" {
		t.Errorf("links = %+v, want a single verifies edge to spec/attest-fixture-story", decoded.Links)
	}
	if decoded.Frozen == nil || decoded.Frozen.Commit != repo.Head {
		t.Errorf("frozen = %+v, want commit == repo HEAD (%s)", decoded.Frozen, repo.Head)
	}
	if !bytes.HasPrefix(body, []byte("<!-- verdi:attestation-unauthored -->")) {
		t.Errorf("body does not start with the unauthored marker:\n%s", body)
	}
	if bytes.Contains(body, []byte("I verified")) || bytes.Contains(body, []byte("observed in staging")) {
		t.Errorf("body contains claim-shaped prose — dc-2 forbids this:\n%s", body)
	}

	if !contains(stdout.String(), wantSlug) {
		t.Errorf("stdout = %q, want it to name the scaffolded path", stdout.String())
	}
}

// TestRunAttest_RefusesUnknownStoryRef proves AC-2's first refusal shape:
// a <story-ref> that does not resolve at all is refused with the verdict
// discipline (exit 1, dc-5 — grouped under the SAME verdict as every other
// "pair does not exist" case, a disclosed divergence from matrix's own
// exit-2 posture for the identical resolution failure) — never exit 0,
// never exit 2, and the working tree is left byte-for-byte unchanged.
func TestRunAttest_RefusesUnknownStoryRef(t *testing.T) {
	repo := buildAttestFixtureRepo(t)
	ctx := context.Background()
	before := snapshotTree(t, repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runAttest(ctx, repo.Dir, "jira:NO-SUCH-STORY", "ac-1", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runAttest(unknown story-ref) = %d, want 1 (verdict)", got)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected an explanatory stderr message")
	}
	assertTreeUnchanged(t, repo.Dir, before)
}

// TestRunAttest_RefusesWrongClass proves AC-2/dc-5's scope boundary: a
// story-ref that resolves to a non-story (here class: feature) spec is
// refused under the SAME "pair does not exist" verdict (exit 1) — "no
// STORY exists to attest an AC against" — never exit 0, never exit 2.
func TestRunAttest_RefusesWrongClass(t *testing.T) {
	repo := buildAttestFixtureRepo(t)
	ctx := context.Background()
	before := snapshotTree(t, repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runAttest(ctx, repo.Dir, "spec/attest-fixture-feature", "ac-1", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runAttest(feature-class target) = %d, want 1 (verdict)", got)
	}
	if !contains(stderr.String(), "feature") {
		t.Errorf("stderr = %q, want it to name the offending class", stderr.String())
	}
	assertTreeUnchanged(t, repo.Dir, before)
}

// TestRunAttest_RefusesUndeclaredAC proves AC-2's third refusal shape: a
// resolved story spec that does not declare the given ac-id is refused
// (exit 1, verdict), never exit 0, never exit 2.
func TestRunAttest_RefusesUndeclaredAC(t *testing.T) {
	repo := buildAttestFixtureRepo(t)
	ctx := context.Background()
	before := snapshotTree(t, repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runAttest(ctx, repo.Dir, "jira:ATTEST-1", "ac-99", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runAttest(undeclared ac) = %d, want 1 (verdict)", got)
	}
	if !contains(stderr.String(), "ac-99") {
		t.Errorf("stderr = %q, want it to name the undeclared ac-id", stderr.String())
	}
	assertTreeUnchanged(t, repo.Dir, before)
}

// TestRunAttest_RefusesAlreadyExists proves AC-2's second refusal shape —
// an attestation already sitting at the exact fold path is never
// overwritten (dc-2's "never overwrite a human record" made mechanical):
// exit 1, verdict, and the pre-existing file's bytes are provably
// untouched.
func TestRunAttest_RefusesAlreadyExists(t *testing.T) {
	repo := buildAttestFixtureRepo(t)
	ctx := context.Background()

	const existingBody = "---\nid: attestation/jira-attest-1--ac-1\nkind: attestation\ntitle: \"already here\"\nowners: [platform-team]\nfrozen: { at: 2026-01-01, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }\n---\n# already attested\nA human wrote this.\n"
	dir := filepath.Join(repo.Dir, ".verdi", "attestations", "jira-attest-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ac-1.md"), []byte(existingBody), 0o644); err != nil {
		t.Fatalf("seeding existing attestation: %v", err)
	}
	before := snapshotTree(t, repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runAttest(ctx, repo.Dir, "jira:ATTEST-1", "ac-1", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runAttest(already exists) = %d, want 1 (verdict)", got)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected an explanatory stderr message")
	}
	assertTreeUnchanged(t, repo.Dir, before)

	// Redundant, maximally-direct proof: the exact bytes are untouched.
	got2, err := os.ReadFile(filepath.Join(dir, "ac-1.md"))
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if string(got2) != existingBody {
		t.Fatal("the pre-existing attestation's bytes changed — dc-2 forbids overwriting a human record")
	}
}

// TestRunAttest_ScaffoldRoundTrips is AC-4's own behavioral register:
// writes a scaffold, reads it back from disk at the fold's own
// path-construction convention, and asserts artifact.DecodeAttestation
// succeeds against the read-back bytes — while the unauthored marker is
// still present, before any claim is ever authored.
func TestRunAttest_ScaffoldRoundTrips(t *testing.T) {
	repo := buildAttestFixtureRepo(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runAttest(ctx, repo.Dir, "jira:ATTEST-1", "ac-2", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAttest = %d, want 0; stderr=%s", got, stderr.String())
	}

	content := readAttestationFile(t, repo.Dir, "jira-attest-1", "ac-2")
	if !contains(content, "<!-- verdi:attestation-unauthored -->") {
		t.Fatalf("scaffold lost its unauthored marker before round-trip:\n%s", content)
	}
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v\ncontent:\n%s", err, content)
	}
	if _, err := artifact.DecodeAttestation(fm); err != nil {
		t.Fatalf("DecodeAttestation on the read-back scaffold: %v\ncontent:\n%s", err, content)
	}
}

// TestClassifyPair is AC-2's static register: table-driven unit tests over
// the pair-existence predicate, exercised DIRECTLY (not only end to end),
// covering its three failure shapes plus the clean case — each row proving
// the 0/1/2 split at the predicate level (dc-5: every failure here is the
// SAME verdict outcome, never operational).
func TestClassifyPair(t *testing.T) {
	repo := buildAttestFixtureRepo(t)

	t.Run("clean pair resolves", func(t *testing.T) {
		spec, refusal := classifyPair(repo.Dir, "jira:ATTEST-1", "ac-1")
		if refusal != "" {
			t.Fatalf("refusal = %q, want none", refusal)
		}
		if spec == nil || spec.ID != "spec/attest-fixture-story" {
			t.Fatalf("spec = %+v, want spec/attest-fixture-story", spec)
		}
	})

	t.Run("story-ref does not resolve", func(t *testing.T) {
		spec, refusal := classifyPair(repo.Dir, "jira:NO-SUCH-STORY", "ac-1")
		if refusal == "" {
			t.Fatal("want a non-empty refusal reason")
		}
		if spec != nil {
			t.Fatalf("spec = %+v, want nil on refusal", spec)
		}
	})

	t.Run("resolved spec is not class story", func(t *testing.T) {
		spec, refusal := classifyPair(repo.Dir, "spec/attest-fixture-feature", "ac-1")
		if refusal == "" {
			t.Fatal("want a non-empty refusal reason")
		}
		if spec != nil {
			t.Fatalf("spec = %+v, want nil on refusal", spec)
		}
		if !contains(refusal, "feature") {
			t.Errorf("refusal = %q, want it to name the offending class", refusal)
		}
	})

	t.Run("ac-id not declared", func(t *testing.T) {
		spec, refusal := classifyPair(repo.Dir, "jira:ATTEST-1", "ac-99")
		if refusal == "" {
			t.Fatal("want a non-empty refusal reason")
		}
		if spec != nil {
			t.Fatalf("spec = %+v, want nil on refusal", spec)
		}
		if !contains(refusal, "ac-99") {
			t.Errorf("refusal = %q, want it to name the undeclared ac-id", refusal)
		}
	})
}

// TestAttestationAlreadyExists is AC-2's other static predicate: the
// already-exists check at the exact fold path, exercised directly.
func TestAttestationAlreadyExists(t *testing.T) {
	root := t.TempDir()

	exists, err := attestationAlreadyExists(root, "jira-attest-1", "ac-1")
	if err != nil {
		t.Fatalf("attestationAlreadyExists: %v", err)
	}
	if exists {
		t.Fatal("attestationAlreadyExists(nothing written) = true, want false")
	}

	dir := filepath.Join(root, ".verdi", "attestations", "jira-attest-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ac-1.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	exists, err = attestationAlreadyExists(root, "jira-attest-1", "ac-1")
	if err != nil {
		t.Fatalf("attestationAlreadyExists: %v", err)
	}
	if !exists {
		t.Fatal("attestationAlreadyExists(file present) = false, want true")
	}
}

// TestCmdAttest_UsageErrors proves the CLI wrapper's own argument-shape
// validation: exactly two positional arguments are required, before any
// store root is even resolved.
func TestCmdAttest_UsageErrors(t *testing.T) {
	cases := [][]string{
		nil,
		{"jira:ATTEST-1"},
		{"jira:ATTEST-1", "ac-1", "extra"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		got := cmdAttest(args, &stdout, &stderr)
		if got != 2 {
			t.Errorf("cmdAttest(%v) = %d, want 2 (usage)", args, got)
		}
		if !contains(stderr.String(), "usage") {
			t.Errorf("cmdAttest(%v) stderr = %q, want a usage message", args, stderr.String())
		}
	}
}

// TestRun_AttestDispatchesToRealVerb proves dispatch.go routes "attest" to
// the real implementation, never "not implemented".
func TestRun_AttestDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	got := run([]string{"attest", "jira:X-1", "ac-1"}, &stderr)
	if got != 2 {
		t.Fatalf("run([attest ...]) outside a store = %d, want 2 (operational)", got)
	}
	if contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}
