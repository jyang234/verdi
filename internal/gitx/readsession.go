package gitx

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// fullOIDPattern matches a full lowercase object id, SHA-1 or SHA-256.
var fullOIDPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// readSessionKey is the context key a read session rides under.
type readSessionKey struct{}

// readSession is one request's reads of one repository (WithReadSession).
// It holds no context: each process it starts runs under the context of
// the call that started it, so it is killed with that call's request.
type readSession struct {
	dir string

	closeOnce sync.Once
	released  atomic.Bool

	batchMu sync.Mutex
	batch   *catFileBatch
	broken  bool

	prefixMu    sync.Mutex
	prefixKnown bool
	topLevel    bool

	memoMu sync.Mutex
	memo   map[string]memoResult

	reachMu sync.Mutex
	reach   map[string]reachWalk
}

// memoResult is one memoized ref read: its output and error, replayed.
type memoResult struct {
	out []byte
	err error
}

// reachWalk is the set of commits one walk found reachable from a head;
// ok is false when the walk failed and every query takes the exact path.
type reachWalk struct {
	commits map[string]bool
	ok      bool
}

// WithReadSession scopes every gitx read of dir made through the returned
// context to one request (ledger SI-352, lane P1; Wave 6 §5.3). Within it:
//
//   - Show's and BlobAt's reads are answered from one long-lived
//     `git cat-file --batch` process instead of a process per read;
//   - each ref resolution (a `symbolic-ref --short -q`, `show-ref --verify
//     --quiet` or `rev-parse --verify` read) runs once, and its answer is
//     replayed for the rest of the request — the accepted ref is resolved
//     once per load, not once per consumer;
//   - ReachableFromHEAD answers a full commit id that one `git rev-list`
//     walk of head found reachable, instead of running a rev-parse and a
//     merge-base per commit (BL-157).
//
// Every shortcut answers only its happy path and is byte-for-byte what
// the per-read exec answers: a blob's bytes, a plain file's blob id, a
// commit proven reachable. Anything else — an object that is missing, not
// a blob, not a plain file, an unusual path, a commit the walk did not
// reach, a failed batch process or walk — takes the original exec path, so
// every error and every negative is exactly what it was.
//
// The session caches nothing across requests (spec/readiness-recovery-v2
// co-2): it dies with the returned release function, which the caller
// defers. A context that already carries a session for dir is returned
// unchanged with a no-op release, so a caller that composes several
// loads into one projection can open the session once around all of
// them.
func WithReadSession(ctx context.Context, dir string) (context.Context, func()) {
	if sessionFor(ctx, dir) != nil {
		return ctx, func() {}
	}
	s := &readSession{dir: filepath.Clean(dir), memo: map[string]memoResult{}, reach: map[string]reachWalk{}}
	return context.WithValue(ctx, readSessionKey{}, s), s.release
}

// sessionFor returns ctx's read session when it reads dir and has not
// been released, else nil.
func sessionFor(ctx context.Context, dir string) *readSession {
	s, ok := ctx.Value(readSessionKey{}).(*readSession)
	if !ok || s == nil || s.released.Load() || s.dir != filepath.Clean(dir) {
		return nil
	}
	return s
}

// release ends the session, once: its batch process stops, and every read
// after it — through any context that still carries it — takes the exec
// path, so nothing the session read is answered from after its request.
func (s *readSession) release() {
	s.closeOnce.Do(func() {
		s.released.Store(true)
		s.batchMu.Lock()
		defer s.batchMu.Unlock()
		if s.batch != nil {
			s.batch.stop()
			s.batch = nil
		}
		s.broken = true
	})
}

// memoizable reports whether args is one of the read-only ref resolutions
// a session runs once: exactly these argument shapes, so no write form of
// the same subcommand (`symbolic-ref HEAD <ref>`) is ever replayed.
func memoizable(args []string) bool {
	switch {
	case len(args) == 4 && args[0] == "symbolic-ref" && args[1] == "--short" && args[2] == "-q":
		return true
	case len(args) == 4 && args[0] == "show-ref" && args[1] == "--verify" && args[2] == "--quiet":
		return true
	case len(args) == 3 && args[0] == "rev-parse" && args[1] == "--verify":
		return true
	case len(args) == 4 && args[0] == "rev-parse" && args[1] == "--verify" && args[2] == "-q":
		return true
	default:
		return false
	}
}

