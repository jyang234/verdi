package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/upstream"
)

// TestCandidateAncestorCommits_CommitItselfIsFirst proves dc-1's central
// invariant in isolation from any forge call: the commit under evaluation
// is always the first candidate in the returned order (a commit is its
// own ancestor — gitx.IsAncestor's own documented self-inclusive
// semantics) — proven over a real multi-layer fixturegit history, not a
// single-commit repo where "first" would be trivial.
func TestCandidateAncestorCommits_CommitItselfIsFirst(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "1"}, Message: "layer 1"},
		{Files: map[string]string{"a.txt": "2"}, Message: "layer 2"},
		{Files: map[string]string{"a.txt": "3"}, Message: "layer 3"},
	})

	got, err := candidateAncestorCommits(context.Background(), repo.Dir, repo.Head)
	if err != nil {
		t.Fatalf("candidateAncestorCommits: %v", err)
	}
	if len(got) == 0 || got[0] != repo.Head {
		t.Fatalf("candidateAncestorCommits(%s) = %v, want %s first", repo.Head, got, repo.Head)
	}
}

// TestCandidateAncestorCommits_MatchesGitxLog_NoDepthLimit proves the
// helper is built DIRECTLY over gitx.Log — no hand-rolled parent walk, no
// second, possibly-disagreeing notion of "ancestor" (dc-1) — by asserting
// its output is identical, in both membership and order, to calling
// gitx.Log(ctx, root, rev) directly and taking each commit's SHA. It also
// proves no depth bound: every layer's commit, all the way to the root,
// must appear — not just a bounded prefix.
func TestCandidateAncestorCommits_MatchesGitxLog_NoDepthLimit(t *testing.T) {
	ctx := context.Background()
	const layerCount = 6
	layers := make([]fixturegit.Layer, 0, layerCount)
	for i := 1; i <= layerCount; i++ {
		layers = append(layers, fixturegit.Layer{
			Files:   map[string]string{"a.txt": fmt.Sprintf("content %d", i)},
			Message: fmt.Sprintf("layer %d", i),
		})
	}
	repo := fixturegit.Build(t, layers)

	got, err := candidateAncestorCommits(ctx, repo.Dir, repo.Head)
	if err != nil {
		t.Fatalf("candidateAncestorCommits: %v", err)
	}

	want, err := gitx.Log(ctx, repo.Dir, repo.Head)
	if err != nil {
		t.Fatalf("gitx.Log (oracle): %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("candidateAncestorCommits returned %d candidates, want %d (gitx.Log's own count)", len(got), len(want))
	}
	if len(got) != layerCount {
		t.Fatalf("candidateAncestorCommits returned %d candidates, want %d (one per layer — the full history, unbounded, dc-1's no-depth-limit)", len(got), layerCount)
	}
	for i := range want {
		if got[i] != want[i].SHA {
			t.Errorf("candidateAncestorCommits[%d] = %s, want %s (gitx.Log's own order at the same index)", i, got[i], want[i].SHA)
		}
	}
}

// TestCandidateAncestorCommits_Negative proves a root/commit git cannot
// resolve (not a git repository at all) is a real, surfaced error — never
// a silently-empty candidate list.
func TestCandidateAncestorCommits_Negative(t *testing.T) {
	dir := t.TempDir() // not a git repository
	if _, err := candidateAncestorCommits(context.Background(), dir, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"); err == nil {
		t.Fatal("candidateAncestorCommits(non-git root): want error, got nil")
	}
}

// countingForge wraps a forge.Forge and counts FetchEvidenceBundle calls
// — used to prove "no walk performed" concretely (ac-2: a bundle at the
// evaluated commit itself must be accepted with exactly one forge call,
// never a speculative probe of any ancestor).
type countingForge struct {
	forge.Forge
	calls int
}

func (c *countingForge) FetchEvidenceBundle(ctx context.Context, ref, commit string) (forge.DerivedTree, error) {
	c.calls++
	return c.Forge.FetchEvidenceBundle(ctx, ref, commit)
}

// buildBranchedAncestorRepo builds a NON-linear history that
// fixturegit.Build alone cannot express (it only supports a straight
// commit chain): A (base, main) -> branch "feature": B (child of A) ->
// back on main: C (child of A, diverging from B) -> merge feature into
// main: M (a REAL merge commit, parents C and B) -> main advances: D
// (child of M, HEAD). B is reachable from D only through M's SECOND
// parent, never a first-parent-only chain — proving the walk (built
// directly over gitx.Log, dc-1) reaches a genuinely non-first-parent
// ancestor that a hand-rolled single-parent walk would miss.
func buildBranchedAncestorRepo(t *testing.T) (dir string, a, b, c, m, d string) {
	t.Helper()
	ctx := context.Background()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"base.txt": "base\n"}, Message: "A: base"},
	})
	dir = repo.Dir
	a = repo.Head

	if err := gitx.CheckoutNewBranch(ctx, dir, "feature"); err != nil {
		t.Fatalf("CheckoutNewBranch(feature): %v", err)
	}
	b = commitAncestorFixtureFile(t, dir, "feature.txt", "feature\n", "B: feature commit")

	if err := gitx.Checkout(ctx, dir, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	c = commitAncestorFixtureFile(t, dir, "main.txt", "main\n", "C: main commit")

	runGitCmd(t, dir, "merge", "--quiet", "--no-ff", "-m", "M: merge feature", "feature")
	m = strings.TrimSpace(gitOutput(t, dir, "rev-parse", "HEAD"))

	d = commitAncestorFixtureFile(t, dir, "after.txt", "after\n", "D: main advances")
	return dir, a, b, c, m, d
}

