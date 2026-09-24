package contextcompile

import (
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/repositoryfacts"
)

// SI-257 (as amended), lane E4c: the accepted-context conflict snapshot
// seals the CI ref beside Repository exactly when the compile's snapshot
// resolved the branch being closed from it (a detached checkout with a
// known CI ref), and carries none otherwise. Repository keeps recording
// the detached checkout.

// ciRefAcceptedFixture is hermeticAcceptedFixture with an injected
// repository snapshot, returning the compiler, the accepted request, and
// the root to compile it at.
func ciRefAcceptedFixture(t *testing.T, snapshot repositoryfacts.Snapshot) (Compiler, Request, string) {
	t.Helper()
	root := installPolicyFixture(t)
	git, states, ref := compilerAcceptedFixture(t)
	gitWT := gitWithWorktree{GitReader: policyDiskFallbackGit{GitReader: git}, worktree: func(context.Context, string) ([]string, error) { return nil, nil }}
	c := newCompilerWithPorts(gitWT, states, defaultAuthorityLoader{}, nil, stubRepoFactsGatherer{snapshot: snapshot}, stubProjectionVerifier{report: &instructionprojection.Report{}})
	return c, validCompileRequest(ref), root
}

func TestCompileConflictSealsCIRefOnlyWhenItSuppliedTheBranchBeingClosed(t *testing.T) {
	const closeBranch = "close/story-multi-parent"
	unresolved := validRepositorySnapshot(compileHead, "main")
	unresolved.Facts.Branch = repositoryfacts.StringFact{}
	unresolved.Disclosures = []repositoryfacts.DisclosureCode{repositoryfacts.DisclosureBranchUnresolved}
	unresolved.CIRef = knownCIRef(closeBranch)
	checkedOutInCI := validRepositorySnapshot(compileHead, "main")
	checkedOutInCI.CIRef = knownCIRef("main")

	tests := []struct {
		name       string
		snapshot   repositoryfacts.Snapshot
		expected   *Expected
		wantCIRef  *repositoryfacts.CIRefFact
		wantBranch repositoryfacts.StringFact
	}{
		{
			name:       "checked-out branch outside CI",
			snapshot:   validRepositorySnapshot(compileHead, "main"),
			expected:   &Expected{Branch: "main", Head: compileHead},
			wantBranch: repositoryfacts.StringFact{Known: true, Value: "main"},
		},
		{
			name:       "checked-out branch in a CI run",
			snapshot:   checkedOutInCI,
			expected:   &Expected{Branch: "main", Head: compileHead},
			wantBranch: repositoryfacts.StringFact{Known: true, Value: "main"},
		},
		{
			name:       "detached checkout with a known CI ref",
			snapshot:   detachedRepositorySnapshot(compileHead, knownCIRef(closeBranch)),
			expected:   &Expected{Branch: closeBranch, Head: compileHead},
			wantCIRef:  &repositoryfacts.CIRefFact{Known: true, Provider: repositoryfacts.CIProviderGitHub, Name: closeBranch},
			wantBranch: repositoryfacts.StringFact{Known: true, Value: closeBranch},
		},
		{
			name:       "detached checkout with a known CI ref and no expectation",
			snapshot:   detachedRepositorySnapshot(compileHead, knownCIRef(closeBranch)),
			wantCIRef:  &repositoryfacts.CIRefFact{Known: true, Provider: repositoryfacts.CIProviderGitHub, Name: closeBranch},
			wantBranch: repositoryfacts.StringFact{Known: true, Value: closeBranch},
		},
		{
			name:     "detached checkout with an unknown CI ref",
			snapshot: detachedRepositorySnapshot(compileHead, repositoryfacts.CIRefFact{Reason: repositoryfacts.CIRefReasonRemoteTrackingUnresolved}),
		},
		{
			name:     "detached checkout outside CI",
			snapshot: detachedRepositorySnapshot(compileHead, repositoryfacts.CIRefFact{}),
		},
		{
			name:     "branch unresolved (not detached) with a known CI ref",
			snapshot: unresolved,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, req, root := ciRefAcceptedFixture(t, tt.snapshot)
			req.Expected = tt.expected
			operands, err := c.CompileConflict(context.Background(), root, req, ConflictFacts{})
			if err != nil {
				t.Fatalf("CompileConflict: %v", err)
			}
			view, err := operands.View()
			if err != nil {
				t.Fatalf("View: %v", err)
			}
			got := view.Snapshot
			switch {
			case tt.wantCIRef == nil && got.CIRef != nil:
				t.Fatalf("sealed CIRef = %+v, want none", *got.CIRef)
			case tt.wantCIRef != nil && (got.CIRef == nil || *got.CIRef != *tt.wantCIRef):
				t.Fatalf("sealed CIRef = %v, want %+v", got.CIRef, *tt.wantCIRef)
			}
			if got.Repository != tt.snapshot.Facts {
				t.Fatalf("sealed Repository = %+v, want the physical checkout's facts %+v", got.Repository, tt.snapshot.Facts)
			}
			if b := got.BranchBeingClosed(); b != tt.wantBranch {
				t.Fatalf("BranchBeingClosed() = %+v, want %+v", b, tt.wantBranch)
			}
		})
	}
}

