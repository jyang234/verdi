// Built-command proofs for `verdi context owner` (VATC F12 controller-owner
// bridge correction §2, implementation plan Task 2 Steps 1, 4 and 5).
//
// The wire bytes below are written literally rather than produced by the code
// under test. internal/sealedexec's mapping producer owns the 22-operation
// exactness proof; this file owns what only the command can be asked: the
// closed grammar, the exclusive frame ceiling, an empty stdout on every
// failure, closed diagnostic classes, and effect freedom under a poisoned
// cwd/HOME/PATH, an inherited FD 3, and a read-only working directory.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/sealedexec"
)

// --- literal fixtures ----------------------------------------------------

// contextOwnerNextStampRequest is the smallest private request payload in the
// registry: an operation whose only operand is its own schema.
const contextOwnerNextStampRequest = `{"schema":"verdi.context-controller/next-stamp-request/v1"}` + "\n"

// contextOwnerVerifyExpansionRequest carries a nested canonical operand, so
// the two rows together cover both the empty and the structured arm shapes.
const contextOwnerVerifyExpansionRequest = `{"key":{"epoch":"epoch-1","flight":"flight-1","lane":"lane-1"},` +
	`"schema":"verdi.context-controller/verify-expansion-request/v1"}` + "\n"

const contextOwnerExpansionRoot = "sha256:1111111111111111111111111111111111111111111111111111111111111111"

// contextOwnerDigest is §3.1's cross-process identity binding, spelled out
// here so a mistake in the bridge's own digest cannot be self-confirming.
func contextOwnerDigest(privateRequest string) string {
	sum := sha256.Sum256([]byte(privateRequest))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// contextOwnerCallDocument assembles the exact canonical public call. Canonical
// JSON sorts object keys, so this member order is the only legal one.
func contextOwnerCallDocument(operation, requestArm, privateRequest string) string {
	return `{"controller_request_digest":"` + contextOwnerDigest(privateRequest) +
		`","operation":"` + operation + `","request":` + requestArm +
		`,"schema":"verdi.context-owner-call/v1"}` + "\n"
}

// contextOwnerReplyDocument wraps a byte-for-byte copy of the call the owner
// answered around one typed public result.
func contextOwnerReplyDocument(callDocument, resultArm string) string {
	return `{"call":` + strings.TrimSuffix(callDocument, "\n") + `,"result":` + resultArm +
		`,"schema":"verdi.context-owner-reply/v1"}` + "\n"
}

func contextOwnerNextStampCall() string {
	return contextOwnerCallDocument("next-stamp",
		`{"schema":"verdi.context-owner/next-stamp-request/v1"}`, contextOwnerNextStampRequest)
}

func contextOwnerVerifyExpansionCall() string {
	return contextOwnerCallDocument("verify-expansion",
		`{"key":{"epoch":"epoch-1","flight":"flight-1","lane":"lane-1"},`+
			`"schema":"verdi.context-owner/verify-expansion-request/v1"}`,
		contextOwnerVerifyExpansionRequest)
}

// --- in-process command proofs -------------------------------------------

// TestCmdContextOwnerGrammar proves the closed grammar §2 fixes: exactly one
// non-empty --operation value after exactly one decode or encode verb.
//
// Every row runs from a directory that is not a store and never becomes one,
// so a refusal here is a refusal on argument shape alone — never a store-root
// failure standing in for one.
func TestCmdContextOwnerGrammar(t *testing.T) {
	t.Chdir(t.TempDir())

	cases := [][]string{
		{"owner"},
		{"owner", "decode"},
		{"owner", "encode"},
		{"owner", "frobnicate", "--operation", "next-stamp"},
		{"owner", "", "--operation", "next-stamp"},
		{"owner", "decode", "--operation"},
		{"owner", "decode", "--operation", ""},
		{"owner", "decode", "--operation="},
		{"owner", "decode", "--operation", "next-stamp", "--operation", "next-stamp"},
		{"owner", "decode", "--operation=next-stamp", "--operation=next-stamp"},
		{"owner", "decode", "--operation", "next-stamp", "extra"},
		{"owner", "decode", "next-stamp"},
		{"owner", "decode", "--bogus", "next-stamp"},
		{"owner", "decode", "--operation", "next-stamp", "--out", "x.json"},
		{"owner", "decode", "encode", "--operation", "next-stamp"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			// Valid stdin: a grammar refusal must happen before it is read.
			stdin := strings.NewReader(contextOwnerNextStampRequest)
			if got := cmdContext(args, stdin, &stdout, &stderr); got != 2 {
				t.Fatalf("cmdContext(%v) = %d, want 2", args, got)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty on a usage error", stdout.String())
			}
			if stderr.String() != contextOwnerUsage+"\n" {
				t.Fatalf("stderr = %q, want exactly %q", stderr.String(), contextOwnerUsage+"\n")
			}
			if stdin.Len() != len(contextOwnerNextStampRequest) {
				t.Fatal("the command read stdin before refusing the grammar")
			}
		})
	}
}

