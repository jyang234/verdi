package constitutionapp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/instructionprojection"
)

// storeFixtureFiles reads testdata/store's complete tree into the map shape
// fixturegit.Layer.Files expects, so every constitutionapp test builds from
// the SAME committed fixture bytes (copied from
// internal/policyauthority/testdata/store — the one existing fixture
// already exercising a complete, cross-validated constitution) rather than
// each test authoring its own ad hoc YAML.
func storeFixtureFiles(t testing.TB) map[string]string {
	t.Helper()
	root := filepath.Join("testdata", "store")
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("reading constitution store fixture: %v", err)
	}
	return files
}

// buildFixtureRepo builds one deterministic fixturegit repository whose
// default branch is "main" and whose HEAD already carries the fixture
// constitution store — the accepted state every test starts from. It sets
// CI_DEFAULT_BRANCH so internal/specstate.ResolveDefaultBranch resolves
// hermetically, mirroring internal/lint's own precedent for a fixturegit
// repo with no configured remote.
func buildFixtureRepo(t testing.TB) string {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: storeFixtureFiles(t), Message: "adopt constitution"},
	})
	resolved, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(resolved)
}

func readTestdataFile(t testing.TB, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "store", ".verdi", "policy", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func testService() Service {
	return Service{Git: gitxReader{}, Authority: policyauthorityStore{}, Conflict: localConflictEvaluator{}}
}

// operandFeatureSpec is a minimal, decodable feature spec at the exact
// shape internal/policyconflict's own operand_test.go fixture uses, so a
// governed target resolves through internal/contextcompile without this
// package inventing a second spec grammar.
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

var fixtureCommitEnv = []string{
	"TZ=UTC",
	"GIT_AUTHOR_NAME=Verdi Fixture",
	"GIT_AUTHOR_EMAIL=fixture@verdi.invalid",
	"GIT_AUTHOR_DATE=1704067200 +0000",
	"GIT_COMMITTER_NAME=Verdi Fixture",
	"GIT_COMMITTER_EMAIL=fixture@verdi.invalid",
	"GIT_COMMITTER_DATE=1704067200 +0000",
}

func runFixtureGit(t testing.TB, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), fixtureCommitEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// noLegacyGoPolicy is a second policy, paired with
// goToolchainAllowsLegacyPolicy below in buildUnauthorizedExemptionFixtureRepo:
// it forbids every value the paired policy's own claim allows, both at
// universal scope, so internal/policyconflict's mechanical solver proves an
// unsatisfiable conjunction outright (no judge involved).
const noLegacyGoPolicy = `---
schema: verdi.policy/v1
id: policy/no-legacy-go
kind: policy
title: "No legacy Go versions"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims:
  - id: forbid-legacy-go
    family: configuration
    operator: forbidden-values
    subject: go-version
    values: ["1.25", "1.24"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: false
instructions: []
payloads: {}
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
Nothing may declare the legacy toolchain versions the exemption departs from.
`

// goToolchainAllowsLegacyPolicy is the first half of the deliberate
// universal-scope conflict; noLegacyGoPolicy above is the second.
const goToolchainAllowsLegacyPolicy = `---
schema: verdi.policy/v1
id: policy/go-toolchain
kind: policy
title: "Go toolchain policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims:
  - id: go-version
    family: configuration
    operator: allowed-values
    subject: go-version
    values: ["1.24", "1.25"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
instructions: []
payloads: {}
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
Pin the Go toolchain.
`

// unauthorizedExemption departs from goToolchainAllowsLegacyPolicy's claim
// at UNIVERSAL scope — the same scope as the conflict itself, so the
// exemption's Scope authority-resolution substate is trivially proven and
// the test isolates exactly one failing substate: Authorization. Its
// approval names a role/principal pair the fixture's own solo-default
// profile does not map (spec/context-integrity-v2 AC-1's governance
// profile: only the mapped role/subject pair may satisfy an authoritative
// approval requirement).
const unauthorizedExemption = `---
schema: verdi.policy-exemption/v1
id: policy-exemption/universal-legacy-go
kind: policy-exemption
title: "Unauthorized universal exemption"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
witnesses:
  - policy: policy/go-toolchain
    claim: go-version
    claim_digest: "sha256:939dc350ca2599363d9b5b89ecf681061f35081ed39025e785696d8f92c23261"
compensating_controls:
  - "None."
approvals:
  - role: policy-owner
    principal: principal/github-org/Ym9i
expiry: "2099-12-31"
template: {identity: "embedded:policy-exemption.md", digest: "sha256:cf3977e08d4259c963e3b7ca9b974e2334d35548ac155b0e972bc7441733dad9"}
---
Nobody named "bob" is a mapped policy-owner in the solo-default profile, so
this exemption's approval can never authorize covering a conflict.
`

// buildUnauthorizedExemptionFixtureRepo builds a self-contained constitution
// (its own constitution/profile, not the shared testdata/store fixture) with
// one deliberate universal-scope mechanical conflict and one universal-scope
// exemption whose approval the governing profile does not authorize —
// isolating the Authorization authority-resolution substate specifically,
// with every other substate (Match/Freshness/Scope/Bound) trivially proven
// so the test cannot be confused for a scope or freshness failure instead.
func buildUnauthorizedExemptionFixtureRepo(t testing.TB) string {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	files := map[string]string{
		".verdi/policy/constitution.md":                   readTestdataFile(t, "constitution.md"),
		".verdi/policy/profiles/solo-default.md":          readTestdataFile(t, "profiles/solo-default.md"),
		".verdi/policy/policies/go-toolchain.md":          goToolchainAllowsLegacyPolicy,
		".verdi/policy/policies/no-legacy-go.md":          noLegacyGoPolicy,
		".verdi/policy/exemptions/universal-legacy-go.md": unauthorizedExemption,
		".verdi/specs/active/operand-feature/spec.md":     operandFeatureSpec,
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "scaffold"}})

	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	runFixtureGit(t, repo.Dir, "add", "-A")
	runFixtureGit(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "generate instruction projection")

	resolved, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(resolved)
}

// buildConflictFixtureRepo builds the same policy-store fixture
// buildFixtureRepo does, plus one accepted feature spec and a generated,
// committed instruction projection — the additional real-git machinery
// internal/contextcompile's own accepted-context pipeline requires before
// it will accept a governed target at all (stage 5: an existing managed
// projection must be present and non-drifted). Used only by the
// conflict-evaluation tests that need a target to actually resolve, rather
// than every test paying this extra cost.
func buildConflictFixtureRepo(t testing.TB) string {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	files := storeFixtureFiles(t)
	files[".verdi/specs/active/operand-feature/spec.md"] = operandFeatureSpec
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "scaffold"}})

	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	runFixtureGit(t, repo.Dir, "add", "-A")
	runFixtureGit(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "generate instruction projection")

	resolved, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(resolved)
}
