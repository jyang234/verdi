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
	"github.com/jyang234/verdi/internal/specstate"
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

	revised := strings.ReplaceAll(candidateFeatureSpec, "The candidate feature outcome.", "The REVISED candidate feature outcome.")
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

// TestResolveConflictCandidateRevisesAcceptedBaseline is the companion
// TestResolveConflictCandidateDiffersFromAcceptedBytes cannot be: that test
// compares two candidate resolutions over two HEAD blobs and never
// establishes an ACCEPTED baseline at all, while plan Task 4 Step 1 names
// "a proposed candidate differing from default-branch accepted bytes".
// Here the baseline spec is genuinely accepted on the default branch (the
// accepted-arm fixture's own state resolution, resolved through
// CompileConflict, which mints a manifest identity), and the candidate then
// resolves over REVISED bytes of that same spec: the candidate identity is
// the revised bytes' digest, no manifest identity is ever minted, and the
// two digests differ.
func TestResolveConflictCandidateRevisesAcceptedBaseline(t *testing.T) {
	acceptedCompiler, req := hermeticAcceptedFixture(t, nil, nil, nil)
	root := hermeticAcceptedRoot(t)
	accepted, err := acceptedCompiler.CompileConflict(context.Background(), root, req, ConflictFacts{})
	if err != nil {
		t.Fatalf("CompileConflict(accepted baseline): unexpected error: %v", err)
	}
	acceptedView, err := accepted.View()
	if err != nil {
		t.Fatalf("View(accepted baseline): %v", err)
	}
	if acceptedView.Snapshot.ManifestDigest == "" {
		t.Fatal("the baseline is not an accepted context: it minted no manifest identity")
	}
	repo := acceptedView.Snapshot.Repository
	if repo.Branch.Value != repo.DefaultBranch.Name {
		t.Fatalf("the baseline was not resolved on the default branch: branch=%q default=%q", repo.Branch.Value, repo.DefaultBranch.Name)
	}
	var baselineDigest string
	for _, s := range acceptedView.Snapshot.Sources {
		if s.Ref == req.Spec {
			baselineDigest = s.ContentDigest
		}
	}
	if baselineDigest == "" {
		t.Fatalf("accepted Sources carries no entry for the baseline target %s: %+v", req.Spec, acceptedView.Snapshot.Sources)
	}

	// The proposed candidate: the identical spec ref, revised bytes.
	storyData, _ := decodeFragmentSpecFixture(t, "story-multi-parent.md")
	storyPath := ".verdi/specs/active/" + strings.TrimPrefix(req.Spec, "spec/") + "/spec.md"
	revised := strings.ReplaceAll(string(storyData), "The story joins both features.", "The story joins both features, revised.")
	if revised == string(storyData) {
		t.Fatal("fixture patch did not apply: story-multi-parent.md no longer carries its outcome text")
	}
	if rawContentDigest([]byte(revised)) == baselineDigest {
		t.Fatal("the revised candidate bytes are identical to the accepted baseline's")
	}

	git, states, _ := compilerAcceptedFixture(t)
	overlay := conflictOverlayGit{
		GitReader: policyDiskFallbackGit{GitReader: git},
		files:     map[string][]byte{storyPath: []byte(revised)},
	}
	gitWT := gitWithWorktree{GitReader: overlay, worktree: func(context.Context, string) ([]string, error) {
		panic("contextcompile: candidate resolution must never read worktree-changed paths")
	}}
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	candidateCompiler := newCompilerWithPorts(gitWT, states, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})

	candidate, err := candidateCompiler.resolveConflictCandidate(context.Background(), root, candidateRequestFor(req.Spec, "main", compileHead), ConflictFacts{})
	if err != nil {
		t.Fatalf("resolveConflictCandidate(revised over accepted baseline): unexpected error: %v", err)
	}
	candidateView, err := candidate.View()
	if err != nil {
		t.Fatalf("View(candidate): %v", err)
	}
	if want := rawContentDigest([]byte(revised)); candidateView.Snapshot.CandidateDigest != want {
		t.Errorf("CandidateDigest = %q, want the revised bytes' digest %q", candidateView.Snapshot.CandidateDigest, want)
	}
	if candidateView.Snapshot.ManifestDigest != "" {
		t.Errorf("ManifestDigest = %q, want always-empty for a candidate", candidateView.Snapshot.ManifestDigest)
	}
	if candidateView.Snapshot.CandidateDigest == baselineDigest {
		t.Errorf("CandidateDigest equals the accepted baseline's content digest %q", baselineDigest)
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

// --- 11: story-class acceptance candidates --------------------------------

// recordingStateResolver wraps a StateResolver, recording every candidate
// path whose state was actually resolved (and optionally overriding the
// result), so a test can prove WHICH artifacts a candidate resolution
// independently proved accepted.
type recordingStateResolver struct {
	inner  StateResolver
	paths  *[]string
	result func(path string) (specstate.Result, bool)
}

func (r recordingStateResolver) Resolve(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error) {
	if r.paths != nil {
		*r.paths = append(*r.paths, candidate.Path)
	}
	if r.result != nil {
		if res, ok := r.result(candidate.Path); ok {
			return res, nil
		}
	}
	return r.inner.Resolve(ctx, root, candidate)
}

// storyCandidateFixture wires a Compiler for a STORY-class acceptance
// candidate over compilerAcceptedFixture's fake tree (the multi-parent
// story plus both governing parent features), recording every state
// resolution and applying override to it. It returns the compiler, the
// candidate's own ref, the root, and the recorded-path slice pointer.
func storyCandidateFixture(t *testing.T, override func(path string) (specstate.Result, bool)) (Compiler, string, string, *[]string) {
	t.Helper()
	root := installPolicyFixture(t)
	git, states, ref := compilerAcceptedFixture(t)
	resolved := &[]string{}
	recorder := recordingStateResolver{inner: states, paths: resolved, result: override}
	gitWT := gitWithWorktree{GitReader: policyDiskFallbackGit{GitReader: git}, worktree: func(context.Context, string) ([]string, error) {
		panic("contextcompile: candidate resolution must never read worktree-changed paths")
	}}
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	c := newCompilerWithPorts(gitWT, recorder, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
	return c, ref, root, resolved
}

// TestResolveConflictCandidateStoryClassResolvesFragments proves the
// story-class candidate arm: fragments resolve from both governing
// parents, each parent's acceptance is independently proven through the
// state resolver, and the candidate's OWN acceptance never is — the
// synthetic specstate.AcceptedPendingBuild the arm places on its target
// only satisfies ResolveFeatureFragments' internal target gate.
func TestResolveConflictCandidateStoryClassResolvesFragments(t *testing.T) {
	c, ref, root, resolved := storyCandidateFixture(t, nil)
	operands, err := c.resolveConflictCandidate(context.Background(), root, candidateRequestFor(ref, "main", compileHead), ConflictFacts{})
	if err != nil {
		t.Fatalf("resolveConflictCandidate(story class): unexpected error: %v", err)
	}
	view, err := operands.View()
	if err != nil {
		t.Fatalf("View: unexpected error: %v", err)
	}
	if view.Snapshot.TargetKind != snapshotTargetAcceptanceCandidate || view.Snapshot.ManifestDigest != "" || view.Snapshot.CandidateDigest == "" {
		t.Errorf("snapshot identity is not a candidate identity: %+v", view.Snapshot)
	}

	storyPath := ".verdi/specs/active/" + strings.TrimPrefix(ref, "spec/") + "/spec.md"
	parents := map[string]string{
		"spec/feature-alpha": ".verdi/specs/active/feature-alpha/spec.md",
		"spec/feature-beta":  ".verdi/specs/active/feature-beta/spec.md",
	}
	for parentRef, parentPath := range parents {
		found := false
		for _, s := range view.Snapshot.Sources {
			if s.Ref == parentRef && s.Path == parentPath {
				found = true
			}
		}
		if !found {
			t.Errorf("Sources missing governing parent %s: %+v", parentRef, view.Snapshot.Sources)
		}
		fragmentProse := false
		for _, pc := range view.ProseClaims {
			if pc.SourceRef == parentRef && pc.Category == categorySpecProblem {
				fragmentProse = true
			}
		}
		if !fragmentProse {
			t.Errorf("ProseClaims missing the governing parent %s's own fragment prose", parentRef)
		}
		if !slicesContains(*resolved, parentPath) {
			t.Errorf("governing parent %s was never independently state-resolved; resolved paths = %v", parentRef, *resolved)
		}
	}
	if slicesContains(*resolved, storyPath) {
		t.Errorf("the candidate's OWN acceptance was state-resolved (%s); a proposed candidate is by definition not yet accepted", storyPath)
	}
}

// TestResolveConflictCandidateStoryClassUnacceptedParentFails proves the
// governing-parent gate is real: a story-class candidate whose parent
// feature is not accepted fails, and the failure names that parent.
func TestResolveConflictCandidateStoryClassUnacceptedParentFails(t *testing.T) {
	const unacceptedParent = ".verdi/specs/active/feature-beta/spec.md"
	c, ref, root, _ := storyCandidateFixture(t, func(path string) (specstate.Result, bool) {
		if path != unacceptedParent {
			return specstate.Result{}, false
		}
		return specstate.Result{State: specstate.Proposed, Relation: specstate.RelationUnproven}, true
	})

	_, err := c.resolveConflictCandidate(context.Background(), root, candidateRequestFor(ref, "main", compileHead), ConflictFacts{})
	if err == nil {
		t.Fatal("resolveConflictCandidate accepted a story-class candidate whose governing parent is not accepted")
	}
	var refusal *AcceptedSpecRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want a wrapped AcceptedSpecRefusal naming the unaccepted parent", err)
	}
	if refusal.Ref != "spec/feature-beta" {
		t.Errorf("refusal names %q, want the unaccepted governing parent spec/feature-beta", refusal.Ref)
	}
}

// slicesContains reports whether haystack contains needle.
func slicesContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// --- 10: the two identity digests are mutually exclusive by target kind ---

// TestConflictOperandsSnapshotIdentityDigestXor pins the plan's identity
// rule: accepted requires ManifestDigest and an empty CandidateDigest,
// acceptance-candidate is exactly the reverse, and an unknown target kind
// is never assembled at all. Each violation is operational — a snapshot
// that claimed both identities (or neither) would let a report be derived
// against a target the operands were not resolved from.
func TestConflictOperandsSnapshotIdentityDigestXor(t *testing.T) {
	base := func(kind, manifest, candidate string) snapshotBuildInput {
		return snapshotBuildInput{
			targetKind:      kind,
			repository:      validRepositorySnapshot(compileHead, "main").Facts,
			manifestDigest:  manifest,
			candidateDigest: candidate,
			authority:       PolicyAuthority{Effective: &policyauthority.EffectivePolicy{}},
		}
	}
	digestA := "sha256:" + strings.Repeat("1", 64)
	digestB := "sha256:" + strings.Repeat("2", 64)

	cases := map[string]snapshotBuildInput{
		"accepted without a manifest digest":   base(snapshotTargetAcceptedContext, "", ""),
		"accepted carrying both digests":       base(snapshotTargetAcceptedContext, digestA, digestB),
		"accepted carrying a candidate digest": base(snapshotTargetAcceptedContext, "", digestB),
		"candidate without a candidate digest": base(snapshotTargetAcceptanceCandidate, "", ""),
		"candidate carrying both digests":      base(snapshotTargetAcceptanceCandidate, digestA, digestB),
		"candidate carrying a manifest digest": base(snapshotTargetAcceptanceCandidate, digestA, ""),
		"unknown target kind":                  base("other-kind", digestA, ""),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := buildSnapshotIdentity(in)
			if err == nil {
				t.Fatalf("buildSnapshotIdentity(%s) = nil error, want an operational failure", name)
			}
			// The diagnostic must name the identity rule itself. These
			// inputs are otherwise incomplete (no resolved store, no
			// target), so a bare non-nil error would pass vacuously on a
			// later stage's complaint without the rule ever being checked.
			if !strings.Contains(err.Error(), "identity digest") {
				t.Fatalf("buildSnapshotIdentity(%s) failed for an unrelated reason, not the identity-digest rule: %v", name, err)
			}
		})
	}
}

