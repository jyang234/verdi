// Built-command proofs for `verdi context resolve` (VATC F12 controller-owner
// bridge correction §2.2, implementation plan Task 3 Steps 1 and 4).
//
// internal/contextresolve's own producer owns the replay and closed-union
// exactness proofs. This file owns what only the command can be asked: the
// exact argv, the exclusive frame ceiling, the 0/1/2 exit split, an empty
// stdout on every operational failure, closed diagnostic classes, and — under
// a real subprocess with a hostile HOME, a shimmed PATH, and an inherited
// FD 3 — that a query which legitimately reads the store and Git still writes
// nothing anywhere and launches nothing.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextresolve"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/sealedexec"
	"github.com/jyang234/verdi/internal/store"
)

const contextResolveSpecRef = "spec/feature-alpha"

// contextResolveRepo builds the same real store fixture the compile tests use
// and returns it together with the canonical resolve-request document for
// contextResolveSpecRef.
func contextResolveRepo(t *testing.T) (*fixturegit.Repo, []byte) {
	t.Helper()
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	return repo, contextResolveRequestBytes(t, repo.Dir, contextResolveSpecRef)
}

// contextResolveRequestBytes compiles the fixture once and freezes the exact
// current manifest into a canonical resolve request naming ref.
func contextResolveRequestBytes(t *testing.T, root, ref string) []byte {
	t.Helper()
	compileRequest := contextcompile.Request{
		Schema:  contextcompile.RequestSchema,
		Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Phase:   contextcompile.PhaseDesign,
		Scope: policyartifact.Scope{
			Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{},
		},
		Spec: contextResolveSpecRef,
	}
	compiled, err := contextcompile.NewCompiler().Compile(context.Background(), root, compileRequest)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	// The base manifest is the compiler's own pre-expansion output, and the
	// terminal tuple is stated explicitly rather than inferred from it. With no
	// installed row the two coincide, which is exactly why IL-099 requires both
	// to be present: a request that carried only one of them could not express
	// a post-expansion state at all.
	data, err := contextresolve.EncodeRequest(contextresolve.Request{
		Schema:       contextresolve.RequestSchema,
		Compile:      compileRequest,
		BaseManifest: compiled.Manifest,
		Identity: contextresolve.Identity{
			Flight: "flight-1", Lane: "lane-1", Epoch: "epoch-1", Session: "session-1",
		},
		Expansions: []contextresolve.Expansion{},
		Terminal: contextresolve.Terminal{
			Revision:       uint64(compiled.Manifest.Revisions.Context),
			ManifestDigest: compiled.Manifest.Digest,
			ExpansionRoot:  "",
		},
		Ref: ref,
	})
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	return data
}

// --- in-process command proofs -------------------------------------------

// TestCmdContextResolveGrammar proves §2.2's closed grammar: the ONLY argv
// this verb accepts after `context resolve` is the two-token sequence
// `--request -`.
//
// The `=`-joined spelling is refused explicitly and by name. Every other flag
// in this binary accepts both spellings, so a reader could reasonably expect
// this one to as well — which is the point: §2.2 fixes one argv for a command
// an external controller invokes programmatically, and two spellings of one
// invocation is one more thing a cross-repository pin has to agree about.
//
// Every row runs from a directory that is not a store, so a refusal here is a
// refusal on argument shape alone rather than a store failure standing in for
// one.
func TestCmdContextResolveGrammar(t *testing.T) {
	t.Chdir(t.TempDir())

	cases := [][]string{
		{"resolve"},
		{"resolve", "--request"},
		{"resolve", "--request", ""},
		{"resolve", "--request="},
		{"resolve", "--request=-"},
		{"resolve", "--request", "request.json"},
		{"resolve", "--request=request.json"},
		{"resolve", "--request", "-", "--request", "-"},
		{"resolve", "--request=-", "--request=-"},
		{"resolve", "--request", "-", "--request=-"},
		{"resolve", "--request", "-", "extra"},
		{"resolve", "extra", "--request", "-"},
		{"resolve", "--out", "result.json", "--request", "-"},
		{"resolve", "--ref", "spec/x", "--request", "-"},
		{"resolve", "-"},
		{"resolve", "--request", "-", ""},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := cmdContext(args, strings.NewReader(""), &stdout, &stderr); got != 2 {
				t.Fatalf("cmdContext(%v) = %d, want 2", args, got)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty on a usage error", stdout.String())
			}
			if stderr.String() != contextResolveUsage+"\n" {
				t.Fatalf("stderr = %q, want the resolve usage line %q", stderr.String(), contextResolveUsage+"\n")
			}
		})
	}
}

