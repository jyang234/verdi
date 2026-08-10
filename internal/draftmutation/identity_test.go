package draftmutation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/specstate"
)

type fakeIdentityReader struct {
	root, branch, head string
	err                error
}

func (f fakeIdentityReader) CheckoutRoot(context.Context, string) (string, error) {
	return f.root, f.err
}
func (f fakeIdentityReader) CurrentBranch(context.Context, string) (string, error) {
	return f.branch, f.err
}
func (f fakeIdentityReader) Head(context.Context, string) (string, error) { return f.head, f.err }

func TestIdentityCanonicalSymlinkAndExpectedEquality(t *testing.T) {
	realRoot := t.TempDir()
	linkParent := t.TempDir()
	link := filepath.Join(linkParent, "checkout-link")
	if err := os.Symlink(realRoot, link); err != nil {
		t.Fatal(err)
	}
	reader := fakeIdentityReader{root: link, branch: "design/sample", head: strings.Repeat("a", 40)}
	identity, err := ResolveCanonicalIdentity(context.Background(), link, "spec/sample", reader)
	if err != nil {
		t.Fatalf("ResolveCanonicalIdentity: %v", err)
	}
	wantRoot, _ := filepath.EvalSymlinks(realRoot)
	wantRoot = filepath.ToSlash(wantRoot)
	if identity.Checkout != wantRoot || identity.Branch != "design/sample" || identity.Head != strings.Repeat("a", 40) || identity.Spec != "spec/sample" {
		t.Fatalf("identity = %+v", identity)
	}
	expected := ExpectedIdentity{Checkout: identity.Checkout, Branch: identity.Branch, Head: identity.Head}
	if err := VerifyExpected(identity, expected); err != nil {
		t.Fatalf("VerifyExpected: %v", err)
	}
	for _, mutate := range []func(*ExpectedIdentity){
		func(v *ExpectedIdentity) { v.Checkout += "/other" },
		func(v *ExpectedIdentity) { v.Branch = "design/other" },
		func(v *ExpectedIdentity) { v.Head = strings.Repeat("b", 40) },
	} {
		changed := expected
		mutate(&changed)
		if err := VerifyExpected(identity, changed); err == nil {
			t.Fatalf("VerifyExpected accepted mismatch %+v", changed)
		}
	}
}

func TestIdentityDetachedAndInvalidRoots(t *testing.T) {
	root := t.TempDir()
	reader := fakeIdentityReader{root: root, branch: "", head: strings.Repeat("a", 40)}
	identity, err := ResolveCanonicalIdentity(context.Background(), root, "spec/sample", reader)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Branch != "DETACHED" {
		t.Fatalf("branch = %q", identity.Branch)
	}
	if _, err := ResolveCanonicalIdentity(context.Background(), root, "spec/sample", fakeIdentityReader{root: filepath.Join(root, "missing"), branch: "design/sample", head: strings.Repeat("a", 40)}); err == nil {
		t.Fatal("unresolvable checkout accepted")
	}
}

type fakeStateProjector struct {
	result specstate.Result
	err    error
}

func (f fakeStateProjector) ResolveState(context.Context, string, specstate.Candidate) (specstate.Result, error) {
	return f.result, f.err
}

func TestStateAllowsOnlyMatchingDesignBranchProposal(t *testing.T) {
	identity := testIdentity()
	for _, tt := range []struct {
		name   string
		state  specstate.State
		branch string
		allow  bool
	}{
		{"proposal", specstate.Proposed, "design/sample", true},
		{"accepted", specstate.AcceptedPendingBuild, "design/sample", false},
		{"closed", specstate.Closed, "design/sample", false},
		{"superseded", specstate.Superseded, "design/sample", false},
		{"unproven", specstate.Unproven, "design/sample", false},
		{"detached", specstate.Proposed, "DETACHED", false},
		{"wrong design branch", specstate.Proposed, "design/other", false},
		{"main", specstate.Proposed, "main", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			candidateIdentity := identity
			candidateIdentity.Branch = tt.branch
			result, err := AuthorizeState(context.Background(), "/repo", candidateIdentity, []byte(baseSpec), fakeStateProjector{result: specstate.Result{State: tt.state}})
			if tt.allow && err != nil {
				t.Fatalf("AuthorizeState: %v", err)
			}
			if !tt.allow {
				if err == nil || err.Code != CodeStateForbidden || err.Identity != candidateIdentity {
					t.Fatalf("AuthorizeState = %+v, %v", result, err)
				}
			}
		})
	}
}
