// storecoords_test.go is the nested-store regression suite for the
// ownership recognizers (owner risk review F3): the store root may sit
// BELOW the git root (store.FindRoot walks up to the nearest .verdi),
// and git answers every listing in REPOSITORY-root-relative paths. A
// recognizer that compares those answers against store-relative zone
// prefixes misclassifies the whole layout, and the advice it emits is
// then wrong twice over — the wrong state, and paths that do not resolve.
//
// Every case here proves BOTH halves: the state is recognized, and the
// manual commands it emits, run from the REPOSITORY ROOT exactly as
// written, reach the state they promise.
package recovery

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// fixtureNestedStore is fixtureStore's nested-layout twin: the same
// spec/checkout store, one directory BELOW the git root. It returns the
// repository, the resolved store.Config rooted at product/, and the
// store root's own repository-relative prefix ("product/").
func fixtureNestedStore(t *testing.T) (*fixturegit.Repo, *store.Config, string) {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "") // see fixtureStore's own comment
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				"above.txt":                 "above the store root\n",
				"product/.verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: gitlab\n",
				"product/.verdi/specs/active/checkout/spec.md": checkoutSpecMD,
			},
			Message: "scaffold a store root one level below the git root",
		},
	})
	root := filepath.Join(repo.Dir, "product")
	cfg, err := store.Open(root)
	if err != nil {
		t.Fatalf("store.Open(%s): %v", root, err)
	}
	cfg.Root = root
	return repo, cfg, "product/"
}

// runEmittedCommand runs one emitted manual command verbatim, from the
// REPOSITORY ROOT — the working directory the projection's own commands
// are written for. `sh -c` is deliberate: the point is that the sentence
// an operator copies works as written, not that a parsed approximation
// of it does.
func runEmittedCommand(t *testing.T, repoRoot, command string) {
	t.Helper()
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("emitted command %q, run from the repository root: %v\n%s", command, err, out)
	}
}

// choiceFor returns the one choice state code offers for target.
func choiceFor(t *testing.T, p Projection, code StateCode, target string) Choice {
	t.Helper()
	s, ok := stateFor(p.States, code, target)
	if !ok {
		t.Fatalf("no %s state for %s in %+v", code, target, p.States)
	}
	if len(s.Choices) != 1 {
		t.Fatalf("%s/%s offers %d choices, want exactly one: %+v", code, target, len(s.Choices), s.Choices)
	}
	return s.Choices[0]
}

// TestDerive_NestedStore_StagedClosureIsRecognized is F3's headline: an
// interrupted close in the supported nested layout stages
// product/.verdi/specs/{active,archive}/checkout, which the ownership
// predicate must read as spec/checkout's OWN closure paths —
// artifacts-staged-uncommitted, not the archive-move state the disk
// shape alone also matches. Following the emitted abandon advice from
// the repository root must leave nothing staged.
func TestDerive_NestedStore_StagedClosureIsRecognized(t *testing.T) {
	repo, cfg, prefix := fixtureNestedStore(t)
	cutEmptyBranch(t, repo, "close/checkout")
	stageNestedClosurePaths(t, repo, cfg.Root)

	p := Derive(mustGather(t, cfg, "spec/checkout"))
	mustValidate(t, p)

	if !hasState(p, StateArtifactsStagedUncommitted) {
		t.Fatalf("no artifacts-staged-uncommitted state in the nested layout: %+v", p.States)
	}
	if hasState(p, StateArchiveMoveUncommitted) {
		t.Fatalf("archive-move-uncommitted must not fire alongside the staged closure: %+v", p.States)
	}

	choice := choiceFor(t, p, StateArtifactsStagedUncommitted, "close/checkout")
	activeRel := prefix + store.SpecDirRelPath(store.ZoneActive, "checkout")
	archiveRel := prefix + store.SpecDirRelPath(store.ZoneArchive, "checkout")
	for _, want := range []string{activeRel, archiveRel} {
		if !strings.Contains(strings.Join(choice.ManualCommands, "\n"), want) {
			t.Fatalf("manual commands %v do not name %q as git named it", choice.ManualCommands, want)
		}
	}

	// The abandon path: every command after "git commit", run verbatim
	// from the repository root.
	for _, command := range choice.ManualCommands[1:] {
		runEmittedCommand(t, repo.Dir, command)
	}
	staged, err := gitx.StagedPaths(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("StagedPaths after following the emitted advice: %v", err)
	}
	if len(staged) != 0 {
		t.Fatalf("after following the emitted advice from the repository root, staged paths remain: %v", staged)
	}
}