// --- 9: normalized authored prose (adr-decision, obligation-declaration) --

// conflictADRFrontmatter is the exact frontmatter block the fixture ADR
// carries. Every value here is FRONTMATTER, never authored decision prose:
// an adr-decision ProseClaim's normalized text must contain none of it
// (authority design §6: normalization "trims only structural frontmatter/
// body delimiters").
const conflictADRFrontmatter = `---
id: adr/alpha-base
kind: adr
title: "Alpha base decision"
status: accepted
owners: [platform-team]
decided: 2026-04-01
frozen: { at: 2026-04-01, commit: ffffffffffffffffffffffffffffffffffffffff }
---
`

// conflictADRNormalized is the exact normalized authored Markdown body the
// fixture ADR must yield: the structural delimiter line and the blank line
// that follows it are trimmed, and NOTHING else about the authored text
// changes (no case folding, rewriting, summarizing, or reordering).
const conflictADRNormalized = `# Alpha base decision

The alpha transport is pinned to the toolchain the policy store adopts.

## Consequences

Downstream consumers pin the identical toolchain.`

// conflictADRDoc is the whole authored ADR artifact: frontmatter, a blank
// structural line, the authored body, and a trailing newline.
const conflictADRDoc = conflictADRFrontmatter + "\n" + conflictADRNormalized + "\n"

