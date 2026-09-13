// Tests for design start's statement-sourcing modes (spec/cli-creation
// ac-1/ac-2, ledger L-N7): --problem/--outcome, --defer-statements, the
// flagless TTY interview, and their refusals. The real, built-binary TTY
// interview path is proven via the disclosed VERDI_DESIGN_ASSUME_TTY=1
// stdin-script harness (mirroring init_test.go's own VERDI_INIT_ASSUME_TTY
// convention, an independent env var so the two verbs' test-injection
// surfaces never interfere with each other in their shared test binary).
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/provider"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

// --- R1 preparation-boundary test support (SI-198) -------------------------
//
// spec/cli-creation ac-1/ac-2's preparation boundary (05 §CLI; the MVP
// release amendment's R1) requires that a statement-sourcing or
// template-load refusal leave Git and the candidate spec directory
// byte-for-byte as they were, and never reach provider title resolution
// or baseline regeneration. The helpers below prove exactly that: spies
// record whether their real call ever fires, and repoGitSnapshot captures
// every Git-visible fact a refusal must not move.

// spyProvider wraps a real provider.StoryProvider, counting Resolve calls
// so a test can assert a preparation refusal never reaches provider-title
// resolution (runDesignStart's resolveStoryTitle, 04 §Semantics) — never
// merely trusting the source's current call order.
type spyProvider struct {
	provider.StoryProvider
	resolveCalls int
}

func (s *spyProvider) Resolve(ctx context.Context, ref provider.StoryRef) (provider.Story, error) {
	s.resolveCalls++
	return s.StoryProvider.Resolve(ctx, ref)
}

// spyRunner is an upstream.Runner that records whether it was ever asked
// to run anything, proving a preparation refusal never reaches baseline
// regeneration's exec surface (baseline.go's regenerateBaseline).
type spyRunner struct{ calls int }

func (s *spyRunner) Run(ctx context.Context, req upstream.Request) (upstream.Result, error) {
	s.calls++
	return upstream.Result{}, nil
}

// spyGoTest is spyRunner's goTestRunner twin.
type spyGoTest struct{ calls int }

func (s *spyGoTest) RunGoTest(ctx context.Context, dir string) ([]byte, error) {
	s.calls++
	return nil, nil
}

// repoGitSnapshot is every Git-visible fact a preparation refusal must
// leave exactly as it found it (R1: "no command-induced change to HEAD,
// current branch, branch refs AND their targets, index, tracked/untracked
// candidate files, or Verdi data").
//
// branchNames and branchTargets are deliberately two fields, not one list
// of names: R1 binds the REFS and their TARGETS, and a name-only snapshot
// is blind to the whole class of damage where the ref set is identical but
// some pre-existing, never-checked-out branch has been moved to a
// different commit.
type repoGitSnapshot struct {
	head          string
	branch        string
	branchNames   []string
	branchTargets map[string]string
	indexTree     string
	changedPaths  []string
	changedBytes  map[string][]byte
	dataFiles     map[string][]byte
}

// snapshotRepoGitState captures dir's current HEAD, symbolic branch, every
// refs/heads ref together with the object it points at, the index tree
// (`git write-tree`), every path that differs from HEAD — staged,
// unstaged, or untracked — along with its bytes, and the ignored
// .verdi/data/ tree (gitignored in a real store, so invisible to `git
// status`; checked directly on disk instead).
func snapshotRepoGitState(t *testing.T, ctx context.Context, dir string) repoGitSnapshot {
	t.Helper()
	head, err := gitx.RevParse(ctx, dir, "HEAD")
	if err != nil {
		t.Fatalf("snapshotRepoGitState: RevParse(HEAD): %v", err)
	}
	branch, err := gitx.CurrentBranch(ctx, dir)
	if err != nil {
		t.Fatalf("snapshotRepoGitState: CurrentBranch: %v", err)
	}
	names, targets := snapshotLocalBranchTargets(t, dir)
	changed := snapshotChangedPaths(t, dir)
	changedBytes := make(map[string][]byte, len(changed))
	for _, p := range changed {
		data, readErr := os.ReadFile(filepath.Join(dir, p))
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue // a deletion/rename-away: the path list already names this
			}
			t.Fatalf("snapshotRepoGitState: reading changed path %s: %v", p, readErr)
		}
		changedBytes[p] = data
	}
	return repoGitSnapshot{
		head:          head,
		branch:        branch,
		branchNames:   names,
		branchTargets: targets,
		indexTree:     strings.TrimSpace(runFixtureGit(t, dir, "write-tree")),
		changedPaths:  changed,
		changedBytes:  changedBytes,
		dataFiles:     snapshotDesignStartData(t, filepath.Join(dir, ".verdi", "data")),
	}
}

// runFixtureGit runs git in dir and returns its stdout, failing t (with
// git's own stderr) on any non-zero exit — the same native-git style this
// file's snapshot already used for `git write-tree`, in one place now that
// the snapshot asks git three separate questions.
func runFixtureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = ee.Stderr
		}
		t.Fatalf("git %s (dir %s): %v\n%s", strings.Join(args, " "), dir, err, stderr)
	}
	return string(out)
}