// TestCmdContextResolveProven proves the exit-0 arm end to end in process:
// one canonical result document on stdout, empty stderr, and an item bound to
// the ref that was asked about.
func TestCmdContextResolveProven(t *testing.T) {
	repo, request := contextResolveRepo(t)
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	args := []string{"resolve", "--request", "-"}
	if got := cmdContext(args, bytes.NewReader(request), &stdout, &stderr); got != 0 {
		t.Fatalf("cmdContext(%v) = %d, want 0\nstderr: %s", args, got, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on success", stderr.String())
	}
	result, err := contextresolve.DecodeResult(stdout.Bytes())
	if err != nil {
		t.Fatalf("DecodeResult(stdout): %v\nstdout=%s", err, stdout.String())
	}
	if result.State != contextresolve.StateProven || result.Item == nil {
		t.Fatalf("state = %q with witnesses %v, want proven with an item", result.State, result.Witnesses)
	}
	if result.Ref != contextResolveSpecRef {
		t.Fatalf("result ref = %q, want %q", result.Ref, contextResolveSpecRef)
	}
}

// TestCmdContextResolveNonProven proves the exit-1 arm: an absent ref is a
// verdict, not an operational failure, and the canonical non-proven document
// is still delivered.
func TestCmdContextResolveNonProven(t *testing.T) {
	repo, _ := contextResolveRepo(t)
	request := contextResolveRequestBytes(t, repo.Dir, "spec/feature-nowhere")
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	args := []string{"resolve", "--request", "-"}
	if got := cmdContext(args, bytes.NewReader(request), &stdout, &stderr); got != 1 {
		t.Fatalf("cmdContext(%v) = %d, want the verdict exit 1\nstderr: %s", args, got, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on a verdict", stderr.String())
	}
	result, err := contextresolve.DecodeResult(stdout.Bytes())
	if err != nil {
		t.Fatalf("DecodeResult(stdout): %v\nstdout=%s", err, stdout.String())
	}
	if result.State != contextresolve.StateNonProven || result.Item != nil {
		t.Fatalf("state = %q with item %v, want non-proven with no item", result.State, result.Item)
	}
	if len(result.Witnesses) == 0 {
		t.Fatal("a non-proven result carried no witness")
	}
}

// TestCmdContextResolveMalformedInput proves every decode refusal is the
// operational 2 with an empty stdout and one closed diagnostic class.
func TestCmdContextResolveMalformedInput(t *testing.T) {
	repo, request := contextResolveRepo(t)
	t.Chdir(repo.Dir)

	cases := map[string]string{
		"empty":             "",
		"not-json":          "nonsense\n",
		"null":              "null\n",
		"wrong-schema":      `{"schema":"verdi.context-resolve-request/v2"}` + "\n",
		"unknown-member":    strings.Replace(string(request), `"ref":`, `"extra":1,"ref":`, 1),
		"trailing-data":     string(request) + "{}\n",
		"noncanonical":      " " + string(request),
		"truncated":         string(request[:len(request)/2]),
		"empty-json-object": "{}\n",
	}
	for name, document := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := []string{"resolve", "--request", "-"}
			if got := cmdContext(args, strings.NewReader(document), &stdout, &stderr); got != 2 {
				t.Fatalf("cmdContext(%v) on %s = %d, want 2", args, name, got)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty on failure", stdout.String())
			}
			if want := "context resolve: " + contextResolveRequestRefused + "\n"; stderr.String() != want {
				t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
			}
		})
	}
}

