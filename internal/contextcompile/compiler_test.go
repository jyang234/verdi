package contextcompile

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/repositoryfacts"
	"github.com/jyang234/verdi/internal/specstate"
)

// TestCompilerZeroValueFailsClosed is the RED witness for the Compile seam.
// Compiler, NewCompiler, and (Compiler).Compile do not exist yet: this file
// intentionally fails to compile until the seam is implemented.
func TestCompilerZeroValueFailsClosed(t *testing.T) {
	var c Compiler
	_, err := c.Compile(context.Background(), "", Request{})
	if err == nil {
		t.Fatal("expected zero-value Compiler.Compile to fail closed, got nil error")
	}
}

func TestNewCompilerCompile(t *testing.T) {
	c := NewCompiler()
	_, err := c.Compile(context.Background(), "", Request{})
	if err == nil {
		t.Fatal("expected Compile to fail on empty root/request")
	}
}

// --- Compile stage 1-5 pipeline wiring (Lane 7D) ----------------------------
//
// These fakes and fixtures prove: (a) the five wired stages run in the
// declared order, recorded via wrapper fakes that append a stage label on
// first touch of each port; and (b) each stage's own refusal/operational
// failure short-circuits the remaining stages, proven by pairing an
// under-test port with "panic if called" fakes for every port only a LATER
// stage may touch.

// panicAuthorityLoader panics if Load or Resolve is ever called: used to
// prove a Compile failure occurred strictly before stage 2.
type panicAuthorityLoader struct{}

func (panicAuthorityLoader) Load(string) (*policyauthority.Store, error) {
	panic("contextcompile: stage 2 authority.Load must not be called")
}

func (panicAuthorityLoader) Resolve(*policyauthority.Store) (*policyauthority.EffectivePolicy, error) {
	panic("contextcompile: stage 2 authority.Resolve must not be called")
}

// panicRepoFactsGatherer panics if Gather is ever called: used to prove a
// Compile failure occurred strictly before stage 3.
type panicRepoFactsGatherer struct{}

func (panicRepoFactsGatherer) Gather(context.Context, repositoryfacts.GatherInput) (repositoryfacts.Snapshot, error) {
	panic("contextcompile: stage 3 repoFacts.Gather must not be called")
}

// panicGitReader panics on every method: used to prove a Compile failure
// occurred strictly before stage 4 (the only wired stage that reads Git).
type panicGitReader struct{}

func (panicGitReader) Show(context.Context, string, string, string) ([]byte, error) {
	panic("contextcompile: stage 4 git.Show must not be called")
}

func (panicGitReader) LsTreeEntries(context.Context, string, string) ([]gitx.TreeEntry, error) {
	panic("contextcompile: stage 4 git.LsTreeEntries must not be called")
}

func (panicGitReader) WorktreeChangedPaths(context.Context, string) ([]string, error) {
	panic("contextcompile: git.WorktreeChangedPaths must not be called")
}

// panicStateResolver panics if Resolve is ever called: used, paired with
// panicGitReader, to prove a Compile failure occurred strictly before
// stage 4.
type panicStateResolver struct{}

func (panicStateResolver) Resolve(context.Context, string, specstate.Candidate) (specstate.Result, error) {
	panic("contextcompile: stage 4 states.Resolve must not be called")
}

// panicProjectionVerifier panics if Verify is ever called: used to prove a
// Compile failure occurred strictly before stage 5.
type panicProjectionVerifier struct{}

func (panicProjectionVerifier) Verify(string) (*instructionprojection.Report, error) {
	panic("contextcompile: stage 5 projection.Verify must not be called")
}

// stubRepoFactsGatherer is a hermetic RepositoryFactsGatherer fake: it
// returns exactly the configured Snapshot or error, ignoring its input.
type stubRepoFactsGatherer struct {
	snapshot repositoryfacts.Snapshot
	err      error
}

func (g stubRepoFactsGatherer) Gather(context.Context, repositoryfacts.GatherInput) (repositoryfacts.Snapshot, error) {
	return g.snapshot, g.err
}

// stubProjectionVerifier is a hermetic ProjectionVerifier fake: it returns
// exactly the configured Report or error, ignoring root.
type stubProjectionVerifier struct {
	report *instructionprojection.Report
	err    error
}

func (v stubProjectionVerifier) Verify(string) (*instructionprojection.Report, error) {
	return v.report, v.err
}

// validRepositorySnapshot returns a fully valid, self-consistent
// repositoryfacts.Snapshot naming head as the exact known HEAD and branch
// as the exact known current branch.
func validRepositorySnapshot(head, branch string) repositoryfacts.Snapshot {
	return repositoryfacts.Snapshot{
		Facts: repositoryfacts.Facts{
			RemoteOrigin:  repositoryfacts.StringFact{Known: true, Value: "github.com/example/repo"},
			Branch:        repositoryfacts.StringFact{Known: true, Value: branch},
			Head:          repositoryfacts.StringFact{Known: true, Value: head},
			DefaultBranch: repositoryfacts.DefaultBranchFact{Known: true, Name: "main", Ref: "refs/heads/main", Head: head},
			Relationship:  repositoryfacts.RelationshipEqual,
			Dirty:         repositoryfacts.BoolFact{Known: true, Value: false},
			Staged:        repositoryfacts.BoolFact{Known: true, Value: false},
			Worktree:      repositoryfacts.WorktreeFact{Managed: false},
			Source:        repositoryfacts.SourceRemoteRef,
		},
		Disclosures: []repositoryfacts.DisclosureCode{},
	}
}

// compileHead is the fixed 40-hex HEAD every stage 3-5 fixture in this
// section agrees on.
const compileHead = "ffffffffffffffffffffffffffffffffffffffff"

// validCompileRequest builds a minimal, grammar-valid Request naming spec
// as the target: schema, adapter, an unrestricted (empty) scope, phase
// build, and no expected-repository assertion.
func validCompileRequest(spec string) Request {
	return Request{
		Schema:  RequestSchema,
		Adapter: AdapterRef{ID: "codex", Version: "1"},
		Phase:   PhaseBuild,
		Scope:   policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		Spec:    spec,
	}
}

// compilerAcceptedFixture wires a GitReader/StateResolver pair that
// resolves the "story-multi-parent.md" fixture spec (and both of its
// feature-fragment parents, "feature-alpha.md"/"feature-beta.md") as
// accepted at compileHead, reusing fragments_test.go's own fixture
// spec-decoding helper. It returns the ports plus the story's exact spec
// ref, ready to pass as a Request.Spec.
func compilerAcceptedFixture(t *testing.T) (GitReader, StateResolver, string) {
	t.Helper()
	storyData, storyFM := decodeFragmentSpecFixture(t, "story-multi-parent.md")
	alphaData, _ := decodeFragmentSpecFixture(t, "feature-alpha.md")
	betaData, _ := decodeFragmentSpecFixture(t, "feature-beta.md")

	storyPath := ".verdi/specs/active/" + strings.TrimPrefix(storyFM.ID, "spec/") + "/spec.md"
	paths := map[string][]byte{
		storyPath: storyData,
		".verdi/specs/active/feature-alpha/spec.md": alphaData,
		".verdi/specs/active/feature-beta/spec.md":  betaData,
	}
	objects := map[string]string{
		storyPath: strings.Repeat("d", 40),
		".verdi/specs/active/feature-alpha/spec.md": strings.Repeat("a", 40),
		".verdi/specs/active/feature-beta/spec.md":  strings.Repeat("b", 40),
	}
	entries := make([]gitx.TreeEntry, 0, len(paths))
	for path := range paths {
		entries = append(entries, gitx.TreeEntry{Mode: "100644", Type: "blob", Object: objects[path], Path: path})
	}

	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			return append([]gitx.TreeEntry(nil), entries...), nil
		},
		show: func(_ context.Context, _ string, _ string, path string) ([]byte, error) {
			data, ok := paths[path]
			if !ok {
				return nil, errors.New("contextcompile: unexpected path in compilerAcceptedFixture")
			}
			return append([]byte(nil), data...), nil
		},
	}
	states := authorityStateResolver{resolve: func(_ context.Context, _ string, candidate specstate.Candidate) (specstate.Result, error) {
		return specstate.Result{
			State:    specstate.AcceptedPendingBuild,
			Relation: specstate.RelationExact,
			Baseline: &specstate.Baseline{Path: candidate.Path, Blob: objects[candidate.Path], LandingCommit: strings.Repeat("c", 40)},
		}, nil
	}}
	return git, states, storyFM.ID
}

