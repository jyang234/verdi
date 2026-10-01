package ritualwitness

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Capture takes a Snapshot of the repository whose store root is dir, and
// of bareRemote, its origin. Every git invocation here is read-only, runs
// with no gitx.Observer (so sensing never enters a ritual's command log),
// and passes --no-optional-locks so even `git status` never rewrites the
// index it is reading.
func Capture(ctx context.Context, dir, bareRemote string) (Snapshot, error) {
	s := Snapshot{StoreRoot: canonicalPath("", dir)}
	top, err := sensorLine(ctx, s.StoreRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return Snapshot{}, fmt.Errorf("ritualwitness: Capture: repository root: %w", err)
	}
	s.Root = canonicalPath("", top)
	prefix, err := sensorGit(ctx, s.StoreRoot, nil, "rev-parse", "--show-prefix")
	if err != nil {
		return Snapshot{}, fmt.Errorf("ritualwitness: Capture: store prefix: %w", err)
	}
	s.Prefix = strings.TrimSuffix(string(prefix), "\n")
	common, err := sensorLine(ctx, s.Root, "rev-parse", "--git-common-dir")
	if err != nil {
		return Snapshot{}, fmt.Errorf("ritualwitness: Capture: common directory: %w", err)
	}
	s.CommonDir = canonicalPath(s.Root, common)

	steps := []struct {
		what string
		run  func() error
	}{
		{"refs", func() (e error) { s.Refs, e = captureRefs(ctx, s.Root); return }},
		{"remote refs", func() (e error) { s.RemoteRefs, e = captureRefs(ctx, bareRemote); return }},
		{"HEAD", func() (e error) { s.Head, e = captureHead(ctx, s.Root); return }},
		{"HEAD tree", func() (e error) { s.HeadTree, e = captureHeadTree(ctx, s.Root); return }},
		{"index", func() (e error) { s.Index, e = captureIndex(ctx, s.Root); return }},
		{"status", func() (e error) { s.Status, e = captureStatus(ctx, s.Root); return }},
		{"working tree", func() (e error) { s.Files, e = captureFiles(ctx, s.Root); return }},
		{"linked worktrees", func() (e error) { s.Worktrees, e = captureWorktrees(ctx, s.CommonDir, s.Refs); return }},
		{"config", func() (e error) { s.Config, e = captureConfig(ctx, s.Root); return }},
		{"hooks and info", func() (e error) { s.GitFiles, e = captureGitFiles(s.CommonDir); return }},
		{"commits", func() (e error) { s.Commits, e = captureCommits(ctx, s.Root, bareRemote); return }},
	}
	for _, step := range steps {
		if err := step.run(); err != nil {
			return Snapshot{}, fmt.Errorf("ritualwitness: Capture: %s: %w", step.what, err)
		}
	}
	return s, nil
}

