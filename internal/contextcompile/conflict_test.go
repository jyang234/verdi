// Task 4 RED/GREEN matrix for the sealed conflict-operand seam (authority
// design §§2-3, 6, 12): accepted-context construction reusing exactly one
// compile/policy resolution, acceptance-candidate construction reading the
// exact HEAD-tree spec blob, sealing/mutation-guard/cross-snapshot
// integrity, and the closed operational failure taxonomy. Test names match
// -run 'Test.*(ConflictOperands|ConflictCandidate|ResolveOperands)'.
package contextcompile

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/repositoryfacts"
)

// --- shared hermetic accepted-arm fixture -----------------------------

// countingLoader wraps an AuthorityLoader, counting Load/Resolve calls so a
// test can prove CompileConflict runs exactly one authority resolution
// (never a second, conflict-only reload).
type countingLoader struct {
	inner           AuthorityLoader
	loads, resolves *int
}

func (l countingLoader) Load(root string) (*policyauthority.Store, error) {
	if l.loads != nil {
		*l.loads++
	}
	return l.inner.Load(root)
}

func (l countingLoader) Resolve(store *policyauthority.Store) (*policyauthority.EffectivePolicy, error) {
	if l.resolves != nil {
		*l.resolves++
	}
	return l.inner.Resolve(store)
}

// countingGather wraps a RepositoryFactsGatherer, counting Gather calls.
type countingGather struct {
	inner RepositoryFactsGatherer
	calls *int
}

func (g countingGather) Gather(ctx context.Context, in repositoryfacts.GatherInput) (repositoryfacts.Snapshot, error) {
	if g.calls != nil {
		*g.calls++
	}
	return g.inner.Gather(ctx, in)
}

// policyDiskFallbackGit wraps a GitReader, answering every `.verdi/policy/`
// Show read directly from disk at the call's own root argument instead of
// delegating to the wrapped fake. compilerAcceptedFixture's own GitReader
// double only knows the story/feature spec paths its fragment fixtures
// need (fragments_test.go's fixture set); it was never taught the policy
// store's paths. compilePipeline's stage 9 unconditionally re-reads every
// selected policy/overlay/exemption/constitution/profile authority
// artifact's exact HEAD bytes for its adopted-digest TOCTOU check
// (authority_selection.go's requireAdoptedAuthorityDigest) — a check a
// real committed constitution store would satisfy from the same commit
// stage 2's authority load resolved from. This double supplies that half
// of the double's contract from the identical real files
// installPolicyFixture already wrote to root, without touching or
// weakening any assertion this test file makes.
type policyDiskFallbackGit struct {
	GitReader
}

func (g policyDiskFallbackGit) Show(ctx context.Context, root, ref, path string) ([]byte, error) {
	if strings.HasPrefix(path, ".verdi/policy/") {
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	}
	return g.GitReader.Show(ctx, root, ref, path)
}

// hermeticAcceptedFixture wires a Compiler over compilerAcceptedFixture's
// fake GitReader/StateResolver, installPolicyFixture's policy store, a
// clean projection report, and a stub repository-facts gatherer, ready for
// an accepted-context CompileConflict call at PhaseBuild. loadCalls,
// resolveCalls, and gatherCalls, when non-nil, count the matching port
// method invocations.
func hermeticAcceptedFixture(t *testing.T, loadCalls, resolveCalls, gatherCalls *int) (Compiler, Request) {
	t.Helper()
	_ = installPolicyFixture(t) // policyauthority.Load reads the real cwd-relative testdata store, not root; see defaultAuthorityLoader below.
	git, states, ref := compilerAcceptedFixture(t)
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	projection := stubProjectionVerifier{report: &instructionprojection.Report{}}
	gitWT := gitWithWorktree{GitReader: policyDiskFallbackGit{GitReader: git}, worktree: func(context.Context, string) ([]string, error) { return nil, nil }}

	loader := AuthorityLoader(countingLoader{inner: defaultAuthorityLoader{}, loads: loadCalls, resolves: resolveCalls})
	rf := RepositoryFactsGatherer(countingGather{inner: repoFacts, calls: gatherCalls})

	c := newCompilerWithPorts(gitWT, states, loader, nil, rf, projection)
	req := validCompileRequest(ref)
	return c, req
}