// conflictObligationNormalized is the exact normalized authored body of the
// fixture obligation, and conflictObligationDoc the whole artifact.
const conflictObligationNormalized = `# Behavioral obligation for ac-1

An end-to-end run must exercise both parents' joined path.`

const conflictObligationDoc = `---
id: obligation/story-multi-parent--ac-1--behavioral
kind: obligation
title: "Behavioral obligation for ac-1"
owners: [story-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/story-multi-parent" }
frozen: { at: 2026-04-01, commit: ffffffffffffffffffffffffffffffffffffffff }
---
` + "\n" + conflictObligationNormalized + "\n"

const (
	conflictADRPath        = ".verdi/adr/alpha-base.md"
	conflictObligationPath = ".verdi/obligations/story-multi-parent/ac-1--behavioral.md"
	conflictAlphaPath      = ".verdi/specs/active/feature-alpha/spec.md"
)

// conflictADRPinnedRef is the exact pinned declared-context ref the fixture
// parent feature declares (SI-92 grammar: kind/name@commit).
const conflictADRPinnedRef = "adr/alpha-base@" + compileHead

// conflictOverlayGit overlays extra HEAD-tree entries and exact file bytes
// onto a wrapped GitReader, so a test can add declared-context and
// obligation artifacts to compilerAcceptedFixture's fake tree without
// touching any committed fixture file.
type conflictOverlayGit struct {
	GitReader
	files map[string][]byte
	extra []gitx.TreeEntry
}