// compilerWrongClassFixture wires a GitReader/StateResolver pair that
// resolves the "feature-alpha.md" fixture spec (class feature, not story)
// as accepted at compileHead: ResolveAcceptedSpec succeeds on it, but
// ResolveFeatureFragments must then reject it as the wrong target class.
func compilerWrongClassFixture(t *testing.T) (GitReader, StateResolver, string) {
	t.Helper()
	data, fm := decodeFragmentSpecFixture(t, "feature-alpha.md")
	path := ".verdi/specs/active/" + strings.TrimPrefix(fm.ID, "spec/") + "/spec.md"
	object := strings.Repeat("a", 40)

	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			return []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: object, Path: path}}, nil
		},
		show: func(context.Context, string, string, string) ([]byte, error) {
			return append([]byte(nil), data...), nil
		},
	}
	states := authorityStateResolver{resolve: func(_ context.Context, _ string, candidate specstate.Candidate) (specstate.Result, error) {
		return specstate.Result{
			State:    specstate.AcceptedPendingBuild,
			Relation: specstate.RelationExact,
			Baseline: &specstate.Baseline{Path: candidate.Path, Blob: object, LandingCommit: strings.Repeat("c", 40)},
		}, nil
	}}
	return git, states, fm.ID
}

// componentSpecFixture is a minimal, grammar-valid class:component
// specification: components carry no object model at all (02: "No story, no
// ACs"), so this is the whole legal artifact.
const componentSpecFixture = `---
id: spec/component-fixture
kind: spec
class: component
title: "Component fixture"
status: active
owners: [platform-team]
---
# Component fixture

Body prose.
`

// compilerComponentClassFixture wires a GitReader/StateResolver pair that
// resolves componentSpecFixture (class component — neither feature nor
// story) as accepted at compileHead: ResolveAcceptedSpec succeeds on it, and
// stage 4's class dispatch must then refuse it as the wrong target class.
func compilerComponentClassFixture(t *testing.T) (GitReader, StateResolver, string) {
	t.Helper()
	data := []byte(componentSpecFixture)
	path := ".verdi/specs/active/component-fixture/spec.md"
	object := strings.Repeat("e", 40)

	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
			return []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: object, Path: path}}, nil
		},
		show: func(context.Context, string, string, string) ([]byte, error) {
			return append([]byte(nil), data...), nil
		},
	}
	states := authorityStateResolver{resolve: func(_ context.Context, _ string, candidate specstate.Candidate) (specstate.Result, error) {
		return specstate.Result{
			State:    specstate.AcceptedPendingBuild,
			Relation: specstate.RelationExact,
			Baseline: &specstate.Baseline{Path: candidate.Path, Blob: object, LandingCommit: strings.Repeat("c", 40)},
		}, nil
	}}
	return git, states, "spec/component-fixture"
}

// gitWithWorktree wraps a GitReader, overriding only WorktreeChangedPaths —
// used to give compilerAcceptedFixture's authorityGit (whose own
// WorktreeChangedPaths always panics; see authority_test.go) a working
// stage-6 candidate-discovery call without disturbing its Show/LsTreeEntries
// behavior.
type gitWithWorktree struct {
	GitReader
	worktree func(context.Context, string) ([]string, error)
}

func (g gitWithWorktree) WorktreeChangedPaths(ctx context.Context, root string) ([]string, error) {
	return g.worktree(ctx, root)
}

// unacceptedTargetGitStates returns a GitReader/StateResolver pair whose
// LsTreeEntries always reports the target absent from HEAD (so
// ResolveAcceptedSpec returns AcceptedSpecRefusal before ever calling
// Show or Resolve).
func unacceptedTargetGitStates() (GitReader, StateResolver) {
	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) { return nil, nil },
		show: func(context.Context, string, string, string) ([]byte, error) {
			return nil, errors.New("contextcompile: unexpected Show call: target is not in the HEAD tree")
		},
	}
	states := authorityStateResolver{resolve: func(context.Context, string, specstate.Candidate) (specstate.Result, error) {
		return specstate.Result{}, errors.New("contextcompile: unexpected Resolve call: target is not in the HEAD tree")
	}}
	return git, states
}

// --- stage 1: request validation --------------------------------------

func TestCompilerStage1MalformedRequestShortCircuits(t *testing.T) {
	c := newCompilerWithPorts(panicGitReader{}, panicStateResolver{}, panicAuthorityLoader{}, nil, panicRepoFactsGatherer{}, panicProjectionVerifier{})
	req := validCompileRequest("spec/example-story")
	req.Schema = "not-the-schema"
	_, err := c.Compile(context.Background(), "/repo", req)
	if err == nil {
		t.Fatal("expected malformed request to fail")
	}
	if IsRefusal(err) {
		t.Fatalf("malformed request classified as refusal: %T %v", err, err)
	}
}

func TestCompilerStage1PhaseScopeRefusalShortCircuits(t *testing.T) {
	c := newCompilerWithPorts(panicGitReader{}, panicStateResolver{}, panicAuthorityLoader{}, nil, panicRepoFactsGatherer{}, panicProjectionVerifier{})
	req := validCompileRequest("spec/example-story")
	req.Phase = PhaseBuild
	req.Scope.Phases = []string{"design"}
	_, err := c.Compile(context.Background(), "/repo", req)
	var refusal *PhaseScopeRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected *PhaseScopeRefusal, got %T %v", err, err)
	}
	if !IsRefusal(err) {
		t.Fatal("PhaseScopeRefusal not classified as a refusal")
	}
}

// --- stage 2: policy authority resolution -------------------------------

func TestCompilerStage2NoConstitutionRefusalShortCircuits(t *testing.T) {
	c := newCompilerWithPorts(panicGitReader{}, panicStateResolver{}, defaultAuthorityLoader{}, nil, panicRepoFactsGatherer{}, panicProjectionVerifier{})
	req := validCompileRequest("spec/example-story")
	_, err := c.Compile(context.Background(), t.TempDir(), req)
	var refusal *NoConstitutionRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected *NoConstitutionRefusal, got %T %v", err, err)
	}
	if !IsRefusal(err) {
		t.Fatal("NoConstitutionRefusal not classified as a refusal")
	}
}