// snapshotLocalBranchTargets lists dir's local branch short names AND the
// object each refs/heads ref points at, from one read-only `git
// for-each-ref` query (refname-sorted, so the name order is
// deterministic). The targets are the half R1 needs beyond the names: a
// command that moved a pre-existing, non-current branch leaves the name
// list byte-identical, so names alone can never witness it.
func snapshotLocalBranchTargets(t *testing.T, dir string) ([]string, map[string]string) {
	t.Helper()
	raw := runFixtureGit(t, dir, "for-each-ref", "--format=%(refname:short) %(objectname)", "refs/heads")
	var names []string
	targets := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if line == "" {
			continue
		}
		name, sha, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("snapshotLocalBranchTargets: malformed for-each-ref line %q", line)
		}
		names = append(names, name)
		targets[name] = sha
	}
	return names, targets
}

// snapshotChangedPaths lists every path that differs from HEAD — staged,
// unstaged, or untracked (`git status --porcelain -uall -z`).
//
// -uall is load-bearing: git's default collapses a wholly-untracked
// directory to the DIRECTORY entry, which the caller's byte read would
// then fail on (EISDIR) rather than compare; listing every untracked FILE
// individually is what lets an operator's untracked work be compared byte
// for byte. -z keeps paths unquoted and unambiguous. Ignored paths are
// deliberately absent (no --ignored): the ignored .verdi/data/ tree is
// captured directly off disk by the caller instead.
func snapshotChangedPaths(t *testing.T, dir string) []string {
	t.Helper()
	records := strings.Split(runFixtureGit(t, dir, "status", "--porcelain", "-uall", "-z"), "\x00")
	var paths []string
	for i := 0; i < len(records); i++ {
		rec := records[i]
		if len(rec) < 4 { // "XY <path>"; the trailing empty record included
			continue
		}
		code, path := rec[:2], rec[3:]
		if code[0] == 'R' || code[0] == 'C' {
			i++ // -z emits a rename/copy's ORIGIN path as the next record
		}
		paths = append(paths, path)
	}
	return paths
}

// snapshotDesignStartData reads every regular file under dir
// (recursively), keyed by its dir-relative slash path; a dir that does not
// exist yet yields an empty, non-nil map — the same "absent" reading a
// pre- and a post-refusal snapshot share when Verdi data was never
// created. Named for its one job (the .verdi/data/ tree behind
// repoGitSnapshot.dataFiles) rather than the generic "snapshotDir", which
// this package's shared test binary already binds to a different helper
// with a different signature.
func snapshotDesignStartData(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return out
		}
		t.Fatalf("snapshotDesignStartData(%s): %v", dir, err)
	}
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		if e.IsDir() {
			for k, v := range snapshotDesignStartData(t, full) {
				out[e.Name()+"/"+k] = v
			}
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			t.Fatalf("snapshotDesignStartData: reading %s: %v", full, err)
		}
		out[e.Name()] = data
	}
	return out
}

