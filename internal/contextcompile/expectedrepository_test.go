package contextcompile

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/repositoryfacts"
)

// SI-257: stage 3 compares the caller expectation against "the branch
// being closed" (repositoryfacts.Snapshot.BranchBeingClosed) — the
// checked-out branch, else a detached checkout's validated CI ref — while
// the snapshot's Facts keep recording the detached checkout.

// detachedRepositorySnapshot is validRepositorySnapshot on a detached
// checkout carrying ciRef.
func detachedRepositorySnapshot(head string, ciRef repositoryfacts.CIRefFact) repositoryfacts.Snapshot {
	snapshot := validRepositorySnapshot(head, "main")
	snapshot.Facts.Branch = repositoryfacts.StringFact{}
	snapshot.Disclosures = []repositoryfacts.DisclosureCode{repositoryfacts.DisclosureBranchDetached}
	snapshot.CIRef = ciRef
	return snapshot
}

func knownCIRef(name string) repositoryfacts.CIRefFact {
	return repositoryfacts.CIRefFact{Known: true, Provider: repositoryfacts.CIProviderGitHub, Name: name}
}

func TestResolveExpectedRepositorySnapshot(t *testing.T) {
	head := strings.Repeat("a", 40)
	unknownCIRef := repositoryfacts.CIRefFact{Reason: repositoryfacts.CIRefReasonRemoteTrackingNotHead}
	unresolved := validRepositorySnapshot(head, "main")
	unresolved.Facts.Branch = repositoryfacts.StringFact{}
	unresolved.Disclosures = []repositoryfacts.DisclosureCode{repositoryfacts.DisclosureBranchUnresolved}
	unresolved.CIRef = knownCIRef("close/spec-x")

	tests := []struct {
		name       string
		expected   *Expected
		snapshot   repositoryfacts.Snapshot
		wantBranch string
		wantKnown  bool
		refusal    bool
		opErr      bool
	}{
		{name: "omitted expectation", snapshot: detachedRepositorySnapshot(head, unknownCIRef)},
		{name: "checked-out branch matches", expected: &Expected{Branch: "main", Head: head}, snapshot: validRepositorySnapshot(head, "main")},
		{name: "detached, CI ref matches", expected: &Expected{Branch: "close/spec-x", Head: head}, snapshot: detachedRepositorySnapshot(head, knownCIRef("close/spec-x"))},
		{
			name: "checked-out branch wins over the CI ref", expected: &Expected{Branch: "close/spec-x", Head: head},
			snapshot: func() repositoryfacts.Snapshot {
				s := validRepositorySnapshot(head, "main")
				s.CIRef = knownCIRef("close/spec-x")
				return s
			}(),
			wantBranch: "main", wantKnown: true, refusal: true,
		},
		{name: "detached, CI ref differs", expected: &Expected{Branch: "close/spec-x", Head: head}, snapshot: detachedRepositorySnapshot(head, knownCIRef("close/other")), wantBranch: "close/other", wantKnown: true, refusal: true},
		{name: "detached, CI ref unknown", expected: &Expected{Branch: "close/spec-x", Head: head}, snapshot: detachedRepositorySnapshot(head, unknownCIRef), refusal: true},
		{name: "detached outside CI", expected: &Expected{Branch: "close/spec-x", Head: head}, snapshot: detachedRepositorySnapshot(head, repositoryfacts.CIRefFact{}), refusal: true},
		{name: "branch unresolved, not detached, with a known CI ref", expected: &Expected{Branch: "close/spec-x", Head: head}, snapshot: unresolved, refusal: true},
		{name: "HEAD mismatch with the CI ref matching", expected: &Expected{Branch: "close/spec-x", Head: strings.Repeat("b", 40)}, snapshot: detachedRepositorySnapshot(head, knownCIRef("close/spec-x")), wantBranch: "close/spec-x", wantKnown: true, refusal: true},
		{name: "empty expected branch", expected: &Expected{Branch: "", Head: head}, snapshot: detachedRepositorySnapshot(head, unknownCIRef), opErr: true},
		{name: "malformed expected head", expected: &Expected{Branch: "close/spec-x", Head: "abc"}, snapshot: detachedRepositorySnapshot(head, knownCIRef("close/spec-x")), opErr: true},
		{
			name: "inconsistent computed facts", expected: &Expected{Branch: "main", Head: head},
			snapshot: func() repositoryfacts.Snapshot {
				s := validRepositorySnapshot(head, "main")
				s.Facts.Relationship = "parallel"
				return s
			}(),
			opErr: true,
		},
		{
			name: "inconsistent computed CI ref", expected: &Expected{Branch: "close/spec-x", Head: head},
			snapshot: detachedRepositorySnapshot(head, repositoryfacts.CIRefFact{Known: true, Provider: repositoryfacts.CIProviderGitHub}),
			opErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ResolveExpectedRepositorySnapshot(tt.expected, tt.snapshot)
			switch {
			case tt.opErr:
				if err == nil || IsRefusal(err) {
					t.Fatalf("err = %T %v, want an operational (non-refusal) error", err, err)
				}
			case tt.refusal:
				var refusal *ExpectedRepositoryMismatchRefusal
				if !errors.As(err, &refusal) || !IsRefusal(err) || !errors.Is(err, ErrExpectedRepositoryMismatch) {
					t.Fatalf("err = %T %v, want *ExpectedRepositoryMismatchRefusal", err, err)
				}
				if refusal.ComputedBranch != tt.wantBranch || refusal.BranchKnown != tt.wantKnown {
					t.Fatalf("refusal branch = %q known=%v, want %q known=%v", refusal.ComputedBranch, refusal.BranchKnown, tt.wantBranch, tt.wantKnown)
				}
				if refusal.ComputedHead != head || !refusal.HeadKnown || refusal.Expected != *tt.expected {
					t.Fatalf("refusal = %+v", refusal)
				}
			default:
				if err != nil {
					t.Fatalf("ResolveExpectedRepositorySnapshot: %v", err)
				}
			}
		})
	}
}