// memoized runs a memoizable read once per session and replays its
// output and error afterwards. The key includes dir as the caller spelled
// it, because execGit's error names it: a replayed error is byte for byte
// the one that read returned.
func (s *readSession) memoized(ctx context.Context, dir string, args []string) ([]byte, error) {
	key := dir + "\x00" + strings.Join(args, "\x00")
	s.memoMu.Lock()
	defer s.memoMu.Unlock()
	if r, ok := s.memo[key]; ok {
		return bytes.Clone(r.out), r.err
	}
	out, err := execGit(ctx, dir, args...)
	s.memo[key] = memoResult{out: bytes.Clone(out), err: err}
	return out, err
}

// blob returns the bytes of the blob rev:path names, or ok == false when
// the batch cannot answer it as a blob.
func (s *readSession) blob(ctx context.Context, rev, filePath string) ([]byte, bool) {
	if strings.HasPrefix(filePath, "./") || strings.HasPrefix(filePath, "../") {
		return nil, false
	}
	obj, ok := s.object(ctx, rev+":"+filePath)
	if !ok || obj.typ != "blob" {
		return nil, false
	}
	return obj.data, true
}

// blobAt answers BlobAt's plain-file case from the parent tree: the blob
// id of a 100644 or 100755 entry, or found == false when the parent tree
// has no entry by that name. ok == false hands every other case — an
// unusual path, a dir that is not the work tree's top level (ls-tree's
// paths are relative to it), a parent that is not a tree, an entry that is
// not a plain file — to the exec path.
func (s *readSession) blobAt(ctx context.Context, ref, filePath string) (oid string, found, ok bool) {
	if !plainRepoPath(filePath) || !s.isTopLevel(ctx) {
		return "", false, false
	}
	dir, base := path.Split(filePath)
	tree, ok := s.object(ctx, ref+":"+strings.TrimSuffix(dir, "/"))
	if !ok || tree.typ != "tree" {
		return "", false, false
	}
	entries, err := parseTree(tree.data, len(tree.oid)/2)
	if err != nil {
		return "", false, false
	}
	entry, present := entries[base]
	if !present {
		return "", false, true
	}
	if entry.mode != "100644" && entry.mode != "100755" {
		return "", false, false
	}
	return entry.oid, true, true
}

