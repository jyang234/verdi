// Built-binary end-to-end tests for `verdi policy adopt --starter`
// (spec/spec-documents ac-10, dc-6; SI-204): buildVerdiBinary + runVerdi
// drive the real compiled binary, mirroring harness_test.go's own
// convention for the newest sibling verb. CI_DEFAULT_BRANCH=main is set
// per test (the other verb tests' env form, e.g.
// TestVocabularyCLI_RenamedStateLabels) since these fixtures carry no
// "origin" remote for specstate.ResolveDefaultBranch to resolve from.
//
// One exception drives runPolicyAdopt in process instead:
// TestPolicyAdopt_PostWriteGitFailuresDiscloseTheWrittenCheckout, which
// forces the verb's git seams to fail — a fault no separate process can
// be made to take deterministically.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/humanartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// adoptCommittedPaths is the complete, sorted `git show --name-only`
// listing the adoption commit must carry: exactly the four starter paths
// the verb itself wrote, never a fifth. Named once because two tests
// assert it — the solo happy path and the pre-staged-index proof — and
// "exactly" is the claim ac-10, the CLI and the workbench guide all make.
const adoptCommittedPaths = ".verdi/constitution/consumers.json\n.verdi/policy/constitution.md\n.verdi/policy/policies/starter.md\n.verdi/policy/profiles/starter-solo.md"

// adoptFixture builds a minimal, real store root (fixturegit) carrying
// nothing under .verdi/policy or .verdi/constitution — every adopt test
// below starts from an unadopted checkout.
func adoptFixture(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n"}, Message: "init store"}})
}

