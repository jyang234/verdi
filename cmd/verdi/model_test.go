// Real, built-binary end-to-end tests for `verdi model check`
// (obligation/model-schema--ac-3--behavioral): mirrors close_test.go's
// own style — driving the actual compiled binary, never a package-
// internal unit test standing in for it — over a plain, non-git store
// root (model check touches no git state at all, matching disposition_
// test.go's writeDispositionStoreRoot precedent).
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/model"
)

// writeModelCheckStoreRoot builds a plain store root: verdi.yaml always,
// model.yaml only when modelYAML != "".
func writeModelCheckStoreRoot(t *testing.T, modelYAML string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\n"))
	if modelYAML != "" {
		writeTestFile(t, filepath.Join(root, ".verdi", "model.yaml"), []byte(modelYAML))
	}
	return root
}

// runModelCheckBinary execs the built verdi binary's "model check" verb
// with cwd=dir, capturing stdout/stderr separately — mirroring
// runDispositionBinary's exact pattern (disposition_test.go).
func runModelCheckBinary(t *testing.T, bin, dir string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, "model", "check")
	cmd.Dir = dir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), ee.ExitCode()
		}
		t.Fatalf("running verdi model check: %v", err)
	}
	return outBuf.String(), errBuf.String(), 0
}

// TestModelCheck_NoModelYAML_OK is ac-3's absent-file case: no
// .verdi/model.yaml at all resolves to the embedded canonical default,
// exit 0, with an OK line naming the schema, canonical's own class/
// transition counts, and canonical's own digest.
func TestModelCheck_NoModelYAML_OK(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, "")

	stdout, stderr, code := runModelCheckBinary(t, bin, root)
	if code != 0 {
		t.Fatalf("verdi model check (no model.yaml) exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "model: OK — verdi.model/v1, ") {
		t.Fatalf("stdout = %q, want it to start with the OK line", stdout)
	}
	wantDigest, err := model.Canonical().Digest()
	if err != nil {
		t.Fatalf("model.Canonical().Digest(): %v", err)
	}
	if !strings.Contains(stdout, wantDigest) {
		t.Fatalf("stdout = %q, want it to contain the canonical model's own digest %q", stdout, wantDigest)
	}
	if !strings.Contains(stdout, "2 classes") || !strings.Contains(stdout, "4 transitions") {
		t.Fatalf("stdout = %q, want it to name canonical's 2 classes / 4 transitions", stdout)
	}
}