func commitAncestorFixtureFile(t *testing.T, dir, path, content, message string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, dir, "add", "-A")
	runGitCmd(t, dir, "commit", "--quiet", "-m", message)
	return strings.TrimSpace(gitOutput(t, dir, "rev-parse", "HEAD"))
}

// TestRunSync_Ancestor_LinearHistory_AcceptsNamedAncestor proves ac-2 over
// a linear history: a bundle present only several commits back from HEAD
// is found and accepted, with the accepted commit and the walked distance
// both disclosed.
func TestRunSync_Ancestor_LinearHistory_AcceptsNamedAncestor(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "1"}, Message: "layer 1"},
		{Files: map[string]string{"a.txt": "2"}, Message: "layer 2"},
		{Files: map[string]string{"a.txt": "3"}, Message: "layer 3"},
		{Files: map[string]string{"a.txt": "4"}, Message: "layer 4"},
	})
	const ref = "main"
	target := repo.Heads[1] // two commits back from HEAD (repo.Heads[3])

	f := fake.New()
	f.SeedBundle(ref, target, forge.DerivedTree{"spec--x/" + target + "/verdicts.json": []byte("[]\n")})

	var stdout, stderr bytes.Buffer
	deps := syncDeps{Runner: upstream.NewFakeRunner(), Forge: f, GoTest: fakeGoTest{}, Stdout: &stdout, Stderr: &stderr}
	code := runSync(context.Background(), repo.Dir, ref, repo.Head, false, false, false, deps)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), target) {
		t.Errorf("stdout = %q, want the accepted ancestor commit %q named", stdout.String(), target)
	}
	if !strings.Contains(stdout.String(), "2 commit(s) back in log order") {
		t.Errorf("stdout = %q, want the disclosed distance as a log-order index (2 commit(s) back in log order — ADJ-41 fix)", stdout.String())
	}

	got, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "data", "derived", "spec--x", target, "verdicts.json"))
	if err != nil {
		t.Fatalf("reading materialized bundle at the accepted ancestor's own key: %v", err)
	}
	if string(got) != "[]\n" {
		t.Errorf("materialized verdicts.json = %q, want the seeded bytes", got)
	}
}