// plainRepoPath accepts a repository-relative path with no pathspec
// magic, no empty, "." or ".." segment, and no leading or trailing slash.
func plainRepoPath(p string) bool {
	if p == "" || strings.ContainsAny(p, "*?[\\:\n") {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// isTopLevel reports, once per session, whether dir is its work tree's
// top level: ls-tree reads paths relative to its working directory, while
// <rev>:<path> names them from the top, so the two agree only there.
func (s *readSession) isTopLevel(ctx context.Context) bool {
	s.prefixMu.Lock()
	defer s.prefixMu.Unlock()
	if !s.prefixKnown {
		out, err := execGit(ctx, s.dir, "rev-parse", "--show-prefix") // RepoPrefix's read
		s.topLevel = err == nil && strings.TrimSpace(string(out)) == ""
		s.prefixKnown = true
	}
	return s.topLevel
}

// treeEntry is one entry of a raw tree object.
type treeEntry struct {
	mode string
	oid  string
}

// parseTree decodes a raw tree object: entries of "<mode> <name>\0"
// followed by a binary object id of oidLen bytes.
func parseTree(data []byte, oidLen int) (map[string]treeEntry, error) {
	if oidLen != 20 && oidLen != 32 {
		return nil, fmt.Errorf("gitx: tree object id length %d", oidLen)
	}
	entries := map[string]treeEntry{}
	for len(data) > 0 {
		sp := bytes.IndexByte(data, ' ')
		nul := bytes.IndexByte(data, 0)
		if sp <= 0 || nul <= sp+1 || len(data) < nul+1+oidLen {
			return nil, errors.New("gitx: malformed tree object")
		}
		entries[string(data[sp+1:nul])] = treeEntry{
			mode: string(data[:sp]),
			oid:  fmt.Sprintf("%x", data[nul+1:nul+1+oidLen]),
		}
		data = data[nul+1+oidLen:]
	}
	return entries, nil
}

// batchObject is one object the batch process returned.
type batchObject struct {
	oid, typ string
	data     []byte
}

// object reads name through the session's batch process, starting it on
// first use. ok == false means the batch did not return an object: the
// name is missing or ambiguous, or the process failed — in which case the
// session stops batching and every later read takes the exec path.
func (s *readSession) object(ctx context.Context, name string) (batchObject, bool) {
	if strings.ContainsAny(name, "\n") {
		return batchObject{}, false
	}
	s.batchMu.Lock()
	defer s.batchMu.Unlock()
	if s.broken {
		return batchObject{}, false
	}
	if s.batch == nil {
		b, err := startCatFileBatch(ctx, s.dir)
		if err != nil {
			s.broken = true
			return batchObject{}, false
		}
		s.batch = b
	}
	obj, found, err := s.batch.read(name)
	if err != nil {
		s.batch.stop()
		s.batch = nil
		s.broken = true
		return batchObject{}, false
	}
	return obj, found
}

// catFileBatch is one running `git cat-file --batch`.
type catFileBatch struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

func startCatFileBatch(ctx context.Context, dir string) (*catFileBatch, error) {
	args := []string{"cat-file", "--batch"}
	observe(ctx, dir, args)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("gitx: cat-file --batch stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("gitx: cat-file --batch stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("gitx: starting cat-file --batch: %w", err)
	}
	return &catFileBatch{cmd: cmd, stdin: stdin, stdout: bufio.NewReader(stdout)}, nil
}

// read asks for name and returns its object; found == false for git's
// "missing" and "ambiguous" answers. Any other answer is a protocol error.
func (b *catFileBatch) read(name string) (batchObject, bool, error) {
	if _, err := io.WriteString(b.stdin, name+"\n"); err != nil {
		return batchObject{}, false, fmt.Errorf("gitx: cat-file --batch request: %w", err)
	}
	header, err := b.stdout.ReadString('\n')
	if err != nil {
		return batchObject{}, false, fmt.Errorf("gitx: cat-file --batch header: %w", err)
	}
	header = strings.TrimSuffix(header, "\n")
	if header == name+" missing" || header == name+" ambiguous" {
		return batchObject{}, false, nil
	}
	fields := strings.Split(header, " ")
	if len(fields) != 3 || !fullOIDPattern.MatchString(fields[0]) {
		return batchObject{}, false, fmt.Errorf("gitx: cat-file --batch header %q", header)
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil || size < 0 {
		return batchObject{}, false, fmt.Errorf("gitx: cat-file --batch size in %q", header)
	}
	data := make([]byte, size+1)
	if _, err := io.ReadFull(b.stdout, data); err != nil {
		return batchObject{}, false, fmt.Errorf("gitx: cat-file --batch content: %w", err)
	}
	if data[size] != '\n' {
		return batchObject{}, false, errors.New("gitx: cat-file --batch content is not newline-terminated")
	}
	return batchObject{oid: fields[0], typ: fields[1], data: data[:size]}, true, nil
}

// stop ends the process and reaps it. Closing its input is enough for a
// clean exit, but after a protocol error the process may still be writing
// content nobody will read, so it is killed rather than waited on.
func (b *catFileBatch) stop() {
	_ = b.stdin.Close()
	_ = b.cmd.Process.Kill()
	_ = b.cmd.Wait()
}

// reachable reports whether commit is in the set one `git rev-list head`
// walk found. The walk runs once per head per session — every commit
// reachable from head, the same parent links `git merge-base
// --is-ancestor` follows, grafts, replacements and a shallow boundary
// included — so a commit in the set is proven reachable exactly as
// ReachableFromHEAD's own per-commit path would prove it. A commit not in
// the set, and every query after a failed walk, is left to that path.
func (s *readSession) reachable(ctx context.Context, dir, commit, head string) bool {
	s.reachMu.Lock()
	defer s.reachMu.Unlock()
	w, walked := s.reach[head]
	if !walked {
		out, err := execGit(ctx, dir, "rev-list", head, "--")
		w = reachWalk{ok: err == nil}
		if w.ok {
			w.commits = map[string]bool{}
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if line != "" {
					w.commits[line] = true
				}
			}
		}
		s.reach[head] = w
	}
	return w.ok && w.commits[commit]
}
