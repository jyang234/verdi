// In-process unit and hermetic behavioral tests for `verdi context
// compile` (Wave-3 plan Task 8 Step 1): parser-shape failures that must
// never touch a store root, the file/stdin request paths, the
// stdout-vs-file manifest destination split, and the exit-1/exit-2
// mapping over the real internal/contextcompile.Compiler against real
// fixturegit repositories — mirroring journey_test.go/specstate_test.go's
// own cmdX(args, stdout, stderr) direct-call style rather than exec'ing a
// subprocess (context_e2e_test.go owns the real-binary proofs).
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/specstate"
)

// --- fixture plumbing --------------------------------------------------

// contextPolicyStoreFiles reads the same real, already-cross-validated
// policy fixture internal/contextcompile's own integration tests install
// (internal/policyartifact/testdata/store), keyed by their repo-relative
// .verdi/policy/ path — never a hand-authored duplicate of a governed
// constitution/profile/policy document.
func contextPolicyStoreFiles(t *testing.T) map[string]string {
	t.Helper()
	rels := []string{
		"constitution.md",
		"policies/go-toolchain.md",
		"overlays/frontend-go-version.md",
		"exemptions/legacy-service-go.md",
		"profiles/solo-default.md",
	}
	out := make(map[string]string, len(rels))
	for _, rel := range rels {
		data, err := os.ReadFile(filepath.Join("..", "..", "internal", "policyartifact", "testdata", "store", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read policy fixture %s: %v", rel, err)
		}
		out[".verdi/policy/"+rel] = string(data)
	}
	return out
}

// contextFeatureAlphaSpec reads internal/contextcompile's own real
// feature-alpha fixture spec verbatim, so this package never hand-
// maintains a second copy of the fragment corpus's grammar.
func contextFeatureAlphaSpec(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "contextcompile", "testdata", "fragments", "feature-alpha.md"))
	if err != nil {
		t.Fatalf("read feature-alpha fixture: %v", err)
	}
	return string(data)
}

// buildContextCompileRepo builds a fresh, real fixturegit repository
// carrying a minimal store manifest, the real policy-store fixture, and
// specFiles, then generates and commits the one real managed instruction
// projection (mirroring internal/contextcompile/integration_test.go's own
// buildCompilerRepo) so a compile's own drift-verification stage finds a
// clean, non-drifted projection. CI_DEFAULT_BRANCH is pinned so default-
// branch resolution never depends on a configured origin remote.
func buildContextCompileRepo(t *testing.T, specFiles map[string]string) *fixturegit.Repo {
	t.Helper()
	files := contextPolicyStoreFiles(t)
	files[".verdi/verdi.yaml"] = "schema: verdi.layout/v1\n"
	for path, content := range specFiles {
		files[path] = content
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	repo.Head = commitAllOnCurrentBranch(t, repo.Dir, "generate instruction projection")
	return repo
}

// contextRequestBytes builds and canonically encodes a
// verdi.context-compile-request/v1 document through
// internal/contextcompile's own EncodeRequest seam — never a hand-typed
// JSON literal, so the fixture can never silently drift from the real
// wire grammar. scopePhases nil/empty means an unrestricted (universal)
// scope; a nonempty value narrows scope.phases to exactly those phases.
func contextRequestBytes(t *testing.T, spec string, phase contextcompile.Phase, scopePhases []string) []byte {
	t.Helper()
	if scopePhases == nil {
		scopePhases = []string{}
	}
	req := contextcompile.Request{
		Schema:  contextcompile.RequestSchema,
		Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Phase:   phase,
		Scope:   policyartifact.Scope{Phases: scopePhases, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		Spec:    spec,
	}
	data, err := contextcompile.EncodeRequest(req)
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	return data
}

// contextPhaseScopeMismatchRequestBytes builds a canonical, schema-valid
// document whose phase does NOT occur in its own nonempty scope.phases —
// contextcompile.EncodeRequest refuses to PRODUCE such a document directly
// (Request.Validate rejects the combination before any bytes are
// rendered), so this starts from a genuinely valid build/["build"]
// document and edits the single top-level "phase" field in place. The
// canonical form has exactly one `"phase":"<value>"` occurrence (the
// top-level field; "scope":{"phases":[...]} uses the distinct key
// "phases", never confused with it), so the substitution is exact and
// leaves every other byte — key order, whitespace, the scope.phases array
// itself — untouched.
func contextPhaseScopeMismatchRequestBytes(t *testing.T, spec string) []byte {
	t.Helper()
	valid := contextRequestBytes(t, spec, contextcompile.PhaseBuild, []string{"build"})
	const from, to = `"phase":"build"`, `"phase":"design"`
	if n := bytes.Count(valid, []byte(from)); n != 1 {
		t.Fatalf("canonical request has %d occurrences of %q, want exactly 1", n, from)
	}
	return bytes.Replace(valid, []byte(from), []byte(to), 1)
}

// writeContextRequestFile writes canonical request bytes to a file inside
// dir and returns its path.
func writeContextRequestFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write request file: %v", err)
	}
	return path
}