// assertRepoGitStateUnchanged fails t naming the specific mismatch the
// moment one field differs between before and after — never a bare
// "state changed" that leaves a reader to guess which fact moved.
func assertRepoGitStateUnchanged(t *testing.T, before, after repoGitSnapshot) {
	t.Helper()
	if before.head != after.head {
		t.Errorf("HEAD changed: before=%s after=%s", before.head, after.head)
	}
	if before.branch != after.branch {
		t.Errorf("current branch changed: before=%q after=%q", before.branch, after.branch)
	}
	if !equalStrings(before.branchNames, after.branchNames) {
		t.Errorf("refs/heads changed: before=%v after=%v", before.branchNames, after.branchNames)
	}
	// Every pre-existing ref must still point at the same object — R1
	// binds refs AND targets, and a branch that was never checked out can
	// be moved without the name list above changing at all.
	for _, name := range before.branchNames {
		was := before.branchTargets[name]
		now, ok := after.branchTargets[name]
		if !ok {
			t.Errorf("branch %s was deleted (it pointed at %s) — R1 forbids deleting an existing branch", name, was)
			continue
		}
		if was != now {
			t.Errorf("branch %s moved: before=%s after=%s — R1 forbids moving an existing branch's target", name, was, now)
		}
	}
	for _, name := range after.branchNames {
		if _, ok := before.branchTargets[name]; !ok {
			t.Errorf("branch %s was created, pointing at %s", name, after.branchTargets[name])
		}
	}
	if before.indexTree != after.indexTree {
		t.Errorf("index tree changed: before=%s after=%s", before.indexTree, after.indexTree)
	}
	if !equalStrings(before.changedPaths, after.changedPaths) {
		t.Errorf("tracked/untracked candidate paths changed: before=%v after=%v", before.changedPaths, after.changedPaths)
	}
	for p, b := range before.changedBytes {
		a, ok := after.changedBytes[p]
		if !ok {
			t.Errorf("path %s present before, absent after", p)
			continue
		}
		if !bytes.Equal(a, b) {
			t.Errorf("path %s bytes changed", p)
		}
	}
	if len(before.dataFiles) != len(after.dataFiles) {
		t.Errorf("Verdi data file count changed: before=%d after=%d", len(before.dataFiles), len(after.dataFiles))
	}
	for p, b := range before.dataFiles {
		a, ok := after.dataFiles[p]
		if !ok {
			t.Errorf(".verdi/data/%s present before, absent after", p)
			continue
		}
		if !bytes.Equal(a, b) {
			t.Errorf(".verdi/data/%s bytes changed", p)
		}
	}
	for p := range after.dataFiles {
		if _, ok := before.dataFiles[p]; !ok {
			t.Errorf(".verdi/data/%s absent before, present after", p)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// hasChangedPath reports whether paths contains want (the snapshot's own
// path list is a small, ordered slice; this file hand-rolls its slice
// helpers, as equalStrings above already does).
func hasChangedPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

// seedUnrelatedRepoState puts a real operator's UNRELATED work into dir's
// checkout before a refusal is provoked: a staged addition, an unstaged
// edit to an already-tracked file, an untracked scratch file, and ignored
// .verdi/data/ bytes.
//
// R1's "leaves Git untouched" is only a meaningful claim against a
// checkout that HAS something to disturb. Over a pristine fixture, a
// refusal that discarded staged work, reverted a tracked edit, deleted an
// untracked file, or rewrote Verdi data would satisfy every snapshot
// comparison unnoticed, because there was nothing there to compare. Every
// path is deliberately unrelated to the spec and branch under test, and
// sits at the repository root (or under an already-partly-tracked
// directory), where `git status` reports each file by its own path.
func seedUnrelatedRepoState(t *testing.T, dir string) {
	t.Helper()
	write := func(rel, content string) {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("seedUnrelatedRepoState: mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("seedUnrelatedRepoState: writing %s: %v", rel, err)
		}
	}

	// Ignored Verdi data: a real store gitignores .verdi/data/, so these
	// bytes never reach `git status` at all and the on-disk snapshot is
	// the only thing that can catch a refusal rewriting them. The
	// .gitignore is written here rather than committed into the shared
	// phase7 fixture, which this file must not alter.
	write(".gitignore", ".verdi/data/\n")
	write(".verdi/data/board/layout.json", "{\"unrelated\":\"board layout bytes the refusal must not touch\"}\n")

	// Staged: a real `git add`ed addition, so the index tree genuinely
	// differs from HEAD before the call (a refusal that ran `git add -A`
	// or committed would take it along).
	write("unrelated-staged.txt", "staged, uncommitted work\n")
	runFixtureGit(t, dir, "add", "--", "unrelated-staged.txt")

	// Unstaged, on an already-TRACKED file — .gitattributes' own comment
	// syntax, inert to every rule the fixture declares. Written AFTER the
	// path-limited `git add` above precisely so it stays unstaged.
	write(".gitattributes", phase7GitAttributes+"# unrelated local edit\n")

	// Untracked: an operator's own scratch file, never staged.
	write("unrelated-untracked.md", "notes the refusal must not touch\n")
}

// assertSeededStateVisible fails fast when the seeded disturbance above
// did not actually register in the snapshot — a silently-ineffective seed
// would make every no-op comparison that follows vacuously true, which is
// exactly the shape of false assurance this test exists to rule out.
func assertSeededStateVisible(t *testing.T, before repoGitSnapshot) {
	t.Helper()
	for _, want := range []string{".gitattributes", ".gitignore", "unrelated-staged.txt", "unrelated-untracked.md"} {
		if !hasChangedPath(before.changedPaths, want) {
			t.Fatalf("test setup: seeded %s is invisible to git status (paths=%v); the no-op assertions below would be vacuous", want, before.changedPaths)
		}
	}
	if len(before.dataFiles) == 0 {
		t.Fatal("test setup: no seeded .verdi/data bytes were captured; the Verdi-data assertions below would be vacuous")
	}
}

// envWithoutDesignAssumeTTY is os.Environ() with any VERDI_DESIGN_ASSUME_TTY
// entry REMOVED — what a built-binary test proving the genuinely
// non-interactive refusal must hand its child process. Merely declining to
// SET the variable is not enough: os/exec hands a nil Env straight to the
// inherited environment, so a developer (or a CI job) that exported
// VERDI_DESIGN_ASSUME_TTY=1 for any reason would silently turn this
// subprocess into an interviewing one and the refusal under test would
// never fire. Removing it makes the child's TTY answer depend only on its
// real (null-device) stdin.
func envWithoutDesignAssumeTTY() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "VERDI_DESIGN_ASSUME_TTY=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// fakeModeManifestYAML is phase7ManifestYAML with providers.jira.mode:
// fake added (store.JiraConfig.Mode; buildProviderRegistry, rollup.go) —
// the config-only switch that selects the in-process
// internal/provider/fake adapter instead of the real Jira one.
//
// Tests that drive design start through cmdDesignStart (or the built
// binary) get their provider from the MANIFEST, not from an injected
// designDeps, so the shared fixture's real base_url would make a
// successful scaffold attempt a live HTTP resolution — a network call,
// which CLAUDE.md forbids outright, token or no token. Derived from the
// shared constant rather than forked from it, and asserted to have
// actually applied, so this can never silently degrade back to the real
// adapter if phase7ManifestYAML's shape changes.
func fakeModeManifestYAML(t *testing.T) string {
	t.Helper()
	const anchor = "  jira:\n"
	if !strings.Contains(phase7ManifestYAML, anchor) {
		t.Fatalf("fakeModeManifestYAML: phase7ManifestYAML no longer carries a %q block, so mode: fake cannot be applied:\n%s", anchor, phase7ManifestYAML)
	}
	return strings.Replace(phase7ManifestYAML, anchor, anchor+"    mode: fake\n", 1)
}

// buildFakeProviderRepo is buildPhase7Repo's hermetic twin: the same
// layer, with fakeModeManifestYAML's verdi.yaml in place of the shared
// one. jira stays a CONFIGURED scheme (ConfiguredStorySchemes reads the
// providers: block's presence, not its mode), so a jira: ref still parses
// and still routes through the real registry construction — it just
// resolves in process. The shared phase7 fixture itself is left untouched.
func buildFakeProviderRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":     fakeModeManifestYAML(t),
				"loansvc/.flowmap.yaml": loansvcFlowmapYAML,
				".gitattributes":        phase7GitAttributes,
			},
			Message: "init store (fake-mode jira provider)",
		},
	})
}