// sensorGit runs `git <args...>` in dir for sensing only — never through
// gitx, so a ritual's command log holds only what the ritual issued.
func sensorGit(ctx context.Context, dir string, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-optional-locks", "--no-replace-objects"}, args...)...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("git %s (dir %s): %w: %s", strings.Join(args, " "), dir, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// sensorLine is sensorGit for a one-line answer, without its newline.
func sensorLine(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := sensorGit(ctx, dir, nil, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

func captureRefs(ctx context.Context, dir string) (map[string]Ref, error) {
	out, err := sensorGit(ctx, dir, nil, "for-each-ref", "--format=%(objectname)%00%(refname)%00%(symref)")
	if err != nil {
		return nil, err
	}
	return parseRefs(out)
}

// captureHead reads HEAD with `symbolic-ref -q HEAD` (a full refname, so a
// tag sharing a branch's short name is never confused with it), where only
// exit 1 means detached, and its commit with rev-parse.
func captureHead(ctx context.Context, dir string) (Head, error) {
	commit, err := sensorLine(ctx, dir, "rev-parse", "--verify", "-q", "HEAD^{commit}")
	if err != nil {
		return Head{}, err
	}
	if !isObjectID(commit) {
		return Head{}, fmt.Errorf("rev-parse HEAD: malformed commit %q", commit)
	}
	out, runErr := sensorGit(ctx, dir, nil, "symbolic-ref", "-q", "HEAD")
	ref, detached, err := headFromSymbolicRef(out, runErr)
	if err != nil {
		return Head{}, err
	}
	return Head{Ref: ref, Detached: detached, Commit: commit}, nil
}

func captureHeadTree(ctx context.Context, dir string) (map[string]TreeEntry, error) {
	out, err := sensorGit(ctx, dir, nil, "ls-tree", "-r", "-z", "--full-tree", "HEAD")
	if err != nil {
		return nil, err
	}
	return parseTree(out)
}

func captureIndex(ctx context.Context, dir string) ([]IndexEntry, error) {
	out, err := sensorGit(ctx, dir, nil, "ls-files", "-s", "-z")
	if err != nil {
		return nil, err
	}
	return parseIndex(out)
}

func captureStatus(ctx context.Context, dir string) ([]StatusEntry, error) {
	out, err := sensorGit(ctx, dir, nil, "-c", "status.relativePaths=false", "status", "--porcelain", "-z", "--untracked-files=all", "--no-renames")
	if err != nil {
		return nil, err
	}
	return parseStatus(out)
}

// captureFiles reads every non-ignored file of the worktree at root,
// tracked or untracked. A directory entry (a nested repository or a
// linked worktree inside the tree, which git lists as "dir/") is skipped:
// its contents are sensed by the worktree sensors instead (doc.go).
func captureFiles(ctx context.Context, root string) (map[string]FileState, error) {
	out, err := sensorGit(ctx, root, nil, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	paths, err := parsePaths(out)
	if err != nil {
		return nil, err
	}
	files := map[string]FileState{}
	for _, p := range paths {
		if strings.HasSuffix(p, "/") {
			continue
		}
		st, ok, err := fileState(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			return nil, err
		}
		if ok {
			files[p] = st
		}
	}
	return files, nil
}

// fileState reads one path with Lstat: a regular file or symlink's state
// and true; false when the path does not exist or is a directory or a
// special file. Only a not-exist error means absent: any other Lstat or
// read failure is an error, never read as a deletion.
func fileState(full string) (FileState, bool, error) {
	info, err := os.Lstat(full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return FileState{}, false, nil
		}
		return FileState{}, false, fmt.Errorf("lstat %s: %w", full, err)
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(full)
		if err != nil {
			return FileState{}, false, fmt.Errorf("readlink %s: %w", full, err)
		}
		return FileState{Kind: KindSymlink, Digest: digest([]byte(target))}, true, nil
	case info.Mode().IsRegular():
		content, err := os.ReadFile(full)
		if err != nil {
			return FileState{}, false, fmt.Errorf("read %s: %w", full, err)
		}
		return FileState{Kind: KindFile, Exec: info.Mode()&0o111 != 0, Digest: digest(content)}, true, nil
	default:
		return FileState{}, false, nil
	}
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// captureWorktrees reads every linked worktree from the common directory's
// administrative entries (ids, gitdir, HEAD, locks), and each present
// worktree's index. Reading the entries directly, rather than `git
// worktree list`, also senses an entry whose worktree directory is gone.
func captureWorktrees(ctx context.Context, commonDir string, refs map[string]Ref) ([]Worktree, error) {
	admin := filepath.Join(commonDir, "worktrees")
	entries, err := os.ReadDir(admin)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var wts []Worktree
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		wt, err := readAdminEntry(ctx, filepath.Join(admin, e.Name()), refs)
		if err != nil {
			return nil, err
		}
		wts = append(wts, wt)
	}
	sort.Slice(wts, func(i, j int) bool {
		if wts[i].Path != wts[j].Path {
			return wts[i].Path < wts[j].Path
		}
		return wts[i].ID < wts[j].ID
	})
	return wts, nil
}

func readAdminEntry(ctx context.Context, entryDir string, refs map[string]Ref) (Worktree, error) {
	wt := Worktree{ID: filepath.Base(entryDir)}
	gitdir, err := os.ReadFile(filepath.Join(entryDir, "gitdir"))
	if err != nil {
		return Worktree{}, fmt.Errorf("worktree %s: gitdir: %w", wt.ID, err)
	}
	link := strings.TrimSuffix(string(gitdir), "\n")
	if link == "" {
		return Worktree{}, fmt.Errorf("worktree %s: empty gitdir", wt.ID)
	}
	wt.Path = filepath.Dir(canonicalPath(entryDir, link))
	head, err := os.ReadFile(filepath.Join(entryDir, "HEAD"))
	if err != nil {
		return Worktree{}, fmt.Errorf("worktree %s: HEAD: %w", wt.ID, err)
	}
	ref, oid, err := parseHeadFile(head)
	if err != nil {
		return Worktree{}, fmt.Errorf("worktree %s: %w", wt.ID, err)
	}
	wt.Head = Head{Ref: ref, Detached: ref == "", Commit: oid}
	if ref != "" {
		wt.Head.Commit = refs[ref].Object
	}
	lock, err := os.ReadFile(filepath.Join(entryDir, "locked"))
	switch {
	case err == nil:
		wt.Locked, wt.LockReason = true, string(lock)
	case !errors.Is(err, fs.ErrNotExist):
		return Worktree{}, fmt.Errorf("worktree %s: lock: %w", wt.ID, err)
	}
	info, err := os.Lstat(wt.Path)
	switch {
	case err == nil && info.IsDir():
		wt.Present = true
		if wt.Index, err = captureIndex(ctx, wt.Path); err != nil {
			return Worktree{}, fmt.Errorf("worktree %s: index: %w", wt.Path, err)
		}
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return Worktree{}, fmt.Errorf("worktree %s: %w", wt.Path, err)
	}
	return wt, nil
}

func captureConfig(ctx context.Context, dir string) (map[string][]string, error) {
	out, err := sensorGit(ctx, dir, nil, "config", "--local", "--list", "-z")
	if err != nil {
		return nil, err
	}
	return parseConfig(out)
}

// captureGitFiles reads every file under the common directory's hooks/
// and info/.
func captureGitFiles(commonDir string) (map[string]FileState, error) {
	files := map[string]FileState{}
	for _, sub := range []string{"hooks", "info"} {
		root := filepath.Join(commonDir, sub)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if errors.Is(walkErr, fs.ErrNotExist) && path == root {
					return filepath.SkipDir
				}
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			st, ok, err := fileState(path)
			if err != nil || !ok {
				return err
			}
			rel, err := filepath.Rel(commonDir, path)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(rel)] = st
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// captureCommits reads every commit object in the repository at root and
// in bareRemote — reachable or not (`cat-file --batch-all-objects`) — with
// its parents and its file list against its first parent.
func captureCommits(ctx context.Context, root, bareRemote string) (map[string]CommitObject, error) {
	commits := map[string]CommitObject{}
	for _, repo := range []string{root, bareRemote} {
		out, err := sensorGit(ctx, repo, nil, "cat-file", "--batch-all-objects", "--batch-check=%(objectname) %(objecttype)")
		if err != nil {
			return nil, err
		}
		ids, err := parseCommitObjects(out)
		if err != nil {
			return nil, err
		}
		var fresh []string
		for _, id := range ids {
			if _, seen := commits[id]; !seen {
				fresh = append(fresh, id)
			}
		}
		if len(fresh) == 0 {
			continue
		}
		out, err = sensorGit(ctx, repo, []byte(strings.Join(fresh, "\n")+"\n"), "rev-list", "--no-walk=unsorted", "--parents", "--stdin")
		if err != nil {
			return nil, err
		}
		parents, err := parseParents(out)
		if err != nil {
			return nil, err
		}
		for _, id := range fresh {
			ps, ok := parents[id]
			if !ok {
				return nil, fmt.Errorf("rev-list --parents omitted commit %s", id)
			}
			files, err := commitFiles(ctx, repo, id, ps)
			if err != nil {
				return nil, err
			}
			commits[id] = CommitObject{Parents: ps, Files: files}
		}
	}
	return commits, nil
}

// commitFiles is a commit's file list against its first parent, or against
// the empty tree (--root) for a root commit, read with -z so every path is
// byte-exact whatever core.quotePath says.
func commitFiles(ctx context.Context, repo, id string, parents []string) ([]string, error) {
	args := []string{"diff-tree", "-r", "-z", "--no-renames", "--name-only"}
	if len(parents) == 0 {
		args = append(args, "--no-commit-id", "--root", id)
	} else {
		args = append(args, parents[0], id)
	}
	out, err := sensorGit(ctx, repo, nil, args...)
	if err != nil {
		return nil, err
	}
	files, err := parsePaths(out)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
