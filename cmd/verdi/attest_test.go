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

// attestMalformedSpecMD reads fine but fails STRICT decode (an unknown
// top-level field) — the "present but malformed" resolution failure dc-5/co-2
// classify as OPERATIONAL (exit 2), distinct from a (story, AC) that simply
// does not exist (a verdict, exit 1). ADJ-51 finding 1's own witness.
const attestMalformedSpecMD = `---
id: spec/attest-malformed
kind: spec
class: story
title: "Malformed story spec"
status: accepted-pending-build
owners: [platform-team]
story: jira:MALFORMED-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "t", evidence: [attestation] }
this_top_level_field_is_unknown: boom
---
# Malformed story spec
`

// attestComponentSpecMD is a class: component spec (no story, no ACs) — the
// storyresolve.Resolve rejection whose verbatim "matrix folds only feature
// and story specs" wording attest must NOT leak (ADJ-51 finding 3).
const attestComponentSpecMD = `---
id: spec/attest-component
kind: spec
class: component
title: "Attest component"
status: active
owners: [platform-team]
---
# Attest component
`

// buildAttestMalformedRepo carries the normal story fixture alongside a spec
// that fails strict decode — so a spec-ref straight at it (the target-decode
// path) and a bare story-ref that matches no feature (the fallback-scan path)
// both hit an operational failure while resolving.
func buildAttestMalformedRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: github\n",
			".verdi/specs/active/attest-fixture-story/spec.md": attestFixtureStorySpecMD,
			".verdi/specs/active/attest-malformed/spec.md":     attestMalformedSpecMD,
		},
		Message: "attest fixture: story + malformed spec",
	}})
}

// buildAttestComponentRepo carries a class: component spec for the
// component-refusal wording test.
func buildAttestComponentRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                            "schema: verdi.layout/v1\nforge: github\n",
			".verdi/specs/active/attest-component/spec.md": attestComponentSpecMD,
		},
		Message: "attest fixture: component spec",
	}})
}

// buildAttestStrayDirRepo carries the normal story fixture alongside a
// directory under specs/active/ that has no spec.md — store corruption a
// bare story-ref's resolution scan walks into.
func buildAttestStrayDirRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: github\n",
			".verdi/specs/active/attest-fixture-story/spec.md": attestFixtureStorySpecMD,
			".verdi/specs/active/stray-no-specmd/README.md":    "a directory under specs/active/ with no spec.md — store corruption\n",
		},
		Message: "attest fixture: story + a stray active dir missing spec.md",
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

