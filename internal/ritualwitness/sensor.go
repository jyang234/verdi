package ritualwitness

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/gitx"
)

// Head is dir's HEAD, read at sensor time: either a symbolic ref to a local
// branch, or detached at a commit.
type Head struct {
	// Branch is the checked-out branch's short name, "" when detached.
	Branch   string
	Detached bool
	// Commit is HEAD's resolved commit sha, always populated.
	Commit string
}

// IndexEntry is one `git ls-files -s -z` line: a path's stage-0 (or
// higher, for an unresolved conflict) index entry.
type IndexEntry struct {
	Mode  string
	SHA   string
	Stage int
	Path  string
}

// WorkingEntry is one `git status --porcelain -z --untracked-files=all`
// entry: Index compares the index with HEAD, Worktree the working tree
// with the index ('.' or ' ' means unchanged on that side; '?' marks an
// untracked path on both columns). OrigPath is a detected rename or copy's
// source path, "" otherwise.
type WorkingEntry struct {
	Index, Worktree byte
	Path, OrigPath  string
}

// Snapshot is everything the obligation's sensors cover, taken at one
// instant: local refs, the remote's own refs (queried directly against the
// bare remote, never through the local clone's remote-tracking refs, which
// a push may or may not have refreshed), HEAD, the index, the working
// tree plus content hashes of every changed or untracked path, the
// linked-worktree list, and the full set of commits reachable from any ref
// — local or remote — together with each such commit's own recorded file
// list (parent dc-2: "the file list of every commit the ritual creates
// proves what it recorded").
type Snapshot struct {
	LocalRefs   map[string]string
	RemoteRefs  map[string]string
	Head        Head
	Index       []IndexEntry
	Working     []WorkingEntry
	Hashes      map[string]string
	Worktrees   []gitx.WorktreeEntry
	Commits     map[string]bool
	CommitFiles map[string][]string
}

// Capture takes a Snapshot of dir (a local checkout) and bareRemote (its
// origin). Every invocation here is read-only and runs with no
// gitx.Observer attached, independent of whatever Driver is under
// observation, so sensing a ritual's effects never itself appears in that
// ritual's own command log.
func Capture(ctx context.Context, dir, bareRemote string) (Snapshot, error) {
	localRefs, err := captureRefs(ctx, dir, "refs/heads")
	if err != nil {
		return Snapshot{}, fmt.Errorf("ritualwitness: Capture: local refs: %w", err)
	}
	remoteRefs, err := captureRefs(ctx, bareRemote, "refs/heads")
	if err != nil {
		return Snapshot{}, fmt.Errorf("ritualwitness: Capture: remote refs: %w", err)
	}
	head, err := captureHead(ctx, dir)
	if err != nil {
		return Snapshot{}, fmt.Errorf("ritualwitness: Capture: HEAD: %w", err)
	}
	index, err := captureIndex(ctx, dir)
	if err != nil {
		return Snapshot{}, fmt.Errorf("ritualwitness: Capture: index: %w", err)
	}
	working, err := captureWorking(ctx, dir)
	if err != nil {
		return Snapshot{}, fmt.Errorf("ritualwitness: Capture: working tree: %w", err)
	}
	hashes, err := captureHashes(ctx, dir, working)
	if err != nil {
		return Snapshot{}, fmt.Errorf("ritualwitness: Capture: content hashes: %w", err)
	}
	worktrees, err := gitx.WorktreeList(ctx, dir)
	if err != nil {
		return Snapshot{}, fmt.Errorf("ritualwitness: Capture: worktree list: %w", err)
	}
	commits, err := captureCommits(ctx, dir, bareRemote)
	if err != nil {
		return Snapshot{}, fmt.Errorf("ritualwitness: Capture: commits: %w", err)
	}
	commitFiles, err := captureCommitFiles(ctx, dir, bareRemote, commits)
	if err != nil {
		return Snapshot{}, fmt.Errorf("ritualwitness: Capture: commit file lists: %w", err)
	}

	return Snapshot{
		LocalRefs:   localRefs,
		RemoteRefs:  remoteRefs,
		Head:        head,
		Index:       index,
		Working:     working,
		Hashes:      hashes,
		Worktrees:   worktrees,
		Commits:     commits,
		CommitFiles: commitFiles,
	}, nil
}

// sensorGit runs `git <args...>` in dir for sensing only, with no
// gitx.Observer in play — never gitx's own run(), so a ritual's recorded
// command log reflects only what the ritual itself issued, never the
// harness's own before/after inspection of it.
func sensorGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s (dir %s): %w: %s", strings.Join(args, " "), dir, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// captureRefs reads every ref under prefix in dir as a full-refname-to-sha
// map (`git for-each-ref --format=%(objectname) %(refname) <prefix>`). An
// empty prefix (no matching refs, e.g. a fresh bare remote before any
// push) is not an error: it yields an empty, non-nil map.
func captureRefs(ctx context.Context, dir, prefix string) (map[string]string, error) {
	out, err := sensorGit(ctx, dir, "for-each-ref", "--format=%(objectname) %(refname)", prefix)
	if err != nil {
		return nil, err
	}
	refs := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		sha, ref, ok := strings.Cut(line, " ")
		if !ok {
			return nil, fmt.Errorf("malformed for-each-ref line %q", line)
		}
		refs[ref] = sha
	}
	return refs, nil
}

