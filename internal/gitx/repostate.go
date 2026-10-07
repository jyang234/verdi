package gitx

import (
	"context"
	"fmt"
	"strings"
)

// The whole-repository readers: what a caller that keys a repository's
// state byte for byte reads (the disclosure cache key, ledger SI-295).
// Each runs one read-only git command through run, so the context's
// observer sees it like every other gitx read (spec/gitx-recorder-seam
// dc-3), and returns git's exact stdout, which is what such a caller keys.

// Version returns `git version`'s exact output, newline included. It needs
// no repository; dir is only where git runs.
func Version(ctx context.Context, dir string) ([]byte, error) {
	out, err := run(ctx, dir, "version")
	if err != nil {
		return nil, fmt.Errorf("gitx: Version(%s): %w", dir, err)
	}
	return out, nil
}

// RefList returns every ref of dir's repository, sorted by name as git
// sorts them, exactly as `git for-each-ref
// --format=%(refname)%00%(objectname)%00%(symref)` prints them: one line
// per ref, holding its full name, the object it names and, for a symbolic
// ref, its target (empty otherwise), separated by NUL bytes.
func RefList(ctx context.Context, dir string) ([]byte, error) {
	out, err := run(ctx, dir, "for-each-ref", "--format=%(refname)%00%(objectname)%00%(symref)")
	if err != nil {
		return nil, fmt.Errorf("gitx: RefList(%s): %w", dir, err)
	}
	return out, nil
}

// ConfigList returns every configuration entry git applies in dir, from
// every scope with includes resolved, exactly as `git config --list -z`
// prints them: each entry is its key, a newline and its value, ended by a
// NUL byte.
func ConfigList(ctx context.Context, dir string) ([]byte, error) {
	out, err := run(ctx, dir, "config", "--list", "-z")
	if err != nil {
		return nil, fmt.Errorf("gitx: ConfigList(%s): %w", dir, err)
	}
	return out, nil
}

// Layout is where a repository keeps the files its reads depend on, from
// one `git rev-parse` call. Every path is absolute.
type Layout struct {
	// GitDir is the directory's own git directory: a linked worktree's is
	// its administrative directory under CommonDir's worktrees/.
	GitDir string
	// CommonDir is the git directory every worktree of the repository
	// shares: refs, objects and the shared configuration live here.
	CommonDir string
	// ObjectsDir is the object store (`--git-path objects`).
	ObjectsDir string
	// ShallowFile is where a shallow repository records its boundary
	// (`--git-path shallow`); it need not exist.
	ShallowFile string
	// GraftsFile is the grafts file (`--git-path info/grafts`); it need not
	// exist.
	GraftsFile string
	// Output is git's exact stdout: the five paths above, then the shallow
	// flag (`--is-shallow-repository`), HEAD's commit and HEAD's symbolic
	// full name ("HEAD" when detached), one per line. A caller that keys
	// the reading keys Output.
	Output []byte
}

// RepositoryLayout reads dir's Layout with `git rev-parse
// --path-format=absolute --git-dir --git-common-dir --git-path objects
// --git-path shallow --git-path info/grafts --is-shallow-repository HEAD
// --symbolic-full-name HEAD`. It fails outside a repository and on an
// unborn HEAD, which names no commit. git prints one line per answer, so a
// path holding a newline cannot be carried unambiguously; that case is an
// error, never a guess.
func RepositoryLayout(ctx context.Context, dir string) (Layout, error) {
	out, err := run(ctx, dir, "rev-parse", "--path-format=absolute",
		"--git-dir", "--git-common-dir",
		"--git-path", "objects", "--git-path", "shallow", "--git-path", "info/grafts",
		"--is-shallow-repository", "HEAD", "--symbolic-full-name", "HEAD")
	if err != nil {
		return Layout{}, fmt.Errorf("gitx: RepositoryLayout(%s): %w", dir, err)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != 8 {
		return Layout{}, fmt.Errorf("gitx: RepositoryLayout(%s): git rev-parse printed %d lines, want 8 (a path holding a newline cannot be read unambiguously)", dir, len(lines))
	}
	return Layout{
		GitDir:      lines[0],
		CommonDir:   lines[1],
		ObjectsDir:  lines[2],
		ShallowFile: lines[3],
		GraftsFile:  lines[4],
		Output:      out,
	}, nil
}