// TestDerive_NestedStore_ScaffoldUnstagedIsRecognized is the same
// coordinate defect in the scaffold recognizer: its active-zone prefix
// is store-relative, and the changed paths it filters are
// repository-relative. The emitted `git add` must stage the scaffold
// edit when run from the repository root.
func TestDerive_NestedStore_ScaffoldUnstagedIsRecognized(t *testing.T) {
	repo, cfg, prefix := fixtureNestedStore(t)
	cutEmptyBranch(t, repo, "design/checkout")
	writeNestedUnstagedScaffold(t, cfg.Root)

	p := Derive(mustGather(t, cfg, "spec/checkout"))
	mustValidate(t, p)

	choice := choiceFor(t, p, StateScaffoldUnstaged, "design/checkout")
	wantPrefix := prefix + store.SpecDirRelPath(store.ZoneActive, "checkout") + "/"
	if !strings.Contains(choice.ManualCommands[0], wantPrefix) {
		t.Fatalf("manual command %q does not name the active zone as git named it (%q)", choice.ManualCommands[0], wantPrefix)
	}

	runEmittedCommand(t, repo.Dir, choice.ManualCommands[0])
	staged, err := gitx.StagedPaths(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("StagedPaths after following the emitted advice: %v", err)
	}
	want := prefix + store.SpecRelPath(store.ZoneActive, "checkout")
	if len(staged) != 1 || staged[0] != want {
		t.Fatalf("staged = %v after the emitted `git add`, want exactly [%s]", staged, want)
	}
}

// TestDerive_NestedStore_ArchiveMoveUncommittedIsRecognized covers the
// third recognizer built out of store.SpecDirRelPath: its state comes
// from the on-disk/at-HEAD shape (which is coordinate-free), but the
// commands it emits are not — they must restore the active zone when run
// from the repository root.
func TestDerive_NestedStore_ArchiveMoveUncommittedIsRecognized(t *testing.T) {
	repo, cfg, prefix := fixtureNestedStore(t)
	cutEmptyBranch(t, repo, "close/checkout")
	moveNestedArchiveUncommitted(t, cfg.Root)

	p := Derive(mustGather(t, cfg, "spec/checkout"))
	mustValidate(t, p)

	choice := choiceFor(t, p, StateArchiveMoveUncommitted, "close/checkout")
	activeRel := prefix + store.SpecDirRelPath(store.ZoneActive, "checkout")
	if !strings.Contains(choice.ManualCommands[0], activeRel) {
		t.Fatalf("manual command %q does not name the active zone as git named it (%q)", choice.ManualCommands[0], activeRel)
	}

	for _, command := range choice.ManualCommands {
		runEmittedCommand(t, repo.Dir, command)
	}
	if _, err := os.Stat(store.ActiveSpecPath(cfg.Root, "checkout")); err != nil {
		t.Fatalf("after following the emitted advice from the repository root, the active zone is still gone: %v", err)
	}
	if _, err := os.Stat(store.ArchiveSpecDir(cfg.Root, "checkout")); !os.IsNotExist(err) {
		t.Fatalf("after following the emitted advice, the leftover archive directory survives (stat err = %v)", err)
	}
}

// TestDerive_NestedStore_ForeignStagedPathIsNeverClaimed is the refusal
// posture the re-basing must not erode: a staged path OUTSIDE the store
// root disowns the whole index, so no ownership state is claimed at all.
func TestDerive_NestedStore_ForeignStagedPathIsNeverClaimed(t *testing.T) {
	repo, cfg, _ := fixtureNestedStore(t)
	cutEmptyBranch(t, repo, "close/checkout")
	stageNestedClosurePaths(t, repo, cfg.Root)
	if err := os.WriteFile(filepath.Join(repo.Dir, "above.txt"), []byte("someone else's edit\n"), 0o644); err != nil {
		t.Fatalf("editing above.txt: %v", err)
	}
	runGit(t, repo.Dir, "add", "--", "above.txt")

	p := Derive(mustGather(t, cfg, "spec/checkout"))
	mustValidate(t, p)

	if hasState(p, StateArtifactsStagedUncommitted) {
		t.Fatalf("artifacts-staged-uncommitted claimed over an index carrying work outside the store: %+v", p.States)
	}
}

