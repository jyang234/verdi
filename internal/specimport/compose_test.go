package specimport

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// minimalStoreRoot returns a fresh temp directory that is a minimally
// valid store root for composeExternal: an empty committed zone (no
// pre-existing corpus) plus a bare verdi.yaml (store.Open succeeds,
// resolving model.Canonical() — the embedded default that maps class
// "feature" -> "feature.md" and "story" -> "story.md", internal/model/
// canonical.yaml).
func minimalStoreRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".verdi"), 0o755); err != nil {
		t.Fatalf("mkdir .verdi: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\n"), 0o644); err != nil {
		t.Fatalf("write verdi.yaml: %v", err)
	}
	return dir
}

// writeStoreFile writes content at root-relative path rel (e.g.
// ".verdi/specs/active/foo/spec.md"), creating parent directories.
func writeStoreFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// requireNoBlocking fails the test with every blocking finding listed.
func requireNoBlocking(t *testing.T, findings []Finding, candidate []byte) {
	t.Helper()
	for _, f := range findings {
		if f.Blocking {
			t.Fatalf("unexpected blocking finding %+v\ncandidate:\n%s", f, candidate)
		}
	}
}

// TestCompose_ExternalFeature_Happy is the smallest possible proof Compose
// compiles, runs, and produces a labeled, fully valid, placeholder-free
// feature candidate from a markdown-v1 request whose automatic
// recognition already resolved problem/outcome/two ACs plus explicit
// evidence mappings supplying the outcome floor's required attestation
// kind.
func TestCompose_ExternalFeature_Happy(t *testing.T) {
	root := minimalStoreRoot(t)
	req := minimalRequest()
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static", "attestation"}},
		{Target: "ac-2", Evidence: []string{"static", "attestation"}},
	}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	requireNoBlocking(t, plan.Findings, nil)

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	requireNoBlocking(t, findings, candidate)
	if candidate == nil {
		t.Fatal("Compose returned nil bytes alongside zero blocking findings")
	}

	fm, body, err := artifact.SplitFrontmatter(candidate)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v\n%s", err, candidate)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v\n%s", err, candidate)
	}
	if err := spec.ResolveObjectAnchors(body); err != nil {
		t.Fatalf("ResolveObjectAnchors: %v\n%s", err, candidate)
	}

	if len(spec.Stubs) != 0 {
		t.Fatalf("stubs not absent: %+v", spec.Stubs)
	}
	if len(spec.AcceptanceCriteria) != 2 {
		t.Fatalf("want 2 acceptance criteria, got %d: %+v\n%s", len(spec.AcceptanceCriteria), spec.AcceptanceCriteria, candidate)
	}
	if spec.Problem == nil || spec.Problem.Text != "First line.\nSecond line." {
		t.Fatalf("problem attribute not mapped: %+v", spec.Problem)
	}
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "## Problem\n\nFirst line.\nSecond line.\n") {
		t.Fatalf("mapped problem text does not appear in the body section:\n%s", bodyStr)
	}
	if strings.Contains(bodyStr, "TODO: design notes.") {
		t.Fatalf("a scaffold placeholder body section survived:\n%s", bodyStr)
	}
	if strings.Contains(string(candidate), "TODO: replace with real acceptance criteria before accept") {
		t.Fatalf("the generated placeholder AC text survived:\n%s", candidate)
	}
}