func (g conflictOverlayGit) Show(ctx context.Context, root, ref, path string) ([]byte, error) {
	if data, ok := g.files[path]; ok {
		return append([]byte(nil), data...), nil
	}
	return g.GitReader.Show(ctx, root, ref, path)
}

func (g conflictOverlayGit) LsTreeEntries(ctx context.Context, root, ref string) ([]gitx.TreeEntry, error) {
	entries, err := g.GitReader.LsTreeEntries(ctx, root, ref)
	if err != nil {
		return nil, err
	}
	return append(append([]gitx.TreeEntry(nil), entries...), g.extra...), nil
}

// candidateFeatureDeclaredContextFixture wires a feature-class candidate
// whose own context: list names rawRef. When includeADR is true the exact
// pinned tree also contains adrDoc at the ADR's fixed store path; otherwise
// the ref is intentionally unresolvable. The real candidate resolver sees
// only exact Git-object reads and the real strict authority decoder.
func candidateFeatureDeclaredContextFixture(t *testing.T, rawRef, adrDoc string, includeADR bool) (Compiler, CandidateRequest, string) {
	t.Helper()
	root := installPolicyFixture(t)
	candidatePath := ".verdi/specs/active/candidate-feature/spec.md"
	candidateSpec := strings.Replace(candidateFeatureSpec, "class: feature\n",
		"class: feature\ncontext: [\""+rawRef+"\"]\n", 1)
	if candidateSpec == candidateFeatureSpec {
		t.Fatal("fixture patch did not apply: candidateFeatureSpec no longer carries a `class: feature` line")
	}

	files := map[string][]byte{candidatePath: []byte(candidateSpec)}
	entries := []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: strings.Repeat("a", 40), Path: candidatePath}}
	if includeADR {
		files[conflictADRPath] = []byte(adrDoc)
		entries = append(entries, gitx.TreeEntry{Mode: "100644", Type: "blob", Object: strings.Repeat("e", 40), Path: conflictADRPath})
	}

	git := policyDiskFallbackGit{GitReader: authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			return append([]gitx.TreeEntry(nil), entries...), nil
		},
		show: func(_ context.Context, _ string, _ string, path string) ([]byte, error) {
			data, ok := files[path]
			if !ok {
				return nil, errors.New("contextcompile: unexpected candidate declared-context path")
			}
			return append([]byte(nil), data...), nil
		},
	}}
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	c := newCompilerWithPorts(git, panicStateResolver{}, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
	return c, candidateRequestFor("spec/candidate-feature", "main", compileHead), root
}

// storyCandidateDeclaredContextFixture wires a story-class candidate whose
// governing feature-alpha parent declares the exact pinned fixture ADR. The
// story has no context: field of its own, so a successful claim proves the
// SI-91 parent-union path rather than the feature-target path.
func storyCandidateDeclaredContextFixture(t *testing.T) (Compiler, CandidateRequest, string) {
	t.Helper()
	root := installPolicyFixture(t)
	git, states, ref := compilerAcceptedFixture(t)
	alphaData, _ := decodeFragmentSpecFixture(t, "feature-alpha.md")
	declaring := strings.Replace(string(alphaData), "class: feature\n",
		"class: feature\ncontext: [\""+conflictADRPinnedRef+"\"]\n", 1)
	if declaring == string(alphaData) {
		t.Fatal("fixture patch did not apply: feature-alpha.md no longer carries a `class: feature` line")
	}

	overlay := conflictOverlayGit{
		GitReader: policyDiskFallbackGit{GitReader: git},
		files: map[string][]byte{
			conflictAlphaPath: []byte(declaring),
			conflictADRPath:   []byte(conflictADRDoc),
		},
		extra: []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: strings.Repeat("e", 40), Path: conflictADRPath}},
	}
	gitWT := gitWithWorktree{GitReader: overlay, worktree: func(context.Context, string) ([]string, error) {
		panic("contextcompile: candidate resolution must never read worktree-changed paths")
	}}
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	c := newCompilerWithPorts(gitWT, states, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
	return c, candidateRequestFor(ref, "main", compileHead), root
}