// TestModelCheck_ValidVocabRename_OK is ac-3's valid-hand-written-
// model.yaml case: a manifest varying only vocabulary and per-class
// template filenames (dc-1's frontier) still exits 0, over ITS OWN
// counts and digest (not canonical's — proving the store's file, not
// the embedded default, was actually read).
func TestModelCheck_ValidVocabRename_OK(t *testing.T) {
	bin := buildVerdiBinary(t)
	vocabRenameYAML := readModelTestdata(t, "vocab-rename.yaml")
	root := writeModelCheckStoreRoot(t, vocabRenameYAML)

	stdout, stderr, code := runModelCheckBinary(t, bin, root)
	if code != 0 {
		t.Fatalf("verdi model check (vocab-rename) exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "model: OK — verdi.model/v1, ") {
		t.Fatalf("stdout = %q, want it to start with the OK line", stdout)
	}

	decoded, err := model.DecodeModel([]byte(vocabRenameYAML))
	if err != nil {
		t.Fatalf("test setup: decoding vocab-rename.yaml: %v", err)
	}
	wantDigest, err := decoded.Digest()
	if err != nil {
		t.Fatalf("test setup: computing vocab-rename.yaml's digest: %v", err)
	}
	if !strings.Contains(stdout, wantDigest) {
		t.Fatalf("stdout = %q, want it to contain vocab-rename.yaml's OWN digest %q (proving the store's file was read, not the embedded default)", stdout, wantDigest)
	}
}

// TestModelCheck_FrontierViolation_Exit1_PinnedText is ac-3's
// structurally-deviant case: exit 1, with the ONE pinned frontier error
// text printed VERBATIM — never a paraphrase.
func TestModelCheck_FrontierViolation_Exit1_PinnedText(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, readModelTestdata(t, "viol-frontier-structural.yaml"))

	stdout, stderr, code := runModelCheckBinary(t, bin, root)
	if code != 1 {
		t.Fatalf("verdi model check (frontier violation) exit = %d, want 1\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	const pinned = "structural model configuration is behind the frontier (verdi.model/v1 accepts the canonical model with vocabulary/template changes only)"
	if !strings.Contains(stderr, pinned) {
		t.Fatalf("stderr = %q, want it to contain the pinned frontier text verbatim: %q", stderr, pinned)
	}
}

// TestModelCheck_KernelViolation_Exit2 proves ac-3's own (frozen)
// grouping: a KERNEL VALIDATION rule violation (here, an obligation kind
// outside the closed catalog) is "undecodable" and so exits 2 — NOT
// exit 1, despite this build's plan document's looser "exit 1 on
// validation/frontier failure" prose (a disclosed plan/spec conflict:
// spec+obligation win, per this build's own precedence rule).
func TestModelCheck_KernelViolation_Exit2(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, readModelTestdata(t, "viol-kind-unknown.yaml"))

	stdout, stderr, code := runModelCheckBinary(t, bin, root)
	if code != 2 {
		t.Fatalf("verdi model check (kernel rule violation) exit = %d, want 2 (ac-3's own text: an undecodable manifest is operational trouble)\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "obligation kind") {
		t.Fatalf("stderr = %q, want it to surface the kernel rule's own error", stderr)
	}
}

// TestModelCheck_StoreLessCwd_Exit2 proves a missing store is
// operational trouble (ac-3's own text names this explicitly).
func TestModelCheck_StoreLessCwd_Exit2(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir() // no .verdi/ anywhere under this tree

	stdout, stderr, code := runModelCheckBinary(t, bin, dir)
	if code != 2 {
		t.Fatalf("verdi model check (no store) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stderr == "" {
		t.Fatal("stderr is empty, want a store-not-found error")
	}
}

// TestModelCheck_UnknownSubcommand_Exit2Usage proves an unrecognized
// `model` subcommand is a usage error (exit 2), never a silent no-op.
func TestModelCheck_UnknownSubcommand_Exit2Usage(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, "")

	cmd := exec.Command(bin, "model", "bogus")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("running verdi model bogus: %v", err)
	}
	if ee.ExitCode() != 2 {
		t.Fatalf("verdi model bogus exit = %d, want 2\nstdout: %s\nstderr: %s", ee.ExitCode(), stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "usage: verdi model check") {
		t.Fatalf("stderr = %q, want it to mention 'usage: verdi model check'", stderr.String())
	}
}

// TestModelCheck_BareVerb_Exit2Usage proves `verdi model` with no
// subcommand at all is the same usage error, not a crash or a silent
// default.
func TestModelCheck_BareVerb_Exit2Usage(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, "")

	cmd := exec.Command(bin, "model")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("running bare verdi model: %v", err)
	}
	if ee.ExitCode() != 2 {
		t.Fatalf("bare verdi model exit = %d, want 2\nstdout: %s\nstderr: %s", ee.ExitCode(), stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "usage: verdi model check") {
		t.Fatalf("stderr = %q, want it to mention 'usage: verdi model check'", stderr.String())
	}
}

// readModelTestdata reads a fixture from internal/model/testdata — this
// package's own tests reuse Task 5's committed fixtures rather than
// duplicating their content (CLAUDE.md: never copy-paste shared content).
func readModelTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "model", "testdata", name))
	if err != nil {
		t.Fatalf("reading internal/model/testdata/%s: %v", name, err)
	}
	return string(data)
}
