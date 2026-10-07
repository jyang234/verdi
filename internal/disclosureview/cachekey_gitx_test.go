package disclosureview

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// argvLog records every git argv gitx runs on its context, in order.
type argvLog struct {
	mu   sync.Mutex
	argv []string
}

func (l *argvLog) Observe(_ string, args []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.argv = append(l.argv, strings.Join(args, " "))
}

// TestKey_GitReadsGoThroughGitx proves the key's git reads run through
// gitx, so the context's observer sees every one of them
// (spec/gitx-recorder-seam dc-3): the version, the layout rev-parse, the
// ref list, the configuration and ls-files, in that order and with the
// argv the key has always run.
func TestKey_GitReadsGoThroughGitx(t *testing.T) {
	fx := newCacheFixture(t)
	log := &argvLog{}
	if _, err := readInputs(gitx.WithObserver(context.Background(), log), fx.root); err != nil {
		t.Fatalf("readInputs: %v", err)
	}
	want := []string{
		"version",
		"rev-parse --path-format=absolute --git-dir --git-common-dir --git-path objects --git-path shallow --git-path info/grafts --is-shallow-repository HEAD --symbolic-full-name HEAD",
		"for-each-ref --format=%(refname)%00%(objectname)%00%(symref)",
		"config --list -z",
	}
	if len(log.argv) != len(want)+1 || strings.Join(log.argv[:len(want)], "\n") != strings.Join(want, "\n") || !strings.HasPrefix(log.argv[len(want)], "ls-files") {
		t.Fatalf("observed git reads:\n  %s\nwant:\n  %s\n  ls-files ...", strings.Join(log.argv, "\n  "), strings.Join(want, "\n  "))
	}
}

// TestKey_ByteIdenticalToPreChangeKey proves moving the key's git reads
// behind gitx changed no key byte (dc-3): for each repository shape the
// key and the stamps equal those of the key as computed before the move,
// kept below verbatim as legacyReadInputs. The shapes cover what the git
// reads key: refs (loose, packed, symbolic, a tag), a linked worktree
// whose git directory is not the common one, a detached HEAD, a store
// below the top level, and a shallow clone whose shallow file exists.
func TestKey_ByteIdenticalToPreChangeKey(t *testing.T) {
	shapes := []struct {
		name  string
		store func(t *testing.T) string
	}{
		{"a store at the top level of its main worktree", func(t *testing.T) string { return newCacheFixture(t).root }},
		{"a store below the top level of a linked worktree on a detached HEAD, refs packed", linkedWorktreeStore},
		{"a store at the top level of a shallow clone", shallowCloneStore},
	}
	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			root := shape.store(t)
			legacy, err := legacyReadInputs(context.Background(), root)
			if err != nil {
				t.Fatalf("legacyReadInputs: %v", err)
			}
			got, err := readInputs(context.Background(), root)
			if err != nil {
				t.Fatalf("readInputs: %v", err)
			}
			if got.key != legacy.key || got.stamps != legacy.stamps || !got.newest.Equal(legacy.newest) {
				t.Fatalf("the key moved:\n  pre-change %+v\n  now        %+v", legacy, got)
			}
		})
	}
	t.Run("a different repository state still keys differently", func(t *testing.T) {
		fx := newCacheFixture(t)
		before, err := readInputs(context.Background(), fx.root)
		if err != nil {
			t.Fatal(err)
		}
		gitIn(t, fx.root, "tag", "moved")
		after, err := readInputs(context.Background(), fx.root)
		if err != nil {
			t.Fatal(err)
		}
		if before.key == after.key {
			t.Fatal("a new tag left the key unchanged: the comparison above would pass vacuously")
		}
	})
}

// linkedWorktreeStore builds the fixture store below the top level, packs
// its refs, adds a tag, and returns the store inside a linked worktree
// detached at the first commit.
func linkedWorktreeStore(t *testing.T) string {
	t.Helper()
	for _, v := range ciEnvVars {
		t.Setenv(v, "")
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Message: "seed the store below the top level", Files: map[string]string{
			"store/.verdi/verdi.yaml":                         manifestYAML,
			"store/.verdi/.gitignore":                         "data/\n",
			"store/.verdi/specs/active/panel-fixture/spec.md": storySpecMD,
		}},
		{Message: "second layer", Files: map[string]string{"README.md": "top\n"}},
	})
	gitIn(t, repo.Dir, "tag", "v1", repo.Heads[0])
	gitIn(t, repo.Dir, "update-ref", "refs/remotes/origin/main", repo.Head)
	gitIn(t, repo.Dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	gitIn(t, repo.Dir, "pack-refs", "--all")
	linked := filepath.Join(t.TempDir(), "linked")
	gitIn(t, repo.Dir, "worktree", "add", "--detach", linked, repo.Heads[0])
	return filepath.Join(linked, "store")
}