// buildPhase7RepoWithBlockedFeatureTemplate is buildPhase7Repo plus an
// UNCOMMITTED directory sitting at the feature class's own template
// override path, so designscaffold.LoadTemplate's os.ReadFile fails with
// a real "is a directory" error — a template-LOAD failure, distinct from
// designscaffoldoverride_test.go's class-mismatch fixture (whose override
// loads and renders fine, then fails self-validation instead, well after
// the branch is cut). Untracked is deliberate: LoadTemplate reads the
// filesystem directly and does not care whether the path is committed.
func buildPhase7RepoWithBlockedFeatureTemplate(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := buildPhase7Repo(t)
	blocked := filepath.Join(repo.Dir, ".verdi", "templates", "feature.md")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatalf("buildPhase7RepoWithBlockedFeatureTemplate: %v", err)
	}
	return repo
}

// TestRunDesignStart_ProblemOutcomeFlags_TODOFree proves --problem/
// --outcome given together render the scaffold's statement attributes
// with the real supplied text — never the Default* TODO placeholders.
func TestRunDesignStart_ProblemOutcomeFlags_TODOFree(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{},
		Problem: "the real problem", Outcome: "the real outcome"}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1482", "real-statements", manifest, phase7Model(t), deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStart = %d, want 0; stderr=%s", got, stderr.String())
	}
	spec, raw := readSpec(t, repo.Dir, "real-statements")
	if spec.Problem == nil || spec.Problem.Text != "the real problem" {
		t.Fatalf("Problem = %+v, want the supplied text", spec.Problem)
	}
	if spec.Outcome == nil || spec.Outcome.Text != "the real outcome" {
		t.Fatalf("Outcome = %+v, want the supplied text", spec.Outcome)
	}
	if strings.Contains(string(raw), designscaffold.DefaultProblem) || strings.Contains(string(raw), designscaffold.DefaultOutcome) {
		t.Fatalf("scaffold still contains a Default placeholder:\n%s", raw)
	}
}

// TestRunDesignStart_DeferStatements_DisclosesAndKeepsPlaceholders proves
// --defer-statements commits the old TODO placeholders deliberately, with
// an explicit disclosure line on stdout naming the deferral.
func TestRunDesignStart_DeferStatements_DisclosesAndKeepsPlaceholders(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}, DeferStatements: true}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1482", "deferred-statements", manifest, phase7Model(t), deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStart = %d, want 0; stderr=%s", got, stderr.String())
	}
	spec, _ := readSpec(t, repo.Dir, "deferred-statements")
	if spec.Problem == nil || spec.Problem.Text != designscaffold.DefaultProblem {
		t.Fatalf("Problem = %+v, want the Default placeholder (deferred)", spec.Problem)
	}
	if !contains(stdout.String(), "defer") {
		t.Fatalf("stdout = %q, want an explicit deferral disclosure line", stdout.String())
	}
}

