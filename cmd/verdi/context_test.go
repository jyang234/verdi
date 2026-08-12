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
	"os/exec"
	"path/filepath"
	"slices"
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
// Task 8 Step 1 names — missing --request, an empty --request/--out value,
// duplicate --request/--out, unknown flag, extra positional arguments, and
// --request equal to --out — all exit 2, all from a rootless tempdir
// (never touching a store root), and all leave stdout empty.
//
// wantStderr, where given, pins the exact flag-shape diagnostic: from a
// rootless tempdir every later failure ALSO exits 2 (store-root
// resolution fails there), so exit code alone cannot witness that a shape
// was rejected during parsing rather than deep in the run.
func TestCmdContextCompile_FlagShapeFailures(t *testing.T) {
	t.Chdir(t.TempDir())

	cases := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "missing --request"},
		{name: "missing --request value", args: []string{"--request"}},
		{name: "duplicate --request", args: []string{"--request", "a.json", "--request", "b.json"}},
		{name: "duplicate --request via =", args: []string{"--request=a.json", "--request=b.json"}},
		{name: "duplicate --out", args: []string{"--request", "a.json", "--out", "x.json", "--out", "y.json"}},
		{name: "missing --out value", args: []string{"--request", "a.json", "--out"}},
		{name: "unknown flag", args: []string{"--request", "a.json", "--bogus"}},
		{name: "extra positional argument", args: []string{"--request", "a.json", "extra"}},
		{name: "empty --request value via =", args: []string{"--request="}, wantStderr: "--request is required"},
		{name: "empty --out value", args: []string{"--request", "a.json", "--out", ""}, wantStderr: "--out requires a value"},
		{name: "empty --out value via =", args: []string{"--request", "a.json", "--out="}, wantStderr: "--out requires a value"},
		{name: "request equals out", args: []string{"--request", "a.json", "--out", "a.json"}},
		{name: "request equals out, different spelling", args: []string{"--request", "./a.json", "--out", "a.json"}},
		{name: "--out with an interior .. element", args: []string{"--request", "a.json", "--out", "sub/../x.json"}, wantStderr: contextOutDotDotDiagnostic},
		{name: "--out with a leading .. element", args: []string{"--request", "a.json", "--out", "../x.json"}, wantStderr: contextOutDotDotDiagnostic},
		{name: "--out that is exactly ..", args: []string{"--request", "a.json", "--out", ".."}, wantStderr: contextOutDotDotDiagnostic},
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
			if tc.wantStderr != "" && !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Fatalf("stderr = %q, want it to contain the flag-shape diagnostic %q", stderr.String(), tc.wantStderr)
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
	// Task 8 Step 1's "exit 0 for completed manifests carrying advisory/
	// unproven facts": a clean compile is not a claim of proven evidence
	// or a resolved principal. The CLI supplies no principal-resolution
	// port in v1 (authority design §2), so the actor posture is always
	// explicitly unproven and says so in its own disclosures, and v1's one
	// legal evidence authority is advisory.
	if manifest.Evidence.Authority != contextcompile.EvidenceAuthorityAdvisory {
		t.Fatalf("evidence.authority = %q, want %q", manifest.Evidence.Authority, contextcompile.EvidenceAuthorityAdvisory)
	}
	if manifest.Actors.Posture != contextcompile.ResolutionUnproven {
		t.Fatalf("actors.posture = %q, want %q", manifest.Actors.Posture, contextcompile.ResolutionUnproven)
	}
	if len(manifest.Actors.Resolutions) != 0 {
		t.Fatalf("actors.resolutions = %v, want none from a CLI compile", manifest.Actors.Resolutions)
	}
	wantDisclosure := contextcompile.DisclosureActorResolutionUnproven
	if !slices.Contains(manifest.Actors.Disclosures, wantDisclosure) {
		t.Fatalf("actors.disclosures = %v, want to contain %q", manifest.Actors.Disclosures, wantDisclosure)
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

// contextTwoAdapterPolicyStoreFiles reads the real two-adapter policy
// fixture internal/instructionprojection's own multiadapter tests install
// (testdata/multi-store/.verdi/policy), keyed by repo-relative
// .verdi/policy/ path: codex manages AGENTS.md, claude-code manages
// CLAUDE.md — never a hand-authored duplicate of a governed constitution.
func contextTwoAdapterPolicyStoreFiles(t *testing.T) map[string]string {
	t.Helper()
	rels := []string{
		"constitution.md",
		"policies/go-toolchain.md",
		"profiles/solo-default.md",
	}
	out := make(map[string]string, len(rels))
	for _, rel := range rels {
		data, err := os.ReadFile(filepath.Join("..", "..", "internal", "instructionprojection", "testdata", "multi-store", ".verdi", "policy", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read two-adapter policy fixture %s: %v", rel, err)
		}
		out[".verdi/policy/"+rel] = string(data)
	}
	return out
}

// buildContextCompileRepoTwoAdapter mirrors buildContextCompileRepo but
// installs the two-adapter constitution (codex/AGENTS.md, claude-code/
// CLAUDE.md) in place of the single-adapter fixture, so a test can prove
// --out is refused when it names the OTHER (non-requested) adapter's
// managed file.
func buildContextCompileRepoTwoAdapter(t *testing.T, specFiles map[string]string) *fixturegit.Repo {
	t.Helper()
	files := contextTwoAdapterPolicyStoreFiles(t)
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

// TestCmdContextCompile_OutIsOtherAdapterManagedFile_Refused proves the
// --out guard covers EVERY adapter's managed projection path, not only
// the REQUESTED adapter's own: with a two-adapter constitution (codex
// manages AGENTS.md, claude-code manages CLAUDE.md), a codex-adapter
// request naming --out CLAUDE.md must be refused exactly like naming its
// own AGENTS.md would (TestCmdContextCompile_OutIsManagedProjectionFile_
// Refused above) — CLAUDE.md is exactly as reserved, and clobbering it
// would violate CLAUDE.md/the plan's "never writes ... managed projection
// files" regardless of which adapter this particular request targeted.
// The case-variant subtest pins the same alias-safety canonicalGuardPath
// already gives every other reserved-path guard.
func TestCmdContextCompile_OutIsOtherAdapterManagedFile_Refused(t *testing.T) {
	repo := buildContextCompileRepoTwoAdapter(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	t.Chdir(repo.Dir)
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
	caseInsensitive := contextFilesystemIsCaseInsensitive(t, repo.Dir)

	cases := map[string]string{
		"other adapter's exact managed path": filepath.Join(repo.Dir, "CLAUDE.md"),
	}
	if caseInsensitive {
		cases["other adapter's case-variant spelling"] = filepath.Join(repo.Dir, "claude.md")
	}
	for name, outPath := range cases {
		t.Run(name, func(t *testing.T) {
			before, err := os.ReadFile(filepath.Join(repo.Dir, "CLAUDE.md"))
			if err != nil {
				t.Fatalf("reading fixture CLAUDE.md: %v", err)
			}

			var stdout, stderr bytes.Buffer
			got := cmdContextCompile([]string{"--request", reqPath, "--out", outPath}, strings.NewReader(""), &stdout, &stderr)
			if got != 2 {
				t.Fatalf("cmdContextCompile(--out %s, request adapter codex) = %d, want 2; stderr=%s", outPath, got, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			after, err := os.ReadFile(filepath.Join(repo.Dir, "CLAUDE.md"))
			if err != nil {
				t.Fatalf("reading CLAUDE.md after refusal: %v", err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("CLAUDE.md content changed despite the refusal — another adapter's managed projection file must never be overwritten")
			}
		})
	}
}

// contextFailingWriter is a stdout stand-in whose every Write fails, the
// in-process equivalent of a closed pipe or a full disk.
type contextFailingWriter struct{}

func (contextFailingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("stdout unavailable")
}

// TestCmdContextCompile_StdoutWriteFailure_ExitTwo proves Task 8 Step 2's
// "a stdout write failure is operational": a manifest that compiled
// cleanly but could not be delivered exits 2, never 0 (a caller reading a
// truncated stream must not see success) and never 1 (nothing was
// refused).
func TestCmdContextCompile_StdoutWriteFailure_ExitTwo(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	t.Chdir(repo.Dir)
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))

	var stderr bytes.Buffer
	got := cmdContextCompile([]string{"--request", reqPath}, strings.NewReader(""), contextFailingWriter{}, &stderr)
	if got != 2 {
		t.Fatalf("cmdContextCompile(failing stdout) = %d, want 2; stderr=%s", got, stderr.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr is empty, want an operational diagnostic")
	}
}

// TestCmdContextCompile_OutWriteFailure_NoPartialFile proves Task 8 Step
// 2's "no partial file may remain after an output failure": --out naming
// an existing DIRECTORY makes atomicfile.Write's final rename fail, which
// must exit 2 and leave no partial manifest and no temp-file debris in the
// destination's parent — the whole point of routing the write through the
// atomic seam.
func TestCmdContextCompile_OutWriteFailure_NoPartialFile(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	t.Chdir(repo.Dir)
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
	outParent := t.TempDir()
	outPath := filepath.Join(outParent, "manifest.json")
	if err := os.Mkdir(outPath, 0o755); err != nil {
		t.Fatalf("creating the directory that blocks --out: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdContextCompile([]string{"--request", reqPath, "--out", outPath}, strings.NewReader(""), &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdContextCompile(--out <existing dir>) = %d, want 2; stderr=%s", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when --out is present", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr is empty, want an operational diagnostic")
	}

	entries, err := os.ReadDir(outParent)
	if err != nil {
		t.Fatalf("ReadDir(outParent): %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "manifest.json" || !entries[0].IsDir() {
		t.Fatalf("outParent entries = %v, want exactly the pre-existing manifest.json directory (no partial file, no temp debris)", entries)
	}
	inner, err := os.ReadDir(outPath)
	if err != nil {
		t.Fatalf("ReadDir(outPath): %v", err)
	}
	if len(inner) != 0 {
		t.Fatalf("the blocking directory gained entries %v, want none", inner)
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

// contextOutDotDotDiagnostic is the exact deterministic diagnostic the
// belt-and-braces `..` rejection emits. It is asserted from both the
// flag-shape table and the symlink-traversal test below, so the two can
// never drift apart.
const contextOutDotDotDiagnostic = `--out must not contain a ".." path element`

// TestCmdContextCompile_OutTraversesSymlinkedParentWithDotDot_Refused is the
// single-string-discipline witness: `filepath.Clean` collapses a `..`
// element LEXICALLY, before any symlink is resolved, but the kernel
// resolves the symlink FIRST and only then applies `..`. So a spelling like
// `a/../sneaky.json`, where `a` is a symlink into `.verdi/`, canonicalizes
// (for the guard) to a harmless destination while the write it authorizes
// actually lands inside the store — exactly the write authority design §11
// and the Wave-3 plan's "never writes .verdi/, managed projection files,
// payload files, Git state, or a worktree" constraint forbid.
//
// Both spellings below must be refused (exit 2) with nothing written
// anywhere: the first would land in `.verdi/`, the second would clobber the
// committed managed projection `AGENTS.md`.
func TestCmdContextCompile_OutTraversesSymlinkedParentWithDotDot_Refused(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	t.Chdir(repo.Dir)
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))

	if err := os.MkdirAll(filepath.Join(repo.Dir, ".verdi", "sub"), 0o755); err != nil {
		t.Fatalf("creating .verdi/sub: %v", err)
	}
	if err := os.Symlink(filepath.Join(repo.Dir, ".verdi", "sub"), filepath.Join(repo.Dir, "a")); err != nil {
		t.Fatalf("creating a -> .verdi/sub symlink: %v", err)
	}

	projectionBefore, err := os.ReadFile(filepath.Join(repo.Dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading the managed projection: %v", err)
	}

	// Spelled with FromSlash rather than filepath.Join: Join CLEANS its
	// result, which would destroy the very `..` element under test.
	cases := []struct {
		name string
		out  string
		// lands names the file the unguarded write would actually create
		// or overwrite, resolved through the kernel's symlink-first rules.
		lands string
	}{
		{"dot-dot after a symlink into the store zone", filepath.FromSlash("a/../sneaky.json"), filepath.Join(repo.Dir, ".verdi", "sneaky.json")},
		{"dot-dot after a symlink onto a managed projection", filepath.FromSlash("a/../../AGENTS.md"), filepath.Join(repo.Dir, "AGENTS.md")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := cmdContextCompile([]string{"--request", reqPath, "--out", tc.out}, strings.NewReader(""), &stdout, &stderr)
			if got != 2 {
				t.Fatalf("cmdContextCompile(--out %s) = %d, want 2; stderr=%s", tc.out, got, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), contextOutDotDotDiagnostic) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), contextOutDotDotDiagnostic)
			}
			if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "sneaky.json")); err == nil {
				t.Fatal(".verdi/sneaky.json was written despite the refusal")
			}
			if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "sub", "sneaky.json")); err == nil {
				t.Fatal(".verdi/sub/sneaky.json was written despite the refusal")
			}
			if _, err := os.Stat(filepath.Join(repo.Dir, "sneaky.json")); err == nil {
				t.Fatal("sneaky.json was written despite the refusal")
			}
			after, err := os.ReadFile(filepath.Join(repo.Dir, "AGENTS.md"))
			if err != nil {
				t.Fatalf("reading the managed projection after the refusal: %v", err)
			}
			if !bytes.Equal(projectionBefore, after) {
				t.Fatalf("the managed projection %s was rewritten despite the refusal", tc.lands)
			}
		})
	}
}