// TestRunSync_Ancestor_BranchedHistory_ReachesMergedAncestor proves ac-2
// over a branched (merge-commit) history: the bundle sits at a commit
// reachable from HEAD only through a merge's second parent, and the walk
// — built over gitx.Log rather than a hand-rolled first-parent walk —
// still reaches and accepts it.
func TestRunSync_Ancestor_BranchedHistory_ReachesMergedAncestor(t *testing.T) {
	dir, _, b, _, _, d := buildBranchedAncestorRepo(t)
	const ref = "main"

	f := fake.New()
	f.SeedBundle(ref, b, forge.DerivedTree{"spec--x/" + b + "/verdicts.json": []byte("[]\n")})

	var stdout, stderr bytes.Buffer
	deps := syncDeps{Runner: upstream.NewFakeRunner(), Forge: f, GoTest: fakeGoTest{}, Stdout: &stdout, Stderr: &stderr}
	code := runSync(context.Background(), dir, ref, d, false, false, false, deps)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), b) {
		t.Errorf("stdout = %q, want the accepted (merged-in, non-first-parent) commit %q named", stdout.String(), b)
	}

	// Cross-check the disclosed distance against gitx.Log's own order —
	// the SAME oracle candidateAncestorCommits is built over (dc-1: no
	// second, disagreeing notion of "ancestor").
	commits, err := gitx.Log(context.Background(), dir, d)
	if err != nil {
		t.Fatalf("gitx.Log oracle: %v", err)
	}
	wantDistance := -1
	for i, c := range commits {
		if c.SHA == b {
			wantDistance = i
			break
		}
	}
	if wantDistance < 0 {
		t.Fatalf("test bug: commit %s (B) not found in gitx.Log(%s)'s own output %v", b, d, commits)
	}
	if !strings.Contains(stdout.String(), fmt.Sprintf("%d commit(s) back in log order", wantDistance)) {
		t.Errorf("stdout = %q, want the disclosed distance %d as a log-order index (matching gitx.Log's own order — ADJ-41 fix)", stdout.String(), wantDistance)
	}
}

// TestRunSync_Ancestor_BundleAtHead_WinsWithNoWalk proves a bundle present
// at the evaluated commit itself wins immediately: distance 0 disclosed,
// and — via countingForge — exactly one FetchEvidenceBundle call, proving
// no ancestor walk was performed at all.
func TestRunSync_Ancestor_BundleAtHead_WinsWithNoWalk(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "1"}, Message: "layer 1"},
		{Files: map[string]string{"a.txt": "2"}, Message: "layer 2"},
	})
	const ref = "main"

	f := fake.New()
	f.SeedBundle(ref, repo.Head, forge.DerivedTree{"spec--x/" + repo.Head + "/verdicts.json": []byte("[]\n")})
	cf := &countingForge{Forge: f}

	var stdout, stderr bytes.Buffer
	deps := syncDeps{Runner: upstream.NewFakeRunner(), Forge: cf, GoTest: fakeGoTest{}, Stdout: &stdout, Stderr: &stderr}
	code := runSync(context.Background(), repo.Dir, ref, repo.Head, false, false, false, deps)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr.String())
	}
	if cf.calls != 1 {
		t.Errorf("forge FetchEvidenceBundle calls = %d, want exactly 1 (a bundle at HEAD must win with no walk performed)", cf.calls)
	}
	if !strings.Contains(stdout.String(), "0 commit(s) back in log order") {
		t.Errorf("stdout = %q, want distance 0 disclosed as a log-order index (ADJ-41 fix)", stdout.String())
	}
	if !strings.Contains(stdout.String(), repo.Head) {
		t.Errorf("stdout = %q, want the accepted commit %q named", stdout.String(), repo.Head)
	}
}

// TestRunSync_Ancestor_NoBundleAnywhere_RefusesNamingRange proves the
// exhausted-walk refusal names the ref and the commit range actually
// walked — never a bare, unqualified "no bundle" message.
func TestRunSync_Ancestor_NoBundleAnywhere_RefusesNamingRange(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "1"}, Message: "layer 1"},
		{Files: map[string]string{"a.txt": "2"}, Message: "layer 2"},
		{Files: map[string]string{"a.txt": "3"}, Message: "layer 3"},
	})
	const ref = "main"
	f := fake.New() // unseeded: no bundle anywhere in history

	var stdout, stderr bytes.Buffer
	deps := syncDeps{Runner: upstream.NewFakeRunner(), Forge: f, GoTest: fakeGoTest{}, Stdout: &stdout, Stderr: &stderr}
	code := runSync(context.Background(), repo.Dir, ref, repo.Head, false, false, false, deps)
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stdout=%s", code, stdout.String())
	}
	got := stderr.String()
	// The refusal names the ref and the range walked, and — fix 2 (ADJ-37)
	// — scopes the claim to THIS clone rather than overclaiming the ref's
	// entire history. A non-shallow fixturegit repo carries no shallow
	// marker, so no truncation disclosure is appended.
	for _, want := range []string{ref, repo.Heads[0], repo.Head, "3 commit(s) walked", "in this clone"} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr = %q, want it to name %q", got, want)
		}
	}
	if strings.Contains(got, "shallow clone") {
		t.Errorf("stderr = %q, must NOT claim a shallow clone when no shallow marker is present", got)
	}
}