// TestResolveExpectedRepositorySnapshot_ParityOutsideCI proves the successor
// decides exactly as ResolveExpectedRepository does whenever no CI ref
// applies (a checked-out branch, or the outside-CI zero value).
func TestResolveExpectedRepositorySnapshot_ParityOutsideCI(t *testing.T) {
	head := strings.Repeat("a", 40)
	snapshots := map[string]repositoryfacts.Snapshot{
		"checked out":         validRepositorySnapshot(head, "main"),
		"detached outside CI": detachedRepositorySnapshot(head, repositoryfacts.CIRefFact{}),
		"checked out, CI ref": func() repositoryfacts.Snapshot {
			s := validRepositorySnapshot(head, "main")
			s.CIRef = knownCIRef("main")
			return s
		}(),
		"detached, CI refused": detachedRepositorySnapshot(head, repositoryfacts.CIRefFact{Reason: repositoryfacts.CIRefReasonNameInvalid}),
	}
	expectations := []*Expected{
		nil,
		{Branch: "main", Head: head},
		{Branch: "feature", Head: head},
		{Branch: "main", Head: strings.Repeat("b", 40)},
		{Branch: "", Head: head},
	}
	for name, snapshot := range snapshots {
		for _, expected := range expectations {
			got := ResolveExpectedRepositorySnapshot(expected, snapshot)
			want := ResolveExpectedRepository(expected, snapshot.Facts)
			if (got == nil) != (want == nil) || IsRefusal(got) != IsRefusal(want) || (got != nil && got.Error() != want.Error()) {
				t.Fatalf("%s, expected %+v: successor = %v, historical = %v", name, expected, got, want)
			}
		}
	}
}

// --- stage 3 through the compiler ----------------------------------------

// TestCompilerStage3DetachedCIRefMatchesExpectedBranch: a detached snapshot
// whose validated CI ref equals the expected branch passes stage 3, so the
// compile proceeds to stage 4 (here an unaccepted target's refusal).
func TestCompilerStage3DetachedCIRefMatchesExpectedBranch(t *testing.T) {
	root := installPolicyFixture(t)
	git, states := unacceptedTargetGitStates()
	repoFacts := stubRepoFactsGatherer{snapshot: detachedRepositorySnapshot(compileHead, knownCIRef("close/spec-x"))}
	c := newCompilerWithPorts(git, states, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
	req := validCompileRequest("spec/example-story")
	req.Expected = &Expected{Branch: "close/spec-x", Head: compileHead}
	_, err := c.Compile(context.Background(), root, req)
	var refusal *AcceptedSpecRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected stage 4's *AcceptedSpecRefusal after stage 3 passed, got %T %v", err, err)
	}
}

func TestCompilerStage3DetachedCIRefRefusals(t *testing.T) {
	tests := []struct {
		name  string
		ciRef repositoryfacts.CIRefFact
	}{
		{"CI ref names another branch", knownCIRef("close/other")},
		{"CI ref unknown", repositoryfacts.CIRefFact{Reason: repositoryfacts.CIRefReasonRemoteTrackingUnresolved}},
		{"outside CI", repositoryfacts.CIRefFact{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := installPolicyFixture(t)
			repoFacts := stubRepoFactsGatherer{snapshot: detachedRepositorySnapshot(compileHead, tt.ciRef)}
			c := newCompilerWithPorts(panicGitReader{}, panicStateResolver{}, defaultAuthorityLoader{}, nil, repoFacts, panicProjectionVerifier{})
			req := validCompileRequest("spec/example-story")
			req.Expected = &Expected{Branch: "close/spec-x", Head: compileHead}
			_, err := c.Compile(context.Background(), root, req)
			var refusal *ExpectedRepositoryMismatchRefusal
			if !errors.As(err, &refusal) || !IsRefusal(err) {
				t.Fatalf("expected *ExpectedRepositoryMismatchRefusal, got %T %v", err, err)
			}
		})
	}
}