// TestRunAttest_OperationalOnMalformedTargetSpec is ADJ-51 finding 1's
// primary witness: a story-ref that resolves (spec-ref form) to a spec.md
// that EXISTS but fails strict decode is an OPERATIONAL failure (exit 2), not
// a "(story, AC) pair does not exist" verdict (exit 1). The pair may well
// exist — the machinery to read it failed. co-2's exit discipline is
// constitutional.
func TestRunAttest_OperationalOnMalformedTargetSpec(t *testing.T) {
	repo := buildAttestMalformedRepo(t)
	ctx := context.Background()
	before := snapshotTree(t, repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runAttest(ctx, repo.Dir, "spec/attest-malformed", "ac-1", &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runAttest(malformed target spec) = %d, want 2 (operational, not a verdict)", got)
	}
	assertTreeUnchanged(t, repo.Dir, before)
}

// TestRunAttest_OperationalOnFallbackScanMalformedSpec is finding 1's "worst"
// case: a bare story-ref matching no feature triggers the class: story
// fallback scan, which hits a malformed UNRELATED active spec. That is an
// operational failure (exit 2), never dressed as a "(story, AC) does not
// exist" verdict for the unrelated file's malformation.
func TestRunAttest_OperationalOnFallbackScanMalformedSpec(t *testing.T) {
	repo := buildAttestMalformedRepo(t)
	ctx := context.Background()
	before := snapshotTree(t, repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runAttest(ctx, repo.Dir, "jira:NO-MATCH-ANYWHERE", "ac-1", &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runAttest(fallback scan hits malformed spec) = %d, want 2 (operational)", got)
	}
	assertTreeUnchanged(t, repo.Dir, before)
}

// TestRunAttest_OperationalOnScanStrayDir completes ADJ-51 finding 1 (the
// af00605 re-sweep's scan refinement): a directory under specs/active/
// lacking spec.md is store corruption walked into mid-scan while resolving a
// bare story-ref — operational (exit 2), never dressed as a "(story, AC) does
// not exist" verdict (exit 1) that would also mask a reachable pair.
func TestRunAttest_OperationalOnScanStrayDir(t *testing.T) {
	repo := buildAttestStrayDirRepo(t)
	ctx := context.Background()
	before := snapshotTree(t, repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runAttest(ctx, repo.Dir, "jira:NO-MATCH-ANYWHERE", "ac-1", &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runAttest(scan hits stray active dir) = %d, want 2 (operational store corruption)", got)
	}
	assertTreeUnchanged(t, repo.Dir, before)
}

// TestRunAttest_RefusesComponentInOwnTerms is ADJ-51 finding 3: a class:
// component spec-ref is refused as a verdict (exit 1) whose message speaks in
// attest's OWN terms — never leaking storyresolve/matrix's "matrix folds only
// feature and story specs" contract wording, which names the wrong verb.
func TestRunAttest_RefusesComponentInOwnTerms(t *testing.T) {
	repo := buildAttestComponentRepo(t)
	ctx := context.Background()
	before := snapshotTree(t, repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runAttest(ctx, repo.Dir, "spec/attest-component", "ac-1", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runAttest(component target) = %d, want 1 (verdict)", got)
	}
	if contains(stderr.String(), "matrix folds only") {
		t.Errorf("refusal leaks matrix's own contract wording (names the wrong verb): %s", stderr.String())
	}
	if !contains(stderr.String(), "STORY") {
		t.Errorf("refusal does not speak in attest's own terms (no STORY to attest against): %s", stderr.String())
	}
	assertTreeUnchanged(t, repo.Dir, before)
}

// TestRunAttest_OperationalOnDirectoryAtFoldPath is ADJ-51 finding 5: a
// DIRECTORY sitting at the exact fold path is a store-corruption operational
// error (exit 2) — the same way evidence.AttestationExists/LoadAttestationState
// read it — not an "an attestation already exists" verdict (exit 1) claiming
// to protect a human record that isn't there.
func TestRunAttest_OperationalOnDirectoryAtFoldPath(t *testing.T) {
	repo := buildAttestFixtureRepo(t)
	ctx := context.Background()

	// A directory where the attestation file would go.
	badPath := filepath.Join(repo.Dir, ".verdi", "attestations", "jira-attest-1", "ac-1.md")
	if err := os.MkdirAll(badPath, 0o755); err != nil {
		t.Fatalf("seeding a directory at the fold path: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := runAttest(ctx, repo.Dir, "jira:ATTEST-1", "ac-1", &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runAttest(directory at fold path) = %d, want 2 (operational store corruption)", got)
	}
	if contains(stderr.String(), "already exists") {
		t.Errorf("stderr claims a human record 'already exists' for a directory: %s", stderr.String())
	}
}

// TestClassifyPair is AC-2's static register: table-driven unit tests over
// the pair-existence predicate, exercised DIRECTLY (not only end to end).
// dc-5's three "(story, AC) pair does not exist" shapes are verdicts (a
// non-empty refusal, opErr nil); a spec present-but-unreadable is operational
// (opErr non-nil, refusal empty) — ADJ-51 finding 1's exit-discipline split.
func TestClassifyPair(t *testing.T) {
	repo := buildAttestFixtureRepo(t)

	t.Run("clean pair resolves", func(t *testing.T) {
		spec, refusal, opErr := classifyPair(repo.Dir, "jira:ATTEST-1", "ac-1")
		if refusal != "" || opErr != nil {
			t.Fatalf("refusal = %q, opErr = %v, want neither", refusal, opErr)
		}
		if spec == nil || spec.ID != "spec/attest-fixture-story" {
			t.Fatalf("spec = %+v, want spec/attest-fixture-story", spec)
		}
	})

	t.Run("story-ref does not resolve (verdict)", func(t *testing.T) {
		spec, refusal, opErr := classifyPair(repo.Dir, "jira:NO-SUCH-STORY", "ac-1")
		if refusal == "" {
			t.Fatal("want a non-empty refusal reason (verdict)")
		}
		if opErr != nil {
			t.Fatalf("opErr = %v, want nil (a missing pair is a verdict, not operational)", opErr)
		}
		if spec != nil {
			t.Fatalf("spec = %+v, want nil on refusal", spec)
		}
	})

	t.Run("resolved spec is not class story (verdict)", func(t *testing.T) {
		spec, refusal, opErr := classifyPair(repo.Dir, "spec/attest-fixture-feature", "ac-1")
		if refusal == "" {
			t.Fatal("want a non-empty refusal reason")
		}
		if opErr != nil {
			t.Fatalf("opErr = %v, want nil", opErr)
		}
		if spec != nil {
			t.Fatalf("spec = %+v, want nil on refusal", spec)
		}
		if !contains(refusal, "feature") {
			t.Errorf("refusal = %q, want it to name the offending class", refusal)
		}
	})

	t.Run("ac-id not declared (verdict)", func(t *testing.T) {
		spec, refusal, opErr := classifyPair(repo.Dir, "jira:ATTEST-1", "ac-99")
		if refusal == "" {
			t.Fatal("want a non-empty refusal reason")
		}
		if opErr != nil {
			t.Fatalf("opErr = %v, want nil", opErr)
		}
		if spec != nil {
			t.Fatalf("spec = %+v, want nil on refusal", spec)
		}
		if !contains(refusal, "ac-99") {
			t.Errorf("refusal = %q, want it to name the undeclared ac-id", refusal)
		}
	})

	t.Run("target spec present but malformed (operational)", func(t *testing.T) {
		mrepo := buildAttestMalformedRepo(t)
		spec, refusal, opErr := classifyPair(mrepo.Dir, "spec/attest-malformed", "ac-1")
		if opErr == nil {
			t.Fatal("want a non-nil operational error for a present-but-undecodable target")
		}
		if refusal != "" {
			t.Fatalf("refusal = %q, want empty (operational, not a verdict)", refusal)
		}
		if spec != nil {
			t.Fatalf("spec = %+v, want nil on operational failure", spec)
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

	// ADJ-51 finding 5: a DIRECTORY at the fold path is store corruption,
	// classified operationally (an error) exactly as evidence's own readers
	// do — never reported as an existing attestation (which would exit 1).
	root2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root2, ".verdi", "attestations", "jira-attest-1", "ac-1.md"), 0o755); err != nil {
		t.Fatalf("mkdir dir-at-path: %v", err)
	}
	exists, err = attestationAlreadyExists(root2, "jira-attest-1", "ac-1")
	if err == nil {
		t.Fatal("attestationAlreadyExists(directory at path) = nil error, want an operational error")
	}
	if exists {
		t.Fatal("attestationAlreadyExists(directory at path) = true, want false (not an existing attestation)")
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