func TestCompilerStage2AdapterMismatchRefusalShortCircuits(t *testing.T) {
	root := installPolicyFixture(t)
	c := newCompilerWithPorts(panicGitReader{}, panicStateResolver{}, defaultAuthorityLoader{}, nil, panicRepoFactsGatherer{}, panicProjectionVerifier{})
	req := validCompileRequest("spec/example-story")
	req.Adapter = AdapterRef{ID: "codex", Version: "no-such-version"}
	_, err := c.Compile(context.Background(), root, req)
	var refusal *AdapterMismatchRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected *AdapterMismatchRefusal, got %T %v", err, err)
	}
	if !IsRefusal(err) {
		t.Fatal("AdapterMismatchRefusal not classified as a refusal")
	}
}

// --- stage 3: repository facts + optional expectation -------------------

func TestCompilerStage3GathererOperationalErrorShortCircuits(t *testing.T) {
	root := installPolicyFixture(t)
	repoFacts := stubRepoFactsGatherer{err: errors.New("contextcompile: fixture gather failure")}
	c := newCompilerWithPorts(panicGitReader{}, panicStateResolver{}, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
	req := validCompileRequest("spec/example-story")
	_, err := c.Compile(context.Background(), root, req)
	if err == nil {
		t.Fatal("expected gatherer failure to fail Compile")
	}
	if IsRefusal(err) {
		t.Fatalf("gatherer operational error classified as refusal: %T %v", err, err)
	}
}

func TestCompilerStage3ExpectedBranchMismatchRefusalShortCircuits(t *testing.T) {
	root := installPolicyFixture(t)
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	c := newCompilerWithPorts(panicGitReader{}, panicStateResolver{}, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
	req := validCompileRequest("spec/example-story")
	req.Expected = &Expected{Branch: "not-main", Head: compileHead}
	_, err := c.Compile(context.Background(), root, req)
	var refusal *ExpectedRepositoryMismatchRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected *ExpectedRepositoryMismatchRefusal, got %T %v", err, err)
	}
	if !IsRefusal(err) {
		t.Fatal("ExpectedRepositoryMismatchRefusal not classified as a refusal")
	}
}

func TestCompilerStage3ExpectedHeadMismatchRefusalShortCircuits(t *testing.T) {
	root := installPolicyFixture(t)
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	c := newCompilerWithPorts(panicGitReader{}, panicStateResolver{}, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
	req := validCompileRequest("spec/example-story")
	req.Expected = &Expected{Branch: "main", Head: strings.Repeat("0", 40)}
	_, err := c.Compile(context.Background(), root, req)
	var refusal *ExpectedRepositoryMismatchRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected *ExpectedRepositoryMismatchRefusal, got %T %v", err, err)
	}
	if !IsRefusal(err) {
		t.Fatal("ExpectedRepositoryMismatchRefusal not classified as a refusal")
	}
}

// --- stage 4: accepted spec + feature fragments --------------------------

func TestCompilerStage4UnacceptedTargetRefusalShortCircuits(t *testing.T) {
	root := installPolicyFixture(t)
	git, states := unacceptedTargetGitStates()
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	c := newCompilerWithPorts(git, states, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
	req := validCompileRequest("spec/example-story")
	_, err := c.Compile(context.Background(), root, req)
	var refusal *AcceptedSpecRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected *AcceptedSpecRefusal, got %T %v", err, err)
	}
	if !IsRefusal(err) {
		t.Fatal("AcceptedSpecRefusal not classified as a refusal")
	}
}

// TestCompilerStage4WrongClassRefusalShortCircuits: a feature specification
// requested as a build target is a state-valid, accepted target whose CLASS
// the requested phase may not consume — the plan's Task 7 Step 2 explicitly
// lists "wrong target class" among the typed refusals a compile must
// return (alongside "no constitution, adapter mismatch, ... unaccepted
// target, wrong target class, and projection drift"), and authority design
// §6's Build section fixes "the build capsule requires an accepted story or
// spike" as a phase/target-class rule, not a decode-time malformation. This
// therefore adjudicates to *DeclaredScopeRefusal (exit 1), matching
// compiler.go's stage 4 special case for a feature-class target requested
// under phase build — not an ordinary operational/exit-2 failure.
func TestCompilerStage4WrongClassRefusalShortCircuits(t *testing.T) {
	root := installPolicyFixture(t)
	git, states, ref := compilerWrongClassFixture(t)
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	c := newCompilerWithPorts(git, states, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
	req := validCompileRequest(ref)
	_, err := c.Compile(context.Background(), root, req)
	var refusal *DeclaredScopeRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected *DeclaredScopeRefusal, got %T %v", err, err)
	}
	if !IsRefusal(err) {
		t.Fatal("wrong-class DeclaredScopeRefusal not classified as a refusal")
	}
	if refusal.Phase != PhaseBuild || refusal.Ref != ref {
		t.Fatalf("DeclaredScopeRefusal = %+v, want phase=build ref=%q", refusal, ref)
	}
}

// --- stage 5: managed instruction projection verification ---------------

func TestCompilerStage5ProjectionDriftRefusal(t *testing.T) {
	root := installPolicyFixture(t)
	git, states, ref := compilerAcceptedFixture(t)
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	projection := stubProjectionVerifier{report: &instructionprojection.Report{
		Findings: []instructionprojection.Finding{
			{Adapter: "codex", Code: instructionprojection.ReasonDrift, Path: "AGENTS.md", Expected: "sha256:one", Actual: "sha256:two"},
		},
	}}
	c := newCompilerWithPorts(git, states, defaultAuthorityLoader{}, nil, repoFacts, projection)
	req := validCompileRequest(ref)
	_, err := c.Compile(context.Background(), root, req)
	var refusal *ProjectionDriftRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected *ProjectionDriftRefusal, got %T %v", err, err)
	}
	if !IsRefusal(err) {
		t.Fatal("ProjectionDriftRefusal not classified as a refusal")
	}
	if len(refusal.Paths) != 1 || refusal.Paths[0] != "AGENTS.md" {
		t.Fatalf("ProjectionDriftRefusal.Paths = %v, want [AGENTS.md]", refusal.Paths)
	}
	// Authority design §10: "Existing generated projection drift | Exit-1
	// typed refusal with closed projection reason". The witness therefore
	// names the closed instructionprojection.Reason code(s), not paths alone.
	if len(refusal.Reasons) != 1 || refusal.Reasons[0] != string(instructionprojection.ReasonDrift) {
		t.Fatalf("ProjectionDriftRefusal.Reasons = %v, want [%q]", refusal.Reasons, instructionprojection.ReasonDrift)
	}
}