// TestCmdContextOwnerUnknownOperation proves the registry is closed and is
// consulted before stdin is read. Operation 23 is the load-bearing row: it is
// a real ATC controller operation that deliberately never enters this wire.
func TestCmdContextOwnerUnknownOperation(t *testing.T) {
	t.Chdir(t.TempDir())

	for _, operation := range []string{"frobnicate", "resolve-claim-mcp", "Next-Stamp", "next-stamp "} {
		for _, verb := range []string{"decode", "encode"} {
			t.Run(verb+" "+operation, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				stdin := strings.NewReader(contextOwnerNextStampRequest)
				args := []string{"owner", verb, "--operation", operation}
				if got := cmdContext(args, stdin, &stdout, &stderr); got != 2 {
					t.Fatalf("cmdContext(%v) = %d, want 2", args, got)
				}
				if stdout.Len() != 0 {
					t.Fatalf("stdout = %q, want empty on failure", stdout.String())
				}
				want := "context owner " + verb + ": unknown-operation\n"
				if stderr.String() != want {
					t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
				}
				if stdin.Len() != len(contextOwnerNextStampRequest) {
					t.Fatal("the command read stdin before refusing the operation")
				}
			})
		}
	}
}

// TestCmdContextOwnerRoundTrip proves the exact decode and encode bytes for an
// empty-operand and a structured-operand operation, in both directions.
func TestCmdContextOwnerRoundTrip(t *testing.T) {
	t.Chdir(t.TempDir())

	cases := []struct {
		operation      string
		privateRequest string
		call           string
		resultArm      string
		privateResult  string
	}{
		{
			operation:      "next-stamp",
			privateRequest: contextOwnerNextStampRequest,
			call:           contextOwnerNextStampCall(),
			resultArm: `{"schema":"verdi.context-owner/next-stamp-result/v1",` +
				`"stamp":"2026-08-28T12:34:56.123456789Z"}`,
			privateResult: `{"schema":"verdi.context-controller/next-stamp-result/v1",` +
				`"stamp":"2026-08-28T12:34:56.123456789Z"}` + "\n",
		},
		{
			operation:      "verify-expansion",
			privateRequest: contextOwnerVerifyExpansionRequest,
			call:           contextOwnerVerifyExpansionCall(),
			resultArm: `{"facts":{"failure":"","root":"` + contextOwnerExpansionRoot +
				`","state":"proven","witnesses":[]},` +
				`"schema":"verdi.context-owner/verify-expansion-result/v1"}`,
			privateResult: `{"facts":{"failure":"","root":"` + contextOwnerExpansionRoot +
				`","state":"proven","witnesses":[]},` +
				`"schema":"verdi.context-controller/verify-expansion-result/v1"}` + "\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.operation+" decode", func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := []string{"owner", "decode", "--operation", tc.operation}
			if got := cmdContext(args, strings.NewReader(tc.privateRequest), &stdout, &stderr); got != 0 {
				t.Fatalf("cmdContext(%v) = %d, want 0\nstderr: %s", args, got, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty on success", stderr.String())
			}
			if stdout.String() != tc.call {
				t.Fatalf("stdout\n got %q\nwant %q", stdout.String(), tc.call)
			}
		})
		t.Run(tc.operation+" encode", func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			reply := contextOwnerReplyDocument(tc.call, tc.resultArm)
			args := []string{"owner", "encode", "--operation", tc.operation}
			if got := cmdContext(args, strings.NewReader(reply), &stdout, &stderr); got != 0 {
				t.Fatalf("cmdContext(%v) = %d, want 0\nstderr: %s", args, got, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty on success", stderr.String())
			}
			if stdout.String() != tc.privateResult {
				t.Fatalf("stdout\n got %q\nwant %q", stdout.String(), tc.privateResult)
			}
		})
	}
}