// TestRunSync_Ancestor_NoBundleAnywhere_RangeNotationNamesEveryWalkedCommit
// pins the ac-2 exhaustion disclosure's range notation to name EXACTLY the
// commits actually walked (ADJ-64). The walk fetches the oldest commit too
// (rest[len-1]), so the disclosed range must be inclusive of it; git's own
// `oldest..commit` two-dot notation is EXCLUSIVE of oldest, naming one fewer
// commit than the stated walked count — the off-by-one this guards against.
func TestRunSync_Ancestor_NoBundleAnywhere_RangeNotationNamesEveryWalkedCommit(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "1"}, Message: "layer 1"},
		{Files: map[string]string{"a.txt": "2"}, Message: "layer 2"},
		{Files: map[string]string{"a.txt": "3"}, Message: "layer 3"},
	})
	const ref = "main"
	f := fake.New() // unseeded: no bundle anywhere in history

	var stdout, stderr bytes.Buffer
	deps := syncDeps{Runner: upstream.NewFakeRunner(), Forge: f, GoTest: fakeGoTest{}, Stdout: &stdout, Stderr: &stderr}
	if code := runSync(context.Background(), repo.Dir, ref, repo.Head, false, false, false, deps); code != 2 {
		t.Fatalf("exit = %d, want 2; stdout=%s", code, stdout.String())
	}
	got := stderr.String()
	// All three commits (HEAD, its parent, the root) were walked, so the
	// disclosure must mark itself inclusive of the oldest walked commit.
	if !strings.Contains(got, "inclusive") {
		t.Errorf("stderr = %q, want the range disclosure to mark itself inclusive of the oldest walked commit", got)
	}
	// It must NOT use git's exclusive `oldest..commit` two-dot range, which
	// excludes the oldest walked commit and would contradict the stated
	// "3 commit(s) walked" count (the ADJ-64 off-by-one).
	if exclusive := repo.Heads[0] + ".." + repo.Head; strings.Contains(got, exclusive) {
		t.Errorf("stderr = %q, must NOT disclose the exclusive git range %q (it excludes the walked oldest commit)", got, exclusive)
	}
}

// TestRunSync_Ancestor_NoBundleAnywhere_ShallowClone_DisclosesTruncation
// proves fix 2 (ADJ-37, disclosure only — no walk-semantics change): in a
// shallow clone, `git log` silently stops at the shallow boundary, so the
// walk saw only a truncated local graph. The exhausted-walk refusal must
// then disclose that the history was truncated and a bundle may sit at a
// deeper true ancestor absent from this clone — detected cheaply via git's
// own shallow-boundary marker. The marker is placed directly in the
// fixture's git dir (an empty shallow file, which `git log` tolerates), so
// the test is fully hermetic (co-1: no network).
func TestRunSync_Ancestor_NoBundleAnywhere_ShallowClone_DisclosesTruncation(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "1"}, Message: "layer 1"},
		{Files: map[string]string{"a.txt": "2"}, Message: "layer 2"},
		{Files: map[string]string{"a.txt": "3"}, Message: "layer 3"},
	})
	const ref = "main"

	// Mark the clone shallow the way a `--depth` fetch would: git's own
	// shallow-boundary marker file in the git dir. An empty marker is
	// tolerated by `git log` (the walk still runs over the local graph),
	// and its mere existence is what sync's cheap detection keys on.
	if err := os.WriteFile(filepath.Join(repo.Dir, ".git", "shallow"), nil, 0o644); err != nil {
		t.Fatalf("placing shallow marker: %v", err)
	}

	f := fake.New() // unseeded: no bundle anywhere in the walked history
	var stdout, stderr bytes.Buffer
	deps := syncDeps{Runner: upstream.NewFakeRunner(), Forge: f, GoTest: fakeGoTest{}, Stdout: &stdout, Stderr: &stderr}
	code := runSync(context.Background(), repo.Dir, ref, repo.Head, false, false, false, deps)
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	got := stderr.String()
	for _, want := range []string{"shallow clone", "truncated"} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr = %q, want the shallow-truncation disclosure to contain %q", got, want)
		}
	}
}