// pinGitCommitIdentity pins, in the environment the child binary
// inherits, the author/committer identity git needs to record the
// adoption commit at all. Every test below that lets the verb reach its
// commit — or reach any step the committer preflight now guards — calls
// it; the one test proving the preflight's own refusal
// (TestPolicyAdopt_NoCommitterIdentityRefusesBeforeWriting) deliberately
// does not.
//
// It answers a requirement of GIT's, entirely separate from the
// local-operator identity the solo profile BINDS as its subject: git
// composes an author and a committer from the GIT_AUTHOR_*/GIT_COMMITTER_*
// environment, then the --local, --global and --system config scopes, and
// finally a name derived from the OS account. Two states break that here,
// and neither says anything about the verb:
//
//   - No identity to fall back on. fixturegit.Build configures
//     user.name/user.email in the fixture's own --local scope, but
//     TestPolicyAdopt_NoLocalIdentityRefusesSolo clears exactly those
//     keys; on a developer's macOS machine git then derives a name from
//     the OS account's full-name field and commits anyway, while a
//     GitHub runner's account has that field empty and git refuses
//     ("fatal: empty ident name (for <runner@...>) not allowed"). That
//     host difference, not the verb, is what reddened CI.
//   - An ambient GIT_AUTHOR_NAME/GIT_COMMITTER_NAME exported EMPTY. Those
//     variables override every config scope even when empty, so a shell
//     (or a gate command masking the host's own identity) that exports
//     them turns the fixture's --local identity back into that same fatal
//     error.
//
// The pinned values are fixturegit's own identity, so nothing observable
// about the resulting commits changes. They are invisible to the
// local-operator binding: readLocalGitIdentity goes through
// gitx.ConfigValue, which reads `git config --local` and never the
// environment.
func pinGitCommitIdentity(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "Verdi Fixture")
	t.Setenv("GIT_AUTHOR_EMAIL", "fixture@verdi.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Verdi Fixture")
	t.Setenv("GIT_COMMITTER_EMAIL", "fixture@verdi.invalid")
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
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir() // no store, no git: every case below must fail before touching either
	for _, args := range [][]string{{"policy"}, {"policy", "adopt"}, {"policy", "frobnicate"}, {"policy", "adopt", "--starter", "--profile", "solo", "--profile", "team"}, {"policy", "adopt", "--starter", "--profile", "high-assurance"}, {"policy", "adopt", "--starter", "--profile=frobnicate"}, {"policy", "adopt", "--starter", "--owner"}, {"policy", "adopt", "--starter", "--profile", "team"}} {
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
	pinGitCommitIdentity(t)
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
	if names != adoptCommittedPaths {
		t.Fatalf("committed paths:\n%s\nwant:\n%s", names, adoptCommittedPaths)
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

// TestPolicyAdopt_PreStagedUnrelatedChangeStaysOutOfTheAdoptCommit proves
// the word "exactly" in ac-10's "commits exactly those paths" — and in the
// policy setup guide's own browser-facing "commits exactly those files"
// (internal/workbench/boardshellrender.go) — against the state that used
// to falsify it: an operator with an unrelated change already staged.
//
// A bare `git commit -m <msg>` records the WHOLE index, so the adoption
// commit carried five paths instead of four and the isolation that is
// policy/adopt's entire purpose was silently lost. The pathspec form
// (gitx.CreateCommitPaths) records exactly the four and leaves the
// operator's own staged work in the index, untouched and uncommitted.
func TestPolicyAdopt_PreStagedUnrelatedChangeStaysOutOfTheAdoptCommit(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := adoptFixture(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	pinGitCommitIdentity(t)

	if err := os.WriteFile(filepath.Join(repo.Dir, "notes.md"), []byte("unrelated work in progress\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, repo.Dir, "add", "notes.md")

	code, stdout, stderr := runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter")
	if code != 0 {
		t.Fatalf("code %d\n%s\n%s", code, stdout, stderr)
	}
	if names := strings.TrimSpace(gitOutput(t, repo.Dir, "show", "--name-only", "--format=", "HEAD")); names != adoptCommittedPaths {
		t.Fatalf("the adoption commit records:\n%s\nwant exactly the four starter paths:\n%s", names, adoptCommittedPaths)
	}
	// The operator's own staged change survives the verb: still staged on
	// policy/adopt, still absent from the commit it was staged before.
	if staged := strings.TrimSpace(gitOutput(t, repo.Dir, "diff", "--cached", "--name-only")); staged != "notes.md" {
		t.Fatalf("staged after adopt = %q, want the operator's own notes.md still staged and nothing else", staged)
	}
	if br := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD")); br != "policy/adopt" {
		t.Fatalf("branch = %s", br)
	}
	if strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "main")) != repo.Head {
		t.Fatal("main moved")
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
	pinGitCommitIdentity(t)

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
	pinGitCommitIdentity(t)

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
// branch — while the team profile, which binds no LOCAL identity of its
// own, still succeeds against that same checkout.
//
// "Needs no local identity" is the whole team claim, and it is narrower
// than "needs no identity": git still has to name an author and a
// committer for the adoption commit. That requirement is supplied
// explicitly below (pinGitCommitIdentity) rather than borrowed from the
// host, because a host that happens to supply one — macOS derives a name
// from the OS account — makes this test pass for a reason CI does not
// have.
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

	// AFTER the solo assertions: the solo refusal above must see a
	// checkout with no repo-local identity, which is exactly what it still
	// sees — these variables live in the environment, and the local-operator
	// read is `git config --local` only (gitx.ConfigValue), which never
	// consults the environment. They give git what IT needs to author the
	// team profile's commit.
	pinGitCommitIdentity(t)

	code, _, stderr = runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter", "--profile", "team", "--owner", "platform-team")
	if code != 0 {
		t.Fatalf("team adopt after clearing identity: code %d stderr %s", code, stderr)
	}
	// The team profile committed with the identity the environment
	// supplied, never one derived from the cleared --local scope.
	if got := strings.TrimSpace(gitOutput(t, repo.Dir, "show", "-s", "--format=%an <%ae>", "HEAD")); got != "Verdi Fixture <fixture@verdi.invalid>" {
		t.Fatalf("adoption commit author = %q, want the environment-supplied identity", got)
	}
}

// TestPolicyAdopt_NoCommitterIdentityRefusesBeforeWriting proves the
// committer preflight: when git can mint no identity at all, the refusal
// happens BEFORE the branch is cut and before a single file is written,
// and says so.
//
// Without the preflight the verb learned this only from its very last
// step — after composing, cutting policy/adopt, writing four files and
// staging them — and the operator was left on a branch they never asked
// to be on, holding four uncommitted files, for a fault git could have
// been asked about up front. R-W4-2's posture is that a refusal leaves
// the checkout untouched; this is the state that used to falsify it, and
// it is not exotic: a CI runner configures no identity in any scope and
// has no OS full-name field to derive one from.
//
// The team profile is what this drives, because the solo profile refuses
// earlier and for its own reason — the LOCAL identity it binds as its
// subject is missing too (TestPolicyAdopt_NoLocalIdentityRefusesSolo's
// first half, whose message must keep winning for solo).
func TestPolicyAdopt_NoCommitterIdentityRefusesBeforeWriting(t *testing.T) {
	bin := buildVerdiBinary(t) // built BEFORE the identity is masked: `go build` is not what is under test
	repo := adoptFixture(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	clearLocalOperatorGitIdentity(t, repo.Dir)
	// Every scope git could mint an identity from, closed: no global or
	// system config, and names exported EMPTY rather than left unset (an
	// unset name is derived from the OS account on a developer's machine,
	// which is exactly the host difference that hid this defect).
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_COMMITTER_NAME", "")

	code, stdout, stderr := runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter", "--profile", "team", "--owner", "platform-team")
	if code != 2 {
		t.Fatalf("code %d, want 2 (operational)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	for _, want := range []string{
		"policy adopt: git cannot mint a committer identity for the adoption commit",
		"configure user.name and user.email, or set GIT_COMMITTER_NAME/GIT_COMMITTER_EMAIL",
		"nothing was written and no branch was cut",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr)
		}
	}
	// The disclosure is true, not merely printed.
	for _, rel := range []string{".verdi/policy", ".verdi/constitution"} {
		if _, err := os.Stat(filepath.Join(repo.Dir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("%s exists, but the refusal said nothing was written (stat err=%v)", rel, err)
		}
	}
	if strings.TrimSpace(gitOutput(t, repo.Dir, "branch", "--list", "policy/adopt")) != "" {
		t.Fatal("the refusal said no branch was cut, but policy/adopt exists")
	}
	if br := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD")); br != "main" {
		t.Fatalf("branch = %s, want the checkout left on main", br)
	}
	if gitOutput(t, repo.Dir, "status", "--porcelain") != "" {
		t.Fatal("the refusal left the tree dirty")
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
	pinGitCommitIdentity(t)
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
	pinGitCommitIdentity(t)

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

// TestPolicyAdopt_InlineFlagValuesAdoptTheTeamProfile proves the
// documented inline `--flag=value` spelling is real and not merely
// documented: parsePolicyAdoptFlags' strings.Cut branch carries both
// --profile=team and --owner=platform-team all the way into the rendered
// artifacts. Its refusal twin (`--profile=frobnicate`) is in
// TestPolicyAdopt_UsageAndFlagShape's table.
func TestPolicyAdopt_InlineFlagValuesAdoptTheTeamProfile(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := adoptFixture(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	pinGitCommitIdentity(t)

	code, stdout, stderr := runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter", "--profile=team", "--owner=platform-team")
	if code != 0 {
		t.Fatalf("code %d\n%s\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "design_assistance mode proposal-only") {
		t.Fatalf("stdout missing the proposal-only mode disclosure:\n%s", stdout)
	}
	store, err := policyauthority.Load(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if sp := store.Profiles["starter-team"]; sp == nil || len(sp.Profile.RoleMappings) != 0 {
		t.Fatalf("team profile = %+v, want the team profile with zero role mappings", sp)
	}
	// The inline --owner value reached the rendered artifacts, so the
	// inline spelling is parsed rather than merely tolerated.
	data, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "policy", "constitution.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "platform-team") {
		t.Fatalf("the inline --owner value never reached the constitution:\n%s", data)
	}
}

// TestPolicyAdopt_WriteFailureDisclosesWhatLandedAndWhereTheCheckoutIs
// proves the write-failure refusal discloses the state it leaves behind
// rather than only the error: every path that landed, and the fact that
// the checkout is now on policy/adopt.
//
// Reaching a PARTIAL write through the verb needs a root that Compose
// accepts but Write cannot finish. `.verdi/policy/policies` as a regular
// file (the obvious shape) is unreachable here — refuseAdopted stats
// `.verdi/policy` itself and refuses the whole adoption at exit 1 long
// before Write runs. An existing-but-unwritable `.verdi/constitution` is
// reachable: refuseAdopted only stats `.verdi/constitution/consumers.json`
// inside it (absent; 0o555 still permits the stat), so Compose passes,
// the branch is cut, the first three artifacts land, and only the fourth
// write fails.
//
// This covers one of the verb's four post-checkout refusals; the other
// three have their own tests —
// TestPolicyAdopt_PostWriteGitFailuresDiscloseTheWrittenCheckout for
// the AddPaths and CreateCommit pair, and
// TestPolicyAdopt_PostCheckoutComposeFailureDisclosesTheBranch for the
// non-ErrAlreadyAdopted re-prove. None is left to inspection.
func TestPolicyAdopt_WriteFailureDisclosesWhatLandedAndWhereTheCheckoutIs(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("DISCLOSURE: running as root — os.Chmod(0o555) does not restrict root's own writes, so this permission-based partial-write path cannot be exercised under this user")
	}
	bin := buildVerdiBinary(t)
	repo := adoptFixture(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	pinGitCommitIdentity(t)

	invDir := filepath.Join(repo.Dir, ".verdi", "constitution")
	if err := os.MkdirAll(invDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(invDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(invDir, 0o755) }) // restore so t.TempDir()'s own cleanup can remove it

	code, _, stderr := runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter")
	if code != 2 {
		t.Fatalf("code %d, want 2 (operational)\n%s", code, stderr)
	}
	landed := []string{".verdi/policy/constitution.md", ".verdi/policy/profiles/starter-solo.md", ".verdi/policy/policies/starter.md"}
	for _, rel := range landed {
		if want := "policy adopt: wrote " + rel + " before the failure"; !strings.Contains(stderr, want) {
			t.Fatalf("stderr does not disclose the landed path %q:\n%s", want, stderr)
		}
	}
	for _, want := range []string{".verdi/constitution/consumers.json", "the checkout is on policy/adopt"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr)
		}
	}
	// The disclosure is true, not merely printed.
	if br := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD")); br != "policy/adopt" {
		t.Fatalf("branch = %s, but the refusal said the checkout is on policy/adopt", br)
	}
	if strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "policy/adopt")) != repo.Head {
		t.Fatal("a failed write still produced a commit on policy/adopt")
	}
	for _, rel := range landed {
		if _, err := os.Stat(filepath.Join(repo.Dir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s was disclosed as written but is not on disk: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "constitution", "consumers.json")); !os.IsNotExist(err) {
		t.Fatalf("the inventory Write failed on exists anyway (stat err=%v)", err)
	}
}

// TestPolicyAdopt_PostWriteGitFailuresDiscloseTheWrittenCheckout drives
// runPolicyAdopt in process with the verb's own git seams forced to fail,
// the house pattern for exactly this (close_test.go's closeAddPaths /
// closeCreateCommit overrides). Both post-write refusals must say where
// the operator now is and what exists there: a bare wrapped git error
// would leave them on a branch they did not ask for, holding four
// uncommitted files, with nothing in the output saying so.
func TestPolicyAdopt_PostWriteGitFailuresDiscloseTheWrittenCheckout(t *testing.T) {
	for _, tc := range []struct {
		name    string
		install func(t *testing.T)
		wantErr string
	}{
		{
			name: "AddPaths",
			install: func(t *testing.T) {
				restore := policyAdoptAddPaths
				policyAdoptAddPaths = func(context.Context, string, ...string) error {
					return fmt.Errorf("forced stage failure")
				}
				t.Cleanup(func() { policyAdoptAddPaths = restore })
			},
			wantErr: "forced stage failure",
		},
		{
			name: "CreateCommitPaths",
			install: func(t *testing.T) {
				restore := policyAdoptCommit
				policyAdoptCommit = func(_ context.Context, _, _ string, paths ...string) (string, error) {
					// The seam also witnesses WHAT the verb asks git to
					// record: exactly the four paths it wrote, never a
					// bare whole-index commit.
					if len(paths) != 4 {
						return "", fmt.Errorf("commit seam received %d paths (%v), want exactly the four the verb wrote", len(paths), paths)
					}
					return "", fmt.Errorf("forced commit failure")
				}
				t.Cleanup(func() { policyAdoptCommit = restore })
			},
			wantErr: "forced commit failure",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := adoptFixture(t)
			t.Setenv("CI_DEFAULT_BRANCH", "main")
			pinGitCommitIdentity(t)
			tc.install(t)

			var stdout, stderr bytes.Buffer
			code := runPolicyAdopt(context.Background(), repo.Dir, policyAdoptOptions{profile: governanceprincipal.ClassSolo, owner: "local-operator"}, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("code %d, want 2 (operational)\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
			}
			for _, want := range []string{tc.wantErr, "the checkout is on policy/adopt with the four files written but not committed"} {
				if !strings.Contains(stderr.String(), want) {
					t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
				}
			}
			// The disclosure is true: that branch, those four files, no commit.
			if br := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD")); br != "policy/adopt" {
				t.Fatalf("branch = %s, but the refusal said the checkout is on policy/adopt", br)
			}
			if strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "policy/adopt")) != repo.Head {
				t.Fatal("a refused adopt committed onto policy/adopt anyway")
			}
			for _, rel := range []string{".verdi/policy/constitution.md", ".verdi/policy/profiles/starter-solo.md", ".verdi/policy/policies/starter.md", ".verdi/constitution/consumers.json"} {
				if _, err := os.Stat(filepath.Join(repo.Dir, filepath.FromSlash(rel))); err != nil {
					t.Fatalf("%s was disclosed as written but is not on disk: %v", rel, err)
				}
			}
		})
	}
}

