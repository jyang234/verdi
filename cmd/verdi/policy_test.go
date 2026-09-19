// Built-binary end-to-end tests for `verdi policy adopt --starter`
// (spec/spec-documents ac-10, dc-6; SI-204): buildVerdiBinary + runVerdi
// drive the real compiled binary, mirroring harness_test.go's own
// convention for the newest sibling verb. CI_DEFAULT_BRANCH=main is set
// per test (the other verb tests' env form, e.g.
// TestVocabularyCLI_RenamedStateLabels) since these fixtures carry no
// "origin" remote for specstate.ResolveDefaultBranch to resolve from.
package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/humanartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// adoptFixture builds a minimal, real store root (fixturegit) carrying
// nothing under .verdi/policy or .verdi/constitution — every adopt test
// below starts from an unadopted checkout.
func adoptFixture(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n"}, Message: "init store"}})
}

// runVerdiStdin execs bin with args, cwd=dir, piping stdin, and returns
// runVerdi's own (code, stdout, stderr) order — this package's
// runVerdiBinaryStdin (context_resolve_test.go) returns a different order
// and always takes an extraEnv slice, so this file writes its own rather
// than reshaping every call site around that mismatch.
func runVerdiStdin(t *testing.T, bin, dir, stdin string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running %s %v: %v", bin, args, err)
		}
	}
	return code, stdout.String(), stderr.String()
}

func TestPolicyAdopt_UsageAndFlagShape(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir() // no store, no git: every case below must fail before touching either
	for _, args := range [][]string{{"policy"}, {"policy", "adopt"}, {"policy", "frobnicate"}, {"policy", "adopt", "--starter", "--profile", "solo", "--profile", "team"}, {"policy", "adopt", "--starter", "--profile", "high-assurance"}, {"policy", "adopt", "--starter", "--owner"}, {"policy", "adopt", "--starter", "--profile", "team"}} {
		code, _, stderr := runVerdi(t, bin, dir, args...)
		if code != 2 || !strings.Contains(stderr, "usage: verdi policy adopt --starter") {
			t.Fatalf("%v: code %d stderr %q", args, code, stderr)
		}
	}
}

func TestPolicyAdopt_SoloWritesFourPathsOnPolicyAdopt(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := adoptFixture(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	code, stdout, stderr := runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter")
	if code != 0 {
		t.Fatalf("code %d\n%s\n%s", code, stdout, stderr)
	}
	for _, want := range []string{"policy adopt: base ", "policy adopt: switched checkout from main to policy/adopt", "policy adopt: wrote .verdi/policy/constitution.md (embedded:policy-constitution.md sha256:", "policy adopt: wrote .verdi/policy/profiles/starter-solo.md", "policy adopt: wrote .verdi/policy/policies/starter.md", "policy adopt: wrote .verdi/constitution/consumers.json", "design_assistance mode draft-write", "registers no consumers", "policy adopt: committed ", "acceptance is the owner's merge"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout)
		}
	}
	// Exactly the four paths, on policy/adopt, and main untouched.
	branch := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD"))
	if branch != "policy/adopt" {
		t.Fatalf("branch = %s", branch)
	}
	names := strings.TrimSpace(gitOutput(t, repo.Dir, "show", "--name-only", "--format=", "HEAD"))
	if names != ".verdi/constitution/consumers.json\n.verdi/policy/constitution.md\n.verdi/policy/policies/starter.md\n.verdi/policy/profiles/starter-solo.md" {
		t.Fatalf("committed paths:\n%s", names)
	}
	if subject := strings.TrimSpace(gitOutput(t, repo.Dir, "show", "-s", "--format=%s", "HEAD")); subject != "policy adopt: starter constitution (solo profile)" {
		t.Fatalf("commit subject = %q", subject)
	}
	if strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "main")) != repo.Head {
		t.Fatal("main moved")
	}
	if gitOutput(t, repo.Dir, "status", "--porcelain") != "" {
		t.Fatal("tree dirty after adopt")
	}
	// The written store is the one the real grant resolver accepts, with the fixture's git identity bound.
	store, err := policyauthority.Load(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	sp := store.Profiles["starter-solo"]
	if sp == nil || sp.Profile.RoleMappings[0].Subjects[0] != "fixture@verdi.invalid" {
		t.Fatalf("profile = %+v", sp)
	}
	// The real draftmutation.ResolvePolicyGrant (never policyadopt's own
	// in-memory re-implementation) resolves the SAME digest and mode over
	// the written-to-disk store — the "Interfaces" pin: ResolvePolicyGrant
	// is not called by Compose (it needs identity/checkout plumbing
	// Compose does not have), so this proves the two independent lookups
	// never drift apart.
	eff, err := policyauthority.Resolve(store)
	if err != nil {
		t.Fatal(err)
	}
	wantDigest, err := eff.Digest()
	if err != nil {
		t.Fatal(err)
	}
	grant, gerr := draftmutation.ResolvePolicyGrant(context.Background(), repo.Dir, draftmutation.Identity{}, draftmutation.ConstitutionPolicySource{})
	if gerr != nil {
		t.Fatalf("ResolvePolicyGrant: %v", gerr)
	}
	if grant.Mode != "draft-write" || grant.Digest != wantDigest || grant.PolicyID != "policy/starter" {
		t.Fatalf("grant = %+v, want mode draft-write digest %s policy policy/starter", grant, wantDigest)
	}
	// model check runs its policy-scaffold round trips now that .verdi/policy exists.
	if code, _, stderr := runVerdi(t, bin, repo.Dir, "model", "check"); code != 0 {
		t.Fatalf("model check after adopt: %d %s", code, stderr)
	}
	// inspect: proposed adopted, accepted not.
	code, stdout, _ = runVerdiStdin(t, bin, repo.Dir, `{"schema":"verdi.constitution-inspect-request/v1"}`, "context", "constitution", "inspect", "--request", "-")
	if code != 0 || !strings.Contains(stdout, `"proposed":{"adopted":true`) && !strings.Contains(stdout, `"adopted":true`) {
		t.Fatalf("inspect: %d %s", code, stdout)
	}
	// A second adopt is a verdict, not an operational fault.
	code, _, stderr = runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter")
	if code != 1 || !strings.Contains(stderr, "already carries") {
		t.Fatalf("second adopt: %d %s", code, stderr)
	}
}