// TestCmdContextOwnerMalformedInput proves every malformed input shape exits 2
// with an empty stdout and one closed diagnostic class. A document the bridge
// refuses may contain anything the caller sent, so the class — never the
// value — is what reaches stderr.
func TestCmdContextOwnerMalformedInput(t *testing.T) {
	t.Chdir(t.TempDir())

	const secret = "s3cr3t-marker-value"
	call := contextOwnerNextStampCall()
	validReply := contextOwnerReplyDocument(call,
		`{"schema":"verdi.context-owner/next-stamp-result/v1","stamp":"2026-08-28T12:34:56.123456789Z"}`)

	cases := []struct {
		name  string
		verb  string
		input string
		class string
	}{
		{"empty stdin", "decode", "", "request-refused"},
		{"lone newline", "decode", "\n", "request-refused"},
		{"json null", "decode", "null\n", "request-refused"},
		{"not json", "decode", secret + "\n", "request-refused"},
		{"missing trailing newline", "decode",
			strings.TrimSuffix(contextOwnerNextStampRequest, "\n"), "request-refused"},
		{"two documents", "decode",
			contextOwnerNextStampRequest + contextOwnerNextStampRequest, "request-refused"},
		{"unknown member", "decode",
			`{"extra":"` + secret + `","schema":"verdi.context-controller/next-stamp-request/v1"}` + "\n",
			"request-refused"},
		{"another operation's schema", "decode",
			`{"schema":"verdi.context-controller/verify-epoch-request/v1"}` + "\n", "request-refused"},
		{"already-public schema", "decode",
			`{"schema":"verdi.context-owner/next-stamp-request/v1"}` + "\n", "request-refused"},
		{"empty stdin", "encode", "", "reply-refused"},
		{"private payload offered as a reply", "encode",
			contextOwnerNextStampRequest, "reply-refused"},
		{"reply for another operation", "encode",
			contextOwnerReplyDocument(contextOwnerVerifyExpansionCall(),
				`{"facts":{"failure":"","root":"`+contextOwnerExpansionRoot+
					`","state":"proven","witnesses":[]},`+
					`"schema":"verdi.context-owner/verify-expansion-result/v1"}`), "reply-refused"},
		{"reply with a corrupted digest", "encode",
			strings.Replace(validReply, contextOwnerDigest(contextOwnerNextStampRequest),
				"sha256:"+strings.Repeat("0", 64), 1), "reply-refused"},
		{"reply with trailing data", "encode", validReply + "{}\n", "reply-refused"},
		{"reply with an unknown member", "encode",
			`{"extra":"` + secret + `",` + strings.TrimPrefix(validReply, "{"), "reply-refused"},
	}

	for _, tc := range cases {
		t.Run(tc.verb+" "+tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := []string{"owner", tc.verb, "--operation", "next-stamp"}
			if got := cmdContext(args, strings.NewReader(tc.input), &stdout, &stderr); got != 2 {
				t.Fatalf("cmdContext(%v) = %d, want 2", args, got)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty on failure", stdout.String())
			}
			want := "context owner " + tc.verb + ": " + tc.class + "\n"
			if stderr.String() != want {
				t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
			}
		})
	}
}

