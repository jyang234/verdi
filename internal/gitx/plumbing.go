package gitx

// Low-level plumbing (blob/tree/commit-tree/update-ref) for a caller that
// must build a new commit — and a new branch pointing at it — WITHOUT
// ever touching the calling process's real index or working tree, and
// without moving HEAD or checking anything out (spec/scoping-canvas ac-6:
// the workbench's stub-instantiate board action scaffolds a new story
// spec on a fresh design/<slug> branch while the serving checkout stays
// exactly where it was; the operator checks the new branch out
// themselves). BuildTreeWithFile does its work against a throwaway
// GIT_INDEX_FILE, never the repository's own .git/index.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// runStdin execs `git <args...>` in dir with stdin piped from stdin and
// optional extra environment variables (appended to the ambient
// environment, e.g. GIT_INDEX_FILE), returning stdout. Only the plumbing
// operations in this file need stdin/env beyond what run (exec.go)
// already covers.
func runStdin(ctx context.Context, dir string, env []string, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gitx: git %s (dir %s): %w: %s", strings.Join(args, " "), dir, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// WriteBlob writes content as a loose git blob object — `git hash-object
// -w --stdin` — WITHOUT creating any file in the working tree, and
// returns its blob SHA (the same id HashObject would compute for
// identical bytes on disk).
func WriteBlob(ctx context.Context, dir string, content []byte) (string, error) {
	out, err := runStdin(ctx, dir, nil, content, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", fmt.Errorf("gitx: WriteBlob: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// BuildTreeWithFile builds a NEW tree object from baseTree (a tree-ish,
// e.g. "<commit>^{tree}") with one file added or replaced at path (mode
// 100644, blobSHA — normally from WriteBlob), and returns the new tree's
// SHA. The work happens against a scratch index file (a fresh temp file,
// removed when this call returns) so the repository's REAL index and
// working tree are never read from or written to.
func BuildTreeWithFile(ctx context.Context, dir, baseTree, path, blobSHA string) (string, error) {
	idx, err := os.CreateTemp("", "verdi-scratch-index-*")
	if err != nil {
		return "", fmt.Errorf("gitx: BuildTreeWithFile: creating scratch index: %w", err)
	}
	idxPath := idx.Name()
	if cerr := idx.Close(); cerr != nil {
		_ = os.Remove(idxPath)
		return "", fmt.Errorf("gitx: BuildTreeWithFile: closing scratch index: %w", cerr)
	}
	defer func() { _ = os.Remove(idxPath) }()
	env := []string{"GIT_INDEX_FILE=" + idxPath}

	if _, err := runStdin(ctx, dir, env, nil, "read-tree", baseTree); err != nil {
		return "", fmt.Errorf("gitx: BuildTreeWithFile: read-tree %s: %w", baseTree, err)
	}
	if _, err := runStdin(ctx, dir, env, nil, "update-index", "--add", "--cacheinfo", "100644,"+blobSHA+","+path); err != nil {
		return "", fmt.Errorf("gitx: BuildTreeWithFile: update-index %s: %w", path, err)
	}
	out, err := runStdin(ctx, dir, env, nil, "write-tree")
	if err != nil {
		return "", fmt.Errorf("gitx: BuildTreeWithFile: write-tree: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CommitTree creates a commit object with the given tree and single
// parent — `git commit-tree <tree> -p <parent> -m <message>` — and
// returns its SHA. No ref is moved or created; the caller decides whether
// (and how) the new commit becomes reachable (UpdateRef, below).
func CommitTree(ctx context.Context, dir, tree, parent, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("gitx: CommitTree: message must not be empty")
	}
	out, err := runStdin(ctx, dir, nil, nil, "commit-tree", tree, "-p", parent, "-m", message)
	if err != nil {
		return "", fmt.Errorf("gitx: CommitTree: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// zeroOID is git's "no object" sentinel (40 zeros) — passed as
// update-ref's expected old value to make ref creation atomic and
// create-only (git refuses if the ref already points anywhere).
const zeroOID = "0000000000000000000000000000000000000000"

// UpdateRef creates ref (e.g. "refs/heads/design/foo") pointing at
// commit, WITHOUT touching HEAD, the index, or the working tree — `git
// update-ref <ref> <commit> <zero-oid>`, whose three-argument form is
// atomically create-only: it fails if ref already exists, rather than
// silently moving it (stub-instantiate: "fail closed if the branch
// exists").
func UpdateRef(ctx context.Context, dir, ref, commit string) error {
	if _, err := runStdin(ctx, dir, nil, nil, "update-ref", ref, commit, zeroOID); err != nil {
		return fmt.Errorf("gitx: UpdateRef(%s): ref may already exist: %w", ref, err)
	}
	return nil
}