// TestCmdContextResolveNoStore proves an invocation outside a store is an
// operational refusal that names only the closed class — never the directory
// it failed to resolve.
func TestCmdContextResolveNoStore(t *testing.T) {
	_, request := contextResolveRepo(t)
	// The fixture store exists, but the command runs from a directory that is
	// not inside it — so the refusal is about where the query was asked, never
	// about whether a store exists anywhere.
	t.Chdir(t.TempDir())

	var stdout, stderr bytes.Buffer
	args := []string{"resolve", "--request", "-"}
	if got := cmdContext(args, bytes.NewReader(request), &stdout, &stderr); got != 2 {
		t.Fatalf("cmdContext(%v) outside a store = %d, want 2", args, got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on failure", stdout.String())
	}
	if want := "context resolve: " + contextResolveStoreUnavailable + "\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
	}
}

// TestCmdContextResolveUnresolvable proves the class §2.2 reserves for a
// "compiler ... inability": a request this binary decoded, asked in a
// directory the store resolver accepts, whose fresh recompile the real
// compiler cannot deliver.
//
// The class is a genuine third thing, and each fixture is built to reach it
// past the two refusals that surround it. The grammar and the document are
// already accepted — the request is the same canonical one the proven test
// sends — and the store root IS resolved, asserted below through the very
// call the command makes, so the refusal can be neither request-refused nor
// store-unavailable. What is missing is only what the compile needs: a
// repository behind the store manifest in the first fixture, an adopted
// constitution in the second.
//
// Whether the compiler answers with a typed refusal or an operational error
// is deliberately not an operand here. §2.2 gives this verb one operational
// class for "there is no answer", and a caller who cannot be handed a document
// learns exactly that and nothing about the store that failed to produce one.
func TestCmdContextResolveUnresolvable(t *testing.T) {
	_, request := contextResolveRepo(t)

	cases := map[string]func(t *testing.T) string{
		"a store manifest with no repository behind it": func(t *testing.T) string {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, ".verdi"), 0o755); err != nil {
				t.Fatalf("creating the store directory: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\n"), 0o644); err != nil {
				t.Fatalf("writing the store manifest: %v", err)
			}
			return dir
		},
		"a real store with no adopted constitution": func(t *testing.T) string {
			repo := fixturegit.Build(t, []fixturegit.Layer{{
				Files: map[string]string{
					".verdi/verdi.yaml":                         "schema: verdi.layout/v1\n",
					".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
				},
				Message: "scaffold, no constitution",
			}})
			t.Setenv("CI_DEFAULT_BRANCH", "main")
			return repo.Dir
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			// Built before the chdir: the fixture reads its spec through a
			// path relative to this package's own directory.
			dir := build(t)
			t.Chdir(dir)
			// The command's own precondition, asked exactly as the command asks
			// it. Without this, a fixture that stopped being a store root would
			// still refuse — as store-unavailable — and prove nothing about the
			// compiler.
			if _, err := store.FindRoot("."); err != nil {
				t.Fatalf("the fixture is not a store root: %v", err)
			}

			var stdout, stderr bytes.Buffer
			args := []string{"resolve", "--request", "-"}
			if got := cmdContext(args, bytes.NewReader(request), &stdout, &stderr); got != 2 {
				t.Fatalf("cmdContext(%v) = %d, want 2\nstderr: %s", args, got, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty on failure", stdout.String())
			}
			if want := "context resolve: " + contextResolveUnresolvable + "\n"; stderr.String() != want {
				t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
			}
		})
	}
}

// contextResolveFiller is an endless reader: it never reaches EOF, so a
// limiter that admitted the ceiling would read forever.
type contextResolveFiller struct{}

func (contextResolveFiller) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