// gitInitTestStore turns a buildTestStore root into a REAL git repository
// with a few commits, so the ancestor walk actually RUNS and reaches the
// exhausted-walk refusal (vs. the enumeration-failure path a non-git root
// takes). Returns HEAD's sha. Hermetic — local git plumbing only (co-1).
func gitInitTestStore(t *testing.T, root string) string {
	t.Helper()
	runGitCmd(t, root, "init", "--quiet")
	runGitCmd(t, root, "config", "user.email", "t@t.t")
	runGitCmd(t, root, "config", "user.name", "t")
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "--quiet", "-m", "store")
	for i := 0; i < 2; i++ {
		if err := os.WriteFile(filepath.Join(root, "svcfix", fmt.Sprintf("nudge%d.txt", i)), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGitCmd(t, root, "add", "-A")
		runGitCmd(t, root, "commit", "--quiet", "-m", fmt.Sprintf("nudge %d", i))
	}
	return strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
}

// TestRunSync_OrRegen_ShallowExhaustedWalk_DisclosesTruncation proves fix 3
// (ADJ-41): the shallow-truncation disclosure must reach the --or-regen
// REGENERATE path, not only the exhausted-walk refusal. In a shallow clone,
// `git log` stops at the boundary, so a `verdi sync --or-regen` walk that
// exhausts the truncated graph is NOT genuine absence-evidence — a bundle
// may sit at a deeper true ancestor the clone never contained. sync must
// disclose that truncation BEFORE regenerating locally.
func TestRunSync_OrRegen_ShallowExhaustedWalk_DisclosesTruncation(t *testing.T) {
	root := buildTestStore(t)
	head := gitInitTestStore(t, root) // a real git repo → the walk runs and exhausts
	// Mark it shallow the way a --depth fetch would (empty marker, tolerated
	// by git log); its existence is what the disclosure keys on.
	if err := os.WriteFile(filepath.Join(root, ".git", "shallow"), nil, 0o644); err != nil {
		t.Fatalf("placing shallow marker: %v", err)
	}

	var stdout, stderr bytes.Buffer
	deps := syncDeps{
		Runner: seedRunner(t, root),
		Forge:  fake.New(), // unseeded → no bundle anywhere on the walked (truncated) graph
		GoTest: fakeGoTest{output: []byte(svcfixGoTestJSON)},
		Stdout: &stdout,
		Stderr: &stderr,
	}
	code := runSync(context.Background(), root, testRef, head, true /*orRegen*/, false, false, deps)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (regeneration proceeds after the disclosure); stderr=%s", code, stderr.String())
	}
	got := stderr.String()
	for _, want := range []string{"shallow clone", "truncated"} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr = %q, want the --or-regen shallow-truncation disclosure to contain %q (ADJ-41 fix 3)", got, want)
		}
	}
	if !strings.Contains(stdout.String(), "regenerated evidence bundle locally") {
		t.Errorf("stdout = %q, want regeneration to still proceed after the disclosure", stdout.String())
	}
}

// TestRunSync_OrRegen_NonShallowExhaustedWalk_StaysQuiet regression-pins
// fix 3's other half: on a full (non-shallow) clone, a --or-regen walk that
// exhausts and regenerates stays byte-quiet about shallowness/truncation
// exactly as today — the disclosure fires ONLY for the shallow sub-case,
// never for a plain absence.
func TestRunSync_OrRegen_NonShallowExhaustedWalk_StaysQuiet(t *testing.T) {
	root := buildTestStore(t)
	head := gitInitTestStore(t, root) // a real git repo, NO shallow marker

	var stdout, stderr bytes.Buffer
	deps := syncDeps{
		Runner: seedRunner(t, root),
		Forge:  fake.New(), // unseeded → exhausted walk over the full local graph
		GoTest: fakeGoTest{output: []byte(svcfixGoTestJSON)},
		Stdout: &stdout,
		Stderr: &stderr,
	}
	code := runSync(context.Background(), root, testRef, head, true /*orRegen*/, false, false, deps)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr.String())
	}
	got := stderr.String()
	if strings.Contains(got, "shallow clone") || strings.Contains(got, "truncated") {
		t.Errorf("stderr = %q, must stay quiet about shallow/truncation on a non-shallow --or-regen exhausted walk", got)
	}
	if !strings.Contains(stdout.String(), "regenerated evidence bundle locally") {
		t.Errorf("stdout = %q, want regeneration to proceed quietly", stdout.String())
	}
}