// TestRunDesignStart_StatementFlags_Negative covers every preparation
// refusal shape (spec/cli-creation ac-1/ac-2; R1, SI-198): a lone
// --problem/--outcome, --defer-statements combined with either, a
// flagless invocation with no attached terminal, an interview that never
// receives an answer (empty stdin) or aborts partway through (partial
// stdin), and a broken template-override path. Every case uses a valid
// kebab-case --name so name validation itself cannot hide the intended
// failure, and names the exact diagnostic its OWN refusal must print
// (wantStderr) so a different guard refusing first — with the same exit 2
// — can never be mistaken for the shape under test.
//
// Each case seeds real, unrelated work first (seedUnrelatedRepoState) and
// snapshots Git's exact state, that work's bytes, and the candidate spec
// directory and branch before and after: the refusal must be a no-op on
// disk and in Git — never merely "exit 2, don't look further" — and must
// never reach provider title resolution or baseline regeneration, the
// two effects the preparation boundary now runs strictly before.
func TestRunDesignStart_StatementFlags_Negative(t *testing.T) {
	manifest := phase7Manifest(t)
	ctx := context.Background()

	cases := []struct {
		name       string
		specName   string
		kind       artifact.SpecClass
		wantStderr string                              // the refusal's OWN diagnostic, not just "something failed"
		buildRepo  func(t *testing.T) *fixturegit.Repo // nil => buildPhase7Repo
		buildDeps  func(p provider.StoryProvider, r upstream.Runner, g goTestRunner) designDeps
	}{
		{
			name: "problem alone", specName: "refused-problem-alone", kind: artifact.ClassFeature,
			wantStderr: "must be given together",
			buildDeps: func(p provider.StoryProvider, r upstream.Runner, g goTestRunner) designDeps {
				return designDeps{Provider: p, Runner: r, GoTest: g, Problem: "p only"}
			},
		},
		{
			name: "outcome alone", specName: "refused-outcome-alone", kind: artifact.ClassFeature,
			wantStderr: "must be given together",
			buildDeps: func(p provider.StoryProvider, r upstream.Runner, g goTestRunner) designDeps {
				return designDeps{Provider: p, Runner: r, GoTest: g, Outcome: "o only"}
			},
		},
		{
			name: "defer plus problem", specName: "refused-defer-plus-problem", kind: artifact.ClassFeature,
			wantStderr: "cannot be combined with",
			buildDeps: func(p provider.StoryProvider, r upstream.Runner, g goTestRunner) designDeps {
				return designDeps{Provider: p, Runner: r, GoTest: g, DeferStatements: true, Problem: "p"}
			},
		},
		{
			name: "defer plus outcome", specName: "refused-defer-plus-outcome", kind: artifact.ClassFeature,
			wantStderr: "cannot be combined with",
			buildDeps: func(p provider.StoryProvider, r upstream.Runner, g goTestRunner) designDeps {
				return designDeps{Provider: p, Runner: r, GoTest: g, DeferStatements: true, Outcome: "o"}
			},
		},
		{
			name: "no flags, no TTY", specName: "refused-no-flags-no-tty", kind: artifact.ClassFeature,
			wantStderr: "cannot interview",
			buildDeps: func(p provider.StoryProvider, r upstream.Runner, g goTestRunner) designDeps {
				return designDeps{Provider: p, Runner: r, GoTest: g, IsTTY: false}
			},
		},
		{
			name: "empty TTY input (immediate EOF)", specName: "refused-empty-tty-input", kind: artifact.ClassFeature,
			wantStderr: "interview aborted",
			buildDeps: func(p provider.StoryProvider, r upstream.Runner, g goTestRunner) designDeps {
				return designDeps{Provider: p, Runner: r, GoTest: g, IsTTY: true, Stdin: strings.NewReader("")}
			},
		},
		{
			name: "partial TTY input (aborts before Outcome)", specName: "refused-partial-tty-input", kind: artifact.ClassFeature,
			wantStderr: "interview aborted",
			buildDeps: func(p provider.StoryProvider, r upstream.Runner, g goTestRunner) designDeps {
				return designDeps{Provider: p, Runner: r, GoTest: g, IsTTY: true, Stdin: strings.NewReader("only the problem\n")}
			},
		},
		{
			name: "template-load failure (override path is a directory)", specName: "refused-template-load-failure", kind: artifact.ClassFeature,
			// The template LOAD's own error (designscaffold.LoadOverride),
			// never a statement refusal: this case's deps are valid
			// (--defer-statements), so only the blocked override can fail it.
			wantStderr: "reading template override",
			buildRepo:  buildPhase7RepoWithBlockedFeatureTemplate,
			buildDeps: func(p provider.StoryProvider, r upstream.Runner, g goTestRunner) designDeps {
				return designDeps{Provider: p, Runner: r, GoTest: g, DeferStatements: true}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buildRepo := tc.buildRepo
			if buildRepo == nil {
				buildRepo = buildPhase7Repo
			}
			repo := buildRepo(t)
			seedUnrelatedRepoState(t, repo.Dir)
			specDir := filepath.Join(repo.Dir, ".verdi", "specs", "active", tc.specName)
			branch := "design/" + tc.specName

			if _, err := os.Stat(specDir); !os.IsNotExist(err) {
				t.Fatalf("test setup: %s already exists before the call", specDir)
			}
			if has, err := gitx.HasLocalBranch(ctx, repo.Dir, branch); err != nil {
				t.Fatalf("test setup: HasLocalBranch: %v", err)
			} else if has {
				t.Fatalf("test setup: branch %s already exists before the call", branch)
			}

			sp := &spyProvider{StoryProvider: seedFakeProvider(t)}
			sr := &spyRunner{}
			sg := &spyGoTest{}
			deps := tc.buildDeps(sp, sr, sg)

			before := snapshotRepoGitState(t, ctx, repo.Dir)
			assertSeededStateVisible(t, before)

			var stdout, stderr bytes.Buffer
			got := runDesignStart(ctx, repo.Dir, tc.kind, "jira:LOAN-1482", tc.specName, manifest, phase7Model(t), deps, &stdout, &stderr)
			if got != 2 {
				t.Fatalf("runDesignStart(%s) = %d, want 2; stderr=%s", tc.name, got, stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Fatalf("stderr = %q, want the %s refusal, which names %q — every case here exits 2, so a DIFFERENT guard refusing first would otherwise pass unnoticed and leave this shape untested", stderr.String(), tc.name, tc.wantStderr)
			}
			after := snapshotRepoGitState(t, ctx, repo.Dir)
			assertRepoGitStateUnchanged(t, before, after)

			if _, err := os.Stat(specDir); !os.IsNotExist(err) {
				t.Fatalf("preparation refusal left a spec directory at %s", specDir)
			}
			if has, err := gitx.HasLocalBranch(ctx, repo.Dir, branch); err != nil {
				t.Fatalf("HasLocalBranch: %v", err)
			} else if has {
				t.Fatalf("preparation refusal left branch %s behind — the intended branch must be ABSENT, not merely preserved", branch)
			}
			if sp.resolveCalls != 0 {
				t.Fatalf("preparation refusal invoked the story provider %d time(s), want 0 (provider title resolution must follow successful preparation)", sp.resolveCalls)
			}
			if sr.calls != 0 {
				t.Fatalf("preparation refusal invoked the upstream runner %d time(s), want 0 (baseline regeneration must follow successful preparation)", sr.calls)
			}
			if sg.calls != 0 {
				t.Fatalf("preparation refusal invoked go test, want 0 calls")
			}
		})
	}
}

// TestRunDesignStart_TTYInterview_CollectsStatements proves the flagless,
// on-a-TTY path runs the interview (internal/designinterview) over the
// injected Stdin and lands the collected answers as the scaffold's real
// problem/outcome text.
func TestRunDesignStart_TTYInterview_CollectsStatements(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{
		Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{},
		IsTTY: true, Stdin: strings.NewReader("interviewed problem\ninterviewed outcome\n"),
	}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassStory, "jira:LOAN-1482", "interviewed-story", manifest, phase7Model(t), deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStart = %d, want 0; stderr=%s", got, stderr.String())
	}
	spec, _ := readSpec(t, repo.Dir, "interviewed-story")
	if spec.Problem == nil || spec.Problem.Text != "interviewed problem" {
		t.Fatalf("Problem = %+v, want the interviewed text", spec.Problem)
	}
	if spec.Outcome == nil || spec.Outcome.Text != "interviewed outcome" {
		t.Fatalf("Outcome = %+v, want the interviewed text", spec.Outcome)
	}
	if !contains(stdout.String(), "Problem") {
		t.Fatalf("stdout = %q, want the interview's own prompts", stdout.String())
	}
}

