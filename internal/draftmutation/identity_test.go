package draftmutation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
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
	if _, err := ResolveCanonicalIdentity(context.Background(), root, "spec/sample", fakeIdentityReader{root: ".", branch: "design/sample", head: strings.Repeat("a", 40)}); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative checkout error = %v", err)
	}
	dirtyRoot := root + string(filepath.Separator) + ".." + string(filepath.Separator) + filepath.Base(root)
	if _, err := ResolveCanonicalIdentity(context.Background(), root, "spec/sample", fakeIdentityReader{root: dirtyRoot, branch: "design/sample", head: strings.Repeat("a", 40)}); err == nil || !strings.Contains(err.Error(), "clean") {
		t.Fatalf("unclean checkout error = %v", err)
	}
}

// rootReads records the git reads gitx runs on its context.
type rootReads struct {
	mu   sync.Mutex
	argv []string
}

func (r *rootReads) Observe(_ string, args []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.argv = append(r.argv, strings.Join(args, " "))
}

// TestGitIdentityReader_CheckoutRoot proves the real reader's top-level
// lookup: the repository's top level from the top level itself or from
// any directory below it, read through gitx so the observer sees it
// (spec/gitx-recorder-seam dc-3), and an error, never a guessed root,
// outside a repository.
func TestGitIdentityReader_CheckoutRoot(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"store/deep/a.txt": "a\n"}, Message: "seed"}})
	top, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		start   string
		wantErr bool
	}{
		{name: "the top level", start: repo.Dir},
		{name: "a directory below the top level", start: filepath.Join(repo.Dir, "store", "deep")},
		{name: "outside any repository", start: t.TempDir(), wantErr: true},
		{name: "a directory that does not exist", start: filepath.Join(t.TempDir(), "missing"), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reads := &rootReads{}
			got, err := GitIdentityReader{}.CheckoutRoot(gitx.WithObserver(context.Background(), reads), tc.start)
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "draftmutation: resolving Git checkout root") {
					t.Fatalf("CheckoutRoot(%s) = %q, %v; want the checkout-root error", tc.start, got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("CheckoutRoot: %v", err)
			}
			if got != top {
				t.Fatalf("CheckoutRoot = %q, want the top level %q", got, top)
			}
			if len(reads.argv) != 1 || !strings.HasPrefix(reads.argv[0], "rev-parse --show-toplevel") {
				t.Fatalf("observed git reads %q, want the one top-level read through gitx", reads.argv)
			}
		})
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
