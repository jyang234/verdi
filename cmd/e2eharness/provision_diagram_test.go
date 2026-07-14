package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/diagrambase"
)

// TestRunGitOut_Happy proves runGitOut returns the command's trimmed
// stdout. Local git only — no network (co-4).
func TestRunGitOut_Happy(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	dir := t.TempDir()
	if err := runGit(dir, nil, "init", "--quiet", "--initial-branch=main"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := runGit(dir, nil, "commit", "--quiet", "--no-verify", "--allow-empty", "-m", "seed"); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	sha, err := runGitOut(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("runGitOut: %v", err)
	}
	if len(sha) != 40 || strings.TrimSpace(sha) != sha {
		t.Fatalf("rev-parse HEAD = %q, want a trimmed 40-hex SHA", sha)
	}
}

// TestRunGitOut_Failure proves a failing invocation surfaces an error.
func TestRunGitOut_Failure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	if _, err := runGitOut(filepath.Join(t.TempDir(), "missing"), "rev-parse", "HEAD"); err == nil {
		t.Fatal("expected an error running git in a nonexistent dir")
	}
}

// TestProvisionDiagrams provisions into a scratch store and proves the
// derived fixture's pinned source_digest genuinely verifies through the
// SAME diagrambase seam the server gates with (never a stubbed
// comparison, ADJ-16 — the editor gates on source_digest, not digest),
// and the corrupted twin genuinely does not.
func TestProvisionDiagrams(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	scratch := t.TempDir()
	storeRoot := filepath.Join(scratch, "store")
	if err := runGit("", nil, "init", "--quiet", "--initial-branch=main", storeRoot); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := runGit(storeRoot, nil, "commit", "--quiet", "--no-verify", "--allow-empty", "-m", "seed"); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	// provisionDiagrams pushes; give it a local origin like the harness's.
	origin := filepath.Join(scratch, "origin.git")
	if err := runGit("", nil, "init", "--bare", "--quiet", "--initial-branch=main", origin); err != nil {
		t.Fatalf("git init --bare: %v", err)
	}
	if err := runGit(storeRoot, nil, "remote", "add", "origin", origin); err != nil {
		t.Fatal(err)
	}
	if err := runGit(storeRoot, nil, "push", "--quiet", "--set-upstream", "origin", "main"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(storeRoot, nil, "checkout", "--quiet", "-b", "design/editor"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(storeRoot, nil, "push", "--quiet", "--set-upstream", "origin", "design/editor"); err != nil {
		t.Fatal(err)
	}

	verificationPath, err := provisionDiagrams(scratch, storeRoot)
	if err != nil {
		t.Fatalf("provisionDiagrams: %v", err)
	}
	if _, err := runGitOut(storeRoot, "cat-file", "-e", "HEAD:.verdi/diagrams/"+diagramDerivedName+".mermaid"); err != nil {
		t.Fatalf("derived proposal not committed: %v", err)
	}
	if verificationPath == "" {
		t.Fatal("no verification path returned")
	}

	// The pinned source_digest matches the base body under the real
	// formula — the field peek/reset gate on (ADJ-16).
	sourceDigest, err := diagrambase.CanonicalGraphDigest([]byte(diagramBaseBody))
	if err != nil {
		t.Fatal(err)
	}
	derivedRaw, err := runGitOut(storeRoot, "show", "HEAD:.verdi/diagrams/"+diagramDerivedName+".mermaid")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(derivedRaw, "source_digest: "+sourceDigest) {
		t.Fatalf("derived fixture does not pin the real base source_digest %s:\n%s", sourceDigest, derivedRaw)
	}
	corruptRaw, err := runGitOut(storeRoot, "show", "HEAD:.verdi/diagrams/"+diagramCorruptName+".mermaid")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(corruptRaw, "source_digest: "+sourceDigest) {
		t.Fatalf("corrupted fixture pins the MATCHING source_digest; it must not verify")
	}
}