// TestCompilerStage5ProjectionDriftRefusalCarriesReasonsWithoutPaths proves a
// drift report whose findings name no path at all (the real
// ReasonIncompleteDiscovery/ReasonOrphanManifest shapes can carry a bare
// directory-level or manifest-level finding) still refuses with a NONEMPTY
// witness: the closed reason codes stand in for the absent paths, so §10's
// "typed refusal with closed projection reason" is never satisfied by an
// empty refusal.
func TestCompilerStage5ProjectionDriftRefusalCarriesReasonsWithoutPaths(t *testing.T) {
	root := installPolicyFixture(t)
	git, states, ref := compilerAcceptedFixture(t)
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	projection := stubProjectionVerifier{report: &instructionprojection.Report{
		Findings: []instructionprojection.Finding{
			{Adapter: "codex", Code: instructionprojection.ReasonIncompleteDiscovery, Detail: "walk failed"},
			{Adapter: "codex", Code: instructionprojection.ReasonOrphanManifest},
		},
	}}
	c := newCompilerWithPorts(git, states, defaultAuthorityLoader{}, nil, repoFacts, projection)
	_, err := c.Compile(context.Background(), root, validCompileRequest(ref))
	var refusal *ProjectionDriftRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected *ProjectionDriftRefusal, got %T %v", err, err)
	}
	if len(refusal.Paths) != 0 {
		t.Fatalf("ProjectionDriftRefusal.Paths = %v, want empty (no finding named a path)", refusal.Paths)
	}
	want := []string{string(instructionprojection.ReasonIncompleteDiscovery), string(instructionprojection.ReasonOrphanManifest)}
	if !reflect.DeepEqual(refusal.Reasons, want) {
		t.Fatalf("ProjectionDriftRefusal.Reasons = %v, want sorted unique %v", refusal.Reasons, want)
	}
}

// TestCompilerStage5ProjectionDriftWithoutWitnessIsOperational proves a
// report that is not clean yet names neither a path nor a reason code cannot
// become an empty exit-1 refusal: an unwitnessable drift claim is malformed
// port output, so it fails closed as an operational error instead.
func TestCompilerStage5ProjectionDriftWithoutWitnessIsOperational(t *testing.T) {
	root := installPolicyFixture(t)
	git, states, ref := compilerAcceptedFixture(t)
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	projection := stubProjectionVerifier{report: &instructionprojection.Report{
		Findings: []instructionprojection.Finding{{Adapter: "codex"}},
	}}
	c := newCompilerWithPorts(git, states, defaultAuthorityLoader{}, nil, repoFacts, projection)
	_, err := c.Compile(context.Background(), root, validCompileRequest(ref))
	if err == nil {
		t.Fatal("expected a witnessless drift report to fail Compile")
	}
	if IsRefusal(err) {
		t.Fatalf("witnessless drift report classified as a refusal: %T %v", err, err)
	}
}

// TestCompilerStage4ComponentClassRefusalIsTyped proves a component-class
// target — like a feature-class one — is a state-valid accepted target the
// requested phase may not consume, so it returns the SAME typed
// *DeclaredScopeRefusal family (plan Task 7 Step 2 lists "wrong target
// class" among the typed refusals), never an untyped exit-2 error.
func TestCompilerStage4ComponentClassRefusalIsTyped(t *testing.T) {
	root := installPolicyFixture(t)
	git, states, ref := compilerComponentClassFixture(t)
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	c := newCompilerWithPorts(git, states, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
	_, err := c.Compile(context.Background(), root, validCompileRequest(ref))
	var refusal *DeclaredScopeRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected *DeclaredScopeRefusal, got %T %v", err, err)
	}
	if !IsRefusal(err) {
		t.Fatal("component-class DeclaredScopeRefusal not classified as a refusal")
	}
	if refusal.Phase != PhaseBuild || refusal.Ref != ref {
		t.Fatalf("DeclaredScopeRefusal = %+v, want phase=build ref=%q", refusal, ref)
	}
}

// sentinelWorktreeErr is the fixed operational failure
// stubWorktreeChangedPathsErr injects at stage 6, so a test can prove
// Compile actually reached stage 6 (rather than stopping at, or being
// vacuously satisfied by, stage 5) without needing this compiler_test.go
// fixture to carry a full classification/manifest-assembly setup — that
// full-success proof belongs to integration_test.go's hermetic
// end-to-end tests instead.
var sentinelWorktreeErr = errors.New("contextcompile: fixture stage 6 worktree failure")

func stubWorktreeChangedPathsErr(context.Context, string) ([]string, error) {
	return nil, sentinelWorktreeErr
}

// TestCompilerStage5CleanProjectionProceedsPastStage5 proves a clean
// managed-projection verification does NOT itself refuse or halt the
// pipeline: Compile continues on into stage 6 (candidate-universe
// discovery), evidenced by reaching (and here, failing inside) the stage-6
// WorktreeChangedPaths call. compiler.go's full stage 6-12 pipeline is
// exercised to actual completion by integration_test.go's hermetic
// end-to-end fixtures; this compiler_test.go file stays scoped to proving
// short-circuit/continuation behavior at each port boundary.
func TestCompilerStage5CleanProjectionProceedsPastStage5(t *testing.T) {
	root := installPolicyFixture(t)
	git, states, ref := compilerAcceptedFixture(t)
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	projection := stubProjectionVerifier{report: &instructionprojection.Report{}}
	c := newCompilerWithPorts(gitWithWorktree{GitReader: git, worktree: stubWorktreeChangedPathsErr}, states, defaultAuthorityLoader{}, nil, repoFacts, projection)
	req := validCompileRequest(ref)
	_, err := c.Compile(context.Background(), root, req)
	if !errors.Is(err, sentinelWorktreeErr) {
		t.Fatalf("expected Compile to reach stage 6 and surface the injected worktree failure, got %T %v", err, err)
	}
	if IsRefusal(err) {
		t.Fatalf("stage 6 operational error classified as refusal: %T %v", err, err)
	}
}

// --- stage order proof ---------------------------------------------------

// orderRecordingGit wraps a GitReader and appends a fixed label to log on
// its first-touched method.
type orderRecordingGit struct {
	GitReader
	log *[]string
}

func (g orderRecordingGit) LsTreeEntries(ctx context.Context, root, ref string) ([]gitx.TreeEntry, error) {
	*g.log = append(*g.log, "stage4:git.LsTreeEntries")
	return g.GitReader.LsTreeEntries(ctx, root, ref)
}

// WorktreeChangedPaths records "stage6" on every call: Compile's own stage
// 6 is the pipeline's only caller of this method (ResolveAcceptedSpec,
// ResolveFeatureFragments, and ResolveBoundObligations — every stage-4 git
// consumer — read only Show/LsTreeEntries).
func (g orderRecordingGit) WorktreeChangedPaths(ctx context.Context, root string) ([]string, error) {
	*g.log = append(*g.log, "stage6:git.WorktreeChangedPaths")
	return g.GitReader.WorktreeChangedPaths(ctx, root)
}

// orderRecordingStates wraps a StateResolver and appends a fixed label to
// log on every Resolve call.
type orderRecordingStates struct {
	StateResolver
	log *[]string
}

func (s orderRecordingStates) Resolve(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error) {
	*s.log = append(*s.log, "stage4:states.Resolve")
	return s.StateResolver.Resolve(ctx, root, candidate)
}

// orderRecordingAuthority wraps an AuthorityLoader and appends a fixed
// label to log on Load.
type orderRecordingAuthority struct {
	AuthorityLoader
	log *[]string
}

func (a orderRecordingAuthority) Load(root string) (*policyauthority.Store, error) {
	*a.log = append(*a.log, "stage2:authority.Load")
	return a.AuthorityLoader.Load(root)
}

// orderRecordingRepoFacts wraps a RepositoryFactsGatherer and appends a
// fixed label to log on Gather.
type orderRecordingRepoFacts struct {
	RepositoryFactsGatherer
	log *[]string
}

func (g orderRecordingRepoFacts) Gather(ctx context.Context, in repositoryfacts.GatherInput) (repositoryfacts.Snapshot, error) {
	*g.log = append(*g.log, "stage3:repoFacts.Gather")
	return g.RepositoryFactsGatherer.Gather(ctx, in)
}

