// In-process behavioral tests for `verdi context project [--root DIR]`
// (design doc 2026-09-05-local-operator-disposition-design.md §2.4,
// ledger SI-178, plan Task 4): a thin CLI wrapper over the read-only
// internal/instructionprojection.Generate. Mirrors context_test.go/
// context_conflict_test.go's own cmdX(args, stdout, stderr) direct-call
// style (context_e2e_test.go owns the real-binary proofs elsewhere in
// this namespace).
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- fixture plumbing ---------------------------------------------------
//
// Fixture CONTENT is never hand-duplicated: the one-adapter store is read
// verbatim from internal/policyartifact/testdata/store (the same real,
// already-cross-validated fixture context_test.go's own
// contextPolicyStoreFiles reads), and the two-adapter store from
// internal/instructionprojection/testdata/multi-store — the library this
// verb wraps. Reading a sibling package's committed testdata by relative
// path (never importing the package) is the established convention both
// call sites already use. The zero-adapter and overlapping-adapter
// constitutions have no other committed home (they exist only to drive
// this verb's own refusal arms), so they are inlined here, copied from
// internal/instructionprojection/generate_test.go's own proven
// zeroAdapterStoreFiles() shape rather than invented fresh (this lane
// cannot import that unexported test helper across the package
// boundary, so the small, stable fixture text is copied, mirroring
// fixture_helpers_test.go's own "copied rather than imported per this
// lane's write-set boundary" convention).

// contextProjectVerdiYAML is the minimal store manifest every fixture
// root below carries, identical to context_test.go's
// buildContextCompileRepo's own literal.
const contextProjectVerdiYAML = "schema: verdi.layout/v1\n"

// contextProjectReadPolicyFiles reads rels (relative to base) and returns
// them keyed by ".verdi/policy/<rel>", failing the test on any read
// error.
func contextProjectReadPolicyFiles(t *testing.T, base string, rels []string) map[string]string {
	t.Helper()
	out := make(map[string]string, len(rels))
	for _, rel := range rels {
		data, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read policy fixture %s: %v", rel, err)
		}
		out[".verdi/policy/"+rel] = string(data)
	}
	return out
}

// contextProjectOneAdapterFiles returns the one-adapter fixture: adapter
// "codex", managed [AGENTS.md] (internal/policyartifact/testdata/store).
func contextProjectOneAdapterFiles(t *testing.T) map[string]string {
	t.Helper()
	base := filepath.Join("..", "..", "internal", "policyartifact", "testdata", "store")
	files := contextProjectReadPolicyFiles(t, base, []string{
		"constitution.md",
		"policies/go-toolchain.md",
		"overlays/frontend-go-version.md",
		"exemptions/legacy-service-go.md",
		"profiles/solo-default.md",
	})
	files[".verdi/verdi.yaml"] = contextProjectVerdiYAML
	return files
}

// contextProjectTwoAdapterFiles returns the two-adapter fixture: "codex"
// (managed [AGENTS.md]) and "claude-code" (managed [CLAUDE.md]), from
// internal/instructionprojection/testdata/multi-store — the realistic
// layout where claude-code's harness also discovers AGENTS.md.
func contextProjectTwoAdapterFiles(t *testing.T) map[string]string {
	t.Helper()
	base := filepath.Join("..", "..", "internal", "instructionprojection", "testdata", "multi-store", ".verdi", "policy")
	files := contextProjectReadPolicyFiles(t, base, []string{
		"constitution.md",
		"policies/go-toolchain.md",
		"profiles/solo-default.md",
	})
	files[".verdi/verdi.yaml"] = contextProjectVerdiYAML
	return files
}

// contextProjectSoloProfileDoc is copied (not imported — cross-package
// unexported access is impossible) from
// internal/instructionprojection/fixture_helpers_test.go's own
// soloDefaultProfileDoc.
const contextProjectSoloProfileDoc = `---
schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - {id: github-org, kind: forge}
role_mappings:
  - {role: author, trust_source: github-org, subjects: [alice]}
  - {role: policy-owner, trust_source: github-org, subjects: [alice]}
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
The solo operator profile.
`