// hermeticAcceptedRoot returns the root a hermeticAcceptedFixture Compiler
// must be called with: the real installed policy fixture directory
// (defaultAuthorityLoader delegates to policyauthority.Load(root), which
// reads root's .verdi/policy tree for real).
func hermeticAcceptedRoot(t *testing.T) string {
	t.Helper()
	return installPolicyFixture(t)
}

// --- 1: accepted construction reuses ONE policy resolution ----------------

func TestCompileConflictOperandsAcceptedReusesOnePolicyLoad(t *testing.T) {
	loads, resolves, gathers := 0, 0, 0
	c, req := hermeticAcceptedFixture(t, &loads, &resolves, &gathers)
	root := hermeticAcceptedRoot(t)

	operands, err := c.CompileConflict(context.Background(), root, req, ConflictFacts{})
	if err != nil {
		t.Fatalf("CompileConflict: unexpected error: %v", err)
	}
	if operands == nil {
		t.Fatal("CompileConflict returned nil operands")
	}
	if loads != 1 {
		t.Errorf("AuthorityLoader.Load called %d times, want exactly 1", loads)
	}
	if resolves != 1 {
		t.Errorf("AuthorityLoader.Resolve called %d times, want exactly 1", resolves)
	}
	if gathers != 1 {
		t.Errorf("RepositoryFactsGatherer.Gather called %d times, want exactly 1", gathers)
	}
}

// --- 2: accepted SnapshotIdentity -----------------------------------------

func TestCompileConflictOperandsAcceptedSnapshotIdentity(t *testing.T) {
	c, req := hermeticAcceptedFixture(t, nil, nil, nil)
	root := hermeticAcceptedRoot(t)
	operands, err := c.CompileConflict(context.Background(), root, req, ConflictFacts{})
	if err != nil {
		t.Fatalf("CompileConflict: unexpected error: %v", err)
	}
	view, err := operands.View()
	if err != nil {
		t.Fatalf("View: unexpected error: %v", err)
	}

	snap := view.Snapshot
	if snap.TargetKind != snapshotTargetAcceptedContext {
		t.Errorf("TargetKind = %q, want %q", snap.TargetKind, snapshotTargetAcceptedContext)
	}
	if snap.ManifestDigest == "" {
		t.Error("ManifestDigest is empty, want set for an accepted-context snapshot")
	}
	if snap.CandidateDigest != "" {
		t.Errorf("CandidateDigest = %q, want empty for an accepted-context snapshot", snap.CandidateDigest)
	}
	if snap.EffectivePolicyDigest == "" || snap.ConstitutionDigest == "" || snap.ProfileID == "" || snap.ProfileDigest == "" {
		t.Errorf("snapshot authority identity incomplete: %+v", snap)
	}
	if snap.Adapter != (AdapterRef{ID: "codex", Version: "1"}) {
		t.Errorf("Adapter = %+v", snap.Adapter)
	}
	if snap.Phase != PhaseBuild {
		t.Errorf("Phase = %q, want %q", snap.Phase, PhaseBuild)
	}
	if snap.GrantDigest == "" {
		t.Error("GrantDigest is empty")
	}
	if err := snap.Repository.Validate(); err != nil {
		t.Errorf("Repository facts invalid: %v", err)
	}

	if len(snap.Sources) == 0 {
		t.Fatal("Sources is empty, want the capsule's contributing artifacts")
	}
	for i := 1; i < len(snap.Sources); i++ {
		a, b := snap.Sources[i-1], snap.Sources[i]
		if a == b {
			t.Fatalf("Sources contains a duplicate entry: %+v", a)
		}
		less := a.Ref < b.Ref || (a.Ref == b.Ref && (a.Path < b.Path || (a.Path == b.Path && a.ContentDigest < b.ContentDigest)))
		if !less {
			t.Fatalf("Sources is not sorted by (ref,path,content_digest): %+v then %+v", a, b)
		}
	}
	foundPolicy, foundTarget := false, false
	for _, s := range snap.Sources {
		if s.Ref == "policy/go-toolchain" {
			foundPolicy = true
		}
		if s.Ref == "spec/story-multi-parent" {
			foundTarget = true
		}
	}
	if !foundPolicy {
		t.Errorf("Sources missing the applicable policy/go-toolchain source: %+v", snap.Sources)
	}
	if !foundTarget {
		t.Errorf("Sources missing the accepted target source: %+v", snap.Sources)
	}

	if len(view.TypedClaims) == 0 {
		t.Error("TypedClaims is empty, want the applicable policy's claims")
	}
	foundInstruction := false
	for _, pc := range view.ProseClaims {
		if pc.Category == categoryPolicyInstruction {
			foundInstruction = true
		}
	}
	if !foundInstruction {
		t.Errorf("ProseClaims missing a policy-instruction claim: %+v", view.ProseClaims)
	}
}