// orderRecordingProjection wraps a ProjectionVerifier and appends a fixed
// label to log on Verify.
type orderRecordingProjection struct {
	ProjectionVerifier
	log *[]string
}

func (p orderRecordingProjection) Verify(root string) (*instructionprojection.Report, error) {
	*p.log = append(*p.log, "stage5:projection.Verify")
	return p.ProjectionVerifier.Verify(root)
}

// TestCompilerStageOrderRecordsSequence proves stage 2 (authority.Load),
// stage 3 (repoFacts.Gather) and stage 5 (projection.Verify) each run
// exactly once, in that relative order, strictly before stage 6 is
// entered — evidenced by the unambiguous, single-call-site
// git.WorktreeChangedPaths (stage 6's only exclusive port method) landing
// last. It deliberately does NOT assert a single global non-decreasing
// order over every logged call: compiler.go's own stage 6 re-calls
// git.LsTreeEntries (already called earlier, from within stage 4's
// ResolveAcceptedSpec/ResolveFeatureFragments/ResolveBoundObligations), so
// that one method's call sites are not stage-exclusive, and compiler.go's
// own Compile doc comment already discloses that stages 6/7/8 are not
// claimed to run in strict Go-statement stage order either. Stage 6 is
// deliberately made to fail (the same injected sentinelWorktreeErr
// TestCompilerStage5CleanProjectionProceedsPastStage5 uses) so this test
// stays a lightweight port-boundary/order proof; the full stage 6-12
// pipeline reaching a successful Result is integration_test.go's concern.
func TestCompilerStageOrderRecordsSequence(t *testing.T) {
	var log []string
	root := installPolicyFixture(t)
	git, states, ref := compilerAcceptedFixture(t)
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	projection := stubProjectionVerifier{report: &instructionprojection.Report{}}

	c := newCompilerWithPorts(
		orderRecordingGit{GitReader: gitWithWorktree{GitReader: git, worktree: stubWorktreeChangedPathsErr}, log: &log},
		orderRecordingStates{StateResolver: states, log: &log},
		orderRecordingAuthority{AuthorityLoader: defaultAuthorityLoader{}, log: &log},
		nil,
		orderRecordingRepoFacts{RepositoryFactsGatherer: repoFacts, log: &log},
		orderRecordingProjection{ProjectionVerifier: projection, log: &log},
	)
	req := validCompileRequest(ref)
	_, err := c.Compile(context.Background(), root, req)
	if !errors.Is(err, sentinelWorktreeErr) {
		t.Fatalf("expected the injected stage 6 worktree failure, got %T %v", err, err)
	}
	if IsRefusal(err) {
		t.Fatalf("stage 6 operational error classified as refusal: %T %v", err, err)
	}

	indexOf := func(entry string) int {
		for i, e := range log {
			if e == entry {
				return i
			}
		}
		t.Fatalf("log entry %q not recorded: full log %v", entry, log)
		return -1
	}
	load := indexOf("stage2:authority.Load")
	gather := indexOf("stage3:repoFacts.Gather")
	verify := indexOf("stage5:projection.Verify")
	worktree := indexOf("stage6:git.WorktreeChangedPaths")
	if !(load < gather && gather < verify && verify < worktree) {
		t.Fatalf("stage order regressed: authority.Load=%d repoFacts.Gather=%d projection.Verify=%d git.WorktreeChangedPaths=%d; full log %v",
			load, gather, verify, worktree, log)
	}
	if worktree != len(log)-1 {
		t.Fatalf("git.WorktreeChangedPaths was not the last recorded call: full log %v", log)
	}
}

// --- stage 9: a Git failure needed for classification is operational ------

// gitShowFailingForPath wraps a GitReader, failing Show only for the one
// named path (every other Show/LsTreeEntries/WorktreeChangedPaths call
// passes through unchanged) — used to prove a Git failure reachable only
// once classification actually needs to read a specific candidate's bytes
// (here, a selected policy operand read by stage 9's
// buildClassificationMaterials) surfaces as an ordinary operational error,
// never a typed refusal.
type gitShowFailingForPath struct {
	GitReader
	path string
	err  error
}

func (g gitShowFailingForPath) Show(ctx context.Context, root, ref, path string) ([]byte, error) {
	if path == g.path {
		return nil, g.err
	}
	return g.GitReader.Show(ctx, root, ref, path)
}

// TestCompilerStage9GitFailureNeededForClassificationIsOperational proves a
// Git read failure that stage 9 classification needs (here, reading the
// one applicable go-toolchain policy operand's HEAD bytes) is an ordinary
// exit-2 operational error, never a typed exit-1 refusal (authority design
// §10: "Git failure that prevents required classification" -> exit 2).
func TestCompilerStage9GitFailureNeededForClassificationIsOperational(t *testing.T) {
	root := installPolicyFixture(t)
	git, states, ref := compilerAcceptedFixture(t)
	repoFacts := stubRepoFactsGatherer{snapshot: validRepositorySnapshot(compileHead, "main")}
	projection := stubProjectionVerifier{report: &instructionprojection.Report{}}
	sentinel := errors.New("contextcompile: fixture stage 9 policy operand read failure")
	brokenGit := gitShowFailingForPath{
		GitReader: gitWithWorktree{GitReader: git, worktree: func(context.Context, string) ([]string, error) { return nil, nil }},
		path:      ".verdi/policy/policies/go-toolchain.md",
		err:       sentinel,
	}

	c := newCompilerWithPorts(brokenGit, states, defaultAuthorityLoader{}, nil, repoFacts, projection)
	_, err := c.Compile(context.Background(), root, validCompileRequest(ref))
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected Compile to surface the injected stage 9 policy-operand read failure, got %T %v", err, err)
	}
	if IsRefusal(err) {
		t.Fatalf("stage 9 Git failure classified as refusal (want operational): %T %v", err, err)
	}
}

// --- declared-context indexing, suppression and pinned-ref widening -------

func declaredItemFixture(logicalRef, pinned, path string) DeclaredContextItem {
	return DeclaredContextItem{
		Ref: pinned, LogicalRef: logicalRef, Path: path,
		ContentDigest: digestSeed('7'), Content: []byte("body\n"),
	}
}

func declaredResultFixture() DeclaredContextResult {
	adr := declaredItemFixture("adr/ctx-note", "adr/ctx-note@"+gitHashSeed('1'), ".verdi/adr/ctx-note.md")
	spec := declaredItemFixture("spec/context-only", "spec/context-only@"+gitHashSeed('1'), ".verdi/specs/active/context-only/spec.md")
	return DeclaredContextResult{
		Items: []DeclaredContextItem{adr, spec},
		Lift:  map[string]string{adr.Path: adr.LogicalRef, spec.Path: spec.LogicalRef},
	}
}

func TestIndexDeclaredContextItems(t *testing.T) {
	got, err := indexDeclaredContextItems(declaredResultFixture())
	if err != nil {
		t.Fatalf("indexDeclaredContextItems: unexpected error: %v", err)
	}
	if len(got) != 2 || got["adr/ctx-note"].Ref != "adr/ctx-note@"+gitHashSeed('1') {
		t.Fatalf("index = %+v, want both logical refs indexed to their pinned items", got)
	}
}

