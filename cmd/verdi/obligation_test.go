package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
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