// TestCmdContextOwnerCeilingIsExclusive proves §2's bound: a document AT the
// 32 MiB controller-frame ceiling is refused as one operational failure rather
// than read in fragments, and a document below it is still accepted.
func TestCmdContextOwnerCeilingIsExclusive(t *testing.T) {
	t.Chdir(t.TempDir())

	if sealedexec.OwnerFrameCeiling != 32<<20 {
		t.Fatalf("OwnerFrameCeiling = %d, want the 32 MiB controller-frame ceiling", sealedexec.OwnerFrameCeiling)
	}

	var stdout, stderr bytes.Buffer
	atCeiling := io.LimitReader(contextOwnerFiller{}, int64(sealedexec.OwnerFrameCeiling))
	args := []string{"owner", "decode", "--operation", "next-stamp"}
	if got := cmdContext(args, atCeiling, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdContext at the ceiling = %d, want 2", got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on failure", stdout.String())
	}
	if want := "context owner decode: input-too-large\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
	}

	// One byte below the ceiling is inside the bound, so the refusal must come
	// from the payload's own shape rather than from its size.
	stdout.Reset()
	stderr.Reset()
	belowCeiling := io.LimitReader(contextOwnerFiller{}, int64(sealedexec.OwnerFrameCeiling)-1)
	if got := cmdContext(args, belowCeiling, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdContext below the ceiling = %d, want 2", got)
	}
	if want := "context owner decode: request-refused\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
	}
}

// contextOwnerFiller is an inexhaustible source of one legal JSON byte, used
// to reach the frame ceiling without materializing a 32 MiB literal.
type contextOwnerFiller struct{}

func (contextOwnerFiller) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

// TestCmdContextOwnerStdoutWriteFailure proves a mapped document that could
// not be delivered is an operational failure, never a success and never a
// verdict: the caller received nothing, and no story was judged.
func TestCmdContextOwnerStdoutWriteFailure(t *testing.T) {
	t.Chdir(t.TempDir())

	var stderr bytes.Buffer
	args := []string{"owner", "decode", "--operation", "next-stamp"}
	got := cmdContext(args, strings.NewReader(contextOwnerNextStampRequest), contextFailingWriter{}, &stderr)
	if got != 2 {
		t.Fatalf("cmdContext with a failing stdout = %d, want 2", got)
	}
	if want := "context owner decode: output-failed\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
	}
}

// contextOwnerShortWriter accepts a prefix of every write and reports success,
// the in-process equivalent of a destination that stops accepting bytes
// without reporting why. io.Writer forbids this, but a real writer on the far
// side of a pipe, a filter, or a third-party wrapper can and does do it.
type contextOwnerShortWriter struct{ accepted []byte }

func (w *contextOwnerShortWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n := len(p) / 2
	if n == 0 {
		n = 1
	}
	w.accepted = append(w.accepted, p[:n]...)
	return n, nil
}

// TestCmdContextOwnerShortWrite proves a partially accepted document is an
// operational failure, never a success.
//
// The document is the whole answer: a caller that received a prefix of one
// canonical JSON object holds a noncanonical, undecodable fragment. Exiting 0
// there would report a translation the controller never received, which is
// worse than any refusal — the caller has no way to tell the truncated bytes
// from the complete ones except by failing to decode them.
func TestCmdContextOwnerShortWrite(t *testing.T) {
	t.Chdir(t.TempDir())

	reply := contextOwnerReplyDocument(contextOwnerNextStampCall(),
		`{"schema":"verdi.context-owner/next-stamp-result/v1","stamp":"2026-08-28T12:34:56.123456789Z"}`)

	cases := []struct {
		verb  string
		input string
	}{
		{"decode", contextOwnerNextStampRequest},
		{"encode", reply},
	}
	for _, tc := range cases {
		t.Run(tc.verb, func(t *testing.T) {
			var stderr bytes.Buffer
			stdout := &contextOwnerShortWriter{}
			args := []string{"owner", tc.verb, "--operation", "next-stamp"}
			if got := cmdContext(args, strings.NewReader(tc.input), stdout, &stderr); got != 2 {
				t.Fatalf("cmdContext(%v) with a short-writing stdout = %d, want 2", args, got)
			}
			if want := "context owner " + tc.verb + ": " + contextOwnerOutputFailed + "\n"; stderr.String() != want {
				t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
			}
			// The refusal is about what the destination accepted, so whatever
			// reached it must not be mistakable for the whole document.
			if bytes.HasSuffix(stdout.accepted, []byte("}\n")) {
				t.Fatalf("the short-written prefix %q looks like a complete document", stdout.accepted)
			}
		})
	}
}