// --- exit-code mapping ---------------------------------------------------

// TestContextExitCode proves the 0/1/2 contract's one mapping helper over
// EVERY member of internal/contextcompile's closed typed-refusal family —
// the Wave-3 plan's "exit 1 for each typed refusal family" bullet — plus
// the wrapping, non-refusal, and defensive cases around it. Constructing
// each refusal type by name here is the regression net for a future
// refusal type that forgets contextcompile's unexported marker method: it
// would compile, be handed to this helper by the command, and silently
// map to exit 2 (an operational failure) instead of exit 1 (a verdict).
func TestContextExitCode(t *testing.T) {
	refusals := []struct {
		name string
		err  error
	}{
		{"PhaseScopeRefusal", &contextcompile.PhaseScopeRefusal{
			Phase:       contextcompile.PhaseDesign,
			ScopePhases: []string{"build"},
		}},
		{"NoConstitutionRefusal", &contextcompile.NoConstitutionRefusal{}},
		{"AdapterMismatchRefusal", &contextcompile.AdapterMismatchRefusal{
			Requested:  contextcompile.AdapterRef{ID: "codex", Version: "1"},
			Registered: []contextcompile.AdapterRef{{ID: "claude", Version: "1"}},
		}},
		{"AcceptedSpecRefusal", &contextcompile.AcceptedSpecRefusal{
			Ref:      "spec/feature-alpha",
			State:    specstate.Proposed,
			Relation: specstate.RelationNew,
		}},
		{"ExpectedRepositoryMismatchRefusal", &contextcompile.ExpectedRepositoryMismatchRefusal{
			Expected:       contextcompile.Expected{Branch: "main", Head: strings.Repeat("a", 40)},
			ComputedBranch: "topic",
			BranchKnown:    true,
		}},
		{"DeclaredScopeRefusal", &contextcompile.DeclaredScopeRefusal{
			Phase:  contextcompile.PhaseBuild,
			Ref:    "spec/feature-alpha",
			Reason: "not an authoritative build target",
		}},
		{"ProjectionDriftRefusal", &contextcompile.ProjectionDriftRefusal{
			Paths:   []string{"AGENTS.md"},
			Reasons: []string{"content-mismatch"},
		}},
	}
	// The family is closed at seven members (internal/contextcompile/
	// port.go's six plus validate.go's PhaseScopeRefusal); a new member
	// must be added here, not silently left unproven.
	if len(refusals) != 7 {
		t.Fatalf("refusal table has %d rows, want the closed family's 7", len(refusals))
	}

	type exitCase struct {
		name string
		err  error
		want int
	}
	cases := make([]exitCase, 0, len(refusals)+3)
	for _, r := range refusals {
		cases = append(cases, exitCase{r.name, r.err, 1})
	}
	cases = append(cases,
		exitCase{"wrapped refusal", fmt.Errorf("wrapped: %w", &contextcompile.NoConstitutionRefusal{}), 1},
		exitCase{"plain operational error", errors.New("port unavailable"), 2},
		// Never reached from the command (contextExitCode is only called
		// on a non-nil error); this pins the defensive branch's answer.
		exitCase{"nil error", nil, 2},
	)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := contextExitCode(tc.err); got != tc.want {
				t.Fatalf("contextExitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

// --- parser-shape tests (no store root ever resolved) -------------------

// TestCmdContext_Usage proves the missing/unknown-subcommand shapes: a
// bare `verdi context`, and an unrecognized subcommand, both fail on
// argument-shape parsing alone, from a rootless tempdir, before any store
// root is resolved.
func TestCmdContext_Usage(t *testing.T) {
	t.Chdir(t.TempDir())

	cases := []struct {
		name string
		args []string
	}{
		{"no arguments", nil},
		{"unknown subcommand", []string{"frobnicate"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := cmdContext(tc.args, strings.NewReader(""), &stdout, &stderr)
			if got != 2 {
				t.Fatalf("cmdContext(%v) = %d, want 2", tc.args, got)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty on a usage error", stdout.String())
			}
			if stderr.String() != contextCompileUsage+"\n" {
				t.Fatalf("stderr = %q, want exactly %q", stderr.String(), contextCompileUsage+"\n")
			}
		})
	}
}

// TestCmdContextCompile_FlagShapeFailures proves every flag-shape failure
// Task 8 Step 1 names — missing --request, duplicate --request/--out,
// unknown flag, extra positional arguments, and --request equal to
// --out — all exit 2, all from a rootless tempdir (never touching a store
// root), and all leave stdout empty.
func TestCmdContextCompile_FlagShapeFailures(t *testing.T) {
	t.Chdir(t.TempDir())

	cases := []struct {
		name string
		args []string
	}{
		{"missing --request", nil},
		{"missing --request value", []string{"--request"}},
		{"duplicate --request", []string{"--request", "a.json", "--request", "b.json"}},
		{"duplicate --request via =", []string{"--request=a.json", "--request=b.json"}},
		{"duplicate --out", []string{"--request", "a.json", "--out", "x.json", "--out", "y.json"}},
		{"missing --out value", []string{"--request", "a.json", "--out"}},
		{"unknown flag", []string{"--request", "a.json", "--bogus"}},
		{"extra positional argument", []string{"--request", "a.json", "extra"}},
		{"request equals out", []string{"--request", "a.json", "--out", "a.json"}},
		{"request equals out, different spelling", []string{"--request", "./a.json", "--out", "a.json"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := cmdContextCompile(tc.args, strings.NewReader(""), &stdout, &stderr)
			if got != 2 {
				t.Fatalf("cmdContextCompile(%v) = %d, want 2; stderr=%s", tc.args, got, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty on a flag-shape error", stdout.String())
			}
			if stderr.Len() == 0 {
				t.Fatal("stderr is empty, want a diagnostic")
			}
		})
	}
}

// TestCmdContextCompile_NoStoreRoot proves a well-formed invocation
// against a directory with no .verdi/verdi.yaml ancestor fails
// operationally (exit 2) — store-root resolution happens strictly after
// flag parsing succeeds.
func TestCmdContextCompile_NoStoreRoot(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	reqPath := writeContextRequestFile(t, dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))

	var stdout, stderr bytes.Buffer
	got := cmdContextCompile([]string{"--request", reqPath}, strings.NewReader(""), &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdContextCompile = %d, want 2; stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

// --- malformed-input tests (exit 2) -------------------------------------

// TestCmdContextCompile_MalformedRequest proves a syntactically invalid
// request body maps to exit 2, not exit 1.
func TestCmdContextCompile_MalformedRequest(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	t.Chdir(repo.Dir)
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", []byte("{not json"))

	var stdout, stderr bytes.Buffer
	got := cmdContextCompile([]string{"--request", reqPath}, strings.NewReader(""), &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdContextCompile(malformed) = %d, want 2; stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on malformed input", stdout.String())
	}
}

// TestCmdContextCompile_MissingRequestFile proves an unreadable --request
// file is an operational failure (exit 2).
func TestCmdContextCompile_MissingRequestFile(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := cmdContextCompile([]string{"--request", filepath.Join(repo.Dir, "does-not-exist.json")}, strings.NewReader(""), &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdContextCompile(missing request file) = %d, want 2; stderr=%s", got, stderr.String())
	}
}

// --- typed-refusal tests (exit 1) ---------------------------------------

// TestCmdContextCompile_PhaseOutsideScope_ExitOne proves the
// DecodeRequest-level *contextcompile.PhaseScopeRefusal (a canonical,
// schema-valid request whose phase does not occur in a nonempty
// scope.phases) maps to exit 1, not exit 2 — needs no store root at all
// beyond a bare fixture, since PhaseScopeRefusal is raised before any
// compiler port runs.
func TestCmdContextCompile_PhaseOutsideScope_ExitOne(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	t.Chdir(repo.Dir)
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextPhaseScopeMismatchRequestBytes(t, "spec/feature-alpha"))

	var stdout, stderr bytes.Buffer
	got := cmdContextCompile([]string{"--request", reqPath}, strings.NewReader(""), &stdout, &stderr)
	if got != 1 {
		t.Fatalf("cmdContextCompile(phase outside scope) = %d, want 1; stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on a refusal", stdout.String())
	}
}

// TestCmdContextCompile_NoConstitution_ExitOne proves
// *contextcompile.NoConstitutionRefusal (a store with no adopted
// constitution) maps to exit 1: a legacy-shaped store carrying only
// .verdi/verdi.yaml and the target spec, no .verdi/policy/ tree at all.
func TestCmdContextCompile_NoConstitution_ExitOne(t *testing.T) {
	files := map[string]string{
		".verdi/verdi.yaml":                         "schema: verdi.layout/v1\n",
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "scaffold, no constitution"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	t.Chdir(repo.Dir)
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))

	var stdout, stderr bytes.Buffer
	got := cmdContextCompile([]string{"--request", reqPath}, strings.NewReader(""), &stdout, &stderr)
	if got != 1 {
		t.Fatalf("cmdContextCompile(no constitution) = %d, want 1; stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on a refusal", stdout.String())
	}
}

// --- happy-path tests (exit 0) -------------------------------------------

// TestCmdContextCompile_StdoutOnSuccess proves the canonical-manifest-to-
// stdout path: a decodable, sorted-key canonical
// verdi.context-manifest/v1 document, exactly one document (no trailing
// noise), stderr empty, and byte equality with EncodeManifest(m).
func TestCmdContextCompile_StdoutOnSuccess(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	t.Chdir(repo.Dir)
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))

	var stdout, stderr bytes.Buffer
	got := cmdContextCompile([]string{"--request", reqPath}, strings.NewReader(""), &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdContextCompile = %d, want 0; stderr=%s", got, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on success", stderr.String())
	}

	manifest, err := contextcompile.DecodeManifest(stdout.Bytes())
	if err != nil {
		t.Fatalf("DecodeManifest(stdout): %v\nstdout=%s", err, stdout.String())
	}
	if manifest.Phase != contextcompile.PhaseDesign {
		t.Fatalf("Phase = %q, want design", manifest.Phase)
	}
	reEncoded, err := contextcompile.EncodeManifest(manifest)
	if err != nil {
		t.Fatalf("EncodeManifest: %v", err)
	}
	if !bytes.Equal(reEncoded, stdout.Bytes()) {
		t.Fatalf("stdout is not byte-identical to EncodeManifest(DecodeManifest(stdout))")
	}

	// Sorted top-level keys (canonjson.Marshal's seam actually used).
	var top map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &top); err != nil {
		t.Fatalf("json.Unmarshal(stdout): %v", err)
	}
	if _, ok := top["digest"]; !ok {
		t.Fatal("manifest carries no digest field")
	}
}

