package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/specstate"
)

// buildObligationAuthorRepo builds a one-layer fixturegit repo carrying
// obligationSeamStoryCleanMD (acceptobligation_test.go) plus its
// implements-edge target and any extra files the caller supplies (a
// pre-existing obligation, to construct the "already frozen" and
// "regenerate" scenarios).
func buildObligationAuthorRepo(t *testing.T, extra map[string]string) *fixturegit.Repo {
	t.Helper()
	files := map[string]string{
		".verdi/verdi.yaml":                        phase7ManifestYAML,
		".gitattributes":                           phase7GitAttributes,
		".verdi/specs/active/some-feature/spec.md": someFeatureMD,
		".verdi/specs/active/widget-story/spec.md": obligationSeamStoryCleanMD,
	}
	for k, v := range extra {
		files[k] = v
	}
	return fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "init store with widget-story draft"}})
}

// TestRunObligationAuthor_Create is the CREATE case: no obligation yet at
// the convention path, no frozen ancestor (diffBase == "", the "cannot
// prove frozen" case, or diffBase pointing at a commit that never had the
// file) — the verb writes a fresh, decodable, unauthored scaffold.
func TestRunObligationAuthor_Create(t *testing.T) {
	repo := buildObligationAuthorRepo(t, nil)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runObligationAuthor(ctx, repo.Dir, "spec/widget-story", "ac-1", "static", "", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runObligationAuthor(create) = %d, want 0; stderr=%s", got, stderr.String())
	}

	path := obligationPathFor(repo.Dir, "ac-1", "static")
	ob, body := readObligation(t, path)
	if ob.ForKind != artifact.EvidenceStatic {
		t.Errorf("for_kind = %q, want static", ob.ForKind)
	}
	assertUnresolvedObligationQuality(t, ob)
	if len(ob.Owners) != 1 || ob.Owners[0] != "platform-team" {
		t.Errorf("owners = %v, want [platform-team] (copied verbatim from the story spec)", ob.Owners)
	}
	if !contains(string(body), "verdi:obligation-unauthored") {
		t.Errorf("body does not carry the unauthored marker:\n%s", body)
	}
	if !contains(stdout.String(), "scaffolded") {
		t.Errorf("stdout = %q, want it to say scaffolded", stdout.String())
	}
}

// TestRunObligationAuthor_Regenerate proves pre-freeze authoring is never a
// one-shot "already exists" refusal: calling the verb a second time against
// the same, still-unfrozen path overwrites it.
func TestRunObligationAuthor_Regenerate(t *testing.T) {
	repo := buildObligationAuthorRepo(t, nil)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	if got := runObligationAuthor(ctx, repo.Dir, "spec/widget-story", "ac-1", "static", "", &stdout, &stderr); got != 0 {
		t.Fatalf("first author call = %d, want 0; stderr=%s", got, stderr.String())
	}
	path := obligationPathFor(repo.Dir, "ac-1", "static")
	firstBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// A human hand-edits the scaffold in place (simulating authored
	// content) — this must survive being OVERWRITTEN by a deliberate
	// second `obligation author` call (never survive it, since
	// "regenerate" means exactly that: the git history, not this verb, is
	// the safety net).
	edited := bytes.Replace(firstBytes, []byte("verdi:obligation-unauthored"), []byte("hand-authored, marker removed"), 1)
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	if got := runObligationAuthor(ctx, repo.Dir, "spec/widget-story", "ac-1", "static", "", &stdout, &stderr); got != 0 {
		t.Fatalf("second (regenerate) author call = %d, want 0; stderr=%s", got, stderr.String())
	}
	secondBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(secondBytes, edited) {
		t.Fatal("regenerate did not overwrite the hand-edited content")
	}
	if !contains(string(secondBytes), "verdi:obligation-unauthored") {
		t.Errorf("regenerated content lost the unauthored marker:\n%s", secondBytes)
	}
}