// contextProjectConstitution renders a minimal, otherwise-valid
// constitution declaring exactly adaptersYAML — the same catalog/
// subjects/profile-selection shape as
// internal/instructionprojection/generate_test.go's own
// zeroAdapterStoreFiles(), so only the adapters list under test varies.
func contextProjectConstitution(title, adaptersYAML string) string {
	return "---\n" +
		"schema: verdi.policy-constitution/v1\n" +
		"id: policy-constitution/constitution\n" +
		"kind: policy-constitution\n" +
		`title: "` + title + "\"\n" +
		"owners: [platform-team]\n" +
		"selected_profile: solo-default\n" +
		"environments: []\n" +
		"catalog:\n" +
		"  roles: [author, reviewer, policy-owner]\n" +
		"  transitions: [accept, close]\n" +
		"  evidence_sources: [ci]\n" +
		"  escalation_metrics: [age-days]\n" +
		"subjects:\n" +
		"  action: []\n" +
		"  configuration: []\n" +
		"  capability: []\n" +
		"  resource: []\n" +
		"  identity: []\n" +
		"  evidence: []\n" +
		"adapters:" + adaptersYAML + "\n" +
		"---\n" +
		"Fixture constitution for context_project_test.go.\n"
}

// contextProjectZeroAdapterFiles is a minimal valid constitution store
// declaring adapters: [] — Generate legitimately writes nothing at all.
func contextProjectZeroAdapterFiles() map[string]string {
	return map[string]string{
		".verdi/verdi.yaml":                      contextProjectVerdiYAML,
		".verdi/policy/constitution.md":          contextProjectConstitution("Zero-adapter fixture constitution", " []"),
		".verdi/policy/profiles/solo-default.md": contextProjectSoloProfileDoc,
	}
}

// contextProjectOverlapFiles declares two adapters ("alpha", "beta") that
// both manage AGENTS.md — an unsatisfiable constitution
// (instructionprojection.ErrOverlappingManagedPath).
func contextProjectOverlapFiles() map[string]string {
	adapters := "\n" +
		"  - id: alpha\n" +
		"    version: \"1\"\n" +
		"    managed: [AGENTS.md]\n" +
		"    discovery_filenames: [AGENTS.md]\n" +
		"  - id: beta\n" +
		"    version: \"1\"\n" +
		"    managed: [AGENTS.md]\n" +
		"    discovery_filenames: [AGENTS.md]"
	return map[string]string{
		".verdi/verdi.yaml":                      contextProjectVerdiYAML,
		".verdi/policy/constitution.md":          contextProjectConstitution("Overlapping adapters fixture constitution", adapters),
		".verdi/policy/profiles/solo-default.md": contextProjectSoloProfileDoc,
	}
}

// contextProjectNotAdoptedFiles carries a store manifest but no
// .verdi/policy/ at all (policyauthority.ErrNotAdopted).
func contextProjectNotAdoptedFiles() map[string]string {
	return map[string]string{
		".verdi/verdi.yaml": contextProjectVerdiYAML,
	}
}

// writeContextProjectTree materializes files under root, creating parent
// directories as needed (fixture_helpers_test.go's writeTree, copied per
// this lane's write-set boundary — see the fixture-plumbing note above).
func writeContextProjectTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

// contextProjectSha256 independently recomputes path's content digest in
// this verb's own "sha256:"+hex form, so happy-path assertions never
// trust the same digest function the production code under test uses.
func contextProjectSha256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// --- flag-shape failures (never touch a store root) ----------------------