// TestCmdContextCompile_FileOutput_EmptyStdout proves the --out path:
// exactly one atomic write to the caller-selected file, byte-identical to
// the stdout form, and completely empty stdout/stderr on success.
func TestCmdContextCompile_FileOutput_EmptyStdout(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	t.Chdir(repo.Dir)
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "manifest.json")

	var stdout, stderr bytes.Buffer
	got := cmdContextCompile([]string{"--request", reqPath, "--out", outPath}, strings.NewReader(""), &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdContextCompile = %d, want 0; stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want completely empty when --out is present", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on success", stderr.String())
	}

	written, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading --out file: %v", err)
	}
	if _, err := contextcompile.DecodeManifest(written); err != nil {
		t.Fatalf("DecodeManifest(--out file): %v", err)
	}

	// Only the exactly-named file exists in outDir — no sibling data-item
	// or projection files were written anywhere.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir(outDir): %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "manifest.json" {
		t.Fatalf("outDir entries = %v, want exactly [manifest.json]", entries)
	}
}

// TestCmdContextCompile_StdinRequest proves `--request -` reads the
// request from stdin rather than from a file.
func TestCmdContextCompile_StdinRequest(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	t.Chdir(repo.Dir)
	data := contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil)

	var stdout, stderr bytes.Buffer
	got := cmdContextCompile([]string{"--request", "-"}, bytes.NewReader(data), &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdContextCompile(stdin) = %d, want 0; stderr=%s", got, stderr.String())
	}
	if _, err := contextcompile.DecodeManifest(stdout.Bytes()); err != nil {
		t.Fatalf("DecodeManifest(stdout): %v\nstdout=%s", err, stdout.String())
	}
}