// TestIndexDeclaredContextItemsBijectionFailsClosed proves the Items<->Lift
// correspondence is checked in BOTH directions: the universe is built from
// Lift while the classification materials are built from Items, so a
// disagreement in either direction would produce a candidate no material can
// classify, or a material naming a candidate the universe never created.
func TestIndexDeclaredContextItemsBijectionFailsClosed(t *testing.T) {
	cases := map[string]func(*DeclaredContextResult){
		"duplicate logical ref among items": func(r *DeclaredContextResult) {
			r.Items[1].LogicalRef = r.Items[0].LogicalRef
		},
		"lift names a logical ref with no item": func(r *DeclaredContextResult) {
			r.Lift[".verdi/adr/other.md"] = "adr/other"
		},
		"item path is claimed by no lift": func(r *DeclaredContextResult) {
			delete(r.Lift, r.Items[0].Path)
		},
		"lift disagrees with its item's logical ref": func(r *DeclaredContextResult) {
			r.Lift[r.Items[0].Path] = r.Items[1].LogicalRef
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			result := declaredResultFixture()
			mutate(&result)
			if _, err := indexDeclaredContextItems(result); err == nil {
				t.Fatal("expected indexDeclaredContextItems to fail closed")
			}
		})
	}
}

// TestSuppressStoreOwnedDeclaredContext proves authority design §5's source
// precedence (SI-92: "an overlapping store-authority path suppresses the
// declared lift") drops the overlapping item AND its lift together, leaving
// every uncontested declared pin untouched. Dropping only one of the two is
// what made a legal overlapping pin abort the compile.
func TestSuppressStoreOwnedDeclaredContext(t *testing.T) {
	declared := declaredResultFixture()
	storeLifts := map[string]string{
		".verdi/specs/active/context-only/spec.md": "spec/context-only",
		".verdi/specs/active/story-x/spec.md":      "spec/story-x",
	}
	effective := suppressStoreOwnedDeclaredContext(declared, storeLifts)
	if len(effective.Items) != 1 || effective.Items[0].LogicalRef != "adr/ctx-note" {
		t.Fatalf("effective items = %+v, want only the uncontested adr/ctx-note", effective.Items)
	}
	if len(effective.Lift) != 1 || effective.Lift[".verdi/adr/ctx-note.md"] != "adr/ctx-note" {
		t.Fatalf("effective lift = %v, want only the uncontested adr/ctx-note path", effective.Lift)
	}
	// The suppressed set is still a consistent bijection.
	if _, err := indexDeclaredContextItems(effective); err != nil {
		t.Fatalf("suppressed set is not self-consistent: %v", err)
	}
	// The input is not mutated.
	if len(declared.Items) != 2 || len(declared.Lift) != 2 {
		t.Fatalf("suppression mutated its input: %+v", declared)
	}
}

func TestSuppressStoreOwnedDeclaredContextKeepsEverythingUncontested(t *testing.T) {
	declared := declaredResultFixture()
	effective := suppressStoreOwnedDeclaredContext(declared, map[string]string{})
	if !reflect.DeepEqual(effective.Items, declared.Items) || !reflect.DeepEqual(effective.Lift, declared.Lift) {
		t.Fatalf("suppression dropped an uncontested declared pin: %+v", effective)
	}
}

func declaredIncludedRow(id, ref string) IncludedEntry {
	value := ref
	return IncludedEntry{
		ID: id, Source: SourceDeclaredContext, Kind: IncludedDeclaredContextRef, Ref: &value,
		Applicability: ApplicabilityApplicable, PayloadChannel: ChannelData,
		ContentDigest: digestSeed('7'), PayloadDigest: digestSeed('8'), Disclosures: []DisclosureCode{},
	}
}

func TestApplyDeclaredContextPinnedRefs(t *testing.T) {
	index, err := indexDeclaredContextItems(declaredResultFixture())
	if err != nil {
		t.Fatalf("indexDeclaredContextItems: %v", err)
	}
	other := IncludedEntry{
		ID: "ref:spec/story-x", Source: SourceStoreAuthority, Kind: IncludedAcceptedSpec,
		Applicability: ApplicabilityApplicable, PayloadChannel: ChannelData,
		ContentDigest: digestSeed('7'), PayloadDigest: digestSeed('8'), Disclosures: []DisclosureCode{},
	}
	rows, err := applyDeclaredContextPinnedRefs([]IncludedEntry{
		declaredIncludedRow("ref:adr/ctx-note", "adr/ctx-note"), other,
	}, index)
	if err != nil {
		t.Fatalf("applyDeclaredContextPinnedRefs: unexpected error: %v", err)
	}
	if rows[0].ID != "ref:adr/ctx-note" {
		t.Fatalf("candidate identity changed: %+v", rows[0])
	}
	if rows[0].Ref == nil || *rows[0].Ref != "adr/ctx-note@"+gitHashSeed('1') {
		t.Fatalf("declared-context row ref = %v, want the complete pinned ref", rows[0].Ref)
	}
	if rows[1].Ref != nil {
		t.Fatalf("a non-declared-context row was rewritten: %+v", rows[1])
	}
}

// TestApplyDeclaredContextPinnedRefsFailsClosed proves the widening never
// invents or silently keeps a logical ref: a declared-context row whose
// logical ref has no resolved item, and one carrying no ref at all, both
// fail closed rather than shipping an unpinned manifest identity.
func TestApplyDeclaredContextPinnedRefsFailsClosed(t *testing.T) {
	index, err := indexDeclaredContextItems(declaredResultFixture())
	if err != nil {
		t.Fatalf("indexDeclaredContextItems: %v", err)
	}
	noRefRow := declaredIncludedRow("ref:adr/ctx-note", "adr/ctx-note")
	noRefRow.Ref = nil

	cases := map[string][]IncludedEntry{
		"no resolved item for the row's logical ref": {declaredIncludedRow("ref:adr/missing", "adr/missing")},
		"declared-context row carries no ref":        {noRefRow},
	}
	for name, rows := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := applyDeclaredContextPinnedRefs(rows, index); err == nil {
				t.Fatal("expected applyDeclaredContextPinnedRefs to fail closed")
			}
		})
	}
}

// --- store-authority lift map (authority design §5) -----------------------

func storeLiftTarget() ResolvedSpec {
	return ResolvedSpec{Ref: "spec/story-x", Path: ".verdi/specs/active/story-x/spec.md"}
}

func storeLiftAuthorityArtifacts() []storeAuthorityArtifact {
	return []storeAuthorityArtifact{
		{Ref: "policy-constitution/constitution", Path: ".verdi/policy/constitution.md", Digest: digestSeed('a')},
		{Ref: "governance-profile/solo-default", Path: ".verdi/policy/profiles/solo-default.md", Digest: digestSeed('b')},
	}
}

func TestBuildStoreLiftsEnumeratesEverySource(t *testing.T) {
	lifts, err := buildStoreLifts(
		storeLiftTarget(),
		[]FeatureFragment{validFragmentFixture("feature-a", "problem a")},
		[]BoundObligation{{Ref: "obligation/story-x--ac-1--static", Path: ".verdi/obligations/story-x/ac-1/static.md"}},
		[]PolicyOperand{{Kind: PolicyEntryPolicy, ID: "policy/go-toolchain", Path: ".verdi/policy/policies/go-toolchain.md"}},
		storeLiftAuthorityArtifacts(),
	)
	if err != nil {
		t.Fatalf("buildStoreLifts: unexpected error: %v", err)
	}
	want := map[string]string{
		".verdi/specs/active/story-x/spec.md":       "spec/story-x",
		".verdi/specs/active/feature-a/spec.md":     "spec/feature-a",
		".verdi/obligations/story-x/ac-1/static.md": "obligation/story-x--ac-1--static",
		".verdi/policy/policies/go-toolchain.md":    "policy/go-toolchain",
		".verdi/policy/constitution.md":             "policy-constitution/constitution",
		".verdi/policy/profiles/solo-default.md":    "governance-profile/solo-default",
	}
	if !reflect.DeepEqual(lifts, want) {
		t.Fatalf("buildStoreLifts = %v, want %v", lifts, want)
	}
}