// TestCmdContextResolveCeilingIsExclusive proves a document at the 32 MiB
// bound is one operational refusal rather than a truncated read.
func TestCmdContextResolveCeilingIsExclusive(t *testing.T) {
	t.Chdir(t.TempDir())

	var stdout, stderr bytes.Buffer
	args := []string{"resolve", "--request", "-"}
	if got := cmdContext(args, contextResolveFiller{}, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdContext(%v) at the ceiling = %d, want 2", args, got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on failure", stdout.String())
	}
	if want := "context resolve: " + contextResolveInputTooLarge + "\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
	}
	if sealedexec.OwnerFrameCeiling != 32<<20 {
		t.Fatalf("frame ceiling = %d, want the 32 MiB controller bound", sealedexec.OwnerFrameCeiling)
	}
}

type contextResolveFailingReader struct{}

func (contextResolveFailingReader) Read([]byte) (int, error) {
	return 0, errors.New("/private/var/secret-checkout: stdin unavailable")
}

// TestCmdContextResolveStdinReadFailure proves an unreadable stdin is one
// closed class and never echoes the underlying reader's message.
func TestCmdContextResolveStdinReadFailure(t *testing.T) {
	t.Chdir(t.TempDir())

	var stdout, stderr bytes.Buffer
	args := []string{"resolve", "--request", "-"}
	if got := cmdContext(args, contextResolveFailingReader{}, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdContext(%v) with a failing stdin = %d, want 2", args, got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on failure", stdout.String())
	}
	if want := "context resolve: " + contextResolveInputUnreadable + "\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
	}
}

// contextResolveOversizedSpec returns the real feature-alpha fixture with a
// block of body prose at least the frame ceiling long spliced into it.
//
// The padding goes into the body, in the region the fixture itself documents
// as prose that never enters a fragment, so the spec's front matter, anchors
// and sections stay exactly the ones every other test in this file compiles —
// the only thing this store changes is how big the one resolved item is. It
// is wrapped at 80 columns rather than written as a single enormous line, so
// no line-oriented reader anywhere in the compile is asked about a 32 MiB
// token, and it is generated at run time rather than committed because a
// fixture this size belongs in no repository.
func contextResolveOversizedSpec(t *testing.T) string {
	t.Helper()
	spec := contextFeatureAlphaSpec(t)
	const marker = "Body prose must not enter the fragment.\n"
	if n := strings.Count(spec, marker); n != 1 {
		t.Fatalf("the feature-alpha fixture carries %d body-prose markers, want exactly 1", n)
	}
	line := strings.Repeat("x", 79) + "\n"
	pad := strings.Repeat(line, sealedexec.OwnerFrameCeiling/len(line)+1)
	if len(pad) < sealedexec.OwnerFrameCeiling {
		t.Fatalf("padding is %d bytes, want at least the %d-byte ceiling", len(pad), sealedexec.OwnerFrameCeiling)
	}
	return strings.Replace(spec, marker, marker+"\n"+pad, 1)
}

// TestCmdContextResolveOutputTooLarge proves the output ceiling is a real
// refusal over a real store: a resolved item whose canonical result reaches
// the 32 MiB controller frame is refused whole, with an empty stdout, rather
// than written and left for the caller to discover mid-document.
//
// The store is what makes the document large. The item comes from the fresh
// compile and never from stdin, so this is the one bound a caller cannot reach
// by handing the command a big input — and the assertion that the REQUEST is
// under the same ceiling is what keeps the two apart: without it an oversized
// request would refuse as input-too-large and this test would pass while
// proving nothing about the output path.
func TestCmdContextResolveOutputTooLarge(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextResolveOversizedSpec(t),
	})
	request := contextResolveRequestBytes(t, repo.Dir, contextResolveSpecRef)
	if len(request) >= sealedexec.OwnerFrameCeiling {
		t.Fatalf("the request is %d bytes, at or over the input ceiling: this fixture would prove input-too-large", len(request))
	}
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	args := []string{"resolve", "--request", "-"}
	if got := cmdContext(args, bytes.NewReader(request), &stdout, &stderr); got != 2 {
		t.Fatalf("cmdContext(%v) = %d, want 2\nstderr: %s", args, got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %d bytes, want empty on failure", stdout.Len())
	}
	if want := "context resolve: " + contextResolveOutputTooLarge + "\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
	}
}