// TestCmdContextOwnerStdinReadFailure proves an unreadable stdin is classified
// separately from a malformed document: nothing was decided about a payload
// that never arrived.
func TestCmdContextOwnerStdinReadFailure(t *testing.T) {
	t.Chdir(t.TempDir())

	var stdout, stderr bytes.Buffer
	args := []string{"owner", "encode", "--operation", "next-stamp"}
	if got := cmdContext(args, contextOwnerFailingReader{}, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdContext with an unreadable stdin = %d, want 2", got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on failure", stdout.String())
	}
	if want := "context owner encode: input-unreadable\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
	}
}

type contextOwnerFailingReader struct{}

func (contextOwnerFailingReader) Read([]byte) (int, error) {
	return 0, errors.New("/private/var/secret-checkout: stdin unavailable")
}

// TestCmdContextOwnerNoStoreDiscovery proves the verb is dispatched before any
// store lookup: it succeeds from a read-only directory that is not a store and
// never becomes one, and leaves that directory byte-identical.
func TestCmdContextOwnerNoStoreDiscovery(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "work")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	if err := os.Chmod(nested, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(nested, 0o755) })

	before := contextOwnerSnapshot(t, dir)

	var stdout, stderr bytes.Buffer
	args := []string{"owner", "decode", "--operation", "next-stamp"}
	if got := cmdContext(args, strings.NewReader(contextOwnerNextStampRequest), &stdout, &stderr); got != 0 {
		t.Fatalf("cmdContext(%v) = %d, want 0 from a rootless directory\nstderr: %s", args, got, stderr.String())
	}
	if stdout.String() != contextOwnerNextStampCall() {
		t.Fatalf("stdout = %q, want the canonical call", stdout.String())
	}

	if after := contextOwnerSnapshot(t, dir); after != before {
		t.Fatalf("the working directory changed\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// contextOwnerSnapshot renders a directory tree as a deterministic string:
// every relative path with its mode, size and content digest. Comparing two
// snapshots is what makes "nothing was written" a checked statement rather
// than an absence of evidence.
func contextOwnerSnapshot(t *testing.T, root string) string {
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
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			lines = append(lines, fmt.Sprintf("dir  %s %04o", rel, info.Mode().Perm()))
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		lines = append(lines, fmt.Sprintf("file %s %04o %d %s", rel, info.Mode().Perm(), info.Size(), hex.EncodeToString(sum[:])))
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return strings.Join(lines, "\n")
}

// --- built-binary proofs -------------------------------------------------

// TestContextOwnerE2EEffectFree runs the real binary as a real OS subprocess
// under a hostile environment and proves §2's effect-freedom clause.
//
// The environment is poisoned in the three ways a store-discovering or
// ambient-executing verb would notice: the working directory is read-only and
// is not a store, HOME names a path that does not exist, and PATH contains
// nothing but marker shims for every executable this build could plausibly
// reach for. A shim that ran would leave a file behind; none may.
//
// FD 3 is the sealed controller channel. The child inherits the write end of a
// real pipe as FD 3 and must leave it untouched, so the bridge cannot be
// answering from — or reporting to — the controller it translates for.
func TestContextOwnerE2EEffectFree(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)

	root := t.TempDir()
	work := filepath.Join(root, "work")
	markers := filepath.Join(root, "markers")
	shims := filepath.Join(root, "shims")
	for _, dir := range []string{work, markers, shims} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Every executable name a store-discovering, credential-reading, or
	// provider-launching path could reach for through PATH.
	for _, name := range []string{"git", "sh", "bash", "env", "verdi", "codex", "claude", "ssh", "curl"} {
		script := "#!/bin/sh\n: > \"" + filepath.Join(markers, name) + "\"\nexit 0\n"
		if err := os.WriteFile(filepath.Join(shims, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(work, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(work, 0o755) })

	controllerRead, controllerWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = controllerRead.Close() }()

	cmd := exec.Command(bin, "context", "owner", "decode", "--operation", "next-stamp")
	cmd.Dir = work
	cmd.Env = []string{
		"PATH=" + shims,
		"HOME=" + filepath.Join(root, "no-such-home"),
	}
	cmd.ExtraFiles = []*os.File{controllerWrite} // becomes the child's FD 3
	cmd.Stdin = strings.NewReader(contextOwnerNextStampRequest)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	before := contextOwnerSnapshot(t, root)
	runErr := cmd.Run()
	// The parent's copy of the write end must go before the read below, or
	// that read waits on an end this process is still holding open.
	_ = controllerWrite.Close()

	if runErr != nil {
		t.Fatalf("verdi context owner decode: %v\nstderr: %s", runErr, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on success", stderr.String())
	}
	if stdout.String() != contextOwnerNextStampCall() {
		t.Fatalf("stdout\n got %q\nwant %q", stdout.String(), contextOwnerNextStampCall())
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
	if after := contextOwnerSnapshot(t, root); after != before {
		t.Fatalf("the filesystem changed\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestContextOwnerE2EEncode proves the encode direction through the same real
// subprocess, so both verbs are witnessed as built commands rather than only
// as in-process calls.
func TestContextOwnerE2EEncode(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	reply := contextOwnerReplyDocument(contextOwnerNextStampCall(),
		`{"schema":"verdi.context-owner/next-stamp-result/v1","stamp":"2026-08-28T12:34:56.123456789Z"}`)
	want := `{"schema":"verdi.context-controller/next-stamp-result/v1",` +
		`"stamp":"2026-08-28T12:34:56.123456789Z"}` + "\n"

	cmd := exec.Command(bin, "context", "owner", "encode", "--operation", "next-stamp")
	cmd.Dir = dir
	cmd.Env = []string{"PATH=", "HOME=" + filepath.Join(dir, "no-such-home")}
	cmd.Stdin = strings.NewReader(reply)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("verdi context owner encode: %v\nstderr: %s", err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on success", stderr.String())
	}
	if stdout.String() != want {
		t.Fatalf("stdout\n got %q\nwant %q", stdout.String(), want)
	}
}

// TestContextOwnerE2EFailureLeavesStdoutEmpty proves the built command's
// stdout stays empty on a real failure and its exit status is the operational
// 2, never a verdict 1 — the bridge asks no question a verdict could answer.
func TestContextOwnerE2EFailureLeavesStdoutEmpty(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	cmd := exec.Command(bin, "context", "owner", "decode", "--operation", "next-stamp")
	cmd.Dir = dir
	cmd.Env = []string{"PATH=", "HOME=" + filepath.Join(dir, "no-such-home")}
	cmd.Stdin = strings.NewReader(`{"schema":"nonsense"}` + "\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("verdi context owner decode on a bad payload = %v, want an exit error", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Fatalf("exit = %d, want 2", exitErr.ExitCode())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on failure", stdout.String())
	}
	if want := "context owner decode: request-refused\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
	}
}

// TestContextOwnerE2ECancellation proves a command whose stdin never ends is
// killed rather than answering, and that it delivered no document first. A
// bridge that emitted a partial or speculative call under cancellation would
// hand its caller an owner call that no controller ever made.
func TestContextOwnerE2ECancellation(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stdinWrite.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "context", "owner", "decode", "--operation", "next-stamp")
	cmd.Dir = dir
	cmd.Env = []string{"PATH=", "HOME=" + filepath.Join(dir, "no-such-home")}
	cmd.Stdin = stdinRead
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// The child now owns the read end; dropping the parent's copy is what
	// leaves the write below as the only thing keeping the pipe open.
	_ = stdinRead.Close()
	// A partial document: the command is now blocked on a stdin that will
	// never reach EOF, which is exactly the state cancellation must resolve.
	if _, err := stdinWrite.WriteString(`{"schema":`); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	cancel()

	if err := cmd.Wait(); err == nil {
		t.Fatal("the canceled command exited 0")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty after cancellation", stdout.String())
	}
}