// --- 5: mutation-after-View safety and hand-built rejection ---------------

func TestConflictOperandsMutationSafetyAcceptedArm(t *testing.T) {
	c, req := hermeticAcceptedFixture(t, nil, nil, nil)
	root := hermeticAcceptedRoot(t)
	operands, err := c.CompileConflict(context.Background(), root, req, ConflictFacts{})
	if err != nil {
		t.Fatalf("CompileConflict: unexpected error: %v", err)
	}

	before, err := operands.View()
	if err != nil {
		t.Fatalf("View (1): unexpected error: %v", err)
	}

	mutated, err := operands.View()
	if err != nil {
		t.Fatalf("View (2): unexpected error: %v", err)
	}
	// Mutate every reachable nested field on the SECOND view; the operands'
	// internal state must be unaffected.
	if len(mutated.TypedClaims) > 0 {
		mutated.TypedClaims[0].PolicyID = "tampered"
		mutated.TypedClaims[0].Claim.ID = "tampered"
		mutated.TypedClaims[0].Claim.Values = append(mutated.TypedClaims[0].Claim.Values, "tampered")
		mutated.TypedClaims[0].Claim.Scope.Refs = append(mutated.TypedClaims[0].Claim.Scope.Refs, "tampered")
	}
	if len(mutated.ProseClaims) > 0 {
		mutated.ProseClaims[0].Text = "tampered"
		mutated.ProseClaims[0].Scope.Paths = append(mutated.ProseClaims[0].Scope.Paths, "tampered")
	}
	mutated.Actors = append(mutated.Actors, mutated.Actors...)
	mutated.Snapshot.Scope.Environments = append(mutated.Snapshot.Scope.Environments, "tampered")
	mutated.Snapshot.Sources = append(mutated.Snapshot.Sources, ConflictSourceIdentity{Ref: "tampered"})
	mutated.EffectivePolicy.Policies = append(mutated.EffectivePolicy.Policies, policyauthority.EffectivePolicyEntry{PolicyID: "tampered"})
	mutated.Profile.RoleMappings = append(mutated.Profile.RoleMappings, governanceprincipal.RoleMapping{Role: "tampered"})
	if len(mutated.Exemptions) > 0 {
		mutated.Exemptions[0].Title = "tampered"
	}

	after, err := operands.View()
	if err != nil {
		t.Fatalf("View (3, after external mutation): unexpected error: %v", err)
	}
	if !conflictViewsEqual(before, after) {
		t.Fatalf("View drifted after mutating a previously returned clone")
	}
}

// --- 6: cross-snapshot substitution and hand-built operands fail closed ---