// contextResolveShortWriter accepts every byte but the last and reports no
// error, the exact contract violation half a canonical document would slip
// through.
type contextResolveShortWriter struct{ n int }

func (w *contextResolveShortWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	w.n += len(p) - 1
	return len(p) - 1, nil
}

type contextResolveFailingWriter struct{}

func (contextResolveFailingWriter) Write([]byte) (int, error) {
	return 0, errors.New("/private/var/secret-checkout: stdout unavailable")
}

// TestCmdContextResolveOutputFailure proves both output failures — a short
// write with a nil error and an outright write error — are the operational 2
// with one closed class.
func TestCmdContextResolveOutputFailure(t *testing.T) {
	repo, request := contextResolveRepo(t)
	t.Chdir(repo.Dir)

	for name, stdout := range map[string]io.Writer{
		"short-write":   &contextResolveShortWriter{},
		"write-failure": contextResolveFailingWriter{},
	} {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer
			args := []string{"resolve", "--request", "-"}
			if got := cmdContext(args, bytes.NewReader(request), stdout, &stderr); got != 2 {
				t.Fatalf("cmdContext(%v) with a %s stdout = %d, want 2", args, name, got)
			}
			if want := "context resolve: " + contextResolveOutputFailed + "\n"; stderr.String() != want {
				t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
			}
		})
	}
}

// --- built-binary proofs -------------------------------------------------