// TestBuildStoreLiftsDuplicatePathFailsClosed proves two DISTINCT refs
// claiming one tracked path fail closed rather than silently overwriting one
// another: authority design §5 lifts a path into store-authority exactly
// once, so a second claimant is an inconsistent authority resolution, not a
// last-writer-wins merge.
func TestBuildStoreLiftsDuplicatePathFailsClosed(t *testing.T) {
	collidingArtifacts := storeLiftAuthorityArtifacts()
	collidingArtifacts[1].Path = collidingArtifacts[0].Path

	cases := map[string]func() error{
		"operand collides with the accepted spec": func() error {
			_, err := buildStoreLifts(
				storeLiftTarget(), nil, nil,
				[]PolicyOperand{{Kind: PolicyEntryPolicy, ID: "policy/go-toolchain", Path: ".verdi/specs/active/story-x/spec.md"}},
				nil,
			)
			return err
		},
		"two authority artifacts collide": func() error {
			_, err := buildStoreLifts(storeLiftTarget(), nil, nil, nil, collidingArtifacts)
			return err
		},
		"two fragments collide": func() error {
			second := validFragmentFixture("feature-b", "problem b")
			second.Feature.Path = ".verdi/specs/active/feature-a/spec.md"
			_, err := buildStoreLifts(
				storeLiftTarget(),
				[]FeatureFragment{validFragmentFixture("feature-a", "problem a"), second},
				nil, nil, nil,
			)
			return err
		},
		"empty operand path": func() error {
			_, err := buildStoreLifts(
				storeLiftTarget(), nil, nil,
				[]PolicyOperand{{Kind: PolicyEntryPolicy, ID: "policy/go-toolchain"}},
				nil,
			)
			return err
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			if err := run(); err == nil {
				t.Fatal("expected buildStoreLifts to fail closed")
			}
		})
	}
}

// TestBuildStoreLiftsIdenticalPairIsAccepted proves the guard rejects
// CONFLICTING claims, not a redundant identical one: re-declaring the same
// (path, ref) pair leaves the lift map unchanged and is therefore not an
// inconsistency.
func TestBuildStoreLiftsIdenticalPairIsAccepted(t *testing.T) {
	target := storeLiftTarget()
	lifts, err := buildStoreLifts(
		target, nil, nil,
		nil,
		[]storeAuthorityArtifact{{Ref: target.Ref, Path: target.Path, Digest: digestSeed('a')}},
	)
	if err != nil {
		t.Fatalf("buildStoreLifts: unexpected error on an identical repeated pair: %v", err)
	}
	if len(lifts) != 1 || lifts[target.Path] != target.Ref {
		t.Fatalf("buildStoreLifts = %v, want the single unchanged pair", lifts)
	}
}

// --- authorityRevision / contextRevisions (Lane 7B, authority design §9) ---
//
// digestSeed returns a valid "sha256:"+64-lowercase-hex digest built by
// repeating a single hex character, so tests can produce many distinct,
// grammar-valid digests without a real hasher.
func digestSeed(c byte) string {
	return "sha256:" + strings.Repeat(string(c), 64)
}

// gitHashSeed returns a valid 40-lowercase-hex git object hash.
func gitHashSeed(c byte) string {
	return strings.Repeat(string(c), 40)
}

func validAcceptedSpecFixture(nameSuffix string) AcceptedSpec {
	name := "root-story-" + nameSuffix
	return AcceptedSpec{
		Ref:           "spec/" + name,
		Path:          ".verdi/specs/active/" + name + "/spec.md",
		Blob:          gitHashSeed('1'),
		Commit:        gitHashSeed('2'),
		ContentDigest: digestSeed('3'),
	}
}

// validFragmentFixture builds one grammar-valid FeatureFragment naming
// spec/<name> with a single AC target. problemText lets sensitivity tests
// perturb the fragment's content without touching its identity (Feature.Ref).
func validFragmentFixture(name, problemText string) FeatureFragment {
	return FeatureFragment{
		Feature: FragmentFeature{
			Ref:          "spec/" + name,
			Path:         ".verdi/specs/active/" + name + "/spec.md",
			SourceDigest: digestSeed('4'),
		},
		Problem: artifact.Attribute{Text: problemText, Anchor: "problem-anchor"},
		Outcome: artifact.Attribute{Text: "outcome text", Anchor: "outcome-anchor"},
		Targets: []FragmentTarget{
			{
				ID:       "ac-example",
				Text:     "target text",
				Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic},
				Anchor:   "target-anchor",
			},
		},
		Constraints: []artifact.Constraint{},
		Decisions:   []artifact.Decision{},
	}
}

// baseAuthorityRevisionInput returns one complete, valid
// authorityRevisionInput with two parent fragments, two decisions and two
// obligations — enough operands in each list to exercise ordering and
// duplicate-detection tests.
func baseAuthorityRevisionInput() authorityRevisionInput {
	return authorityRevisionInput{
		EffectivePolicyDigest: digestSeed('a'),
		AcceptedSpec:          validAcceptedSpecFixture("x"),
		ParentFragments: []FeatureFragment{
			validFragmentFixture("feature-a", "problem a"),
			validFragmentFixture("feature-b", "problem b"),
		},
		Decisions: []authorityRevisionDecision{
			{Ref: "spec/feature-a#dc-one", Digest: digestSeed('b')},
			{Ref: "spec/feature-b#dc-two", Digest: digestSeed('c')},
		},
		Obligations: []authorityRevisionObligation{
			{Ref: "obligation/story-a--ac-1--static", Digest: digestSeed('d')},
			{Ref: "obligation/story-b--ac-2--static", Digest: digestSeed('e')},
		},
	}
}

