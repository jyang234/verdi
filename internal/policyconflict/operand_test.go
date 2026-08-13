// Task 4 RED/GREEN matrix for ResolveOperands (authority design §§2-3):
// dispatch to the injected compiler's accepted-context arm, mapping onto
// contextcompile.ResolveConflictCandidate for the acceptance-candidate arm,
// and Task 3's own malformed-request refusals. Test names match
// -run 'Test.*(ConflictOperands|ConflictCandidate|ResolveOperands)'.
package policyconflict

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// operandCommitEnv is a fixed author/committer/date environment so this
// file's own direct git commits (outside fixturegit.Build's own layers)
// stay deterministic, mirroring internal/contextcompile's own
// integrationCommitEnv.
var operandCommitEnv = []string{
	"TZ=UTC",
	"GIT_AUTHOR_NAME=Verdi Fixture",
	"GIT_AUTHOR_EMAIL=fixture@verdi.invalid",
	"GIT_AUTHOR_DATE=1704067200 +0000",
	"GIT_COMMITTER_NAME=Verdi Fixture",
	"GIT_COMMITTER_EMAIL=fixture@verdi.invalid",
	"GIT_COMMITTER_DATE=1704067200 +0000",
}

func runOperandGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), operandCommitEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

const operandFeatureSpec = `---
id: spec/operand-feature
kind: spec
class: feature
title: "Operand feature"
owners: [platform-team]
problem: {text: "The operand feature problem.", anchor: problem}
outcome: {text: "The operand feature outcome.", anchor: outcome}
acceptance_criteria:
  - {id: ac-1, text: "the operand feature acceptance criterion.", evidence: [static]}
---
# Operand feature

## Problem

The operand feature problem.

## Outcome

The operand feature outcome.
`

// operandPolicyStoreFiles reads the same real policy fixture
// internal/contextcompile's own tests install, keyed by its repo-relative
// .verdi/policy/ path.
func operandPolicyStoreFiles(t *testing.T) map[string]string {
	t.Helper()
	files := []string{
		"constitution.md",
		"policies/go-toolchain.md",
		"overlays/frontend-go-version.md",
		"exemptions/legacy-service-go.md",
		"profiles/solo-default.md",
	}
	out := make(map[string]string, len(files))
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join("..", "policyartifact", "testdata", "store", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read policy fixture %s: %v", rel, err)
		}
		out[".verdi/policy/"+rel] = string(data)
	}
	return out
}

// operandFixtureRepo builds a hermetic real git repository carrying the
// policy store plus one feature spec, then generates and commits the real
// managed instruction projection so an accepted-context compile's stage 5
// finds a clean, non-drifted projection.
func operandFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	files := operandPolicyStoreFiles(t)
	files[".verdi/specs/active/operand-feature/spec.md"] = operandFeatureSpec
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	runOperandGit(t, repo.Dir, "add", "-A")
	runOperandGit(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "generate instruction projection")
	repo.Head = strings.TrimSpace(runOperandGit(t, repo.Dir, "rev-parse", "HEAD"))
	return repo
}

func universalScope() policyartifact.Scope {
	return policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}}
}

func acceptedOperandRequest(spec string) contextcompile.Request {
	return contextcompile.Request{
		Schema:  contextcompile.RequestSchema,
		Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Phase:   contextcompile.PhaseDesign,
		Scope:   universalScope(),
		Spec:    spec,
	}
}

// --- 9: accepted arm delegates to the injected compiler -------------------

func TestResolveOperandsAcceptedArmDelegatesToInjectedCompiler(t *testing.T) {
	repo := operandFixtureRepo(t)
	ccReq := acceptedOperandRequest("spec/operand-feature")
	req := Request{
		Schema: RequestSchema,
		Target: Target{Kind: TargetAcceptedContext, AcceptedContext: &ccReq},
	}

	operands, err := ResolveOperands(context.Background(), contextcompile.NewCompiler(), repo.Dir, req, contextcompile.ConflictFacts{})
	if err != nil {
		t.Fatalf("ResolveOperands: unexpected error: %v", err)
	}
	view, err := operands.View()
	if err != nil {
		t.Fatalf("View: unexpected error: %v", err)
	}
	if view.Snapshot.TargetKind != "accepted-context" {
		t.Errorf("Snapshot.TargetKind = %q, want accepted-context", view.Snapshot.TargetKind)
	}
	if view.Snapshot.ManifestDigest == "" {
		t.Error("Snapshot.ManifestDigest is empty, want set")
	}
	if view.Snapshot.CandidateDigest != "" {
		t.Error("Snapshot.CandidateDigest is non-empty for the accepted arm")
	}

	// Cross-check against a direct CompileConflict call: ResolveOperands is
	// a thin dispatcher, so both paths must resolve identical operands.
	direct, err := contextcompile.NewCompiler().CompileConflict(context.Background(), repo.Dir, ccReq, contextcompile.ConflictFacts{})
	if err != nil {
		t.Fatalf("CompileConflict (direct): unexpected error: %v", err)
	}
	directView, err := direct.View()
	if err != nil {
		t.Fatalf("View (direct): unexpected error: %v", err)
	}
	if directView.Snapshot.ManifestDigest != view.Snapshot.ManifestDigest {
		t.Errorf("ResolveOperands and direct CompileConflict disagree on ManifestDigest: %q vs %q", view.Snapshot.ManifestDigest, directView.Snapshot.ManifestDigest)
	}
}