// shallowCloneStore clones the fixture store at depth one over the local
// file protocol, so the clone's shallow file exists.
func shallowCloneStore(t *testing.T) string {
	t.Helper()
	fx := newCacheFixture(t)
	clone := filepath.Join(t.TempDir(), "clone")
	gitIn(t, t.TempDir(), "clone", "--quiet", "--depth", "1", "file://"+filepath.ToSlash(fx.root), clone)
	if got := gitIn(t, clone, "rev-parse", "--is-shallow-repository"); got != "true" {
		t.Fatalf("the clone is not shallow (%q): this shape would not cover the shallow file", got)
	}
	return clone
}

// ---- the pre-change key, verbatim ----

// legacyReadInputs is readInputs as it was before the key's git reads
// moved behind gitx (dc-3), with its git step legacyGit. It is the
// reference TestKey_ByteIdenticalToPreChangeKey compares against; it is
// never to be edited to follow readInputs.
func legacyReadInputs(ctx context.Context, root string) (inputs, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return inputs{}, uncomputable("resolving root", err)
	}
	r := &inputReader{key: sha256.New(), stamps: sha256.New()}
	r.field("root", []byte(abs))
	r.environment()
	steps := []func() error{
		func() error { return r.legacyGit(ctx, abs) },
		func() error { return r.verdiTree(abs) },
		func() error { return r.mutableZone(abs) },
		func() error { return r.services(abs) },
		func() error { return r.rootFiles(abs) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return inputs{}, err
		}
	}
	return inputs{
		key:    hex.EncodeToString(r.key.Sum(nil)),
		stamps: hex.EncodeToString(r.stamps.Sum(nil)),
		newest: r.newest,
	}, nil
}

// legacyGitOutput is the pre-change gitOutput: one read-only git command
// run directly, outside gitx.
func legacyGitOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, uncomputable("git "+strings.Join(args, " "), fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String())))
	}
	return stdout.Bytes(), nil
}

// legacyGit is the pre-change (*inputReader).git.
func (r *inputReader) legacyGit(ctx context.Context, root string) error {
	if v := os.Getenv("GIT_ALTERNATE_OBJECT_DIRECTORIES"); v != "" {
		return uncomputable("object store", errors.New("GIT_ALTERNATE_OBJECT_DIRECTORIES names a second object store"))
	}
	version, err := legacyGitOutput(ctx, root, "version")
	if err != nil {
		return err
	}
	r.field("git version", version)

	revParse, err := legacyGitOutput(ctx, root, "rev-parse", "--path-format=absolute",
		"--git-dir", "--git-common-dir",
		"--git-path", "objects", "--git-path", "shallow", "--git-path", "info/grafts",
		"--is-shallow-repository", "HEAD", "--symbolic-full-name", "HEAD")
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(revParse), "\n"), "\n")
	if len(lines) != 8 {
		return uncomputable("git rev-parse", fmt.Errorf("want 8 lines, got %d", len(lines)))
	}
	gitDir, commonDir, objectsDir, shallowPath, graftsPath := lines[0], lines[1], lines[2], lines[3], lines[4]
	r.field("git rev-parse", revParse)

	refs, err := legacyGitOutput(ctx, root, "for-each-ref", "--format=%(refname)%00%(objectname)%00%(symref)")
	if err != nil {
		return err
	}
	r.field("git refs", refs)

	config, err := legacyGitOutput(ctx, root, "config", "--list", "-z")
	if err != nil {
		return err
	}
	r.field("git config", config)
	if err := refsAreStampable(config, gitDir, commonDir); err != nil {
		return err
	}

	tracked, err := gitx.LsFiles(ctx, root)
	if err != nil {
		return uncomputable("git ls-files", err)
	}
	r.field("git ls-files", []byte(strings.Join(tracked, "\x00")))

	if err := r.optionalFile("git shallow", shallowPath); err != nil {
		return err
	}
	if err := r.optionalFile("git grafts", graftsPath); err != nil {
		return err
	}
	if err := r.objects(objectsDir); err != nil {
		return err
	}

	for _, p := range []string{
		filepath.Join(gitDir, "HEAD"),
		filepath.Join(gitDir, "index"),
		filepath.Join(gitDir, "config.worktree"),
		filepath.Join(commonDir, "packed-refs"),
		filepath.Join(commonDir, "config"),
	} {
		if err := r.stamp(p, false); err != nil {
			return err
		}
	}
	refDirs := []string{filepath.Join(commonDir, "refs")}
	if gitDir != commonDir {
		refDirs = append(refDirs, filepath.Join(gitDir, "refs"))
	}
	for _, dir := range refDirs {
		if err := r.stampTree(dir); err != nil {
			return err
		}
	}
	return nil
}