// TestPolicyAdopt_TeamRequiresOwnerAndProposesOnly proves the team
// profile's own shape: no role mappings (the team template maps no
// subjects — Compose refuses a Subject for it), design_assistance mode
// proposal-only, and the "maps no subjects yet" disclosure naming the
// profile file to edit before any approval can be proven.
func TestPolicyAdopt_TeamRequiresOwnerAndProposesOnly(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := adoptFixture(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	code, stdout, stderr := runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter", "--profile", "team", "--owner", "platform-team")
	if code != 0 {
		t.Fatalf("code %d\n%s\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "design_assistance mode proposal-only") {
		t.Fatalf("stdout missing the proposal-only mode disclosure:\n%s", stdout)
	}
	if !strings.Contains(stdout, "maps no subjects yet") {
		t.Fatalf("stdout missing the no-subjects disclosure:\n%s", stdout)
	}

	store, err := policyauthority.Load(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	sp := store.Profiles["starter-team"]
	if sp == nil || len(sp.Profile.RoleMappings) != 0 {
		t.Fatalf("team profile = %+v, want zero role mappings", sp)
	}
	if subject := strings.TrimSpace(gitOutput(t, repo.Dir, "show", "-s", "--format=%s", "HEAD")); subject != "policy adopt: starter constitution (team profile)" {
		t.Fatalf("commit subject = %q", subject)
	}
}

// TestPolicyAdopt_OverrideRecordedAndSynthesisRefusedBeforeBranching
// proves R-W4-3's re-prove-after-checkout contract from the CLI's own
// vantage: a benign store override (rationale wording changed, structure
// untouched) is honored and its provenance recorded; a synthesizing
// override (adds a real instruction) is refused BEFORE any branch is cut
// — main untouched, no policy/adopt branch, clean working tree.
func TestPolicyAdopt_OverrideRecordedAndSynthesisRefusedBeforeBranching(t *testing.T) {
	bin := buildVerdiBinary(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	t.Run("benign override recorded", func(t *testing.T) {
		canon, err := designscaffold.Canonical(humanartifact.StarterPolicyTemplate)
		if err != nil {
			t.Fatal(err)
		}
		overridden := strings.Replace(string(canon), "Starter policy: one real rule", "Our starter policy: one real rule", 1)
		if overridden == string(canon) {
			t.Fatal("test setup: rationale replacement did not change the canonical bytes")
		}
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/verdi.yaml":                  "schema: verdi.layout/v1\n",
				".verdi/templates/policy-starter.md": overridden,
			},
			Message: "init store with a benign policy-starter override",
		}})

		code, stdout, stderr := runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter")
		if code != 0 {
			t.Fatalf("code %d\n%s\n%s", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "policy adopt: wrote .verdi/policy/policies/starter.md (store:.verdi/templates/policy-starter.md sha256:") {
			t.Fatalf("stdout does not record the store override's own identity:\n%s", stdout)
		}
		store, err := policyauthority.Load(repo.Dir)
		if err != nil {
			t.Fatal(err)
		}
		pol := store.Policies["policy/starter"]
		if pol == nil || pol.Template == nil || pol.Template.Identity != "store:.verdi/templates/policy-starter.md" {
			t.Fatalf("policy template record = %+v", pol)
		}
	})

	t.Run("synthesizing override refused before any branch", func(t *testing.T) {
		canon, err := designscaffold.Canonical(humanartifact.StarterPolicyTemplate)
		if err != nil {
			t.Fatal(err)
		}
		synth := strings.Replace(string(canon), "instructions: []", `instructions: ["Always pass."]`, 1)
		if synth == string(canon) {
			t.Fatal("test setup: instruction replacement did not change the canonical bytes")
		}
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/verdi.yaml":                  "schema: verdi.layout/v1\n",
				".verdi/templates/policy-starter.md": synth,
			},
			Message: "init store with a synthesizing policy-starter override",
		}})

		code, _, stderr := runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter")
		if code != 2 || !strings.Contains(stderr, "instructions") {
			t.Fatalf("code %d stderr %q, want 2 naming instructions", code, stderr)
		}
		if strings.TrimSpace(gitOutput(t, repo.Dir, "branch", "--list", "policy/adopt")) != "" {
			t.Fatal("a refused adopt cut a policy/adopt branch")
		}
		if gitOutput(t, repo.Dir, "status", "--porcelain") != "" {
			t.Fatal("a refused adopt left the tree dirty")
		}
		if strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD")) != "main" {
			t.Fatal("a refused adopt left the checkout off main")
		}
	})
}