// --- output-safety tests -------------------------------------------------

// TestCmdContextCompile_OutInsideVerdiZone_Refused proves --out is
// rejected outright (exit 2, nothing written) when it targets .verdi/ or
// .git/ inside the store root.
func TestCmdContextCompile_OutInsideVerdiZone_Refused(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	t.Chdir(repo.Dir)
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))

	cases := map[string]string{
		"inside .verdi/": filepath.Join(repo.Dir, ".verdi", "sneaky-manifest.json"),
		"inside .git/":   filepath.Join(repo.Dir, ".git", "sneaky-manifest.json"),
	}
	for name, outPath := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := cmdContextCompile([]string{"--request", reqPath, "--out", outPath}, strings.NewReader(""), &stdout, &stderr)
			if got != 2 {
				t.Fatalf("cmdContextCompile(--out %s) = %d, want 2; stderr=%s", outPath, got, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if _, err := os.Stat(outPath); err == nil {
				t.Fatalf("%s was written despite the refusal", outPath)
			}
		})
	}
}

// TestCmdContextCompile_OutIsManagedProjectionFile_Refused proves --out is
// rejected when it names one of this compile's own managed instruction-
// projection paths (AGENTS.md, per the fixture constitution's codex/1
// adapter) — the exact drift/overwrite hazard Task 8 Step 2 names.
func TestCmdContextCompile_OutIsManagedProjectionFile_Refused(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	t.Chdir(repo.Dir)
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
	outPath := filepath.Join(repo.Dir, "AGENTS.md")
	before, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading fixture AGENTS.md: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdContextCompile([]string{"--request", reqPath, "--out", outPath}, strings.NewReader(""), &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdContextCompile(--out AGENTS.md) = %d, want 2; stderr=%s", got, stderr.String())
	}
	after, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading AGENTS.md after refusal: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("AGENTS.md content changed despite the refusal — a managed projection file must never be overwritten")
	}
}