func TestConflictOperandsHandBuiltFailsClosed(t *testing.T) {
	var zero ConflictOperands
	if _, err := zero.View(); err == nil {
		t.Fatal("zero-value ConflictOperands.View() = nil error, want failure")
	}

	var nilPtr *ConflictOperands
	if _, err := nilPtr.View(); err == nil {
		t.Fatal("nil *ConflictOperands.View() = nil error, want failure")
	}

	c, req := hermeticAcceptedFixture(t, nil, nil, nil)
	root := hermeticAcceptedRoot(t)
	operands, err := c.CompileConflict(context.Background(), root, req, ConflictFacts{})
	if err != nil {
		t.Fatalf("CompileConflict: unexpected error: %v", err)
	}
	view, err := operands.View()
	if err != nil {
		t.Fatalf("View: unexpected error: %v", err)
	}
	handBuilt := &ConflictOperands{view: view} // no seal minted
	if _, err := handBuilt.View(); err == nil {
		t.Fatal("hand-built ConflictOperands (view only, no seal).View() = nil error, want failure")
	}
	handBuilt2 := &ConflictOperands{view: view, seal: "sha256:" + strings.Repeat("0", 64)}
	if _, err := handBuilt2.View(); err == nil {
		t.Fatal("hand-built ConflictOperands with a forged seal.View() = nil error, want failure")
	}
}

func TestConflictOperandsCrossSnapshotSubstitutionFailsClosed(t *testing.T) {
	root := hermeticAcceptedRoot(t)
	cA, reqA := hermeticAcceptedFixture(t, nil, nil, nil)
	opA, err := cA.CompileConflict(context.Background(), root, reqA, ConflictFacts{})
	if err != nil {
		t.Fatalf("CompileConflict(A): unexpected error: %v", err)
	}
	cB, reqB := hermeticAcceptedFixture(t, nil, nil, nil)
	opB, err := cB.CompileConflict(context.Background(), root, reqB, ConflictFacts{})
	if err != nil {
		t.Fatalf("CompileConflict(B): unexpected error: %v", err)
	}

	// Splice B's view under A's seal: a cross-snapshot substitution.
	tampered := &ConflictOperands{view: opB.view, seal: opA.seal}
	if _, err := tampered.View(); err == nil {
		t.Fatal("cross-snapshot spliced ConflictOperands.View() = nil error, want failure")
	}
}

// --- 3/4/7/8: acceptance-candidate construction (fixture Git) -------------

const candidateFeatureSpec = `---
id: spec/candidate-feature
kind: spec
class: feature
title: "Candidate feature"
owners: [platform-team]
problem: {text: "The candidate feature problem.", anchor: problem}
outcome: {text: "The candidate feature outcome.", anchor: outcome}
acceptance_criteria:
  - {id: ac-1, text: "the candidate feature acceptance criterion.", evidence: [static]}
---
# Candidate feature

## Problem

The candidate feature problem.

## Outcome

The candidate feature outcome.
`

// candidateFixtureRepo builds a hermetic real git repository carrying the
// policy store plus one spec at .verdi/specs/active/<name>/spec.md on
// branch main.
func candidateFixtureRepo(t *testing.T, specPath, specContent string) *fixturegit.Repo {
	t.Helper()
	files := policyStoreFiles(t)
	files[specPath] = specContent
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return repo
}

func candidateRequestFor(spec, branch, head string) CandidateRequest {
	return CandidateRequest{
		Adapter:  AdapterRef{ID: "codex", Version: "1"},
		Expected: Expected{Branch: branch, Head: head},
		Grants:   execworkspace.GrantSet{},
		Scope:    policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		Spec:     spec,
	}
}