// TestRunDesignStart_TTYInterview_Aborted proves a short-stdin abort mid
// interview refuses (exit 2) rather than silently landing a partial or
// empty statement.
func TestRunDesignStart_TTYInterview_Aborted(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{
		Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{},
		IsTTY: true, Stdin: strings.NewReader("only the problem\n"),
	}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1482", "aborted-interview", manifest, phase7Model(t), deps, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runDesignStart(aborted interview) = %d, want 2; stderr=%s", got, stderr.String())
	}
	if _, err := os.Stat(store.ActiveSpecDir(repo.Dir, "aborted-interview")); err == nil {
		t.Fatal("aborted interview left a spec.md on disk; want nothing written")
	}
}

// TestCmdDesignStart_StatementFlags_ParseAndRoundTrip proves --problem/
// --outcome/--defer-statements parse correctly through the REAL flag
// grammar (extractFlags) in every position, mirroring
// TestCmdDesignStart_NameFlagOrdering's own style.
//
// The three succeeding cases build buildFakeProviderRepo, not the shared
// phase7 fixture: cmdDesignStart wires its provider from the MANIFEST
// (buildProviderRegistry), so scaffolding successfully with the shared
// fixture's real base_url would resolve jira:LOAN-1482 over the network.
// The positional ref stays — it is half of what the flag grammar under
// test has to parse around — and resolves in process instead. The
// parse-refusal case below never reaches a provider at all (it exits at
// extractFlags), so it keeps the shared fixture.
func TestCmdDesignStart_StatementFlags_ParseAndRoundTrip(t *testing.T) {
	t.Run("--problem/--outcome round trip", func(t *testing.T) {
		repo := buildFakeProviderRepo(t)
		t.Chdir(repo.Dir)
		var stdout, stderr bytes.Buffer
		args := []string{"jira:LOAN-1482", "--kind", "feature", "--name", "flag-parsed",
			"--problem", "flagged problem", "--outcome", "flagged outcome"}
		got := cmdDesignStart(args, &stdout, &stderr)
		if got != 0 {
			t.Fatalf("cmdDesignStart(%v) = %d, want 0; stderr=%s", args, got, stderr.String())
		}
		spec, _ := readSpec(t, repo.Dir, "flag-parsed")
		if spec.Problem == nil || spec.Problem.Text != "flagged problem" {
			t.Fatalf("Problem = %+v, want the flagged text", spec.Problem)
		}
	})

	t.Run("--problem= form", func(t *testing.T) {
		repo := buildFakeProviderRepo(t)
		t.Chdir(repo.Dir)
		var stdout, stderr bytes.Buffer
		args := []string{"jira:LOAN-1482", "--kind=feature", "--name=flag-eq-parsed",
			"--problem=flagged problem", "--outcome=flagged outcome"}
		got := cmdDesignStart(args, &stdout, &stderr)
		if got != 0 {
			t.Fatalf("cmdDesignStart(%v) = %d, want 0; stderr=%s", args, got, stderr.String())
		}
	})

	t.Run("--defer-statements bare flag", func(t *testing.T) {
		repo := buildFakeProviderRepo(t)
		t.Chdir(repo.Dir)
		var stdout, stderr bytes.Buffer
		args := []string{"jira:LOAN-1482", "--kind", "feature", "--name", "flag-deferred", "--defer-statements"}
		got := cmdDesignStart(args, &stdout, &stderr)
		if got != 0 {
			t.Fatalf("cmdDesignStart(%v) = %d, want 0; stderr=%s", args, got, stderr.String())
		}
	})

	t.Run("--problem given twice refuses", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		t.Chdir(repo.Dir)
		var stdout, stderr bytes.Buffer
		args := []string{"--kind", "feature", "--name", "x", "--problem", "a", "--problem", "b"}
		got := cmdDesignStart(args, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdDesignStart(--problem twice) = %d, want 2", got)
		}
	})
}