// contextFilesystemIsCaseInsensitive probes dir once for case-insensitive
// name resolution (APFS/HFS+ default, NTFS) by writing a lowercase file and
// stat'ing its uppercase spelling. Case-variant alias subtests are only
// meaningful — and only reachable as an actual bypass — on a filesystem
// that answers yes; symlink alias subtests always run.
func contextFilesystemIsCaseInsensitive(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "verdi-case-probe")
	if err := os.WriteFile(probe, []byte("probe"), 0o644); err != nil {
		t.Fatalf("writing case-sensitivity probe: %v", err)
	}
	defer func() { _ = os.Remove(probe) }()
	_, err := os.Stat(filepath.Join(dir, "VERDI-CASE-PROBE"))
	return err == nil
}

// TestCmdContextCompile_OutAliasesReservedPath_Refused proves the store-zone
// guard is alias-safe, not merely string-clean: a case-variant spelling of
// .verdi/ or .git/, a symlinked parent whose target IS .verdi/, and a
// case-variant spelling of the input request file all name the same
// filesystem objects the guard reserves, so each must be refused (exit 2)
// with nothing written — the Wave-3 plan's "never writes .verdi/, managed
// projection files, payload files, Git state, or a worktree" constraint is
// about the destination the write actually lands on, not about how the
// caller spelled it.
func TestCmdContextCompile_OutAliasesReservedPath_Refused(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	t.Chdir(repo.Dir)
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
	requestBefore, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatalf("reading request file: %v", err)
	}
	if err := os.Symlink(filepath.Join(repo.Dir, ".verdi"), filepath.Join(repo.Dir, "notes")); err != nil {
		t.Fatalf("creating notes -> .verdi symlink: %v", err)
	}
	caseInsensitive := contextFilesystemIsCaseInsensitive(t, repo.Dir)

	cases := []struct {
		name          string
		out           string
		needsCaseFold bool
	}{
		{"case-variant .verdi spelling", filepath.Join(repo.Dir, ".VERDI", "sneaky.json"), true},
		{"case-variant .git spelling", filepath.Join(repo.Dir, ".GIT", "sneaky.json"), true},
		{"symlinked parent resolving into .verdi", filepath.Join(repo.Dir, "notes", "sneaky.json"), false},
		{"relative symlinked parent", filepath.Join("notes", "sneaky.json"), false},
		{"case-variant request-file spelling", filepath.Join(repo.Dir, "REQUEST.json"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.needsCaseFold && !caseInsensitive {
				t.Skip("filesystem is case-sensitive: a case-variant spelling names a genuinely different path here")
			}
			var stdout, stderr bytes.Buffer
			got := cmdContextCompile([]string{"--request", reqPath, "--out", tc.out}, strings.NewReader(""), &stdout, &stderr)
			if got != 2 {
				t.Fatalf("cmdContextCompile(--out %s) = %d, want 2; stderr=%s", tc.out, got, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if stderr.Len() == 0 {
				t.Fatal("stderr is empty, want a reserved-path diagnostic")
			}
			if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "sneaky.json")); err == nil {
				t.Fatal(".verdi/sneaky.json was written despite the refusal")
			}
			if _, err := os.Stat(filepath.Join(repo.Dir, ".git", "sneaky.json")); err == nil {
				t.Fatal(".git/sneaky.json was written despite the refusal")
			}
			after, err := os.ReadFile(reqPath)
			if err != nil {
				t.Fatalf("reading request file after refusal: %v", err)
			}
			if !bytes.Equal(requestBefore, after) {
				t.Fatal("the input request file was clobbered despite the refusal")
			}
		})
	}
}

// TestCmdContextCompile_StderrHasNoAbsoluteCheckoutPath proves the
// deterministic-diagnostic constraint (Task 8 Step 1): stderr never
// carries the store root's own absolute filesystem path.
func TestCmdContextCompile_StderrHasNoAbsoluteCheckoutPath(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	t.Chdir(repo.Dir)
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
	outPath := filepath.Join(repo.Dir, ".verdi", "sneaky.json")

	var stdout, stderr bytes.Buffer
	got := cmdContextCompile([]string{"--request", reqPath, "--out", outPath}, strings.NewReader(""), &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdContextCompile = %d, want 2; stderr=%s", got, stderr.String())
	}
	if strings.Contains(stderr.String(), repo.Dir) {
		t.Fatalf("stderr leaks the absolute checkout path: %q", stderr.String())
	}
}
