package specimport

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// fakePolicySource is a fixed-value draftmutation.PolicySource — the same
// injection seam draftmutation's own tests use — so this package's Apply
// tests never need a policy fixture committed into the Git repository
// under test: Service.Policy is consulted directly, never re-derived from
// root.
type fakePolicySource struct {
	policy *policyauthority.EffectivePolicy
	err    error
}

func (f fakePolicySource) ResolveEffectivePolicy(context.Context, string) (*policyauthority.EffectivePolicy, error) {
	return f.policy, f.err
}

// resolvedPolicyFor loads the existing internal/policyauthority fixture
// store (the same one draftmutation/policy_test.go and internal/designapp's
// tests already reuse) with go-toolchain.md's design_assistance mode
// overridden to mode, and resolves it.
func resolvedPolicyFor(t *testing.T, mode string) *policyauthority.EffectivePolicy {
	t.Helper()
	source := filepath.Join("..", "policyauthority", "testdata", "store")
	dest := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if entry.Name() == "go-toolchain.md" {
			data = bytes.Replace(data, []byte("mode: proposal-only"), []byte("mode: "+mode), 1)
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := policyauthority.Load(dest)
	if err != nil {
		t.Fatalf("policyauthority.Load: %v", err)
	}
	policy, err := policyauthority.Resolve(store)
	if err != nil {
		t.Fatalf("policyauthority.Resolve: %v", err)
	}
	return policy
}

// TestApply_DelegatedAgent_ProposalOnly_PolicyForbidden proves Apply
// reuses draftmutation's existing design_assistance mode matrix unchanged:
// a delegated agent is refused under proposal-only, and no branch is
// created.
func TestApply_DelegatedAgent_ProposalOnly_PolicyForbidden(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	svc.Policy = fakePolicySource{policy: resolvedPolicyFor(t, "proposal-only")}
	req := evidencedRequest()

	preview, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Apply(context.Background(), repo.Dir, req, preview.Digest, testAgent(t))
	if !errors.Is(err, ErrPolicyForbidden) {
		t.Fatalf("Apply(delegated agent, proposal-only) = %v, want ErrPolicyForbidden", err)
	}
	if exists, _ := gitx.HasLocalBranch(context.Background(), repo.Dir, "design/sample-feature"); exists {
		t.Fatal("a branch was created despite the policy-forbidden refusal")
	}
}

// TestApply_BrowserHuman_NoAdoptedPolicy_Succeeds proves a human with no
// adopted assistance policy can complete import (spec-import-contract.md:
// "A human with no adopted assistance policy can complete import in the
// browser"), reusing AuthorizeBrowserHuman's explicit not-applicable arm —
// never a broadened allowance for any other actor.
func TestApply_BrowserHuman_NoAdoptedPolicy_Succeeds(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	svc.Policy = fakePolicySource{err: policyauthority.ErrNotAdopted}
	req := evidencedRequest()

	human, err := draftmutation.NewUnauthenticatedHuman()
	if err != nil {
		t.Fatal(err)
	}
	preview, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.Apply(context.Background(), repo.Dir, req, preview.Digest, human)
	if err != nil {
		t.Fatalf("Apply(browser-human, not adopted): %v", err)
	}
	if result.Status != StatusCreated {
		t.Fatalf("Status = %q, want %q", result.Status, StatusCreated)
	}
}

// TestApply_UnsealedActor_ActorForbidden proves a zero-value (never
// adapter-minted) Actor is refused, never silently treated as a valid
// delegated agent.
func TestApply_UnsealedActor_ActorForbidden(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()

	preview, err := svc.Preview(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Apply(context.Background(), repo.Dir, req, preview.Digest, draftmutation.Actor{})
	if !errors.Is(err, ErrActorForbidden) {
		t.Fatalf("Apply(unsealed actor) = %v, want ErrActorForbidden", err)
	}
}