// TestDesignGo_NoOwnersFlag is spec/cli-creation ac-4's own negative pin:
// --owners deliberately stays out of design start's flag surface
// (I-10/X-4 ratified posture), disclosed rather than silently
// reconsidered now that this story adds --problem/--outcome/
// --defer-statements/--from-stub. Two proofs, deliberately distinct:
//
//  1. Behavioral — extractFlags' own switch recognizes no "--owners"
//     case, so cmdDesignStart given --owners on the command line treats
//     it as a stray POSITIONAL argument (never a value-carrying flag),
//     which the existing positional-count grammar then refuses.
//  2. Static, on the printed usage text ONLY (the two literal "usage:"
//     lines runDesignVerb/cmdDesignStart print) — never the whole file's
//     source, which legitimately documents the --owners absence in prose
//     (a doc comment mentioning the word is not the flag existing).
func TestDesignGo_NoOwnersFlag(t *testing.T) {
	t.Run("behavioral: --owners is a stray positional, not a flag", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		t.Chdir(repo.Dir)
		var stdout, stderr bytes.Buffer
		// A feature takes at most one positional ref; "--owners" and
		// "alice" both land in rest (extractFlags recognizes neither),
		// giving two positionals — the existing "too many positionals"
		// refusal fires, proof --owners was never consumed as a flag.
		got := cmdDesignStart([]string{"--kind", "feature", "--name", "x", "--owners", "alice"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdDesignStart(--owners alice) = %d, want 2 (stray positional refusal)", got)
		}
	})

	t.Run("static: the printed usage text never mentions --owners", func(t *testing.T) {
		// Each call below is chosen to hit an actual "usage: ..." print
		// site (verified against design.go/designfromstub.go's source):
		// runDesignVerb's subcommand mismatch, cmdDesignStart's too-many-
		// positionals default arm, and cmdDesignStartFromStub's arg-count
		// guard.
		var stderr1 bytes.Buffer
		runDesignVerb([]string{"bogus"}, &bytes.Buffer{}, &stderr1)
		var stderr2 bytes.Buffer
		cmdDesignStart([]string{"--kind", "feature", "--name", "x", "one", "two"}, &bytes.Buffer{}, &stderr2)
		var stderr3 bytes.Buffer
		cmdDesignStartFromStub(nil, &bytes.Buffer{}, &stderr3)
		for i, s := range []string{stderr1.String(), stderr2.String(), stderr3.String()} {
			if !contains(s, "usage") {
				t.Fatalf("case #%d did not hit a usage print at all (test setup drifted from design.go's refusal shape): %q", i+1, s)
			}
			if strings.Contains(s, "owners") {
				t.Errorf("usage text #%d mentions owners: %q", i+1, s)
			}
		}
	})
}

// TestRun_DesignStart_TTYInterview_BuiltBinary is the real, built-binary
// proof of the flagless TTY interview end to end: VERDI_DESIGN_ASSUME_TTY=1
// stands in for the TTY predicate (this story's own disclosed stdin-script
// harness, mirroring init_test.go's VERDI_INIT_ASSUME_TTY), a scripted
// stdin answers the interview's two prompts, and the committed scaffold
// carries the real answers.
//
// No tracker operand: a feature's ref is OPTIONAL (05 §CLI), and nothing
// this test asserts depends on one, while a real binary given a real
// jira: ref would resolve its title through the manifest-configured
// adapter — a live HTTP call from a test, which CLAUDE.md forbids.
// Omitting it leaves the interview path, the only thing under test here,
// exactly as it was.
func TestRun_DesignStart_TTYInterview_BuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildPhase7Repo(t)

	cmd := exec.Command(bin, "design", "start", "--kind", "feature", "--name", "built-binary-interview")
	cmd.Dir = repo.Dir
	// The one place the override belongs: this test's whole subject is the
	// interview, which needs the TTY predicate to answer yes. Appended
	// last, so it wins over any ambient value.
	cmd.Env = append(os.Environ(), "VERDI_DESIGN_ASSUME_TTY=1")
	cmd.Stdin = strings.NewReader("built-binary interviewed problem\nbuilt-binary interviewed outcome\n")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("verdi design start (TTY interview): %v\nstdout:\n%s\nstderr:\n%s", err, outBuf.String(), errBuf.String())
	}

	spec, _ := readSpec(t, repo.Dir, "built-binary-interview")
	if spec.Problem == nil || spec.Problem.Text != "built-binary interviewed problem" {
		t.Fatalf("Problem = %+v, want the interviewed text", spec.Problem)
	}
	if spec.Outcome == nil || spec.Outcome.Text != "built-binary interviewed outcome" {
		t.Fatalf("Outcome = %+v, want the interviewed text", spec.Outcome)
	}
}

// TestRun_DesignStart_NoTTY_NoFlags_Refuses_BuiltBinary is the built-binary
// twin of the no-TTY refusal: with VERDI_DESIGN_ASSUME_TTY removed from
// the child's environment, a real subprocess genuinely has no attached
// terminal, and design start refuses by name rather than silently
// emitting the old TODO placeholders.
//
// No tracker operand, for the same reason the interview twin above omits
// one — and here it also has to hold while the boundary is still being
// moved: a runtime that resolves the provider BEFORE reaching this
// refusal would otherwise make the RED run itself egress.
func TestRun_DesignStart_NoTTY_NoFlags_Refuses_BuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildPhase7Repo(t)

	cmd := exec.Command(bin, "design", "start", "--kind", "feature", "--name", "no-tty-no-flags")
	cmd.Dir = repo.Dir
	cmd.Env = envWithoutDesignAssumeTTY()
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("verdi design start (no TTY, no flags): want an ExitError, got %v\nstdout:\n%s", err, outBuf.String())
	}
	if ee.ExitCode() != 2 {
		t.Fatalf("exit code = %d, want 2\nstderr:\n%s", ee.ExitCode(), errBuf.String())
	}
	if !contains(errBuf.String(), "cannot interview") {
		t.Fatalf("stderr = %q, want the cannot-interview refusal", errBuf.String())
	}
}