// captureHead reads dir's HEAD: a symbolic ref to a local branch
// (`git symbolic-ref --short -q HEAD`), or a detached commit
// (`git rev-parse HEAD`) when no symbolic ref resolves.
func captureHead(ctx context.Context, dir string) (Head, error) {
	commit, err := sensorGit(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return Head{}, err
	}
	sha := strings.TrimSpace(string(commit))

	branchOut, err := sensorGit(ctx, dir, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		// A non-zero exit here means detached HEAD, not an operational
		// failure: rev-parse above already proved dir is a real repository
		// with a resolvable HEAD.
		return Head{Detached: true, Commit: sha}, nil
	}
	return Head{Branch: strings.TrimSpace(string(branchOut)), Commit: sha}, nil
}

// captureIndex reads dir's index via `git ls-files -s -z`: "<mode> <sha>
// <stage>\t<path>" entries, NUL-terminated so every legal path byte
// survives.
func captureIndex(ctx context.Context, dir string) ([]IndexEntry, error) {
	out, err := sensorGit(ctx, dir, "ls-files", "-s", "-z")
	if err != nil {
		return nil, err
	}
	var entries []IndexEntry
	for _, field := range bytes.Split(out, []byte{0}) {
		if len(field) == 0 {
			continue
		}
		meta, path, ok := strings.Cut(string(field), "\t")
		if !ok {
			return nil, fmt.Errorf("malformed ls-files -s -z entry %q", field)
		}
		parts := strings.Fields(meta)
		if len(parts) != 3 {
			return nil, fmt.Errorf("malformed ls-files -s -z metadata %q", meta)
		}
		var stage int
		if _, err := fmt.Sscanf(parts[2], "%d", &stage); err != nil {
			return nil, fmt.Errorf("malformed ls-files -s -z stage %q: %w", parts[2], err)
		}
		entries = append(entries, IndexEntry{Mode: parts[0], SHA: parts[1], Stage: stage, Path: path})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

// captureWorking reads dir's working tree via `git status --porcelain -z
// --untracked-files=all`: one "XY <path>" entry per path, NUL-terminated,
// with a rename or copy's original path as the NUL-terminated field
// immediately after (git's -z form, "to\0from\0", never "from -> to").
func captureWorking(ctx context.Context, dir string) ([]WorkingEntry, error) {
	out, err := sensorGit(ctx, dir, "status", "--porcelain", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	fields := bytes.Split(out, []byte{0})
	var entries []WorkingEntry
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if len(entry) == 0 {
			continue
		}
		if len(entry) < 4 || entry[2] != ' ' {
			return nil, fmt.Errorf("malformed status --porcelain -z entry %q", entry)
		}
		index, worktree, path := entry[0], entry[1], string(entry[3:])
		orig := ""
		if index == 'R' || index == 'C' {
			i++
			if i >= len(fields) || len(fields[i]) == 0 {
				return nil, fmt.Errorf("status --porcelain -z entry %q declares a rename/copy with no original-path field", entry)
			}
			orig = string(fields[i])
		}
		entries = append(entries, WorkingEntry{Index: index, Worktree: worktree, Path: path, OrigPath: orig})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

// captureHashes computes the git blob content hash of every working-tree
// entry whose worktree column shows a change ('?' untracked, or any
// non-space/non-'.' worktree status) and that still exists on disk —
// proving a dirty or untracked file's content is byte-identical before and
// after a ritual runs, independent of whatever the index or a commit says
// about it.
func captureHashes(ctx context.Context, dir string, working []WorkingEntry) (map[string]string, error) {
	hashes := map[string]string{}
	for _, w := range working {
		if w.Worktree == ' ' || w.Worktree == '.' {
			continue
		}
		full := filepath.Join(dir, filepath.FromSlash(w.Path))
		if _, statErr := os.Stat(full); statErr != nil {
			continue // deleted on disk: no content to hash
		}
		sha, err := gitx.HashObject(ctx, dir, w.Path)
		if err != nil {
			return nil, fmt.Errorf("hashing %s: %w", w.Path, err)
		}
		hashes[w.Path] = sha
	}
	return hashes, nil
}

// captureCommits returns the full set of commits reachable from any ref —
// `git rev-list --all`, unioned over dir (every local ref, including any
// remote-tracking ref a push happened to refresh there) and bareRemote
// (the remote's own refs, read directly rather than trusting dir's
// remote-tracking refs to be current).
func captureCommits(ctx context.Context, dir, bareRemote string) (map[string]bool, error) {
	commits := map[string]bool{}
	for _, d := range []string{dir, bareRemote} {
		out, err := sensorGit(ctx, d, "rev-list", "--all")
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				commits[line] = true
			}
		}
	}
	return commits, nil
}

// captureCommitFiles returns, for every commit in commits, the file list
// `git diff-tree --no-commit-id --name-only -r --root <sha>` reports: the
// paths that commit's tree changed relative to its first parent (or, for a
// root commit, --root's empty-tree comparison). It tries dir first and
// falls back to bareRemote, since a commit reachable only from the
// remote's own refs (e.g. after a push whose local remote-tracking ref was
// never refreshed) may not resolve in dir.
func captureCommitFiles(ctx context.Context, dir, bareRemote string, commits map[string]bool) (map[string][]string, error) {
	files := map[string][]string{}
	for sha := range commits {
		out, err := sensorGit(ctx, dir, "diff-tree", "--no-commit-id", "--name-only", "-r", "--root", sha)
		if err != nil {
			out, err = sensorGit(ctx, bareRemote, "diff-tree", "--no-commit-id", "--name-only", "-r", "--root", sha)
			if err != nil {
				return nil, fmt.Errorf("diff-tree %s: %w", sha, err)
			}
		}
		var paths []string
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				paths = append(paths, line)
			}
		}
		sort.Strings(paths)
		files[sha] = paths
	}
	return files, nil
}
