// fixture_test.go is internal/readinessload's own hermetic fixturegit
// recipe: cmd/verdi's readiness_snapshot_test.go/
// readiness_snapshot_integration_test.go used cmd/verdi's own package-main
// test helpers (context_test.go, gate_test.go, context_conflict_e2e_test.go
// — a different package, not importable here), so this file re-homes the
// minimal subset Task 2's moved tests need, reading the same real,
// already-cross-validated fixtures those helpers did (never a hand-authored
// duplicate of the policy/fragment corpus).
package readinessload

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// policyStoreFiles reads the real, already-cross-validated policy fixture
// internal/contextcompile's own integration tests install, keyed by their
// repo-relative .verdi/policy/ path — mirrors cmd/verdi/context_test.go's
// own contextPolicyStoreFiles.
func policyStoreFiles(t *testing.T) map[string]string {
	t.Helper()
	rels := []string{
		"constitution.md",
		"policies/go-toolchain.md",
		"overlays/frontend-go-version.md",
		"exemptions/legacy-service-go.md",
		"profiles/solo-default.md",
	}
	out := make(map[string]string, len(rels))
	for _, rel := range rels {
		data, err := os.ReadFile(filepath.Join("..", "policyartifact", "testdata", "store", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read policy fixture %s: %v", rel, err)
		}
		out[".verdi/policy/"+rel] = string(data)
	}
	return out
}

// featureAlphaSpec reads internal/contextcompile's own real feature-alpha
// fixture spec verbatim — mirrors cmd/verdi/context_test.go's own
// contextFeatureAlphaSpec.
func featureAlphaSpec(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "contextcompile", "testdata", "fragments", "feature-alpha.md"))
	if err != nil {
		t.Fatalf("read feature-alpha fixture: %v", err)
	}
	return string(data)
}

// buildCompileRepo builds a fresh, real fixturegit repository carrying a
// minimal store manifest, the real policy-store fixture, and specFiles,
// then generates and commits the one real managed instruction projection —
// mirrors cmd/verdi/context_test.go's own buildContextCompileRepo.
func buildCompileRepo(t *testing.T, specFiles map[string]string) *fixturegit.Repo {
	t.Helper()
	files := policyStoreFiles(t)
	files[".verdi/verdi.yaml"] = "schema: verdi.layout/v1\n"
	for path, content := range specFiles {
		files[path] = content
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	repo.Head = commitAll(t, repo.Dir, "generate instruction projection")
	return repo
}

// requestBytes builds and canonically encodes a
// verdi.context-compile-request/v1 document through
// internal/contextcompile's own EncodeRequest seam — mirrors
// cmd/verdi/context_test.go's own contextRequestBytes.
func requestBytes(t *testing.T, spec string, phase contextcompile.Phase) []byte {
	t.Helper()
	req := contextcompile.Request{
		Schema:  contextcompile.RequestSchema,
		Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Phase:   phase,
		Scope:   policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		Spec:    spec,
	}
	data, err := contextcompile.EncodeRequest(req)
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	return data
}

func writeRequestFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write request file: %v", err)
	}
	return path
}

func checkoutBranch(t *testing.T, dir, name string) {
	t.Helper()
	cmd := exec.Command("git", "checkout", "-b", name)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b %s: %v\n%s", name, err, out)
	}
}

// commitAll stages every working-tree change and commits it on whatever
// branch dir currently has checked out, returning the new HEAD sha —
// mirrors cmd/verdi/gate_test.go's own commitAllOnCurrentBranch.
func commitAll(t *testing.T, dir, message string) string {
	t.Helper()
	add := exec.Command("git", "add", "-A")
	add.Dir = dir
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	commit := exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--quiet", "--no-verify", "-m", message)
	commit.Dir = dir
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	rev := exec.Command("git", "rev-parse", "HEAD")
	rev.Dir = dir
	out, err := rev.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return string(out[:len(out)-1])
}

// writeJudgeScript writes a hermetic, no-network fake judge command — body
// is its shell script body — mirrors
// cmd/verdi/context_conflict_e2e_test.go's own writeContextConflictJudge.
func writeJudgeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "judge.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("write fake judge: %v", err)
	}
	return path
}

// configureJudge configures repo's manifest align.judge_cmd to command and
// commits it — mirrors
// cmd/verdi/context_conflict_e2e_test.go's own configureContextConflictJudge.
func configureJudge(t *testing.T, repo *fixturegit.Repo, command string) {
	t.Helper()
	manifest := "schema: verdi.layout/v1\nalign:\n  judge_cmd: [" + command + "]\n"
	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi", "verdi.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write verdi.yaml: %v", err)
	}
	repo.Head = commitAll(t, repo.Dir, "configure conflict judge")
}

// noConflictJudgeResult is a strict, canonical, empty-findings
// verdi.policy-conflict-judge-result/v1 document — the fixed successful
// judge stdout every hermetic judge script in this file's tests prints.
const noConflictJudgeResult = `{"findings":[],"recommendation":"no-conflict","schema":"verdi.policy-conflict-judge-result/v1"}`

// readinessRepo builds a fresh fixturegit repository carrying one feature
// or story spec (class selects which) and returns it alongside its target
// ref — the moved recipe cmd/verdi/readiness_snapshot_integration_test.go's
// old readinessSnapshotRepo built, minus the mandatory design-branch
// checkout ac-2 removes: callers that need a specific branch check one out
// themselves.
func readinessRepo(t *testing.T, class string) (*fixturegit.Repo, string) {
	t.Helper()
	name := "feature-alpha"
	files := map[string]string{".verdi/specs/active/feature-alpha/spec.md": featureAlphaSpec(t)}
	if class == "story" {
		name = "story-alpha"
		files = map[string]string{".verdi/specs/active/story-alpha/spec.md": readinessStorySpec}
	}
	repo := buildCompileRepo(t, files)
	return repo, "spec/" + name
}

const readinessStorySpec = `---
id: spec/story-alpha
kind: spec
title: "Borrower appeal: exact source title"
owners: [alpha-team]
class: story
story: jira:ALPHA-1
problem: {text: "The story is unclear.", anchor: problem}
outcome: {text: "The story is reviewable.", anchor: outcome}
acceptance_criteria:
  - {id: ac-1, text: "the story works", evidence: [behavioral], anchor: ac-1}
open_questions:
  - {id: oq-1, text: "which route applies?", anchor: oq-1}
links:
  - {type: implements, ref: spec/feature-alpha#ac-1}
---
# Story Alpha

## Problem

The story is unclear.

## Outcome

The story is reviewable.

## AC-1

The story works.

## OQ-1

Which route applies?
`