// contextRunGit runs one git command in dir, failing the test on a
// non-zero exit (this package has no shared runner; gate_test.go/
// feature_test.go each carry their own single-purpose one).
func contextRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// TestHasDotDotElement covers the belt-and-braces `--out` traversal ban
// element-wise in both directions: every spelling that carries a real ".."
// path element, and every honest name that merely CONTAINS two dots and
// must stay allowed.
func TestHasDotDotElement(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"..", true},
		{"../x.json", true},
		{"a/../x.json", true},
		{"a/b/..", true},
		{"./a/../b/x.json", true},
		{filepath.Join("a", "b") + string(filepath.Separator) + ".." + string(filepath.Separator) + "x.json", true},

		{"", false},
		{"x.json", false},
		{"./x.json", false},
		{"a/b/x.json", false},
		{"..notes.json", false},
		{"notes...json", false},
		{"a..b/x.json", false},
		{"...", false},
		{"/abs/path/x.json", false},
	} {
		t.Run(tc.in, func(t *testing.T) {
			if got := hasDotDotElement(tc.in); got != tc.want {
				t.Fatalf("hasDotDotElement(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestRedactCheckoutRoot covers the stderr sanitizer's happy path (every
// absolute spelling of the root replaced by the fixed token), its
// substring-overlap hazard (a resolved root that CONTAINS the unresolved
// one), its determinism, and the negative cases where it must change
// nothing — an empty root, a root that is "/", and a message that never
// mentions the checkout at all.
func TestRedactCheckoutRoot(t *testing.T) {
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("EvalSymlinks(root): %v", err)
	}

	t.Run("redacts the root as given", func(t *testing.T) {
		got := redactCheckoutRoot(root, "policyauthority: reading constitution.md: open "+filepath.Join(root, ".verdi", "policy", "constitution.md")+": permission denied")
		if strings.Contains(got, root) {
			t.Fatalf("redactCheckoutRoot left the root in %q", got)
		}
		want := "policyauthority: reading constitution.md: open " + contextCheckoutToken + filepath.Join(string(filepath.Separator), ".verdi", "policy", "constitution.md") + ": permission denied"
		if got != want {
			t.Fatalf("redactCheckoutRoot = %q, want %q", got, want)
		}
	})

	t.Run("redacts the symlink-resolved spelling too", func(t *testing.T) {
		got := redactCheckoutRoot(root, "gitx: git ls-tree -rz --full-tree HEAD (dir "+resolved+"): exit status 128")
		for _, spelling := range []string{root, resolved} {
			if strings.Contains(got, spelling) {
				t.Fatalf("redactCheckoutRoot left the spelling %q in %q", spelling, got)
			}
		}
	})

	t.Run("overlapping spellings redact cleanly", func(t *testing.T) {
		// The hazard is real only where one variant contains the other as
		// a substring; on macOS /var/... vs /private/var/... is exactly
		// that shape. Where the filesystem gives one spelling this is a
		// no-op assertion, which is still correct.
		got := redactCheckoutRoot(root, "dir "+resolved)
		if got != "dir "+contextCheckoutToken {
			t.Fatalf("redactCheckoutRoot = %q, want a single clean token", got)
		}
	})

	t.Run("is deterministic", func(t *testing.T) {
		msg := "open " + filepath.Join(resolved, "x") + " and " + filepath.Join(root, "y")
		first := redactCheckoutRoot(root, msg)
		for i := 0; i < 5; i++ {
			if again := redactCheckoutRoot(root, msg); again != first {
				t.Fatalf("redactCheckoutRoot is not deterministic: %q vs %q", first, again)
			}
		}
	})

	for _, tc := range []struct {
		name, root, msg string
	}{
		{"empty root changes nothing", "", "open /etc/passwd: permission denied"},
		{"filesystem root is never redacted", string(filepath.Separator), "open /etc/passwd: permission denied"},
		{"unmentioned root changes nothing", root, "contextcompile: stage 1 validate request: phase outside scope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactCheckoutRoot(tc.root, tc.msg); got != tc.msg {
				t.Fatalf("redactCheckoutRoot = %q, want it unchanged (%q)", got, tc.msg)
			}
		})
	}
}

// contextGitOutput runs one git command in dir and returns its stdout,
// failing the test on a non-zero exit.
func contextGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

// contextStderrSecrets are the sentinel strings the hygiene assertion
// requires never to appear in a diagnostic: the drifted projection file's
// own bytes, an uncommitted worktree file's bytes, and the spec fixture's
// body prose (payload content that only a leaking diagnostic could echo).
var contextStderrSecrets = []string{
	"SENTINEL-DRIFTED-PROJECTION-BYTES",
	"SENTINEL-UNCOMMITTED-WORKTREE-BYTES",
	"Body prose must not enter the fragment.",
}

// contextStderrRemoteURL is the sentinel remote the hygiene fixtures
// configure, so "no raw remote URL" is an assertion with something real to
// find rather than a vacuous one.
const contextStderrRemoteURL = "https://verdi-hygiene.invalid/secret-origin.git"

// assertContextStderrHygienic enforces Task 8 Step 1's deterministic-
// diagnostic constraint on one stderr text: no absolute checkout path, no
// raw remote URL (nor any URL at all), and no payload or uncommitted
// content. A caller-supplied absolute --request path echoed back is
// permitted and is deliberately NOT asserted away — the constraint is
// about what the command discloses from the checkout, not about repeating
// what the caller typed.
func assertContextStderrHygienic(t *testing.T, stderr, root string) {
	t.Helper()
	if stderr == "" {
		t.Fatal("stderr is empty, want a diagnostic to inspect")
	}
	// BOTH spellings of the checkout must be absent. On macOS a t.TempDir()
	// root is reached as /var/... while os.Getwd — and therefore
	// store.FindRoot, and therefore every `dir` gitx execs with — reports
	// /private/var/..., so asserting only the spelling the test happens to
	// hold would miss the leak entirely.
	roots := []string{root}
	if resolved, err := filepath.EvalSymlinks(root); err == nil && resolved != root {
		roots = append(roots, resolved)
	}
	for _, r := range roots {
		if strings.Contains(stderr, r) {
			t.Fatalf("stderr leaks the absolute checkout path %q: %q", r, stderr)
		}
	}
	if strings.Contains(stderr, contextStderrRemoteURL) || strings.Contains(stderr, "://") {
		t.Fatalf("stderr leaks a raw remote URL: %q", stderr)
	}
	for _, secret := range contextStderrSecrets {
		if strings.Contains(stderr, secret) {
			t.Fatalf("stderr leaks payload or uncommitted content %q: %q", secret, stderr)
		}
	}
}

// buildContextHygieneRepo builds the shared hygiene fixture: the standard
// compile repo plus a sentinel origin remote and a sentinel uncommitted
// worktree file, so a leaking diagnostic has something recognizable to
// leak.
func buildContextHygieneRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	contextRunGit(t, repo.Dir, "remote", "add", "origin", contextStderrRemoteURL)
	if err := os.WriteFile(filepath.Join(repo.Dir, "uncommitted-notes.txt"), []byte("SENTINEL-UNCOMMITTED-WORKTREE-BYTES\n"), 0o644); err != nil {
		t.Fatalf("writing uncommitted sentinel: %v", err)
	}
	return repo
}

// TestCmdContextCompile_StderrHygiene proves the deterministic-diagnostic
// constraint (Task 8 Step 1) across the command's whole diagnostic surface
// as a TABLE, not as a handful of hand-picked failures that happen to
// carry fixed, path-free strings. Every case below is driven through the
// one assertion, so a newly reachable diagnostic is covered by adding a row
// rather than by remembering to re-assert.
//
// Two rows exist specifically because the shared error seams underneath
// DO carry absolute paths and must be sanitized at this boundary rather
// than upstream (internal/gitx and internal/policyauthority have other
// consumers whose diagnostics are theirs to define):
//
//   - internal/gitx formats every failure as "gitx: git %s (dir %s): %w: %s",
//     whose `dir` is the absolute checkout;
//   - internal/policyauthority/store.go wraps *os.PathError from os.ReadFile,
//     whose Path is the absolute artifact path.
func TestCmdContextCompile_StderrHygiene(t *testing.T) {
	cases := []struct {
		name string
		// setup prepares the hygiene repo (already the cwd) and returns
		// the argv to run.
		setup func(t *testing.T, repo *fixturegit.Repo) []string
		// wantExitTwo pins exit 2 where the case is specifically an
		// operational failure; zero means "any nonzero exit".
		wantExitTwo bool
	}{
		{
			name: "reserved --out refusal",
			setup: func(t *testing.T, repo *fixturegit.Repo) []string {
				reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
				return []string{"--request", reqPath, "--out", filepath.Join(repo.Dir, ".verdi", "sneaky.json")}
			},
			wantExitTwo: true,
		},
		{
			name: "request read failure",
			setup: func(t *testing.T, repo *fixturegit.Repo) []string {
				// The missing request lives OUTSIDE the checkout, so the
				// permitted echo of the caller's own --request path can
				// never account for a checkout path in the diagnostic.
				return []string{"--request", filepath.Join(t.TempDir(), "does-not-exist.json")}
			},
			wantExitTwo: true,
		},
		{
			name: "compile-stage failure: unresolvable spec target",
			setup: func(t *testing.T, repo *fixturegit.Repo) []string {
				reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-nonexistent", contextcompile.PhaseDesign, nil))
				return []string{"--request", reqPath}
			},
		},
		{
			name: "compile-stage failure: drifted managed projection",
			setup: func(t *testing.T, repo *fixturegit.Repo) []string {
				if err := os.WriteFile(filepath.Join(repo.Dir, "AGENTS.md"), []byte("SENTINEL-DRIFTED-PROJECTION-BYTES\n"), 0o644); err != nil {
					t.Fatalf("drifting AGENTS.md: %v", err)
				}
				reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
				return []string{"--request", reqPath}
			},
		},
		{
			name: "operational failure: unreadable policy artifact",
			setup: func(t *testing.T, repo *fixturegit.Repo) []string {
				artifact := filepath.Join(repo.Dir, ".verdi", "policy", "constitution.md")
				if err := os.Chmod(artifact, 0o000); err != nil {
					t.Fatalf("chmod constitution: %v", err)
				}
				t.Cleanup(func() { _ = os.Chmod(artifact, 0o644) })
				if _, err := os.ReadFile(artifact); err == nil {
					t.Skip("chmod 0000 did not make the artifact unreadable (running as root?)")
				}
				reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
				return []string{"--request", reqPath}
			},
			wantExitTwo: true,
		},
		{
			name: "operational failure: unreadable git object store",
			setup: func(t *testing.T, repo *fixturegit.Repo) []string {
				// Point the checked-out branch at a well-formed but
				// absent object id: `git rev-parse` still answers, and
				// the first plumbing call that must actually READ the
				// object fails — a genuine gitx failure whose message
				// carries the absolute `dir`.
				branch := strings.TrimSpace(contextGitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD"))
				const absent = "0123456789abcdef0123456789abcdef01234567"
				if err := os.WriteFile(filepath.Join(repo.Dir, ".git", "refs", "heads", branch), []byte(absent+"\n"), 0o644); err != nil {
					t.Fatalf("pointing %s at an absent object: %v", branch, err)
				}
				reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
				return []string{"--request", reqPath}
			},
			wantExitTwo: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildContextHygieneRepo(t)
			t.Chdir(repo.Dir)
			args := tc.setup(t, repo)

			var stdout, stderr bytes.Buffer
			got := cmdContextCompile(args, strings.NewReader(""), &stdout, &stderr)
			if tc.wantExitTwo && got != 2 {
				t.Fatalf("cmdContextCompile = %d, want 2; stderr=%s", got, stderr.String())
			}
			if got == 0 {
				t.Fatalf("cmdContextCompile = 0, want a failure; stdout=%s stderr=%s", stdout.String(), stderr.String())
			}
			assertContextStderrHygienic(t, stderr.String(), repo.Dir)
		})
	}
}
