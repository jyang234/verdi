package disclosureview

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jyang234/verdi/internal/disclosure"
)

// refFormatStore builds the cache fixture's store in a repository whose
// refs use format ("files" or "reftable"), with a second branch "other" at
// HEAD. A git that cannot create a reftable repository skips the test with
// an explicit disclosure: a skip is not a pass.
func refFormatStore(t *testing.T, format string) string {
	t.Helper()
	for _, v := range ciEnvVars {
		t.Setenv(v, "")
	}
	root := t.TempDir()
	args := []string{"init", "-q", "-b", "main"}
	if format == "reftable" {
		args = append(args, "--ref-format=reftable")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		if format == "reftable" {
			version, _ := exec.Command("git", "version").Output()
			t.Skipf("DISCLOSURE: %s cannot create a reftable repository (git init --ref-format=reftable: %v: %s). Reftable detection is NOT verified on this machine; a skip is not a pass. CI's git (2.45 or later) runs this case.", strings.TrimSpace(string(version)), err, strings.TrimSpace(string(out)))
		}
		t.Fatalf("git init: %v\n%s", err, out)
	}
	writeFile(t, filepath.Join(root, ".verdi", "verdi.yaml"), manifestYAML)
	writeFile(t, filepath.Join(root, ".verdi", ".gitignore"), "data/\n")
	writeFile(t, filepath.Join(root, ".verdi", "specs", "active", "panel-fixture", "spec.md"), storySpecMD)
	writeFile(t, filepath.Join(root, ".gitattributes"), ".verdi/specs/*/*/board.json          gitlab-generated\n.verdi/specs/*/*/rollup.json         gitlab-generated\n.verdi/specs/*/*/deviation-report.md gitlab-generated\n")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "seed")
	first := gitIn(t, root, "rev-parse", "HEAD")
	writeFile(t, filepath.Join(root, "README.md"), "store\n")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "second")
	gitIn(t, root, "update-ref", "refs/remotes/origin/main", first)
	gitIn(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	gitIn(t, root, "branch", "other")
	return root
}

// TestReadInputs_ReftableIsUncomputable (SI-295; B3-RR3): under git's
// reftable ref backend HEAD and every ref live in $GIT_DIR/reftable/,
// which the store guard does not stamp, so a ref changed and changed back
// during an enumeration would go unseen. Such a repository is an
// uncomputable key: detected by extensions.refStorage or by the reftable
// directory, it is never cached, and a HEAD toggled during an enumeration
// never yields a stale result.
func TestReadInputs_ReftableIsUncomputable(t *testing.T) {
	tests := []struct {
		name  string
		store func(t *testing.T) string
		// wantUncomputable is true for a reftable repository.
		wantUncomputable bool
	}{
		{"a files repository", func(t *testing.T) string { return refFormatStore(t, "files") }, false},
		{"a reftable repository", func(t *testing.T) string { return refFormatStore(t, "reftable") }, true},
		{"a files repository with a reftable directory", func(t *testing.T) string {
			root := refFormatStore(t, "files")
			if err := os.Mkdir(filepath.Join(root, ".git", "reftable"), 0o755); err != nil {
				t.Fatal(err)
			}
			return root
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.store(t)
			pastRacyWindow(t)
			_, err := readInputs(context.Background(), root)
			if got := errors.Is(err, errUncomputable); got != tt.wantUncomputable {
				t.Fatalf("readInputs error = %v, want uncomputable %v", err, tt.wantUncomputable)
			}

			// The re-review's probe: HEAD moved to "other" and back while
			// the first enumeration runs. The second call must not serve
			// that run.
			orig := enumerateLint
			var n atomic.Int64
			enumerateLint = func(ctx context.Context, r string) ([]disclosure.Disclosure, error) {
				if n.Add(1) == 1 {
					gitIn(t, root, "symbolic-ref", "HEAD", "refs/heads/other")
					defer gitIn(t, root, "symbolic-ref", "HEAD", "refs/heads/main")
				}
				return orig(ctx, r)
			}
			t.Cleanup(func() { enumerateLint = orig })

			var c Cache
			if _, err := c.Current(context.Background(), root); err != nil {
				t.Fatal(err)
			}
			want, wantErr := fresh(t, root)
			got, err := c.Current(context.Background(), root)
			sameResult(t, "the call after a toggled enumeration", got, err, want, wantErr)
			if got := n.Load(); got != 2 {
				t.Fatalf("enumerations = %d, want 2: the toggled run must not be stored", got)
			}
			if tt.wantUncomputable {
				if _, err := c.Current(context.Background(), root); err != nil || n.Load() != 3 {
					t.Fatalf("a third call enumerated %d times in all (err %v), want 3: a reftable repository is never cached", n.Load(), err)
				}
			}
		})
	}
}

// TestRefsAreStampable pins each reftable signal on its own: the
// configuration's extensions.refStorage (any value but files, any key
// case) and a reftable directory in the repository's own or common git
// directory.
func TestRefsAreStampable(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		dirs    []string // "git" or "common": where a reftable directory exists
		wantErr bool
	}{
		{"no ref-storage setting, no reftable directory", "core.bare\nfalse\x00user.name\nF\x00", nil, false},
		{"refStorage files", "extensions.refstorage\nfiles\x00", nil, false},
		{"refStorage reftable", "core.bare\nfalse\x00extensions.refstorage\nreftable\x00", nil, true},
		{"refStorage reftable, key in mixed case", "Extensions.RefStorage\nreftable\x00", nil, true},
		{"refStorage an unknown backend", "extensions.refstorage\nsomething-new\x00", nil, true},
		{"a value mentioning reftable under another key", "user.name\nreftable\x00", nil, false},
		{"a reftable directory in the git directory", "", []string{"git"}, true},
		{"a reftable directory in the common directory", "", []string{"common"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitDir, commonDir := t.TempDir(), t.TempDir()
			for _, d := range tt.dirs {
				dir := gitDir
				if d == "common" {
					dir = commonDir
				}
				if err := os.Mkdir(filepath.Join(dir, "reftable"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			err := refsAreStampable([]byte(tt.config), gitDir, commonDir)
			if (err != nil) != tt.wantErr || (err != nil && !errors.Is(err, errUncomputable)) {
				t.Fatalf("refsAreStampable = %v, want uncomputable %v", err, tt.wantErr)
			}
		})
	}
}