// TestPolicyAdopt_PostCheckoutComposeFailureDisclosesTheBranch covers the
// post-checkout re-prove's OTHER refusal — the non-ErrAlreadyAdopted one,
// which until now was implemented but never exercised. The default branch
// carries a synthesizing template override the current checkout is simply
// behind: the FIRST Compose (old working tree, no override) passes and a
// branch is cut, then checking policy/adopt out from main brings the
// override in and the second Compose refuses. Nothing is written, so the
// refusal must say that AND say where the checkout now is.
func TestPolicyAdopt_PostCheckoutComposeFailureDisclosesTheBranch(t *testing.T) {
	bin := buildVerdiBinary(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	pinGitCommitIdentity(t)

	canon, err := designscaffold.Canonical(humanartifact.StarterPolicyTemplate)
	if err != nil {
		t.Fatal(err)
	}
	synth := strings.Replace(string(canon), "instructions: []", `instructions: ["Always pass."]`, 1)
	if synth == string(canon) {
		t.Fatal("test setup: instruction replacement did not change the canonical bytes")
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n"}, Message: "init store"},
		{Files: map[string]string{".verdi/templates/policy-starter.md": synth}, Message: "main gains a synthesizing policy-starter override"},
	})
	gitOutput(t, repo.Dir, "checkout", "--quiet", repo.Heads[0])
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "templates")); !os.IsNotExist(err) {
		t.Fatalf("test setup: the pre-override checkout unexpectedly carries .verdi/templates (stat err=%v)", err)
	}

	code, _, stderr := runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter")
	if code != 2 {
		t.Fatalf("code %d, want 2 (operational)\n%s", code, stderr)
	}
	for _, want := range []string{"instructions", "the checkout is on policy/adopt with nothing written"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr)
		}
	}
	if br := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD")); br != "policy/adopt" {
		t.Fatalf("branch = %s, but the refusal said the checkout is on policy/adopt", br)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "policy")); !os.IsNotExist(err) {
		t.Fatalf("the refusal said nothing was written, but .verdi/policy exists (stat err=%v)", err)
	}
}