func TestCmdContextProject_FlagShapeFailures(t *testing.T) {
	t.Chdir(t.TempDir())

	cases := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "unknown flag", args: []string{"--bogus"}},
		{name: "root missing value", args: []string{"--root"}, wantStderr: "--root requires a value"},
		{name: "duplicate root", args: []string{"--root", "a", "--root", "b"}, wantStderr: "--root given more than once"},
		{name: "duplicate root via =", args: []string{"--root=a", "--root=b"}, wantStderr: "--root given more than once"},
		{name: "extra positional argument", args: []string{"extra"}},
		{name: "root with extra positional", args: []string{"--root", "a", "extra"}},
		{name: "empty root value via =", args: []string{"--root="}, wantStderr: "--root requires a value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := cmdContextProject(tc.args, &stdout, &stderr)
			if got != 2 {
				t.Fatalf("cmdContextProject(%v) = %d, want 2; stderr=%s", tc.args, got, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty on a flag-shape error", stdout.String())
			}
			if stderr.Len() == 0 {
				t.Fatal("stderr is empty, want a diagnostic")
			}
			if tc.wantStderr != "" && !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), tc.wantStderr)
			}
		})
	}
}

// --- root resolution: operational (exit 2) -------------------------------

func TestCmdContextProject_NoStoreRoot(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	got := cmdContextProject(nil, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdContextProject(nil) = %d, want 2; stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr empty, want a diagnostic")
	}
}

func TestCmdContextProject_RootFlagUnusable(t *testing.T) {
	t.Chdir(t.TempDir())

	parent := t.TempDir()
	notAStore := filepath.Join(parent, "not-a-store")
	if err := os.MkdirAll(notAStore, 0o755); err != nil {
		t.Fatal(err)
	}
	notADir := filepath.Join(parent, "plain-file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		root string
	}{
		{"nonexistent directory", filepath.Join(parent, "does-not-exist")},
		{"directory without verdi.yaml", notAStore},
		{"root is a plain file", notADir},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := cmdContextProject([]string{"--root", tc.root}, &stdout, &stderr)
			if got != 2 {
				t.Fatalf("cmdContextProject(--root %s) = %d, want 2; stderr=%s", tc.root, got, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if stderr.Len() == 0 {
				t.Fatal("stderr empty, want a diagnostic")
			}
		})
	}
}

// --- policy/constitution refusals: verdict (exit 1) -----------------------

func TestCmdContextProject_NotAdopted_ExitsVerdict(t *testing.T) {
	root := t.TempDir()
	writeContextProjectTree(t, root, contextProjectNotAdoptedFiles())

	var stdout, stderr bytes.Buffer
	got := cmdContextProject([]string{"--root", root}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("cmdContextProject on an unadopted store = %d, want 1 (verdict); stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on a refusal", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr empty, want a diagnostic naming the refusal")
	}
}

func TestCmdContextProject_OverlappingManagedPaths_ExitsVerdict(t *testing.T) {
	root := t.TempDir()
	writeContextProjectTree(t, root, contextProjectOverlapFiles())

	var stdout, stderr bytes.Buffer
	got := cmdContextProject([]string{"--root", root}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("cmdContextProject on overlapping managed paths = %d, want 1 (verdict); stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on a refusal", stdout.String())
	}
	if !strings.Contains(stderr.String(), "AGENTS.md") {
		t.Fatalf("stderr = %q, want it to name the overlapping path AGENTS.md", stderr.String())
	}

	// Nothing was written: an unsatisfiable constitution is refused before
	// any file touches disk.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != ".verdi" {
		t.Fatalf("root entries = %v, want only the seeded .verdi tree", entries)
	}
}

// TestCmdContextProject_IncompleteAdoption_ExitsOperational proves the
// half-adopted state (.verdi/policy/ exists, no constitution.md) is
// classified operational (exit 2), not verdict — matching
// cmd/verdi/context_conflict.go's own IsNotAdopted precedent
// (internal/policyconflict.Service only folds policyauthority.ErrNotAdopted
// into its verdict-class refusal, never ErrIncompleteAdoption), and the
// design doc's own two named verdict examples ("overlapping managed paths,
// no constitution") do not name this distinct, already-started-but-broken
// state.
func TestCmdContextProject_IncompleteAdoption_ExitsOperational(t *testing.T) {
	root := t.TempDir()
	writeContextProjectTree(t, root, map[string]string{".verdi/verdi.yaml": contextProjectVerdiYAML})
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "policy"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdContextProject([]string{"--root", root}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdContextProject on an incomplete adoption = %d, want 2 (operational); stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on a refusal", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr empty, want a diagnostic")
	}
}