// TestDerive_UnresolvedRepoPrefix_OwnershipStates is the negative half
// of the coordinate bridge: when the store root's own repository prefix
// could not be read at all (Gather discloses it), the two vocabularies
// are unrelatable and nothing assumes they coincide — the assumption
// that produced F3 in the nested layout.
//
// R-RR3-26 splits what "not assuming" means by what the prefix is FOR.
// Where it answers the recognition question itself (which paths under
// the store the index or the working tree carries), there is no state to
// describe and the recognizer withholds. Where the state is already
// proved off disk and HEAD and the prefix only names its paths the way
// git would, the state is emitted with no choice and an uncertainty
// carrying `git rev-parse --show-prefix`. No single git failure reaches
// the prefix guard alone — `rev-parse --show-prefix` fails only where
// `git status` fails with it — so this case is built through the
// package's own fact-construction seam, Derive over a hand-built Facts.
func TestDerive_UnresolvedRepoPrefix_OwnershipStates(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(f *Facts)
		code  StateCode
		// diagnosisOnly is R-RR3-26's split: true when the prefix is
		// needed only to DESCRIBE the state, false when it is needed to
		// RECOGNIZE it.
		diagnosisOnly bool
	}{
		{
			name: "artifacts-staged-uncommitted is withheld: the prefix decides whose index this is",
			setup: func(f *Facts) {
				f.StagedPaths = []string{".verdi/specs/active/checkout/spec.md", ".verdi/specs/archive/checkout/spec.md"}
			},
			code: StateArtifactsStagedUncommitted,
		},
		{
			name: "scaffold-unstaged is withheld: the prefix decides which changed paths are the scaffold",
			setup: func(f *Facts) {
				f.Design = RitualBranch{Name: "design/checkout", Exists: true, Tip: "d1"}
				f.CurrentBranch = "design/checkout"
				f.WorktreeChangedPaths = []string{".verdi/specs/active/checkout/spec.md"}
			},
			code: StateScaffoldUnstaged,
		},
		{
			name: "archive-move-uncommitted is diagnosed: disk and HEAD already prove it",
			setup: func(f *Facts) {
				f.ArchiveSpecOnDisk = true
				f.ActiveSpecAtHead = true
			},
			code:          StateArchiveMoveUncommitted,
			diagnosisOnly: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observed := baseFacts()
			tc.setup(&observed)
			if !hasState(Derive(observed), tc.code) {
				t.Fatalf("fixture does not produce %s with the prefix observed; the case below would pass for the wrong reason", tc.code)
			}

			unobserved := observed
			unobserved.RepoPrefix = ""
			unobserved.RepoPrefixObserved = false
			p := Derive(unobserved)
			mustValidate(t, p)

			if !tc.diagnosisOnly {
				if hasState(p, tc.code) {
					t.Fatalf("%s claimed with no way to relate git's paths to the store's: %+v", tc.code, p.States)
				}
				return
			}
			s := stateByCode(t, p, tc.code)
			if len(s.Choices) != 0 {
				t.Fatalf("Choices = %+v, want none: no command can be named in git's coordinates without the prefix", s.Choices)
			}
			u := uncertaintyNaming(t, s, "path inside the repository could not be observed")
			if u.Witness != repoPrefixWitness {
				t.Fatalf("witness = %q, want %q", u.Witness, repoPrefixWitness)
			}
			for _, fact := range s.Facts {
				if !strings.HasPrefix(fact, ".verdi/specs/") {
					t.Fatalf("fact %q does not name the path in the one vocabulary this run can prove, the store's", fact)
				}
			}
		})
	}
}

// stageNestedClosurePaths is stageClosurePaths for a store root below the
// git root: git is driven from the STORE root (as every verb does), so
// the pathspecs stay store-relative while git's own answers do not.
func stageNestedClosurePaths(t *testing.T, repo *fixturegit.Repo, root string) {
	t.Helper()
	moveNestedArchiveUncommitted(t, root)
	active := store.SpecDirRelPath(store.ZoneActive, "checkout")
	archive := store.SpecDirRelPath(store.ZoneArchive, "checkout")
	if err := gitx.AddPaths(context.Background(), root, active, archive); err != nil {
		t.Fatalf("staging closure paths: %v", err)
	}
}

// moveNestedArchiveUncommitted is moveArchiveUncommitted against an
// arbitrary store root.
func moveNestedArchiveUncommitted(t *testing.T, root string) {
	t.Helper()
	activeDir := store.ActiveSpecDir(root, "checkout")
	archiveDir := store.ArchiveSpecDir(root, "checkout")
	if err := os.MkdirAll(filepath.Dir(archiveDir), 0o755); err != nil {
		t.Fatalf("creating archive zone: %v", err)
	}
	if err := os.Rename(activeDir, archiveDir); err != nil {
		t.Fatalf("moving %s to %s: %v", activeDir, archiveDir, err)
	}
}