// assertCandidateADRDecisionProse proves the complete candidate-visible ADR
// behavior: normalized prose and digest, exact raw source digest and pinned
// ref/path, source-set membership, and preservation of candidate identity.
func assertCandidateADRDecisionProse(t *testing.T, view ConflictView) {
	t.Helper()
	if view.Snapshot.TargetKind != snapshotTargetAcceptanceCandidate || view.Snapshot.ManifestDigest != "" || view.Snapshot.CandidateDigest == "" {
		t.Errorf("snapshot identity is not exclusively a candidate identity: %+v", view.Snapshot)
	}
	claims := conflictProseClaimsByCategory(view, categoryADRDecision)
	if len(claims) != 1 {
		t.Fatalf("adr-decision ProseClaims = %d, want exactly 1: %+v", len(claims), claims)
	}
	claim := claims[0]
	if claim.Text != conflictADRNormalized {
		t.Errorf("adr-decision Text =\n%q\nwant\n%q", claim.Text, conflictADRNormalized)
	}
	if want := rawContentDigest([]byte(conflictADRNormalized)); claim.TextDigest != want {
		t.Errorf("adr-decision TextDigest = %q, want normalized-text digest %q", claim.TextDigest, want)
	}
	rawDigest := rawContentDigest([]byte(conflictADRDoc))
	if claim.SourceRef != conflictADRPinnedRef || claim.SourcePath != conflictADRPath || claim.SourceDigest != rawDigest {
		t.Errorf("adr-decision source = ref %q path %q digest %q, want exact pinned source %q %q %q",
			claim.SourceRef, claim.SourcePath, claim.SourceDigest, conflictADRPinnedRef, conflictADRPath, rawDigest)
	}
	wantSource := ConflictSourceIdentity{Ref: conflictADRPinnedRef, Path: conflictADRPath, ContentDigest: rawDigest}
	for _, source := range view.Snapshot.Sources {
		if source == wantSource {
			return
		}
	}
	t.Errorf("Snapshot.Sources is missing the declared ADR %+v: %+v", wantSource, view.Snapshot.Sources)
}

// TestResolveConflictCandidateFeatureDeclaredContextADRDecisionProse catches
// candidate feature construction dropping its own exact pinned context: list.
func TestResolveConflictCandidateFeatureDeclaredContextADRDecisionProse(t *testing.T) {
	c, req, root := candidateFeatureDeclaredContextFixture(t, conflictADRPinnedRef, conflictADRDoc, true)
	operands, err := c.resolveConflictCandidate(context.Background(), root, req, ConflictFacts{})
	if err != nil {
		t.Fatalf("resolveConflictCandidate(feature declared context): unexpected error: %v", err)
	}
	view, err := operands.View()
	if err != nil {
		t.Fatalf("View: unexpected error: %v", err)
	}
	assertCandidateADRDecisionProse(t, view)
}

// TestResolveConflictCandidateStoryDeclaredContextADRDecisionProse catches
// candidate story construction dropping the declared-context union inherited
// from its accepted governing feature parents.
func TestResolveConflictCandidateStoryDeclaredContextADRDecisionProse(t *testing.T) {
	c, req, root := storyCandidateDeclaredContextFixture(t)
	operands, err := c.resolveConflictCandidate(context.Background(), root, req, ConflictFacts{})
	if err != nil {
		t.Fatalf("resolveConflictCandidate(story declared context): unexpected error: %v", err)
	}
	view, err := operands.View()
	if err != nil {
		t.Fatalf("View: unexpected error: %v", err)
	}
	assertCandidateADRDecisionProse(t, view)
}

