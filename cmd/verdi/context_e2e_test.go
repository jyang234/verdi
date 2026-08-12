// Built-binary end-to-end tests for `verdi context compile` (Wave-3 plan
// Task 8 Step 1): the real compiled verdi binary as a real OS subprocess
// against a real, local fixturegit repository — never a package-internal
// call standing in for it (obligationseam_e2e_test.go's own
// buildVerdiBinary/runVerdiBinary-driven style). context_test.go owns the
// faster in-process parser/flag-shape proofs; this file owns the ones that
// genuinely need a real subprocess: stdin piping through a real os.Stdin,
// and the store/worktree-unchanged invariant, which only a real external
// process (rather than an in-process call sharing this test binary's own
// working tree) proves without ambiguity.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// contextE2EPorcelainStatus returns `git status --porcelain` output for
// dir, failing the test on a non-zero exit (mirrors
// journeyPorcelainStatus/porcelainStatus's own established idiom).
func contextE2EPorcelainStatus(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status --porcelain: %v", err)
	}
	return string(out)
}

// contextE2ECurrentHead returns dir's current HEAD sha.
func contextE2ECurrentHead(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestContextCompileE2E_StdoutHappyPath proves the real binary's default
// (no --out) path: exit 0, a decodable canonical manifest on stdout, empty
// stderr, and — the store/worktree-unchanged invariant — neither HEAD nor
// `git status --porcelain` changes.
func TestContextCompileE2E_StdoutHappyPath(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))

	headBefore := contextE2ECurrentHead(t, repo.Dir)
	statusBefore := contextE2EPorcelainStatus(t, repo.Dir)

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "context", "compile", "--request", reqPath)
	if code != 0 {
		t.Fatalf("verdi context compile exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty on success", stderr)
	}
	manifest, err := contextcompile.DecodeManifest([]byte(stdout))
	if err != nil {
		t.Fatalf("DecodeManifest(stdout): %v\nstdout=%s", err, stdout)
	}
	if manifest.Phase != contextcompile.PhaseDesign {
		t.Fatalf("Phase = %q, want design", manifest.Phase)
	}

	if got := contextE2ECurrentHead(t, repo.Dir); got != headBefore {
		t.Fatalf("HEAD changed: before=%s after=%s", headBefore, got)
	}
	// The request file itself is untracked (written after the fixture's
	// own commits), so it is EXPECTED to appear in status both before and
	// after — the assertion is that status is otherwise byte-identical
	// (no NEW untracked/modified paths from the compile itself).
	if got := contextE2EPorcelainStatus(t, repo.Dir); got != statusBefore {
		t.Fatalf("git status --porcelain changed: before=%q after=%q", statusBefore, got)
	}
}

// TestContextCompileE2E_OutFile_LeavesOnlyThatFile proves the --out path
// through the real binary: exit 0, completely empty stdout/stderr, the
// exact requested file written with decodable manifest bytes, and —
// Step 1's "leaves the store and Git worktree unchanged except for the
// explicit output file" — `git status --porcelain` names exactly that one
// new path and nothing else (no data-item or projection file anywhere).
func TestContextCompileE2E_OutFile_LeavesOnlyThatFile(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))

	statusBefore := contextE2EPorcelainStatus(t, repo.Dir)

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "context", "compile", "--request", reqPath, "--out", "manifest.json")
	if code != 0 {
		t.Fatalf("verdi context compile --out exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want completely empty when --out is present", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty on success", stderr)
	}

	outPath := filepath.Join(repo.Dir, "manifest.json")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading --out file: %v", err)
	}
	if _, err := contextcompile.DecodeManifest(data); err != nil {
		t.Fatalf("DecodeManifest(--out file): %v", err)
	}

	statusAfter := contextE2EPorcelainStatus(t, repo.Dir)
	wantNewLine := "?? manifest.json\n"
	if statusAfter != statusBefore+wantNewLine && statusAfter != wantNewLine+statusBefore {
		t.Fatalf("git status --porcelain = %q, want exactly %q added to the before-state %q (no data item or projection file written anywhere)",
			statusAfter, wantNewLine, statusBefore)
	}
}

// TestContextCompileE2E_StdinRequest proves `--request -` reads from the
// real subprocess's os.Stdin.
func TestContextCompileE2E_StdinRequest(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	data := contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil)

	cmd := exec.Command(bin, "context", "compile", "--request", "-")
	cmd.Dir = repo.Dir
	cmd.Stdin = bytes.NewReader(data)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("running verdi context compile --request -: %v\nstderr: %s", err, errBuf.String())
	}
	if _, err := contextcompile.DecodeManifest(outBuf.Bytes()); err != nil {
		t.Fatalf("DecodeManifest(stdout): %v\nstdout=%s", err, outBuf.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on success", errBuf.String())
	}
}