func TestAuthorityRevisionOrderingInvariance(t *testing.T) {
	in := baseAuthorityRevisionInput()
	want, err := authorityRevision(in)
	if err != nil {
		t.Fatalf("authorityRevision(base): unexpected error: %v", err)
	}

	shuffled := baseAuthorityRevisionInput()
	shuffled.ParentFragments[0], shuffled.ParentFragments[1] = shuffled.ParentFragments[1], shuffled.ParentFragments[0]
	shuffled.Decisions[0], shuffled.Decisions[1] = shuffled.Decisions[1], shuffled.Decisions[0]
	shuffled.Obligations[0], shuffled.Obligations[1] = shuffled.Obligations[1], shuffled.Obligations[0]

	got, err := authorityRevision(shuffled)
	if err != nil {
		t.Fatalf("authorityRevision(shuffled): unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("authorityRevision is order-sensitive: base=%q shuffled=%q", want, got)
	}
}

func TestAuthorityRevisionSensitivity(t *testing.T) {
	base := baseAuthorityRevisionInput()
	baseDigest, err := authorityRevision(base)
	if err != nil {
		t.Fatalf("authorityRevision(base): unexpected error: %v", err)
	}

	mutations := map[string]func(*authorityRevisionInput){
		"effective_policy_digest": func(in *authorityRevisionInput) {
			in.EffectivePolicyDigest = digestSeed('9')
		},
		"accepted_spec.ref": func(in *authorityRevisionInput) {
			in.AcceptedSpec = validAcceptedSpecFixture("y")
		},
		"accepted_spec.path": func(in *authorityRevisionInput) {
			in.AcceptedSpec.Path = ".verdi/specs/active/root-story-x/spec.md.alt"
		},
		"accepted_spec.blob": func(in *authorityRevisionInput) {
			in.AcceptedSpec.Blob = gitHashSeed('7')
		},
		"accepted_spec.commit": func(in *authorityRevisionInput) {
			in.AcceptedSpec.Commit = gitHashSeed('8')
		},
		"accepted_spec.content_digest": func(in *authorityRevisionInput) {
			in.AcceptedSpec.ContentDigest = digestSeed('f')
		},
		"fragment_digest": func(in *authorityRevisionInput) {
			in.ParentFragments[0] = validFragmentFixture("feature-a", "a different problem statement entirely")
		},
		"decision_digest": func(in *authorityRevisionInput) {
			in.Decisions[0].Digest = digestSeed('0')
		},
		"obligation_digest": func(in *authorityRevisionInput) {
			in.Obligations[0].Digest = digestSeed('1')
		},
	}

	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			mutated := baseAuthorityRevisionInput()
			mutate(&mutated)
			got, err := authorityRevision(mutated)
			if err != nil {
				t.Fatalf("authorityRevision(mutated %s): unexpected error: %v", name, err)
			}
			if got == baseDigest {
				t.Fatalf("mutating %s did not change the authority revision digest (still %q)", name, got)
			}
		})
	}
}

// TestAuthorityRevisionInputExcludesNonAuthorityOperands documents and
// enforces authority design §9's exclusion list by construction, not by
// comment alone: authorityRevisionInput's field set is asserted exactly, so
// a field named repository/grants/actors/classification/projection/
// disclosure (or any other operand the manifest self digest binds instead)
// cannot be added to this type without this test failing. There is
// deliberately no way to construct an authorityRevisionInput carrying
// repository state, grants, actor posture, payload classification,
// projection files, or opaque disclosures.
func TestAuthorityRevisionInputExcludesNonAuthorityOperands(t *testing.T) {
	want := []string{"EffectivePolicyDigest", "AcceptedSpec", "ParentFragments", "Decisions", "Obligations"}
	typ := reflect.TypeOf(authorityRevisionInput{})
	if typ.NumField() != len(want) {
		t.Fatalf("authorityRevisionInput has %d fields, want exactly %v (%d)", typ.NumField(), want, len(want))
	}
	for i, name := range want {
		if got := typ.Field(i).Name; got != name {
			t.Fatalf("authorityRevisionInput field %d = %q, want %q", i, got, name)
		}
	}
}

func TestAuthorityRevisionDuplicateFragmentFailsClosed(t *testing.T) {
	in := baseAuthorityRevisionInput()
	in.ParentFragments[1] = validFragmentFixture("feature-a", "a different problem, same identity")
	if _, err := authorityRevision(in); err == nil {
		t.Fatal("expected duplicate parent fragment identity to fail closed")
	}
}

func TestAuthorityRevisionDuplicateDecisionFailsClosed(t *testing.T) {
	in := baseAuthorityRevisionInput()
	in.Decisions[1].Ref = in.Decisions[0].Ref
	if _, err := authorityRevision(in); err == nil {
		t.Fatal("expected duplicate decision identity to fail closed")
	}
}

func TestAuthorityRevisionDuplicateObligationFailsClosed(t *testing.T) {
	in := baseAuthorityRevisionInput()
	in.Obligations[1].Ref = in.Obligations[0].Ref
	if _, err := authorityRevision(in); err == nil {
		t.Fatal("expected duplicate obligation identity to fail closed")
	}
}

func TestAuthorityRevisionMalformedDigestFailsClosed(t *testing.T) {
	cases := map[string]func(*authorityRevisionInput){
		"effective_policy_digest": func(in *authorityRevisionInput) {
			in.EffectivePolicyDigest = "not-a-digest"
		},
		"accepted_spec.content_digest": func(in *authorityRevisionInput) {
			in.AcceptedSpec.ContentDigest = "not-a-digest"
		},
		"decision.digest": func(in *authorityRevisionInput) {
			in.Decisions[0].Digest = "not-a-digest"
		},
		"obligation.digest": func(in *authorityRevisionInput) {
			in.Obligations[0].Digest = "not-a-digest"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := baseAuthorityRevisionInput()
			mutate(&in)
			if _, err := authorityRevision(in); err == nil {
				t.Fatalf("expected malformed %s to fail closed", name)
			}
		})
	}
}

func TestAuthorityRevisionEmptyIdentityFailsClosed(t *testing.T) {
	cases := map[string]func(*authorityRevisionInput){
		"decision.ref": func(in *authorityRevisionInput) {
			in.Decisions[0].Ref = ""
		},
		"obligation.ref": func(in *authorityRevisionInput) {
			in.Obligations[0].Ref = ""
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := baseAuthorityRevisionInput()
			mutate(&in)
			if _, err := authorityRevision(in); err == nil {
				t.Fatalf("expected empty %s to fail closed", name)
			}
		})
	}
}

func TestAuthorityRevisionInputSlicesUnmodified(t *testing.T) {
	in := baseAuthorityRevisionInput()

	wantFragmentRefs := make([]string, len(in.ParentFragments))
	for i, f := range in.ParentFragments {
		wantFragmentRefs[i] = f.Feature.Ref
	}
	wantDecisions := append([]authorityRevisionDecision(nil), in.Decisions...)
	wantObligations := append([]authorityRevisionObligation(nil), in.Obligations...)

	if _, err := authorityRevision(in); err != nil {
		t.Fatalf("authorityRevision: unexpected error: %v", err)
	}

	for i, f := range in.ParentFragments {
		if f.Feature.Ref != wantFragmentRefs[i] {
			t.Fatalf("ParentFragments[%d] mutated: got ref %q, want %q", i, f.Feature.Ref, wantFragmentRefs[i])
		}
	}
	if !reflect.DeepEqual(in.Decisions, wantDecisions) {
		t.Fatalf("Decisions slice mutated: got %+v, want %+v", in.Decisions, wantDecisions)
	}
	if !reflect.DeepEqual(in.Obligations, wantObligations) {
		t.Fatalf("Obligations slice mutated: got %+v, want %+v", in.Obligations, wantObligations)
	}
}

func TestContextRevisions(t *testing.T) {
	authority := digestSeed('c')
	got, err := contextRevisions(authority)
	if err != nil {
		t.Fatalf("contextRevisions: unexpected error: %v", err)
	}
	want := Revisions{Authority: authority, Context: 1}
	if got != want {
		t.Fatalf("contextRevisions(%q) = %+v, want %+v", authority, got, want)
	}
}

func TestContextRevisionsMalformedAuthorityFailsClosed(t *testing.T) {
	if _, err := contextRevisions("not-a-digest"); err == nil {
		t.Fatal("expected contextRevisions to fail closed on a malformed authority digest")
	}
}
