package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// buildDexTestStore assembles a minimal, real git-backed store (verdi
// dex build execs git plumbing, so a bare temp directory with no history
// will not do): a single fixturegit commit carrying one ADR.
func buildDexTestStore(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
				".verdi/adr/0001-a.md": "---\n" +
					"id: adr/0001-a\n" +
					"kind: adr\n" +
					"title: \"ADR one\"\n" +
					"status: proposed\n" +
					"owners: [platform-team]\n" +
					"---\n# ADR one\n",
			},
			Message: "add adr",
		},
	})
	return repo.Dir
}

func TestRunDexBuild_Happy(t *testing.T) {
	root := buildDexTestStore(t)
	t.Chdir(root)
	outDir := filepath.Join(t.TempDir(), "site")

	var stdout, stderr bytes.Buffer
	got := runDexVerb([]string{"build", "-o", outDir}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDexVerb(build) = %d, want 0; stderr: %s", got, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(outDir, "index.html")); err != nil {
		t.Fatalf("expected %s/index.html to exist: %v", outDir, err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "a", "adr", "0001-a", "index.html")); err != nil {
		t.Fatalf("expected the adr permalink page to exist: %v", err)
	}
}

func TestRunDexVerb_Negative_NoSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := runDexVerb(nil, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runDexVerb(nil) = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr = %q, want a usage message", stderr.String())
	}
}

func TestRunDexVerb_Negative_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := runDexVerb([]string{"frobnicate"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runDexVerb(frobnicate) = %d, want 2", got)
	}
}

func TestRunDexBuild_Negative_MissingOutDirFlag(t *testing.T) {
	root := buildDexTestStore(t)
	t.Chdir(root)

	var stdout, stderr bytes.Buffer
	got := runDexVerb([]string{"build"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runDexVerb(build, no -o) = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "-o") {
		t.Fatalf("stderr = %q, want it to mention -o", stderr.String())
	}
}

func TestRunDexBuild_Negative_NoStoreRoot(t *testing.T) {
	t.Chdir(t.TempDir())

	var stdout, stderr bytes.Buffer
	got := runDexVerb([]string{"build", "-o", t.TempDir()}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runDexVerb(build) outside a store = %d, want 2", got)
	}
}