// --- symlink / non-regular-file refusals: operational (exit 2) -----------

func TestCmdContextProject_SymlinkedProjectionsDir_Refused(t *testing.T) {
	root := t.TempDir()
	writeContextProjectTree(t, root, contextProjectOneAdapterFiles(t))

	elsewhere := t.TempDir()
	if err := os.Symlink(elsewhere, filepath.Join(root, ".verdi", "policy", "projections")); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdContextProject([]string{"--root", root}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdContextProject with a symlinked projections dir = %d, want 2; stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on a refusal", stdout.String())
	}
	if !strings.Contains(stderr.String(), "projections") {
		t.Fatalf("stderr = %q, want it to name the projections path", stderr.String())
	}
}

func TestCmdContextProject_SymlinkedManagedFile_Refused(t *testing.T) {
	root := t.TempDir()
	writeContextProjectTree(t, root, contextProjectOneAdapterFiles(t))

	target := filepath.Join(t.TempDir(), "elsewhere.md")
	if err := os.WriteFile(target, []byte("not managed content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdContextProject([]string{"--root", root}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdContextProject with a symlinked managed file = %d, want 2; stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on a refusal", stdout.String())
	}
	if !strings.Contains(stderr.String(), "AGENTS.md") {
		t.Fatalf("stderr = %q, want it to name AGENTS.md", stderr.String())
	}
}

func TestCmdContextProject_ManagedPathIsDirectory_Refused(t *testing.T) {
	root := t.TempDir()
	writeContextProjectTree(t, root, contextProjectOneAdapterFiles(t))

	if err := os.MkdirAll(filepath.Join(root, "AGENTS.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdContextProject([]string{"--root", root}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdContextProject with a directory at a managed path = %d, want 2; stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on a refusal", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr empty, want a diagnostic")
	}
}

// --- happy paths -----------------------------------------------------------

func TestCmdContextProject_OneAdapter_HappyPath(t *testing.T) {
	root := t.TempDir()
	writeContextProjectTree(t, root, contextProjectOneAdapterFiles(t))

	var stdout, stderr bytes.Buffer
	got := cmdContextProject([]string{"--root", root}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdContextProject = %d, want 0; stderr=%s", got, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on success", stderr.String())
	}

	agentsDigest := contextProjectSha256(t, filepath.Join(root, "AGENTS.md"))
	manifestDigest := contextProjectSha256(t, filepath.Join(root, ".verdi", "policy", "projections", "codex.json"))

	want := manifestDigest + "  .verdi/policy/projections/codex.json\n" +
		agentsDigest + "  AGENTS.md\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestCmdContextProject_TwoAdapters_HappyPath(t *testing.T) {
	root := t.TempDir()
	writeContextProjectTree(t, root, contextProjectTwoAdapterFiles(t))

	var stdout, stderr bytes.Buffer
	got := cmdContextProject([]string{"--root", root}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdContextProject = %d, want 0; stderr=%s", got, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on success", stderr.String())
	}

	claudeManifest := contextProjectSha256(t, filepath.Join(root, ".verdi", "policy", "projections", "claude-code.json"))
	codexManifest := contextProjectSha256(t, filepath.Join(root, ".verdi", "policy", "projections", "codex.json"))
	agents := contextProjectSha256(t, filepath.Join(root, "AGENTS.md"))
	claude := contextProjectSha256(t, filepath.Join(root, "CLAUDE.md"))

	want := claudeManifest + "  .verdi/policy/projections/claude-code.json\n" +
		codexManifest + "  .verdi/policy/projections/codex.json\n" +
		agents + "  AGENTS.md\n" +
		claude + "  CLAUDE.md\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestCmdContextProject_ZeroAdapters_EmptyOutput(t *testing.T) {
	root := t.TempDir()
	writeContextProjectTree(t, root, contextProjectZeroAdapterFiles())

	var stdout, stderr bytes.Buffer
	got := cmdContextProject([]string{"--root", root}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdContextProject = %d, want 0; stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when no adapters are declared", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on success", stderr.String())
	}
}