// TestResolveConflictCandidateDeclaredContextFailsOperationally catches a
// candidate arm silently omitting invalid authority. Malformed exact refs,
// unresolved pinned artifacts, and strict-decode failures are operational,
// never state refusals or favorable empty claim sets.
func TestResolveConflictCandidateDeclaredContextFailsOperationally(t *testing.T) {
	malformedADR := strings.Replace(conflictADRDoc, "status: accepted\n", "status: accepted\nunknown: true\n", 1)
	tests := []struct {
		name       string
		ref        string
		adrDoc     string
		includeADR bool
		wantStage  string
	}{
		{name: "malformed unpinned ref", ref: "adr/alpha-base", wantStage: "decode candidate spec"},
		{name: "unresolvable pinned artifact", ref: "adr/missing@" + compileHead, wantStage: "resolve declared context"},
		{name: "malformed pinned artifact", ref: conflictADRPinnedRef, adrDoc: malformedADR, includeADR: true, wantStage: "resolve declared context"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, req, root := candidateFeatureDeclaredContextFixture(t, tt.ref, tt.adrDoc, tt.includeADR)
			_, err := c.resolveConflictCandidate(context.Background(), root, req, ConflictFacts{})
			if err == nil {
				t.Fatal("resolveConflictCandidate accepted malformed or unresolvable declared context")
			}
			if IsRefusal(err) {
				t.Fatalf("declared-context authority error was classified as a state refusal: %T %v", err, err)
			}
			if !strings.Contains(err.Error(), tt.wantStage) {
				t.Fatalf("error did not come from the expected %q stage: %v", tt.wantStage, err)
			}
		})
	}
}

// hermeticAcceptedProseFixture is hermeticAcceptedFixture plus authored
// prose authority: the governing parent feature declares one exact pinned
// ADR as declared context, that ADR exists at its fixed store path with
// adrDoc's bytes, and the story's ac-1 behavioral obligation exists at its
// fixed path. It returns the wired Compiler, its Request, and the root.
func hermeticAcceptedProseFixture(t *testing.T, adrDoc string) (Compiler, Request, string) {
	t.Helper()
	root := installPolicyFixture(t)
	git, states, ref := compilerAcceptedFixture(t)

	alphaData, _ := decodeFragmentSpecFixture(t, "feature-alpha.md")
	declaring := strings.Replace(string(alphaData), "class: feature\n",
		"class: feature\ncontext: [\""+conflictADRPinnedRef+"\"]\n", 1)
	if declaring == string(alphaData) {
		t.Fatal("fixture patch did not apply: feature-alpha.md no longer carries a `class: feature` line")
	}

	overlay := conflictOverlayGit{
		GitReader: policyDiskFallbackGit{GitReader: git},
		files: map[string][]byte{
			conflictAlphaPath:      []byte(declaring),
			conflictADRPath:        []byte(adrDoc),
			conflictObligationPath: []byte(conflictObligationDoc),
		},
		extra: []gitx.TreeEntry{
			{Mode: "100644", Type: "blob", Object: strings.Repeat("e", 40), Path: conflictADRPath},
			{Mode: "100644", Type: "blob", Object: strings.Repeat("f", 40), Path: conflictObligationPath},
		},
	}
	gitWT := gitWithWorktree{GitReader: overlay, worktree: func(context.Context, string) ([]string, error) { return nil, nil }}
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	projection := stubProjectionVerifier{report: &instructionprojection.Report{}}

	c := newCompilerWithPorts(gitWT, states, defaultAuthorityLoader{}, nil, repoFacts, projection)
	return c, validCompileRequest(ref), root
}

// conflictProseClaimsByCategory returns view's prose claims of one category.
func conflictProseClaimsByCategory(view ConflictView, category string) []ProseClaim {
	var out []ProseClaim
	for _, pc := range view.ProseClaims {
		if pc.Category == category {
			out = append(out, pc)
		}
	}
	return out
}