// TestRunSync_OrRegen_UnwalkableHistory_DisclosesWalkNeverRan proves fix 1
// (ADJ-37): when the commit itself carries no bundle AND its further
// ancestry cannot even be enumerated (here buildTestStore's root is not a
// git repository, so gitx.Log fails), the ancestor walk never ran — that
// is NOT the same evidence as a genuine no-bundle miss. Under --or-regen,
// sync must DISCLOSE that the walk never ran (and why) BEFORE regenerating
// locally, never silently treating an unwalkable history as absence-
// evidence. Regeneration still proceeds afterward (disclose, then fall
// back).
func TestRunSync_OrRegen_UnwalkableHistory_DisclosesWalkNeverRan(t *testing.T) {
	root := buildTestStore(t) // a store, but NOT a git repo → gitx.Log fails
	var stdout, stderr bytes.Buffer
	deps := syncDeps{
		Runner: seedRunner(t, root),
		Forge:  fake.New(), // unseeded → ErrNoBundle at the commit itself
		GoTest: fakeGoTest{output: []byte(svcfixGoTestJSON)},
		Stdout: &stdout,
		Stderr: &stderr,
	}

	code := runSync(context.Background(), root, testRef, testCommit, true /*orRegen*/, false, false, deps)
	if code != 0 {
		t.Fatalf("runSync(--or-regen, unwalkable history) exit = %d, want 0 (regeneration still proceeds after disclosure); stderr=%s", code, stderr.String())
	}

	gotErr := stderr.String()
	if !strings.Contains(gotErr, "the nearest-ancestor bundle walk never ran") {
		t.Errorf("stderr = %q, want an explicit disclosure that the nearest-ancestor bundle walk never ran (ADJ-37 fix 1)", gotErr)
	}
	// The disclosure must also carry WHY — the enumeration failure the walk
	// hit (the wrapped cause), not a bare, contextless caveat.
	if !strings.Contains(gotErr, "could not be walked") {
		t.Errorf("stderr = %q, want the disclosure to name the enumeration failure (the why)", gotErr)
	}
	if !strings.Contains(stdout.String(), "regenerated evidence bundle locally") {
		t.Errorf("stdout = %q, want regeneration to still proceed after the disclosure", stdout.String())
	}
}

// TestRunSync_NoOrRegen_UnwalkableHistory_StaysByteIdentical regression-
// pins fix 1's other half: the fix touches ONLY the --or-regen branch. The
// no---or-regen path over the very same unwalkable history stays exactly as
// today — an exit-2 operational refusal that discloses the enumeration
// failure and points at --or-regen — and must NOT gain the new
// walk-never-ran disclosure line the --or-regen branch prints (that line
// belongs only where sync is about to fall back regardless).
func TestRunSync_NoOrRegen_UnwalkableHistory_StaysByteIdentical(t *testing.T) {
	root := buildTestStore(t) // not a git repo → gitx.Log fails, same as above
	var stdout, stderr bytes.Buffer
	deps := syncDeps{
		Runner: seedRunner(t, root),
		Forge:  fake.New(), // unseeded → ErrNoBundle at the commit itself
		GoTest: fakeGoTest{output: []byte(svcfixGoTestJSON)},
		Stdout: &stdout,
		Stderr: &stderr,
	}

	code := runSync(context.Background(), root, testRef, testCommit, false /*orRegen*/, false, false, deps)
	if code != 2 {
		t.Fatalf("runSync(no --or-regen, unwalkable history) exit = %d, want 2; stderr=%s", code, stderr.String())
	}
	gotErr := stderr.String()
	for _, want := range []string{"could not be walked", "pass --or-regen"} {
		if !strings.Contains(gotErr, want) {
			t.Errorf("stderr = %q, want the unchanged no---or-regen refusal to contain %q", gotErr, want)
		}
	}
	if strings.Contains(gotErr, "walk never ran") {
		t.Errorf("stderr = %q, must NOT carry the --or-regen-only walk-never-ran disclosure on the no---or-regen path", gotErr)
	}
}