func TestResolveConflictCandidateReadsExactHeadBlob(t *testing.T) {
	repo := candidateFixtureRepo(t, ".verdi/specs/active/candidate-feature/spec.md", candidateFeatureSpec)
	c := NewCompiler()
	operands, err := c.resolveConflictCandidate(context.Background(), repo.Dir, candidateRequestFor("spec/candidate-feature", "main", repo.Head), ConflictFacts{})
	if err != nil {
		t.Fatalf("resolveConflictCandidate: unexpected error: %v", err)
	}
	view, err := operands.View()
	if err != nil {
		t.Fatalf("View: unexpected error: %v", err)
	}
	snap := view.Snapshot
	if snap.TargetKind != snapshotTargetAcceptanceCandidate {
		t.Errorf("TargetKind = %q, want %q", snap.TargetKind, snapshotTargetAcceptanceCandidate)
	}
	if snap.ManifestDigest != "" {
		t.Errorf("ManifestDigest = %q, want empty for an acceptance-candidate snapshot", snap.ManifestDigest)
	}
	if snap.CandidateDigest == "" {
		t.Error("CandidateDigest is empty, want set for an acceptance-candidate snapshot")
	}
	if snap.Phase != PhaseDesign {
		t.Errorf("Phase = %q, want %q (candidate phase is fixed to design)", snap.Phase, PhaseDesign)
	}
	wantDigest := rawContentDigest([]byte(candidateFeatureSpec))
	if snap.CandidateDigest != wantDigest {
		t.Errorf("CandidateDigest = %q, want %q (the exact HEAD-tree blob's digest)", snap.CandidateDigest, wantDigest)
	}
	found := false
	for _, pc := range view.ProseClaims {
		if pc.Category == categorySpecProblem {
			found = true
		}
	}
	if !found {
		t.Errorf("ProseClaims missing spec-problem for the candidate's own target: %+v", view.ProseClaims)
	}
}

func TestResolveConflictCandidateNeverManifestIdentity(t *testing.T) {
	repo := candidateFixtureRepo(t, ".verdi/specs/active/candidate-feature/spec.md", candidateFeatureSpec)
	c := NewCompiler()
	operands, err := c.resolveConflictCandidate(context.Background(), repo.Dir, candidateRequestFor("spec/candidate-feature", "main", repo.Head), ConflictFacts{})
	if err != nil {
		t.Fatalf("resolveConflictCandidate: unexpected error: %v", err)
	}
	view, err := operands.View()
	if err != nil {
		t.Fatalf("View: unexpected error: %v", err)
	}
	if view.Snapshot.ManifestDigest != "" {
		t.Fatalf("ManifestDigest = %q, want always-empty for a candidate", view.Snapshot.ManifestDigest)
	}
}

// TestResolveConflictCandidateDiffersFromAcceptedBytes proves a proposed
// candidate whose bytes differ from an earlier committed baseline resolves
// distinct content digests, using the exact HEAD-tree bytes of the
// candidate commit rather than any earlier bytes.
func TestResolveConflictCandidateDiffersFromAcceptedBytes(t *testing.T) {
	files := policyStoreFiles(t)
	files[".verdi/specs/active/candidate-feature/spec.md"] = candidateFeatureSpec
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	baseHead := repo.Head

	// The base resolution MUST run while repo.Dir's actual checkout is
	// still at baseHead: resolveConflictCandidate verifies the caller's
	// declared expected branch/HEAD against computed repository facts
	// (authority design §2: "verifies the actual branch and HEAD"), the
	// same equality ResolveExpectedRepository already enforces for the
	// accepted-context arm — so it must run BEFORE writeFileAndCommit
	// advances this single mutable working directory's actual HEAD past
	// baseHead, not after.
	c := NewCompiler()
	base, err := c.resolveConflictCandidate(context.Background(), repo.Dir, candidateRequestFor("spec/candidate-feature", "main", baseHead), ConflictFacts{})
	if err != nil {
		t.Fatalf("resolveConflictCandidate(base): unexpected error: %v", err)
	}

	revised := strings.Replace(candidateFeatureSpec, "The candidate feature outcome.", "The REVISED candidate feature outcome.", -1)
	writeFileAndCommit(t, repo, ".verdi/specs/active/candidate-feature/spec.md", revised, "revise candidate")

	revisedOperands, err := c.resolveConflictCandidate(context.Background(), repo.Dir, candidateRequestFor("spec/candidate-feature", "main", repo.Head), ConflictFacts{})
	if err != nil {
		t.Fatalf("resolveConflictCandidate(revised): unexpected error: %v", err)
	}
	baseView, err := base.View()
	if err != nil {
		t.Fatalf("View(base): %v", err)
	}
	revisedView, err := revisedOperands.View()
	if err != nil {
		t.Fatalf("View(revised): %v", err)
	}
	if baseView.Snapshot.CandidateDigest == revisedView.Snapshot.CandidateDigest {
		t.Fatalf("CandidateDigest did not change between base %s and revised %s HEADs", baseHead, repo.Head)
	}
}