// TestContextCompileE2E_NoConstitution_ExitOne proves the real binary maps
// *contextcompile.NoConstitutionRefusal to exit 1 against a legacy-shaped
// store fixture that carries no .verdi/policy/ tree at all.
func TestContextCompileE2E_NoConstitution_ExitOne(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                         "schema: verdi.layout/v1\n",
			".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
		},
		Message: "scaffold, no constitution",
	}})
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "context", "compile", "--request", reqPath)
	if code != 1 {
		t.Fatalf("verdi context compile (no constitution) exit = %d, want 1\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty on a refusal", stdout)
	}
	if stderr == "" {
		t.Fatal("stderr is empty, want a diagnostic")
	}
}

// TestContextCompileE2E_MalformedRequest_ExitTwo proves the real binary
// maps a syntactically invalid request to exit 2.
func TestContextCompileE2E_MalformedRequest_ExitTwo(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", []byte("{not json"))

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "context", "compile", "--request", reqPath)
	if code != 2 {
		t.Fatalf("verdi context compile (malformed) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty on malformed input", stdout)
	}
}

// TestContextCompileE2E_OutInVerdiZone_RefusedNoWrite proves the real
// binary refuses a --out destination inside .verdi/ (exit 2) and writes
// nothing at all — the store and worktree are completely unchanged.
func TestContextCompileE2E_OutInVerdiZone_RefusedNoWrite(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
	statusBefore := contextE2EPorcelainStatus(t, repo.Dir)

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "context", "compile", "--request", reqPath, "--out", ".verdi/sneaky.json")
	if code != 2 {
		t.Fatalf("verdi context compile (--out .verdi/sneaky.json) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "sneaky.json")); err == nil {
		t.Fatal(".verdi/sneaky.json was written despite the refusal")
	}
	if got := contextE2EPorcelainStatus(t, repo.Dir); got != statusBefore {
		t.Fatalf("git status --porcelain changed despite the refusal: before=%q after=%q", statusBefore, got)
	}
}

// TestContextCompileE2E_OutAliasesReservedPath_RefusedNoWrite proves the
// real binary refuses the alias spellings of a reserved destination that a
// clean-string guard cannot see — a case-variant `.VERDI/` spelling, a
// symlinked parent whose target IS `.verdi/`, and a case-variant spelling
// of the input request file — each with exit 2, nothing written into
// .verdi/, an unclobbered request file, and an unchanged worktree.
func TestContextCompileE2E_OutAliasesReservedPath_RefusedNoWrite(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	reqPath := writeContextRequestFile(t, repo.Dir, "request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
	requestBefore, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatalf("reading request file: %v", err)
	}
	if err := os.Symlink(filepath.Join(repo.Dir, ".verdi"), filepath.Join(repo.Dir, "notes")); err != nil {
		t.Fatalf("creating notes -> .verdi symlink: %v", err)
	}
	caseInsensitive := contextFilesystemIsCaseInsensitive(t, repo.Dir)
	statusBefore := contextE2EPorcelainStatus(t, repo.Dir)

	cases := []struct {
		name          string
		out           string
		needsCaseFold bool
	}{
		{"case-variant .verdi spelling", ".VERDI/sneaky.json", true},
		{"symlinked parent resolving into .verdi", "notes/sneaky.json", false},
		{"case-variant request-file spelling", "REQUEST.json", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.needsCaseFold && !caseInsensitive {
				t.Skip("filesystem is case-sensitive: a case-variant spelling names a genuinely different path here")
			}
			stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "context", "compile", "--request", "request.json", "--out", tc.out)
			if code != 2 {
				t.Fatalf("verdi context compile (--out %s) exit = %d, want 2\nstdout: %s\nstderr: %s", tc.out, code, stdout, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "sneaky.json")); err == nil {
				t.Fatal(".verdi/sneaky.json was written despite the refusal")
			}
			after, err := os.ReadFile(reqPath)
			if err != nil {
				t.Fatalf("reading request file after refusal: %v", err)
			}
			if !bytes.Equal(requestBefore, after) {
				t.Fatal("the input request file was clobbered despite the refusal")
			}
			if got := contextE2EPorcelainStatus(t, repo.Dir); got != statusBefore {
				t.Fatalf("git status --porcelain changed despite the refusal: before=%q after=%q", statusBefore, got)
			}
		})
	}
}

// TestContextCompileE2E_UnknownSubcommand_ExitTwo proves a bare invocation
// against a real live tree still fails deterministically on usage alone
// (no store root resolved, nothing touched), matching the hermetic
// posture internal/specalign's verb inventory relies on.
func TestContextCompileE2E_UnknownSubcommand_ExitTwo(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	stdout, stderr, code := runVerdiBinary(t, bin, dir, nil, "context")
	if code != 2 {
		t.Fatalf("verdi context (bare) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if stderr != contextCompileUsage+"\n" {
		t.Fatalf("stderr = %q, want exactly %q", stderr, contextCompileUsage+"\n")
	}
}