func TestResolveOperandsAcceptedArmZeroCompilerFails(t *testing.T) {
	repo := operandFixtureRepo(t)
	ccReq := acceptedOperandRequest("spec/operand-feature")
	req := Request{
		Schema: RequestSchema,
		Target: Target{Kind: TargetAcceptedContext, AcceptedContext: &ccReq},
	}
	var zero contextcompile.Compiler
	_, err := ResolveOperands(context.Background(), zero, repo.Dir, req, contextcompile.ConflictFacts{})
	if err == nil {
		t.Fatal("expected a zero-value Compiler to fail closed for the accepted arm")
	}
}

// --- 10: candidate arm maps AcceptanceCandidate exactly onto CandidateRequest

const operandCandidateSpec = `---
id: spec/operand-candidate
kind: spec
class: feature
title: "Operand candidate"
owners: [platform-team]
problem: {text: "The operand candidate problem.", anchor: problem}
outcome: {text: "The operand candidate outcome.", anchor: outcome}
acceptance_criteria:
  - {id: ac-1, text: "the operand candidate acceptance criterion.", evidence: [static]}
---
# Operand candidate

## Problem

The operand candidate problem.

## Outcome

The operand candidate outcome.
`

func TestResolveOperandsCandidateArmMapsFieldsExactly(t *testing.T) {
	files := operandPolicyStoreFiles(t)
	files[".verdi/specs/active/operand-candidate/spec.md"] = operandCandidateSpec
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	candidate := &AcceptanceCandidate{
		Adapter:  contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Expected: contextcompile.Expected{Branch: "main", Head: repo.Head},
		Grants:   execworkspace.GrantSet{},
		Scope:    universalScope(),
		Spec:     "spec/operand-candidate",
	}
	req := Request{
		Schema: RequestSchema,
		Target: Target{Kind: TargetAcceptanceCandidate, AcceptanceCandidate: candidate},
	}

	operands, err := ResolveOperands(context.Background(), contextcompile.NewCompiler(), repo.Dir, req, contextcompile.ConflictFacts{})
	if err != nil {
		t.Fatalf("ResolveOperands: unexpected error: %v", err)
	}
	view, err := operands.View()
	if err != nil {
		t.Fatalf("View: unexpected error: %v", err)
	}
	if view.Snapshot.TargetKind != "acceptance-candidate" {
		t.Errorf("Snapshot.TargetKind = %q, want acceptance-candidate", view.Snapshot.TargetKind)
	}
	if view.Snapshot.ManifestDigest != "" {
		t.Error("Snapshot.ManifestDigest is non-empty for the candidate arm")
	}
	if view.Snapshot.CandidateDigest == "" {
		t.Error("Snapshot.CandidateDigest is empty, want set")
	}
	if view.Snapshot.Adapter != candidate.Adapter {
		t.Errorf("Snapshot.Adapter = %+v, want %+v (mapped from AcceptanceCandidate.Adapter)", view.Snapshot.Adapter, candidate.Adapter)
	}
}

// --- 11: malformed requests fail, reusing Task 3 validation ---------------

func TestResolveOperandsMalformedRequestsFail(t *testing.T) {
	repo := operandFixtureRepo(t)
	ccReq := acceptedOperandRequest("spec/operand-feature")
	validCandidate := &AcceptanceCandidate{
		Adapter:  contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Expected: contextcompile.Expected{Branch: "main", Head: strings.Repeat("a", 40)},
		Grants:   execworkspace.GrantSet{},
		Scope:    universalScope(),
		Spec:     "spec/operand-feature",
	}

	cases := map[string]Request{
		"wrong schema": {
			Schema: "not-the-schema",
			Target: Target{Kind: TargetAcceptedContext, AcceptedContext: &ccReq},
		},
		"both arms present": {
			Schema: RequestSchema,
			Target: Target{Kind: TargetAcceptedContext, AcceptedContext: &ccReq, AcceptanceCandidate: validCandidate},
		},
		"neither arm present": {
			Schema: RequestSchema,
			Target: Target{Kind: TargetAcceptedContext},
		},
		"kind/arm mismatch": {
			Schema: RequestSchema,
			Target: Target{Kind: TargetAcceptanceCandidate, AcceptedContext: &ccReq},
		},
		"unknown target kind": {
			Schema: RequestSchema,
			Target: Target{Kind: "bogus"},
		},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ResolveOperands(context.Background(), contextcompile.NewCompiler(), repo.Dir, req, contextcompile.ConflictFacts{})
			if err == nil {
				t.Fatalf("%s: expected ResolveOperands to fail closed", name)
			}
		})
	}
}

func TestResolveOperandsEmptyRootFails(t *testing.T) {
	ccReq := acceptedOperandRequest("spec/operand-feature")
	req := Request{Schema: RequestSchema, Target: Target{Kind: TargetAcceptedContext, AcceptedContext: &ccReq}}
	_, err := ResolveOperands(context.Background(), contextcompile.NewCompiler(), "", req, contextcompile.ConflictFacts{})
	if err == nil {
		t.Fatal("expected an empty root to fail closed")
	}
}