func writeFileAndCommit(t *testing.T, repo *fixturegit.Repo, relPath, content, message string) {
	t.Helper()
	dst := filepath.Join(repo.Dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", relPath, err)
	}
	if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
	env := append(os.Environ(),
		"TZ=UTC", "GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid",
		"GIT_AUTHOR_DATE=1704067201 +0000", "GIT_COMMITTER_NAME=Verdi Fixture",
		"GIT_COMMITTER_EMAIL=fixture@verdi.invalid", "GIT_COMMITTER_DATE=1704067201 +0000",
	)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo.Dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("add", "-A")
	run("commit", "--quiet", "--no-verify", "-m", message)
	out, err := exec.Command("git", "-C", repo.Dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	repo.Head = strings.TrimSpace(string(out))
}

// --- 7: symlink / missing / archive-only / declared-id-mismatch -----------

func TestResolveConflictCandidateFailureModes(t *testing.T) {
	root := installPolicyFixture(t)
	specPath := ".verdi/specs/active/candidate-feature/spec.md"

	cases := map[string]struct {
		entries []gitx.TreeEntry
		show    map[string][]byte
	}{
		"symlinked spec path": {
			entries: []gitx.TreeEntry{{Mode: "120000", Type: "blob", Object: strings.Repeat("a", 40), Path: specPath}},
			show:    map[string][]byte{specPath: []byte("../elsewhere")},
		},
		"missing spec blob": {
			entries: nil,
			show:    map[string][]byte{},
		},
		"archive-only location": {
			entries: []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: strings.Repeat("a", 40), Path: ".verdi/specs/archive/candidate-feature/spec.md"}},
			show:    map[string][]byte{".verdi/specs/archive/candidate-feature/spec.md": []byte(candidateFeatureSpec)},
		},
		"declared id mismatch": {
			entries: []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: strings.Repeat("a", 40), Path: specPath}},
			show:    map[string][]byte{specPath: []byte(strings.Replace(candidateFeatureSpec, "spec/candidate-feature", "spec/other-name", 1))},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			git := authorityGit{
				tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) { return tc.entries, nil },
				show: func(_ context.Context, _ string, _ string, path string) ([]byte, error) {
					data, ok := tc.show[path]
					if !ok {
						return nil, errors.New("contextcompile: unexpected Show path")
					}
					return data, nil
				},
			}
			gitWT := gitWithWorktree{GitReader: git, worktree: func(context.Context, string) ([]string, error) {
				panic("contextcompile: candidate resolution must never read worktree-changed paths")
			}}
			repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
			c := newCompilerWithPorts(gitWT, panicStateResolver{}, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
			_, err := c.resolveConflictCandidate(context.Background(), root, candidateRequestFor("spec/candidate-feature", "main", compileHead), ConflictFacts{})
			if err == nil {
				t.Fatalf("%s: expected an operational failure, got nil", name)
			}
		})
	}
}

