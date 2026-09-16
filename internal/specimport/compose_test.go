package specimport

import (
	"bytes"
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

// sourceWithRetainedCommand is validMarkdownSource plus one extra,
// unrecognized section carrying a shell command — content RetainUnmapped
// keeps as retained-only source, never promoted into the candidate.
const sourceWithRetainedCommand = "# Sample Feature\n" +
	"\n" +
	"## Problem\n" +
	"\n" +
	"First line.\n" +
	"Second line.\n" +
	"\n" +
	"## Outcome\n" +
	"\n" +
	"Users get value.\n" +
	"\n" +
	"## Acceptance Criteria\n" +
	"\n" +
	"- Criterion one.\n" +
	"- Criterion two.\n" +
	"\n" +
	"## Appendix\n" +
	"\n" +
	"Run `git commit -m \"wip\"` before pushing.\n"

// TestCompose_RetainedCommandNeverPromoted proves a retained-only shell
// command in the source — never mapped to any target — never enters the
// composed candidate (spec-import-contract.md's own worked example:
// "if bytes.Contains(candidate, []byte(\"git commit -m\")) { t.Fatal(...)
// }"). Compose only ever writes Field.Text values; nothing in its
// pipeline copies raw, unmapped source bytes into the output at all, so
// this holds by construction — pinned here as a regression guard.
func TestCompose_RetainedCommandNeverPromoted(t *testing.T) {
	root := minimalStoreRoot(t)
	req := minimalRequest()
	req.Sources[0].Data = []byte(sourceWithRetainedCommand)
	req.RetainUnmapped = true
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static", "attestation"}},
		{Target: "ac-2", Evidence: []string{"static", "attestation"}},
	}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	requireNoBlocking(t, findings, candidate)
	if candidate == nil {
		t.Fatal("Compose returned nil bytes alongside zero blocking findings")
	}
	if bytes.Contains(candidate, []byte("git commit -m")) {
		t.Fatalf("retained shell command promoted into the candidate:\n%s", candidate)
	}
}

// TestCompose_BlockingFindings_ReturnsNilBytes pins Compose's own
// documented contract: an incomplete candidate (here, minimalRequest()'s
// two automatically-recognized ACs with no evidence mapped at all —
// Normalize's own missingEvidenceFindings already marks both blocking)
// returns nil bytes alongside the blocking findings, never a best-effort
// candidate a caller could mistake for one ready to commit.
func TestCompose_BlockingFindings_ReturnsNilBytes(t *testing.T) {
	root := minimalStoreRoot(t)
	req := minimalRequest() // no evidence mappings supplied

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if !hasBlocking(plan.Findings) {
		t.Fatalf("test fixture assumption broken: expected Normalize to already report a blocking missing-evidence finding: %+v", plan.Findings)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if candidate != nil {
		t.Fatalf("Compose returned %d bytes alongside a blocking finding, want nil:\n%s", len(candidate), candidate)
	}
	if !hasBlocking(findings) {
		t.Fatalf("want the blocking missing-evidence finding preserved in Compose's own result, got: %+v", findings)
	}
	var sawMissingEvidence bool
	for _, f := range findings {
		if f.Code == FindingMissingEvidence {
			sawMissingEvidence = true
		}
	}
	if !sawMissingEvidence {
		t.Fatalf("want a missing-evidence finding surfaced through Compose, got: %+v", findings)
	}
}

// TestCompose_F13PinnedFixture_BlockingFindings_NotAValidCandidate runs
// the F13 pinned fixture (profile_f13_test.go's own f13Request, already
// proven by TestNormalize_F13ProfileMissingStatementsAndAllEvidenceGaps to
// leave problem/outcome unmapped and every AC without evidence) through
// Compose end to end: the result must carry blocking findings and nil
// bytes — an F13 recognition pass alone is never a valid candidate.
func TestCompose_F13PinnedFixture_BlockingFindings_NotAValidCandidate(t *testing.T) {
	root := minimalStoreRoot(t)
	req := f13Request(t, true)

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if candidate != nil {
		t.Fatalf("F13 recognition alone composed a valid candidate; want blocking findings and nil bytes:\n%s", candidate)
	}
	if !hasBlocking(findings) {
		t.Fatalf("want at least one blocking finding for the unmapped F13 pinned fixture, got: %+v", findings)
	}
}