// TestRunObligationAuthor_RefusesOnAlreadyFrozen is ac-5's core proof: an
// obligation reachable from the given diffBase (mirroring how
// internal/lint/vl010_test.go passes a fixture commit directly as
// Context.DiffBase, rather than fabricating a real origin/main remote)
// refuses outright, exit 2, naming the path, leaving the tree untouched.
func TestRunObligationAuthor_RefusesOnAlreadyFrozen(t *testing.T) {
	frozenObligationMD := `---
id: obligation/widget-story--ac-1--static
kind: obligation
title: "already frozen by a prior merge"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/widget-story" }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# already frozen by a prior merge

Reachable from the merge-base: accept, obligation author, and everyone
else must treat this as immutable.
`
	repo := buildObligationAuthorRepo(t, map[string]string{
		".verdi/obligations/widget-story/ac-1--static.md": frozenObligationMD,
	})
	ctx := context.Background()

	// The single init commit (repo.Head) already carries the obligation —
	// passing it as diffBase is exactly "reachable from the merge-base",
	// the frozen predicate ac-5 specifies.
	var stdout, stderr bytes.Buffer
	got := runObligationAuthor(ctx, repo.Dir, "spec/widget-story", "ac-1", "static", repo.Head, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runObligationAuthor(frozen) = %d, want 2 (operational, per the task's explicit contract); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !contains(stderr.String(), "ac-1--static.md") {
		t.Fatalf("stderr = %q, want it to name the frozen path", stderr.String())
	}

	// Untouched: byte-identical to the fixture's own content.
	got2, err := os.ReadFile(obligationPathFor(repo.Dir, "ac-1", "static"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got2) != frozenObligationMD {
		t.Fatalf("a frozen obligation must never be touched:\n--- got ---\n%s\n--- want ---\n%s", got2, frozenObligationMD)
	}
}

// TestRunObligationAuthor_OperationalGitError_RefusesNeverGuesses is
// judged-frozen-check-fail-open's proof: a NON-EMPTY diffBase whose Show/
// ls-tree probe fails operationally (a well-formed sha that resolves to no
// commit) must never be read as "not frozen — proceed to overwrite". The
// verb cannot prove the target is unfrozen, so it refuses (exit 2) naming
// the git failure rather than silently regenerating what a merge to main may
// have frozen. The already-approved diffBase=="" posture (frozen-ness
// unprovable at the DEFAULT-BRANCH step) is unchanged — this is about a Show
// error AFTER a base resolved.
func TestRunObligationAuthor_OperationalGitError_RefusesNeverGuesses(t *testing.T) {
	repo := buildObligationAuthorRepo(t, nil)
	ctx := context.Background()

	// A syntactically valid sha that names no object in this repo: the frozen
	// probe against it is an operational git failure, not a clean "absent at
	// a resolvable base" answer.
	const unresolvableBase = "0000000000000000000000000000000000000000"

	var stdout, stderr bytes.Buffer
	got := runObligationAuthor(ctx, repo.Dir, "spec/widget-story", "ac-1", "static", unresolvableBase, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runObligationAuthor(operational git error on the frozen probe) = %d, want 2 (never guess unfrozen on a git failure); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !contains(stderr.String(), "ac-1--static.md") {
		t.Errorf("stderr = %q, want it to name the path whose frozen-ness could not be determined", stderr.String())
	}
	// It must NOT have proceeded to regenerate: a refused frozen probe writes
	// nothing.
	if _, err := os.Stat(obligationPathFor(repo.Dir, "ac-1", "static")); !os.IsNotExist(err) {
		t.Errorf("the verb wrote an obligation despite an undecidable frozen probe (err=%v)", err)
	}
}

// TestRunObligationAuthor_NotYetFrozen_SameFileAbsentAtDiffBase proves the
// frozen predicate is commit-scoped, not path-existence-scoped: a
// diffBase commit that never had the file (even though a LATER, still
// only-on-this-branch commit does) is NOT frozen — the accept-then-
// obligation-author-before-push workflow spec/obligation-seam's outcome
// describes.
func TestRunObligationAuthor_NotYetFrozen_SameFileAbsentAtDiffBase(t *testing.T) {
	repo := buildObligationAuthorRepo(t, nil)
	ctx := context.Background()

	// diffBase = the init commit, which never had this obligation at all.
	var stdout, stderr bytes.Buffer
	if got := runObligationAuthor(ctx, repo.Dir, "spec/widget-story", "ac-1", "static", repo.Head, &stdout, &stderr); got != 0 {
		t.Fatalf("runObligationAuthor(not yet frozen) = %d, want 0; stderr=%s", got, stderr.String())
	}
	if _, err := os.Stat(obligationPathFor(repo.Dir, "ac-1", "static")); err != nil {
		t.Fatalf("expected a scaffold to be written: %v", err)
	}
}

// frozenAc1StaticObligationMD is a decodable obligation used to observe
// whether the verb wrongly "proceeds" (regenerates over it) instead of
// refusing.
const frozenAc1StaticObligationMD = `---
id: obligation/widget-story--ac-1--static
kind: obligation
title: "already frozen by a prior merge"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/widget-story" }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# already frozen by a prior merge

Must be treated as immutable — never regenerated in place.
`

// TestCmdObligationAuthor_MergeBaseOperationalFailure_Refuses is the round-2
// judged-frozen-check-fail-open proof, one seam upstream of the round-1 fix:
// when the default branch RESOLVES but merge-base(HEAD, default) fails
// operationally, the frozen-probe base is unknowable due to a git failure, not
// a proven absence. cmdObligationAuthor must refuse (exit 2) naming the git
// failure rather than treat the empty base as the hermetic "proceed" posture
// and regenerate what a merge to main may have frozen. Injected via the
// obligationFrozenProbeBase seam, since a clean fixture repo cannot
// deterministically produce an operational merge-base failure.
func TestCmdObligationAuthor_MergeBaseOperationalFailure_Refuses(t *testing.T) {
	repo := buildObligationAuthorRepo(t, map[string]string{
		".verdi/obligations/widget-story/ac-1--static.md": frozenAc1StaticObligationMD,
	})
	t.Chdir(repo.Dir)

	orig := obligationFrozenProbeBase
	obligationFrozenProbeBase = func(ctx context.Context, root string) (string, bool, error) {
		return "", true, errors.New("injected merge-base operational failure (default branch resolved)")
	}
	defer func() { obligationFrozenProbeBase = orig }()

	var stdout, stderr bytes.Buffer
	got := cmdObligationAuthor([]string{"spec/widget-story", "ac-1", "static"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdObligationAuthor(operational merge-base failure) = %d, want 2 (never guess unfrozen on a git failure); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !contains(stderr.String(), "frozen") {
		t.Errorf("stderr = %q, want it to explain the frozen-ness could not be determined", stderr.String())
	}

	// It must NOT have regenerated the frozen obligation.
	got2, err := os.ReadFile(obligationPathFor(repo.Dir, "ac-1", "static"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got2) != frozenAc1StaticObligationMD {
		t.Fatalf("the frozen obligation was regenerated despite an undecidable merge-base:\n--- got ---\n%s\n--- want ---\n%s", got2, frozenAc1StaticObligationMD)
	}
}

// TestCmdObligationAuthor_HermeticNoDefaultBranch_Proceeds is the companion
// guard: when the default branch cannot be resolved AT ALL, the disclosed
// hermetic "can't prove frozen, proceed" posture (§Ac 5) is preserved — the
// verb creates the scaffold rather than refusing. Distinct from the
// operational-failure case above by exactly the seam's discrimination.
func TestCmdObligationAuthor_HermeticNoDefaultBranch_Proceeds(t *testing.T) {
	repo := buildObligationAuthorRepo(t, nil)
	t.Chdir(repo.Dir)

	orig := obligationFrozenProbeBase
	obligationFrozenProbeBase = func(ctx context.Context, root string) (string, bool, error) {
		return "", false, nil // no default branch resolves at all — hermetic
	}
	defer func() { obligationFrozenProbeBase = orig }()

	var stdout, stderr bytes.Buffer
	got := cmdObligationAuthor([]string{"spec/widget-story", "ac-1", "static"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdObligationAuthor(hermetic no default branch) = %d, want 0 (disclosed proceed posture); stderr=%s", got, stderr.String())
	}
	if _, err := os.Stat(obligationPathFor(repo.Dir, "ac-1", "static")); err != nil {
		t.Fatalf("expected a scaffold to be written under the hermetic posture: %v", err)
	}
}

// TestObligationFrozenProbeBase_ResolvedNameUnresolvableRef_RefusesOperationally
// is fix-round-1 finding 3's proof, over the REAL (uninjected)
// obligationFrozenProbeBase: a default branch NAME resolves
// (CI_DEFAULT_BRANCH=main is configured) but no local branch and no
// origin/main remote-tracking ref exists for it (main configured but
// never fetched — the shape a shallow/partial clone leaves behind). This
// is NOT the hermetic "no signal at all" case (TestCmdObligationAuthor_
// HermeticNoDefaultBranch_Proceeds, which injects the fake result
// directly); it must refuse operationally, naming the branch and the
// fetch remedy, exactly like judged-frozen-check-fail-open's other two
// discriminated cases.
func TestObligationFrozenProbeBase_ResolvedNameUnresolvableRef_RefusesOperationally(t *testing.T) {
	repo := buildObligationAuthorRepo(t, nil)
	// fixturegit always inits with a LOCAL branch literally named "main";
	// rename it away so neither a local "main" branch nor an
	// origin/main remote-tracking ref exists (fixturegit repos carry no
	// origin remote at all) — CI_DEFAULT_BRANCH=main then names a branch
	// that resolves to no ref whatsoever.
	rename := exec.Command("git", "branch", "-m", "main", "not-main")
	rename.Dir = repo.Dir
	if out, err := rename.CombinedOutput(); err != nil {
		t.Fatalf("git branch -m main not-main: %v\n%s", err, out)
	}
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	base, operationalFailure, err := obligationFrozenProbeBase(context.Background(), repo.Dir)
	if !operationalFailure {
		t.Fatalf("obligationFrozenProbeBase(resolved name, unresolvable ref) = (%q, %v, %v), want operationalFailure=true", base, operationalFailure, err)
	}
	if base != "" {
		t.Fatalf("base = %q, want empty on an operational failure", base)
	}
	if err == nil {
		t.Fatal("err = nil, want an error naming the branch and the fetch remedy")
	}
	if !strings.Contains(err.Error(), "main") || !strings.Contains(err.Error(), "fetch") {
		t.Fatalf("err = %q, want it to name the branch %q and a fetch remedy", err.Error(), "main")
	}
}

// TestRunObligationAuthor_Negative covers the refusal/error paths that
// never write anything.
func TestRunObligationAuthor_Negative(t *testing.T) {
	repo := buildObligationAuthorRepo(t, nil)
	ctx := context.Background()

	cases := []struct {
		name              string
		storyRef, ac, knd string
		wantExit          int
		wantStderr        string
	}{
		{"unknown evidence kind fails closed", "spec/widget-story", "ac-1", "bogus", 2, "not a known evidence kind"},
		{"unresolvable story ref", "jira:NO-SUCH-STORY", "ac-1", "static", 1, "no active"},
		{"undeclared AC", "spec/widget-story", "ac-9", "static", 1, "does not"},
		{"AC does not declare the requested kind", "spec/widget-story", "ac-1", "runtime", 1, "does not declare"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := runObligationAuthor(ctx, repo.Dir, tc.storyRef, tc.ac, tc.knd, "", &stdout, &stderr)
			if got != tc.wantExit {
				t.Fatalf("exit = %d, want %d; stderr=%s", got, tc.wantExit, stderr.String())
			}
			if !contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tc.wantStderr)
			}
		})
	}

	// None of the above wrote anything to .verdi/obligations/ at all.
	if _, err := os.Stat(obligationPathFor(repo.Dir, "ac-1", "static")); !os.IsNotExist(err) {
		t.Errorf("a refused author call wrote an obligation (err=%v)", err)
	}
}

// TestRunObligationVerb_Usage and TestCmdObligationAuthor_UsageNegative
// pin the verb's own argument-shape checks (mirroring
// TestCmdAccept_UsageNegative/TestRun_AcceptDispatchesToRealVerb's style).
func TestRunObligationVerb_Usage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := runObligationVerb(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("runObligationVerb(no args) = %d, want 2", got)
	}
	stdout.Reset()
	stderr.Reset()
	if got := runObligationVerb([]string{"bogus"}, &stdout, &stderr); got != 2 {
		t.Fatalf("runObligationVerb(bogus subcommand) = %d, want 2", got)
	}
}

func TestCmdObligationScaffold_UsageNegative(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := cmdObligationScaffold(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdObligationScaffold(no args) = %d, want 2", got)
	}
	stdout.Reset()
	stderr.Reset()
	if got := cmdObligationScaffold([]string{"a", "b"}, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdObligationScaffold(two args) = %d, want 2", got)
	}
}

func TestRunObligationVerb_DispatchesScaffold(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	got := runObligationVerb([]string{"scaffold", "spec/x"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runObligationVerb([scaffold spec/x]) outside a store = %d, want 2 (operational)", got)
	}
	if contains(stderr.String(), "usage") {
		t.Fatalf("stderr = %q, want a real store-root error, not the usage message", stderr.String())
	}
}

// --- `verdi obligation scaffold` (Task 7, docs/superpowers/specs/2026-08-
// 01-merge-signals-spec-acceptance-design.md): the pre-review, idempotent,
// batch-creation surface that replaces accept's retired freeze-moment
// backstop. Step 3's brief: for a story declaring static and behavioral
// evidence on ac-1, prove it creates both convention paths, a second run
// creates zero and reports both as present, an existing authored file is
// byte-identical after the run, unknown story/AC/kind fails closed, and an
// accepted story refuses mutation.

// fakeScaffoldResolver is a specStateResolver test double for
// runObligationScaffold, mirroring buildstart_test.go's own seam: it
// returns a fixed Result for every candidate, letting these tests drive
// the Proposed/Accepted/Unproven branches without needing a real default
// branch to land bytes on.
type fakeScaffoldResolver struct {
	result specstate.Result
	err    error
}

func (f fakeScaffoldResolver) Resolve(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error) {
	return f.result, f.err
}

func (f fakeScaffoldResolver) ResolveMany(ctx context.Context, root string, candidates []specstate.Candidate) ([]specstate.Result, error) {
	results := make([]specstate.Result, len(candidates))
	for i := range candidates {
		results[i] = f.result
	}
	return results, f.err
}

var proposedResolver = fakeScaffoldResolver{result: specstate.Result{State: specstate.Proposed, Relation: specstate.RelationDiverged}}

// TestRunObligationScaffold_Happy is step 3's core proof: a story
// declaring static and behavioral evidence on ac-1 (obligationSeamStoryCleanMD
// also declares ac-2/behavioral) creates every missing convention path,
// owned by the operator, carrying the disclosure line.
//
// guide-claim: 7.1-accept-freeze-obligations
func TestRunObligationScaffold_Happy(t *testing.T) {
	t.Setenv("USER", "test-operator")
	repo := buildObligationSeamStoryRepo(t, nil)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	if got := runObligationScaffold(ctx, repo.Dir, "spec/widget-story", proposedResolver, phase7Model(t), &stdout, &stderr); got != 0 {
		t.Fatalf("runObligationScaffold = %d, want 0; stderr=%s", got, stderr.String())
	}

	for _, tc := range []struct{ acID, kind string }{
		{"ac-1", "static"},
		{"ac-2", "behavioral"},
	} {
		path := obligationPathFor(repo.Dir, tc.acID, tc.kind)
		ob, body := readObligation(t, path)
		if ob.ForKind != artifact.EvidenceKind(tc.kind) {
			t.Errorf("%s: for_kind = %q, want %q", path, ob.ForKind, tc.kind)
		}
		assertUnresolvedObligationQuality(t, ob)
		if len(ob.Owners) != 1 || ob.Owners[0] != "test-operator" {
			t.Errorf("%s: owners = %v, want [test-operator] (O-6)", path, ob.Owners)
		}
		if ob.Frozen == nil || ob.Frozen.Commit == "" {
			t.Errorf("%s: frozen = %+v, want a resolved HEAD stamp", path, ob.Frozen)
		}
		if !contains(string(body), obligationBackstopDisclosureLine()) {
			t.Errorf("%s: body does not carry the disclosure line verbatim:\n%s", path, body)
		}
		if !contains(stdout.String(), "created "+path) {
			t.Errorf("stdout = %q, want it to report %s as created", stdout.String(), path)
		}
	}
}

func assertUnresolvedObligationQuality(t *testing.T, ob *artifact.ObligationFrontmatter) {
	t.Helper()
	if ob.Quality == nil || ob.Quality.State != artifact.ObligationQualityUnresolved {
		t.Fatalf("quality = %+v, want explicit unresolved-design-debt", ob.Quality)
	}
	if ob.Quality.Claim != "" || ob.Quality.Falsifier != "" || ob.Quality.Scope != "" || ob.Quality.Producer.Ref != "" || ob.Quality.AuthoritativeSource.Ref != "" || len(ob.Quality.Freshness.InvalidatedBy) != 0 || ob.Quality.Freshness.Rule != "" {
		t.Fatalf("unresolved scaffold fabricated quality meanings: %+v", ob.Quality)
	}
}

// TestRunObligationScaffold_SecondRunIsIdempotent proves step 3's "a
// second run creates zero and reports both as present": re-running against
// the same story after the first scaffold writes nothing new and reports
// every pair as already present.
func TestRunObligationScaffold_SecondRunIsIdempotent(t *testing.T) {
	repo := buildObligationSeamStoryRepo(t, nil)
	ctx := context.Background()

	var stdout1, stderr1 bytes.Buffer
	if got := runObligationScaffold(ctx, repo.Dir, "spec/widget-story", proposedResolver, phase7Model(t), &stdout1, &stderr1); got != 0 {
		t.Fatalf("first run = %d, want 0; stderr=%s", got, stderr1.String())
	}
	firstStatic, err := os.ReadFile(obligationPathFor(repo.Dir, "ac-1", "static"))
	if err != nil {
		t.Fatal(err)
	}
	firstBehavioral, err := os.ReadFile(obligationPathFor(repo.Dir, "ac-2", "behavioral"))
	if err != nil {
		t.Fatal(err)
	}

	var stdout2, stderr2 bytes.Buffer
	if got := runObligationScaffold(ctx, repo.Dir, "spec/widget-story", proposedResolver, phase7Model(t), &stdout2, &stderr2); got != 0 {
		t.Fatalf("second run = %d, want 0; stderr=%s", got, stderr2.String())
	}
	if !contains(stdout2.String(), "ac-1 static: already present") || !contains(stdout2.String(), "ac-2 behavioral: already present") {
		t.Fatalf("second run stdout = %q, want both pairs reported already present", stdout2.String())
	}
	if contains(stdout2.String(), "created") {
		t.Fatalf("second run stdout = %q, want zero pairs reported created", stdout2.String())
	}

	secondStatic, err := os.ReadFile(obligationPathFor(repo.Dir, "ac-1", "static"))
	if err != nil {
		t.Fatal(err)
	}
	secondBehavioral, err := os.ReadFile(obligationPathFor(repo.Dir, "ac-2", "behavioral"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstStatic, secondStatic) || !bytes.Equal(firstBehavioral, secondBehavioral) {
		t.Fatal("the second run rewrote an already-scaffolded obligation")
	}
}

// TestRunObligationScaffold_NeverOverwritesAnAuthoredFile proves an
// existing, hand-authored obligation is byte-identical after the run and
// only the still-missing pair is scaffolded.
func TestRunObligationScaffold_NeverOverwritesAnAuthoredFile(t *testing.T) {
	repo := buildObligationSeamStoryRepo(t, map[string]string{
		".verdi/obligations/widget-story/ac-1--static.md": preExistingAc1StaticMD,
	})
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	if got := runObligationScaffold(ctx, repo.Dir, "spec/widget-story", proposedResolver, phase7Model(t), &stdout, &stderr); got != 0 {
		t.Fatalf("runObligationScaffold = %d, want 0; stderr=%s", got, stderr.String())
	}

	got, err := os.ReadFile(obligationPathFor(repo.Dir, "ac-1", "static"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != preExistingAc1StaticMD {
		t.Fatalf("pre-existing obligation was modified:\n--- got ---\n%s\n--- want (byte-identical) ---\n%s", got, preExistingAc1StaticMD)
	}
	if !contains(stdout.String(), "ac-1 static: already present") {
		t.Fatalf("stdout = %q, want ac-1/static reported already present", stdout.String())
	}
	if _, err := os.Stat(obligationPathFor(repo.Dir, "ac-2", "behavioral")); err != nil {
		t.Fatalf("the still-missing pair was not scaffolded: %v", err)
	}
}

// TestRunObligationScaffold_UnknownStoryFailsClosed proves an unresolvable
// story ref fails closed (operational, exit 2) rather than silently doing
// nothing.
func TestRunObligationScaffold_UnknownStoryFailsClosed(t *testing.T) {
	repo := buildObligationSeamStoryRepo(t, nil)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runObligationScaffold(ctx, repo.Dir, "jira:NO-SUCH-STORY", proposedResolver, phase7Model(t), &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runObligationScaffold(unknown story) = %d, want 2; stderr=%s", got, stderr.String())
	}
}

// TestRunObligationScaffold_NonStorySpecFailsClosed proves a feature-class
// target (no ac/kind obligations ever apply, dc-3) fails closed rather
// than silently no-op'ing.
func TestRunObligationScaffold_NonStorySpecFailsClosed(t *testing.T) {
	repo := buildObligationSeamStoryRepo(t, nil)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runObligationScaffold(ctx, repo.Dir, "spec/some-feature", proposedResolver, phase7Model(t), &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runObligationScaffold(feature spec) = %d, want 2; stderr=%s", got, stderr.String())
	}
	if !contains(stderr.String(), "story") {
		t.Fatalf("stderr = %q, want it to name the story-only restriction", stderr.String())
	}
}

// TestRunObligationScaffold_AcceptedStoryRefusesMutation is I-41's own
// proof: a story whose Git-derived effective state is anything other than
// Proposed refuses (a verdict failure, exit 1) — obligation scaffolding is
// pre-review preparation only, resolved through the specStateResolver seam
// (buildstart.go's established pattern), never through raw status or a
// merge-base approximation. Table-drives every non-Proposed state.
func TestRunObligationScaffold_AcceptedStoryRefusesMutation(t *testing.T) {
	cases := []struct {
		name  string
		state specstate.State
	}{
		{"accepted-pending-build", specstate.AcceptedPendingBuild},
		{"superseded", specstate.Superseded},
		{"closed", specstate.Closed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildObligationSeamStoryRepo(t, nil)
			ctx := context.Background()
			resolver := fakeScaffoldResolver{result: specstate.Result{State: tc.state, Relation: specstate.RelationExact}}

			var stdout, stderr bytes.Buffer
			got := runObligationScaffold(ctx, repo.Dir, "spec/widget-story", resolver, phase7Model(t), &stdout, &stderr)
			if got != 1 {
				t.Fatalf("runObligationScaffold(%s story) = %d, want 1 (verdict refusal); stderr=%s", tc.name, got, stderr.String())
			}
			if _, err := os.Stat(obligationPathFor(repo.Dir, "ac-1", "static")); !os.IsNotExist(err) {
				t.Errorf("an already-%s story must not be mutated: err=%v", tc.name, err)
			}
		})
	}
}

// TestRunObligationScaffold_UnprovenRefusesOperationally proves an
// unprovable Git-derived state (no default branch resolvable, ...) refuses
// operationally (exit 2) rather than guessing either way.
func TestRunObligationScaffold_UnprovenRefusesOperationally(t *testing.T) {
	repo := buildObligationSeamStoryRepo(t, nil)
	ctx := context.Background()
	resolver := fakeScaffoldResolver{result: specstate.Result{State: specstate.Unproven, Relation: specstate.RelationUnproven, Disclosures: []string{"no default branch could be resolved"}}}

	var stdout, stderr bytes.Buffer
	got := runObligationScaffold(ctx, repo.Dir, "spec/widget-story", resolver, phase7Model(t), &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runObligationScaffold(unproven) = %d, want 2; stderr=%s", got, stderr.String())
	}
	if _, err := os.Stat(obligationPathFor(repo.Dir, "ac-1", "static")); !os.IsNotExist(err) {
		t.Errorf("an unproven story must not be mutated: err=%v", err)
	}
}

// TestRunObligationScaffold_PartialFailureLeavesResidue is spec/
// obligation-seam ac-3's OWN characterization under Task 7's narrowed
// guarantee (fix round 1 finding 2 — see the invention-ledger addendum in
// the phase report): unlike the retired accept-time backstop
// (unlinkScaffoldedObligations, deleted with accept's own mutation), a
// mid-loop failure does NOT roll back or unlink whatever this command
// already wrote before hitting it. widget-story declares two pairs
// (ac-1/static, ac-2/behavioral, declaration order); ac-2's convention
// path is occupied by a present-but-undecodable file, so scaffolding
// fails on the SECOND pair, after ac-1's has already been written — ac-1's
// scaffold must survive the failure on disk, exactly as
// scaffoldMissingObligations' own unit test already proves at the
// function level (TestScaffoldMissingObligations, acceptobligation_test.go)
// — this is the CLI-level companion proving `runObligationScaffold` itself
// performs no cleanup on top of that. The design branch (git diff/
// checkout) is the safety net now, not this command — the same posture
// `verdi obligation author`'s own regenerate case already has.
func TestRunObligationScaffold_PartialFailureLeavesResidue(t *testing.T) {
	repo := buildObligationSeamStoryRepo(t, map[string]string{
		".verdi/obligations/widget-story/ac-2--behavioral.md": malformedAc2BehavioralMD,
	})
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runObligationScaffold(ctx, repo.Dir, "spec/widget-story", proposedResolver, phase7Model(t), &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runObligationScaffold(malformed existing obligation at ac-2) = %d, want 2; stderr=%s", got, stderr.String())
	}
	if !contains(stderr.String(), "ac-2") {
		t.Errorf("stderr = %q, want it to name ac-2", stderr.String())
	}

	// ac-1's pair, scaffolded before the ac-2 failure, is NOT rolled back —
	// if a rollback guarantee is ever reintroduced, this assertion (and
	// spec/obligation-seam ac-3's own binding) must be updated together,
	// never silently.
	if _, err := os.Stat(obligationPathFor(repo.Dir, "ac-1", "static")); err != nil {
		t.Fatalf("ac-1's scaffold did not survive the ac-2 failure (err=%v) — the narrowed, disclosed posture is that this command performs no rollback", err)
	}

	// The malformed file at ac-2 itself is untouched — refused, never
	// clobbered (unchanged from before this task).
	got2, err := os.ReadFile(obligationPathFor(repo.Dir, "ac-2", "behavioral"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got2) != malformedAc2BehavioralMD {
		t.Fatalf("the malformed pre-existing file was modified:\n--- got ---\n%s\n--- want (byte-identical) ---\n%s", got2, malformedAc2BehavioralMD)
	}
}

func TestCmdObligationAuthor_UsageNegative(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := cmdObligationAuthor(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdObligationAuthor(no args) = %d, want 2", got)
	}
	stdout.Reset()
	stderr.Reset()
	if got := cmdObligationAuthor([]string{"a", "b"}, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdObligationAuthor(two args) = %d, want 2", got)
	}
	stdout.Reset()
	stderr.Reset()
	if got := cmdObligationAuthor([]string{"a", "b", "c", "d"}, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdObligationAuthor(four args) = %d, want 2", got)
	}
}

// TestRun_ObligationDispatchesToRealVerb proves dispatch.go routes
// "obligation" to the real implementation, mirroring
// TestRun_AcceptDispatchesToRealVerb's exact pattern.
func TestRun_ObligationDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	got := run([]string{"obligation", "author", "spec/x", "ac-1", "static"}, &stderr)
	if got != 2 {
		t.Fatalf("run([obligation author ...]) outside a store = %d, want 2 (operational)", got)
	}
	if contains(stderr.String(), "usage") || contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}

// TestObligationAuthor_AtomicWrite_NoDirectCreateTemp is obligation.go's
// own source-text witness twin of internal/workbench/
// obligationauthor_test.go's TestObligationAuthor_AtomicWrite_NoDirectCreateTemp
// (spec/obligation-seam ac-4's static leg): cmd/verdi must never hand-roll
// obligation frontmatter or a second self-validate — only ever call the
// shared internal/evidence seam.
// TestObligationRender_SingleSharedSeam_PackageWide strengthens ac-4's static
// witness (judged-second-render-witness-scope). The prior witness read only two
// NAMED files for three negative markers, proving "these two files avoid three
// specific calls" rather than the AC's stated "cmd/verdi carries no second
// render/self-validate implementation" — a copy in any third file, or a Sprintf
// copy that trips none of the three markers, was outside its reach. This walks
// EVERY non-test .go file in cmd/verdi and proves two things package-wide:
//
//  1. Positive render-signature scan: a copy-pasted Sprintf render of
//     evidence.RenderObligation necessarily emits the obligation frontmatter
//     literals `kind: obligation` and `for_kind:` together; no cmd/verdi file
//     may carry that signature (the sole renderer lives in internal/evidence).
//  2. Call-graph allowlist: the only producers of obligation bytes reachable
//     from cmd/verdi are evidence.RenderObligation / evidence.WriteObligationFile,
//     called from exactly the two legitimate call sites — a third file reaching
//     for the seam trips this, forcing a conscious allowlist update.
//
// Mutation-witnessed during development: a scratch cmd/verdi file carrying a
// hand-rolled fmt.Sprintf obligation render trips clause (1) (then removed).
//
// Task 7 (docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-
// design.md) moves scaffoldMissingObligations — the freeze-moment
// backstop's own render/write call — out of acceptobligation.go into
// obligation.go alongside `verdi obligation author`'s own call; the
// allowlist below shrinks to the one remaining call site accordingly.
// acceptobligation.go still exists (owner resolution, disclosure/body
// rendering helpers both callers share) but no longer calls the seam
// itself.
func TestObligationRender_SingleSharedSeam_PackageWide(t *testing.T) {
	renderSeamCallSites := map[string]bool{
		"obligation.go": true,
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading cmd/verdi package dir: %v", err)
	}
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		src := string(data)
		scanned++

		// (1) No hand-rolled obligation-frontmatter render anywhere in cmd/verdi.
		if strings.Contains(src, "kind: obligation") && strings.Contains(src, "for_kind:") {
			t.Errorf("%s carries the obligation-frontmatter render signature (`kind: obligation` + `for_kind:`) — obligations must be rendered ONLY through internal/evidence.RenderObligation, never a second hand-rolled implementation in cmd/verdi (O-5, ac-4)", name)
		}

		// (2) The shared render/write seam is called only from the allowlisted
		// call sites — obligation bytes have exactly one producer, from known
		// sites.
		if strings.Contains(src, "evidence.RenderObligation(") || strings.Contains(src, "evidence.WriteObligationFile(") {
			if !renderSeamCallSites[name] {
				t.Errorf("%s calls the obligation render/write seam but is not an allowlisted call site (allowed: obligation.go, acceptobligation.go) — obligation bytes must be produced only through the one shared seam from the known callers (ac-4)", name)
			}
		}
	}
	if scanned < 3 {
		t.Fatalf("package-wide witness scanned only %d non-test files — expected the whole cmd/verdi package; is the test running in the package dir?", scanned)
	}
}

func TestObligationAuthor_AtomicWrite_NoDirectCreateTemp(t *testing.T) {
	for _, f := range []string{"obligation.go", "acceptobligation.go"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		if contains(string(data), "os.CreateTemp") {
			t.Errorf("%s calls os.CreateTemp directly — obligation writes must route through internal/evidence.WriteObligationFile instead (O-5)", f)
		}
		if contains(string(data), "yaml.Marshal") {
			t.Errorf("%s calls yaml.Marshal — obligation frontmatter must be hand-rendered through internal/evidence.RenderObligation only (O-5)", f)
		}
		if contains(string(data), "DecodeObligation(") {
			t.Errorf("%s calls artifact.DecodeObligation directly — the pre-write self-validate belongs solely to internal/evidence.WriteObligationFile (O-5, no re-render/no re-validate copy-paste)", f)
		}
	}
}