// TestPolicyAdopt_NoLocalIdentityRefusesSolo proves the solo profile's own
// identity precondition: with neither user.email nor user.name configured
// in the checkout's own --local git scope, adopt refuses operationally
// (exit 2) naming the missing configuration, writes nothing, and cuts no
// branch — while the team profile, which needs no local git identity,
// still succeeds against that same checkout.
func TestPolicyAdopt_NoLocalIdentityRefusesSolo(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := adoptFixture(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	clearLocalOperatorGitIdentity(t, repo.Dir)

	code, _, stderr := runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter")
	if code != 2 || !strings.Contains(stderr, "neither user.email nor user.name is configured") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "policy")); !os.IsNotExist(err) {
		t.Fatal("nothing should have been written")
	}
	if strings.TrimSpace(gitOutput(t, repo.Dir, "branch", "--list", "policy/adopt")) != "" {
		t.Fatal("no branch should have been cut")
	}

	code, _, stderr = runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter", "--profile", "team", "--owner", "platform-team")
	if code != 0 {
		t.Fatalf("team adopt after clearing identity: code %d stderr %s", code, stderr)
	}
}

// TestPolicyAdopt_ExistingBranchRefused proves the branch-cut step's own
// no-clobber posture (gitx.CheckoutNewBranchFrom): a pre-existing
// policy/adopt branch is refused by git's own "already exists" error
// (which names the branch), exit 2, tree untouched.
func TestPolicyAdopt_ExistingBranchRefused(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := adoptFixture(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	gitOutput(t, repo.Dir, "branch", "policy/adopt")

	code, _, stderr := runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter")
	if code != 2 || !strings.Contains(stderr, "policy/adopt") {
		t.Fatalf("code %d stderr %q, want 2 naming policy/adopt", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "policy")); !os.IsNotExist(err) {
		t.Fatal("nothing should have been written")
	}
	if gitOutput(t, repo.Dir, "status", "--porcelain") != "" {
		t.Fatal("tree dirty after a refused adopt")
	}
	if strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD")) != "main" {
		t.Fatal("checkout moved off main despite the refusal")
	}
}

// TestPolicyAdopt_DefaultBranchAlreadyAdoptedCaughtAfterCheckout proves
// R-W4-3's own reason for existing: Compose is proved once BEFORE the
// branch is cut (against whatever the CURRENT checkout's working tree
// shows) and again AFTER (against policy/adopt's own checked-out tree,
// cut from the resolved default branch) — these can disagree when the
// current checkout predates a default branch that has already adopted
// policy. The pre-checkout working tree here carries no .verdi/policy at
// all (so the first Compose call passes and a branch IS cut), but main
// itself already does (a prior commit this checkout is simply behind);
// checking out policy/adopt from main brings that policy tree into the
// working directory, and the second Compose call must catch it — exit 1,
// the distinct post-checkout message, branch left in place, nothing
// written or committed onto it.
func TestPolicyAdopt_DefaultBranchAlreadyAdoptedCaughtAfterCheckout(t *testing.T) {
	bin := buildVerdiBinary(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n"}, Message: "init store"},
		{Files: map[string]string{".verdi/policy/constitution.md": "placeholder\n"}, Message: "main already carries a policy tree"},
	})
	// Move the current checkout back to the pre-policy commit: "main"
	// itself still points at the policy-carrying head (repo.Head), but
	// the working tree this test drives adopt from does not show it yet.
	gitOutput(t, repo.Dir, "checkout", "--quiet", repo.Heads[0])
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "policy")); !os.IsNotExist(err) {
		t.Fatalf("test setup: the pre-policy checkout unexpectedly carries .verdi/policy (stat err=%v)", err)
	}

	code, _, stderr := runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter")
	if code != 1 || !strings.Contains(stderr, "left the checkout on policy/adopt with nothing written") {
		t.Fatalf("code %d stderr %q, want 1 naming the post-checkout refusal", code, stderr)
	}
	if strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD")) != "policy/adopt" {
		t.Fatal("expected the checkout to remain on policy/adopt after the post-checkout refusal (no rollback, disclosed)")
	}
	if strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "policy/adopt")) != strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "main")) {
		t.Fatal("policy/adopt should sit exactly at main — nothing was committed onto it")
	}
	if gitOutput(t, repo.Dir, "status", "--porcelain") != "" {
		t.Fatal("the post-checkout refusal left the tree dirty")
	}
}