// TestResolveConflictCandidateDirtyWorktreeCannotSubstitute proves the
// candidate path never calls WorktreeChangedPaths at all (a panicking fake
// would otherwise crash the test): worktree bytes can never substitute for
// the exact HEAD-tree blob because the code path that would read them is
// never reached.
func TestResolveConflictCandidateDirtyWorktreeCannotSubstitute(t *testing.T) {
	root := installPolicyFixture(t)
	specPath := ".verdi/specs/active/candidate-feature/spec.md"
	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			return []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: strings.Repeat("a", 40), Path: specPath}}, nil
		},
		show: func(context.Context, string, string, string) ([]byte, error) {
			return []byte(candidateFeatureSpec), nil
		},
	}
	gitWT := gitWithWorktree{GitReader: git, worktree: func(context.Context, string) ([]string, error) {
		panic("contextcompile: candidate resolution must never read worktree-changed paths")
	}}
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	c := newCompilerWithPorts(gitWT, panicStateResolver{}, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
	_, err := c.resolveConflictCandidate(context.Background(), root, candidateRequestFor("spec/candidate-feature", "main", compileHead), ConflictFacts{})
	if err != nil {
		t.Fatalf("resolveConflictCandidate: unexpected error: %v", err)
	}
}

// --- 8: zero-value Compiler / empty root / context cancellation -----------

func TestCompileConflictZeroValueCompilerFails(t *testing.T) {
	var c Compiler
	_, err := c.CompileConflict(context.Background(), "/repo", validCompileRequest("spec/example-story"), ConflictFacts{})
	if err == nil {
		t.Fatal("expected zero-value Compiler.CompileConflict to fail closed")
	}
}

func TestResolveConflictCandidateZeroValueCompilerFails(t *testing.T) {
	var c Compiler
	_, err := c.resolveConflictCandidate(context.Background(), "/repo", candidateRequestFor("spec/example", "main", compileHead), ConflictFacts{})
	if err == nil {
		t.Fatal("expected zero-value Compiler.resolveConflictCandidate to fail closed")
	}
}

func TestResolveConflictCandidateEmptyRootFails(t *testing.T) {
	c := NewCompiler()
	_, err := c.resolveConflictCandidate(context.Background(), "", candidateRequestFor("spec/example", "main", compileHead), ConflictFacts{})
	if err == nil {
		t.Fatal("expected an empty root to fail closed")
	}
}

func TestCompileConflictEmptyRootFails(t *testing.T) {
	c := NewCompiler()
	_, err := c.CompileConflict(context.Background(), "", validCompileRequest("spec/example-story"), ConflictFacts{})
	if err == nil {
		t.Fatal("expected an empty root to fail closed")
	}
}

// cancelledContextGit wraps a GitReader, returning ctx.Err() from every
// method once ctx is already done — used to prove context cancellation
// propagates rather than being silently swallowed.
type cancelledContextGit struct{ GitReader }

func (cancelledContextGit) Show(ctx context.Context, root, ref, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("contextcompile: unreachable")
}

func (cancelledContextGit) LsTreeEntries(ctx context.Context, root, ref string) ([]gitx.TreeEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("contextcompile: unreachable")
}

func TestResolveConflictCandidateContextCancellationPropagates(t *testing.T) {
	root := installPolicyFixture(t)
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	c := newCompilerWithPorts(cancelledContextGit{}, panicStateResolver{}, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.resolveConflictCandidate(ctx, root, candidateRequestFor("spec/example", "main", compileHead), ConflictFacts{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("resolveConflictCandidate: err = %v, want wrapped context.Canceled", err)
	}
}

func TestCompileConflictContextCancellationPropagates(t *testing.T) {
	root := installPolicyFixture(t)
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	c := newCompilerWithPorts(cancelledContextGit{}, panicStateResolver{}, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := validCompileRequest("spec/example-story")
	_, err := c.CompileConflict(ctx, root, req, ConflictFacts{})
	if err == nil {
		t.Fatal("expected context cancellation to fail CompileConflict")
	}
}

// --- helpers ---------------------------------------------------------------

// conflictViewsEqual compares two ConflictView values by canonical digest,
// which — like every other sealed value in this store — ignores the
// unexported seal fields nested inside EffectivePolicy/Profile/
// PrincipalResolution identically on both sides.
func conflictViewsEqual(a, b ConflictView) bool {
	da, errA := canonjson.Digest(a)
	db, errB := canonjson.Digest(b)
	if errA != nil || errB != nil {
		return false
	}
	return da == db
}