func TestSnapshotIdentityBranchBeingClosed(t *testing.T) {
	known := func(v string) repositoryfacts.StringFact { return repositoryfacts.StringFact{Known: true, Value: v} }
	ref := func(f repositoryfacts.CIRefFact) *repositoryfacts.CIRefFact { return &f }
	tests := []struct {
		name   string
		branch repositoryfacts.StringFact
		ciRef  *repositoryfacts.CIRefFact
		want   repositoryfacts.StringFact
	}{
		{name: "known branch, no CI ref", branch: known("main"), want: known("main")},
		{name: "known branch wins over a CI ref", branch: known("main"), ciRef: ref(knownCIRef("close/x")), want: known("main")},
		{name: "unknown branch, sealed CI ref", ciRef: ref(knownCIRef("close/x")), want: known("close/x")},
		{name: "unknown branch, no CI ref"},
		{name: "unknown branch, unknown CI ref", ciRef: ref(repositoryfacts.CIRefFact{Reason: repositoryfacts.CIRefReasonNameInvalid})},
		{name: "unknown branch, CI ref with an invalid name", ciRef: ref(repositoryfacts.CIRefFact{Known: true, Provider: repositoryfacts.CIProviderGitHub, Name: "main~1"})},
		{name: "unknown branch, CI ref with an unknown provider", ciRef: ref(repositoryfacts.CIRefFact{Known: true, Provider: "jenkins", Name: "close/x"})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := SnapshotIdentity{Repository: repositoryfacts.Facts{Branch: tt.branch}, CIRef: tt.ciRef}
			if got := s.BranchBeingClosed(); got != tt.want {
				t.Fatalf("BranchBeingClosed() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestConflictOperandsSnapshotTransportCIRef pins the sealed pair's
// invariant at the transport seam every seal and View() runs: a CI ref is
// carried only on an accepted-context snapshot whose recorded branch is
// unknown, and only as a known, valid CI ref.
func TestConflictOperandsSnapshotTransportCIRef(t *testing.T) {
	detached := detachedRepositorySnapshot(compileHead, knownCIRef("close/story-multi-parent"))
	c, req, root := ciRefAcceptedFixture(t, detached)
	operands, err := c.CompileConflict(context.Background(), root, req, ConflictFacts{})
	if err != nil {
		t.Fatalf("CompileConflict: %v", err)
	}
	valid, err := operands.View()
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	if valid.Snapshot.CIRef == nil {
		t.Fatal("test setup: the detached fixture sealed no CI ref")
	}
	if _, err := sealConflictOperands(cloneConflictView(valid)); err != nil {
		t.Fatalf("sealConflictOperands(valid sealed CI ref): %v", err)
	}

	cases := map[string]func(*ConflictView){
		"CI ref beside a known branch": func(v *ConflictView) {
			v.Snapshot.Repository.Branch = repositoryfacts.StringFact{Known: true, Value: "main"}
		},
		"unknown CI ref": func(v *ConflictView) {
			v.Snapshot.CIRef = &repositoryfacts.CIRefFact{Reason: repositoryfacts.CIRefReasonNameInvalid}
		},
		"zero CI ref": func(v *ConflictView) { v.Snapshot.CIRef = &repositoryfacts.CIRefFact{} },
		"CI ref with an invalid name": func(v *ConflictView) {
			v.Snapshot.CIRef = &repositoryfacts.CIRefFact{Known: true, Provider: repositoryfacts.CIProviderGitHub, Name: "main~1"}
		},
		"CI ref on a candidate snapshot": func(v *ConflictView) {
			v.Snapshot.TargetKind = snapshotTargetAcceptanceCandidate
			v.Snapshot.ManifestDigest = ""
			v.Snapshot.CandidateDigest = "sha256:" + strings.Repeat("1", 64)
			v.Snapshot.CandidateBlob = strings.Repeat("b", 40)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			view := cloneConflictView(valid)
			mutate(&view)
			if _, err := sealConflictOperands(view); err == nil || !strings.Contains(err.Error(), "ci ref") {
				t.Fatalf("sealConflictOperands error = %v, want the sealed CI-ref validation failure", err)
			}
		})
	}

	t.Run("builder refuses a CI ref it may not seal", func(t *testing.T) {
		_, err := buildSnapshotIdentity(snapshotBuildInput{
			targetKind:     snapshotTargetAcceptedContext,
			repository:     validRepositorySnapshot(compileHead, "main").Facts,
			manifestDigest: "sha256:" + strings.Repeat("1", 64),
			authority:      PolicyAuthority{Effective: &policyauthority.EffectivePolicy{}},
			ciRef:          &repositoryfacts.CIRefFact{Known: true, Provider: repositoryfacts.CIProviderGitHub, Name: "main"},
		})
		if err == nil || !strings.Contains(err.Error(), "ci ref") {
			t.Fatalf("buildSnapshotIdentity error = %v, want the sealed CI-ref validation failure", err)
		}
	})

	t.Run("clone safety", func(t *testing.T) {
		first, err := operands.View()
		if err != nil {
			t.Fatalf("View(first): %v", err)
		}
		first.Snapshot.CIRef.Name = "mutated"
		second, err := operands.View()
		if err != nil {
			t.Fatalf("View(second): unexpected error after caller-owned clone mutation: %v", err)
		}
		if second.Snapshot.CIRef == nil || second.Snapshot.CIRef.Name != "close/story-multi-parent" {
			t.Fatalf("sealed CI ref changed through a returned clone: %+v", second.Snapshot.CIRef)
		}
	})
}