// TestCompileConflictOperandsAcceptedADRDecisionProse proves the accepted
// arm emits exactly one adr-decision ProseClaim per declared-context ADR,
// whose text is the normalized authored body with no frontmatter value in
// it, whose source digest remains the digest of the EXACT RAW pinned bytes,
// and whose artifact appears in Snapshot.Sources.
func TestCompileConflictOperandsAcceptedADRDecisionProse(t *testing.T) {
	c, req, root := hermeticAcceptedProseFixture(t, conflictADRDoc)
	operands, err := c.CompileConflict(context.Background(), root, req, ConflictFacts{})
	if err != nil {
		t.Fatalf("CompileConflict: unexpected error: %v", err)
	}
	view, err := operands.View()
	if err != nil {
		t.Fatalf("View: unexpected error: %v", err)
	}

	claims := conflictProseClaimsByCategory(view, "adr-decision")
	if len(claims) != 1 {
		t.Fatalf("adr-decision ProseClaims = %d, want exactly 1: %+v", len(claims), claims)
	}
	claim := claims[0]

	if claim.Text != conflictADRNormalized {
		t.Errorf("adr-decision Text =\n%q\nwant\n%q", claim.Text, conflictADRNormalized)
	}
	if want := rawContentDigest([]byte(conflictADRNormalized)); claim.TextDigest != want {
		t.Errorf("adr-decision TextDigest = %q, want %q (the digest of the normalized text)", claim.TextDigest, want)
	}
	for _, frontmatterOnly := range []string{"kind: adr", "platform-team", "2026-04-01", "status:", "frozen:"} {
		if strings.Contains(claim.Text, frontmatterOnly) {
			t.Errorf("adr-decision Text leaked the frontmatter value %q:\n%s", frontmatterOnly, claim.Text)
		}
	}

	rawDigest := rawContentDigest([]byte(conflictADRDoc))
	if claim.SourceDigest != rawDigest {
		t.Errorf("adr-decision SourceDigest = %q, want %q (normalization never changes a source digest)", claim.SourceDigest, rawDigest)
	}
	if claim.SourcePath != conflictADRPath {
		t.Errorf("adr-decision SourcePath = %q, want %q", claim.SourcePath, conflictADRPath)
	}
	if claim.SourceRef != conflictADRPinnedRef {
		t.Errorf("adr-decision SourceRef = %q, want the exact pinned declared-context ref %q", claim.SourceRef, conflictADRPinnedRef)
	}
	if claim.AuthorityDigest == "" {
		t.Error("adr-decision AuthorityDigest is empty, want the effective-policy digest")
	}
	if claim.ID != claim.LineIdentity || claim.ID != claim.SourceRef+"#"+claim.Object {
		t.Errorf("adr-decision identity is inconsistent with its neighbors: id=%q object=%q line=%q", claim.ID, claim.Object, claim.LineIdentity)
	}
	if len(claim.Scope.Refs) != 1 || claim.Scope.Refs[0] != claim.LineIdentity {
		t.Errorf("adr-decision Scope.Refs = %v, want inheritance narrowed to its own ref %q", claim.Scope.Refs, claim.LineIdentity)
	}

	want := ConflictSourceIdentity{Ref: conflictADRPinnedRef, Path: conflictADRPath, ContentDigest: rawDigest}
	found := false
	for _, s := range view.Snapshot.Sources {
		if s == want {
			found = true
		}
	}
	if !found {
		t.Errorf("Snapshot.Sources is missing the declared ADR %+v: %+v", want, view.Snapshot.Sources)
	}
}

// TestCompileConflictOperandsAcceptedObligationProseExcludesFrontmatter
// proves the pre-existing obligation-declaration claims run through the
// same normalization: a whole-artifact trim would leak the obligation's
// frontmatter into Text/TextDigest.
func TestCompileConflictOperandsAcceptedObligationProseExcludesFrontmatter(t *testing.T) {
	c, req, root := hermeticAcceptedProseFixture(t, conflictADRDoc)
	operands, err := c.CompileConflict(context.Background(), root, req, ConflictFacts{})
	if err != nil {
		t.Fatalf("CompileConflict: unexpected error: %v", err)
	}
	view, err := operands.View()
	if err != nil {
		t.Fatalf("View: unexpected error: %v", err)
	}

	claims := conflictProseClaimsByCategory(view, categoryObligationDeclaration)
	if len(claims) != 1 {
		t.Fatalf("obligation-declaration ProseClaims = %d, want exactly 1: %+v", len(claims), claims)
	}
	claim := claims[0]
	if claim.Text != conflictObligationNormalized {
		t.Errorf("obligation-declaration Text =\n%q\nwant\n%q", claim.Text, conflictObligationNormalized)
	}
	if want := rawContentDigest([]byte(conflictObligationNormalized)); claim.TextDigest != want {
		t.Errorf("obligation-declaration TextDigest = %q, want %q", claim.TextDigest, want)
	}
	for _, frontmatterOnly := range []string{"for_kind", "kind: obligation", "verifies", "frozen:"} {
		if strings.Contains(claim.Text, frontmatterOnly) {
			t.Errorf("obligation-declaration Text leaked the frontmatter value %q:\n%s", frontmatterOnly, claim.Text)
		}
	}
	if want := rawContentDigest([]byte(conflictObligationDoc)); claim.SourceDigest != want {
		t.Errorf("obligation-declaration SourceDigest = %q, want the raw artifact digest %q", claim.SourceDigest, want)
	}
}