// writeNestedUnstagedScaffold is writeUnstagedScaffold against an
// arbitrary store root.
func writeNestedUnstagedScaffold(t *testing.T, root string) {
	t.Helper()
	path := store.ActiveSpecPath(root, "checkout")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	data = append(data, []byte("\n<!-- scaffold in progress -->\n")...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// fixtureNestedStoreNoSpec is fixtureNestedStore without spec/checkout
// at all — the nested twin of fixtureStoreNoSpec, so a fallback-chain
// case can plant the spec at exactly one location that is not the
// current checkout.
func fixtureNestedStoreNoSpec(t *testing.T) (*fixturegit.Repo, *store.Config, string) {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "") // see fixtureStore's own comment
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				"above.txt":                 "above the store root\n",
				"product/.verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: gitlab\n",
			},
			Message: "scaffold a store root one level below the git root",
		},
	})
	root := filepath.Join(repo.Dir, "product")
	cfg, err := store.Open(root)
	if err != nil {
		t.Fatalf("store.Open(%s): %v", root, err)
	}
	cfg.Root = root
	return repo, cfg, "product/"
}

// plantSpecOnDesignBranch commits spec/checkout's active-zone spec.md on
// design/checkout and returns to main, leaving the spec visible in NO
// on-disk zone of the current checkout — specClassAt's own gitx.Show
// fallback is then the only thing that can resolve its class.
func plantSpecOnDesignBranch(t *testing.T, root string) {
	t.Helper()
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, root, "design/checkout"); err != nil {
		t.Fatalf("CheckoutNewBranch(design/checkout): %v", err)
	}
	if err := os.MkdirAll(store.ActiveSpecDir(root, "checkout"), 0o755); err != nil {
		t.Fatalf("creating the active spec dir: %v", err)
	}
	if err := os.WriteFile(store.ActiveSpecPath(root, "checkout"), []byte(checkoutSpecMD), 0o644); err != nil {
		t.Fatalf("writing spec.md: %v", err)
	}
	if err := gitx.AddPaths(ctx, root, store.SpecDirRelPath(store.ZoneActive, "checkout")); err != nil {
		t.Fatalf("AddPaths: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, root, "design: scaffold spec/checkout"); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
	if err := gitx.CheckoutExisting(ctx, root, "main"); err != nil {
		t.Fatalf("CheckoutExisting(main): %v", err)
	}
	if pathExists(store.ActiveSpecPath(root, "checkout")) {
		t.Fatal("test setup bug: spec/checkout is visible on disk on main")
	}
}

// TestSpecClassAt_NestedStore_ResolvesFromRitualBranchTree is R-RR3-27:
// `git show <rev>:<path>` resolves its path against the REPOSITORY root
// (unlike `git ls-tree`'s pathspec, which is cwd-relative and so already
// correct here), so specClassAt's own fallback chain must ask for the
// store-relative path rebased through the store root's repository
// prefix. Without it, a nested store whose spec lives only in a ritual
// branch's tree makes Gather an operational error — exit 2 where the
// operator is owed a diagnosis (ac-8).
func TestSpecClassAt_NestedStore_ResolvesFromRitualBranchTree(t *testing.T) {
	_, cfg, prefix := fixtureNestedStoreNoSpec(t)
	plantSpecOnDesignBranch(t, cfg.Root)

	class, err := specClassAt(context.Background(), cfg.Root, "checkout", "", prefix, true)
	if err != nil {
		t.Fatalf("specClassAt: %v, want the class read from design/checkout's own tree in the nested layout", err)
	}
	if class != artifact.ClassFeature {
		t.Fatalf("class = %q, want feature", class)
	}
	if f := mustGather(t, cfg, "spec/checkout"); !f.Design.Exists {
		t.Fatal("Design.Exists = false, want true")
	}
}

// TestSpecClassAt_UnobservedPrefix_SkipsTheGitTreeFallbacks is the
// negative half: with the store root's own repository prefix unobserved,
// the path `git show` would resolve cannot be named at all, so the tree
// fallbacks are skipped rather than asked a question in the wrong
// coordinates. The refusal names only the locations actually tried and
// says why the rest were not.
func TestSpecClassAt_UnobservedPrefix_SkipsTheGitTreeFallbacks(t *testing.T) {
	_, cfg, _ := fixtureNestedStoreNoSpec(t)
	plantSpecOnDesignBranch(t, cfg.Root)

	_, err := specClassAt(context.Background(), cfg.Root, "checkout", "main", "", false)
	if err == nil {
		t.Fatal("specClassAt resolved a class through a path it had no way to name in git's coordinates")
	}
	if strings.Contains(err.Error(), "design/checkout") {
		t.Fatalf("refusal %q lists a location that was never tried", err)
	}
	if !strings.Contains(err.Error(), specClassPrefixSkipped) {
		t.Fatalf("refusal %q does not say the tree fallbacks were skipped, and why", err)
	}
}