// contextResolveSnapshot renders a directory tree as a deterministic string.
func contextResolveSnapshot(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			lines = append(lines, "dir  "+rel)
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		lines = append(lines, "file "+rel+" "+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return strings.Join(lines, "\n")
}

// TestContextResolveE2EReadOnly runs the real binary as a real OS subprocess
// against a real store and proves §2.2's read-only clause under a hostile
// environment.
//
// The query legitimately reads the store and Git, so — unlike the owner
// bridge — the real git must stay reachable. Everything else a
// profile-activating, credential-reading, or provider-launching path would
// reach for is a marker shim that leaves a file behind if it ever runs; none
// may. HOME names a path that does not exist, and FD 3 is a real pipe the
// child inherits and must leave untouched, so the resolver cannot be
// answering from — or reporting to — the sealed controller channel.
func TestContextResolveE2EReadOnly(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo, request := contextResolveRepo(t)

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	harness := t.TempDir()
	markers := filepath.Join(harness, "markers")
	shims := filepath.Join(harness, "shims")
	for _, dir := range []string{markers, shims} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"verdi", "codex", "claude", "ssh", "curl", "gh"} {
		script := "#!/bin/sh\n: > \"" + filepath.Join(markers, name) + "\"\nexit 0\n"
		if err := os.WriteFile(filepath.Join(shims, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	controllerRead, controllerWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = controllerRead.Close() }()

	treeBefore := contextResolveSnapshot(t, repo.Dir)
	statusBefore := contextE2EPorcelainStatus(t, repo.Dir)
	headBefore := contextE2ECurrentHead(t, repo.Dir)

	cmd := exec.Command(bin, "context", "resolve", "--request", "-")
	cmd.Dir = repo.Dir
	cmd.Env = []string{
		"PATH=" + shims + string(os.PathListSeparator) + filepath.Dir(gitPath),
		"HOME=" + filepath.Join(harness, "no-such-home"),
		"CI_DEFAULT_BRANCH=main",
	}
	cmd.ExtraFiles = []*os.File{controllerWrite} // becomes the child's FD 3
	cmd.Stdin = bytes.NewReader(request)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	// The parent's copy of the write end must go before the read below, or
	// that read waits on an end this process is still holding open.
	_ = controllerWrite.Close()

	if runErr != nil {
		t.Fatalf("verdi context resolve: %v\nstderr: %s", runErr, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on success", stderr.String())
	}
	result, err := contextresolve.DecodeResult(stdout.Bytes())
	if err != nil {
		t.Fatalf("DecodeResult(stdout): %v\nstdout=%s", err, stdout.String())
	}
	if result.State != contextresolve.StateProven || result.Item == nil {
		t.Fatalf("state = %q with witnesses %v, want proven with an item", result.State, result.Witnesses)
	}

	fd3, err := io.ReadAll(controllerRead)
	if err != nil {
		t.Fatalf("reading the inherited FD 3: %v", err)
	}
	if len(fd3) != 0 {
		t.Fatalf("the command wrote %d bytes to FD 3, want none", len(fd3))
	}
	entries, err := os.ReadDir(markers)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("the command executed PATH-discovered %s", strings.Join(names, ", "))
	}

	if after := contextResolveSnapshot(t, repo.Dir); after != treeBefore {
		t.Fatalf("the store changed\nbefore:\n%s\nafter:\n%s", treeBefore, after)
	}
	if after := contextE2EPorcelainStatus(t, repo.Dir); after != statusBefore {
		t.Fatalf("git status changed: before=%q after=%q", statusBefore, after)
	}
	if after := contextE2ECurrentHead(t, repo.Dir); after != headBefore {
		t.Fatalf("HEAD changed: before=%s after=%s", headBefore, after)
	}
}

// TestContextResolveE2EExitCodes proves the built command's own 0/1/2 split
// and that a failure leaves stdout completely empty.
func TestContextResolveE2EExitCodes(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo, proven := contextResolveRepo(t)
	absent := contextResolveRequestBytes(t, repo.Dir, "spec/feature-nowhere")

	const usage = "usage: verdi context resolve --request -\n"
	cases := []struct {
		name     string
		args     []string
		stdin    string
		wantExit int
		wantOut  bool
		wantErr  string
	}{
		{"proven", []string{"context", "resolve", "--request", "-"}, string(proven), 0, true, ""},
		{"non-proven", []string{"context", "resolve", "--request", "-"}, string(absent), 1, true, ""},
		{"malformed", []string{"context", "resolve", "--request", "-"}, `{"schema":"nonsense"}` + "\n", 2, false,
			"context resolve: request-refused\n"},
		{"usage-path-operand", []string{"context", "resolve", "--request", "request.json"}, string(proven), 2, false, usage},
		// The joined spelling is a usage error even though its operand is the
		// stdin sentinel: the accepted argv is two tokens, not one token that
		// happens to mean the same thing.
		{"usage-joined-spelling", []string{"context", "resolve", "--request=-"}, string(proven), 2, false, usage},
		{"usage-extra-token", []string{"context", "resolve", "--request", "-", "extra"}, string(proven), 2, false, usage},
		{"usage-missing-operand", []string{"context", "resolve"}, string(proven), 2, false, usage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args
			extraEnv := []string{"CI_DEFAULT_BRANCH=main"}
			stdout, stderr, code := runVerdiBinaryStdin(t, bin, repo.Dir, extraEnv, tc.stdin, args...)
			if code != tc.wantExit {
				t.Fatalf("exit = %d, want %d\nstdout: %s\nstderr: %s", code, tc.wantExit, stdout, stderr)
			}
			if tc.wantOut != (stdout != "") {
				t.Fatalf("stdout = %q, wantNonEmpty=%v", stdout, tc.wantOut)
			}
			if stderr != tc.wantErr {
				t.Fatalf("stderr = %q, want %q", stderr, tc.wantErr)
			}
		})
	}
}

// runVerdiBinaryStdin runs the built binary with stdin piped, returning its
// streams and exit status. obligationseam_e2e_test.go's runVerdiBinary owns
// the no-stdin form; this verb's whole operand arrives on stdin.
func runVerdiBinaryStdin(t *testing.T, bin, dir string, extraEnv []string, stdin string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("verdi %s: %v\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), stderr.String(), code
}