// TestRun_DesignStart_NoTTYRefusalThenSameNameRetry_BuiltBinary is the
// built-binary reproduction of the MVP adoption baseline's B-02/B-03
// witness (SI-198, R1): a flagless, genuinely non-interactive invocation
// refuses (exit 2) leaving Git untouched, so the exact retry its own
// diagnostic invites — the SAME name, now with both statement flags —
// succeeds outright. Before this story, the first (refused) call already
// left design/<name> checked out with nothing on it, so the identical
// retry failed on "branch already exists" instead — the installed
// baseline's own observed opposite result (docs/superpowers/reports/
// 2026-09-13-mvp-adoption-baseline.md, B-03).
//
// Two deliberate choices about this fixture. No tracker operand: a
// feature's ref is optional (05 §CLI) and this test asserts nothing about
// one, while a real jira: ref through the real binary would resolve over
// the network — including during RED, where the baseline runtime still
// reaches provider resolution before refusing. And no seeded unrelated
// work, unlike the negative table's own refusals: Phase B here really
// does commit (`git add -A`), so any sentinel left lying around would be
// swept into the scaffold commit this test then inspects.
func TestRun_DesignStart_NoTTYRefusalThenSameNameRetry_BuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	const name = "same-name-retry"
	branch := "design/" + name

	before := snapshotRepoGitState(t, ctx, repo.Dir)

	// Phase A: flagless, real non-TTY stdin — cmd.Stdin left nil, which
	// os/exec connects to the null device (never a terminal), matching
	// this file's own TestRun_DesignStart_NoTTY_NoFlags_Refuses_BuiltBinary
	// and the baseline's own reproduction. VERDI_DESIGN_ASSUME_TTY is
	// REMOVED from the child environment, not merely left unset here: an
	// ambient export would otherwise send this invocation into the
	// interview and the refusal under test would never fire at all.
	cmd1 := exec.Command(bin, "design", "start", "--kind", "feature", "--name", name)
	cmd1.Dir = repo.Dir
	cmd1.Env = envWithoutDesignAssumeTTY()
	var out1, err1 bytes.Buffer
	cmd1.Stdout = &out1
	cmd1.Stderr = &err1
	runErr := cmd1.Run()
	ee, ok := runErr.(*exec.ExitError)
	if !ok {
		t.Fatalf("first invocation: want an ExitError, got %v\nstdout:\n%s", runErr, out1.String())
	}
	if ee.ExitCode() != 2 {
		t.Fatalf("first invocation exit = %d, want 2\nstderr:\n%s", ee.ExitCode(), err1.String())
	}

	afterRefusal := snapshotRepoGitState(t, ctx, repo.Dir)
	assertRepoGitStateUnchanged(t, before, afterRefusal)
	if has, err := gitx.HasLocalBranch(ctx, repo.Dir, branch); err != nil {
		t.Fatalf("HasLocalBranch: %v", err)
	} else if has {
		t.Fatalf("first (refused) invocation left branch %s behind — the SI-198 defect this story fixes", branch)
	}
	if _, statErr := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", name)); !os.IsNotExist(statErr) {
		t.Fatalf("first (refused) invocation left a spec directory behind (stat err = %v)", statErr)
	}

	// Phase B: the retry the refusal's own diagnostic advises — the SAME
	// name, now with both statement flags — must succeed outright, never
	// fail on a branch the refusal itself left behind.
	cmd2 := exec.Command(bin, "design", "start", "--kind", "feature", "--name", name,
		"--problem", "Readers cannot distinguish current evidence from missing evidence.",
		"--outcome", "Readers can inspect evidence and identify the next action for one small change.")
	cmd2.Dir = repo.Dir
	cmd2.Env = envWithoutDesignAssumeTTY()
	var out2, err2 bytes.Buffer
	cmd2.Stdout = &out2
	cmd2.Stderr = &err2
	if err := cmd2.Run(); err != nil {
		t.Fatalf("retry with paired statement flags: %v\nstdout:\n%s\nstderr:\n%s", err, out2.String(), err2.String())
	}

	gotBranch, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if gotBranch != branch {
		t.Fatalf("CurrentBranch after retry = %q, want %q", gotBranch, branch)
	}
	ahead, behind, err := gitx.AheadBehind(ctx, repo.Dir, repo.Head, "HEAD")
	if err != nil {
		t.Fatalf("AheadBehind: %v", err)
	}
	if ahead != 0 || behind != 1 {
		t.Fatalf("(ahead, behind) between repo.Head and the retry's HEAD = (%d, %d), want exactly one new commit (0, 1)", ahead, behind)
	}

	spec, _ := readSpec(t, repo.Dir, name)
	if spec.Problem == nil || spec.Problem.Text != "Readers cannot distinguish current evidence from missing evidence." {
		t.Fatalf("Problem = %+v, want the supplied, strict-decoded text", spec.Problem)
	}
	if spec.Outcome == nil || spec.Outcome.Text != "Readers can inspect evidence and identify the next action for one small change." {
		t.Fatalf("Outcome = %+v, want the supplied, strict-decoded text", spec.Outcome)
	}
}