// TestCompileConflictOperandsAcceptedADRProseNormalizesCRLF proves CRLF
// authored bytes normalize to LF text (authority design §6) — the claim is
// byte-identical to the LF fixture's, while the source digest is the CRLF
// artifact's own raw digest.
func TestCompileConflictOperandsAcceptedADRProseNormalizesCRLF(t *testing.T) {
	crlf := strings.ReplaceAll(conflictADRDoc, "\n", "\r\n")
	c, req, root := hermeticAcceptedProseFixture(t, crlf)
	operands, err := c.CompileConflict(context.Background(), root, req, ConflictFacts{})
	if err != nil {
		t.Fatalf("CompileConflict: unexpected error: %v", err)
	}
	view, err := operands.View()
	if err != nil {
		t.Fatalf("View: unexpected error: %v", err)
	}
	claims := conflictProseClaimsByCategory(view, "adr-decision")
	if len(claims) != 1 {
		t.Fatalf("adr-decision ProseClaims = %d, want exactly 1: %+v", len(claims), claims)
	}
	if strings.Contains(claims[0].Text, "\r") {
		t.Errorf("adr-decision Text retained a CR: %q", claims[0].Text)
	}
	if claims[0].Text != conflictADRNormalized {
		t.Errorf("adr-decision Text =\n%q\nwant the identical LF normalization\n%q", claims[0].Text, conflictADRNormalized)
	}
	if want := rawContentDigest([]byte(crlf)); claims[0].SourceDigest != want {
		t.Errorf("adr-decision SourceDigest = %q, want the CRLF artifact's own raw digest %q", claims[0].SourceDigest, want)
	}
}

// TestCompileConflictOperandsAcceptedADRInvalidUTF8FailsClosed proves
// invalid-UTF-8 authored bytes never produce a claim. The refusal happens
// upstream, in ResolveDeclaredContext's own invalid-authority check, before
// this file's normalization is reached; this test pins that the compile as
// a whole still fails operationally rather than silently skipping the ADR.
// normalizeAuthorityProse's own independent UTF-8 refusal is proven
// directly by TestConflictOperandsNormalizeAuthorityProse.
func TestCompileConflictOperandsAcceptedADRInvalidUTF8FailsClosed(t *testing.T) {
	invalid := conflictADRFrontmatter + "\n# Alpha base decision\n\n\xff\xfe not utf-8\n"
	c, req, root := hermeticAcceptedProseFixture(t, invalid)
	if _, err := c.CompileConflict(context.Background(), root, req, ConflictFacts{}); err == nil {
		t.Fatal("CompileConflict accepted an invalid-UTF-8 declared ADR, want an operational failure")
	}
}

// TestConflictOperandsNormalizeAuthorityProse pins the shared §6
// normalization contract directly, on both arms: what it preserves and
// every input it refuses operationally.
func TestConflictOperandsNormalizeAuthorityProse(t *testing.T) {
	t.Run("preserves authored text exactly", func(t *testing.T) {
		cases := map[string]struct {
			raw  string
			want string
		}{
			"trims the structural delimiters only": {
				raw:  "---\nid: adr/x\n---\n\n# Title\n\nBody.\n",
				want: "# Title\n\nBody.",
			},
			"converts CRLF to LF": {
				raw:  "---\r\nid: adr/x\r\n---\r\n\r\n# Title\r\n\r\nBody.\r\n",
				want: "# Title\n\nBody.",
			},
			"never case-folds or reorders": {
				raw:  "---\nid: adr/x\n---\n\nZebra THEN Apple.\n",
				want: "Zebra THEN Apple.",
			},
			"keeps authored indentation and interior blank lines": {
				raw:  "---\nid: adr/x\n---\n\n    indented\n\n\nlater\n",
				want: "    indented\n\n\nlater",
			},
		}
		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				got, err := normalizeAuthorityProse("adr/x", []byte(tc.raw))
				if err != nil {
					t.Fatalf("normalizeAuthorityProse: unexpected error: %v", err)
				}
				if got != tc.want {
					t.Errorf("normalizeAuthorityProse = %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("refuses operationally", func(t *testing.T) {
		cases := map[string]string{
			"invalid UTF-8":            "---\nid: adr/x\n---\n\n\xff\xfe\n",
			"no frontmatter block":     "# Title\n\nBody.\n",
			"unterminated frontmatter": "---\nid: adr/x\n\n# Title\n",
			"blank authored body":      "---\nid: adr/x\n---\n\n   \n\n",
			"empty artifact":           "",
		}
		for name, raw := range cases {
			t.Run(name, func(t *testing.T) {
				got, err := normalizeAuthorityProse("adr/x", []byte(raw))
				if err == nil {
					t.Fatalf("normalizeAuthorityProse = %q, want an operational failure", got)
				}
				if !strings.Contains(err.Error(), "adr/x") {
					t.Errorf("error %q does not name the artifact it refused", err)
				}
			})
		}
	})
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