// --- determinism ------------------------------------------------------------

func TestCmdContextProject_DeterministicAcrossIndependentRoots(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	writeContextProjectTree(t, rootA, contextProjectOneAdapterFiles(t))
	writeContextProjectTree(t, rootB, contextProjectOneAdapterFiles(t))

	var stdoutA, stderrA, stdoutB, stderrB bytes.Buffer
	if got := cmdContextProject([]string{"--root", rootA}, &stdoutA, &stderrA); got != 0 {
		t.Fatalf("cmdContextProject(rootA) = %d, want 0; stderr=%s", got, stderrA.String())
	}
	if got := cmdContextProject([]string{"--root", rootB}, &stdoutB, &stderrB); got != 0 {
		t.Fatalf("cmdContextProject(rootB) = %d, want 0; stderr=%s", got, stderrB.String())
	}
	if stdoutA.String() != stdoutB.String() {
		t.Fatalf("two independent runs over identical fixture content produced different stdout:\nA: %q\nB: %q", stdoutA.String(), stdoutB.String())
	}
	if stdoutA.Len() == 0 {
		t.Fatal("stdout empty, want the one-adapter output")
	}
}

func TestCmdContextProject_IdempotentAcrossRepeatedRuns(t *testing.T) {
	root := t.TempDir()
	writeContextProjectTree(t, root, contextProjectOneAdapterFiles(t))

	var first, second bytes.Buffer
	var stderr1, stderr2 bytes.Buffer
	if got := cmdContextProject([]string{"--root", root}, &first, &stderr1); got != 0 {
		t.Fatalf("first run = %d, want 0; stderr=%s", got, stderr1.String())
	}
	if got := cmdContextProject([]string{"--root", root}, &second, &stderr2); got != 0 {
		t.Fatalf("second run = %d, want 0; stderr=%s", got, stderr2.String())
	}
	if first.String() != second.String() {
		t.Fatalf("re-running against an already-generated store changed output:\nfirst:  %q\nsecond: %q", first.String(), second.String())
	}
}

// --- --root wired end-to-end, and dispatcher routing ------------------------

// TestCmdContextProject_RootFlagOverridesCwd proves --root is honored from
// a cwd that is itself not a store at all (never falling back to
// store.FindRoot(".")).
func TestCmdContextProject_RootFlagOverridesCwd(t *testing.T) {
	fixture := t.TempDir()
	writeContextProjectTree(t, fixture, contextProjectOneAdapterFiles(t))

	t.Chdir(t.TempDir())

	var stdout, stderr bytes.Buffer
	got := cmdContextProject([]string{"--root", fixture}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdContextProject(--root fixture) = %d, want 0; stderr=%s", got, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("stdout empty, want the one-adapter output")
	}
}

// TestCmdContext_ProjectRouting proves the namespace routes "project" to
// this verb's own grammar rather than the compile usage line, without
// widening the namespace to accept a similar-looking word.
func TestCmdContext_ProjectRouting(t *testing.T) {
	t.Chdir(t.TempDir())

	var stdout, stderr bytes.Buffer
	if got := cmdContext([]string{"project"}, strings.NewReader(""), &stdout, &stderr); got != 2 {
		t.Fatalf("cmdContext(project) = %d, want 2; stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.String() == contextCompileUsage+"\n" {
		t.Fatal("cmdContext(project) fell through to the namespace usage banner instead of routing to context project")
	}

	stdout.Reset()
	stderr.Reset()
	if got := cmdContext([]string{"projects"}, strings.NewReader(""), &stdout, &stderr); got != 2 {
		t.Fatalf("cmdContext(projects) = %d, want 2", got)
	}
	if stderr.String() != contextCompileUsage+"\n" {
		t.Fatalf("stderr = %q, want the namespace usage line %q", stderr.String(), contextCompileUsage+"\n")
	}
}
